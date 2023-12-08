package ez

import (
	"testing"
)

func TestParser(t *testing.T) {
	// Log Error Fatal
	parser, err := BuildParser(func(g *Grammar) {
		g.Start = "expr"
		g.Whitespace = []string{" ", "\t"}
		g.Newline = []string{"\r\n", "\r", "\n"}

		g.Define("expr", func() {
			g.Choice(func() {
				g.Call("truerule")
				g.Optional(func() {
					g.Literal("y")
				})
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
		t.Fatalf("err: %v", err)
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
