// Copyright 2026 Glenn Lewis. All rights reserved.
//
// Use of this source code is governed by the Reticulum License
// that can be found in the LICENSE file.

package network

import (
	"encoding/binary"
	"errors"
	"math"
)

var ErrInvalidData = errors.New("invalid data format")

// PackData serializes a map[byte]any into a simple binary format
// compatible with the Python LXST msgpack format.
// This is a simplified msgpack encoder for the specific data types used by LXST.
func PackData(data map[byte]any) ([]byte, error) {
	if len(data) == 0 {
		return nil, nil
	}
	if len(data) > 15 {
		return nil, ErrInvalidData
	}

	result := []byte{byte(0x80 | len(data))}
	for k, v := range data {
		result = append(result, k)
		encoded, err := encodeValue(v)
		if err != nil {
			return nil, err
		}
		result = append(result, encoded...)
	}
	return result, nil
}

func encodeValue(v any) ([]byte, error) {
	switch val := v.(type) {
	case nil:
		return []byte{0xc0}, nil
	case []byte:
		if len(val) <= 255 {
			result := []byte{0xc4, byte(len(val))}
			result = append(result, val...)
			return result, nil
		}
		result := make([]byte, 5+len(val))
		result[0] = 0xc5
		binary.BigEndian.PutUint32(result[1:], uint32(len(val)))
		copy(result[5:], val)
		return result, nil
	case []any:
		if len(val) > 15 {
			return nil, ErrInvalidData
		}
		result := []byte{byte(0x90 | len(val))}
		for _, item := range val {
			encoded, err := encodeValue(item)
			if err != nil {
				return nil, err
			}
			result = append(result, encoded...)
		}
		return result, nil
	case byte:
		return encodeInt(int(val)), nil
	case int:
		return encodeInt(val), nil
	case int8:
		return encodeInt(int(val)), nil
	case int16:
		return encodeInt(int(val)), nil
	case int32:
		return encodeInt(int(val)), nil
	case int64:
		return encodeInt(int(val)), nil
	case uint16:
		return encodeInt(int(val)), nil
	case uint32:
		return encodeInt(int(val)), nil
	case uint64:
		return encodeInt(int(val)), nil
	case bool:
		if val {
			return []byte{0xc3}, nil
		}
		return []byte{0xc2}, nil
	case string:
		b := []byte(val)
		if len(b) <= 31 {
			return append([]byte{byte(0xa0 | len(b))}, b...), nil
		}
		result := []byte{0xd9, byte(len(b))}
		result = append(result, b...)
		return result, nil
	default:
		return []byte{0xc0}, nil
	}
}

// encodeInt encodes an integer using the smallest msgpack representation,
// matching Python umsgpack's encoding for all integer sizes.
func encodeInt(val int) []byte {
	if val >= 0 && val <= 127 {
		return []byte{byte(val)}
	}
	if val >= 0 && val <= 255 {
		return []byte{0xcc, byte(val)}
	}
	if val >= 0 && val <= 65535 {
		result := make([]byte, 3)
		result[0] = 0xcd
		binary.BigEndian.PutUint16(result[1:], uint16(val))
		return result
	}
	if val >= 0 && val <= 4294967295 {
		result := make([]byte, 5)
		result[0] = 0xce
		binary.BigEndian.PutUint32(result[1:], uint32(val))
		return result
	}
	if val >= 0 {
		result := make([]byte, 9)
		result[0] = 0xcf
		binary.BigEndian.PutUint64(result[1:], uint64(val))
		return result
	}
	if val >= -32 && val < 0 {
		return []byte{byte(val)}
	}
	if val >= -128 && val < 0 {
		return []byte{0xd0, byte(val)}
	}
	if val >= -32768 && val < 0 {
		result := make([]byte, 3)
		result[0] = 0xd1
		binary.BigEndian.PutUint16(result[1:], uint16(val))
		return result
	}
	if val >= -2147483648 && val < 0 {
		result := make([]byte, 5)
		result[0] = 0xd2
		binary.BigEndian.PutUint32(result[1:], uint32(val))
		return result
	}
	result := make([]byte, 9)
	result[0] = 0xd3
	binary.BigEndian.PutUint64(result[1:], uint64(val))
	return result
}

// UnpackData deserializes data packed by PackData.
func UnpackData(data []byte) (any, error) {
	if len(data) == 0 {
		return nil, ErrInvalidData
	}

	b := data[0]

	// fixmap
	if b >= 0x80 && b <= 0x8f {
		mapLen := int(b & 0x0f)
		result := make(map[byte]any)
		offset := 1
		for i := 0; i < mapLen; i++ {
			if offset >= len(data) {
				return nil, ErrInvalidData
			}
			key := data[offset]
			offset++
			val, consumed, err := decodeValue(data[offset:])
			if err != nil {
				return nil, err
			}
			offset += consumed
			result[key] = val
		}
		return result, nil
	}

	return nil, ErrInvalidData
}

