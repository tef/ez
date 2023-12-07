package main

import (
	"fmt"

	"github.com/tef/ez"
)


func main() {
	parser := ez.BuildParser(func(g *ez.Grammar) {
		g.Start = "expr"

		g.Define("expr", func() {
			g.Choice(func() {
				g.Call("truerule")
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

	if parser.Err != nil {
		fmt.Println("err:", parser.Err)
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
}
