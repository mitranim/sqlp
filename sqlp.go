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

Tokenization vs Parsing

This library supports incremental parsing token by token, via `Tokenizer`. It
also lets you convert a sequence of tokens into a fully-built AST via `Parser`.
Choose the approach that better suits your use case.

Usage

Oversimplified example:

	nodes, err := Parse(`select * from some_table where :ident::uuid = id`)
	panic(err)

	WalkNodePtr(nodes, func(ptr *Node) {
		switch node := (*ptr).(type) {
		case NodeNamedParam:
			*ptr = node + `_renamed`
		}
	})

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
AST node. May be a primitive token or a structure. `Tokenizer` emits only
primitive tokens.
*/
type Node interface {
	// Implement `fmt.Stringer`. Must return the SQL representation of the node,
	// matching the source it was parsed from.
	String() string

	// Must append the same text representation as `.String`. Allows more
	// efficient text encoding for large AST.
	AppendTo([]byte) []byte
}

// Implemented by collection types such as `Nodes` and `ParenNodes`. Used by the
// global `CopyNode` function.
type Copier interface{ CopyNode() Node }

// Implemented by collection types such as `Nodes` and `ParenNodes`.
type Coll interface{ Nodes() Nodes }

// Implemented by collection types such as `Nodes` and `ParenNodes`. Used by the
// global function `WalkNode`.
type Walker interface{ WalkNode(func(Node)) }

// Implemented by collection types such as `Nodes` and `ParenNodes`. Used by the
// global function `WalkNodePtr`.
type PtrWalker interface{ WalkNodePtr(func(*Node)) }

/*
Walks the node, invoking the given function for each non-nil node that doesn't
implement `Walker`. Nodes that implement `Walker` receive the function as
input, with implementation-specific behavior. All `Walker` implementations in
this package perform a shallow walk, invoking a given function once for each
immediate child.
*/
func WalkNode(val Node, fun func(Node)) {
	if val == nil || fun == nil {
		return
	}

	impl, _ := val.(Walker)
	if impl != nil {
		impl.WalkNode(fun)
		return
	}

	fun(val)
}

/*
Similar to `WalkNode`, but invokes the function for node pointers rather than
node values. Allows AST editing.
*/
func WalkNodePtr(val *Node, fun func(*Node)) {
	if val == nil || *val == nil || fun == nil {
		return
	}

	impl, _ := (*val).(PtrWalker)
	if impl != nil {
		impl.WalkNodePtr(fun)
		return
	}

	fun(val)
}

/*
Similar to `WalkNode`, but performs a deep walk, invoking the function only for
"leaf nodes" that don't implement `Walker`.
*/
func DeepWalkNode(val Node, fun func(Node)) {
	if val == nil || fun == nil {
		return
	}

	impl, _ := val.(Walker)
	if impl != nil {
		// TODO does this need optimization?
		impl.WalkNode(func(val Node) {
			DeepWalkNode(val, fun)
		})
		return
	}

	fun(val)
}

// Makes a copy that should be safe to modify without affecting the original.
func CopyNode(node Node) Node {
	impl, _ := node.(Copier)
	if impl != nil {
		return impl.CopyNode()
	}
	return node
}
