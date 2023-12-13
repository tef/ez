package json

import (
	"testing"

	"ez"
)

func TestJson(t *testing.T) {
	if JsonParser.Err() != nil {
		t.Error("error", JsonParser.Err())
	}

	tree, err := JsonParser.ParseTree("[1,2,3]")

	if err != nil {
		t.Error("bad json parse: ", err)
	} else {
		tree.Walk(func(n *ez.Node) {
			t.Logf("node %q ", n)
		})
	}

	out, err := JsonParser.Parse("[1,2,3]")

	if err != nil {
		t.Error("bad json parse: ", err)
	} else {
		t.Logf("Output: %v", out)
	}

	out2, err := JsonParser.Parse(`{"A":1}`)

	if err != nil {
		t.Error("bad json parse: ", err)
	} else {
		t.Logf("Output: %v", out2)
	}
}
