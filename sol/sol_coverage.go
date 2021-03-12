package sol

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/rand"
	"sort"
	"strconv"
	"strings"
	"text/template"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/itchyny/gojq"
	"github.com/pkg/errors"
	"github.com/tidwall/sjson"
)

var (
	astStateMutabilityKeys, _  = gojq.Parse("..|.stateMutability?")
	astStateMutabilityPaths, _ = gojq.Parse("path(..|.stateMutability?)")
	astSrc, _                  = gojq.Parse(".src")
	astBlocksKeys, _           = gojq.Parse(`..| select(.nodeType? == "Block")`)
	astBlocksPaths, _          = gojq.Parse(`path(..| select(.nodeType? == "Block"))`)
	astContractNodePaths, _    = gojq.Parse(`path(.nodes[] | select(.nodeType == "ContractDefinition" and .contractKind != "interface"))`)
	astInnerBlocks, _          = gojq.Parse(`. | select(.nodeType == "Block")`)
)

func addCoverageMarkers(fileIdx int, contractName string, ast json.RawMessage) (out json.RawMessage, statements [][]int, err error) {
	eventDefinitionID := randN()

	contractPaths, err := contractDefinitionPaths(ast)
	if err != nil {
		return nil, nil, err
	}

	for _, path := range contractPaths {
		coverageEventIDAST := newCoverageEventID(contractName, eventDefinitionID+uint64(fileIdx))

		// append ___coverage event definition onto AST node of a ContractDefinition
		eventDefinitionAST := newEventDefinition(eventDefinitionID + uint64(fileIdx))
		ast, err = sjson.SetBytes(ast, path+".nodes.-1", eventDefinitionAST)
		if err != nil {
			err = errors.Wrap(err, "sjson failed to parse value")
			return nil, nil, err
		}

		// append ___coverage_id constant onto AST node of every contract definition
		ast, err = sjson.SetBytes(ast, path+".nodes.-1", coverageEventIDAST)
		if err != nil {
			err = errors.Wrap(err, "sjson failed to parse value")
			return nil, nil, err
		}
	}

	stateMutabilities, err := getStateMutabilities(ast)
	if err != nil {
		return nil, statements, err
	}

	for path, value := range stateMutabilities {
		if value != "view" && value != "pure" {
			continue
		}

		ast, err = sjson.SetBytes(ast, path, "nonpayable")
		if err != nil {
			return nil, statements, err
		}
	}

	pathsSortedByDepth, blocksMap, err := getBlocks(ast)
	if err != nil {
		return nil, nil, err
	}

	statements = make([][]int, 0, len(blocksMap))

	for _, path := range pathsSortedByDepth {
		var statementsSrc [][]int

		block := blocksMap[path]
		block, statementsSrc, err = orchestrateBlock(block, eventDefinitionID, fileIdx)
		if err != nil {
			return nil, statements, err
		}

		statements = append(statements, statementsSrc...)

		ast, err = sjson.SetBytes(ast, path, block)
		if err != nil {
			return nil, statements, err
		}
	}

	return ast, statements, nil
}

type Values []interface{}

func contractDefinitionPaths(ast json.RawMessage) (paths []string, err error) {
	var in interface{}
	if err = json.Unmarshal(ast, &in); err != nil {
		return nil, err
	}

	iter := astContractNodePaths.Run(in)
	for {
		v, ok := iter.Next()
		if !ok {
			break
		}

		if err, ok := v.(error); ok {
			err = errors.Wrap(err, "failed to parse JSON")
			return nil, err
		}

		paths = append(paths, joinPath(v))
	}

	return paths, nil
}

