package main

import (
	"fmt"

	"github.com/tef/ez"
)

func main() {
	parser, err := ez.BuildParser(func(g *ez.Grammar) {
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
		fmt.Println("err:", err)
		return
	}

	fmt.Println("-")
	if parser.Accept("true") {
		fmt.Println("parsed true!")
	}

	fmt.Println("-")
	if parser.Accept("false") {
		fmt.Println("parsed false!")
	}

	fmt.Println("-")
	if !parser.Accept("blue") {
		fmt.Println("didn't parse! (good)")
	}
	fmt.Println("-")
	if parser.Accept("truey") {
		fmt.Println("parsed truey!")
	}
}
