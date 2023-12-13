package ez

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"unicode/utf8"
)

const (
	grammarAction = "Grammar"
	defineAction  = "Define"
	builderAction = "Builder"

	printAction = "Print"
	traceAction = "Trace"

	callAction      = "Call"
	choiceAction    = "Choice"
	cutAction       = "Cut"
	sequenceAction  = "Sequence"
	optionalAction  = "Optional"
	repeatAction    = "Repeat"
	lookaheadAction = "Lookahead"
	rejectAction    = "Reject"
	captureAction   = "Capture"

	startOfFileAction = "StartOfFie"
	endOfFileAction   = "EndOfFile"

	peekAction       = "Peek"
	runeAction       = "Rune"
	peekRuneAction   = "PeekRune"
	literalAction    = "Literal"
	rangeAction      = "Range"
	whitespaceAction = "Whitespace"
	newlineAction    = "Newline"

	partialTabAction  = "PartialTab"
	startOfLineAction = "StartOfLine"
	endOfLineAction   = "EndOfLine"

	indentAction = "Indent"
	dedentAction = "Dedent"

	byteAction       = "Byte"
	byteRangeAction  = "ByteRange"
	peekByteAction   = "PeekByte"
	byteListAction   = "Bytes"
	byteStringAction = "ByteString"
)

var ParseError = errors.New("failed to parse")

func Printf(format string, a ...any) {
	// used for g.LogFunc = ez.Printf
	fmt.Printf(format, a...)
}

func TextMode() *textMode {
	return &textMode{
		whitespace: []string{" ", "\t"},
		newline:    []string{"\r\n", "\r", "\n"},
		tabstop:    8,
		actionsDisabled: []string{
			byteAction,
			byteRangeAction,
			byteListAction,
			byteStringAction,
		},
	}
}
func StringMode() ParserMode {
	return &stringMode{
		whitespace: []string{" ", "\t", "\r\n", "\r", "\n"},
		newline:    []string{"\r\n", "\r", "\n"},
		actionsDisabled: []string{
			partialTabAction,
			startOfLineAction,
			endOfLineAction,
			indentAction,
			dedentAction,
		},
	}
}

func BinaryMode() ParserMode {
	return &binaryMode{
		actionsDisabled: []string{
			runeAction,
			literalAction,
			whitespaceAction,
			newlineAction,
			partialTabAction,
			startOfLineAction,
			endOfLineAction,
			rangeAction,
			indentAction,
			dedentAction,
		},
	}
}

func BuildGrammar(stub func(*Grammar)) *Grammar {
	g := &Grammar{}
	g.LogFunc = Printf
	g.pos = g.markPosition(grammarAction)

	if stub == nil {
		g.Error(g.pos, "cant call BuildGrammar() with nil")
		return g
	}

	err := g.buildGrammar(stub)
	if err != nil {
		return &Grammar{err: err}
	}
	return g
}

func BuildParser(stub func(*Grammar)) *Parser {
	g := &Grammar{}
	g.LogFunc = Printf
	g.pos = g.markPosition(grammarAction)
	if stub == nil {
		g.Error(g.pos, "cant call BuildParser() with nil")
		return &Parser{err: g.err}
	}

	err := g.buildGrammar(stub)
	if err != nil {
		return &Parser{err: err}
	}

	return g.Parser()
}

//
//  	Grammar Builder
//

type ParserMode interface {
	name() string
	columnParser() columnParserFunc
	actionAllowed(string) bool
	check() bool
	whitespaces() []string
	newlines() []string
	getTabstop() int
}

type binaryMode struct {
	actionsDisabled []string
}

func (m *binaryMode) name() string {
	return "binary mode"
}

func (m *binaryMode) columnParser() columnParserFunc {
	return nil
}

func (m *binaryMode) actionAllowed(s string) bool {
	for _, v := range m.actionsDisabled {
		if v == s {
			return false
		}
	}
	return true
}

func (m *binaryMode) check() bool {
	return true
}

func (m *binaryMode) newlines() []string {
	return []string{}
}

func (m *binaryMode) whitespaces() []string {
	return []string{}
}

func (m *binaryMode) getTabstop() int {
	return 0
}

type stringMode struct {
	whitespace []string
	// Tab
	newline         []string
	actionsDisabled []string
}

func (m *stringMode) columnParser() columnParserFunc {
	return nil
}

func (m *stringMode) name() string {
	return "string mode"
}

func (m *stringMode) actionAllowed(s string) bool {
	for _, v := range m.actionsDisabled {
		if v == s {
			return false
		}
	}
	return true
}

func (m *stringMode) check() bool {
	return true
}

func (m *stringMode) newlines() []string {
	return m.newline
}

func (m *stringMode) whitespaces() []string {
	return m.whitespace
}