func orchestrateBlock(blockAST json.RawMessage, eventDefinitionID uint64, fileIdx int) (out json.RawMessage, statements [][]int, err error) {
	var block map[string]interface{}
	if err = json.Unmarshal(blockAST, &block); err != nil {
		return nil, nil, err
	}

	list, ok := block["statements"].([]interface{})
	if !ok {
		err = errors.New("expected statements in the block")
		return
	}

	var statementIdx int
	for _, statement := range list {
		v, _ := json.Marshal(statement)
		statementAST := json.RawMessage(v)

		// re-add statements with correct orchestration
		start, end, file, err := getStatementSrcLocation(statementAST)
		if err != nil {
			return nil, statements, err
		}

		statements = append(statements, []int{start, end, file})

		// orchestrate inner blocks
		{
			pathsSortedByDepth, blocksMap, err := getBlocks(statementAST)
			if err != nil {
				return nil, nil, err
			}

			for _, blockPath := range pathsSortedByDepth {
				var statementsSrc [][]int

				innerBlock := blocksMap[blockPath]
				innerBlock, statementsSrc, err = orchestrateBlock(innerBlock, eventDefinitionID, fileIdx)
				if err != nil {
					return nil, statements, err
				}

				statements = append(statements, statementsSrc...)

				if blockPath == "" {
					// statement is a block itself
					v, _ := json.Marshal(innerBlock)
					statementAST = json.RawMessage(v)
					continue
				} else {
					statementAST, err = sjson.SetBytes(statementAST, blockPath, innerBlock)
				}

				if err != nil {
					return nil, statements, err
				}
			}
		}

		// now proceed with statement
		newStatementAST, isRequire, err := orchestrateRequireStatement(statementAST)
		if err != nil {
			return nil, statements, err
		}

		if !isRequire {
			marker := newCoverageMarker(eventDefinitionID+uint64(fileIdx), uint64(start), uint64(end), uint64(file))
			blockAST, err = sjson.SetBytes(blockAST, fmt.Sprintf("statements.%d", statementIdx), marker)
			if err != nil {
				return nil, statements, err
			}
			statementIdx++

			blockAST, err = sjson.SetBytes(blockAST, fmt.Sprintf("statements.%d", statementIdx), newStatementAST)
			if err != nil {
				return nil, statements, err
			}
			statementIdx++
		} else { // is require and we have new orchestrated statement
			blockAST, err = sjson.SetBytes(blockAST, fmt.Sprintf("statements.%d", statementIdx), newStatementAST)
			if err != nil {
				return nil, statements, err
			}
			statementIdx++
		}
	}

	return blockAST, statements, nil
}

func getBlocks(ast json.RawMessage) (pathsSortedByDepth []string, blocks map[string]json.RawMessage, err error) {
	var in interface{}
	if err = json.Unmarshal(ast, &in); err != nil {
		return nil, nil, err
	}

	astBlockMap := make(map[string]json.RawMessage)

	var stage0 []interface{}
	var stage1 []interface{}

	iter := astBlocksKeys.Run(in)
	for {
		v, ok := iter.Next()
		if !ok {
			break
		}

		if err, ok := v.(error); ok {
			err = errors.Wrap(err, "failed to parse JSON")
			return nil, nil, err
		}

		stage0 = append(stage0, v)
	}

	iter = astBlocksPaths.Run(in)
	for {
		v, ok := iter.Next()
		if !ok {
			break
		}

		if err, ok := v.(error); ok {
			err = errors.Wrap(err, "failed to parse JSON")
			return nil, nil, err
		}

		stage1 = append(stage1, v)
	}

	pathsSortedByDepth = make([]string, 0, len(stage1))

	for i := range stage0 {
		if stage0[i] == nil {
			continue
		}

		block, ok := stage0[i].(map[string]interface{})
		if !ok {
			continue
		}

		path := joinPath(stage1[i])
		if strings.Contains(path, ".AST") {
			// do not support Yul blocks
			continue
		}

		pathsSortedByDepth = append(pathsSortedByDepth, path)

		rawBlock, err := json.Marshal(block)
		if err != nil {
			return nil, nil, err
		}

		astBlockMap[path] = rawBlock
	}

	sort.SliceStable(pathsSortedByDepth, func(i, j int) bool {
		pathA := pathsSortedByDepth[i]
		pathB := pathsSortedByDepth[j]

		return len(strings.Split(pathA, ".")) > len(strings.Split(pathB, "."))
	})

	return pathsSortedByDepth, astBlockMap, nil
}

var ErrStatementNoSrc = errors.New("statement without src reference")

func srcToLocation(src string) (start, end, file int, err error) {
	parts := strings.Split(src, ":")
	if len(parts) != 3 {
		err = errors.Errorf("src reference has wrong amount of parts: %d", len(parts))
		return
	}

	start, _ = strconv.Atoi(parts[0])
	end, _ = strconv.Atoi(parts[1])
	file, _ = strconv.Atoi(parts[2])

	return
}

