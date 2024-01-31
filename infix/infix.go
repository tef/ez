package infix

import (
	"ez"
)

var InfixParser = ez.BuildParser(func(g *ez.G) {
	g.Mode = ez.StringMode()
	g.Start = "statement"

	g.Define("statement").Do(func() {
		g.Whitespace()
		g.Call("expression")
		g.Whitespace()
	})

	g.Define("expression").Recursive("expression").Choice(func() {
		g.Capture("add", func() {
			g.Corner("expression", 1)
			g.Recur("expression") 
			g.Whitespace()
			g.String("+")
			g.Whitespace()
			g.Stump("expression")
		})
	}, func() {
		g.Capture("=", func() {
			g.Corner("expression", 2)
			g.Stump("expression") 
			g.Whitespace()
			g.String("=")
			g.Whitespace()
			g.Recur("expression")
		})
	}, func() {
		g.NoCorner("expression",3)
		g.Call("number")
	})

	g.Define("number").Do(func() {
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
})
