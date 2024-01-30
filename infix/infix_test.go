package infix

import (
	"testing"

	"ez"
)

func TestInfix(t *testing.T) {
	if InfixParser.Err() != nil {
		t.Fatal("error", InfixParser.Err())
	}

	tree, err := InfixParser.ParseTree("1")

	if err != nil {
		t.Error("bad infix parse: ", err)
	} else {
		tree.Walk(func(n *ez.Node) {
			t.Logf("node %q ", n)
		})
	}

	t.Log("----")

	out, err := InfixParser.ParseTree("1+2")

	if err != nil {
		t.Error("bad infix parse 1+2: ", err)
	} else {
		t.Logf("Output: %v", out)
	}

	t.Log("----")
	out2, err := InfixParser.ParseTree(`1+2+3`)

	if err != nil {
		t.Error("bad infix parse 1+2+3: ", err)
	} else {
		t.Logf("Output: %v", out2)
	}
}