func getStatementSrcLocation(ast json.RawMessage) (start, end, file int, err error) {
	var statement map[string]interface{}
	if err = json.Unmarshal(ast, &statement); err != nil {
		return
	}

	src, ok := statement["src"].(string)
	if !ok {
		err = ErrStatementNoSrc
		return
	}

	start, end, file, err = srcToLocation(src)
	if err != nil {
		return
	}

	iter := astInnerBlocks.Run(statement)
	v, ok := iter.Next()
	if ok {
		if err, ok = v.(error); ok {
			return
		}

		// statement is a block itself
		block, ok := v.(map[string]interface{})
		if !ok {
			err = errors.Errorf("block expected as map[string]interface{}, got %T", v)
			return
		}

		blockStatements, ok := block["statements"].([]interface{})
		if !ok || len(blockStatements) == 0 {
			return
		}

		blockStatement, ok := blockStatements[0].(map[string]interface{})
		if !ok {
			err = errors.Errorf("first statement expected as map[string]interface{}, got %T", v)
			return
		}

		blockStatementSrc, ok := blockStatement["src"].(string)
		if !ok {
			err = errors.New("statement expected to have src reference")
			return
		}

		srcStart, _, srcFile, srcErr := srcToLocation(blockStatementSrc)
		if srcErr != nil {
			err = errors.Wrap(srcErr, "failed to parse src reference of the first inner statement of a block")
			return
		} else if srcFile != file {
			err = errors.Errorf("wrong inner statment src reference: %s", blockStatementSrc)
			return
		}

		// now we can set the real disposition of the parent block-statement, based on the
		// position of its first child statement.
		end = srcStart - start
	}

	return
}

func orchestrateRequireStatement(ast json.RawMessage) (out json.RawMessage, isRequire bool, err error) {
	var statement map[string]interface{}
	if err = json.Unmarshal(ast, &statement); err != nil {
		err = errors.Wrap(err, "failed to unmarshal statement AST")
		return
	}

	out = ast

	expression, ok := statement["expression"].(map[string]interface{})
	if !ok {
		return
	}

	nodeType, ok := expression["nodeType"].(string)
	if !ok || nodeType != "FunctionCall" {
		return
	}

	subExpression := expression["expression"].(map[string]interface{})
	if !ok {
		return
	}

	if subExpressionName, ok := subExpression["name"].(string); !ok || subExpressionName != "require" {
		return
	}

	isRequire = true

	expressionArguments := expression["arguments"].([]interface{})
	if !ok {
		return
	}

	expressionArgument2, ok := expressionArguments[1].(map[string]interface{})
	if !ok {
		return
	}

	textValue, ok := expressionArgument2["value"].(string)
	if !ok {
		return
	}

	statementSrc, ok := statement["src"].(string)
	if !ok {
		return
	}

	start, end, file, err := srcToLocation(statementSrc)
	if err != nil {
		return
	}

	textValue = fmt.Sprintf("%s @coverage,%d,%d,%d", textValue, start, end, file)
	textValueHash := crypto.Keccak256Hash([]byte(textValue)).Bytes()
	ast, _ = sjson.SetBytes(ast, "expression.arguments.1.typeDescriptions.typeString", fmt.Sprintf("t_stringliteral_%x", textValueHash))
	ast, _ = sjson.SetBytes(ast, "expression.arguments.1.typeDescriptions.typeIdentifier", fmt.Sprintf("literal_string \"%s\"", textValue))
	ast, _ = sjson.SetBytes(ast, "expression.arguments.1.value", textValue)
	ast, _ = sjson.SetBytes(ast, "expression.arguments.1.hexValue", fmt.Sprintf("%x", textValue))
	ast, _ = sjson.SetBytes(ast, "expression.expression.argumentTypes.1.typeString", fmt.Sprintf("t_stringliteral_%x", textValueHash))
	ast, _ = sjson.SetBytes(ast, "expression.expression.argumentTypes.1.typeIdentifier", fmt.Sprintf("literal_string \"%s\"", textValue))
	out = ast

	return
}

