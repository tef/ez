package json

import (
	"ez"
)

var JsonParser = ez.BuildParser(func(g *ez.G) {
	g.Mode = ez.StringMode()
	g.Start = "document"

	g.Define("document", func() {
		g.Whitespace()
		g.Lookahead(func() {
			g.String("{", "[")
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
				g.String("true")
			})
		}, func() {
			g.Capture("false", func() {
				g.String("false")
			})
		}, func() {
			g.Capture("null", func() {
				g.String("null")
			})
		})

	})

	g.Define("list", func() {
		g.String("[")
		g.Whitespace()
		g.Capture("list", func() {
			g.Optional().Do(func() {
				g.Call("value")
				g.Repeat().Do(func() {
					g.Whitespace()
					g.String(",")
					g.Whitespace()
					g.Call("value")
				})
			})
		})
		g.String("]")

	})

	g.Define("object", func() {
		g.String("{")
		g.Whitespace()
		g.Capture("object", func() {
			g.Optional().Do(func() {
				g.Call("string")
				g.Whitespace()
				g.String(":")
				g.Whitespace()
				g.Call("value")
			})
			g.Whitespace()
			g.Repeat().Do(func() {
				g.String(",")
				g.Whitespace()
				g.Call("string")
				g.Whitespace()
				g.String(":")
				g.Whitespace()
				g.Call("value")
				g.Whitespace()
			})
		})
		g.String("}")
	})

	g.Define("string", func() {
		g.String("\"")
		g.Capture("string", func() {
			g.Repeat().Choice(func() {
				g.String("\\u")
				g.Cut()
				g.Rune().Range("0-9", "a-f", "A-F")
				g.Rune().Range("0-9", "a-f", "A-F")
				g.Rune().Range("0-9", "a-f", "A-F")
				g.Rune().Range("0-9", "a-f", "A-F")
			}, func() {
				g.String("\\")
				g.Cut()
				g.String(
					"\"", "\\", "/", "b",
					"f", "n", "r", "t",
				)
			}, func() {
				g.Reject(func() {
					g.String("\\", "\"")
				})
				g.Rune()
			})
		})
		g.String("\"")
	})

	g.Define("number", func() {
		g.Capture("number", func() {
			g.Optional().Do(func() {
				g.String("-")
			})
			g.Choice(func() {
				g.String("0")
			}, func() {
				g.Rune().Range("1-9")
				g.Repeat().Do(func() {
					g.Rune().Range("0-9")
				})
			})
			g.Optional().Do(func() {
				g.String(".")
				g.Repeat().Do(func() {
					g.Rune().Range("0-9")
				})
			})
			g.Optional().Do(func() {
				g.String("e", "E")
				g.Optional().Do(func() {
					g.String("+", "-")
					g.Repeat().Do(func() {
						g.Rune().Range("0-9")
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
