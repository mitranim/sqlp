package sqlp

import (
	"strings"
	"unicode/utf8"
)

/*
Incremental parser. Example usage:

	tokenizer := Tokenizer{Source: `select * from some_table where some_col = $1`}

	for {
		node := tokenizer.Next()
		if node == nil {
			break
		}
		fmt.Println(node)
	}

`Tokenizer` emits only primitive tokens. For delimited structures, it emits
opening and closing tokens, such as `NodeParenOpen` and `NodeParenClose`. This
allows `Tokenizer` to be used for algorithms that prefer incremental traversal
over building a full AST.
*/
type Tokenizer struct {
	Source string
	cursor int
}

// Returns the next node if possible, or nil.
func (self *Tokenizer) Next() Node {
	node := self.node()
	if node != nil {
		return node
	}

	if self.more() {
		return self.text()
	}

	return nil
}

func (self *Tokenizer) text() NodeText {
	start := self.cursor
	cursor := start

	for self.more() {
		// Parsing ahead, discarding the result, and rewinding is somewhat wasteful.
		// TODO better approach.
		if self.node() != nil {
			self.cursor = cursor
			break
		}
		self.skipChar()
		cursor = self.cursor
	}

	return NodeText(self.from(start))
}

func (self *Tokenizer) node() Node {
	start := self.cursor
	if node := self.maybeWhitespace(); self.cursor > start {
		return node
	}
	if node := self.maybeQuoteSingle(); self.cursor > start {
		return node
	}
	if node := self.maybeQuoteDouble(); self.cursor > start {
		return node
	}
	if node := self.maybeQuoteGrave(); self.cursor > start {
		return node
	}
	if node := self.maybeCommentLine(); self.cursor > start {
		return node
	}
	if node := self.maybeCommentBlock(); self.cursor > start {
		return node
	}
	if node := self.maybeDoubleColon(); self.cursor > start {
		return node
	}
	if node := self.maybeOrdinalParam(); self.cursor > start {
		return node
	}
	if node := self.maybeNamedParam(); self.cursor > start {
		return node
	}
	if node := self.maybeParenOpen(); self.cursor > start {
		return node
	}
	if node := self.maybeParenClose(); self.cursor > start {
		return node
	}
	if node := self.maybeBracketOpen(); self.cursor > start {
		return node
	}
	if node := self.maybeBracketClose(); self.cursor > start {
		return node
	}
	if node := self.maybeBraceOpen(); self.cursor > start {
		return node
	}
	if node := self.maybeBraceClose(); self.cursor > start {
		return node
	}
	return nil
}

func (self *Tokenizer) maybeWhitespace() NodeWhitespace {
	start := self.cursor
	for self.isNextWhitespace() {
		self.skipByte()
	}
	return NodeWhitespace(self.from(start))
}

func (self *Tokenizer) maybeQuoteSingle() NodeQuoteSingle {
	return NodeQuoteSingle(self.maybeStringBetweenBytes('\'', '\''))
}

func (self *Tokenizer) maybeQuoteDouble() NodeQuoteDouble {
	return NodeQuoteDouble(self.maybeStringBetweenBytes('"', '"'))
}

func (self *Tokenizer) maybeQuoteGrave() NodeQuoteGrave {
	return NodeQuoteGrave(self.maybeStringBetweenBytes('`', '`'))
}

func (self *Tokenizer) maybeCommentLine() NodeCommentLine {
	if !self.skippedString(`--`) {
		return ""
	}

	start := self.cursor
	for self.more() {
		if self.skippedNewline() {
			break
		}
		self.skipChar()
	}

	return NodeCommentLine(self.from(start))
}

func (self *Tokenizer) maybeCommentBlock() NodeCommentBlock {
	return NodeCommentBlock(self.maybeStringBetween(`/*`, `*/`))
}

func (self *Tokenizer) maybeDoubleColon() NodeDoubleColon {
	_ = self.skippedString(`::`)
	return NodeDoubleColon{}
}

func (self *Tokenizer) maybeOrdinalParam() NodeOrdinalParam {
	const prefix = '$'
	const length = 1

	if self.isNextByte(prefix) {
		digits := prefixDigits(self.rest()[length:])
		if len(digits) > 0 {
			self.skipNBytes(length + len(digits))
			return NodeOrdinalParam(mustParseNumber(string(digits)))
		}
	}

	return 0
}

