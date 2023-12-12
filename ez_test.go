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

	// range must be sensible
	g = BuildGrammar(func(g *Grammar) {
		g.Start = "expr"

		g.Define("expr", func() {
			g.Range("9-0")
		})
	})

	if g.err == nil {
		t.Error("bad range should raise error")
	} else {
		t.Logf("test grammar raised error:\n %v", g.err)
	}

	// can't call whitespace inside Binary Mode
	g = BuildGrammar(func(g *Grammar) {
		g.Mode = BinaryMode()
		g.Start = "expr"

		g.Define("expr", func() {
			g.Whitespace()
		})
	})

	if g.err == nil {
		t.Error("whitespace should raise error")
	} else {
		t.Logf("test grammar raised error:\n %v", g.err)
	}

	// can't call byte inside text Mode
	g = BuildGrammar(func(g *Grammar) {
		g.Start = "expr"

		g.Define("expr", func() {
			g.Byte()
		})
	})

	if g.err == nil {
		t.Error("byte should raise error")
	} else {
		t.Logf("test grammar raised error:\n %v", g.err)
	}

	// missing capture should raise error for builder
	g = BuildGrammar(func(g *Grammar) {
		g.Start = "expr"

		g.Define("expr", func() {
			g.Literal("foo")
		})

		g.Builder("expr", func(string, []any) (any, error) {
			return nil, nil
		})
	})

	if g.err == nil {
		t.Error("missing capture should raise error")
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
		g.Mode = TextMode()
		g.Start = "start"
		g.Define("start", func() {
			g.Call("test_call")
			g.Call("test_choice")
			// g.Call("test_cut")
			g.Call("test_sequence")
			g.Call("test_optional")
			g.Call("test_repeat")
			g.Call("test_lookahead")
			g.Call("test_reject")
		})
		g.Define("test_call", func() {
			g.Call("example")
		})
		g.Define("example", func() {
			g.Literal("example")
		})
		g.Define("test_choice", func() {
			g.Choice(func() {
				g.Literal("a")
			}, func() {
				g.Literal("b")
			}, func() {
				g.Literal("c")
			})
		})
		/*
			g.Define("test_cut", func() {
				g.Trace(func(){
				g.Choice(func() {
					g.Literal("a")
					g.Print()
					g.Cut()
					g.Cut()
					g.Literal("1")
				}, func() {
					g.Literal("aa")
				})
				})
			})
		*/
		g.Define("test_sequence", func() {
			g.Sequence(func() {
				g.Literal("a")
				g.Literal("b")
				g.Literal("c")
			})
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
		g.Define("test_repeat", func() {
			g.Repeat(0, 0, func() {
				g.Literal("a")
			})
		})
		g.Define("test_lookahead", func() {
			g.Lookahead(func() {
				g.Literal("a")
			})
			g.Rune()
		})
		g.Define("test_reject", func() {
			g.Reject(func() {
				g.Literal("a")
			})
			g.Rune()
		})
	})
	if parser.err != nil {
		t.Errorf("error defining grammar:\n%v", parser.err)
	} else {
		ok = parser.testRule("test_call",
			[]string{"example"},
			[]string{"failure"},
		)
		if !ok {
			t.Error("call test case failed")
		}
		ok = parser.testRule("test_choice",
			[]string{"a", "b", "c"},
			[]string{"", "d"},
		)
		if !ok {
			t.Error("choice test case failed")
		}
		/*
			ok = parser.testRule("test_cut",
				[]string{"a1"},
				[]string{"", "aa"},
			)
			if !ok {
				t.Error("cut test case failed")
			}
		*/
		ok = parser.testRule("test_sequence",
			[]string{"abc"},
			[]string{"", "a", "ab", "abcd"},
		)
		if !ok {
			t.Error("sequence test case failed")
		}
		ok = parser.testRule("test_optional",
			[]string{"24", "124", "234", "1234"},
			[]string{"", "1", "34", "23", "123"},
		)
		if !ok {
			t.Error("optional test case failed")
		}
		ok = parser.testRule("test_repeat",
			[]string{"", "a", "aa", "aaaa"},
			[]string{"ab", "ba", "aba"},
		)
		if !ok {
			t.Error("repeat test case failed")
		}
		ok = parser.testRule("test_lookahead",
			[]string{"a"},
			[]string{"", "b"},
		)
		if !ok {
			t.Error("lookahead test case failed")
		}
		ok = parser.testRule("test_reject",
			[]string{"b"},
			[]string{"", "a"},
		)
		if !ok {
			t.Error("reject test case failed")
		}
	}
}

