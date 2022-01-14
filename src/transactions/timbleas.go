package transactions

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

func FromASM(asmIn string, version uint8) ([]byte, error) {
	lookUpOp := ASM_TO_OPCODES["CHAIN_NAME_LOOKUP"]
	var byteCodeHeader []byte
	var ok bool
	if byteCodeHeader, ok = BYTECODE_VERSION[version]; !ok {
		return make([]byte, 0), errors.New(fmt.Sprintf("Cannot handle ASM specification %v", version))
	}
	out := make([]byte, 0)
	out = append(out, byteCodeHeader...)
	for _, chunk := range strings.Split(asmIn, " ") {
		var abyte uint8
		if abyte, ok = ASM_TO_OPCODES[chunk]; ok {
			out = append(out, abyte)
		} else if abyte, ok = REVERSE_CHAIN_TABLE[chunk]; ok {
			out = append(out, lookUpOp)
			out = append(out, abyte)
		} else {
			if len(chunk) >= 2 && chunk[:2] == "0x" {
				lenBytes := uint64(len(chunk) / 2)
				encodedLenStr := strconv.FormatUint(uint64(len(chunk[2:])/2), 16)
				if len(encodedLenStr)%2 != 0 {
					encodedLenStr = fmt.Sprintf("0%s", encodedLenStr)
				}
				encodedLen, err := hex.DecodeString(encodedLenStr)
				if err != nil {
					return make([]byte, 0), err
				}
				if lenBytes < uint64(0xff) {
					out = append(out, ASM_TO_OPCODES["OP_PUSHDATA1"])
				} else if lenBytes < uint64(0xffff) {
					out = append(out, ASM_TO_OPCODES["OP_PUSHDATA2"])
				} else if lenBytes < uint64(0xffffffff) {
					out = append(out, ASM_TO_OPCODES["OP_PUSHDATA4"])
				} else {
					return make([]byte, 0), errors.New(fmt.Sprintf("Cannot compile chunk %v", chunk))
				}
				encodedPayload, err := hex.DecodeString(chunk[2:])
				if err != nil {
					return make([]byte, 0), err
				}
				out = append(out, encodedLen...)
				out = append(out, encodedPayload...)
			} else {
				encoded := []byte(chunk)
				encodedLenStr := strconv.FormatUint(uint64(len(encoded)), 16)
				if len(encodedLenStr)%2 != 0 {
					encodedLenStr = fmt.Sprintf("0%s", encodedLenStr)
				}
				encodedLen, err := hex.DecodeString(encodedLenStr)
				if err != nil {
					return make([]byte, 0), err
				}
				out = append(out, ASM_TO_OPCODES["OP_PUSHSTR"])
				if len(encoded) < 0xff && len(encodedLen) == 1 {
					out = append(out, encodedLen[0])
					out = append(out, encoded...)
				} else {
					return make([]byte, 0), errors.New(fmt.Sprintf("PUSHSTR data is too long: %v / %v bytes > 255", encodedLen, len(encoded)))
				}
			}
		}
	}
	return out, nil
}

func ToASM(bytecode []byte, version uint8) (string, error) {
	var byteCodeHeader []byte
	var ok bool
	if byteCodeHeader, ok = BYTECODE_VERSION[version]; !ok {
		return "", errors.New(fmt.Sprintf("Cannot handle ASM specification %v", version))
	}
	if bytes.Compare(byteCodeHeader, bytecode[:len(byteCodeHeader)]) != 0 {
		return "", errors.New(fmt.Sprintf("Bytecode Header does not match specified version! %v != %v", byteCodeHeader, bytecode[:len(byteCodeHeader)]))
	}
	toProcess := bytecode[len(byteCodeHeader):]
	out := make([]string, 0)
	for len(toProcess) > 0 {
		abyte := toProcess[0]
		var opCode string
		var ok bool
		if opCode, ok = OPCODES_TO_ASM[abyte]; ok {
			toProcess = toProcess[1:]
			length := uint32(0)
			isString := false
			switch {
			case opCode == DATA_OPS[0]: // OP_PUSHDATA1
				length = uint32(toProcess[0])
				toProcess = toProcess[1:]
			case opCode == DATA_OPS[1]: // OP_PUSHDATA2
				length = uint32(binary.BigEndian.Uint16(toProcess[:2]))
				toProcess = toProcess[2:]
			case opCode == DATA_OPS[2]: // OP_PUSHDATA4
				length = binary.BigEndian.Uint32(toProcess[:4])
				toProcess = toProcess[4:]
			case opCode == DATA_OPS[3]: // OP_PUSHSTR
				length = uint32(toProcess[0])
				toProcess = toProcess[1:]
				isString = true
			case opCode == "CHAIN_NAME_LOOKUP":
				chainByte := toProcess[0]
				toProcess = toProcess[1:]
				if chainStr, ok := CHAIN_TABLE[chainByte]; ok {
					out = append(out, chainStr)
				} else {
					return "", errors.New(fmt.Sprintf("Processed byte is not a valid chain label: %v", chainByte))
				}
			default:
				out = append(out, opCode)
			}
			if length > 0 {
				if isString {
					out = append(out, string(toProcess[:length]))
				} else {
					out = append(out, fmt.Sprintf("0x%s", hex.EncodeToString(toProcess[:length])))
				}
				toProcess = toProcess[length:]
			}
		} else {
			return "", errors.New(fmt.Sprintf("Processed byte is not a valid OPCODE: 0x%x", abyte))
		}
	}
	return strings.Join(out, " "), nil
}
