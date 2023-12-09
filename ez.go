package ez

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
)

const (
	grammarKind     = "Grammar"
	defineKind      = "Define"
	printKind       = "Debug.Print"
	traceKind       = "Debug.Trace"
	callKind        = "Call"
	literalKind     = "Literal"
	whitespaceKind  = "Whitespace"
	newlineKind     = "Newline"
	partialTabKind  = "PartialTab"
	startOfLineKind = "StartOfLine"
	endOfLineKind   = "EndOfLine"
	startOfFileKind = "StartOfFie"
	endOfFileKind   = "EndOfFile"
	choiceKind      = "Choice"
	sequenceKind    = "Sequence"
	optionalKind    = "Optional"
	repeatKind      = "Repeat"
	lookaheadKind   = "Lookahead"
	rejectKind      = "Reject"
	captureKind     = "Capture"
	rangeKind       = "Range"
	indentKind      = "Indent"
	dedentKind      = "Dedent"
)

type parseRule func(*Parser, *parserState) bool

type parserState struct {
	buf    string
	length int
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

func (s *parserState) atEnd() bool {
	return s.offset >= s.length
}

func (s *parserState) advance(v string) bool {
	length_v := len(v)
	if length_v+s.offset > s.length {
		return false
	}
	b := s.buf[s.offset : s.offset+length_v]
	if b == v {
		s.offset += length_v
		return true
	}
	return false
}

func (s *parserState) advanceAny(o []string) bool {
	for _, v := range o {
		if s.advance(v) {
			return true
		}
	}
	return false
}

type grammarNode struct {
	pos     int
	kind    string
	args    []*grammarNode
	arg1    string
	arg2    int
	arg3    int
	message []any
}

func (n *grammarNode) buildRule(g *Grammar) parseRule {
	switch n.kind {
	case printKind:
		p := g.posInfo[n.pos]
		r := g.names[*p.rule]
		prefix := fmt.Sprintf("%v:%v", p.file, p.line)
		return func(p *Parser, s *parserState) bool {
			msg := fmt.Sprint(n.message...)
			log.Printf("%v: g.Print(%q) called (inside %q at pos %v)\n", prefix, msg, r, s.offset)
			return true
		}
	case traceKind:
		p := g.posInfo[n.pos]
		r := g.names[*p.rule]
		prefix := fmt.Sprintf("%v:%v", p.file, p.line)

		rules := make([]parseRule, len(n.args))
		for i, r := range n.args {
			rules[i] = r.buildRule(g)
		}

		return func(p *Parser, s *parserState) bool {
			log.Printf("%v: g.Trace() called (inside %q at pos %v)\n", prefix, r, s.offset)
			result := true

			s1 := s.clone()
			for _, v := range rules {
				if !v(p, s1) {
					result = false
					break
				}
			}
			if result {
				s.merge(s1)
				log.Printf("%v: g.Trace() exiting (inside %q at pos %v)\n", prefix, r, s.offset)
			} else {
				log.Printf("%v: g.Trace() failing (inside %q at pos %v)\n", prefix, r, s.offset)
			}
			return result
		}

	// case partialTabKind
	// case indentKind
	// case dedentKind

	case whitespaceKind:
		return func(p *Parser, s *parserState) bool {
			for {
				if !s.advanceAny(g.Whitespaces) {
					break
				}
			}
			return true
		}
	case newlineKind:
		return func(p *Parser, s *parserState) bool {
			return s.advanceAny(g.Newlines)
		}

	case startOfLineKind:
		return func(p *Parser, s *parserState) bool {
			return true // column = 0
		}
	case endOfLineKind:
		return func(p *Parser, s *parserState) bool {
			return s.advanceAny(g.Newlines)
		}
	case startOfFileKind:
		return func(p *Parser, s *parserState) bool {
			return s.offset == 0
		}
	case endOfFileKind:
		return func(p *Parser, s *parserState) bool {
			return s.offset == len(s.buf)
		}

	case literalKind:
		return func(p *Parser, s *parserState) bool {
			return s.advance(n.arg1)
		}
	case callKind:
		name := n.arg1
		idx := g.nameIdx[name]
		return func(p *Parser, s *parserState) bool {
			r := p.rules[idx]
			return r(p, s)
		}
	case optionalKind:
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
	case lookaheadKind:
		rules := make([]parseRule, len(n.args))
		for i, r := range n.args {
			rules[i] = r.buildRule(g)
		}
		return func(p *Parser, s *parserState) bool {
			s1 := s.clone()
			for _, r := range rules {
				if !r(p, s1) {
					return false
				}
			}
			return true
		}
	case rejectKind:
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
			return false
		}

	case repeatKind:
		rules := make([]parseRule, len(n.args))
		for i, r := range n.args {
			rules[i] = r.buildRule(g)
		}
		min_n := n.arg2
		max_n := n.arg3

		return func(p *Parser, s *parserState) bool {
			c := 0
			s1 := s.clone()
			for {
				for _, r := range rules {
					if !r(p, s1) {
						return c >= min_n
					}
				}
				c += 1
				if c >= min_n {
					s.merge(s1)
				}

				if max_n != 0 && c >= max_n {
					break
				}
			}
			return true
		}

	case choiceKind:
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
	case sequenceKind:
		rules := make([]parseRule, len(n.args))
		for i, r := range n.args {
			rules[i] = r.buildRule(g)
		}
		return func(p *Parser, s *parserState) bool {
			s1 := s.clone()
			for _, r := range rules {
				if !r(p, s1) {
					return false
				}
			}
			s.merge(s1)
			return true
		}
	default:
		return func(p *Parser, s *parserState) bool {
			return true
		}
	}
}

