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

	g = BuildGrammar(func(g *G) {})
	if g.Err == nil {
		t.Error("empty grammar should raise error")
	} else {
		t.Logf("test grammar raised error:\n %v", g.Err)
	}

	// start rule must exist
	g = BuildGrammar(func(g *G) {
		g.Start = "missing"
	})
	if g.Err == nil {
		t.Error("missing start should raise error")
	} else {
		t.Logf("test grammar raised error:\n %v", g.Err)
	}

	// all called rules must be defined
	g = BuildGrammar(func(g *G) {
		g.Start = "expr"

		g.Define("expr", func() {
			g.Call("missing")
		})
	})
	if g.Err == nil {
		t.Error("missing rule should raise error")
	} else {
		t.Logf("test grammar raised error:\n %v", g.Err)
	}

	// all defined rules must be called
	g = BuildGrammar(func(g *G) {
		g.Start = "expr"

		g.Define("expr", func() {
		})
		g.Define("expr2", func() {
		})
	})

	if g.Err == nil {
		t.Error("unused rule should raise error")
	} else {
		t.Logf("test grammar raised error:\n %v", g.Err)
	}

	// nested defines should fail
	g = BuildGrammar(func(g *G) {
		g.Start = "expr"

		g.Define("expr", func() {
			g.Define("expr2", func() {
			})
		})
	})

	if g.Err == nil {
		t.Error("nested define should raise error")
	} else {
		t.Logf("test grammar raised error:\n %v", g.Err)
	}
	// operators outside defines should fail
	g = BuildGrammar(func(g *G) {
		g.Start = "expr"

		g.Define("expr", func() {})
		g.String("true")
	})

	if g.Err == nil {
		t.Error("builder outside define should raise error")
	} else {
		t.Logf("test grammar raised error:\n %v", g.Err)
	}

	// invert must be called after Range
	g = BuildGrammar(func(g *G) {
		g.Start = "expr"

		g.Define("expr", func() {
			w := g.Whitespace()
			g.String("x")
			w.Min(1)
		})
	})

	if g.Err == nil {
		t.Error("bad post op should raise error")
	} else {
		t.Logf("test grammar raised error:\n %v", g.Err)
	}

	// range must be sensible
	g = BuildGrammar(func(g *G) {
		g.Start = "expr"

		g.Define("expr", func() {
			g.Rune().Range("9-0")
		})
	})

	if g.Err == nil {
		t.Error("bad range should raise error")
	} else {
		t.Logf("test grammar raised error:\n %v", g.Err)
	}

	// can't call whitespace inside Binary Mode
	g = BuildGrammar(func(g *G) {
		g.Mode = BinaryMode()
		g.Start = "expr"

		g.Define("expr", func() {
			g.Whitespace()
		})
	})

	if g.Err == nil {
		t.Error("whitespace should raise error")
	} else {
		t.Logf("test grammar raised error:\n %v", g.Err)
	}

	// can't call String(\t) inside text Mode
	g = BuildGrammar(func(g *G) {
		g.Start = "expr"

		g.Define("expr", func() {
			g.String("no tab\t")
		})
	})

	if g.Err == nil {
		t.Error("tab should raise error")
	} else {
		t.Logf("test grammar raised error:\n %v", g.Err)
	}

	// can't call byte inside text Mode
	g = BuildGrammar(func(g *G) {
		g.Start = "expr"

		g.Define("expr", func() {
			g.Byte()
		})
	})

	if g.Err == nil {
		t.Error("byte should raise error")
	} else {
		t.Logf("test grammar raised error:\n %v", g.Err)
	}

	// can't call String() with nonrune
	g = BuildGrammar(func(g *G) {
		g.Define("test", func() {
			g.String("\xFF")
		})
	})

	if g.Err == nil {
		t.Error("bad rune should raise error")
	} else {
		t.Logf("test grammar raised error:\n %v", g.Err)
	}

	// missing capture should raise error for builder
	g = BuildGrammar(func(g *G) {
		g.Start = "expr"

		g.Define("expr", func() {
			g.String("foo")
		})

		g.Builder("expr", func(string, []any) (any, error) {
			return nil, nil
		})
	})

	if g.Err == nil {
		t.Error("missing capture should raise error")
	} else {
		t.Logf("test grammar raised error:\n %v", g.Err)
	}
	// bad builder
	g = BuildGrammar(func(g *G) {
		g.Start = "expr"

		g.Define("expr", func() {
			g.Capture("expr", func() {
				g.String("foo")
			})
		})

		g.Builder("expr", func(int, []any) (any, error) {
			return nil, nil
		})
	})

	if g.Err == nil {
		t.Error("missing capture should raise error")
	} else {
		t.Logf("test grammar raised error:\n %v", g.Err)
	}

	// cut in wrong place

	g = BuildGrammar(func(g *G) {
		g.Start = "expr"

		g.Define("expr", func() {
			g.Cut()
		})
	})

	if g.Err == nil {
		t.Error("cut should raise error")
	} else {
		t.Logf("test grammar raised error:\n %v", g.Err)
	}
	// boo

	g = BuildGrammar(func(g *G) {
		g.Start = "expr"

		g.Define("expr", func() {
			g.Cut()
			g.Byte()
			g.String("\t")
		})
	})

	if g.Err == nil {
		t.Error("multiple should raise error")
	} else {
		t.Logf("test grammar raised error:\n %v", g.Err)
	}
}

