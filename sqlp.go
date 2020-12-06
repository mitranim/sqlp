/*
Parser and formatter for rewriting foreign code embedded in SQL queries, such as
parameter placeholders: `$1` or `:ident`, or code encased in delimiters: `()`
`[]` `{}`. It supports the following SQL features:

• '' : single quotes.

• "" : double quotes.

• `` : grave quotes (non-standard).

• -- : line comments.

• /* : block comments.

• :: : Postgres-style cast operator (non-standard).

In addition, it supports the following:

• ()          : content in parens.

• []          : content in brackets.

• {}          : content in braces.

• $1 $2 ...   : ordinal parameter placeholders.

• :identifier : named parameter placeholders.

Supporting SQL quotes and comments allows us to correctly ignore text inside
special delimiters that happens to be part of a string, quoted identifier, or
comment.

Usage

Oversimplified example:

	nodes, err := Parse(`select * from some_table where :ident::uuid = id`)
	panic(err)

	err := TraverseDeep(nodes, func(ptr *Node) error {
		name, ok := (*ptr).(NodeNamedParam)
		if ok {
			// Guaranteed to break the query.
			*ptr = name + `_renamed`
		}
		return nil
	})
	panic(err)

	sql := nodes.String()

The AST now looks like this:

	nodes := Nodes{
		NodeText(`select * from some_table where `),
		NodeNamedParam(`ident_renamed`),
		NodeDoubleColon{},
		NodeText(`uuid = id`),
	}
*/
package sqlp

/*
Notes on AST

In a language with variant types (tagged unions), we would have represented AST
nodes as a variant. Go lacks variants, so the closest alternatives are:

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

Misc Notes

The parser doesn't discard unknown content. Instead, it accumulates anything it
doesn't recognize until it finds some "known" syntax, at which point it takes
anything heretofore accumulated, and appends it as `NodeText` to the current
level of the AST.

The parsed AST must serialize back into EXACTLY the source content.

Serializing the AST by bashing strings together is slightly inefficient; we
could instead use a builder/append interface, reducing the allocations. But the
actual difference in performance is very minor, and the current approach is
simpler and shorter.
*/

import (
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"
)

// Any AST node.
type Node fmt.Stringer

// Arbitrary text. Anything that wasn't recognized by the parser.
type NodeText string

// Implement the `Node` interface.
func (self NodeText) String() string { return string(self) }

// Text inside single quotes: ''. Escape sequences are not supported yet.
type NodeQuoteSingle string

// Implement the `Node` interface.
func (self NodeQuoteSingle) String() string { return `'` + string(self) + `'` }

// Text inside double quotes: "". Escape sequences are not supported yet.
type NodeQuoteDouble string

// Implement the `Node` interface.
func (self NodeQuoteDouble) String() string { return `"` + string(self) + `"` }

// Text inside grave quotes: ``. Escape sequences are not supported yet.
type NodeQuoteGrave string

// Implement the `Node` interface.
func (self NodeQuoteGrave) String() string { return "`" + string(self) + "`" }

// Content of a line comment: --, including the newline.
type NodeCommentLine string

// Implement the `Node` interface.
func (self NodeCommentLine) String() string { return `--` + string(self) }

// Content of a block comment: /* */.
type NodeCommentBlock string

// Implement the `Node` interface.
func (self NodeCommentBlock) String() string { return `/*` + string(self) + `*/` }

// Postgres type cast operator: ::.
type NodeDoubleColon struct{}

// Implement the `Node` interface.
func (self NodeDoubleColon) String() string { return `::` }

// Postgres-style ordinal parameter placeholder: $1, $2, $3, ...
type NodeOrdinalParam uint

// Implement the `Node` interface.
func (self NodeOrdinalParam) String() string { return `$` + strconv.Itoa(int(self)) }

// Named parameter preceded by colon: :identifier.
type NodeNamedParam string

// Implement the `Node` interface.
func (self NodeNamedParam) String() string { return `:` + string(self) }

