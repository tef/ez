package ez

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

const (
	grammarAction     = "Grammar"
	defineAction      = "Define"
	printAction       = "Print"
	traceAction       = "Trace"
	callAction        = "Call"
	literalAction     = "Literal"
	whitespaceAction  = "Whitespace"
	newlineAction     = "Newline"
	partialTabAction  = "PartialTab"
	startOfLineAction = "StartOfLine"
	endOfLineAction   = "EndOfLine"
	startOfFileAction = "StartOfFie"
	endOfFileAction   = "EndOfFile"
	choiceAction      = "Choice"
	sequenceAction    = "Sequence"
	optionalAction    = "Optional"
	repeatAction      = "Repeat"
	lookaheadAction   = "Lookahead"
	rejectAction      = "Reject"
	captureAction     = "Capture"
	rangeAction       = "Range"
	indentAction      = "Indent"
	dedentAction      = "Dedent"
)

func Printf(format string, a ...any) {
	// used for g.LogFunc = ez.Printf
	fmt.Printf(format, a...)
}

type filePosition struct {
	file   string
	line   int
	rule   *int
	action string
}

type grammarError struct {
	g       *Grammar
	pos     int
	message string
	fatal   bool
}

func (e *grammarError) Error() string {
	p := e.g.posInfo[e.pos]
	if p.rule != nil {
		name := e.g.names[*p.rule]
		rulePos := e.g.posInfo[e.g.rulePos[*p.rule]]
		return fmt.Sprintf("%v:%v:%v: %v (inside %q at %v:%v)", p.file, p.line, p.action, e.message, name, rulePos.file, rulePos.line)

	} else {
		return fmt.Sprintf("%v:%v:%v: %v", p.file, p.line, p.action, e.message)

	}
}

type nodeBuilder struct {
	kind string
	rule *int
	args []*parseAction
}

func (b *nodeBuilder) buildSequence(pos int) *parseAction {
	if len(b.args) == 0 {
		return nil
	}
	if len(b.args) == 1 {
		return b.args[0]
	}
	return &parseAction{kind: sequenceAction, args: b.args, pos: pos}
}

func (b *nodeBuilder) buildNode(pos int, kind string) *parseAction {
	return &parseAction{kind: kind, args: b.args, pos: pos}

}

func (b *nodeBuilder) append(a *parseAction) {
	b.args = append(b.args, a)
}

func (b *nodeBuilder) inRule() bool {
	return b != nil && b.kind != grammarAction
}

