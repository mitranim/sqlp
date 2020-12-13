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
also lets you parse into a fully-built AST via `Parse`. Choose the approach
that better suits your use case.

Usage

Oversimplified example:

	nodes, err := Parse(`select * from some_table where :ident::uuid = id`)
	panic(err)

	err := TraverseLeaves(nodes, func(ptr *Node) error {
		switch node := (*ptr).(type) {
		case NodeNamedParam:
			*ptr = node + `_renamed`
		}
		return nil
	})
	panic(err)

The AST now looks like this:

	nodes := Nodes{
		NodeText(`select * from some_table where `),
		NodeNamedParam(`ident_renamed`),
		NodeDoubleColon{},
		NodeText(`uuid = id`),
	}
*/
package sqlp

import (
	"fmt"
)

/*
Parses a query and returns the AST. For the AST structure, see `Node` and the
various node types. Also see `Tokenizer` and `Tokenizer.Next` for incremental
parsing.

Example:

	nodes, err := Parse(`select * from some_table where id = :ident`)
	panic(err)

	err := TraverseLeaves(nodes, func(ptr *Node) error {
		switch (*ptr).(type) {
		case NodeNamedParam:
			*ptr = NodeOrdinalParam(1)
		}
		return nil
	})
	panic(err)
*/
func Parse(input string) (nodes Nodes, err error) {
	defer rec(&err)
	parse(&Tokenizer{Source: input}, &nodes)
	return
}

func parse(tokenizer *Tokenizer, nodes *Nodes) {
	for {
		node := tokenizer.Next()
		if node == nil {
			break
		}
		onNode(tokenizer, nodes, node)
	}
}

func onNode(tokenizer *Tokenizer, nodes *Nodes, node Node) {
	switch node := node.(type) {
	case NodeParenOpen:
		parseParens(tokenizer, nodes)

	case NodeBracketOpen:
		parseBrackets(tokenizer, nodes)

	case NodeBraceOpen:
		parseBraces(tokenizer, nodes)

	case NodeParenClose, NodeBracketClose, NodeBraceClose:
		panic(fmt.Errorf(`[sqlp] unexpected closing %q`, node))

	default:
		*nodes = append(*nodes, node)
	}
}

func parseParens(tokenizer *Tokenizer, parent *Nodes) {
	var nodes NodeParens
	parseBetween(tokenizer, parent, (*Nodes)(&nodes), NodeParenClose{})
	*parent = append(*parent, nodes)
}

func parseBrackets(tokenizer *Tokenizer, parent *Nodes) {
	var nodes NodeBrackets
	parseBetween(tokenizer, parent, (*Nodes)(&nodes), NodeBracketClose{})
	*parent = append(*parent, nodes)
}

func parseBraces(tokenizer *Tokenizer, parent *Nodes) {
	var nodes NodeBraces
	parseBetween(tokenizer, parent, (*Nodes)(&nodes), NodeBraceClose{})
	*parent = append(*parent, nodes)
}

func parseBetween(tokenizer *Tokenizer, parent *Nodes, nodes *Nodes, close Node) {
	for {
		node := tokenizer.Next()
		if node == nil {
			break
		}
		if node == close {
			return
		}
		onNode(tokenizer, nodes, node)
	}
	panic(fmt.Errorf(`[sqlp] missing closing %q`, close))
}

/*
Performs a shallow traversal, invoking `fun` for the pointer to each non-nil
node in the sequence.
*/
func TraverseShallow(nodes Nodes, fun func(*Node) error) error {
	for i := range nodes {
		if nodes[i] == nil {
			continue
		}
		err := fun(&nodes[i])
		if err != nil {
			return err
		}
	}
	return nil
}

/*
Performs a deep traversal, invoking `fun` for the pointer to each leaf node.
*/
func TraverseLeaves(nodes Nodes, fun func(*Node) error) error {
	for i := range nodes {
		err := NodeTraverseLeaves(&nodes[i], fun)
		if err != nil {
			return err
		}
	}
	return nil
}

/*
Performs a deep traversal, invoking `fun` for the pointer to each leaf node,
which may include the root passed to the function.
*/
func NodeTraverseLeaves(ptr *Node, fun func(*Node) error) error {
	if ptr == nil {
		return nil
	}

	switch val := (*ptr).(type) {
	case nil:
		return nil
	case Nodes:
		return TraverseLeaves(val, fun)
	case NodeParens:
		return TraverseLeaves(Nodes(val), fun)
	case NodeBrackets:
		return TraverseLeaves(Nodes(val), fun)
	case NodeBraces:
		return TraverseLeaves(Nodes(val), fun)
	default:
		return fun(ptr)
	}
}

// Makes a deep copy of `Nodes` whose mutations won't affect the original.
func CopyDeep(src Nodes) Nodes {
	if src == nil {
		return src
	}
	out := make(Nodes, len(src))
	for i := range src {
		out[i] = NodeCopyDeep(src[i])
	}
	return out
}

/*
For mutable node types, makes a deep copy. For immutable node types, returns the
node as-is.
*/
func NodeCopyDeep(node Node) Node {
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
Returns the first leaf node from the provided collection, or nil.
*/
func FirstLeaf(nodes Nodes) Node {
	if len(nodes) > 0 {
		return NodeFirstLeaf(nodes[0])
	}
	return nil
}

/*
If the provided node is a collection, returns the last leaf node from it.
Otherwise, returns the node as-is.
*/
func NodeFirstLeaf(node Node) Node {
	return nodeBy(node, FirstLeaf)
}

/*
Returns the last leaf node from the provided collection, or nil.
*/
func LastLeaf(nodes Nodes) Node {
	if len(nodes) > 0 {
		return NodeLastLeaf(nodes[len(nodes)-1])
	}
	return nil
}

/*
If the provided node is a collection, returns the last leaf node from it.
Otherwise, returns the node as-is.
*/
func NodeLastLeaf(node Node) Node {
	return nodeBy(node, LastLeaf)
}

func nodeBy(node Node, fun func(Nodes) Node) Node {
	switch node := node.(type) {
	case Nodes:
		return fun(node)
	case NodeParens:
		return fun(Nodes(node))
	case NodeBrackets:
		return fun(Nodes(node))
	case NodeBraces:
		return fun(Nodes(node))
	default:
		return node
	}
}
