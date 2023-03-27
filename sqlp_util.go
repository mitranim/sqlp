package sqlp

import (
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"
	"unsafe"
)

const (
	ordinalPrefix      = '$'
	namedPrefix        = ':'
	castPrefix         = `::`
	commentLinePrefix  = `--`
	commentBlockPrefix = `/*`
	commentBlockSuffix = `*/`
	quoteSingle        = '\''
	quoteDouble        = '"'
	quoteGrave         = '`'
	parenOpen          = '('
	parenClose         = ')'
	bracketOpen        = '['
	bracketClose       = ']'
	braceOpen          = '{'
	braceClose         = '}'

	byteLen          = 1
	ordinalPrefixLen = byteLen
	namedPrefixLen   = byteLen
)

var (
	nodeWhitespaceSingle = Node(NodeWhitespace(` `))
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
		if !charsetDigitDec.has(str[i]) {
			return str[:i]
		}
	}
	return str
}

func prefixIdent(str string) string {
	for i := range str {
		if i == 0 {
			if !charsetIdentStart.has(str[i]) {
				return ""
			}
		} else {
			if !charsetIdent.has(str[i]) {
				return str[:i]
			}
		}
	}
	return str
}

func tryParseInt(str string) int64 {
	num, err := strconv.ParseInt(str, 10, 64)
	if err != nil {
		panic(err)
	}
	return num
}

type charset [256]bool

func (self *charset) has(val byte) bool { return self[val] }

func (self *charset) addStr(vals string) *charset {
	for _, val := range vals {
		self[val] = true
	}
	return self
}

func (self *charset) addSet(vals *charset) *charset {
	for i, val := range vals {
		if val {
			self[i] = true
		}
	}
	return self
}

var (
	charsetDigitDec   = new(charset).addStr(`0123456789`)
	charsetIdentStart = new(charset).addStr(`ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz_`)
	charsetIdent      = new(charset).addSet(charsetIdentStart).addSet(charsetDigitDec)
	charsetSpace      = new(charset).addStr(" \t\v")
	charsetNewline    = new(charset).addStr("\r\n")
	charsetWhitespace = new(charset).addSet(charsetSpace).addSet(charsetNewline)
)

func appenderStr(val interface{ AppendTo([]byte) []byte }) string {
	return bytesToMutableString(val.AppendTo(nil))
}

func appendNodesEnclosed(buf []byte, prefix byte, nodes Nodes, suffix byte) []byte {
	buf = append(buf, prefix)
	buf = nodes.AppendTo(buf)
	buf = append(buf, suffix)
	return buf
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

func headChar(str string) (rune, int) {
	return utf8.DecodeRuneInString(str)
}

func tryTrimPrefixByte(val string, prefix byte) string {
	if !(len(val) >= byteLen && val[0] == prefix) {
		panic(fmt.Errorf(`[sqlp] expected %q to begin with %q`, val, rune(prefix)))
	}
	return val[byteLen:]
}

func tryTrimPrefix(val, prefix string) string {
	if !strings.HasPrefix(val, prefix) {
		panic(fmt.Errorf(`[sqlp] expected %q to begin with %q`, val, prefix))
	}
	return val[len(prefix):]
}

func tryTrimPrefixSuffixByte(val string, prefix, suffix byte) string {
	end := len(val) - 1
	if !(len(val) >= (byteLen*2) && val[0] == prefix && val[end] == suffix) {
		panic(fmt.Errorf(`[sqlp] expected %q to begin with %q and end with %q`, val, rune(prefix), rune(suffix)))
	}
	return val[byteLen:end]
}

func tryTrimPrefixSuffix(val, prefix, suffix string) string {
	prefixLen := len(prefix)
	suffixLen := len(suffix)
	if !(len(val) >= (prefixLen+suffixLen) && strings.HasPrefix(val, prefix) && strings.HasSuffix(val, suffix)) {
		panic(fmt.Errorf(`[sqlp] expected %q to begin with %q and end with %q`, val, prefix, suffix))
	}
	return val[prefixLen : len(val)-suffixLen]
}

func reqStrEq(val, exp string) {
	if val == exp {
		return
	}
	panic(fmt.Errorf(`[sqlp] expected token %q, found %q`, exp, val))
}
