package sqlp

import (
	"fmt"
	"reflect"
	"testing"
)

func TestParse(_ *testing.T) {
	// For brevity.
	type (
		T = NodeText
		W = NodeWhitespace
		O = NodeOrdinalParam
		N = NodeNamedParam
		D = NodeDoubleColon
		P = ParenNodes
		B = BracketNodes
		C = NodeCommentLine
	)

	test := func(input string, astExpected Nodes) {
		ast, err := Parse(input)
		try(err)
		eq(astExpected, ast)
		eq(input, ast.String())
	}

	test(
		`[one two]`,
		Nodes{B{T(`one`), W(` `), T(`two`)}},
	)

	test(
		`one [two three]`,
		Nodes{T(`one`), W(` `), B{T(`two`), W(` `), T(`three`)}},
	)

	test(
		`one [two three] four`,
		Nodes{T(`one`), W(` `), B{T(`two`), W(` `), T(`three`)}, W(` `), T(`four`)},
	)

	test(
		`[one two] three`,
		Nodes{B{NodeText(`one`), W(` `), T(`two`)}, W(` `), T(`three`)},
	)

	test(
		`[one two] [three four]`,
		Nodes{B{T(`one`), W(` `), T(`two`)}, W(` `), B{T(`three`), W(` `), T(`four`)}},
	)

	test(
		`'[one]'`,
		Nodes{NodeQuoteSingle(`[one]`)},
	)

	test(
		`"[one]"`,
		Nodes{NodeQuoteDouble(`[one]`)},
	)

	test(
		"`[one]`",
		Nodes{NodeQuoteGrave(`[one]`)},
	)

	test(
		`[[({one two})]]`,
		Nodes{B{B{P{BraceNodes{T(`one`), W(` `), T(`two`)}}}}},
	)

	test(
		"one -- two",
		Nodes{T(`one`), W(` `), C(` two`)},
	)

	test(
		"one -- two \n three",
		Nodes{T(`one`), W(` `), C(" two \n"), W(` `), T(`three`)},
	)

	test(
		`one::two :three`,
		Nodes{T(`one`), NodeDoubleColon{}, T(`two`), W(` `), N(`three`)},
	)

	test(
		`1 $2::int`,
		Nodes{T(`1`), W(` `), O(2), NodeDoubleColon{}, T(`int`)},
	)

	test(
		`one = $1 and two = $2`,
		Nodes{T(`one`), W(` `), T(`=`), W(` `), O(1), W(` `), T(`and`), W(` `), T(`two`), W(` `), T(`=`), W(` `), O(2)},
	)

	test(
		`$1 $2::int :three::text`,
		Nodes{O(1), W(` `), O(2), D{}, T(`int`), W(` `), N(`three`), D{}, T(`text`)},
	)

	test(
		`one = $1 and two = $1 and three = $2`,
		Nodes{T(`one`), W(` `), T(`=`), W(` `), O(1), W(` `), T(`and`), W(` `), T(`two`), W(` `), T(`=`), W(` `), O(1), W(` `), T(`and`), W(` `), T(`three`), W(` `), T(`=`), W(` `), O(2)},
	)
}

func TestRewrite(_ *testing.T) {
	ast, err := Parse(`select * from [bracketed] where col1 = 123`)
	if err != nil {
		panic(fmt.Errorf("%+v", err))
	}

	for i, node := range ast {
		switch node.(type) {
		case BracketNodes:
			ast[i] = NodeText(`(select * from some_table where col2 = '456') as _`)
		}
	}

	expected := `select * from (select * from some_table where col2 = '456') as _ where col1 = 123`
	actual := ast.String()
	if expected != actual {
		panic(fmt.Errorf(`expected serialized AST to be %q, got %q`, expected, actual))
	}
}

func TestWalkNode(_ *testing.T) {
	src := Nodes{
		NodeText(`one`),
		Nodes{NodeText(`two`), NodeOrdinalParam(3)},
		ParenNodes{NodeText(`four`)},
	}

	var visited Nodes
	WalkNode(src, func(val Node) {
		visited = append(visited, val)
	})
	eq(src, visited)
}

// TODO also test `DeepWalkNodePtr`.
func TestDeepWalkNode(_ *testing.T) {
	src := Nodes{
		NodeText(`one`),
		Nodes{NodeText(`two`), NodeOrdinalParam(3)},
		ParenNodes{NodeText(`four`)},
	}

	var visited Nodes
	DeepWalkNode(src, func(val Node) {
		visited = append(visited, val)
	})

	expected := Nodes{
		NodeText(`one`),
		NodeText(`two`),
		NodeOrdinalParam(3),
		NodeText(`four`),
	}
	eq(expected, visited)
}

func TestCopyNode(_ *testing.T) {
	src := Nodes{
		NodeText(`one`),
		Nodes{NodeText(`two`), NodeOrdinalParam(3)},
		ParenNodes{NodeText(`four`)},
	}

	srcBackup := Nodes{
		NodeText(`one`),
		Nodes{NodeText(`two`), NodeOrdinalParam(3)},
		ParenNodes{NodeText(`four`)},
	}

	copy := CopyNode(src)
	eq(src, copy)
	eq(srcBackup, copy)

	src[0] = nil
	src[1].(Nodes)[0] = nil
	src[1].(Nodes)[1] = nil
	src[2].(ParenNodes)[0] = nil

	expectedSrc := Nodes{nil, Nodes{nil, nil}, ParenNodes{nil}}
	eq(expectedSrc, src)
	eq(srcBackup, copy)
}

func try(err error) {
	if err != nil {
		panic(err)
	}
}

func eq(exp, act interface{}) {
	if !reflect.DeepEqual(exp, act) {
		panic(fmt.Errorf(`
expected (detailed):
	%#[1]v
actual (detailed):
	%#[2]v
expected (simple):
	%[1]s
actual (simple):
	%[2]s
`, exp, act))
	}
}
