package ez

import (
	"testing"
)

// t.Log(...) / t.Logf("%v", err)
// t.Error(...) Errorf,  mark fail and continue
// t.Fatal(...) FatalF,  mark fail, exit

func TestErrors(t *testing.T) {
	var g *Grammar

	// grammars need a start and one rule

	g = BuildGrammar(func(g *Grammar) {})
	if g.err == nil {
		t.Error("empty grammar should raise error")
	} else {
		t.Logf("test grammar raised error:\n %v", g.err)
	}

	// start rule must exist
	g = BuildGrammar(func(g *Grammar) {
		g.Start = "missing"
	})
	if g.err == nil {
		t.Error("missing start should raise error")
	} else {
		t.Logf("test grammar raised error:\n %v", g.err)
	}

	// all called rules must be defined
	g = BuildGrammar(func(g *Grammar) {
		g.Start = "expr"

		g.Define("expr", func() {
			g.Call("missing")
		})
	})
	if g.err == nil {
		t.Error("missing rule should raise error")
	} else {
		t.Logf("test grammar raised error:\n %v", g.err)
	}

	// all defined rules must be called
	g = BuildGrammar(func(g *Grammar) {
		g.Start = "expr"

		g.Define("expr", func() {
		})
		g.Define("expr2", func() {
		})
	})

	if g.err == nil {
		t.Error("unused rule should raise error")
	} else {
		t.Logf("test grammar raised error:\n %v", g.err)
	}

	// nested defines should fail
	g = BuildGrammar(func(g *Grammar) {
		g.Start = "expr"

		g.Define("expr", func() {
			g.Define("expr2", func() {
			})
		})
	})

	if g.err == nil {
		t.Error("nested define should raise error")
	} else {
		t.Logf("test grammar raised error:\n %v", g.err)
	}
	// operators outside defines should fail
	g = BuildGrammar(func(g *Grammar) {
		g.Start = "expr"

		g.Define("expr", func() {})
		g.Literal("true")
	})

	if g.err == nil {
		t.Error("builder outside define should raise error")
	} else {
		t.Logf("test grammar raised error:\n %v", g.err)
	}

	// calling builders outside should fail
	g = BuildGrammar(func(g *Grammar) {
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
	// invert must be called after Range
	g = BuildGrammar(func(g *Grammar) {
		g.Start = "expr"

		g.Define("expr", func() {
			ro := g.Range("0-9")
			g.Literal("x")
			ro.Invert()
		})
	})

	if g.err == nil {
		t.Error("bad invert should raise error")
	} else {
		t.Logf("test grammar raised error:\n %v", g.err)
	}

}
func TestLogger(t *testing.T) {
	var parser *Parser
	var ok bool
	var logMessages int

	logMessages = 0

	parser = BuildParser(func(g *Grammar) {
		g.Start = "expr"

		g.LogFunc = func(f string, o ...any) {
			t.Logf(f, o...)
			logMessages += 1
		}

		g.Define("expr", func() {
			g.Print("TEST")
			g.Literal("TEST")

		})
	})

	if parser.err != nil {
		t.Errorf("error defining grammar:\n%v", parser.err)
	} else {
		ok = parser.testGrammar(
			[]string{"TEST"},
			[]string{""},
		)
		if !ok {
			t.Error("print test case failed to parse")
		}
		if logMessages < 2 { // two tests above
			t.Error("print test case failed to log")
		}

	}

	logMessages = 0

	parser = BuildParser(func(g *Grammar) {
		g.Start = "expr"

		g.LogFunc = func(f string, o ...any) {
			t.Logf(f, o...)
			logMessages += 1
		}

		g.Define("expr", func() {
			g.Trace(func() {
				g.Call("test")
			})
		})
		g.Define("test", func() {
			g.Literal("TEST")
		})
	})

	if parser.err != nil {
		t.Errorf("error defining grammar:\n%v", parser.err)
	} else {
		ok = parser.testGrammar(
			[]string{"TEST"},
			[]string{""},
		)
		if !ok {
			t.Error("trace test case failed to parse")
		}
		if logMessages < 2 { // two tests above * two trace messages (enter, exit)
			t.Error("trace test case failed to log")
		}

	}

}
func TestParser(t *testing.T) {
	var parser *Parser
	var ok bool

	parser = BuildParser(func(g *Grammar) {
		g.Start = "start"
		g.Define("start", func() {
			g.Call("test_literal")
			g.Call("test_optional")
			g.Call("test_range")
			g.Call("test_inverted_range")
			g.Call("test_rune")
			g.Call("test_byte")
			g.Call("test_byterange")
			g.Call("test_inverted_byterange")
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
		g.Define("test_inverted_range", func() {
			g.Range("0-9").Invert()
		})
		g.Define("test_rune", func() {
			g.Rune()
		})
		g.Define("test_byte", func() {
			g.Byte()
		})
		g.Define("test_byterange", func() {
			g.ByteRange("0-9")
		})
		g.Define("test_inverted_byterange", func() {
			g.ByteRange("0-9").Invert()
		})
	})

	if parser.err != nil {
		t.Errorf("error defining grammar:\n%v", parser.err)
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
		ok = parser.testRule("test_inverted_range",
			[]string{"a", "b", "c", "A", "B", "C"},
			[]string{"", "0", "1", "2", "3", "4", "5", "6", "7", "8", "9", "10"},
		)
		if !ok {
			t.Error("inverted range test case failed")
		}
		ok = parser.testRule("test_rune",
			[]string{"a", "A"},
			[]string{"", "aa"},
		)
		if !ok {
			t.Error("rune test case failed")
		}

		ok = parser.testRule("test_byte",
			[]string{"a", "A"},
			[]string{"", "aa"},
		)
		if !ok {
			t.Error("byte test case failed")
		}
	}
}

func TestWhitespace(t *testing.T) {
	var parser *Parser
	var ok bool

	parser = BuildParser(func(g *Grammar) {
		g.Start = "expr"
		g.Whitespaces = []string{" ", "\t"}
		g.Newlines = []string{"\r\n", "\r", "\n"}

		g.Define("expr", func() {
			g.StartOfLine()
			g.Literal("example")
			g.Newline()
			g.StartOfLine()
			g.EndOfFile()
		})

	})

	if parser.err != nil {
		t.Errorf("error defining grammar:\n%v", parser.err)
	} else {
		ok = parser.testRule("expr",
			[]string{"example\n", "example\r", "example\r\n"},
			[]string{"", "example\n\n", "\nexample"},
		)
		if !ok {
			t.Error("column test case failed")
		}
	}
}

func TestCapture(t *testing.T) {
	var parser *Parser
	var ok bool
	var tree *ParseTree
	var err error

	parser = BuildParser(func(g *Grammar) {
		g.Start = "start"
		g.Define("start", func() {
			g.Capture("main", func() {
				g.Literal("A")
				g.Choice(func() {
					g.Capture("bcd", func() {
						g.Literal("BCD")
					})

				}, func() {
					g.Capture("b", func() {
						g.Literal("B")
						g.Capture("c", func() {
							g.Literal("C")
						})
					})
				})
			})
		})
	})

	if parser.err != nil {
		t.Errorf("error defining grammar:\n%v", parser.err)
	} else {
		ok = parser.testGrammar(
			[]string{"ABC", "ABCD"},
			[]string{""},
		)
		if !ok {
			t.Error("literal test case failed")
		}

		tree, err = parser.ParseTree("ABC")

		if err != nil {
			t.Error("literal test case failed")
		} else {
			t.Log("ABC parsed")
			tree.Walk(func(n *Node) {
				t.Logf("node %q %q %v", n.name, tree.buf[n.start:n.end], n.children)
			})
			if len(tree.nodes) != 3 {
				t.Error("wrong nodes count")
			}
		}

		tree, err = parser.ParseTree("ABCD")

		if err != nil {
			t.Error("literal test case failed")
		} else {
			t.Log("ABCD parsed")
			tree.Walk(func(n *Node) {
				t.Logf("node %q %q", n.name, tree.buf[n.start:n.end])
			})
			if len(tree.nodes) != 2 {
				t.Error("wrong node count")
			}
		}
	}
	parser = BuildParser(func(g *Grammar) {
		g.Start = "start"
		g.Define("start", func() {
			g.Capture("main", func() {
				g.Literal("A")
			})
		})
		g.Builder("main", func(s string, args []any) (any, error) {
			return &s, nil
		})
	})

	if parser.err != nil {
		t.Errorf("error defining grammar:\n%v", parser.err)
	} else {
		ok = parser.testGrammar(
			[]string{"A"},
			[]string{""},
		)
		if !ok {
			t.Error("literal test case failed")
		}

		tree, err = parser.ParseTree("A")

		if err != nil {
			t.Error("literal test case failed")
		} else {
			t.Log("A parsed")
			tree.Walk(func(n *Node) {
				t.Logf("node %q %q %v", n.name, tree.buf[n.start:n.end], n.children)
			})
			if len(tree.nodes) != 1 {
				t.Error("wrong nodes count")
			}
		}

		t.Logf("builders %v", parser.builders)
		out, err := parser.Parse("A")

		if err != nil || out == nil {
			t.Error("build failed")
		} else {
			s, ok := out.(*string)
			if ok && *s == "A" {
				t.Log("build success")
			} else {
				t.Errorf("build failed, got %v:", out)
			}
		}
	}
}
