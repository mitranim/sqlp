package sqlp

import (
	"reflect"
	"testing"
)

func TestParse(t *testing.T) {
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
		Nodes{NodeBrackets{NodeText(`one two`)}},
	)

	test(
		`one [two three]`,
		Nodes{NodeText(`one `), NodeBrackets{NodeText(`two three`)}},
	)

	test(
		`one [two three] four`,
		Nodes{NodeText(`one `), NodeBrackets{NodeText(`two three`)}, NodeText(` four`)},
	)

	test(
		`[one two] three`,
		Nodes{NodeBrackets{NodeText(`one two`)}, NodeText(` three`)},
	)

	test(
		`[one two] [three four]`,
		Nodes{NodeBrackets{NodeText(`one two`)}, NodeText(` `), NodeBrackets{NodeText(`three four`)}},
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
		Nodes{NodeBrackets{NodeBrackets{NodeParens{NodeBraces{NodeText(`one two`)}}}}},
	)

	test(
		"one -- two",
		Nodes{NodeText(`one `), NodeCommentLine(" two")},
	)

	test(
		"one -- two \n three",
		Nodes{NodeText(`one `), NodeCommentLine(" two \n"), NodeText(" three")},
	)

	test(
		`one::two :three`,
		Nodes{NodeText(`one`), NodeDoubleColon{}, NodeText(`two `), NodeNamedParam(`three`)},
	)

	test(
		`1 $2::int`,
		Nodes{NodeText(`1 `), NodeOrdinalParam(2), NodeDoubleColon{}, NodeText(`int`)},
	)

	test(
		`one = $1 and two = $2`,
		Nodes{NodeText(`one = `), NodeOrdinalParam(1), NodeText(` and two = `), NodeOrdinalParam(2)},
	)

	// For brevity.
	type T = NodeText
	type O = NodeOrdinalParam
	type N = NodeNamedParam
	type D = NodeDoubleColon

	test(
		`$1 $2::int :three::text`,
		Nodes{O(1), T(` `), O(2), D{}, T(`int `), N(`three`), D{}, T(`text`)},
	)

	test(
		`one = $1 and two = $1 and three = $2`,
		Nodes{T(`one = `), O(1), T(` and two = `), O(1), T(` and three = `), O(2)},
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