func decodeValue(data []byte) (any, int, error) {
	if len(data) == 0 {
		return nil, 0, ErrInvalidData
	}

	b := data[0]

	// nil
	if b == 0xc0 {
		return nil, 1, nil
	}

	// false
	if b == 0xc2 {
		return false, 1, nil
	}

	// true
	if b == 0xc3 {
		return true, 1, nil
	}

	// positive fixint
	if b <= 0x7f {
		return int(b), 1, nil
	}

	// negative fixint
	if b >= 0xe0 {
		return int(int8(b)), 1, nil
	}

	// bin8
	if b == 0xc4 {
		if len(data) < 2 {
			return nil, 0, ErrInvalidData
		}
		length := int(data[1])
		if len(data) < 2+length {
			return nil, 0, ErrInvalidData
		}
		result := make([]byte, length)
		copy(result, data[2:2+length])
		return result, 2 + length, nil
	}

	// bin16
	if b == 0xc5 {
		if len(data) < 3 {
			return nil, 0, ErrInvalidData
		}
		length := int(binary.BigEndian.Uint16(data[1:3]))
		if len(data) < 3+length {
			return nil, 0, ErrInvalidData
		}
		result := make([]byte, length)
		copy(result, data[3:3+length])
		return result, 3 + length, nil
	}

	// bin32
	if b == 0xc6 {
		if len(data) < 5 {
			return nil, 0, ErrInvalidData
		}
		length := int(binary.BigEndian.Uint32(data[1:5]))
		if len(data) < 5+length {
			return nil, 0, ErrInvalidData
		}
		result := make([]byte, length)
		copy(result, data[5:5+length])
		return result, 5 + length, nil
	}

	// fixarray
	if b >= 0x90 && b <= 0x9f {
		arrLen := int(b & 0x0f)
		result := make([]any, arrLen)
		offset := 1
		for i := 0; i < arrLen; i++ {
			val, consumed, err := decodeValue(data[offset:])
			if err != nil {
				return nil, 0, err
			}
			result[i] = val
			offset += consumed
		}
		return result, offset, nil
	}

	// fixstr
	if b >= 0xa0 && b <= 0xbf {
		strLen := int(b & 0x1f)
		if len(data) < 1+strLen {
			return nil, 0, ErrInvalidData
		}
		return string(data[1 : 1+strLen]), 1 + strLen, nil
	}

	// uint8
	if b == 0xcc {
		if len(data) < 2 {
			return nil, 0, ErrInvalidData
		}
		return int(data[1]), 2, nil
	}

	// uint16
	if b == 0xcd {
		if len(data) < 3 {
			return nil, 0, ErrInvalidData
		}
		return int(binary.BigEndian.Uint16(data[1:3])), 3, nil
	}

	// uint32
	if b == 0xce {
		if len(data) < 5 {
			return nil, 0, ErrInvalidData
		}
		return int(binary.BigEndian.Uint32(data[1:5])), 5, nil
	}

	// uint64
	if b == 0xcf {
		if len(data) < 9 {
			return nil, 0, ErrInvalidData
		}
		return int(binary.BigEndian.Uint64(data[1:9])), 9, nil
	}

	// int8
	if b == 0xd0 {
		if len(data) < 2 {
			return nil, 0, ErrInvalidData
		}
		return int(int8(data[1])), 2, nil
	}

	// int16
	if b == 0xd1 {
		if len(data) < 3 {
			return nil, 0, ErrInvalidData
		}
		return int(int16(binary.BigEndian.Uint16(data[1:3]))), 3, nil
	}

	// int32
	if b == 0xd2 {
		if len(data) < 5 {
			return nil, 0, ErrInvalidData
		}
		return int(int32(binary.BigEndian.Uint32(data[1:5]))), 5, nil
	}

	// int64
	if b == 0xd3 {
		if len(data) < 9 {
			return nil, 0, ErrInvalidData
		}
		return int(int64(binary.BigEndian.Uint64(data[1:9]))), 9, nil
	}

	// str8
	if b == 0xd9 {
		if len(data) < 2 {
			return nil, 0, ErrInvalidData
		}
		strLen := int(data[1])
		if len(data) < 2+strLen {
			return nil, 0, ErrInvalidData
		}
		return string(data[2 : 2+strLen]), 2 + strLen, nil
	}

	// float32
	if b == 0xca {
		if len(data) < 5 {
			return nil, 0, ErrInvalidData
		}
		bits := binary.BigEndian.Uint32(data[1:5])
		return float32FromBits(bits), 5, nil
	}

	// float64
	if b == 0xcb {
		if len(data) < 9 {
			return nil, 0, ErrInvalidData
		}
		bits := binary.BigEndian.Uint64(data[1:9])
		return float64FromBits(bits), 9, nil
	}

	return nil, 0, ErrInvalidData
}

func float32FromBits(bits uint32) float64 {
	return float64(math.Float32frombits(bits))
}

func float64FromBits(bits uint64) float64 {
	return math.Float64frombits(bits)
}
