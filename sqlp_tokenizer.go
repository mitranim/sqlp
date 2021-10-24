package sqlp

import (
	"fmt"
	"strings"
)

/*
Incremental parser. Example usage:

	tokenizer := Tokenizer{Source: `select * from some_table where some_col = $1`}

	for {
		tok := tokenizer.Next()
		if tok.IsInvalid() {
			break
		}
		fmt.Printf("%#v\n", tok)
	}

Tokenization is allocation-free, but parsing is always slow, and should be
amortized by caching whenever possible.
*/
type Tokenizer struct {
	Source string
	cursor int
	next   Token
}

/*
Returns the next token. Upon reaching EOF, returns `Token{}`. Use
`Token.IsInvalid` to detect end of iteration.
*/
func (self *Tokenizer) Token() Token {
	next := self.next
	if !next.IsInvalid() {
		self.next = Token{}
		return next
	}

	start := self.cursor

	for self.more() {
		mid := self.cursor
		if self.maybeWhitespace(); self.cursor > mid {
			return self.choose(start, mid, self.cursor, TypeWhitespace)
		}
		if self.maybeQuoteSingle(); self.cursor > mid {
			return self.choose(start, mid, self.cursor, TypeQuoteSingle)
		}
		if self.maybeQuoteDouble(); self.cursor > mid {
			return self.choose(start, mid, self.cursor, TypeQuoteDouble)
		}
		if self.maybeQuoteGrave(); self.cursor > mid {
			return self.choose(start, mid, self.cursor, TypeQuoteGrave)
		}
		if self.maybeCommentLine(); self.cursor > mid {
			return self.choose(start, mid, self.cursor, TypeCommentLine)
		}
		if self.maybeCommentBlock(); self.cursor > mid {
			return self.choose(start, mid, self.cursor, TypeCommentBlock)
		}
		if self.maybeDoubleColon(); self.cursor > mid {
			return self.choose(start, mid, self.cursor, TypeDoubleColon)
		}
		if self.maybeOrdinalParam(); self.cursor > mid {
			return self.choose(start, mid, self.cursor, TypeOrdinalParam)
		}
		if self.maybeNamedParam(); self.cursor > mid {
			return self.choose(start, mid, self.cursor, TypeNamedParam)
		}
		if self.maybeParenOpen(); self.cursor > mid {
			return self.choose(start, mid, self.cursor, TypeParenOpen)
		}
		if self.maybeParenClose(); self.cursor > mid {
			return self.choose(start, mid, self.cursor, TypeParenClose)
		}
		if self.maybeBracketOpen(); self.cursor > mid {
			return self.choose(start, mid, self.cursor, TypeBracketOpen)
		}
		if self.maybeBracketClose(); self.cursor > mid {
			return self.choose(start, mid, self.cursor, TypeBracketClose)
		}
		if self.maybeBraceOpen(); self.cursor > mid {
			return self.choose(start, mid, self.cursor, TypeBraceOpen)
		}
		if self.maybeBraceClose(); self.cursor > mid {
			return self.choose(start, mid, self.cursor, TypeBraceClose)
		}
		self.skipChar()
	}

	if self.cursor > start {
		return Token{Region{start, self.cursor}, TypeText}
	}
	return Token{}
}

func (self *Tokenizer) choose(start, mid, end int, typ Type) Token {
	prev := Token{Region{start, mid}, TypeText}
	next := Token{Region{mid, end}, typ}

	if !next.Region.HasLen() {
		panic(fmt.Errorf(`[sqlp] internal error: attempted to provide empty region for next token`))
	}

	if prev.Region.HasLen() {
		self.setNext(next)
		return prev
	}

	return next
}

func (self *Tokenizer) setNext(next Token) {
	if !self.next.IsInvalid() {
		panic(fmt.Errorf(
			`[sqlp] internal error: attempted to overwrite non-empty pending token %#v with token %#v`,
			self.next, next,
		))
	}
	self.next = next
}

func (self *Tokenizer) maybeWhitespace() {
	for self.isNextWhitespace() {
		self.skipByte()
	}
}

func (self *Tokenizer) maybeQuoteSingle() {
	self.maybeStringBetweenBytes(quoteSingle, quoteSingle)
}

func (self *Tokenizer) maybeQuoteDouble() {
	self.maybeStringBetweenBytes(quoteDouble, quoteDouble)
}

