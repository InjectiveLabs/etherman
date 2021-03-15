package main

import (
	"encoding/hex"
	"fmt"
	"math/big"
	"strconv"
	"strings"

	"github.com/pkg/errors"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
)

func mapStringArgs(inputs abi.Arguments, args []string) ([]interface{}, error) {
	if len(inputs) != len(args) {
		err := errors.Errorf("wrong args count, expected %d but got %d", len(inputs), len(args))
		return nil, err
	} else if len(args) == 0 {
		return nil, nil
	}

	out := make([]interface{}, len(inputs))

	for idx, input := range inputs {
		output, err := mapInput(idx, 0, args[idx], input.Type, input.Name)
		if err != nil {
			return nil, err
		}

		out[idx] = output
	}

	return out, nil
}

func mapInput(idx, level int, arg string, inputType abi.Type, inputName string) (output interface{}, err error) {
	switch inputType.T {
	case abi.IntTy:
		switch inputType.Size {
		case 128, 256:
			i, ok := new(big.Int).SetString(arg, 10)
			if !ok {
				err := errors.Errorf("argument %s (idx %d) type %s failed to parse: %s",
					inputName, idx, inputType.String(), arg)
				return nil, err
			}

			output = i
			return output, nil
		}

		i, err := strconv.ParseInt(arg, 10, inputType.Size)
		if err != nil {
			err := errors.Wrapf(err, "argument %s (idx %d) type %s failed to parse: %s",
				inputName, idx, inputType.String(), arg)
			return nil, err
		}

		switch inputType.Size {
		case 8:
			output = int8(i)
			return output, nil
		case 16:
			output = int16(i)
			return output, nil
		case 32:
			output = int32(i)
			return output, nil
		case 64:
			output = int64(i)
			return output, nil
		default:
			err := errors.Errorf("argument %s (idx %d) type %s has wrong size: %d",
				inputName, idx, inputType.String(), inputType.Size)
			return nil, err
		}

	case abi.UintTy:
		switch inputType.Size {
		case 128, 256:
			i, ok := new(big.Int).SetString(arg, 10)
			if !ok {
				err := errors.Errorf("argument %s (idx %d) type %s failed to parse: %s",
					inputName, idx, inputType.String(), arg)
				return nil, err
			}

			output = i
			return output, nil
		}

		i, err := strconv.ParseUint(arg, 10, inputType.Size)
		if err != nil {
			err := errors.Wrapf(err, "argument %s (idx %d) type %s failed to parse: %s",
				inputName, idx, inputType.String(), arg)
			return nil, err
		}

		switch inputType.Size {
		case 8:
			output = uint8(i)
			return output, nil
		case 16:
			output = uint16(i)
			return output, nil
		case 32:
			output = uint32(i)
			return output, nil
		case 64:
			output = uint64(i)
			return output, nil
		default:
			err := errors.Errorf("argument %s (idx %d) type %s has wrong size: %d",
				inputName, idx, inputType.String(), inputType.Size)
			return nil, err
		}

	case abi.BoolTy:
		output = toBool(arg)
		return output, nil
	case abi.StringTy:
		output = arg
		return output, nil
	case abi.AddressTy:
		output = common.HexToAddress(arg)
		return output, nil
	case abi.BytesTy:
		output = hexToBytes(arg)
		return output, nil
	case abi.FixedBytesTy:
		switch inputType.Size {
		case 1:
			buf := [1]byte{}
			copy(buf[:], hexToBytes(arg))
			output = buf
			return output, nil
		case 2:
			buf := [2]byte{}
			copy(buf[:], hexToBytes(arg))
			output = buf
			return output, nil
		case 3:
			buf := [3]byte{}
			copy(buf[:], hexToBytes(arg))
			output = buf
			return output, nil
		case 4:
			buf := [4]byte{}
			copy(buf[:], hexToBytes(arg))
			output = buf
			return output, nil
		case 5:
			buf := [5]byte{}
			copy(buf[:], hexToBytes(arg))
			output = buf
			return output, nil
		case 6:
			buf := [6]byte{}
			copy(buf[:], hexToBytes(arg))
			output = buf
			return output, nil
		case 7:
			buf := [7]byte{}
			copy(buf[:], hexToBytes(arg))
			output = buf
			return output, nil
		case 8:
			buf := [8]byte{}
			copy(buf[:], hexToBytes(arg))
			output = buf
			return output, nil
		case 9:
			buf := [9]byte{}
			copy(buf[:], hexToBytes(arg))
			output = buf
			return output, nil
		case 10:
			buf := [10]byte{}
			copy(buf[:], hexToBytes(arg))
			output = buf
			return output, nil
		case 16:
			buf := [16]byte{}
			copy(buf[:], hexToBytes(arg))
			output = buf
			return output, nil
		case 20:
			buf := [20]byte{}
			copy(buf[:], hexToBytes(arg))
			output = buf
			return output, nil
		case 32:
			buf := [32]byte{}
			copy(buf[:], hexToBytes(arg))
			output = buf
			return output, nil
		case 40:
			buf := [40]byte{}
			copy(buf[:], hexToBytes(arg))
			output = buf
			return output, nil
		case 64:
			buf := [64]byte{}
			copy(buf[:], hexToBytes(arg))
			output = buf
			return output, nil
		case 128:
			buf := [128]byte{}
			copy(buf[:], hexToBytes(arg))
			output = buf
			return output, nil
		case 256:
			buf := [256]byte{}
			copy(buf[:], hexToBytes(arg))
			output = buf
			return output, nil
		case 512:
			buf := [512]byte{}
			copy(buf[:], hexToBytes(arg))
			output = buf
			return output, nil
		case 1024:
			buf := [1024]byte{}
			copy(buf[:], hexToBytes(arg))
			output = buf
			return output, nil
		default:
			err := errors.Errorf("argument %s (idx %d) has fixed array size: %d", inputName, idx, inputType.Size)
			return nil, err
		}
	case abi.ArrayTy, abi.SliceTy:
		if level > 0 {
			err := errors.Errorf("wrong argument %s (idx %d) - nested arrays unsupported", inputName, idx)
			return nil, err
		}
		elems := strings.Split(arg, ",")

		switch inputType.Elem.T {
		case abi.IntTy:
			switch inputType.Elem.Size {
			case 128, 256:
				elemOuts := make([]*big.Int, len(elems))
				for elemIdx, elem := range elems {
					elemOut, err := mapInput(idx, level+1, elem, *inputType.Elem, fmt.Sprintf("%s[%d]", inputName, elemIdx))
					if err != nil {
						err = errors.Wrap(err, "failed to parse array of elements")
						return nil, err
					}

					elemOuts[elemIdx] = elemOut.(*big.Int)
				}
				output = elemOuts
				return output, nil
			}

			switch inputType.Elem.Size {
			case 8:
				elemOuts := make([]int8, len(elems))
				for elemIdx, elem := range elems {
					elemOut, err := mapInput(idx, level+1, elem, *inputType.Elem, fmt.Sprintf("%s[%d]", inputName, elemIdx))
					if err != nil {
						err = errors.Wrap(err, "failed to parse array of elements")
						return nil, err
					}

					elemOuts[elemIdx] = elemOut.(int8)
				}
				output = elemOuts
				return output, nil
			case 16:
				elemOuts := make([]int16, len(elems))
				for elemIdx, elem := range elems {
					elemOut, err := mapInput(idx, level+1, elem, *inputType.Elem, fmt.Sprintf("%s[%d]", inputName, elemIdx))
					if err != nil {
						err = errors.Wrap(err, "failed to parse array of elements")
						return nil, err
					}

					elemOuts[elemIdx] = elemOut.(int16)
				}
				output = elemOuts
				return output, nil
			case 32:
				elemOuts := make([]int32, len(elems))
				for elemIdx, elem := range elems {
					elemOut, err := mapInput(idx, level+1, elem, *inputType.Elem, fmt.Sprintf("%s[%d]", inputName, elemIdx))
					if err != nil {
						err = errors.Wrap(err, "failed to parse array of elements")
						return nil, err
					}

					elemOuts[elemIdx] = elemOut.(int32)
				}
				output = elemOuts
				return output, nil
			case 64:
				elemOuts := make([]int64, len(elems))
				for elemIdx, elem := range elems {
					elemOut, err := mapInput(idx, level+1, elem, *inputType.Elem, fmt.Sprintf("%s[%d]", inputName, elemIdx))
					if err != nil {
						err = errors.Wrap(err, "failed to parse array of elements")
						return nil, err
					}

					elemOuts[elemIdx] = elemOut.(int64)
				}
				output = elemOuts
				return output, nil
			default:
				err := errors.Errorf("argument %s (idx %d) type %s has wrong size: %d",
					inputName, idx, inputType.String(), inputType.Elem.Size)
				return nil, err
			}

		case abi.UintTy:
			switch inputType.Elem.Size {
			case 128, 256:
				elemOuts := make([]*big.Int, len(elems))
				for elemIdx, elem := range elems {
					elemOut, err := mapInput(idx, level+1, elem, *inputType.Elem, fmt.Sprintf("%s[%d]", inputName, elemIdx))
					if err != nil {
						err = errors.Wrap(err, "failed to parse array of elements")
						return nil, err
					}

					elemOuts[elemIdx] = elemOut.(*big.Int)
				}
				output = elemOuts
				return output, nil
			}

			switch inputType.Elem.Size {
			case 8:
				elemOuts := make([]uint8, len(elems))
				for elemIdx, elem := range elems {
					elemOut, err := mapInput(idx, level+1, elem, *inputType.Elem, fmt.Sprintf("%s[%d]", inputName, elemIdx))
					if err != nil {
						err = errors.Wrap(err, "failed to parse array of elements")
						return nil, err
					}

					elemOuts[elemIdx] = elemOut.(uint8)
				}
				output = elemOuts
				return output, nil
			case 16:
				elemOuts := make([]uint16, len(elems))
				for elemIdx, elem := range elems {
					elemOut, err := mapInput(idx, level+1, elem, *inputType.Elem, fmt.Sprintf("%s[%d]", inputName, elemIdx))
					if err != nil {
						err = errors.Wrap(err, "failed to parse array of elements")
						return nil, err
					}

					elemOuts[elemIdx] = elemOut.(uint16)
				}
				output = elemOuts
				return output, nil
			case 32:
				elemOuts := make([]uint32, len(elems))
				for elemIdx, elem := range elems {
					elemOut, err := mapInput(idx, level+1, elem, *inputType.Elem, fmt.Sprintf("%s[%d]", inputName, elemIdx))
					if err != nil {
						err = errors.Wrap(err, "failed to parse array of elements")
						return nil, err
					}

					elemOuts[elemIdx] = elemOut.(uint32)
				}
				output = elemOuts
				return output, nil
			case 64:
				elemOuts := make([]uint64, len(elems))
				for elemIdx, elem := range elems {
					elemOut, err := mapInput(idx, level+1, elem, *inputType.Elem, fmt.Sprintf("%s[%d]", inputName, elemIdx))
					if err != nil {
						err = errors.Wrap(err, "failed to parse array of elements")
						return nil, err
					}

					elemOuts[elemIdx] = elemOut.(uint64)
				}
				output = elemOuts
				return output, nil
			default:
				err := errors.Errorf("argument %s (idx %d) type %s has wrong size: %d",
					inputName, idx, inputType.String(), inputType.Elem.Size)
				return nil, err
			}

		case abi.BoolTy:
			elemOuts := make([]bool, len(elems))
			for elemIdx, elem := range elems {
				elemOut, err := mapInput(idx, level+1, elem, *inputType.Elem, fmt.Sprintf("%s[%d]", inputName, elemIdx))
				if err != nil {
					err = errors.Wrap(err, "failed to parse array of elements")
					return nil, err
				}

				elemOuts[elemIdx] = elemOut.(bool)
			}
			output = elemOuts
			return output, nil
		case abi.StringTy:
			elemOuts := make([]string, len(elems))
			for elemIdx, elem := range elems {
				elemOut, err := mapInput(idx, level+1, elem, *inputType.Elem, fmt.Sprintf("%s[%d]", inputName, elemIdx))
				if err != nil {
					err = errors.Wrap(err, "failed to parse array of elements")
					return nil, err
				}

				elemOuts[elemIdx] = elemOut.(string)
			}
			output = elemOuts
			return output, nil
		case abi.AddressTy:
			elemOuts := make([]common.Address, len(elems))
			for elemIdx, elem := range elems {
				elemOut, err := mapInput(idx, level+1, elem, *inputType.Elem, fmt.Sprintf("%s[%d]", inputName, elemIdx))
				if err != nil {
					err = errors.Wrap(err, "failed to parse array of elements")
					return nil, err
				}

				elemOuts[elemIdx] = elemOut.(common.Address)
			}
			output = elemOuts
			return output, nil
		case abi.BytesTy:
			elemOuts := make([][]byte, len(elems))
			for elemIdx, elem := range elems {
				elemOut, err := mapInput(idx, level+1, elem, *inputType.Elem, fmt.Sprintf("%s[%d]", inputName, elemIdx))
				if err != nil {
					err = errors.Wrap(err, "failed to parse array of elements")
					return nil, err
				}

				elemOuts[elemIdx] = elemOut.([]byte)
			}
			output = elemOuts
			return output, nil
		case abi.FixedBytesTy:
			switch inputType.Elem.Size {
			case 1:
				elemOuts := make([][1]byte, len(elems))
				for elemIdx, elem := range elems {
					elemOut, err := mapInput(idx, level+1, elem, *inputType.Elem, fmt.Sprintf("%s[%d]", inputName, elemIdx))
					if err != nil {
						err = errors.Wrap(err, "failed to parse array of elements")
						return nil, err
					}

					elemOuts[elemIdx] = elemOut.([1]byte)
				}
				output = elemOuts
				return output, nil
			case 2:
				elemOuts := make([][2]byte, len(elems))
				for elemIdx, elem := range elems {
					elemOut, err := mapInput(idx, level+1, elem, *inputType.Elem, fmt.Sprintf("%s[%d]", inputName, elemIdx))
					if err != nil {
						err = errors.Wrap(err, "failed to parse array of elements")
						return nil, err
					}

					elemOuts[elemIdx] = elemOut.([2]byte)
				}
				output = elemOuts
				return output, nil
			case 3:
				elemOuts := make([][3]byte, len(elems))
				for elemIdx, elem := range elems {
					elemOut, err := mapInput(idx, level+1, elem, *inputType.Elem, fmt.Sprintf("%s[%d]", inputName, elemIdx))
					if err != nil {
						err = errors.Wrap(err, "failed to parse array of elements")
						return nil, err
					}

					elemOuts[elemIdx] = elemOut.([3]byte)
				}
				output = elemOuts
				return output, nil
			case 4:
				elemOuts := make([][4]byte, len(elems))
				for elemIdx, elem := range elems {
					elemOut, err := mapInput(idx, level+1, elem, *inputType.Elem, fmt.Sprintf("%s[%d]", inputName, elemIdx))
					if err != nil {
						err = errors.Wrap(err, "failed to parse array of elements")
						return nil, err
					}

					elemOuts[elemIdx] = elemOut.([4]byte)
				}
				output = elemOuts
				return output, nil
			case 5:
				elemOuts := make([][5]byte, len(elems))
				for elemIdx, elem := range elems {
					elemOut, err := mapInput(idx, level+1, elem, *inputType.Elem, fmt.Sprintf("%s[%d]", inputName, elemIdx))
					if err != nil {
						err = errors.Wrap(err, "failed to parse array of elements")
						return nil, err
					}

					elemOuts[elemIdx] = elemOut.([5]byte)
				}
				output = elemOuts
				return output, nil
			case 6:
				elemOuts := make([][6]byte, len(elems))
				for elemIdx, elem := range elems {
					elemOut, err := mapInput(idx, level+1, elem, *inputType.Elem, fmt.Sprintf("%s[%d]", inputName, elemIdx))
					if err != nil {
						err = errors.Wrap(err, "failed to parse array of elements")
						return nil, err
					}

					elemOuts[elemIdx] = elemOut.([6]byte)
				}
				output = elemOuts
				return output, nil
			case 7:
				elemOuts := make([][7]byte, len(elems))
				for elemIdx, elem := range elems {
					elemOut, err := mapInput(idx, level+1, elem, *inputType.Elem, fmt.Sprintf("%s[%d]", inputName, elemIdx))
					if err != nil {
						err = errors.Wrap(err, "failed to parse array of elements")
						return nil, err
					}

					elemOuts[elemIdx] = elemOut.([7]byte)
				}
				output = elemOuts
				return output, nil
			case 8:
				elemOuts := make([][8]byte, len(elems))
				for elemIdx, elem := range elems {
					elemOut, err := mapInput(idx, level+1, elem, *inputType.Elem, fmt.Sprintf("%s[%d]", inputName, elemIdx))
					if err != nil {
						err = errors.Wrap(err, "failed to parse array of elements")
						return nil, err
					}

					elemOuts[elemIdx] = elemOut.([8]byte)
				}
				output = elemOuts
				return output, nil
			case 9:
				elemOuts := make([][9]byte, len(elems))
				for elemIdx, elem := range elems {
					elemOut, err := mapInput(idx, level+1, elem, *inputType.Elem, fmt.Sprintf("%s[%d]", inputName, elemIdx))
					if err != nil {
						err = errors.Wrap(err, "failed to parse array of elements")
						return nil, err
					}

					elemOuts[elemIdx] = elemOut.([9]byte)
				}
				output = elemOuts
				return output, nil
			case 10:
				elemOuts := make([][10]byte, len(elems))
				for elemIdx, elem := range elems {
					elemOut, err := mapInput(idx, level+1, elem, *inputType.Elem, fmt.Sprintf("%s[%d]", inputName, elemIdx))
					if err != nil {
						err = errors.Wrap(err, "failed to parse array of elements")
						return nil, err
					}

					elemOuts[elemIdx] = elemOut.([10]byte)
				}
				output = elemOuts
				return output, nil
			case 16:
				elemOuts := make([][16]byte, len(elems))
				for elemIdx, elem := range elems {
					elemOut, err := mapInput(idx, level+1, elem, *inputType.Elem, fmt.Sprintf("%s[%d]", inputName, elemIdx))
					if err != nil {
						err = errors.Wrap(err, "failed to parse array of elements")
						return nil, err
					}

					elemOuts[elemIdx] = elemOut.([16]byte)
				}
				output = elemOuts
				return output, nil
			case 20:
				elemOuts := make([][20]byte, len(elems))
				for elemIdx, elem := range elems {
					elemOut, err := mapInput(idx, level+1, elem, *inputType.Elem, fmt.Sprintf("%s[%d]", inputName, elemIdx))
					if err != nil {
						err = errors.Wrap(err, "failed to parse array of elements")
						return nil, err
					}

					elemOuts[elemIdx] = elemOut.([20]byte)
				}
				output = elemOuts
				return output, nil
			case 32:
				elemOuts := make([][32]byte, len(elems))
				for elemIdx, elem := range elems {
					elemOut, err := mapInput(idx, level+1, elem, *inputType.Elem, fmt.Sprintf("%s[%d]", inputName, elemIdx))
					if err != nil {
						err = errors.Wrap(err, "failed to parse array of elements")
						return nil, err
					}

					elemOuts[elemIdx] = elemOut.([32]byte)
				}
				output = elemOuts
				return output, nil
			case 40:
				elemOuts := make([][40]byte, len(elems))
				for elemIdx, elem := range elems {
					elemOut, err := mapInput(idx, level+1, elem, *inputType.Elem, fmt.Sprintf("%s[%d]", inputName, elemIdx))
					if err != nil {
						err = errors.Wrap(err, "failed to parse array of elements")
						return nil, err
					}

					elemOuts[elemIdx] = elemOut.([40]byte)
				}
				output = elemOuts
				return output, nil
			case 64:
				elemOuts := make([][64]byte, len(elems))
				for elemIdx, elem := range elems {
					elemOut, err := mapInput(idx, level+1, elem, *inputType.Elem, fmt.Sprintf("%s[%d]", inputName, elemIdx))
					if err != nil {
						err = errors.Wrap(err, "failed to parse array of elements")
						return nil, err
					}

					elemOuts[elemIdx] = elemOut.([64]byte)
				}
				output = elemOuts
				return output, nil
			case 128:
				elemOuts := make([][128]byte, len(elems))
				for elemIdx, elem := range elems {
					elemOut, err := mapInput(idx, level+1, elem, *inputType.Elem, fmt.Sprintf("%s[%d]", inputName, elemIdx))
					if err != nil {
						err = errors.Wrap(err, "failed to parse array of elements")
						return nil, err
					}

					elemOuts[elemIdx] = elemOut.([128]byte)
				}
				output = elemOuts
				return output, nil
			case 256:
				elemOuts := make([][256]byte, len(elems))
				for elemIdx, elem := range elems {
					elemOut, err := mapInput(idx, level+1, elem, *inputType.Elem, fmt.Sprintf("%s[%d]", inputName, elemIdx))
					if err != nil {
						err = errors.Wrap(err, "failed to parse array of elements")
						return nil, err
					}

					elemOuts[elemIdx] = elemOut.([256]byte)
				}
				output = elemOuts
				return output, nil
			case 512:
				elemOuts := make([][512]byte, len(elems))
				for elemIdx, elem := range elems {
					elemOut, err := mapInput(idx, level+1, elem, *inputType.Elem, fmt.Sprintf("%s[%d]", inputName, elemIdx))
					if err != nil {
						err = errors.Wrap(err, "failed to parse array of elements")
						return nil, err
					}

					elemOuts[elemIdx] = elemOut.([512]byte)
				}
				output = elemOuts
				return output, nil
			case 1024:
				elemOuts := make([][1024]byte, len(elems))
				for elemIdx, elem := range elems {
					elemOut, err := mapInput(idx, level+1, elem, *inputType.Elem, fmt.Sprintf("%s[%d]", inputName, elemIdx))
					if err != nil {
						err = errors.Wrap(err, "failed to parse array of elements")
						return nil, err
					}

					elemOuts[elemIdx] = elemOut.([1024]byte)
				}
				output = elemOuts
				return output, nil
			default:
				err := errors.Errorf("argument %s (idx %d) has fixed array size: %d", inputName, idx, inputType.Elem.Size)
				return nil, err
			}
		}
	default:
		err := errors.Errorf("argument %s (idx %d) has unsupported type: %s", inputName, idx, inputType.String())
		return nil, err
	}

	err = errors.Errorf("argument %s (idx %d) has unsupported type: %s", inputName, idx, inputType.String())
	return nil, err
}

func hexToBytes(str string) []byte {
	if strings.HasPrefix(str, "0x") {
		str = str[2:]
	}

	data, err := hex.DecodeString(str)
	if err != nil {
		panic(err)
	}

	return data
}

func toBool(s string) bool {
	switch strings.ToLower(s) {
	case "true":
		return true
	default:
		return false
	}
}
