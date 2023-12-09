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
			// Call optional
		})

		g.Define("test_literal", func() {
			g.Literal("example")
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
			t.Error("test case failed")
		}
	}

	parser, err = BuildParser(func(g *Grammar) {
		g.Start = "expr"
		g.Whitespaces = []string{" ", "\t"}
		g.Newlines = []string{"\r\n", "\r", "\n"}

		g.Define("expr", func() {
		//	g.Print("test")
			g.Sequence(func() {
				g.Choice(func() {
					g.Call("truerule")
				}, func() {
					g.Call("falserule")
				}, func() {
					g.Optional(func() {
						g.Literal("1")
					})
					g.Literal("2")
					g.Optional(func() {
						g.Literal("3")
					})
					g.Literal("4")
				})
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

		if !parser.testParse("true") {
			t.Error("didn't parse true")
		}

		if !parser.testParse("false") {
			t.Error("didn't parse false")
		}

		if parser.testParse("blue") {
			t.Error("shouldn't parse blue")
		}
		if !parser.testParse("24") {
			t.Error("didn't parse 24")
		}
		if !parser.testParse("234") {
			t.Error("didn't parse 234")
		}
		if !parser.testParse("124") {
			t.Error("didn't parse 124")
		}
		if !parser.testParse("1234") {
			t.Error("didn't parse 1234")
		}
	}
}