func (m *stringMode) getTabstop() int {
	return 0
}

type textMode struct {
	whitespace      []string
	newline         []string
	tabstop         int
	actionsDisabled []string
}

func (m *textMode) Tabstop(t int) *textMode {
	m.tabstop = t
	return m
}

func (m *textMode) columnParser() columnParserFunc {
	return textModeColumnParser
}

func (m *textMode) name() string {
	return "text mode"
}

func (m *textMode) actionAllowed(s string) bool {
	for _, v := range m.actionsDisabled {
		if v == s {
			return false
		}
	}
	return true
}
func (m *textMode) check() bool {
	return true
}

func (m *textMode) newlines() []string {
	return m.newline
}

func (m *textMode) whitespaces() []string {
	return m.whitespace
}

func (m *textMode) getTabstop() int {
	return m.tabstop
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
	prefix := e.g.formatPosition(&p)
	return fmt.Sprintf("%v: error in %v(): %v", prefix, p.action, e.message)
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

type BuilderFunc func(string, []any) (any, error)

type Grammar struct {
	Start   string
	Mode    ParserMode
	LogFunc func(string, ...any)

	rules   []*parseAction
	names   []string
	nameIdx map[string]int

	// list of pos for each name
	callPos    map[string][]int
	capturePos map[string][]int

	rulePos []int // posInfo[rulePos[ruleNum]]
	posInfo []filePosition

	builders   map[string]BuilderFunc
	builderPos map[string]int

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

func (g *Grammar) shouldExit(pos int, kind string) bool {
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
	if g.Mode == nil {
		g.Error(pos, "no Mode set")
		return true
	}
	if !g.Mode.actionAllowed(kind) {
		g.Errorf(pos, "cannot call %v() in %v", kind, g.Mode.name())
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
	g.capturePos = make(map[string][]int, 0)
	g.builders = make(map[string]BuilderFunc, 0)
	g.builderPos = make(map[string]int, 0)
	g.nb = &nodeBuilder{kind: grammarAction}
	g.Mode = TextMode()

	stub(g)
	g.nb = nil

	return g.Check()
}

func (g *Grammar) formatPosition(p *filePosition) string {
	if p.rule != nil {
		return fmt.Sprintf("%v:%v:%v", p.file, p.line, g.names[*p.rule])
	} else {
		return fmt.Sprintf("%v:%v", p.file, p.line)
	}

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
	} else if stub == nil {
		g.Error(p, "cant call Define() with nil")
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
	if g.shouldExit(p, printAction) {
		return
	}
	a := &parseAction{kind: printAction, message: args, pos: p}
	g.nb.append(a)
}
func (g *Grammar) Trace(stub func()) {
	p := g.markPosition(traceAction)
	if g.shouldExit(p, traceAction) {
		return
	} else if stub == nil {
		g.Error(p, "cant call Trace() with nil")
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
	if g.shouldExit(p, whitespaceAction) {
		return
	}
	a := &parseAction{kind: whitespaceAction, pos: p}
	g.nb.append(a)
}

func (g *Grammar) Newline() {
	p := g.markPosition(newlineAction)
	if g.shouldExit(p, newlineAction) {
		return
	}
	a := &parseAction{kind: newlineAction, pos: p}
	g.nb.append(a)
}

func (g *Grammar) StartOfFile() {
	p := g.markPosition(startOfFileAction)
	if g.shouldExit(p, startOfFileAction) {
		return
	}

	a := &parseAction{kind: startOfFileAction, pos: p}
	g.nb.append(a)
}

func (g *Grammar) EndOfFile() {
	p := g.markPosition(endOfFileAction)
	if g.shouldExit(p, endOfFileAction) {
		return
	}

	a := &parseAction{kind: endOfFileAction, pos: p}
	g.nb.append(a)
}

func (g *Grammar) StartOfLine() {
	p := g.markPosition(startOfLineAction)
	if g.shouldExit(p, startOfLineAction) {
		return
	}

	a := &parseAction{kind: startOfLineAction, pos: p}
	g.nb.append(a)
}

func (g *Grammar) EndOfLine() {
	p := g.markPosition(endOfLineAction)
	if g.shouldExit(p, endOfLineAction) {
		return
	}

	a := &parseAction{kind: endOfLineAction, pos: p}
	g.nb.append(a)
}

func (g *Grammar) Rune() {
	p := g.markPosition(runeAction)
	if g.shouldExit(p, runeAction) {
		return
	}

	a := &parseAction{kind: runeAction, pos: p}
	g.nb.append(a)
}

func (g *Grammar) Peek(stubs map[string]func()) {
	p := g.markPosition(peekAction)
	if g.shouldExit(p, peekAction) {
		return
	} else if stubs == nil {
		g.Error(p, "cant call Peek() with nil map")
		return
	}

	args := make(map[string]*parseAction, len(stubs))
	for c, stub := range stubs {
		if stub == nil {
			g.Error(p, "cant call Peek() with nil function")
			return
		}

		r := g.buildStub(peekAction, stub)

		if g.err != nil {
			return
		}

		args[c] = r.buildSequence(p)
	}
	a := &parseAction{kind: peekAction, stringSwitch: args, pos: p}
	g.nb.append(a)
}
func (g *Grammar) PeekRune(stubs map[rune]func()) {
	p := g.markPosition(peekRuneAction)
	if g.shouldExit(p, peekRuneAction) {
		return
	} else if stubs == nil {
		g.Error(p, "cant call PeekRune() with nil map")
		return
	}

	args := make(map[rune]*parseAction, len(stubs))
	for c, stub := range stubs {
		if stub == nil {
			g.Error(p, "cant call PeekRune() with nil function")
			return
		}

		r := g.buildStub(peekRuneAction, stub)

		if g.err != nil {
			return
		}

		args[c] = r.buildSequence(p)
	}
	a := &parseAction{kind: peekRuneAction, runeSwitch: args, pos: p}
	g.nb.append(a)
}
func (g *Grammar) Literal(s ...string) {
	p := g.markPosition(literalAction)
	if g.shouldExit(p, literalAction) {
		return
	}
	if len(s) == 0 {
		g.Error(p, "missing operand")
		return
	}

	a := &parseAction{kind: literalAction, literals: s, pos: p}
	g.nb.append(a)
}

func (g *Grammar) Range(s ...string) RangeOptions {
	p := g.markPosition(rangeAction)
	ro := RangeOptions{
		g: g,
		p: p,
	}
	if g.shouldExit(p, rangeAction) {
		return ro
	}
	if len(s) == 0 {
		g.Error(p, "missing operand")
		return ro
	}

	args := make([]string, len(s))
	for i, v := range s {
		r := []rune(v)
		if !(len(r) == 1 || (len(r) == 3 && r[1] == '-' && r[0] < r[2])) {
			g.Error(p, "invalid range", v)
			return ro
		}
		args[i] = v
	}
	a := &parseAction{kind: rangeAction, ranges: args, pos: p}
	ro.a = a
	g.nb.append(a)
	return ro
}

func (g *Grammar) Byte() {
	p := g.markPosition(byteAction)
	if g.shouldExit(p, byteAction) {
		return
	}

	a := &parseAction{kind: byteAction, pos: p}
	g.nb.append(a)
}

func (g *Grammar) PeekByte(stubs map[byte]func()) {
	p := g.markPosition(peekByteAction)
	if g.shouldExit(p, peekByteAction) {
		return
	} else if stubs == nil {
		g.Error(p, "cant call PeekByte() with nil map")
		return
	}

	args := make(map[byte]*parseAction, len(stubs))
	for c, stub := range stubs {
		if stub == nil {
			g.Error(p, "cant call PeekByte() with nil function")
			return
		}

		r := g.buildStub(peekByteAction, stub)

		if g.err != nil {
			return
		}

		args[c] = r.buildSequence(p)
	}
	a := &parseAction{kind: peekByteAction, byteSwitch: args, pos: p}
	g.nb.append(a)
}

func (g *Grammar) ByteString(s ...string) {
	p := g.markPosition(byteStringAction)
	if g.shouldExit(p, byteStringAction) {
		return
	}
	if len(s) == 0 {
		g.Error(p, "missing operand")
		return
	}

	b := make([][]byte, len(s))
	for i, v := range s {
		b[i] = []byte(v)
	}

	a := &parseAction{kind: byteStringAction, bytes: b, pos: p}
	g.nb.append(a)
}

func (g *Grammar) Bytes(s ...[]byte) {
	p := g.markPosition(byteListAction)
	if g.shouldExit(p, byteListAction) {
		return
	}
	if len(s) == 0 {
		g.Error(p, "missing operand")
		return
	}

	a := &parseAction{kind: byteListAction, bytes: s, pos: p}
	g.nb.append(a)
}

func (g *Grammar) ByteRange(s ...string) RangeOptions {
	p := g.markPosition(byteRangeAction)
	ro := RangeOptions{
		g: g,
		p: p,
	}
	if g.shouldExit(p, byteRangeAction) {
		return ro
	}
	if len(s) == 0 {
		g.Error(p, "missing operand")
		return ro
	}

	args := make([]string, len(s))
	for i, v := range s {
		r := []byte(v)
		if !(len(r) == 1 || (len(r) == 3 && r[1] == '-' && r[0] < r[2])) {
			g.Error(p, "invalid range", v)
			return ro
		}
		args[i] = v
	}
	a := &parseAction{kind: byteRangeAction, ranges: args, pos: p}
	ro.a = a
	g.nb.append(a)
	return ro
}

type RangeOptions struct {
	g *Grammar
	a *parseAction
	p int
}

func (ro RangeOptions) Invert() RangeOptions {
	return ro.g.invertRange(ro.p, ro.a)
}

func (g *Grammar) invertRange(rangePos int, a *parseAction) RangeOptions {
	p := g.markPosition(rangeAction)
	ro := RangeOptions{
		g: g,
		p: p,
	}
	if g.shouldExit(p, a.kind) || a == nil {
		return ro
	}

	if p-rangePos != 1 {
		g.Error(p, "called in wrong position")
		return ro
	}
	a.inverted = !a.inverted
	ro.a = a
	return ro
}

func (g *Grammar) Call(name string) {
	p := g.markPosition(callAction)
	if g.shouldExit(p, callAction) {
		return
	}
	g.callPos[name] = append(g.callPos[name], p)
	a := &parseAction{kind: callAction, name: name, pos: p}
	g.nb.append(a)
}

func (g *Grammar) Sequence(stub func()) {
	p := g.markPosition(sequenceAction)
	if g.shouldExit(p, sequenceAction) {
		return
	} else if stub == nil {
		g.Error(p, "cant call Sequence() with nil")
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
	if g.shouldExit(p, captureAction) {
		return
	} else if stub == nil {
		g.Error(p, "cant call Capture() with nil")
	}

	r := g.buildStub(sequenceAction, stub)

	if g.err != nil {
		return
	}

	g.capturePos[name] = append(g.capturePos[name], p)

	a := &parseAction{kind: captureAction, name: name, args: r.args, pos: p}
	g.nb.append(a)
}

func (g *Grammar) Cut() {
	p := g.markPosition(cutAction)
	if g.shouldExit(p, cutAction) {
		return
	}
	if g.nb.kind != choiceAction {
		g.Error(p, "cut must be called directly inside choice, sorry.")
		return
	}

	a := &parseAction{kind: cutAction, pos: p}
	g.nb.append(a)
}

func (g *Grammar) Choice(options ...func()) {
	p := g.markPosition(choiceAction)
	if g.shouldExit(p, choiceAction) {
		return
	} else if options == nil {
		g.Error(p, "cant call Choice() with nil")
		return
	}

	args := make([]*parseAction, len(options))
	for i, stub := range options {
		if stub == nil {
			g.Error(p, "cant call Choice() with nil")
			return
		}

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
	if g.shouldExit(p, optionalAction) {
		return
	} else if stub == nil {
		g.Error(p, "cant call Optional() with nil")
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
	if g.shouldExit(p, lookaheadAction) {
		return
	} else if stub == nil {
		g.Error(p, "cant call Lookahead() with nil")
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
	if g.shouldExit(p, rejectAction) {
		return
	} else if stub == nil {
		g.Error(p, "cant call Reject() with nil")
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
	if g.shouldExit(p, repeatAction) {
		return
	} else if stub == nil {
		g.Error(p, "cant call Repeat() with nil")
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
	if !g.Mode.check() {
		g.Error(g.pos, "misconfigured mode")
		return g.err
	}

	if g.Start == "" && len(g.names) == 1 {
		g.Start = g.names[0]
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

	for name, _ := range g.builders {
		if _, ok := g.capturePos[name]; !ok {
			p := g.builderPos[name]
			g.Errorf(p, "missing capture %q for builder", name)
		}
	}

	if len(g.builders) > 0 {
		for name, pos := range g.capturePos {
			if _, ok := g.builderPos[name]; !ok {
				for _, p := range pos {
					g.Errorf(p, "missing builder %q for capture", name)
				}
			}
		}
	}

	if g.Start == "" {
		g.Error(g.pos, "starting rule undefined")
	} else if _, ok := g.nameIdx[g.Start]; !ok {
		g.Errorf(g.pos, "starting rule %q is missing", g.Start)
	}

	return g.err
}

func (g *Grammar) Builder(name string, stub BuilderFunc) {
	p := g.markPosition(builderAction)
	if g.err != nil {
		return
	} else if g.nb == nil {
		g.Error(p, "must call Builder() inside grammar")
		return
	} else if g.nb.inRule() {
		g.Error(p, "cant call Builder() inside define")
		return
	} else if stub == nil {
		g.Error(p, "cant call Builder() with nil")
	}

	if _, ok := g.builders[name]; ok {
		oldPos := g.builderPos[name]
		g.Errorf(p, "cant redefine %q, already defined at %v", name, oldPos)
		return
	}
	g.builderPos[name] = p
	g.builders[name] = stub
}

func (g *Grammar) Parser() *Parser {
	if g.Check() != nil {
		p := &Parser{err: g.err}
		return p
	}

	rules := make([]parseFunc, len(g.rules))

	for k, v := range g.rules {
		rules[k] = v.buildFunc(g)
	}

	start := g.nameIdx[g.Start]

	p := &Parser{
		start:    start,
		rules:    rules,
		builders: g.builders,
		grammar:  g,
	}
	return p
}

//
//  	Parser
//

type parseFunc func(*parserState) bool

type parseAction struct {
	// this is a 'wide style' variant struct

	kind     string
	pos      int
	name     string         // call, capture
	args     []*parseAction // choice, seq, cap
	literals []string
	ranges   []string
	bytes    [][]byte

	stringSwitch map[string]*parseAction
	runeSwitch   map[rune]*parseAction
	byteSwitch   map[byte]*parseAction

	min      int
	max      int
	inverted bool
	message  []any
}

func (a *parseAction) buildFunc(g *Grammar) parseFunc {
	if a == nil {
		// when a func() stub has no rules
		return func(s *parserState) bool {
			return true
		}
	}
	switch a.kind {
	case printAction:
		p := g.posInfo[a.pos]
		prefix := g.formatPosition(&p)
		fn := g.LogFunc
		return func(s *parserState) bool {
			msg := fmt.Sprint(a.message...)
			fn("%v: Print(%q) called, at offset %v, column %v\n", prefix, msg, s.offset, s.column)
			return true
		}
	case traceAction:
		p := g.posInfo[a.pos]
		prefix := g.formatPosition(&p)
		fn := g.LogFunc

		rules := make([]parseFunc, len(a.args))
		for i, r := range a.args {
			rules[i] = r.buildFunc(g)
		}

		return func(s *parserState) bool {
			fn("%v: Trace() starting, at offset %v\n", prefix, s.offset)
			result := true

			var s1 parserState
			s.copyInto(&s1)
			s1.trace = true
			for _, v := range rules {
				if !v(&s1) {
					result = false
					break
				}
			}
			if result {
				s1.trace = false
				fn("%v: Trace() ending, at offset %v\n", prefix, s1.offset)
				s.merge(&s1)
			} else {
				s.choiceExit = s1.choiceExit
				fn("%v: Trace() failing, at offset %v\n", prefix, s.offset)
			}
			return result
		}
	case callAction:
		p := g.posInfo[a.pos]
		prefix := g.formatPosition(&p)

		name := a.name
		idx := g.nameIdx[name]
		fn := g.LogFunc
		return func(s *parserState) bool {
			if s.trace {
				fn("%v: Call(%q) starting, at offset %v\n", prefix, name, s.offset)
			}

			rule := s.i.rules[idx] // can't move this out to runtime unless we reorder defs
			out := rule(s)

			if s.trace {
				if out {
					fn("%v: Call(%q) exiting, at offset %v\n", prefix, name, s.offset)
				} else {
					fn("%v: Call(%q) failing, at offset %v\n", prefix, name, s.offset)
				}
			}
			return out
		}

	// case partialTabAction
	// case indentAction
	// case dedentAction

	case whitespaceAction:
		ws := g.Mode.whitespaces()
		return func(s *parserState) bool {
			for {
				if !s.acceptAny(ws) {
					break
				}
			}
			return true
		}
	case endOfLineAction, newlineAction:
		nl := g.Mode.newlines()
		return func(s *parserState) bool {
			return s.acceptAny(nl)
		}

	case startOfLineAction:
		return func(s *parserState) bool {
			return s.column == 0
		}
	case startOfFileAction:
		return func(s *parserState) bool {
			return s.offset == 0
		}
	case endOfFileAction:
		return func(s *parserState) bool {
			return s.offset == s.i.length
		}

	case runeAction:
		return func(s *parserState) bool {
			if s.atEnd() {
				return false
			}
			_, n := s.peekRune()
			s.advance(n)
			return true
		}
	case byteAction:
		return func(s *parserState) bool {
			if s.atEnd() {
				return false
			}
			s.advance(1)
			return true
		}
	case literalAction:
		return func(s *parserState) bool {
			for _, v := range a.literals {
				if s.acceptString(v) {
					return true
				}
			}
			return false
		}
	case byteListAction, byteStringAction:
		return func(s *parserState) bool {
			for _, v := range a.bytes {
				if s.acceptBytes(v) {
					return true
				}
			}
			return false
		}
	case peekAction:
		rules := make(map[string]parseFunc, len(a.stringSwitch))
		size := 0
		for i, r := range a.stringSwitch {
			rules[i] = r.buildFunc(g)
			if len(i) > size {
				size = len(i)
			}
		}
		return func(s *parserState) bool {
			if s.atEnd() {
				return false
			}
			r := s.peekString(size)

			if fn, ok := rules[r]; ok {
				return fn(s)
			}

			return false
		}
	case peekRuneAction:
		rules := make(map[rune]parseFunc, len(a.runeSwitch))
		for i, r := range a.runeSwitch {
			rules[i] = r.buildFunc(g)
		}
		return func(s *parserState) bool {
			if s.atEnd() {
				return false
			}
			r, _ := s.peekRune()

			if fn, ok := rules[r]; ok {
				return fn(s)
			}

			return false
		}
	case peekByteAction:
		rules := make(map[byte]parseFunc, len(a.byteSwitch))
		for i, r := range a.byteSwitch {
			rules[i] = r.buildFunc(g)
		}
		return func(s *parserState) bool {
			if s.atEnd() {
				return false
			}
			r := s.peekByte()

			if fn, ok := rules[r]; ok {
				return fn(s)
			}

			return false
		}
	case rangeAction:
		inverted := a.inverted
		runeRanges := make([][]rune, len(a.ranges))
		for i, v := range a.ranges {
			n := []rune(v)
			if len(n) == 1 {
				runeRanges[i] = []rune{n[0], n[0]}
			} else {
				runeRanges[i] = []rune{n[0], n[2]}
			}
		}
		return func(s *parserState) bool {
			if s.atEnd() {
				return false
			}
			r, size := s.peekRune()
			result := false
			for _, v := range runeRanges {
				minR := v[0]
				maxR := v[1]
				if r >= minR && r <= maxR {
					result = true
					break
				}
			}
			if inverted {
				result = !result
			}

			if result {
				s.advance(size)
				return true
			}

			return false
		}
	case byteRangeAction:
		inverted := a.inverted
		byteRanges := make([][]byte, len(a.ranges))
		for i, v := range a.ranges {
			n := []byte(v)
			if len(n) == 1 {
				byteRanges[i] = []byte{n[0], n[0]}
			} else {
				byteRanges[i] = []byte{n[0], n[2]}
			}
		}
		return func(s *parserState) bool {
			if s.atEnd() {
				return false
			}
			r := s.peekByte()
			result := false
			for _, v := range byteRanges {
				minR := v[0]
				maxR := v[1]
				if r >= minR && r <= maxR {
					result = true
					break
				}
			}
			if inverted {
				result = !result
			}

			if result {
				s.advance(1)
				return true
			}

			return false
		}
	case optionalAction:
		rules := make([]parseFunc, len(a.args))
		for i, r := range a.args {
			rules[i] = r.buildFunc(g)
		}
		return func(s *parserState) bool {
			var s1 parserState
			s.copyInto(&s1)
			for _, r := range rules {
				if !r(&s1) {
					s.choiceExit = s1.choiceExit
					return true
				}
			}
			s.choiceExit = s1.choiceExit
			s.merge(&s1)
			return true
		}
	case lookaheadAction:
		rules := make([]parseFunc, len(a.args))
		for i, r := range a.args {
			rules[i] = r.buildFunc(g)
		}
		return func(s *parserState) bool {
			var s1 parserState
			s.copyInto(&s1)
			for _, r := range rules {
				if !r(&s1) {
					s.choiceExit = s1.choiceExit
					return false
				}
			}
			s.choiceExit = s1.choiceExit
			return true
		}
	case rejectAction:
		rules := make([]parseFunc, len(a.args))
		for i, r := range a.args {
			rules[i] = r.buildFunc(g)
		}
		return func(s *parserState) bool {
			var s1 parserState
			s.copyInto(&s1)
			for _, r := range rules {
				if !r(&s1) {
					s.choiceExit = s1.choiceExit
					return true
				}
			}
			s.choiceExit = s1.choiceExit
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
			var s1 parserState
			s.copyInto(&s1)
			for {
				for _, r := range rules {
					if !r(&s1) {
						s.choiceExit = s1.choiceExit
						return c >= min_n
					}
				}
				if c >= min_n {
					s.merge(&s1)
				} else {
					s.choiceExit = s1.choiceExit
				}

				if max_n != 0 && c >= max_n {
					break
				}
			}
			return c >= min_n
		}

	case cutAction:
		return func(s *parserState) bool {
			s.choiceExit = true
			return true
		}
	case choiceAction:
		rules := make([]parseFunc, len(a.args))
		for i, r := range a.args {
			rules[i] = r.buildFunc(g)
		}
		return func(s *parserState) bool {
			for _, r := range rules {
				var s1 parserState
				s.copyInto(&s1)
				s1.choiceExit = false
				if r(&s1) {
					s1.choiceExit = s.choiceExit
					s.merge(&s1)
					return true
				}
				s.trim(&s1)
				if s1.choiceExit {
					break
				}
			}
			return false
		}
	case sequenceAction:
		rules := make([]parseFunc, len(a.args))
		for i, r := range a.args {
			rules[i] = r.buildFunc(g)
		}
		return func(s *parserState) bool {
			var s1 parserState
			s.copyInto(&s1)
			for _, r := range rules {
				if !r(&s1) {
					s.choiceExit = s1.choiceExit
					return false
				}
			}
			s.merge(&s1)
			return true
		}
	case captureAction:
		rules := make([]parseFunc, len(a.args))
		for i, r := range a.args {
			rules[i] = r.buildFunc(g)
		}
		return func(s *parserState) bool {
			var s1 parserState
			s.startCapture(&s1)
			for _, r := range rules {
				if !r(&s1) {
					s.choiceExit = s1.choiceExit
					return false
				}
			}
			s.choiceExit = s1.choiceExit
			s.mergeCapture(a.name, &s1)
			return true
		}
	default:
		return func(s *parserState) bool {
			return true
		}
	}
}

type columnParserFunc func(string, int, int, int, int) int

func textModeColumnParser(buf string, column int, tabstop int, oldOffset int, newOffset int) int {
	for i := oldOffset; i < newOffset; i++ {
		switch buf[i] {
		case byte('\t'):
			width := tabstop - (column % tabstop)
			column += width
		case byte('\r'):
			column = 0
		case byte('\n'):
			if i > 1 && buf[i-1] != byte('\r') {
				column = 0
			}

		default:
			column += 1
		}
	}
	return column
}

type parserInput struct {
	rules        []parseFunc
	buf          string
	length       int
	columnParser columnParserFunc
}

type parserState struct {
	i      *parserInput
	offset int

	choiceExit bool
	column     int
	tabstop    int

	nodes    []Node
	numNodes int

	lastSibling  int
	countSibling int
	// children []int

	trace bool

	// indent_column int
	// for when we match n whitespace against a tab
	// leftover_tab int
	// leftover_tab pos
	// indents, dedent
	// parent
	// values map[string]any

}

func (s *parserState) copyInto(into *parserState) {
	*into = *s
}

func (s *parserState) merge(new *parserState) {
	*s = *new
}
func (s *parserState) trim(new *parserState) {
	s.nodes = s.nodes[:s.numNodes]
}
func (s *parserState) startCapture(st *parserState) {
	*st = *s
	// st.children = []int{}
	st.countSibling = 0
	st.lastSibling = 0
}

func (s *parserState) mergeCapture(name string, new *parserState) {

	nextSibling := new.lastSibling
	lastSibling := 0

	for i := 0; i < new.countSibling; i++ {
		nodeSibling := new.nodes[nextSibling].sibling

		new.nodes[nextSibling].sibling = lastSibling
		new.nodes[nextSibling].nsibling = i

		lastSibling = nextSibling
		nextSibling = nodeSibling
	}

	new.lastSibling = lastSibling

	node := Node{
		name:  name,
		start: s.offset,
		end:   new.offset,
		//	children: new.children,
		sibling:  s.lastSibling,
		nsibling: s.countSibling,
		child:    new.lastSibling,
		nchild:   new.countSibling,
	}

	new.nodes = append(new.nodes[:new.numNodes], node)
	// new.children = append(s.children, new.numNodes)
	new.lastSibling = new.numNodes
	new.countSibling = s.countSibling + 1
	new.numNodes = new.numNodes + 1
	*s = *new

}

func (s *parserState) captureNode(name string) int {
	if s.countSibling == 1 {
		return s.lastSibling
	} else {
		nextSibling := s.lastSibling
		lastSibling := 0

		for i := 0; i < s.countSibling; i++ {
			nodeSibling := s.nodes[nextSibling].sibling

			s.nodes[nextSibling].sibling = lastSibling
			s.nodes[nextSibling].nsibling = i

			lastSibling = nextSibling
			nextSibling = nodeSibling
		}

		s.lastSibling = lastSibling

		node := Node{
			name:  name,
			start: 0,
			end:   s.offset,
			//	children: s.children,
			child:  s.lastSibling,
			nchild: s.countSibling,
		}
		s.nodes = append(s.nodes[:s.numNodes], node)
		// s.children = []int{}
		s.countSibling = 0
		s.lastSibling = s.numNodes
		s.numNodes = s.numNodes + 1
		return s.numNodes - 1
	}
}

func (s *parserState) atEnd() bool {
	return s.offset >= s.i.length
}

func (s *parserState) peekByte() byte {
	return s.i.buf[s.offset]
}
func (s *parserState) peekString(n int) string {
	end := s.offset + n
	if end > s.i.length {
		end = s.i.length
	}
	return s.i.buf[s.offset:end]
}

func (s *parserState) peekRune() (rune, int) {
	return utf8.DecodeRuneInString(s.i.buf[s.offset:])
}

func (s *parserState) advance(length int) {
	newOffset := s.offset + length
	if s.i.columnParser != nil {
		s.column = s.i.columnParser(s.i.buf, s.column, s.tabstop, s.offset, newOffset)
	}
	s.offset = newOffset

}

func (s *parserState) acceptString(v string) bool {
	length_v := len(v)
	if length_v+s.offset > s.i.length {
		return false
	}
	b := s.i.buf[s.offset : s.offset+length_v]
	if b == v {
		s.advance(length_v)
		return true
	}
	return false
}
func (s *parserState) acceptBytes(v []byte) bool {
	length_v := len(v)
	if length_v+s.offset > s.i.length {
		return false
	}
	b := s.i.buf[s.offset : s.offset+length_v]
	if b == string(v) {
		s.advance(length_v)
		return true
	}
	return false
}

func (s *parserState) acceptAny(o []string) bool {
	for _, v := range o {
		if s.acceptString(v) {
			return true
		}
	}
	return false
}

type Parser struct {
	rules    []parseFunc
	start    int
	builders map[string]BuilderFunc
	grammar  *Grammar
	err      error
}

func (p *Parser) Err() error {
	return p.err
}

func (p *Parser) newParserState(s string) *parserState {
	mode := p.grammar.Mode
	i := &parserInput{
		rules:        p.rules,
		buf:          s,
		length:       len(s),
		columnParser: mode.columnParser(),
	}

	return &parserState{i: i, tabstop: mode.getTabstop()}
}

func (p *Parser) ParseTree(s string) (*ParseTree, error) {
	if p.err != nil {
		return nil, p.err
	}
	parserState := p.newParserState(s)
	start := p.rules[p.start]
	if start(parserState) && parserState.atEnd() {
		name := p.grammar.Start
		n := parserState.captureNode(name)
		return &ParseTree{root: n, buf: s, nodes: parserState.nodes}, nil
	}
	return nil, ParseError
}

func (p *Parser) Parse(s string) (any, error) {
	if p.err != nil {
		return nil, p.err
	}

	tree, err := p.ParseTree(s)

	if err != nil {
		return nil, err
	}

	if p.builders == nil {
		return tree, nil
	}
	return tree.Build(p.builders)
}

func (p *Parser) testGrammar(accept []string, reject []string) bool {
	if p.err != nil {
		return false
	}
	start := p.rules[p.start]
	return p.testParseFunc(start, accept, reject)
}

func (p *Parser) testRule(name string, accept []string, reject []string) bool {
	if p.err != nil {
		return false
	}
	n, ok := p.grammar.nameIdx[name]
	if !ok {
		return false
	}
	start := p.rules[n]
	return p.testParseFunc(start, accept, reject)
}

func (p *Parser) testParseFunc(rule parseFunc, accept []string, reject []string) bool {
	if p.err != nil {
		return false
	}
	for _, s := range accept {
		parserState := p.newParserState(s)
		complete := rule(parserState) && parserState.atEnd()

		if !complete {
			return false
		}
	}
	for _, s := range reject {
		parserState := p.newParserState(s)
		complete := rule(parserState) && parserState.atEnd()

		if complete {
			return false
		}
	}
	return true
}

type Node struct {
	name     string
	start    int
	end      int
	child    int
	nchild   int
	sibling  int
	nsibling int
	// children []int
}

func (n *Node) children(t *ParseTree) []int {
	children := make([]int, n.nchild)
	c := n.child

	for j := 0; j < n.nchild; j++ {
		children[j] = c
		c = t.nodes[c].sibling
	}
	return children
}

type ParseTree struct {
	buf   string
	nodes []Node
	root  int
}

func (t *ParseTree) children(i int) []int {
	n := t.nodes[i]
	children := make([]int, n.nchild)
	c := n.child

	for j := 0; j < n.nchild; j++ {
		children[j] = c
		c = t.nodes[c].sibling
	}
	return children
}

func (t *ParseTree) Walk(f func(*Node)) {
	var walk func(int)

	walk = func(i int) {
		n := &t.nodes[i]
		c := n.child
		for i := 0; i < n.nchild; i++ {
			walk(c)
			c = t.nodes[c].sibling
		}
		f(n)

	}
	walk(t.root)
}

func (t *ParseTree) Build(builders map[string]BuilderFunc) (any, error) {
	var build func(int) (any, error)

	build = func(i int) (any, error) {
		var err error
		n := &t.nodes[i]
		args := make([]any, n.nchild)
		nextChild := n.child
		for idx := 0; idx < n.nchild; idx++ {

			args[idx], err = build(nextChild)
			nextChild = t.nodes[nextChild].sibling

			if err != nil {
				return nil, err
			}
		}
		return builders[n.name](t.buf[n.start:n.end], args)
	}
	return build(t.root)
}
