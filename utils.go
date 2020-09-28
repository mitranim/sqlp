package sqlp

import "strconv"

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

func prefixDigits(runes []rune) []rune {
	for i, char := range runes {
		if !isCharIn(charMapDigitsDecimal, char) {
			return runes[:i]
		}
	}
	if len(runes) > 0 {
		return runes
	}
	return nil
}

func prefixIdent(runes []rune) []rune {
	for i, char := range runes {
		if i == 0 {
			if !isCharIn(charMapIdentifierStart, char) {
				return nil
			}
		} else {
			if !isCharIn(charMapIdentifier, char) {
				return runes[:i]
			}
		}
	}
	if len(runes) > 0 {
		return runes
	}
	return nil
}

func runesHavePrefix(runes []rune, prefix string) bool {
	for i, char := range prefix {
		if i < len(runes) && char == runes[i] {
			continue
		}
		return false
	}
	return true
}

func mustParseUint64(str string) uint64 {
	num, err := strconv.ParseUint(str, 10, 64)
	if err != nil {
		panic(err)
	}
	return num
}

var charMapIdentifierStart = charMap(`ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz_`)
var charMapIdentifier = charMap(`ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz_0123456789`)
var charMapDigitsDecimal = charMap(`0123456789`)

func isCharIn(chars []bool, char rune) bool {
	index := int(char)
	return index < len(chars) && chars[index]
}

func charMap(str string) []bool {
	var max int
	for _, char := range str {
		if int(char) > max {
			max = int(char)
		}
	}

	charMap := make([]bool, max+1)
	for _, char := range str {
		charMap[int(char)] = true
	}
	return charMap
}

func maybeAppendText(nodes *Nodes, node NodeText) {
	if len(node) > 0 {
		*nodes = append(*nodes, node)
	}
}

func maybeAppendNode(nodes *Nodes, node Node) {
	if node != nil {
		*nodes = append(*nodes, node)
	}
}
