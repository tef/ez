# ez

`ez` is a parser toolkit in golang, for those stubborn hard to reach grammars:

```
parser := ez.BuildParser(func(g *ez.Grammar) {
        g.Start = "expr"

        g.Define("expr", func() {
                g.Choice(func() {
                        g.Call("rule_a")
                }, func() {
                        g.Call("rule_b")
                })
        })

        g.Define("rule_a", func() {
                g.Literal("true")
        })

        g.Define("rule_b", func() {
                g.Literal("false")
        })
})

if p.Err != nil {
        fmt.Println("err:", p.Err)
}

if p.Accept("true") {
        fmt.Println("parsed true!")
}
```



