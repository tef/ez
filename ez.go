package ez

import (
	// "errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
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
	inGrammar  = "inside-grammar"
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
	pos  int
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
		idx := g.nameIdx[name]
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

func (b *nodeBuilder) buildNode(pos int) *grammarNode {
	if len(b.args) == 0 {
		return nil
	}
	if len(b.args) == 1 {
		return b.args[0]
	}
	return &grammarNode{kind: sequenceNode, args: b.args, pos: pos}
}

func (b *nodeBuilder) append(a *grammarNode) {
	b.args = append(b.args, a)
}

func (b *nodeBuilder) inRule() bool {
	return b != nil && b.context != inGrammar
}

type grammarError struct {
	g       *Grammar
	pos     int
	rule    *int
	message string
	fatal   bool
}

func (e *grammarError) Error() string {
	p := e.g.posInfo[e.pos]
	if e.rule != nil {
		name := e.g.names[*e.rule]
		rulePos := e.g.posInfo[e.g.rulePos[*e.rule]]
		return fmt.Sprintf("%v: %v (inside %q at %v)", p, e.message, name, rulePos)

	} else {
		return fmt.Sprintf("%v: %v", p, e.message)

	}
}

type Grammar struct {
	Start      string
	Whitespace []string
	Newline    []string

	rules   []*grammarNode
	names   []string
	nameIdx map[string]int

	// list of pos for each name
	callPos map[string][]int

	// list of pos for each numbered rule
	rulePos []int
	// list of positions
	posInfo []string

	nb *nodeBuilder

	pos    int // grammar position
	errors []error
	err    error
}

func (g *Grammar) markPosition() int {
	_, file, no, ok := runtime.Caller(2)
	if ok {
		base, _ := os.Getwd()
		file, _ := filepath.Rel(base, file)
		pos := fmt.Sprintf("%v:%v", file, no)
		p := len(g.posInfo)
		g.posInfo = append(g.posInfo, pos)
		return p
	}
	return -1
}

func (g *Grammar) Err() error {
	return g.err
}

func (g *Grammar) Errors() []error {
	if g.errors == nil {
		return []error{}
	}
	return g.errors
}

func (g *Grammar) Error(pos int, args ...any) {
	msg := fmt.Sprint(args...)
	err := &grammarError{
		g:       g,
		message: msg,
		pos:     pos,
	}
	if g.err == nil {
		g.err = err
	}
	g.errors = append(g.errors, err)
}

func (g *Grammar) Errorf(pos int, s string, args ...any) {
	msg := fmt.Sprintf(s, args...)
	err := &grammarError{
		g:       g,
		message: msg,
		pos:     pos,
	}
	if g.err == nil {
		g.err = err
	}
	g.errors = append(g.errors, err)
}

func (g *Grammar) Warn(pos int, args ...any) {
	msg := fmt.Sprint(args...)
	err := &grammarError{
		g:       g,
		message: msg,
		pos:     pos,
	}
	g.errors = append(g.errors, err)
}

func (g *Grammar) Warnf(pos int, s string, args ...any) {
	msg := fmt.Sprintf(s, args...)
	err := &grammarError{
		g:       g,
		message: msg,
		pos:     pos,
	}
	g.errors = append(g.errors, err)
}

func (g *Grammar) shouldExit(pos int) bool {
	if g.err != nil {
		return true
	}
	if g.nb == nil {
		g.Error(pos, "must call builder methods inside builder")
		return true
	}
	if !g.nb.inRule() {
		g.Error(pos, "must call builder methods inside Define()")
		return true
	}
	return false

}

func (g *Grammar) callStub(b *nodeBuilder, stub func()) {
	oldNb := g.nb
	g.nb = b
	stub()
	g.nb = oldNb
}

func (g *Grammar) Define(name string, stub func()) {
	p := g.markPosition()
	if g.err != nil {
		return
	} else if g.nb == nil {
		g.Error(p, "must call define inside grammar")
		return
	} else if g.nb.inRule() {
		g.Error(p, "cant call define inside define")
		return
	}

	if old, ok := g.nameIdx[name]; ok {
		oldPos := g.posInfo[g.rulePos[old]]
		g.Errorf(p, "cant redefine %q, already defined at %v", name, oldPos)
		return
	}

	r := &nodeBuilder{
		context: inDef,
	}

	g.callStub(r, stub)
	if g.err != nil {
		return
	}

	pos := len(g.names)
	g.names = append(g.names, name)
	g.nameIdx[name] = pos
	g.rulePos = append(g.rulePos, p)
	g.rules = append(g.rules, r.buildNode(p))
}

func (g *Grammar) Call(name string) {
	p := g.markPosition()
	if g.shouldExit(p) {
		return
	}
	g.callPos[name] = append(g.callPos[name], p)
	a := &grammarNode{kind: callNode, arg1: name, pos: p}
	g.nb.append(a)
}

func (g *Grammar) Literal(s ...string) {
	p := g.markPosition()
	if g.shouldExit(p) {
		return
	}
	if len(s) == 0 {
		g.Error(p, "missing operand")
	}

	if len(s) == 1 {
		a := &grammarNode{kind: literalNode, arg1: s[0], pos: p}
		g.nb.append(a)
	} else {
		args := make([]*grammarNode, len(s))
		for i, v := range s {
			args[i] = &grammarNode{kind: literalNode, arg1: v, pos: p}
		}
		a := &grammarNode{kind: choiceNode, args: args, pos: p}
		g.nb.append(a)
	}
}

func (g *Grammar) Choice(options ...func()) {
	p := g.markPosition()
	if g.shouldExit(p) {
		return
	}

	args := make([]*grammarNode, len(options))
	for i, stub := range options {
		r := &nodeBuilder{
			context: inChoice,
		}

		g.callStub(r, stub)

		if g.err != nil {
			return
		}

		args[i] = r.buildNode(p)
	}
	a := &grammarNode{kind: choiceNode, args: args, pos: p}
	g.nb.append(a)
}

func (g *Grammar) Optional(stub func()) {
	p := g.markPosition()
	if g.shouldExit(p) {
		return
	}
	r := &nodeBuilder{context: inOptional}
	g.callStub(r, stub)

	if g.err != nil {
		return
	}

	a := &grammarNode{kind: optionalNode, args: r.args, pos: p}
	g.nb.append(a)
}

func (g *Grammar) Repeat(min_t int, max_t int, stub func()) {
	p := g.markPosition()
	if g.shouldExit(p) {
		return
	}

	r := &nodeBuilder{context: inRepeat}
	g.callStub(r, stub)

	if g.err != nil {
		return
	}

	a := &grammarNode{kind: repeatNode, args: r.args, arg2: min_t, arg3: max_t, pos: p}
	g.nb.append(a)
}

func (g *Grammar) Check() error {
	if g.err != nil {
		return g.err
	}
	for name, pos := range g.callPos {
		if _, ok := g.nameIdx[name]; !ok {
			for _, p := range pos {
				g.Errorf(p, "missing rule %q", name)
			}
		}
	}

	for n, name := range g.names {
		if name != g.Start && g.callPos[name] == nil {
			p := g.rulePos[n]
			g.Errorf(p, "unused rule %q", name)
		}
	}

	if g.Start == "" {
		g.Error(g.pos, "starting rule undefined")
	}

	_, ok := g.nameIdx[g.Start]
	if !ok {
		g.Errorf(g.pos, "starting rule %q is missing", g.Start)
	}

	return g.err
}

func (g *Grammar) Parser() (*Parser, error) {
	if g.Check() != nil {
		return nil, g.err
	}

	rules := make([]parseRule, len(g.rules))

	for k, v := range g.rules {
		rules[k] = v.buildRule(g)
	}

	start := g.nameIdx[g.Start]

	p := &Parser{
		start:   start,
		rules:   rules,
		names:   g.names,
		nameIdx: g.nameIdx,
	}
	return p, nil
}

type Parser struct {
	rules   []parseRule
	names   []string
	nameIdx map[string]int
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
	b := &nodeBuilder{
		context: inGrammar,
	}
	g := &Grammar{
		nameIdx: make(map[string]int, 0),
		callPos: make(map[string][]int, 0),
		nb:      b,
	}
	g.pos = g.markPosition()
	stub(g)

	if g.Check() != nil {
		return nil, g.err
	}

	return g, nil
}

func BuildParser(stub func(*Grammar)) (*Parser, error) {
	b := &nodeBuilder{
		context: inGrammar,
	}
	g := &Grammar{
		nameIdx: make(map[string]int, 0),
		callPos: make(map[string][]int, 0),
		nb:      b,
	}
	g.pos = g.markPosition()
	stub(g)

	return g.Parser()
}
