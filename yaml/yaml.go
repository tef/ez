package yaml

import (
	"ez"
)

var YamlParser = ez.BuildParser(func(g *ez.G) {
	g.Mode = ez.TextMode()
	g.Start = "document"

	g.Define("whitespace?", func() {
		g.Repeat().Choice(func() {
			g.Whitespace()
			g.Newline()
		}, func() {
			g.Whitespace()
			g.String("#")
			g.Repeat().Do(func() {
				g.Rune().Except("\n")
			})
			g.Newline()
		}, func() {
			g.Whitespace().Min(1)
		})
	})

	g.Define("newline?", func() {
		g.Repeat().Choice(func() {
			g.Whitespace()
			g.Newline()
		}, func() {
			g.Whitespace()
			g.String("#")
			g.Repeat().Do(func() {
				g.Rune().Except("\n")
			})
			g.Newline()
		})
	})

	g.Define("document", func() {
		g.Call("newline?")
		g.Choice(func() {
			g.Call("indented-object")
		}, func() {
			g.Call("indented-list")
		}, func() {
			g.Call("whitespace?")
			g.Call("list")
		}, func() {
			g.Call("whitespace?")
			g.Call("object")
		})
		g.Repeat().Do(func() {
			g.Call("newline?")
		})
	})

	g.Define("indented-value", func() {
		g.Choice(func() {
			g.Call("indented-object")
		}, func() {
			g.Call("indented-list")
		}, func() {
			g.Call("value")
		})
	})

	g.Define("indented-object", func() {
		g.Capture("object", func() {
			g.OffsideBlock(func() {
				g.Call("key")
				g.Whitespace() // on same line
				g.String(":")
				g.Choice(func() {
					g.Whitespace()
					g.Call("indented-value")
				}, func() {
					g.Whitespace()
					g.Newline()
					g.Indent()
					g.Whitespace().Min(1)
					g.Call("indented-value")
				})

				g.Repeat().Do(func() {
					g.Newline()
					g.Call("key")
					g.Whitespace()
					g.String(":")
					g.Choice(func() {
						g.Whitespace()
						g.Call("indented-value")
					}, func() {
						g.Whitespace()
						g.Newline()
						g.Indent()
						g.Whitespace().Min(1)
						g.Call("indented-value")
					})
				})
			})

		})
	})

	g.Define("indented-list", func() {
		g.Capture("list", func() {
			g.OffsideBlock(func() {
				g.String("-")
				g.Choice(func() {
					g.Whitespace()
					g.Call("indented-value")
				}, func() {
					g.Whitespace()
					g.Newline()
					g.Indent()
					g.Whitespace().Min(1)
					g.Call("indented-value")
				})
				g.Repeat().Do(func() {
					g.Newline()
					g.Indent()
					g.String("-")
					g.Choice(func() {
						g.Whitespace()
						g.Call("indented-value")
					}, func() {
						g.Whitespace()
						g.Newline()
						g.Indent()
						g.Whitespace().Min(1)
						g.Call("indented-value")
					})
				})

			})
		})
	})

	g.Define("key", func() {
		g.Choice(func() {
			g.Call("string")
		}, func() {
			g.Capture("key", func() {
				g.Rune().Range("a-z", "A-Z", "_")
				g.Repeat().Do(func() {
					g.Rune().Range("a-z", "A-Z", "_", "0-9")
				})
			})
		})
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
			g.Repeat().Min(1).Do(func() {
				g.Rune().Range("0-9")
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

	g.Builder("key", func(s string, args []any) (any, error) {
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