func (self *Tokenizer) maybeQuoteGrave() {
	self.maybeStringBetweenBytes(quoteGrave, quoteGrave)
}

func (self *Tokenizer) maybeCommentLine() {
	if !self.skippedString(commentLinePrefix) {
		return
	}

	for self.more() {
		if self.skippedNewline() {
			break
		}
		self.skipChar()
	}
}

func (self *Tokenizer) maybeCommentBlock() {
	self.maybeStringBetween(commentBlockPrefix, commentBlockSuffix)
}

func (self *Tokenizer) maybeDoubleColon() {
	self.maybeSkipString(castPrefix)
}

func (self *Tokenizer) maybeOrdinalParam() {
	if !self.isNextByte(ordinalPrefix) {
		return
	}

	digits := prefixDigits(self.restAfter(ordinalPrefixLen))
	size := len(digits)
	if size == 0 {
		return
	}

	self.skipBytes(ordinalPrefixLen + size)
}

func (self *Tokenizer) maybeNamedParam() {
	if !self.isNextByte(namedPrefix) {
		return
	}

	ident := prefixIdent(self.restAfter(namedPrefixLen))
	size := len(ident)
	if size == 0 {
		return
	}

	self.skipBytes(namedPrefixLen + size)
}

func (self *Tokenizer) maybeParenOpen() {
	self.maybeSkipByte(parenOpen)
}

func (self *Tokenizer) maybeParenClose() {
	self.maybeSkipByte(parenClose)
}

func (self *Tokenizer) maybeBracketOpen() {
	self.maybeSkipByte(bracketOpen)
}

func (self *Tokenizer) maybeBracketClose() {
	self.maybeSkipByte(bracketClose)
}

func (self *Tokenizer) maybeBraceOpen() {
	self.maybeSkipByte(braceOpen)
}

func (self *Tokenizer) maybeBraceClose() {
	self.maybeSkipByte(braceClose)
}

func (self *Tokenizer) maybeStringBetween(prefix string, suffix string) {
	if !self.skippedString(prefix) {
		return
	}

	for self.more() {
		if self.isNextString(suffix) {
			self.skipBytes(len(suffix))
			return
		}
		self.skipChar()
	}

	panic(fmt.Errorf(`[sqlp] expected closing %q, got unexpected EOF`, suffix))
}

// Faster than `maybeStringBetween`, enough to make a difference in benchmarks.
func (self *Tokenizer) maybeStringBetweenBytes(prefix byte, suffix byte) {
	if !self.skippedByte(prefix) {
		return
	}

	for self.more() {
		if self.skippedByte(suffix) {
			return
		}
		self.skipChar()
	}

	panic(fmt.Errorf(`[sqlp] expected closing %q, got unexpected EOF`, rune(suffix)))
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

func (self *Tokenizer) rest() string {
	return self.restFrom(self.cursor)
}

func (self *Tokenizer) restFrom(start int) string {
	if start < len(self.Source) {
		return self.Source[start:]
	}
	return ``
}

func (self *Tokenizer) restAfter(delta int) string {
	return self.restFrom(self.cursor + delta)
}

func (self *Tokenizer) isNextString(prefix string) bool {
	return strings.HasPrefix(self.rest(), prefix)
}

func (self *Tokenizer) isNextByte(char byte) bool {
	return self.headByte() == char
}

func (self *Tokenizer) isNextWhitespace() bool {
	return charsetWhitespace.has(self.headByte())
}

func (self *Tokenizer) skipByte() { self.skipBytes(1) }

func (self *Tokenizer) skipBytes(count int) { self.cursor += count }

func (self *Tokenizer) skipChar() {
	_, size := headChar(self.rest())
	self.skipBytes(size)
}

func (self *Tokenizer) skippedByte(char byte) bool {
	if self.isNextByte(char) {
		self.skipByte()
		return true
	}
	return false
}

func (self *Tokenizer) maybeSkipByte(char byte) {
	_ = self.skippedByte(char)
}

func (self *Tokenizer) skippedString(prefix string) bool {
	if self.isNextString(prefix) {
		self.skipBytes(len(prefix))
		return true
	}
	return false
}

func (self *Tokenizer) maybeSkipString(prefix string) string {
	if self.isNextString(prefix) {
		self.skipBytes(len(prefix))
		return prefix
	}
	return ``
}

func (self *Tokenizer) skippedNewline() bool {
	return self.skippedString("\r\n") || self.skippedByte('\n') || self.skippedByte('\r')
}