func (self *Tokenizer) maybeNamedParam() NodeNamedParam {
	const prefix = ':'
	const length = 1

	if self.isNextByte(prefix) {
		ident := prefixIdent(self.rest()[length:])
		if len(ident) > 0 {
			self.skipNBytes(length + len(ident))
			return NodeNamedParam(ident)
		}
	}

	return ""
}

func (self *Tokenizer) maybeParenOpen() NodeParenOpen {
	_ = self.skippedByte('(')
	return NodeParenOpen{}
}

func (self *Tokenizer) maybeParenClose() NodeParenClose {
	_ = self.skippedByte(')')
	return NodeParenClose{}
}

func (self *Tokenizer) maybeBracketOpen() NodeBracketOpen {
	_ = self.skippedByte('[')
	return NodeBracketOpen{}
}

func (self *Tokenizer) maybeBracketClose() NodeBracketClose {
	_ = self.skippedByte(']')
	return NodeBracketClose{}
}

func (self *Tokenizer) maybeBraceOpen() NodeBraceOpen {
	_ = self.skippedByte('{')
	return NodeBraceOpen{}
}

func (self *Tokenizer) maybeBraceClose() NodeBraceClose {
	_ = self.skippedByte('}')
	return NodeBraceClose{}
}

func (self *Tokenizer) maybeStringBetween(prefix string, suffix string) string {
	if !self.skippedString(prefix) {
		return ""
	}

	start := self.cursor
	for self.more() {
		if self.isNextString(suffix) {
			chunk := self.from(start)
			self.skipString(suffix)
			return chunk
		}
		self.skipChar()
	}

	/**
	Instead of returning the remaining string, it might be better to detect the
	lack of a closing delimiter, and return an error. If we did that, we could
	also add detection of mismatched delimiters for parens/brackets/braces.
	Unfortunately this would make the interface much more cumbersome. It's
	unclear how much our users want this.
	*/
	return self.from(start)
}

// Faster than `maybeStringBetween`, enough to make a difference in benchmarks.
func (self *Tokenizer) maybeStringBetweenBytes(prefix byte, suffix byte) string {
	if !self.skippedByte(prefix) {
		return ""
	}

	start := self.cursor
	for self.more() {
		if self.isNextByte(suffix) {
			chunk := self.from(start)
			self.skipByte()
			return chunk
		}
		self.skipChar()
	}

	// See note in `maybeStringBetween` about delimiter detection.
	return self.from(start)
}

func (self *Tokenizer) rewind(cursor int) {
	self.cursor = cursor
}

func (self *Tokenizer) more() bool {
	return self.left() > 0
}

func (self *Tokenizer) left() int {
	return len(self.Source) - self.cursor
}

func (self *Tokenizer) headByte() byte {
	if self.cursor < len(self.Source) {
		return self.Source[self.cursor]
	}
	return 0
}

func (self *Tokenizer) from(start int) string {
	if start < 0 {
		start = 0
	}
	if start < self.cursor {
		return self.Source[start:self.cursor]
	}
	return ""
}

func (self *Tokenizer) rest() string {
	if self.more() {
		return self.Source[self.cursor:]
	}
	return ""
}

func (self *Tokenizer) isNextString(prefix string) bool {
	return strings.HasPrefix(self.rest(), prefix)
}

func (self *Tokenizer) isNextByte(char byte) bool {
	return self.headByte() == char
}

func (self *Tokenizer) isNextWhitespace() bool {
	return isByteIn(byteMapWhitespace, self.headByte())
}

func (self *Tokenizer) skipByte() {
	self.cursor++
}

func (self *Tokenizer) skipChar() {
	_, size := utf8.DecodeRuneInString(self.rest())
	self.cursor += size
}

func (self *Tokenizer) skipString(str string) {
	self.skipNBytes(len(str))
}

func (self *Tokenizer) skipNBytes(n int) {
	self.cursor += n
}

func (self *Tokenizer) skippedByte(char byte) bool {
	if self.isNextByte(char) {
		self.skipByte()
		return true
	}
	return false
}

func (self *Tokenizer) skippedString(prefix string) bool {
	if self.isNextString(prefix) {
		self.skipString(prefix)
		return true
	}
	return false
}

func (self *Tokenizer) skippedNewline() bool {
	return self.skippedString("\r\n") || self.skippedByte('\n') || self.skippedByte('\r')
}
