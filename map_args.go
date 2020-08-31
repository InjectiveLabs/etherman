package main

import (
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
		switch input.Type.T {
		case abi.IntTy:
			switch input.Type.Size {
			case 128, 256:
				i, ok := new(big.Int).SetString(args[idx], 10)
				if !ok {
					err := errors.Errorf("argument %s (idx %d) type %s failed to parse: %s",
						input.Name, idx, input.Type.String(), args[idx])
					return nil, err
				}

				out[idx] = i
				continue
			}

			i, err := strconv.ParseInt(args[idx], 10, input.Type.Size)
			if err != nil {
				err := errors.Wrapf(err, "argument %s (idx %d) type %s failed to parse: %s",
					input.Name, idx, input.Type.String(), args[idx])
				return nil, err
			}

			switch input.Type.Size {
			case 8:
				out[idx] = int8(i)
			case 16:
				out[idx] = int16(i)
			case 32:
				out[idx] = int32(i)
			case 64:
				out[idx] = int64(i)
			default:
				err := errors.Errorf("argument %s (idx %d) type %s has wrong size: %d",
					input.Name, idx, input.Type.String(), input.Type.Size)
				return nil, err
			}

		case abi.UintTy:
			switch input.Type.Size {
			case 128, 256:
				i, ok := new(big.Int).SetString(args[idx], 10)
				if !ok {
					err := errors.Errorf("argument %s (idx %d) type %s failed to parse: %s",
						input.Name, idx, input.Type.String(), args[idx])
					return nil, err
				}

				out[idx] = i
				continue
			}

			i, err := strconv.ParseUint(args[idx], 10, input.Type.Size)
			if err != nil {
				err := errors.Wrapf(err, "argument %s (idx %d) type %s failed to parse: %s",
					input.Name, idx, input.Type.String(), args[idx])
				return nil, err
			}

			switch input.Type.Size {
			case 8:
				out[idx] = uint8(i)
			case 16:
				out[idx] = uint16(i)
			case 32:
				out[idx] = uint32(i)
			case 64:
				out[idx] = uint64(i)
			default:
				err := errors.Errorf("argument %s (idx %d) type %s has wrong size: %d",
					input.Name, idx, input.Type.String(), input.Type.Size)
				return nil, err
			}

		case abi.BoolTy:
			out[idx] = toBool(args[idx])
		case abi.StringTy:
			out[idx] = args[idx]
		case abi.AddressTy:
			out[idx] = common.HexToAddress(args[idx])
		case abi.FixedBytesTy, abi.BytesTy:
			out[idx] = common.Hex2Bytes(args[idx])
		default:
			err := errors.Errorf("argument %s (idx %d) has unsupported type: %s", input.Name, idx, input.Type.String())
			return nil, err
		}
	}

	return out, nil
}

func toBool(s string) bool {
	switch strings.ToLower(s) {
	case "true":
		return true
	default:
		return false
	}
}
