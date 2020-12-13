package sqlp

/*
Notes on node representation

In a language with variant types (tagged unions), we would have represented
tokens/nodes as a variant. Go lacks variants, so the closest alternatives are:

	1) Emulating a variant type by using a struct where every field is a pointer,
	   and only one field must be non-nil. Type detection is performed by checking
	   which of the fields is non-nil.

	2) Using a single struct type, with an explicit type field. Type detection is
	   performed by comparing the type field to constants.

	3) Using an interface and a collection of concrete type implementing it. Type
	   detection is performed by unwrapping the interface and checking the
	   underlying concrete type.

The problems with (1) are: it allows too many invalid representations; it makes
it hard or annoying to check the type.

The problem with (2) is that the single node type must have the fields to
support every possible node, but not every type uses every field. It makes it
too hard to express or undestand which representations are valid, and makes
invalid representations too likely.

The problem with (3) is that it may involve more individual heap allocations and
indirections. But unlike the other two, it allows extremely simple specialized
node types (which may ultimately use less memory), and avoids invalid
representations. Unlike the other two, (3) allows the set of possible values to
be open. The user may introduce additional AST nodes that the parser didn't know
about, though this only goes from AST to formatted code, not the other
direction.

Misc notes

The parser doesn't discard unknown content. Instead, it accumulates anything it
doesn't recognize until it finds some "known" syntax, at which point it emits
the heretofore accumulated content as `NodeText`.

The token stream or parsed AST must serialize back into EXACTLY the source
content.
*/

import (
	"fmt"
	"strconv"
)

/*
AST node. May be a primitive token or a structure. `Tokenizer` emits only
primitive tokens.
*/
type Node interface {
	Append(*[]byte)
	fmt.Stringer
}

// Arbitrary text. Anything that wasn't recognized by the parser.
type NodeText string

func (self NodeText) Append(buf *[]byte) { appendStr(buf, string(self)) }
func (self NodeText) String() string     { return appenderToStr(self) }

// Text inside single quotes: ''. Escape sequences are not supported yet.
type NodeQuoteSingle string

func (self NodeQuoteSingle) Append(buf *[]byte) {
	appendByte(buf, '\'')
	appendStr(buf, string(self))
	appendByte(buf, '\'')
}

func (self NodeQuoteSingle) String() string { return appenderToStr(self) }

// Text inside double quotes: "". Escape sequences are not supported yet.
type NodeQuoteDouble string

func (self NodeQuoteDouble) Append(buf *[]byte) {
	appendByte(buf, '"')
	appendStr(buf, string(self))
	appendByte(buf, '"')
}

func (self NodeQuoteDouble) String() string { return appenderToStr(self) }

// Text inside grave quotes: ``. Escape sequences are not supported yet.
type NodeQuoteGrave string

func (self NodeQuoteGrave) Append(buf *[]byte) {
	appendByte(buf, '`')
	appendStr(buf, string(self))
	appendByte(buf, '`')
}

func (self NodeQuoteGrave) String() string { return appenderToStr(self) }

// Content of a line comment: --, including the newline.
type NodeCommentLine string

func (self NodeCommentLine) Append(buf *[]byte) {
	appendStr(buf, `--`)
	appendStr(buf, string(self))
}

func (self NodeCommentLine) String() string { return appenderToStr(self) }

// Content of a block comment: /* */.
type NodeCommentBlock string

func (self NodeCommentBlock) Append(buf *[]byte) {
	appendStr(buf, `/*`)
	appendStr(buf, string(self))
	appendStr(buf, `*/`)
}

func (self NodeCommentBlock) String() string { return appenderToStr(self) }

// Postgres cast operator: ::. Allows to disambiguate casts from named params.
type NodeDoubleColon struct{}

func (self NodeDoubleColon) Append(buf *[]byte) { appendStr(buf, `::`) }
func (self NodeDoubleColon) String() string     { return appenderToStr(self) }

// Postgres-style ordinal parameter placeholder: $1, $2, $3, ...
type NodeOrdinalParam int

func (self NodeOrdinalParam) Append(buf *[]byte) {
	appendByte(buf, '$')
	*buf = strconv.AppendInt(*buf, int64(self), 10)
}

func (self NodeOrdinalParam) String() string { return appenderToStr(self) }

// Convenience method that returns the corresponding Go index (starts at zero).
func (self NodeOrdinalParam) Index() int { return int(self) - 1 }

// Named parameter preceded by colon: :identifier
type NodeNamedParam string

func (self NodeNamedParam) Append(buf *[]byte) {
	appendByte(buf, ':')
	appendStr(buf, string(self))
}

func (self NodeNamedParam) String() string { return appenderToStr(self) }

// Opening parenthesis: (.
type NodeParenOpen struct{}

func (self NodeParenOpen) Append(buf *[]byte) { appendByte(buf, '(') }
func (self NodeParenOpen) String() string     { return appenderToStr(self) }

// Closing parenthesis: ).
type NodeParenClose struct{}

func (self NodeParenClose) Append(buf *[]byte) { appendByte(buf, ')') }
func (self NodeParenClose) String() string     { return appenderToStr(self) }

// Opening bracket: [.
type NodeBracketOpen struct{}

func (self NodeBracketOpen) Append(buf *[]byte) { appendByte(buf, '[') }
func (self NodeBracketOpen) String() string     { return appenderToStr(self) }

// Closing bracket: ].
type NodeBracketClose struct{}

func (self NodeBracketClose) Append(buf *[]byte) { appendByte(buf, ']') }
func (self NodeBracketClose) String() string     { return appenderToStr(self) }

// Opening brace: {.
type NodeBraceOpen struct{}

func (self NodeBraceOpen) Append(buf *[]byte) { appendByte(buf, '{') }
func (self NodeBraceOpen) String() string     { return appenderToStr(self) }

// Closing brace: }.
type NodeBraceClose struct{}

func (self NodeBraceClose) Append(buf *[]byte) { appendByte(buf, '}') }
func (self NodeBraceClose) String() string     { return appenderToStr(self) }

/*
Arbitrary sequence of AST nodes. When serializing, doesn't print any start or
end delimiters.
*/
type Nodes []Node

/*
Implement the `Node` interface. Simply concatenates the stringified
representations of the inner nodes, skipping any nil nodes.

`Nodes` can be arbitrarily nested without affecting the output. For example,
both `Nodes{}` and `Nodes{Nodes{}}` will print "".
*/
func (self Nodes) Append(buf *[]byte) {
	for _, node := range self {
		if node != nil {
			node.Append(buf)
		}
	}
}

func (self Nodes) String() string { return appenderToStr(self) }

// Nodes enclosed in parentheses: ().
type NodeParens Nodes

func (self NodeParens) Append(buf *[]byte) { appendEnclosed(buf, `(`, Nodes(self), `)`) }
func (self NodeParens) String() string     { return appenderToStr(self) }

// Nodes enclosed in brackets: [].
type NodeBrackets Nodes

func (self NodeBrackets) Append(buf *[]byte) { appendEnclosed(buf, `[`, Nodes(self), `]`) }
func (self NodeBrackets) String() string     { return appenderToStr(self) }

// Nodes enclosed in braces: {}.
type NodeBraces Nodes

func (self NodeBraces) Append(buf *[]byte) { appendEnclosed(buf, `{`, Nodes(self), `}`) }
func (self NodeBraces) String() string     { return appenderToStr(self) }
