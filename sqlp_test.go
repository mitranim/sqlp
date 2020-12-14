package sqlp

import (
	"reflect"
	"testing"
)

func TestParse(t *testing.T) {
	// For brevity.
	type (
		T = NodeText
		W = NodeWhitespace
		O = NodeOrdinalParam
		N = NodeNamedParam
		D = NodeDoubleColon
		P = NodeParens
		B = NodeBrackets
		C = NodeCommentLine
	)

	test := func(input string, astExpected Nodes) {
		ast, err := Parse(input)
		if err != nil {
			t.Fatalf("failed to parse source:\n%v\nerror:\n%+v", input, err)
		}

		if !reflect.DeepEqual(astExpected, ast) {
			t.Fatalf("expected parsed AST to be:\n%#v\ngot:\n%#v\n", astExpected, ast)
		}

		strExpected := input
		str := ast.String()
		if strExpected != str {
			t.Fatalf(`expected serialized AST to be %q, got %q`, strExpected, str)
		}
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
		Nodes{B{B{P{NodeBraces{T(`one`), W(` `), T(`two`)}}}}},
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

func TestRewrite(t *testing.T) {
	ast, err := Parse(`select * from [bracketed] where col1 = 123`)
	if err != nil {
		t.Fatalf("%+v", err)
	}

	for i, node := range ast {
		switch node.(type) {
		case NodeBrackets:
			ast[i] = NodeText(`(select * from some_table where col2 = '456') as _`)
		}
	}

	expected := `select * from (select * from some_table where col2 = '456') as _ where col1 = 123`
	actual := ast.String()
	if expected != actual {
		t.Fatalf(`expected serialized AST to be %q, got %q`, expected, actual)
	}
}

func TestTraverseShallow(t *testing.T) {
	nodes := Nodes{
		NodeText(`one`),
		Nodes{NodeText(`two`), NodeOrdinalParam(3)},
		NodeParens{NodeText(`four`)},
	}

	var visited Nodes

	err := TraverseShallow(nodes, func(ptr *Node) error {
		visited = append(visited, *ptr)
		return nil
	})
	if err != nil {
		t.Fatalf("%+v", err)
	}

	if !reflect.DeepEqual(nodes, visited) {
		t.Fatalf("expected traversed AST to be:\n%#v\ngot:\n%#v", nodes, visited)
	}
}

// TODO also test `NodeTraverseLeaves`.
func TestTraverseLeaves(t *testing.T) {
	nodes := Nodes{
		NodeText(`one`),
		Nodes{NodeText(`two`), NodeOrdinalParam(3)},
		NodeParens{NodeText(`four`)},
	}

	var visited Nodes

	err := TraverseLeaves(nodes, func(ptr *Node) error {
		visited = append(visited, *ptr)
		return nil
	})
	if err != nil {
		t.Fatalf("%+v", err)
	}

	expected := Nodes{
		NodeText(`one`),
		NodeText(`two`),
		NodeOrdinalParam(3),
		NodeText(`four`),
	}

	if !reflect.DeepEqual(expected, visited) {
		t.Fatalf("expected traversed AST to be:\n%#v\ngot:\n%#v", expected, visited)
	}
}

// TODO also test `NodeCopyDeep`.
func TestCopyDeep(t *testing.T) {
	source := Nodes{
		NodeText(`one`),
		Nodes{NodeText(`two`), NodeOrdinalParam(3)},
		NodeParens{NodeText(`four`)},
	}

	copy := CopyDeep(source)

	if !reflect.DeepEqual(source, copy) {
		t.Fatalf(`expected source and copy to be identical; source: %#v; copy %#v`, source, copy)
	}

	// Also test the ability to deeply mutate the tree via leaf traversal. TODO
	// move this to the appropriate test.
	err := TraverseLeaves(source, func(ptr *Node) error {
		*ptr = nil
		return nil
	})
	if err != nil {
		t.Fatalf("%+v", err)
	}

	expectedSource := Nodes{nil, Nodes{nil, nil}, NodeParens{nil}}
	if !reflect.DeepEqual(expectedSource, source) {
		t.Fatalf(`expected source nodes to become %#v, got %#v`, expectedSource, source)
	}

	expectedCopy := Nodes{
		NodeText(`one`),
		Nodes{NodeText(`two`), NodeOrdinalParam(3)},
		NodeParens{NodeText(`four`)},
	}

	if !reflect.DeepEqual(expectedCopy, copy) {
		t.Fatalf(`expected copy nodes to remain: %#v, got %#v`, expectedCopy, copy)
	}
}