type nodeBuilder struct {
	kind string
	rule *int
	args []*grammarNode
}

func (b *nodeBuilder) buildSequence(pos int) *grammarNode {
	if len(b.args) == 0 {
		return nil
	}
	if len(b.args) == 1 {
		return b.args[0]
	}
	return &grammarNode{kind: sequenceKind, args: b.args, pos: pos}
}

func (b *nodeBuilder) append(a *grammarNode) {
	b.args = append(b.args, a)
}

func (b *nodeBuilder) inRule() bool {
	return b != nil && b.kind != grammarKind
}

type grammarError struct {
	g       *Grammar
	pos     int
	message string
	fatal   bool
}

type position struct {
	file string
	line int
	rule *int
}

func (e *grammarError) Error() string {
	p := e.g.posInfo[e.pos]
	if p.rule != nil {
		name := e.g.names[*p.rule]
		rulePos := e.g.posInfo[e.g.rulePos[*p.rule]]
		return fmt.Sprintf("%v:%v: %v (inside %q at %v:%v)", p.file, p.line, e.message, name, rulePos.file, rulePos.line)

	} else {
		return fmt.Sprintf("%v:%v: %v", p.file, p.line, e.message)

	}
}

type Grammar struct {
	Start       string
	Whitespaces []string
	Newlines    []string

	rules   []*grammarNode
	names   []string
	nameIdx map[string]int

	// list of pos for each name
	callPos map[string][]int

	// list of pos for each numbered rule
	rulePos []int
	// list of positions
	posInfo []position

	nb *nodeBuilder

	pos    int // grammar position
	errors []error
	err    error
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

func (g *Grammar) markPosition() int {
	_, file, no, ok := runtime.Caller(2)
	if !ok {
		return -1
	}
	base, _ := os.Getwd()
	file, _ = filepath.Rel(base, file)
	var rule *int = nil
	if g.nb != nil {
		rule = g.nb.rule
	}
	pos := position{file: file, line: no, rule: rule}
	p := len(g.posInfo)

	g.posInfo = append(g.posInfo, pos)
	return p
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

func (g *Grammar) buildStub(kind string, stub func()) *nodeBuilder {
	var rule *int
	oldNb := g.nb
	if oldNb != nil {
		rule = oldNb.rule
	}
	newNb := &nodeBuilder{kind: kind, rule: rule}
	g.nb = newNb
	stub()
	g.nb = oldNb
	return newNb
}

func (g *Grammar) buildRule(rule int, stub func()) *nodeBuilder {
	oldNb := g.nb
	newNb := &nodeBuilder{kind: defineKind, rule: &rule}
	g.nb = newNb
	stub()
	g.nb = oldNb
	return newNb
}

func (g *Grammar) buildGrammar(stub func(*Grammar)) error {
	if g.nb != nil || g.names != nil {
		return errors.New("use empty grammar")
	}
	g.nameIdx = make(map[string]int, 0)
	g.callPos = make(map[string][]int, 0)
	g.nb = &nodeBuilder{kind: grammarKind}

	stub(g)
	g.nb = nil

	return g.Check()
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

	ruleNum := len(g.names)
	g.names = append(g.names, name)
	g.nameIdx[name] = ruleNum
	g.rulePos = append(g.rulePos, p)

	r := g.buildRule(ruleNum, stub)
	g.rules = append(g.rules, r.buildSequence(p))
}

func (g *Grammar) Print(args ...any) {
	p := g.markPosition()
	if g.shouldExit(p) {
		return
	}
	a := &grammarNode{kind: printKind, message: args, pos: p}
	g.nb.append(a)
}
func (g *Grammar) Trace(stub func()) {
	p := g.markPosition()
	if g.shouldExit(p) {
		return
	}
	r := g.buildStub(traceKind, stub)
	if g.err != nil {
		return
	}

	a := &grammarNode{kind: traceKind, args: r.args, pos: p}
	g.nb.append(a)
}

func (g *Grammar) Whitespace() {
	p := g.markPosition()
	if g.shouldExit(p) {
		return
	}
	a := &grammarNode{kind: whitespaceKind, pos: p}
	g.nb.append(a)
}

func (g *Grammar) Newline() {
	p := g.markPosition()
	if g.shouldExit(p) {
		return
	}
	a := &grammarNode{kind: newlineKind, pos: p}
	g.nb.append(a)
}

func (g *Grammar) Call(name string) {
	p := g.markPosition()
	if g.shouldExit(p) {
		return
	}
	g.callPos[name] = append(g.callPos[name], p)
	a := &grammarNode{kind: callKind, arg1: name, pos: p}
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
		a := &grammarNode{kind: literalKind, arg1: s[0], pos: p}
		g.nb.append(a)
	} else {
		args := make([]*grammarNode, len(s))
		for i, v := range s {
			args[i] = &grammarNode{kind: literalKind, arg1: v, pos: p}
		}
		a := &grammarNode{kind: choiceKind, args: args, pos: p}
		g.nb.append(a)
	}
}
func (g *Grammar) Sequence(stub func()) {
	p := g.markPosition()
	if g.shouldExit(p) {
		return
	}

	r := g.buildStub(sequenceKind, stub)

	if g.err != nil {
		return
	}

	a := r.buildSequence(p)
	g.nb.append(a)
}

func (g *Grammar) Choice(options ...func()) {
	p := g.markPosition()
	if g.shouldExit(p) {
		return
	}

	args := make([]*grammarNode, len(options))
	for i, stub := range options {
		r := g.buildStub(choiceKind, stub)

		if g.err != nil {
			return
		}

		args[i] = r.buildSequence(p)
	}
	a := &grammarNode{kind: choiceKind, args: args, pos: p}
	g.nb.append(a)
}

func (g *Grammar) Optional(stub func()) {
	p := g.markPosition()
	if g.shouldExit(p) {
		return
	}
	r := g.buildStub(optionalKind, stub)
	if g.err != nil {
		return
	}

	a := &grammarNode{kind: optionalKind, args: r.args, pos: p}
	g.nb.append(a)
}

func (g *Grammar) Lookahead(stub func()) {
	p := g.markPosition()
	if g.shouldExit(p) {
		return
	}
	r := g.buildStub(lookaheadKind, stub)
	if g.err != nil {
		return
	}

	a := &grammarNode{kind: lookaheadKind, args: r.args, pos: p}
	g.nb.append(a)
}

func (g *Grammar) Reject(stub func()) {
	p := g.markPosition()
	if g.shouldExit(p) {
		return
	}
	r := g.buildStub(rejectKind, stub)
	if g.err != nil {
		return
	}

	a := &grammarNode{kind: rejectKind, args: r.args, pos: p}
	g.nb.append(a)
}

func (g *Grammar) Repeat(min_t int, max_t int, stub func()) {
	p := g.markPosition()
	if g.shouldExit(p) {
		return
	}

	r := g.buildStub(repeatKind, stub)

	if g.err != nil {
		return
	}

	a := &grammarNode{kind: repeatKind, args: r.args, arg2: min_t, arg3: max_t, pos: p}
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
	} else if _, ok := g.nameIdx[g.Start]; !ok {
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

func (p *Parser) testParser(accept []string, reject []string) bool {
	start := p.rules[p.start]
	return p.testParseRule(start, accept, reject)
}

func (p *Parser) testRule(name string, accept []string, reject []string) bool {
	start := p.rules[p.nameIdx[name]]
	return p.testParseRule(start, accept, reject)
}

func (p *Parser) testParseRule(rule parseRule, accept []string, reject []string) bool {
	for _, s := range accept {
		parserState := &parserState{
			buf:    s,
			length: len(s),
		}
		complete := rule(p, parserState) && parserState.atEnd()

		if !complete {
			return false
		}
	}
	for _, s := range reject {
		parserState := &parserState{
			buf:    s,
			length: len(s),
		}
		complete := rule(p, parserState) && parserState.atEnd()

		if complete {
			return false
		}
	}
	return true
}

func BuildGrammar(stub func(*Grammar)) (*Grammar, error) {
	g := &Grammar{}
	g.pos = g.markPosition()
	err := g.buildGrammar(stub)
	if err != nil {
		return nil, err
	}
	return g, nil
}

func BuildParser(stub func(*Grammar)) (*Parser, error) {
	g := &Grammar{}
	g.pos = g.markPosition()
	err := g.buildGrammar(stub)
	if err != nil {
		return nil, err
	}

	return g.Parser()
}
