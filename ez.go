package ez

import (
	"errors"
	// "fmt"
)

const (
	callNode    = "Call"
	literalNode = "Literal"

	whitespaceNode  = "Whitespace"
	newlineNode     = "Newline"
	partialTabNOde  = "PartialTab"
	startOfLineNode = "StartOfLine"
	endOfLineNode   = "EndOfLine"
	endOfFileNode   = "EndOfFile"

	choiceNode    = "Choice"
	sequenceNode  = "Sequence"
	captureNode   = "Capture"
	lookaheadNode = "Lookahead"
	rejectNode    = "Reject"
	rangeNode     = "Range"
	optionalNode  = "Optional"
	repeatNode    = "Repeat"

	indentNode = "Indent"
	dedentNode = "Dedent"
)

const (
	inGrammar  = ""
	inDef      = "inside-definition"
	inChoice   = "inside-choice"
	inOptional = "inside-optional"
	inRepeat   = "inside-repeat"
)

type parseRule func(*Parser, *parserState) bool

type parserState struct {
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

func (n *grammarNode) buildRule(g *Grammar) parseRule {
	switch n.kind {
	case literalNode:
		return func(p *Parser, s *parserState) bool {
			return s.advance(n.arg1)
		}
	case callNode:
		name := n.arg1
		idx := *g.nameIdx[name]
		return func(p *Parser, s *parserState) bool {
			r := p.rules[idx]
			return r(p, s)
		}
	case optionalNode:
		rules := make([]parseRule, len(n.args))
		for i, r := range n.args {
			rules[i] = r.buildRule(g)
		}
		return func(p *Parser, s *parserState) bool {
			s1 := s.clone()
			for _, r := range rules {
				if !r(p, s1) {
					return true
				}
			}
			s.merge(s1)
			return true
		}

	case repeatNode:
		rules := make([]parseRule, len(n.args))
		for i, r := range n.args {
			rules[i] = r.buildRule(g)
		}
		min_n := n.arg2
		max_n := n.arg3

		return func(p *Parser, s *parserState) bool {
			c := 0
			for {
				s1 := s.clone()
				for _, r := range rules {
					if !r(p, s1) {
						return c >= min_n
					}
				}
				s.merge(s1)
				c += 1
				if max_n != 0 && c >= max_n {
					break
				}
			}
			return true
		}

	case choiceNode:
		rules := make([]parseRule, len(n.args))
		for i, r := range n.args {
			rules[i] = r.buildRule(g)
		}
		return func(p *Parser, s *parserState) bool {
			for _, r := range rules {
				s1 := s.clone()
				if r(p, s1) {
					s.merge(s1)
					return true
				}
			}
			return false
		}
	case sequenceNode:
		rules := make([]parseRule, len(n.args))
		for i, r := range n.args {
			rules[i] = r.buildRule(g)
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
	return &grammarNode{kind: sequenceNode, args: b.args}
}

func (b *nodeBuilder) append(a *grammarNode) {
	b.args = append(b.args, a)
}

type Grammar struct {
	Start      string
	Whitespace []string
	Newline    []string

	rules   []*grammarNode
	names   []string
	nameIdx map[string]*int
	nb      *nodeBuilder
	err     error
}

func (g *Grammar) Define(name string, stub func()) {
	if g.err != nil {
		return
	}

	if g.nb != nil {
		g.err = errors.New("cant define inside a define")
		return
	}

	if g.nameIdx[name] != nil {
		g.err = errors.New("cant redefine")
		return
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
	pos := len(g.names)
	g.names = append(g.names, name)
	g.nameIdx[name] = &pos
	g.rules = append(g.rules, r.buildNode())
}

func (g *Grammar) Call(name string) {
	if g.err != nil {
		return
	}
	if g.nb == nil {
		g.err = errors.New("called outside of definition")
		return
	}
	a := &grammarNode{kind: callNode, arg1: name}
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
		a := &grammarNode{kind: literalNode, arg1: s[0]}
		g.nb.append(a)
	} else {
		args := make([]*grammarNode, len(s))
		for i, v := range s {
			args[i] = &grammarNode{kind: literalNode, arg1: v}
		}
		a := &grammarNode{kind: choiceNode, args: args}
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
	a := &grammarNode{kind: choiceNode, args: args}
	g.nb.append(a)
}

func (g *Grammar) Optional(stub func()) {
	if g.err != nil {
		return
	}
	if g.nb == nil {
		g.err = errors.New("called outside of definition")
		return
	}

	old_r := g.nb
	new_r := &nodeBuilder{
		context: inOptional,
		args:    make([]*grammarNode, 0),
	}

	g.nb = new_r
	stub()
	g.nb = old_r

	if g.err != nil {
		return
	}

	args := new_r.args
	a := &grammarNode{kind: optionalNode, args: args}
	g.nb.append(a)
}

func (g *Grammar) Repeat(min_t int, max_t int, stub func()) {
	if g.err != nil {
		return
	}
	if g.nb == nil {
		g.err = errors.New("called outside of definition")
		return
	}

	old_r := g.nb
	new_r := &nodeBuilder{
		context: inRepeat,
		args:    make([]*grammarNode, 0),
	}

	g.nb = new_r
	stub()
	g.nb = old_r

	if g.err != nil {
		return
	}

	args := new_r.args
	a := &grammarNode{kind: repeatNode, args: args, arg2: min_t, arg3: max_t}
	g.nb.append(a)
}

func (g *Grammar) Parser() (*Parser, error) {
	rules := make([]parseRule, len(g.rules))
	start := g.nameIdx[g.Start]

	for k, v := range g.rules {
		rules[k] = v.buildRule(g)
	}

	p := &Parser{
		start:   *start,
		rules:   rules,
		names:   g.names,
		nameIdx: g.nameIdx,
	}
	return p, nil
}

type Parser struct {
	rules   []parseRule
	names   []string
	nameIdx map[string]*int
	start   int
	Err     error
}

func (p *Parser) Accept(s string) bool {
	parserState := &parserState{
		buf: s,
	}
	start := p.rules[p.start]
	return start(p, parserState) && parserState.offset == len(parserState.buf)

}

func BuildGrammar(stub func(*Grammar)) (*Grammar, error) {
	g := &Grammar{
		rules:   make([]*grammarNode, 0),
		nameIdx: make(map[string]*int, 0),
	}
	stub(g)

	if g.err != nil {
		return nil, g.err
	}

	return g, nil
}

func BuildParser(stub func(*Grammar)) (*Parser, error) {
	g := &Grammar{
		rules:   make([]*grammarNode, 0),
		nameIdx: make(map[string]*int, 0),
	}
	stub(g)

	if g.err != nil {
		return nil, g.err
	}

	return g.Parser()
}