// Nodes enclosed in parentheses: ().
type NodeParens []Node

// Implement the `Node` interface.
func (self NodeParens) String() string { return `(` + Nodes(self).String() + `)` }

// Nodes enclosed in brackets: [].
type NodeBrackets []Node

// Implement the `Node` interface.
func (self NodeBrackets) String() string { return `[` + Nodes(self).String() + `]` }

// Nodes enclosed in braces: {}.
type NodeBraces []Node

// Implement the `Node` interface.
func (self NodeBraces) String() string { return `{` + Nodes(self).String() + `}` }

/*
Arbitrary sequence of AST nodes. This is the main AST type, used by `Parse()`,
`Traverse()`, and some other functions.
*/
type Nodes []Node

/*
Implement the `Node` interface. Simply concatenates the stringified
representations of the inner nodes, skipping any nil nodes.

`Nodes` can be arbitrarily nested without affecting the output. For example,
both `Nodes{}` and `Nodes{Nodes{}}` will print "".
*/
func (self Nodes) String() string {
	var out string
	for _, node := range self {
		if node != nil {
			out += node.String()
		}
	}
	return out
}

/*
Parses a query and returns the AST. For the AST structure, see `Node` and the
various node types.

Example:

	ast, err := Parse(`select * from some_table where id = :ident`)
	panic(err)

	// Non-recursive example for simplicity.
	for i, node := range ast {
		switch node.(type) {
		case NodeNamedParam:
			ast[i] = NodeOrdinalParam(1)
		}
	}

	sql := ast.String()
*/
func Parse(input string) (Nodes, error) {
	state := parseState{source: input}
	return state.parse()
}

type parseState struct {
	source string
	cursor int
}

func (self *parseState) parse() (nodes Nodes, err error) {
	defer rec(&err)

	tail := self.cursor
	for self.more() {
		self.advance(&nodes, &tail)
	}
	self.flushText(&nodes, tail)

	return
}

func (self *parseState) maybePopNonText() Node {
	head := self.cursor
	if node := self.maybePopQuoteSingle(); self.cursor > head {
		return node
	}
	if node := self.maybePopQuoteDouble(); self.cursor > head {
		return node
	}
	if node := self.maybePopQuoteGrave(); self.cursor > head {
		return node
	}
	if node := self.maybePopCommentLine(); self.cursor > head {
		return node
	}
	if node := self.maybePopCommentBlock(); self.cursor > head {
		return node
	}
	if node := self.maybePopDoubleColon(); self.cursor > head {
		return node
	}
	if node := self.maybePopOrdinalParam(); self.cursor > head {
		return node
	}
	if node := self.maybePopNamedParam(); self.cursor > head {
		return node
	}
	if node := self.maybePopParens(); self.cursor > head {
		return node
	}
	if node := self.maybePopBrackets(); self.cursor > head {
		return node
	}
	if node := self.maybePopBraces(); self.cursor > head {
		return node
	}
	return nil
}

func (self *parseState) maybePopQuoteSingle() NodeQuoteSingle {
	return NodeQuoteSingle(self.maybePopStringBetween(`'`, `'`))
}

func (self *parseState) maybePopQuoteDouble() NodeQuoteDouble {
	return NodeQuoteDouble(self.maybePopStringBetween(`"`, `"`))
}

func (self *parseState) maybePopQuoteGrave() NodeQuoteGrave {
	return NodeQuoteGrave(self.maybePopStringBetween("`", "`"))
}

func (self *parseState) maybePopCommentLine() NodeCommentLine {
	if self.next(`--`) {
		self.cursor += len(`--`)
		return NodeCommentLine(self.popStringUntilNewlineOrEof())
	}
	return ""
}

func (self *parseState) maybePopCommentBlock() NodeCommentBlock {
	return NodeCommentBlock(self.maybePopStringBetween(`/*`, `*/`))
}

func (self *parseState) maybePopDoubleColon() NodeDoubleColon {
	if self.next(`::`) {
		self.cursor += len(`::`)
	}
	return NodeDoubleColon{}
}

