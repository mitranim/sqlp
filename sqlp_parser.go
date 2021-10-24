package sqlp

import "fmt"

/*
Parses SQL text and returns the resulting AST. For the AST structure, see `Node`
and the various node types. Also see `Tokenizer` and `Tokenizer.Next` for
incremental parsing.

Example:

	nodes, err := Parse(`select * from some_table where id = :ident`)
	panic(err)

	WalkNodePtr(nodes, func(ptr *Node) {
		switch (*ptr).(type) {
		case NodeNamedParam:
			*ptr = NodeOrdinalParam(1)
		}
	})
*/
func Parse(src string) (Nodes, error) {
	parser := Parser{Tokenizer: Tokenizer{Source: src}}
	return parser.Parse()
}

// See `Parse`.
type Parser struct{ Tokenizer }

// See `Parse`.
func (self *Parser) Parse() (nodes Nodes, err error) {
	defer rec(&err)
	self.parse(&nodes)
	return
}

func (self *Parser) parse(nodes *Nodes) {
	for {
		tok := self.Token()
		if tok.IsInvalid() {
			return
		}
		self.parseToken(nodes, tok)
	}
}

func (self *Parser) parseToken(nodes *Nodes, tok Token) {
	switch tok.Type {
	case TypeParenOpen:
		*nodes = append(*nodes, self.parseParens())

	case TypeBracketOpen:
		*nodes = append(*nodes, self.parseBrackets())

	case TypeBraceOpen:
		*nodes = append(*nodes, self.parseBraces())

	case TypeParenClose, TypeBracketClose, TypeBraceClose:
		panic(fmt.Errorf(`[sqlp] unexpected closing %q`, tok.Slice(self.Source)))

	default:
		*nodes = append(*nodes, tok.Node(self.Source))
	}
}

func (self *Parser) parseParens() (out ParenNodes) {
	self.parseUntil((*Nodes)(&out), TypeParenClose, `)`)
	return
}

func (self *Parser) parseBrackets() (out BracketNodes) {
	self.parseUntil((*Nodes)(&out), TypeBracketClose, `]`)
	return
}

func (self *Parser) parseBraces() (out BraceNodes) {
	self.parseUntil((*Nodes)(&out), TypeBraceClose, `}`)
	return
}

func (self *Parser) parseUntil(nodes *Nodes, typ Type, str string) {
	for {
		tok := self.Token()
		if tok.IsInvalid() {
			break
		}
		if tok.Type == typ {
			return
		}
		self.parseToken(nodes, tok)
	}

	panic(fmt.Errorf(`[sqlp] missing closing delimiter %q`, str))
}
