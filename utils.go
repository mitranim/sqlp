package sqlp

import (
	"strconv"
	"unsafe"
)

func rec(ptr *error) {
	val := recover()
	if val == nil {
		return
	}

	recErr, ok := val.(error)
	if ok {
		*ptr = recErr
		return
	}

	panic(val)
}

func prefixDigits(str string) string {
	for i := range str {
		if !isByteIn(byteMapDigitsDecimal, str[i]) {
			return str[:i]
		}
	}
	return str
}

func prefixIdent(str string) string {
	for i := range str {
		if i == 0 {
			if !isByteIn(byteMapIdentifierStart, str[i]) {
				return ""
			}
		} else {
			if !isByteIn(byteMapIdentifier, str[i]) {
				return str[:i]
			}
		}
	}
	return str
}

func mustParseNumber(str string) int64 {
	num, err := strconv.ParseInt(str, 10, 64)
	if err != nil {
		panic(err)
	}
	return num
}

var byteMapIdentifierStart = byteMap(`ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz_`)
var byteMapIdentifier = byteMap(`ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz_0123456789`)
var byteMapDigitsDecimal = byteMap(`0123456789`)
var byteMapWhitespace = byteMap(" \n\r\t\v")

func isByteIn(chars []bool, char byte) bool {
	index := int(char)
	return index < len(chars) && chars[index]
}

func byteMap(str string) []bool {
	var max int
	for _, char := range str {
		if int(char) > max {
			max = int(char)
		}
	}

	byteMap := make([]bool, max+1)
	for _, char := range str {
		byteMap[int(char)] = true
	}
	return byteMap
}

func appendStr(buf *[]byte, str string) {
	*buf = append(*buf, str...)
}

func appendByte(buf *[]byte, char byte) {
	*buf = append(*buf, char)
}

func appenderToStr(val interface{ Append(*[]byte) }) string {
	var buf []byte
	val.Append(&buf)
	return bytesToMutableString(buf)
}

func appendEnclosed(buf *[]byte, prefix string, nodes Nodes, suffix string) {
	appendStr(buf, prefix)
	nodes.Append(buf)
	appendStr(buf, suffix)
}

/*
Allocation-free conversion. Reinterprets a byte slice as a string. Borrowed from
the standard library. Reasonably safe. Should not be used when the underlying
byte array is volatile, for example when it's part of a scratch buffer during
SQL scanning.
*/
func bytesToMutableString(bytes []byte) string {
	return *(*string)(unsafe.Pointer(&bytes))
}
