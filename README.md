# ez

`ez` is a parser toolkit in golang, for those stubborn hard to reach grammars.

```
parser := ez.BuildParser(func(g *ez.G) {
        g.Start = "expr"

        g.Define("expr", func() {
                g.Choice(func() {
                        g.Call("rule_a")
                }, func() {
                        g.Call("rule_b")
                })
        })

        g.Define("rule_a", func() {
                g.String("true")
        })

        g.Define("rule_b", func() {
                g.String("false")
        })
})

if p.Err != nil {
        fmt.Println("err:", p.Err)
}

_, err := p.Parse("true") 
if err != nil {
        fmt.Println("parsed true!")
}
```

`ez` handles things like parsing indented blocks, back references (for matching delimiters),
data dependent grammars (length prefixed values), and infix operators with precedence.

`ez` also comes with `Print()` and `Trace()` operators to help you debug a grammar, too.

# what makes `ez` different

`ez` is a little different from other parsing libraries. 
`ez` uses callbacks to build up nested structures, like so:

```
ez.Sequence(func(){
    ez.String("foo")
    ez.String("bar")
})
```

It's a little more verbose, but it's a little less error prone. It lets you write
more interesting grammars than you could in other methods, and you don't need to learn
a new syntax either. There's even error messages with line numbers, too.

`ez` also takes a different approach to parsing algorithms

`ez` currently works like a scannerless recursive descent parser. That's a fancy way 
of saying you don't need to define a tokenizer or lexer, and that the parser works
from top to bottom, from left to right.

it's very much like a parsing evaluation grammar, but there's no full on backtracking, 
or memoization. that's a fancy way of saying that if you have "(a or b) and c", and
a parses, but c doesn't, the parser will not try parsing b.

`ez` provides built in operators for handling things like indentation, matching
delimiters, and other features of markup languages. there's also operators
for debugging your grammar, too.

# other work

`ez` is a port of a python parser-generator, used to write a peg-like parser
for CommonMark

https://github.com/tef/toyparser2019/tree/master/toyparser