func getStateMutabilities(ast json.RawMessage) (map[string]string, error) {
	var in interface{}
	if err := json.Unmarshal(ast, &in); err != nil {
		return nil, err
	}

	astStateMutabilityMap := make(map[string]string)

	var stage0 []interface{}
	var stage1 []interface{}

	iter := astStateMutabilityKeys.Run(in)
	for {
		v, ok := iter.Next()
		if !ok {
			break
		}

		if err, ok := v.(error); ok {
			err = errors.Wrap(err, "failed to parse JSON")
			return nil, err
		}

		stage0 = append(stage0, v)
	}

	iter = astStateMutabilityPaths.Run(in)
	for {
		v, ok := iter.Next()
		if !ok {
			break
		}

		if err, ok := v.(error); ok {
			err = errors.Wrap(err, "failed to parse JSON")
			return nil, err
		}

		stage1 = append(stage1, v)
	}

	for i := range stage0 {
		if stage0[i] == nil {
			continue
		}

		path := joinPath(stage1[i])
		if strings.Contains(path, ".AST") {
			// do not support Yul blocks
			continue
		}

		astStateMutabilityMap[path] = stage0[i].(string)
	}

	return astStateMutabilityMap, nil
}

func joinPath(in interface{}) string {
	buf := new(bytes.Buffer)

	keys := in.([]interface{})
	for idx, k := range keys {
		if idx == len(keys)-1 {
			fmt.Fprintf(buf, "%v", k)
			continue
		}

		fmt.Fprintf(buf, "%v.", k)
	}

	return buf.String()
}

type EventDefinitionArgs struct {
	EventDefinitionID uint64
	RandomIDs         []uint64
}

func randN() uint64 {
	const offset = 1000000000
	return uint64(rand.Int63n(9*offset) + offset)
}

func newEventDefinition(id uint64) (ast json.RawMessage) {
	buf := new(bytes.Buffer)

	opts := EventDefinitionArgs{
		EventDefinitionID: id,
		RandomIDs:         make([]uint64, 8),
	}
	for i := range opts.RandomIDs {
		opts.RandomIDs[i] = randN()
	}

	if err := eventDefinitionTemplate.Execute(buf, opts); err != nil {
		panic(err)
	}

	return buf.Bytes()
}

// uint64ToAstHex is a weird hack because I don't understand "Hex" values
// in AST dumps that Solc generates.
func uint64ToAstHex(value uint64) (result string) {
	str := fmt.Sprintf("%d", value)
	for _, digit := range str {
		result += "3" + string(digit)
	}

	return result
}

type CoverageMarkerArgs struct {
	EventDefinitionID    uint64
	RandomIDs            []uint64
	EventCallArgValue    []string
	EventCallArgHexValue []string
}

func newCoverageMarker(eventDefinitionID uint64, start, end, file uint64) json.RawMessage {
	buf := new(bytes.Buffer)

	opts := CoverageMarkerArgs{
		EventDefinitionID: eventDefinitionID,
		RandomIDs:         make([]uint64, 6),
		EventCallArgValue: []string{
			fmt.Sprintf("%d", start),
			fmt.Sprintf("%d", end),
			fmt.Sprintf("%d", file),
		},
		EventCallArgHexValue: []string{
			uint64ToAstHex(start),
			uint64ToAstHex(end),
			uint64ToAstHex(file),
		},
	}
	for i := range opts.RandomIDs {
		opts.RandomIDs[i] = randN()
	}

	if err := coverageMarkerTemplate.Execute(buf, opts); err != nil {
		panic(err)
	}

	return buf.Bytes()
}

type CoverageEventIDArgs struct {
	EventDefinitionID    uint64
	EventDefinitionIDHex string
	RandomIDs            []uint64
	ContractName         string
}

func newCoverageEventID(contractName string, eventDefinitionID uint64) json.RawMessage {
	buf := new(bytes.Buffer)

	opts := CoverageEventIDArgs{
		EventDefinitionID:    eventDefinitionID,
		EventDefinitionIDHex: uint64ToAstHex(eventDefinitionID),
		RandomIDs:            make([]uint64, 4),
		ContractName:         contractName,
	}
	for i := range opts.RandomIDs {
		opts.RandomIDs[i] = randN()
	}

	if err := coverageEventIDTemplate.Execute(buf, opts); err != nil {
		panic(err)
	}

	return buf.Bytes()
}

