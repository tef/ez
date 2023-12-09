package ez

import (
	"testing"
)

// t.Log(...) / t.Logf("%v", err)
// t.Error(...) Errorf,  mark fail and continue
// t.Fatal(...) FatalF,  mark fail, exit

func TestErrors(t *testing.T) {
	var err error

	// grammars need a start and one rule

	_, err = BuildGrammar(func(g *Grammar) {})
	if err == nil {
		t.Error("empty grammar should raise error")
	} else {
		t.Logf("test grammar raised error:\n %v", err)
	}

	// start rule must exist
	_, err = BuildGrammar(func(g *Grammar) {
		g.Start = "missing"
	})
	if err == nil {
		t.Error("missing start should raise error")
	} else {
		t.Logf("test grammar raised error:\n %v", err)
	}

	// all called rules must be defined
	_, err = BuildGrammar(func(g *Grammar) {
		g.Start = "expr"

		g.Define("expr", func() {
			g.Call("missing")
		})
	})
	if err == nil {
		t.Error("missing rule should raise error")
	} else {
		t.Logf("test grammar raised error:\n %v", err)
	}

	// all defined rules must be called
	_, err = BuildGrammar(func(g *Grammar) {
		g.Start = "expr"

		g.Define("expr", func() {
		})
		g.Define("expr2", func() {
		})
	})

	if err == nil {
		t.Error("unused rule should raise error")
	} else {
		t.Logf("test grammar raised error:\n %v", err)
	}

	// nested defines should fail
	_, err = BuildGrammar(func(g *Grammar) {
		g.Start = "expr"

		g.Define("expr", func() {
			g.Define("expr2", func() {
			})
		})
	})

	if err == nil {
		t.Error("nested define should raise error")
	} else {
		t.Logf("test grammar raised error:\n %v", err)
	}
	// operators outside defines should fail
	_, err = BuildGrammar(func(g *Grammar) {
		g.Start = "expr"

		g.Define("expr", func() {})
		g.Literal("true")
	})

	if err == nil {
		t.Error("builder outside define should raise error")
	} else {
		t.Logf("test grammar raised error:\n %v", err)
	}

	// calling builders outside should fail
	g, err := BuildGrammar(func(g *Grammar) {
		g.Start = "expr"

		g.Define("expr", func() {})
	})
	g.Define("expr2", func() {})

	if g.err == nil {
		t.Error("define should raise error")
	} else {
		t.Logf("test grammar raised error:\n %v", g.err)
	}
	// calling builders outside should fail
	g = &Grammar{}
	g.Define("expr2", func() {})

	if g.err == nil {
		t.Error("define should raise error")
	} else {
		t.Logf("test grammar raised error:\n %v", g.err)
	}

}

func TestParser(t *testing.T) {
	var parser *Parser
	var err error
	var ok bool

	parser, err = BuildParser(func(g *Grammar) {
		g.Start = "start"
		g.Define("start", func() {
			g.Call("test_literal")
			g.Call("test_optional")
			g.Call("test_range")
			// Call optional
		})

		g.Define("test_literal", func() {
			g.Literal("example")
		})
		g.Define("test_optional", func() {
			g.Optional(func() {
				g.Literal("1")
			})
			g.Literal("2")
			g.Optional(func() {
				g.Literal("3")
			})
			g.Literal("4")
		})
		g.Define("test_range", func() {
			g.Range("0-9")
		})

		// test optional
	})

	if err != nil {
		t.Errorf("error defining grammar:\n%v", err)
	} else {
		ok = parser.testRule("test_literal",
			[]string{"example"},
			[]string{"", "bad", "longer example", "example bad"},
		)
		if !ok {
			t.Error("literal test case failed")
		}
		ok = parser.testRule("test_optional",
			[]string{"24", "124", "234", "1234"},
			[]string{"", "1", "34", "23", "123"},
		)
		if !ok {
			t.Error("optional test case failed")
		}
		ok = parser.testRule("test_range",
			[]string{"0", "1", "2", "3", "4", "5", "6", "7", "8", "9"},
			[]string{"", "00", "a0", "0a", "a0a"},
		)
		if !ok {
			t.Error("range test case failed")
		}
	}

	parser, err = BuildParser(func(g *Grammar) {
		g.Start = "expr"
		g.Whitespaces = []string{" ", "\t"}
		g.Newlines = []string{"\r\n", "\r", "\n"}

		g.Define("expr", func() {
			g.Choice(func() {
				//	g.Print("test")
				g.Call("truerule")
			}, func() {
				g.Call("falserule")
			})
		})

		g.Define("truerule", func() {
			g.Literal("true")
		})

		g.Define("falserule", func() {
			g.Literal("false")
		})
	})

	if err != nil {
		t.Errorf("error defining grammar:\n%v", err)
	} else {
		ok = parser.testRule("test_optional",
			[]string{"true", "false"},
			[]string{"", "true1", "0false", "null"},
		)
		if !ok {
			t.Error("rules test case failed")
		}
	}
}
