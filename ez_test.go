package ez

import (
	"testing"
)

// t.Log(...) / t.Logf("%v", err)
// t.Error(...) Errorf,  mark fail and continue
// t.Fatal(...) FatalF,  mark fail, exit

func TestParser(t *testing.T) {
	parser, err := BuildParser(func(g *Grammar) {
		g.Start = "expr"
		g.Whitespace = []string{" ", "\t"}
		g.Newline = []string{"\r\n", "\r", "\n"}

		g.Define("expr", func() {
			g.Choice(func() {
				g.Call("true rule")
				g.Optional(func() {
					g.Literal("y")
				})
			}, func() {
				g.Call("false rule")
			})
		})

		g.Define("true rule", func() {
			g.Literal("true")
		})

		g.Define("false rule", func() {
			g.Literal("false")
		})
	})

	if err != nil {
		t.Fatalf("err\n%v", err)
	}

	if !parser.Accept("true") {
		t.Error("didn't parse true")
	}

	if !parser.Accept("false") {
		t.Error("didn't parse false")
	}

	if parser.Accept("blue") {
		t.Error("shouldn't parse blue")
	}
	if !parser.Accept("truey") {
		t.Error("didn't parse truey")
	}
}