func TestStringMode(t *testing.T) {
	var parser *Parser
	var ok bool
	parser = BuildParser(func(g *Grammar) {
		g.Start = "expr"
		g.Mode = StringMode()
		g.Define("expr", func() {
			g.Whitespace()
			g.Literal("example")
			g.Whitespace()
			g.EndOfFile()
		})

	})

	if parser.err != nil {
		t.Errorf("error defining grammar:\n%v", parser.err)
	} else {
		ok = parser.testRule("expr",
			[]string{"example\n", "example \r", " example \r\n"},
			[]string{"", "example\n2", "1\nexample"},
		)
		if !ok {
			t.Error("StringMode test case failed")
		}
	}
	parser = BuildParser(func(g *Grammar) {
		g.Mode = StringMode()
		g.Start = "start"
		g.Define("start", func() {
			g.Call("test_rune")
			g.Call("test_literal")
			g.Call("test_range")
			g.Call("test_inverted_range")
		})

		g.Define("test_rune", func() {
			g.Rune()
		})

		g.Define("test_literal", func() {
			g.Literal("example")
		})
		g.Define("test_range", func() {
			g.Range("0-9")
		})
		g.Define("test_inverted_range", func() {
			g.Range("0-9").Invert()
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

	}
}
func TestBinaryMode(t *testing.T) {
	var parser *Parser
	var ok bool
	parser = BuildParser(func(g *Grammar) {
		g.Mode = BinaryMode()
		g.Start = "start"

		g.Define("start", func() {
			g.Call("test_byte")
			g.Call("test_byterange")
			g.Call("test_inverted_byterange")
			g.Call("test_bytestring")
			g.Call("test_bytes")
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
		g.Define("test_bytes", func() {
			g.Bytes([]byte("test"))
		})
		g.Define("test_bytestring", func() {
			g.ByteString("test")
		})
	})
	if parser.err != nil {
		t.Errorf("error defining grammar:\n%v", parser.err)
	} else {
		ok = parser.testRule("test_byte",
			[]string{"a", "A"},
			[]string{"", "aa"},
		)
		if !ok {
			t.Error("byte test case failed")
		}
		ok = parser.testRule("test_byterange",
			[]string{"0", "1", "2", "3", "4", "5", "6", "7", "8", "9"},
			[]string{"", "00", "a0", "0a", "a0a"},
		)
		if !ok {
			t.Error("byterange test case failed")
		}
		ok = parser.testRule("test_inverted_byterange",
			[]string{"a", "b", "c", "A", "B", "C"},
			[]string{"", "0", "1", "2", "3", "4", "5", "6", "7", "8", "9", "10"},
		)
		if !ok {
			t.Error("inverted byterange test case failed")
		}
		ok = parser.testRule("test_bytes",
			[]string{"test"},
			[]string{"", "aaa"},
		)
		if !ok {
			t.Error("bytes test case failed")
		}
		ok = parser.testRule("test_bytestring",
			[]string{"test"},
			[]string{"", "aa"},
		)
		if !ok {
			t.Error("bytestring test case failed")
		}
	}

}

func TestTextMode(t *testing.T) {
	var parser *Parser
	var ok bool

	parser = BuildParser(func(g *Grammar) {
		g.Start = "expr"
		g.Mode = TextMode()
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
