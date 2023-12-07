package main

import (
	"errors"
	"fmt"
)

const (
	literal  = "Literal"
	choice   = "Choice"
	sequence = "Sequence"
	call     = "Call"
)

const (
	inGrammar = ""
	inDef     = "inside-definition"
	inChoice  = "inside-choice"
)

type parseRule func(*Parser, *parserState) bool

type parserState struct {
	rules  map[string]parseRule
	buf    string
	offset int
	// column int
	// indent_column int
	// for when we match n whitespace against a tab
	// leftover_tab int
	// leftover_tab pos
	// indents, dedents
	// parent
	// values map[string]any

}

func (s *parserState) clone() *parserState {
	st := parserState{}
	st = *s
	return &st
}

func (s *parserState) merge(new *parserState) {
	*s = *new
}

func (s *parserState) advance(v string) bool {
	fmt.Println("advance", s.offset)
	if len(v)+s.offset > len(s.buf) {
		return false
	}
	b := s.buf[s.offset : s.offset+len(v)]
	if b == v {
		s.offset += len(v)
		return true
	}
	return false
}

type grammarNode struct {
	kind string
	args []*grammarNode
	arg1 string
	arg2 int
	arg3 int
}

func (n *grammarNode) buildRule() parseRule {
	switch n.kind {
	case literal:
		return func(p *Parser, s *parserState) bool {
			return s.advance(n.arg1)
		}
	case call:
		name := n.arg1
		return func(p *Parser, s *parserState) bool {
			r := p.rules[name]
			return r(p, s)
		}
	case choice:
		rules := make([]parseRule, len(n.args))
		for i, r := range n.args {
			rules[i] = r.buildRule()
		}
		return func(p *Parser, s *parserState) bool {
			for _, r := range rules {
				fmt.Println("choice")
				s1 := s.clone()
				if r(p, s1) {
					s.merge(s1)
					return true
				}
			}
			return false
		}
	case sequence:
		rules := make([]parseRule, len(n.args))
		for i, r := range n.args {
			rules[i] = r.buildRule()
		}
		return func(p *Parser, s *parserState) bool {
			for _, r := range rules {
				if !r(p, s) {
					return false
				}
			}
			return true
		}
	default:
		return func(p *Parser, s *parserState) bool {
			fmt.Println(n.kind, "fake")
			return true
		}
	}
}

type nodeBuilder struct {
	context string
	args    []*grammarNode
}

func (b *nodeBuilder) buildNode() *grammarNode {
	if len(b.args) == 0 {
		return nil
	}
	if len(b.args) == 1 {
		return b.args[0]
	}
	return &grammarNode{kind: sequence, args: b.args}
}

func (b *nodeBuilder) append(a *grammarNode) {
	b.args = append(b.args, a)
}

type Grammar struct {
	Start string
	rules map[string]*grammarNode
	nb    *nodeBuilder
	err   error
}

func (g *Grammar) Define(name string, stub func()) {
	if g.err != nil {
		return
	}

	if g.nb != nil {
		g.err = errors.New("cant define inside a define")
	}

	r := &nodeBuilder{
		context: inDef,
		args:    make([]*grammarNode, 0),
	}
	g.nb = r
	stub()
	g.nb = nil

	if g.err != nil {
		return
	}
	g.rules[name] = r.buildNode()
}

func (g *Grammar) Call(name string) {
	if g.err != nil {
		return
	}
	if g.nb == nil {
		g.err = errors.New("called outside of definition")
		return
	}
	a := &grammarNode{kind: call, arg1: name}
	g.nb.append(a)
}

func (g *Grammar) Literal(s ...string) {
	if g.err != nil {
		return
	}
	if g.nb == nil {
		g.err = errors.New("called outside of definition")
		return
	}
	if len(s) == 0 {
		g.err = errors.New("missing operand")
	}

	if len(s) == 1 {
		a := &grammarNode{kind: literal, arg1: s[0]}
		g.nb.append(a)
	} else {
		args := make([]*grammarNode, len(s))
		for i, v := range s {
			args[i] = &grammarNode{kind: literal, arg1: v}
		}
		a := &grammarNode{kind: choice, args: args}
		g.nb.append(a)
	}
}

func (g *Grammar) Choice(options ...func()) {
	if g.err != nil {
		return
	}
	if g.nb == nil {
		g.err = errors.New("called outside of definition")
		return
	}

	args := make([]*grammarNode, len(options))
	for i, stub := range options {
		old_r := g.nb
		new_r := &nodeBuilder{
			context: inChoice,
			args:    make([]*grammarNode, 0),
		}

		g.nb = new_r
		stub()
		g.nb = old_r

		if g.err != nil {
			return
		}
		args[i] = new_r.buildNode()
	}
	a := &grammarNode{kind: choice, args: args}
	g.nb.append(a)
}

type Parser struct {
	rules map[string]parseRule
	start string
	Err   error
}

func (p *Parser) Accept(s string) bool {
	parserState := &parserState{
		rules: p.rules,
		buf:   s,
	}
	start := p.rules[p.start]
	return start(p, parserState) && parserState.offset == len(parserState.buf)

}

func BuildParser(stub func(*Grammar)) *Parser {
	g := &Grammar{
		rules: make(map[string]*grammarNode, 1),
	}
	stub(g)

	if g.err != nil {
		return &Parser{Err: g.err}
	}

	rules := make(map[string]parseRule, len(g.rules))
	start := g.Start

	for k, v := range g.rules {
		fmt.Println("rule", k, v.kind, v.args)
		rules[k] = v.buildRule()
	}

	p := &Parser{
		start: start,
		rules: rules,
	}
	return p
}

func main() {
	p := BuildParser(func(g *Grammar) {
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

	if p.Err != nil {
		fmt.Println("err:", p.Err)
	}

	fmt.Println("-")
	if p.Accept("true") {
		fmt.Println("parsed true!")
	}

	fmt.Println("-")

	if p.Accept("false") {
		fmt.Println("parsed false!")
	}
	fmt.Println("-")
	if !p.Accept("blue") {
		fmt.Println("didn't parse! (good)")
	}
}