func (self *parseState) maybePopOrdinalParam() NodeOrdinalParam {
	const prefix = `$`

	if self.next(prefix) {
		digits := prefixDigits(self.rest()[len(prefix):])
		if len(digits) > 0 {
			self.cursor += len(prefix) + len(digits)
			return NodeOrdinalParam(mustParseUint64(string(digits)))
		}
	}

	return 0
}

// Must be preceded by `maybePopDoubleColon`.
func (self *parseState) maybePopNamedParam() NodeNamedParam {
	const prefix = `:`

	if self.next(prefix) {
		ident := prefixIdent(self.rest()[len(prefix):])
		if len(ident) > 0 {
			self.cursor += len(prefix) + len(ident)
			return NodeNamedParam(ident)
		}
	}

	return ""
}

func (self *parseState) maybePopParens() NodeParens {
	return NodeParens(self.maybePopNodesBetween(`(`, `)`))
}

func (self *parseState) maybePopBrackets() NodeBrackets {
	return NodeBrackets(self.maybePopNodesBetween(`[`, `]`))
}

func (self *parseState) maybePopBraces() NodeBraces {
	return NodeBraces(self.maybePopNodesBetween(`{`, `}`))
}

func (self *parseState) maybePopStringBetween(prefix string, suffix string) string {
	if !self.next(prefix) {
		return ``
	}

	self.cursor += len(prefix)
	start := self.cursor

	for self.more() {
		if self.next(suffix) {
			chunk := self.from(start)
			self.cursor += len(suffix)
			return chunk
		}

		self.inc()
	}

	panic(self.err(fmt.Errorf(`expected closing %q, found EOF`, suffix)))
}

func (self *parseState) popStringUntilNewlineOrEof() string {
	start := self.cursor

	for self.more() {
		if self.next("\r\n") {
			self.cursor += len("\r\n")
			return self.from(start)
		}

		if self.next("\n") {
			self.cursor += len("\n")
			return self.from(start)
		}

		if self.next("\r") {
			self.cursor += len("\r")
			return self.from(start)
		}

		self.inc()
	}

	return self.from(start)
}

func (self *parseState) maybePopNodesBetween(prefix string, suffix string) Nodes {
	if !self.next(prefix) {
		return nil
	}

	self.cursor += len(prefix)
	tail := self.cursor
	var nodes Nodes

	for self.more() {
		if self.next(suffix) {
			self.flushText(&nodes, tail)
			self.cursor += len(suffix)
			return nodes
		}

		self.advance(&nodes, &tail)
	}

	panic(self.err(fmt.Errorf(`expected closing %q, found EOF`, suffix)))
}

func (self *parseState) flushText(nodes *Nodes, tail int) {
	maybeAppendText(nodes, NodeText(self.from(tail)))
}

func (self *parseState) advance(nodes *Nodes, tail *int) {
	head := self.cursor
	node := self.maybePopNonText()

	if self.cursor > head {
		if node == nil {
			panic(self.err(fmt.Errorf(`unexpected nil node after advancing cursor`)))
		}

		maybeAppendText(nodes, NodeText(self.source[*tail:head]))
		maybeAppendNode(nodes, node)
		*tail = self.cursor
		return
	}

	if node != nil {
		panic(self.err(fmt.Errorf(`unexpected non-nil node without advancing cursor`)))
	}
	self.inc()
}

func (self *parseState) inc() {
	_, size := utf8.DecodeRuneInString(self.rest())
	self.cursor += size
}

func (self parseState) left() int {
	return len(self.source) - self.cursor
}

func (self parseState) more() bool {
	return self.left() > 0
}

func (self parseState) from(index int) string {
	if index < 0 {
		index = 0
	}
	if index < self.cursor {
		return self.source[index:self.cursor]
	}
	return ``
}

func (self parseState) next(prefix string) bool {
	return strings.HasPrefix(self.rest(), prefix)
}

func (self parseState) rest() string {
	if self.more() {
		return self.source[self.cursor:]
	}
	return ""
}

