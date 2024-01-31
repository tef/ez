package infix

import (
	"testing"

	"ez"
)

func TestInfix(t *testing.T) {

	var out *ez.ParseTree
	var err error

	if InfixParser.Err() != nil {
		t.Fatal("error", InfixParser.Err())
	}

	out, err = InfixParser.ParseTree("1")

	if err != nil {
		t.Error("bad infix parse: ", err)
	} else {
		out.Walk(func(n *ez.Node) {
			t.Logf("node %q ", n)
		})
	}

	t.Log("----")

	out, err = InfixParser.ParseTree("1+2")

	if err != nil {
		t.Error("bad infix parse 1+2: ", err)
	} else {
		t.Logf("Output: %v", out)
	}

	t.Log("----")
	out, err = InfixParser.ParseTree(`1+2+3`)

	if err != nil {
		t.Error("bad infix parse 1+2+3: ", err)
	} else {
		t.Logf("Output: %v", out)
	}
	t.Log("----")

	out, err = InfixParser.ParseTree(`1=2=3`)

	if err != nil {
		t.Error("bad infix parse 1+2+3: ", err)
	} else {
		t.Logf("Output: %v", out)
	}

	out, err = InfixParser.ParseTree(`1+2=3=4+5+6`)

	if err != nil {
		t.Error("bad infix parse 1+2+3: ", err)
	} else {
		t.Logf("Output: %v", out)
	}
}