var eventDefinitionTemplate = template.Must(template.New("eventDefinition").Parse(`{
  "anonymous": false,
  "id": {{.EventDefinitionID}},
  "name": "___coverage_{{.EventDefinitionID}}",
  "nameLocation": "-1:-1:-1",
  "nodeType": "EventDefinition",
  "parameters": {
    "id": {{index .RandomIDs 0}},
    "nodeType": "ParameterList",
    "parameters": [
      {
        "constant": false,
        "id": {{index .RandomIDs 1}},
        "indexed": false,
        "mutability": "mutable",
        "name": "start",
        "nameLocation": "-1:-1:-1",
        "nodeType": "VariableDeclaration",
        "scope": {{index .RandomIDs 2}},
        "src": "-1:-1:-1",
        "stateVariable": false,
        "storageLocation": "default",
        "typeDescriptions": {
          "typeIdentifier": "t_uint64",
          "typeString": "uint64"
        },
        "typeName": {
          "id": {{index .RandomIDs 3}},
          "name": "uint64",
          "nodeType": "ElementaryTypeName",
          "src": "-1:-1:-1",
          "typeDescriptions": {
            "typeIdentifier": "t_uint64",
            "typeString": "uint64"
          }
        },
        "visibility": "internal"
      },
      {
        "constant": false,
        "id": {{index .RandomIDs 4}},
        "indexed": false,
        "mutability": "mutable",
        "name": "end",
        "nameLocation": "-1:-1:-1",
        "nodeType": "VariableDeclaration",
        "scope": {{index .RandomIDs 2}},
        "src": "-1:-1:-1",
        "stateVariable": false,
        "storageLocation": "default",
        "typeDescriptions": {
          "typeIdentifier": "t_uint64",
          "typeString": "uint64"
        },
        "typeName": {
          "id": {{index .RandomIDs 5}},
          "name": "uint64",
          "nodeType": "ElementaryTypeName",
          "src": "-1:-1:-1",
          "typeDescriptions": {
            "typeIdentifier": "t_uint64",
            "typeString": "uint64"
          }
        },
        "visibility": "internal"
      },
      {
        "constant": false,
        "id": {{index .RandomIDs 6}},
        "indexed": false,
        "mutability": "mutable",
        "name": "file",
        "nameLocation": "-1:-1:-1",
        "nodeType": "VariableDeclaration",
        "scope": {{index .RandomIDs 2}},
        "src": "-1:-1:-1",
        "stateVariable": false,
        "storageLocation": "default",
        "typeDescriptions": {
          "typeIdentifier": "t_uint64",
          "typeString": "uint64"
        },
        "typeName": {
          "id": {{index .RandomIDs 7}},
          "name": "uint64",
          "nodeType": "ElementaryTypeName",
          "src": "-1:-1:-1",
          "typeDescriptions": {
            "typeIdentifier": "t_uint64",
            "typeString": "uint64"
          }
        },
        "visibility": "internal"
      }
    ],
    "src": "-1:-1:-1"
  },
  "src": "-1:-1:-1"
}`))