type Grammar struct {
	Start       string
	Whitespaces []string
	Newlines    []string
	LogFunc     func(string, ...any)

	rules   []*parseAction
	names   []string
	nameIdx map[string]int

	// list of pos for each name
	callPos map[string][]int

	rulePos []int // posInfo[rulePos[ruleNum]]
	posInfo []filePosition

	nb *nodeBuilder

	pos    int // posInfo[pos] for grammar define
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

func (g *Grammar) markPosition(actionKind string) int {
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
	pos := filePosition{file: file, line: no, rule: rule, action: actionKind}
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
	newNb := &nodeBuilder{kind: defineAction, rule: &rule}
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
	g.nb = &nodeBuilder{kind: grammarAction}

	stub(g)
	g.nb = nil

	return g.Check()
}

func (g *Grammar) Define(name string, stub func()) {
	p := g.markPosition(defineAction)
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
	p := g.markPosition(printAction)
	if g.shouldExit(p) {
		return
	}
	a := &parseAction{kind: printAction, message: args, pos: p}
	g.nb.append(a)
}
func (g *Grammar) Trace(stub func()) {
	p := g.markPosition(traceAction)
	if g.shouldExit(p) {
		return
	}
	r := g.buildStub(traceAction, stub)
	if g.err != nil {
		return
	}

	a := &parseAction{kind: traceAction, args: r.args, pos: p}
	g.nb.append(a)
}

func (g *Grammar) Whitespace() {
	p := g.markPosition(whitespaceAction)
	if g.shouldExit(p) {
		return
	}
	a := &parseAction{kind: whitespaceAction, pos: p}
	g.nb.append(a)
}

func (g *Grammar) Newline() {
	p := g.markPosition(newlineAction)
	if g.shouldExit(p) {
		return
	}
	a := &parseAction{kind: newlineAction, pos: p}
	g.nb.append(a)
}

func (g *Grammar) Call(name string) {
	p := g.markPosition(callAction)
	if g.shouldExit(p) {
		return
	}
	g.callPos[name] = append(g.callPos[name], p)
	a := &parseAction{kind: callAction, name: name, pos: p}
	g.nb.append(a)
}

func (g *Grammar) Literal(s ...string) {
	p := g.markPosition(literalAction)
	if g.shouldExit(p) {
		return
	}
	if len(s) == 0 {
		g.Error(p, "missing operand")
		return
	}

	a := &parseAction{kind: literalAction, literals: s, pos: p}
	g.nb.append(a)
}
func (g *Grammar) Range(s ...string) {
	p := g.markPosition(rangeAction)
	if g.shouldExit(p) {
		return
	}
	if len(s) == 0 {
		g.Error(p, "missing operand")
		return
	}

	args := make([]string, len(s))
	for i, v := range s {
		if len(v) != 3 || v[1] != byte('-') {
			g.Error(p, "invalid range", v)
			return
		}
		args[i] = v
	}
	a := &parseAction{kind: rangeAction, ranges: args, pos: p}
	g.nb.append(a)
}
func (g *Grammar) Sequence(stub func()) {
	p := g.markPosition(sequenceAction)
	if g.shouldExit(p) {
		return
	}

	r := g.buildStub(sequenceAction, stub)

	if g.err != nil {
		return
	}

	a := r.buildSequence(p)
	g.nb.append(a)
}

func (g *Grammar) Capture(name string, stub func()) {
	p := g.markPosition(sequenceAction)
	if g.shouldExit(p) {
		return
	}

	r := g.buildStub(sequenceAction, stub)

	if g.err != nil {
		return
	}

	a := &parseAction{kind: captureAction, name: name, args: r.args, pos: p}
	g.nb.append(a)
}

func (g *Grammar) Choice(options ...func()) {
	p := g.markPosition(choiceAction)
	if g.shouldExit(p) {
		return
	}

	args := make([]*parseAction, len(options))
	for i, stub := range options {
		r := g.buildStub(choiceAction, stub)

		if g.err != nil {
			return
		}

		args[i] = r.buildSequence(p)
	}
	a := &parseAction{kind: choiceAction, args: args, pos: p}
	g.nb.append(a)
}

func (g *Grammar) Optional(stub func()) {
	p := g.markPosition(optionalAction)
	if g.shouldExit(p) {
		return
	}
	r := g.buildStub(optionalAction, stub)
	if g.err != nil {
		return
	}

	a := &parseAction{kind: optionalAction, args: r.args, pos: p}
	g.nb.append(a)
}

func (g *Grammar) Lookahead(stub func()) {
	p := g.markPosition(lookaheadAction)
	if g.shouldExit(p) {
		return
	}
	r := g.buildStub(lookaheadAction, stub)
	if g.err != nil {
		return
	}

	a := &parseAction{kind: lookaheadAction, args: r.args, pos: p}
	g.nb.append(a)
}

func (g *Grammar) Reject(stub func()) {
	p := g.markPosition(rejectAction)
	if g.shouldExit(p) {
		return
	}
	r := g.buildStub(rejectAction, stub)
	if g.err != nil {
		return
	}

	a := &parseAction{kind: rejectAction, args: r.args, pos: p}
	g.nb.append(a)
}

func (g *Grammar) Repeat(min_t int, max_t int, stub func()) {
	p := g.markPosition(repeatAction)
	if g.shouldExit(p) {
		return
	}

	r := g.buildStub(repeatAction, stub)

	if g.err != nil {
		return
	}

	a := &parseAction{kind: repeatAction, args: r.args, min: min_t, max: max_t, pos: p}
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

	rules := make([]parseFunc, len(g.rules))

	for k, v := range g.rules {
		rules[k] = v.buildFunc(g)
	}

	start := g.nameIdx[g.Start]

	p := &Parser{
		start:   start,
		rules:   rules,
		grammar: g,
	}
	return p, nil
}

type Node struct {
	name     string
	start    int
	end      int
	children []int
}

type NodeTree struct {
	nodes []Node
	root  int
}

type parseFunc func(*parserState) bool

type parserState struct {
	rules []parseFunc
	buf   string

	length int
	offset int

	nodes    []Node
	numNodes int
	children []int

	trace bool

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
func (s *parserState) trim(new *parserState) {
	s.nodes = s.nodes[:s.numNodes]
}
func (s *parserState) startCapture() *parserState {
	st := parserState{}
	st = *s
	st.children = []int{}
	return &st
}
func (s *parserState) mergeCapture(name string, new *parserState) {
	node := Node{
		name:     name,
		start:    s.offset,
		end:      new.offset,
		children: new.children,
	}
	new.nodes = append(new.nodes[:new.numNodes], node)
	new.children = append(s.children, new.numNodes)
	new.numNodes = new.numNodes + 1
	*s = *new
}

func (s *parserState) captureNode(name string) int {
	if len(s.children) == 1 {
		return s.children[0]
	} else {
		node := Node{
			name:     name,
			start:    0,
			end:      s.offset,
			children: s.children,
		}
		s.nodes = append(s.nodes[:s.numNodes], node)
		s.children = []int{}
		s.numNodes = s.numNodes + 1
		return s.numNodes - 1
	}
}

func (s *parserState) atEnd() bool {
	return s.offset >= s.length
}

func (s *parserState) peek() byte {
	return s.buf[s.offset]
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

type parseAction struct {
	kind     string
	pos      int
	name     string         // call, capture
	args     []*parseAction // choice, seq, cap
	literals []string
	ranges   []string

	min     int
	max     int
	message []any
}

func (a *parseAction) buildFunc(g *Grammar) parseFunc {
	switch a.kind {
	case printAction:
		p := g.posInfo[a.pos]
		r := g.names[*p.rule]
		prefix := fmt.Sprintf("%v:%v", p.file, p.line)
		fn := g.LogFunc
		return func(s *parserState) bool {
			msg := fmt.Sprint(a.message...)
			fn("%v: g.Print(%q) called (inside %q at pos %v)\n", prefix, msg, r, s.offset)
			return true
		}
	case traceAction:
		p := g.posInfo[a.pos]
		r := g.names[*p.rule]
		prefix := fmt.Sprintf("%v:%v", p.file, p.line)
		fn := g.LogFunc

		rules := make([]parseFunc, len(a.args))
		for i, r := range a.args {
			rules[i] = r.buildFunc(g)
		}

		return func(s *parserState) bool {
			fn("%v: Trace called (inside %q at pos %v)\n", prefix, r, s.offset)
			result := true

			s1 := s.clone()
			s1.trace = true
			for _, v := range rules {
				if !v(s1) {
					result = false
					break
				}
			}
			if result {
				s1.trace = false
				fn("%v: Trace exiting (inside %q at pos %v)\n", prefix, r, s1.offset)
				s.merge(s1)
			} else {
				fn("%v: g.Trace() failing (inside %q at pos %v)\n", prefix, r, s.offset)
			}
			return result
		}

	// case partialTabAction
	// case indentAction
	// case dedentAction

	case whitespaceAction:
		return func(s *parserState) bool {
			for {
				if !s.advanceAny(g.Whitespaces) {
					break
				}
			}
			return true
		}
	case newlineAction:
		return func(s *parserState) bool {
			return s.advanceAny(g.Newlines)
		}

	case startOfLineAction:
		return func(s *parserState) bool {
			return true // column = 0
		}
	case endOfLineAction:
		return func(s *parserState) bool {
			return s.advanceAny(g.Newlines)
		}
	case startOfFileAction:
		return func(s *parserState) bool {
			return s.offset == 0
		}
	case endOfFileAction:
		return func(s *parserState) bool {
			return s.offset == len(s.buf)
		}

	case literalAction:
		return func(s *parserState) bool {
			for _, v := range a.literals {
				if s.advance(v) {
					return true
				}
			}
			return false
		}
	case rangeAction:
		inverted := false
		return func(s *parserState) bool {
			if s.atEnd() {
				return false
			}
			r := s.peek()
			for _, v := range a.ranges {
				minR := v[0]
				maxR := v[2]
				if r >= minR && r <= maxR {
					s.offset += 1
					return !inverted
				}
			}
			return inverted
		}
	case callAction:
		p := g.posInfo[a.pos]
		prefix := fmt.Sprintf("%v:%v", p.file, p.line)
		fn := g.LogFunc
		name := a.name
		idx := g.nameIdx[name]
		return func(s *parserState) bool {
			if s.trace {
				fn("%v: %v called (at pos %v)\n", prefix, name, s.offset)
			}

			rule := s.rules[idx]
			out := rule(s)

			if s.trace {
				if out {
					fn("%v: %v exiting (at pos %v)\n", prefix, name, s.offset)
				} else {
					fn("%v: %v failing (at pos %v)\n", prefix, name, s.offset)
				}
			}
			return out
		}
	case optionalAction:
		rules := make([]parseFunc, len(a.args))
		for i, r := range a.args {
			rules[i] = r.buildFunc(g)
		}
		return func(s *parserState) bool {
			s1 := s.clone()
			for _, r := range rules {
				if !r(s1) {
					return true
				}
			}
			s.merge(s1)
			return true
		}
	case lookaheadAction:
		rules := make([]parseFunc, len(a.args))
		for i, r := range a.args {
			rules[i] = r.buildFunc(g)
		}
		return func(s *parserState) bool {
			s1 := s.clone()
			for _, r := range rules {
				if !r(s1) {
					return false
				}
			}
			return true
		}
	case rejectAction:
		rules := make([]parseFunc, len(a.args))
		for i, r := range a.args {
			rules[i] = r.buildFunc(g)
		}
		return func(s *parserState) bool {
			s1 := s.clone()
			for _, r := range rules {
				if !r(s1) {
					return true
				}
			}
			return false
		}

	case repeatAction:
		rules := make([]parseFunc, len(a.args))
		for i, r := range a.args {
			rules[i] = r.buildFunc(g)
		}
		min_n := a.min
		max_n := a.max

		return func(s *parserState) bool {
			c := 0
			s1 := s.clone()
			for {
				for _, r := range rules {
					if !r(s1) {
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

	case choiceAction:
		rules := make([]parseFunc, len(a.args))
		for i, r := range a.args {
			rules[i] = r.buildFunc(g)
		}
		return func(s *parserState) bool {
			for _, r := range rules {
				s1 := s.clone()
				if r(s1) {
					s.merge(s1)
					return true
				}
				s.trim(s1)
			}
			return false
		}
	case sequenceAction:
		rules := make([]parseFunc, len(a.args))
		for i, r := range a.args {
			rules[i] = r.buildFunc(g)
		}
		return func(s *parserState) bool {
			s1 := s.clone()
			for _, r := range rules {
				if !r(s1) {
					return false
				}
			}
			s.merge(s1)
			return true
		}
	case captureAction:
		rules := make([]parseFunc, len(a.args))
		for i, r := range a.args {
			rules[i] = r.buildFunc(g)
		}
		return func(s *parserState) bool {
			s1 := s.startCapture()
			for _, r := range rules {
				if !r(s1) {
					return false
				}
			}
			s.mergeCapture(a.name, s1)
			return true
		}
	default:
		return func(s *parserState) bool {
			return true
		}
	}
}

type Parser struct {
	rules   []parseFunc
	start   int
	grammar *Grammar
	Err     error
}

func (p *Parser) testString(s string) *NodeTree {
	parserState := &parserState{
		rules:  p.rules,
		buf:    s,
		length: len(s),
	}
	start := p.rules[p.start]
	if start(parserState) && parserState.atEnd() {
		name := p.grammar.Start
		n := parserState.captureNode(name)
		return &NodeTree{root: n, nodes: parserState.nodes}
	}
	return nil
}

func (p *Parser) testGrammar(accept []string, reject []string) bool {
	start := p.rules[p.start]
	return p.testParseFunc(start, accept, reject)
}

func (p *Parser) testRule(name string, accept []string, reject []string) bool {
	start := p.rules[p.grammar.nameIdx[name]]
	return p.testParseFunc(start, accept, reject)
}

func (p *Parser) testParseFunc(rule parseFunc, accept []string, reject []string) bool {
	for _, s := range accept {
		parserState := &parserState{
			rules:  p.rules,
			buf:    s,
			length: len(s),
		}
		complete := rule(parserState) && parserState.atEnd()

		if !complete {
			return false
		}
	}
	for _, s := range reject {
		parserState := &parserState{
			rules:  p.rules,
			buf:    s,
			length: len(s),
		}
		complete := rule(parserState) && parserState.atEnd()

		if complete {
			return false
		}
	}
	return true
}

func BuildGrammar(stub func(*Grammar)) (*Grammar, error) {
	g := &Grammar{}
	g.LogFunc = Printf
	g.pos = g.markPosition(grammarAction)
	err := g.buildGrammar(stub)
	if err != nil {
		return nil, err
	}
	return g, nil
}

func BuildParser(stub func(*Grammar)) (*Parser, error) {
	g := &Grammar{}
	g.LogFunc = Printf
	g.pos = g.markPosition(grammarAction)
	err := g.buildGrammar(stub)
	if err != nil {
		return nil, err
	}

	return g.Parser()
}