func (self parseState) preview() string {
	const limit = 32
	if self.left() > limit {
		return self.source[self.cursor:self.cursor+limit] + ` ...`
	}
	return self.rest()
}

func (self parseState) err(cause error) error {
	return &Error{Cause: cause, parseState: self}
}

/*
Performs a shallow traversal, invoking `fun` for each node.
*/
func Traverse(nodes Nodes, fun func(*Node) error) error {
	for i := range nodes {
		err := fun(&nodes[i])
		if err != nil {
			return err
		}
	}
	return nil
}

/*
Similar to `Traverse`, but deep. Calls `fun` for each leaf node, and ONLY for
leaf nodes.
*/
func TraverseDeep(nodes Nodes, fun func(*Node) error) error {
	for i := range nodes {
		var err error
		switch node := nodes[i].(type) {
		case Nodes:
			err = TraverseDeep(node, fun)
		case NodeParens:
			err = TraverseDeep(Nodes(node), fun)
		case NodeBrackets:
			err = TraverseDeep(Nodes(node), fun)
		case NodeBraces:
			err = TraverseDeep(Nodes(node), fun)
		default:
			err = fun(&nodes[i])
		}
		if err != nil {
			return err
		}
	}
	return nil
}

/*
Makes a deep copy of the input AST. Useful when you plan to mutate one but keep
the other.
*/
func CopyDeep(src Nodes) Nodes {
	if src == nil {
		return nil
	}
	out := make(Nodes, len(src))
	for i := range src {
		out[i] = copyNode(src[i])
	}
	return out
}

func copyNode(node Node) Node {
	switch node := node.(type) {
	case Nodes:
		return CopyDeep(node)
	case NodeParens:
		return NodeParens(CopyDeep(Nodes(node)))
	case NodeBrackets:
		return NodeBrackets(CopyDeep(Nodes(node)))
	case NodeBraces:
		return NodeBraces(CopyDeep(Nodes(node)))
	default:
		return node
	}
}

/*
Returns the first leaf node from the provided AST, or nil.
*/
func FirstLeaf(nodes Nodes) Node {
	for len(nodes) > 0 {
		switch node := nodes[0].(type) {
		case Nodes:
			nodes = node
		case NodeParens:
			nodes = Nodes(node)
		case NodeBrackets:
			nodes = Nodes(node)
		case NodeBraces:
			nodes = Nodes(node)
		default:
			return node
		}
	}
	return nil
}

/*
Returns the last leaf node from the provided AST, or nil.
*/
func LastLeaf(nodes Nodes) Node {
	for len(nodes) > 0 {
		switch node := nodes[len(nodes)-1].(type) {
		case Nodes:
			nodes = node
		case NodeParens:
			nodes = Nodes(node)
		case NodeBrackets:
			nodes = Nodes(node)
		case NodeBraces:
			nodes = Nodes(node)
		default:
			return node
		}
	}
	return nil
}

/*
Parsing error. Includes the parser state corresponding to the place where the
error had occurred. The parser state should be used for detailed error printing
(but isn't yet).
*/
type Error struct {
	Cause      error
	parseState parseState
}

/*
Prints the error message. Should include parser context such as line, column,
and surrounding text, but doesn't yet.
*/
func (self *Error) Error() string {
	if self.Cause != nil {
		return self.Cause.Error()
	}
	return ""
}

// Implement a hidden interface in "errors".
func (self *Error) Unwrap() error {
	return self.Cause
}

/*
Fancier printing.

TODO: support '#' properly; include parser context such as line, column, and
surrounding text.
*/
func (self *Error) Format(fms fmt.State, verb rune) {
	switch verb {
	case 'v':
		if fms.Flag('+') || fms.Flag('#') {
			if self.Cause != nil {
				fmt.Fprintf(fms, "%+v", self.Cause)
			}
			return
		}
		fms.Write([]byte(self.Error()))
	default:
		fms.Write([]byte(self.Error()))
	}
}