var coverageMarkerTemplate = template.Must(template.New("coverageMarker").Parse(`{
  "eventCall": {
    "arguments": [
      {
        "hexValue": "{{index .EventCallArgHexValue 0}}",
        "id": {{index .RandomIDs 0}},
        "isConstant": false,
        "isLValue": false,
        "isPure": true,
        "kind": "number",
        "lValueRequested": false,
        "nodeType": "Literal",
        "src": "-1:-1:-1",
        "typeDescriptions": {
          "typeIdentifier": "t_rational_{{index .EventCallArgValue 0}}_by_1",
          "typeString": "int_const {{index .EventCallArgValue 0}}"
        },
        "value": "{{index .EventCallArgValue 0}}"
      },
      {
        "hexValue": "{{index .EventCallArgHexValue 1}}",
        "id": {{index .RandomIDs 1}},
        "isConstant": false,
        "isLValue": false,
        "isPure": true,
        "kind": "number",
        "lValueRequested": false,
        "nodeType": "Literal",
        "src": "-1:-1:-1",
        "typeDescriptions": {
          "typeIdentifier": "t_rational_{{index .EventCallArgValue 1}}_by_1",
          "typeString": "int_const {{index .EventCallArgValue 1}}"
        },
        "value": "{{index .EventCallArgValue 1}}"
      },
      {
        "hexValue": "{{index .EventCallArgHexValue 2}}",
        "id": {{index .RandomIDs 2}},
        "isConstant": false,
        "isLValue": false,
        "isPure": true,
        "kind": "number",
        "lValueRequested": false,
        "nodeType": "Literal",
        "src": "-1:-1:-1",
        "typeDescriptions": {
          "typeIdentifier": "t_rational_{{index .EventCallArgValue 2}}_by_1",
          "typeString": "int_const {{index .EventCallArgValue 2}}"
        },
        "value": "{{index .EventCallArgValue 2}}"
      }
    ],
    "expression": {
      "argumentTypes": [
        {
          "typeIdentifier": "t_rational_{{index .EventCallArgValue 0}}_by_1",
          "typeString": "int_const {{index .EventCallArgValue 0}}"
        },
        {
          "typeIdentifier": "t_rational_{{index .EventCallArgValue 1}}_by_1",
          "typeString": "int_const {{index .EventCallArgValue 1}}"
        },
        {
          "typeIdentifier": "t_rational_{{index .EventCallArgValue 2}}_by_1",
          "typeString": "int_const {{index .EventCallArgValue 2}}"
        }
      ],
      "id": {{index .RandomIDs 3}},
      "name": "___coverage_{{.EventDefinitionID}}",
      "nodeType": "Identifier",
      "overloadedDeclarations": [],
      "referencedDeclaration": {{.EventDefinitionID}},
      "src": "-1:-1:-1",
      "typeDescriptions": {
            "typeIdentifier": "t_function_event_nonpayable$_t_uint64_$_t_uint64_$_t_uint64_$returns$__$",
            "typeString": "function (uint64,uint64,uint64)"
      }
    },
    "id": {{index .RandomIDs 4}},
    "isConstant": false,
    "isLValue": false,
    "isPure": false,
    "kind": "functionCall",
    "lValueRequested": false,
    "names": [],
    "nodeType": "FunctionCall",
    "src": "-1:-1:-1",
    "tryCall": false,
    "typeDescriptions": {
      "typeIdentifier": "t_tuple$__$",
      "typeString": "tuple()"
    }
  },
  "id": {{index .RandomIDs 5}},
  "nodeType": "EmitStatement",
  "src": "-1:-1:-1"
}`))

var coverageEventIDTemplate = template.Must(template.New("coverageEventID").Parse(`{
  "constant": true,
  "id": {{index .RandomIDs 0}},
  "mutability": "constant",
  "name": "___coverage_id_{{.ContractName}}",
  "nameLocation": "-1:-1:-1",
  "nodeType": "VariableDeclaration",
  "scope": {{index .RandomIDs 1}},
  "src": "-1:-1:-1",
  "stateVariable": true,
  "storageLocation": "default",
  "typeDescriptions": {
    "typeIdentifier": "t_uint64",
    "typeString": "uint64"
  },
  "typeName": {
    "id": {{index .RandomIDs 2}},
    "name": "uint64",
    "nodeType": "ElementaryTypeName",
    "src": "-1:-1:-1",
    "typeDescriptions": {
      "typeIdentifier": "t_uint64",
      "typeString": "uint64"
    }
  },
  "value": {
    "hexValue": "{{.EventDefinitionIDHex}}",
    "id": {{index .RandomIDs 3}},
    "isConstant": false,
    "isLValue": false,
    "isPure": true,
    "kind": "number",
    "lValueRequested": false,
    "nodeType": "Literal",
    "src": "-1:-1:-1",
    "typeDescriptions": {
      "typeIdentifier": "t_rational_{{.EventDefinitionID}}_by_1",
      "typeString": "int_const {{.EventDefinitionID}}"
    },
    "value": "{{.EventDefinitionID}}"
  },
  "visibility": "public"
}
`))