func TestLogger(t *testing.T) {
	var parser *Parser
	var ok bool
	var logMessages int

	logMessages = 0

	parser = BuildParser(func(g *G) {
		g.Start = "expr"

		g.LogFunc = func(f string, o ...any) {
			t.Logf(f, o...)
			logMessages += 1
		}

		g.Define("expr", func() {
			g.Print("TEST")
			g.String("TEST")

		})
	})

	if parser.Err() != nil {
		t.Errorf("error defining grammar:\n%v", parser.Err())
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

	parser = BuildParser(func(g *G) {
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
			g.String("TEST")
		})
	})

	if parser.Err() != nil {
		t.Errorf("error defining grammar:\n%v", parser.Err())
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
	parser = BuildParser(func(g *G) {
		g.Mode = TextMode()
		g.Start = "start"
		g.Define("start", func() {
			g.Call("test_call")
			g.Call("test_choice")
			g.Call("test_cut")
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
			g.String("example")
		})
		g.Define("test_choice", func() {
			g.Choice(func() {
				g.String("a")
			}, func() {
				g.String("b")
			}, func() {
				g.String("c")
			})
		})
		g.Define("test_cut", func() {
			g.Choice(func() {
				g.Capture("a", func() {
					g.String("a")
					g.Cut()
				})
				g.String("1")
			}, func() {
				g.String("aa")
			})
		})
		g.Define("test_sequence", func() {
			g.Sequence(func() {
				g.String("a")
				g.String("b")
				g.String("c")
			})
		})
		g.Define("test_optional", func() {
			g.Optional().Do(func() {
				g.String("1")
			})
			g.String("2")
			g.Optional().Do(func() {
				g.String("3")
			})
			g.String("4")
		})
		g.Define("test_repeat", func() {
			g.Repeat().Min(1).Do(func() {
				g.String("a")
			})
		})
		g.Define("test_lookahead", func() {
			g.Lookahead(func() {
				g.String("a")
			})
			g.Rune()
		})
		g.Define("test_reject", func() {
			g.Reject(func() {
				g.String("a")
			})
			g.Rune()
		})
	})
	if parser.Err() != nil {
		t.Errorf("error defining grammar:\n%v", parser.Err())
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
		ok = parser.testRule("test_cut",
			[]string{"a1"},
			[]string{"", "aa"},
		)
		if !ok {
			t.Error("cut test case failed")
		}
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
			[]string{"a", "aa", "aaaa"},
			[]string{"", "ab", "ba", "aba"},
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
	parser = BuildParser(func(g *G) {
		g.Start = "expr"
		g.Mode = StringMode()
		g.Define("expr", func() {
			g.StartOfFile()
			g.WhitespaceNewline()
			g.String("example")
			g.WhitespaceNewline()
			g.EndOfFile()
		})

	})

	if parser.Err() != nil {
		t.Errorf("error defining grammar:\n%v", parser.Err())
	} else {
		ok = parser.testRule("expr",
			[]string{"example\n", "example \r", " example \r\n"},
			[]string{"", "example\n2", "1\nexample"},
		)
		if !ok {
			t.Error("StringMode test case failed")
		}
	}
	parser = BuildParser(func(g *G) {
		g.Mode = StringMode()
		g.Start = "start"
		g.Define("start", func() {
			g.Call("test_rune")
			g.Call("test_string")
			g.Call("test_range")
			g.Call("test_inverted_range")
			g.Call("test_match_rune")
		})

		g.Define("test_rune", func() {
			g.Rune()
		})

		g.Define("test_string", func() {
			g.String("example")
		})
		g.Define("test_range", func() {
			g.Rune().Range("0", "1-9")
		})
		g.Define("test_inverted_range", func() {
			g.Rune().Except("0-9")
		})
		g.Define("test_match_rune", func() {
			g.MatchRune(map[rune]func(){
				'1': func() {
					g.String("1")
				},
				'2': func() {
					g.String("22")
				},
				'3': func() {
					g.String("333")
				},
			})
		})
	})

	if parser.Err() != nil {
		t.Errorf("error defining grammar:\n%v", parser.Err())
	} else {
		ok = parser.testRule("test_string",
			[]string{"example"},
			[]string{"", "bad", "longer example", "example bad"},
		)
		if !ok {
			t.Error("string test case failed")
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
		ok = parser.testRule("test_match_rune",
			[]string{"1", "22", "333"},
			[]string{"111", "2", "33"},
		)
		if !ok {
			t.Error("match rune test case failed")
		}

	}
}
func TestBinaryMode(t *testing.T) {
	var parser *Parser
	var ok bool
	parser = BuildParser(func(g *G) {
		g.Mode = BinaryMode()
		g.Start = "start"

		g.Define("start", func() {
			g.Call("test_byte")
			g.Call("test_byterange")
			g.Call("test_inverted_byterange")
			g.Call("test_bytestring")
			g.Call("test_bytes")
			g.Call("test_match_byte")
		})

		g.Define("test_byte", func() {
			g.Byte()
		})
		g.Define("test_byterange", func() {
			g.Byte().Range("0-9")
		})
		g.Define("test_inverted_byterange", func() {
			g.Byte().Except("0-9")
		})
		g.Define("test_bytes", func() {
			g.Bytes([]byte("test"))
		})
		g.Define("test_bytestring", func() {
			g.ByteString("test")
		})
		g.Define("test_match_byte", func() {
			g.MatchByte(map[byte]func(){
				'1': func() {
					g.ByteString("1")
				},
				'2': func() {
					g.ByteString("22")
				},
				'3': func() {
					g.ByteString("333")
				},
			})
		})
	})
	if parser.Err() != nil {
		t.Errorf("error defining grammar:\n%v", parser.Err())
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
		ok = parser.testRule("test_match_byte",
			[]string{"1", "22", "333"},
			[]string{"111", "2", "33"},
		)
		if !ok {
			t.Error("match byte test case failed")
		}
	}

}

func TestTextMode(t *testing.T) {
	var parser *Parser
	var ok bool

	parser = BuildParser(func(g *G) {
		g.Start = "expr"
		g.Mode = TextMode().Tabstop(8)
		g.Define("expr", func() {
			g.StartOfLine()
			g.String("example")
			g.Newline()
			g.StartOfLine()
			g.EndOfFile()
		})

	})

	if parser.Err() != nil {
		t.Errorf("error defining grammar:\n%v", parser.Err())
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

	parser = BuildParser(func(g *G) {
		g.Start = "start"
		g.Define("start", func() {
			g.Capture("main", func() {
				g.String("A")
				g.Choice(func() {
					g.Capture("bcd", func() {
						g.String("BCD")
					})

				}, func() {
					g.Capture("b", func() {
						g.Capture("b2", func() {
							g.String("B")
						})
						g.Capture("c", func() {
							g.String("C")
						})
					})
				})
			})
		})
	})

	if parser.Err() != nil {
		t.Errorf("error defining grammar:\n%v", parser.Err())
	} else {
		ok = parser.testGrammar(
			[]string{"ABC", "ABCD"},
			[]string{""},
		)
		if !ok {
			t.Error("string test case failed")
		}

		tree, err = parser.ParseTree("ABC")

		if err != nil {
			t.Error("string test case failed")
		} else {
			t.Log("ABC parsed")
			tree.Walk(func(n *Node) {
				t.Logf("node %q %q %v, %v %v", n.name, tree.buf[n.start:n.end], n.children(tree), n.nchild, n.nsibling)
			})
			if len(tree.nodes) != 4 {
				t.Error("wrong nodes count")
			}
		}

		tree, err = parser.ParseTree("ABCD")

		if err != nil {
			t.Error("string test case failed")
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
	parser = BuildParser(func(g *G) {
		g.Start = "start"
		g.Define("start", func() {
			g.Capture("main", func() {
				g.String("A")
			})
		})
		g.Builder("main", func(s string, args []any) (any, error) {
			return &s, nil
		})
	})

	if parser.Err() != nil {
		t.Errorf("error defining grammar:\n%v", parser.Err())
	} else {
		ok = parser.testGrammar(
			[]string{"A"},
			[]string{""},
		)
		if !ok {
			t.Error("string test case failed")
		}

		tree, err = parser.ParseTree("A")

		if err != nil {
			t.Error("string test case failed")
		} else {
			t.Log("A parsed")
			tree.Walk(func(n *Node) {
				t.Logf("node %q %q %v, %v %v", n.name, tree.buf[n.start:n.end], n.children(tree), n.nchild, n.nsibling)
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

func TestBlockIndent(t *testing.T) {
	var parser *Parser
	var ok bool

	parser = BuildParser(func(g *G) {
		g.Start = "expr"
		g.Mode = TextMode().Tabstop(8)
		g.Define("expr", func() {
			g.Choice(func() {
				g.String("block:")
				g.Newline()
				g.IndentedBlock(func() {
					g.Repeat().Do(func() {
						g.Indent()
						g.Call("expr")
					})
				})
			}, func() {
				g.String("row")
				g.Newline()
			})
		})

	})

	if parser.Err() != nil {
		t.Errorf("error defining grammar:\n%v", parser.Err())
	} else {
		ok = parser.testRule("expr",
			[]string{}, // "block:\n row\n", "block:\n row\n row\n row\n", "block:\n block:\n  row\n"},
			[]string{
				"", "block:\nrow\n\n", "\n row",
				"block:\n row\n row\n  row\n",
				"block:\nblock:\n  row\n",
				"block:\nxrow\n",
			},
		)
		if !ok {
			t.Error("indent test case failed")
		}
	}
}

func TestOffsideIndent(t *testing.T) {
	var parser *Parser
	var ok bool

	parser = BuildParser(func(g *G) {
		g.Start = "expr"
		g.Mode = TextMode().Tabstop(8)
		g.Define("expr", func() {
			g.Choice(func() {
				g.Choice(func() {
					g.String("do")
				}, func() {
					g.String("let")
				})

				g.OffsideBlock(func() {
					g.Whitespace()
					g.Newline()
					g.Repeat().Do(func() {
						g.Indent()
						g.Call("expr")
					})
				})
			}, func() {
				g.String("row")
				g.Newline()
			})
		})

	})

	if parser.Err() != nil {
		t.Errorf("error defining grammar:\n%v", parser.Err())
	} else {
		ok = parser.testRule("expr",
			[]string{"row\n", "do\n  row\n", "let\n   row\n   row\n   row\n", "do\n  let\n     row\n"},
			[]string{"", "do\nrow\n", "do\nxxrow\n", "\nrow", "do\n  let\n    row\n"},
		)
		if !ok {
			t.Error("indent test case failed")
		}
	}
}

func TestTabStop(t *testing.T) {
	var parser *Parser
	var ok bool

	parser = BuildParser(func(g *G) {
		g.Start = "expr"
		g.Mode = TextMode().Tabstop(8)
		g.Define("expr", func() {
			g.Whitespace().Width(4)
			g.Whitespace().Width(5)
			g.String("hello")
		})

	})

	if parser.Err() != nil {
		t.Errorf("error defining grammar:\n%v", parser.Err())
	} else {
		ok = parser.testRule("expr",
			[]string{
				"\t hello",
				" \t hello",
				"  \t hello",
				"   \t hello",
				"    \t hello",
				"     \t hello",
				"      \t hello",
				"       \t hello",
				"         hello",
			},
			[]string{
				" hello",
				"\thello",
				"        \thello",
			},
		)
		if !ok {
			t.Error("tab test case failed")
		}
	}
}

var ok bool

func BenchmarkParser(b *testing.B) {
	var parser *Parser

	parser = BuildParser(func(g *G) {
		g.Define("expr", func() {
			g.Capture("e", func() {
				g.String("x")

				g.Optional().Do(func() {
					g.Call("expr")
				})
			})
		})
	})

	if parser.Err() != nil {
		b.Errorf("error defining grammar:\n%v", parser.Err())
	}
	x := "xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"
	x = x + x + x + x + x + x + x + x + x + x + x + x + x + x + x
	x = x + x + x + x + x + x + x + x + x + x + x + x + x + x + x

	b.ResetTimer()

	ok = parser.testGrammar(
		[]string{x, x, x},
		[]string{""},
	)
	if !ok {
		b.Error("print test case failed to parse")
	}
}
