package json

import (
	"ez"
)

var JsonParser = ez.BuildParser(func(g *ez.Grammar) {
	g.Mode = ez.StringMode()
	g.Start = "document"

	g.Define("document", func() {
		g.Whitespace()
		g.Lookahead(func() {
			g.Literal("{", "[")
		})
		g.Call("value")
		g.Whitespace()
	})

	g.Define("value", func() {
		g.Choice(func() {
			g.Call("list")
		}, func() {
			g.Call("object")
		}, func() {
			g.Call("string")
		}, func() {
			g.Call("number")
		}, func() {
			g.Capture("true", func() {
				g.Literal("true")
			})
		}, func() {
			g.Capture("false", func() {
				g.Literal("false")
			})
		}, func() {
			g.Capture("null", func() {
				g.Literal("null")
			})
		})

	})

	g.Define("list", func() {
		g.Literal("[")
		g.Whitespace()
		g.Capture("list", func() {
			g.Optional(func() {
				g.Call("value")
				g.Repeat(0, 0, func() {
					g.Whitespace()
					g.Literal(",")
					g.Whitespace()
					g.Call("value")
				})
			})
		})
		g.Literal("]")

	})

	g.Define("object", func() {
		g.Literal("{")
		g.Whitespace()
		g.Capture("object", func() {
			g.Optional(func() {
				g.Call("string")
				g.Whitespace()
				g.Literal(":")
				g.Whitespace()
				g.Call("value")
			})
			g.Whitespace()
			g.Repeat(0, 0, func() {
				g.Literal(",")
				g.Whitespace()
				g.Call("string")
				g.Whitespace()
				g.Literal(":")
				g.Whitespace()
				g.Call("value")
				g.Whitespace()
			})
		})
		g.Literal("}")
	})

	g.Define("string", func() {
		g.Literal("\"")
		g.Capture("string", func() {
			g.Repeat(0, 0, func() {
				g.Choice(func() {
					g.Literal("\\u")
					g.Cut()
					g.Range("0-9", "a-f", "A-F")
					g.Range("0-9", "a-f", "A-F")
					g.Range("0-9", "a-f", "A-F")
					g.Range("0-9", "a-f", "A-F")
				}, func() {
					g.Literal("\\")
					g.Cut()
					g.Literal(
						"\"", "\\", "/", "b",
						"f", "n", "r", "t",
					)
				}, func() {
					g.Reject(func() {
						g.Literal("\\", "\"")
					})
					g.Rune()
				})
			})
		})
		g.Literal("\"")
	})

	g.Define("number", func() {
		g.Capture("number", func() {
			g.Optional(func() {
				g.Literal("-")
			})
			g.Choice(func() {
				g.Literal("0")
			}, func() {
				g.Range("1-9")
				g.Repeat(0, 0, func() {
					g.Range("0-9")
				})
			})
			g.Optional(func() {
				g.Literal(".")
				g.Repeat(0, 0, func() {
					g.Range("0-9")
				})
			})
			g.Optional(func() {
				g.Literal("e", "E")
				g.Optional(func() {
					g.Literal("+", "-")
					g.Repeat(0, 0, func() {
						g.Range("0-9")
					})
				})
			})
		})
	})
	g.Builder("list", func(s string, args []any) (any, error) {
		return args, nil
	})
	g.Builder("object", func(s string, args []any) (any, error) {
		l := len(args)
		m := make(map[string]any, l/2)
		for c := 0; c < l; c += 2 {
			key := args[c].(*string)
			value := args[c+1]
			m[*key] = value
		}
		return m, nil
	})

	g.Builder("string", func(s string, args []any) (any, error) {
		return &s, nil
	})
	g.Builder("number", func(s string, args []any) (any, error) {
		return &s, nil
	})
	g.Builder("true", func(s string, args []any) (any, error) {
		v := true
		return &v, nil
	})
	g.Builder("false", func(s string, args []any) (any, error) {
		v := false
		return &v, nil
	})
	g.Builder("null", func(s string, args []any) (any, error) {
		return nil, nil
	})
})
