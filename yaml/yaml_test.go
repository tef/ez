package yaml

import (
	"fmt"
	"testing"

	"ez"
)

func TestYaml(t *testing.T) {
	var out any
	var err error

	if YamlParser.Err() != nil {
		t.Fatal("error", YamlParser.Err())
	}

	tree, err := YamlParser.ParseTree("[1,2,3]")

	if err != nil {
		t.Error("bad yaml parse:", err)
	} else {
		tree.Walk(func(n *ez.Node) {
			t.Logf("node %q ", n)
		})
	}

	out, err = YamlParser.Parse("[1,2,3]")

	if err != nil {
		t.Error("bad yaml parse: ", err)
	} else {
		t.Logf("Output: %v", out)
	}

	out, err = YamlParser.Parse(`{"A":1}`)

	if err != nil {
		t.Error("bad yaml parse: ", err)
	} else {
		t.Logf("Output: %v", out)
	}

	out, err = YamlParser.Parse(`a: 1`)

	if err != nil {
		t.Error("bad yaml parse: ", err)
	} else {
		t.Logf("Output: %v", out)
	}

	fmt.Println("hey")

	out, err = YamlParser.Parse("- 1\n- 2\n- 3\n")

	if err != nil {
		t.Error("bad yaml parse: ", err)
	} else {
		t.Logf("Output: %v", out)
	}

	y := `
- 1
- 2
- 
 - 3
 - 4
`

	out, err = YamlParser.Parse(y)

	if err != nil {
		t.Error("bad yaml parse: ", err)
	} else {
		t.Logf("Output: %v", out)
	}

}
