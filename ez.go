package ez

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
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

	peekStringAction = "PeekString"
	runeAction       = "Rune"
	peekRuneAction   = "PeekRune"
	stringAction     = "String"
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
		whitespace:      []string{" ", "\t"},
		newline:         []string{"\r\n", "\r", "\n"},
		tabstop:         8,
		stringsReserved: []string{"\r\n", "\r", "\n", "\t"},
		actionsDisabled: []string{
			byteAction,
			byteRangeAction,
			byteListAction,
			byteStringAction,
			peekByteAction,
		},
	}
}
func StringMode() *stringMode {
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

func BinaryMode() *binaryMode {
	return &binaryMode{
		actionsDisabled: []string{
			runeAction,
			stringAction,
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

func BuildGrammar(stub func(*G)) *Grammar {
	pos := getCallerPosition(1, grammarAction) // 1 is inside DefineGrammar, 2 is where DG was called
	return buildGrammar(pos, TextMode(), stub)
}

func BuildParser(stub func(*G)) *Parser {
	pos := getCallerPosition(1, grammarAction) // 1 is inside DefineGrammar, 2 is where DG was called
	grammar := buildGrammar(pos, TextMode(), stub)
	if grammar.Err != nil {
		return &Parser{err: grammar.Err}
	}

	return grammar.Parser()
}

// ---

type filePosition struct {
	n      int
	file   string
	line   int
	inside *string
	action string
}

func getCallerPosition(depth int, action string) *filePosition {
	_, file, no, ok := runtime.Caller(depth + 1)
	if !ok {
		return nil
	}
	base, _ := os.Getwd()
	file, _ = filepath.Rel(base, file)
	return &filePosition{file: file, line: no, action: action}
}

func (p *filePosition) String() string {
	if p.inside != nil {
		return fmt.Sprintf("%v:%v:%v", p.file, p.line, *p.inside)
	} else {
		return fmt.Sprintf("%v:%v", p.file, p.line)
	}

}

//
//  	Grammar Modes
//

type GrammarMode interface {
	parserConfig() *parserConfig
}

type parserConfig struct {
	name            string
	actionsDisabled []string
	columnParser    columnParserFunc
	whitespace      []string
	newlines        []string
	stringsReserved []string
	tabstop         int
}

func (c *parserConfig) actionAllowed(s string) bool {
	for _, v := range c.actionsDisabled {
		if v == s {
			return false
		}
	}
	return true
}

type binaryMode struct {
	actionsDisabled []string
}

func (m *binaryMode) parserConfig() *parserConfig {
	return &parserConfig{
		name:            "binary mode",
		actionsDisabled: m.actionsDisabled,
		columnParser:    nil,
		whitespace:      []string{},
		newlines:        []string{},
		tabstop:         0,
	}
}

type stringMode struct {
	whitespace      []string
	newline         []string
	actionsDisabled []string
}

func (m *stringMode) parserConfig() *parserConfig {
	return &parserConfig{
		name:            "string mode",
		actionsDisabled: m.actionsDisabled,
		columnParser:    nil,
		whitespace:      m.whitespace,
		newlines:        m.newline,
		tabstop:         0,
	}
}

type textMode struct {
	whitespace      []string
	newline         []string
	tabstop         int
	actionsDisabled []string
	stringsReserved []string
}

func (m *textMode) parserConfig() *parserConfig {
	return &parserConfig{
		name:            "text mode",
		actionsDisabled: m.actionsDisabled,
		stringsReserved: m.stringsReserved,
		columnParser:    textModeColumnParser,
		whitespace:      m.whitespace,
		newlines:        m.newline,
		tabstop:         m.tabstop,
	}
}

func (m *textMode) Tabstop(t int) *textMode {
	m.tabstop = t
	return m
}

// ---

type BuilderFunc func(string, []any) (any, error)

type grammarError struct {
	pos     *filePosition
	message string
	fatal   bool
}

func (e *grammarError) Error() string {
	return fmt.Sprintf("%v: error in %v(): %v", e.pos, e.pos.action, e.message)
}

type parseAction struct {
	// this is a 'wide style' variant struct

	kind    string
	pos     *filePosition
	name    string         // call, capture
	args    []*parseAction // choice, seq, cap
	strings []string
	ranges  []string
	bytes   [][]byte

	stringSwitch map[string]*parseAction
	runeSwitch   map[rune]*parseAction
	byteSwitch   map[byte]*parseAction

	min      int
	max      int
	inverted bool
	message  []any
}

type Grammar struct {
	config  *parserConfig
	start   string
	logFunc func(string, ...any)

	names    []string
	rules    map[string]*parseAction
	builders map[string]BuilderFunc

	pos *filePosition //
	Err error
}

func (g *Grammar) Parser() *Parser {
	if g.Err != nil {
		p := &Parser{err: g.Err}
		return p
	}

	index := make(map[string]int, len(g.rules))
	for i, n := range g.names {
		index[n] = i
	}

	rules := make([]parseFunc, len(g.rules))
	for i, n := range g.names {
		rule := g.rules[n]
		rules[i] = buildAction(g, rule, index)
	}

	p := &Parser{
		start:    g.start,
		rules:    rules,
		index:    index,
		builders: g.builders,
		grammar:  g,
	}
	return p
}

func buildGrammar(pos *filePosition, mode GrammarMode, stub func(*G)) *Grammar {
	g := &Grammar{}
	g.builders = make(map[string]BuilderFunc, 0)
	g.rules = make(map[string]*parseAction, 0)
	g.pos = pos

	bg := &G{
		grammar:    g,
		n:          1,
		nb:         &nodeBuilder{kind: grammarAction},
		LogFunc:    Printf,
		Mode:       mode,
		callPos:    make(map[string][]*filePosition, 0),
		capturePos: make(map[string][]*filePosition, 0),
		builderPos: make(map[string]*filePosition, 0),
		rulePos:    make(map[string]*filePosition, 0),
	}

	if stub == nil {
		bg.addError(g.pos, "cant call BuildGrammar() with nil")
	}

	stub(bg)

	if bg.Start == "" && len(g.rules) == 1 {
		for k := range g.rules {
			bg.Start = k
			break
		}
	}

	for name, pos := range bg.callPos {
		if _, ok := g.rules[name]; !ok {
			for _, p := range pos {
				bg.addErrorf(p, "missing rule %q", name)
			}
		}
	}

	for name := range g.rules {
		if name != bg.Start && bg.callPos[name] == nil {
			p := bg.rulePos[name]
			bg.addErrorf(p, "unused rule %q", name)
		}
	}

	for name, _ := range g.builders {
		if _, ok := bg.capturePos[name]; !ok {
			p := bg.builderPos[name]
			bg.addErrorf(p, "missing capture %q for builder", name)
		}
	}

	if len(g.builders) > 0 {
		for name, pos := range bg.capturePos {
			if _, ok := bg.builderPos[name]; !ok {
				for _, p := range pos {
					bg.addErrorf(p, "missing builder %q for capture", name)
				}
			}
		}
	}

	if bg.Start == "" {
		bg.addError(g.pos, "starting rule undefined")
	} else if _, ok := g.rules[bg.Start]; !ok {
		bg.addErrorf(g.pos, "starting rule %q is missing", bg.Start)
	}

	if bg.err != nil {
		return &Grammar{Err: bg.err}
	}

	g.start = bg.Start
	g.logFunc = bg.LogFunc

	return g
}

// Builder

type nodeBuilder struct {
	kind   string
	parent *nodeBuilder
	rule   *string
	args   []*parseAction
}

func (b *nodeBuilder) buildSequence(pos *filePosition) *parseAction {
	if len(b.args) == 0 {
		return nil
	}
	if len(b.args) == 1 {
		return b.args[0]
	}
	return &parseAction{kind: sequenceAction, args: b.args, pos: pos}
}

func (b *nodeBuilder) buildNode(pos *filePosition, kind string) *parseAction {
	return &parseAction{kind: kind, args: b.args, pos: pos}
}

func (b *nodeBuilder) append(a *parseAction) {
	b.args = append(b.args, a)
}

func (b *nodeBuilder) inRule() bool {
	return b != nil && b.kind != grammarAction
}

func (b *nodeBuilder) nestedInside(k string) bool {
	n := b
	for n != nil {
		if n.kind == k {
			return true
		}
		n = n.parent
	}
	return false
}

type G struct {
	grammar *Grammar

	Start   string
	LogFunc func(string, ...any)

	Mode       GrammarMode
	configmode GrammarMode

	callPos    map[string][]*filePosition
	capturePos map[string][]*filePosition

	builderPos map[string]*filePosition
	rulePos    map[string]*filePosition

	nb     *nodeBuilder
	err    error
	errors []error
	n      int
}

func (g *G) parserConfig() *parserConfig {
	if g.configmode == nil {
		g.configmode = g.Mode
		g.grammar.config = g.configmode.parserConfig()
	}
	return g.grammar.config
}

func (g *G) addError(pos *filePosition, args ...any) {
	msg := fmt.Sprint(args...)
	err := &grammarError{
		message: msg,
		pos:     pos,
	}
	if g.err == nil {
		g.err = err
	}
	g.errors = append(g.errors, err)
}

func (g *G) addErrorf(pos *filePosition, s string, args ...any) {
	msg := fmt.Sprintf(s, args...)
	err := &grammarError{
		message: msg,
		pos:     pos,
	}
	if g.err == nil {
		g.err = err
	}
	g.errors = append(g.errors, err)
}

func (g *G) addWarn(pos *filePosition, args ...any) {
	msg := fmt.Sprint(args...)
	err := &grammarError{
		message: msg,
		pos:     pos,
	}
	g.errors = append(g.errors, err)
}

func (g *G) addWarnf(pos *filePosition, s string, args ...any) {
	msg := fmt.Sprintf(s, args...)
	err := &grammarError{
		message: msg,
		pos:     pos,
	}
	g.errors = append(g.errors, err)
}

func (g *G) shouldExit(pos *filePosition, kind string) bool {
	if g.grammar == nil {
		return true
	}
	if len(g.errors) > 0 {
		return true
	}
	if g.nb == nil {
		g.addError(pos, "must call builder methods inside builder")
		return true
	}
	if !g.nb.inRule() {
		g.addError(pos, "must call builder methods inside Define()")
		return true
	}
	if g.Mode == nil {
		g.addError(pos, "no Mode set")
		return true
	}
	if config := g.parserConfig(); !config.actionAllowed(kind) {
		g.addErrorf(pos, "cannot call %v() in %v", kind, config.name)
		return true
	}
	return false

}

func (g *G) markPosition(actionKind string) *filePosition {
	// would be one if called inside BuilderFunc()
	pos := getCallerPosition(2, actionKind)
	rule := g.nb.rule
	if rule != nil {
		pos.inside = rule
	}
	pos.n = g.n
	g.n++

	return pos
}

func (g *G) buildStub(kind string, stub func()) *nodeBuilder {
	var rule *string
	oldNb := g.nb
	if oldNb != nil {
		rule = oldNb.rule
	}
	newNb := &nodeBuilder{kind: kind, rule: rule, parent: oldNb}
	g.nb = newNb
	stub()
	g.nb = oldNb
	return newNb
}

func (g *G) buildRule(rule string, stub func()) *nodeBuilder {
	oldNb := g.nb
	newNb := &nodeBuilder{kind: defineAction, rule: &rule, parent: oldNb}
	g.nb = newNb
	stub()
	g.nb = oldNb
	return newNb
}

func (g *G) Define(name string, stub func()) {
	p := g.markPosition(defineAction)
	if g.grammar == nil || g.err != nil {
		return
	} else if g.nb == nil {
		g.addError(p, "must call define inside grammar")
		return
	} else if g.nb.inRule() {
		g.addError(p, "cant call define inside define")
		return
	} else if stub == nil {
		g.addError(p, "cant call Define() with nil")
		return
	}

	if oldPos, ok := g.rulePos[name]; ok {
		g.addErrorf(p, "cant redefine %q, already defined at %v", name, oldPos)
		return
	}

	g.rulePos[name] = p

	r := g.buildRule(name, stub)
	g.grammar.rules[name] = r.buildSequence(p)
	g.grammar.names = append(g.grammar.names, name)
}

func (g *G) Print(args ...any) {
	p := g.markPosition(printAction)
	if g.shouldExit(p, printAction) {
		return
	}
	a := &parseAction{kind: printAction, message: args, pos: p}
	g.nb.append(a)
}
func (g *G) Trace(stub func()) {
	p := g.markPosition(traceAction)
	if g.shouldExit(p, traceAction) {
		return
	} else if stub == nil {
		g.addError(p, "cant call Trace() with nil")
	}

	r := g.buildStub(traceAction, stub)
	if g.err != nil {
		return
	}

	a := &parseAction{kind: traceAction, args: r.args, pos: p}
	g.nb.append(a)
}

func (g *G) Whitespace() {
	p := g.markPosition(whitespaceAction)
	if g.shouldExit(p, whitespaceAction) {
		return
	}
	a := &parseAction{kind: whitespaceAction, pos: p}
	g.nb.append(a)
}

func (g *G) Newline() {
	p := g.markPosition(newlineAction)
	if g.shouldExit(p, newlineAction) {
		return
	}
	a := &parseAction{kind: newlineAction, pos: p}
	g.nb.append(a)
}

func (g *G) StartOfFile() {
	p := g.markPosition(startOfFileAction)
	if g.shouldExit(p, startOfFileAction) {
		return
	}

	a := &parseAction{kind: startOfFileAction, pos: p}
	g.nb.append(a)
}

func (g *G) EndOfFile() {
	p := g.markPosition(endOfFileAction)
	if g.shouldExit(p, endOfFileAction) {
		return
	}

	a := &parseAction{kind: endOfFileAction, pos: p}
	g.nb.append(a)
}

func (g *G) StartOfLine() {
	p := g.markPosition(startOfLineAction)
	if g.shouldExit(p, startOfLineAction) {
		return
	}

	a := &parseAction{kind: startOfLineAction, pos: p}
	g.nb.append(a)
}

func (g *G) EndOfLine() {
	p := g.markPosition(endOfLineAction)
	if g.shouldExit(p, endOfLineAction) {
		return
	}

	a := &parseAction{kind: endOfLineAction, pos: p}
	g.nb.append(a)
}

func (g *G) Rune() {
	p := g.markPosition(runeAction)
	if g.shouldExit(p, runeAction) {
		return
	}

	a := &parseAction{kind: runeAction, pos: p}
	g.nb.append(a)
}

func (g *G) PeekString(stubs map[string]func()) {
	p := g.markPosition(peekStringAction)
	if g.shouldExit(p, peekStringAction) {
		return
	} else if stubs == nil {
		g.addError(p, "cant call PeekString() with nil map")
		return
	}

	args := make(map[string]*parseAction, len(stubs))
	for c, stub := range stubs {
		if stub == nil {
			g.addError(p, "cant call PeekString() with nil function")
			return
		}

		r := g.buildStub(peekStringAction, stub)

		if g.err != nil {
			return
		}

		args[c] = r.buildSequence(p)
	}
	a := &parseAction{kind: peekStringAction, stringSwitch: args, pos: p}
	g.nb.append(a)
}
func (g *G) PeekRune(stubs map[rune]func()) {
	p := g.markPosition(peekRuneAction)
	if g.shouldExit(p, peekRuneAction) {
		return
	} else if stubs == nil {
		g.addError(p, "cant call PeekRune() with nil map")
		return
	}

	args := make(map[rune]*parseAction, len(stubs))
	for c, stub := range stubs {
		if stub == nil {
			g.addError(p, "cant call PeekRune() with nil function")
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
func (g *G) String(s ...string) {
	p := g.markPosition(stringAction)
	if g.shouldExit(p, stringAction) {
		return
	}
	if len(s) == 0 {
		g.addError(p, "missing operand")
		return
	}
	for _, v := range s {
		if !utf8.ValidString(v) {
			g.addErrorf(p, "String(%q) contains invalid UTF-8", v)
			return
		}
		for _, b := range g.parserConfig().stringsReserved {
			if strings.Index(v, b) > -1 {
				g.addErrorf(p, "String(%q) contains reserved string %q", v, b)
				return
			}
		}

	}

	a := &parseAction{kind: stringAction, strings: s, pos: p}
	g.nb.append(a)
}

func (g *G) Range(s ...string) RangeOptions {
	p := g.markPosition(rangeAction)
	ro := RangeOptions{
		g: g,
		p: p,
	}
	if g.shouldExit(p, rangeAction) {
		return ro
	}
	if len(s) == 0 {
		g.addError(p, "missing operand")
		return ro
	}

	args := make([]string, len(s))
	for i, v := range s {
		r := []rune(v)
		if !(len(r) == 1 || (len(r) == 3 && r[1] == '-' && r[0] < r[2])) {
			g.addError(p, "invalid range", v)
			return ro
		}
		args[i] = v
	}
	a := &parseAction{kind: rangeAction, ranges: args, pos: p}
	ro.a = a
	g.nb.append(a)
	return ro
}

func (g *G) Byte() {
	p := g.markPosition(byteAction)
	if g.shouldExit(p, byteAction) {
		return
	}

	a := &parseAction{kind: byteAction, pos: p}
	g.nb.append(a)
}

func (g *G) PeekByte(stubs map[byte]func()) {
	p := g.markPosition(peekByteAction)
	if g.shouldExit(p, peekByteAction) {
		return
	} else if stubs == nil {
		g.addError(p, "cant call PeekByte() with nil map")
		return
	}

	args := make(map[byte]*parseAction, len(stubs))
	for c, stub := range stubs {
		if stub == nil {
			g.addError(p, "cant call PeekByte() with nil function")
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

func (g *G) ByteString(s ...string) {
	p := g.markPosition(byteStringAction)
	if g.shouldExit(p, byteStringAction) {
		return
	}
	if len(s) == 0 {
		g.addError(p, "missing operand")
		return
	}

	b := make([][]byte, len(s))
	for i, v := range s {
		bs := make([]byte, utf8.RuneCountInString(v))
		c := 0
		for _, r := range v {
			if r > 255 {
				g.addErrorf(p, "ByteString(%q) contains rune > 255, cannot convert to []byte", v)
				return
			}
			bs[c] = byte(r)
			c++
		}
		b[i] = bs

	}

	a := &parseAction{kind: byteStringAction, bytes: b, pos: p}
	g.nb.append(a)
}

func (g *G) Bytes(s ...[]byte) {
	p := g.markPosition(byteListAction)
	if g.shouldExit(p, byteListAction) {
		return
	}
	if len(s) == 0 {
		g.addError(p, "missing operand")
		return
	}

	a := &parseAction{kind: byteListAction, bytes: s, pos: p}
	g.nb.append(a)
}

func (g *G) ByteRange(s ...string) RangeOptions {
	p := g.markPosition(byteRangeAction)
	ro := RangeOptions{
		g: g,
		p: p,
	}
	if g.shouldExit(p, byteRangeAction) {
		return ro
	}
	if len(s) == 0 {
		g.addError(p, "missing operand")
		return ro
	}

	args := make([]string, len(s))
	for i, v := range s {
		r := []byte(v)
		if !(len(r) == 1 || (len(r) == 3 && r[1] == '-' && r[0] < r[2])) {
			g.addError(p, "invalid range", v)
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
	g *G
	a *parseAction
	p *filePosition
}

func (ro RangeOptions) Invert() RangeOptions {
	return ro.g.invertRange(ro.p, ro.a)
}

func (g *G) invertRange(rangePos *filePosition, a *parseAction) RangeOptions {
	p := g.markPosition(rangeAction)
	ro := RangeOptions{
		g: g,
		p: p,
	}
	if a == nil || g.shouldExit(p, a.kind) {
		return ro
	}

	if p.n-rangePos.n != 1 {
		g.addError(p, "called in wrong position")
		return ro
	}
	a.inverted = !a.inverted
	ro.a = a
	return ro
}

func (g *G) Call(name string) {
	p := g.markPosition(callAction)
	if g.shouldExit(p, callAction) {
		return
	}
	g.callPos[name] = append(g.callPos[name], p)
	a := &parseAction{kind: callAction, name: name, pos: p}
	g.nb.append(a)
}

func (g *G) Sequence(stub func()) {
	p := g.markPosition(sequenceAction)
	if g.shouldExit(p, sequenceAction) {
		return
	} else if stub == nil {
		g.addError(p, "cant call Sequence() with nil")
	}

	r := g.buildStub(sequenceAction, stub)

	if g.err != nil {
		return
	}

	a := r.buildSequence(p)
	g.nb.append(a)
}

func (g *G) Capture(name string, stub func()) {
	p := g.markPosition(sequenceAction)
	if g.shouldExit(p, captureAction) {
		return
	} else if stub == nil {
		g.addError(p, "cant call Capture() with nil")
	}

	r := g.buildStub(sequenceAction, stub)

	if g.err != nil {
		return
	}

	g.capturePos[name] = append(g.capturePos[name], p)

	a := &parseAction{kind: captureAction, name: name, args: r.args, pos: p}
	g.nb.append(a)
}

func (g *G) Cut() {
	p := g.markPosition(cutAction)
	if g.shouldExit(p, cutAction) {
		return
	}
	if !g.nb.nestedInside(choiceAction) {
		g.addError(p, "cut must be called directly inside choice, sorry.")
		return
	}

	a := &parseAction{kind: cutAction, pos: p}
	g.nb.append(a)
}

func (g *G) Choice(options ...func()) {
	p := g.markPosition(choiceAction)
	if g.shouldExit(p, choiceAction) {
		return
	} else if options == nil {
		g.addError(p, "cant call Choice() with nil")
		return
	}

	args := make([]*parseAction, len(options))
	for i, stub := range options {
		if stub == nil {
			g.addError(p, "cant call Choice() with nil")
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

func (g *G) Optional(stub func()) {
	p := g.markPosition(optionalAction)
	if g.shouldExit(p, optionalAction) {
		return
	} else if stub == nil {
		g.addError(p, "cant call Optional() with nil")
	}
	r := g.buildStub(optionalAction, stub)
	if g.err != nil {
		return
	}

	a := &parseAction{kind: optionalAction, args: r.args, pos: p}
	g.nb.append(a)
}

func (g *G) Lookahead(stub func()) {
	p := g.markPosition(lookaheadAction)
	if g.shouldExit(p, lookaheadAction) {
		return
	} else if stub == nil {
		g.addError(p, "cant call Lookahead() with nil")
	}
	r := g.buildStub(lookaheadAction, stub)
	if g.err != nil {
		return
	}

	a := &parseAction{kind: lookaheadAction, args: r.args, pos: p}
	g.nb.append(a)
}

func (g *G) Reject(stub func()) {
	p := g.markPosition(rejectAction)
	if g.shouldExit(p, rejectAction) {
		return
	} else if stub == nil {
		g.addError(p, "cant call Reject() with nil")
	}
	r := g.buildStub(rejectAction, stub)
	if g.err != nil {
		return
	}

	a := &parseAction{kind: rejectAction, args: r.args, pos: p}
	g.nb.append(a)
}

func (g *G) Repeat(min_t int, max_t int, stub func()) {
	p := g.markPosition(repeatAction)
	if g.shouldExit(p, repeatAction) {
		return
	} else if stub == nil {
		g.addError(p, "cant call Repeat() with nil")
	}

	r := g.buildStub(repeatAction, stub)

	if g.err != nil {
		return
	}

	a := &parseAction{kind: repeatAction, args: r.args, min: min_t, max: max_t, pos: p}
	g.nb.append(a)
}

func (g *G) Builder(name string, stub BuilderFunc) {
	p := g.markPosition(builderAction)
	if g.err != nil {
		return
	} else if g.nb == nil {
		g.addError(p, "must call Builder() inside grammar")
		return
	} else if g.nb.inRule() {
		g.addError(p, "cant call Builder() inside define")
		return
	} else if stub == nil {
		g.addError(p, "cant call Builder() with nil")
	}

	if _, ok := g.grammar.builders[name]; ok {
		oldPos := g.builderPos[name]
		g.addErrorf(p, "cant redefine %q, already defined at %v", name, oldPos)
		return
	}
	g.builderPos[name] = p
	g.grammar.builders[name] = stub
}

//
//  	Parser
//

type parseFunc func(*parserInput, *parserState) bool

type parserInput struct {
	rules        []parseFunc
	buf          string
	length       int
	columnParser columnParserFunc
	tabstop      int
}

func atEnd(i *parserInput, s *parserState) bool {
	return s.offset >= i.length
}

func peekByte(i *parserInput, s *parserState) byte {
	return i.buf[s.offset]
}

func peekString(i *parserInput, s *parserState, n int) string {
	end := s.offset + n
	if end > i.length {
		end = i.length
	}
	return i.buf[s.offset:end]
}

func peekBytes(i *parserInput, s *parserState, n int) []byte {
	end := s.offset + n
	if end > i.length {
		end = i.length
	}
	return []byte(i.buf[s.offset:end])
}

func peekRune(i *parserInput, s *parserState) (rune, int) {
	return utf8.DecodeRuneInString(i.buf[s.offset:])
}

type parserState struct {
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

func advanceState(i *parserInput, s *parserState, length int) {
	newOffset := s.offset + length
	if i.columnParser != nil {
		s.column = i.columnParser(i.buf, s.column, i.tabstop, s.offset, newOffset)
	}
	s.offset = newOffset

}

func acceptString(i *parserInput, s *parserState, v string) bool {
	length_v := len(v)
	b := peekString(i, s, length_v)
	if b == v {
		advanceState(i, s, length_v)
		return true
	}
	return false
}
func acceptBytes(i *parserInput, s *parserState, v []byte) bool {
	length_v := len(v)
	b := peekString(i, s, length_v)
	if b == string(v) {
		advanceState(i, s, length_v)
		return true
	}
	return false
}

func acceptAny(i *parserInput, s *parserState, o []string) bool {
	for _, v := range o {
		if acceptString(i, s, v) {
			return true
		}
	}
	return false
}

func copyState(s *parserState, into *parserState) {
	*into = *s
}

func mergeState(s *parserState, new *parserState) {
	*s = *new
}

func startCapture(s *parserState, st *parserState) {
	*st = *s
	st.countSibling = 0
	st.lastSibling = 0
}

func trimCapture(s *parserState, new *parserState) {
	s.nodes = s.nodes[:s.numNodes]
}

func mergeCapture(s *parserState, name string, new *parserState) {

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

func (s *parserState) finalNode(name string) int {
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

func buildAction(g *Grammar, a *parseAction, index map[string]int) parseFunc {
	if a == nil {
		// when a func() stub has no rules
		return func(i *parserInput, s *parserState) bool {
			return true
		}
	}
	switch a.kind {
	case printAction:
		prefix := a.pos
		fn := g.logFunc
		return func(i *parserInput, s *parserState) bool {
			msg := fmt.Sprint(a.message...)
			fn("%v: Print(%q) called, at offset %v, column %v\n", prefix, msg, s.offset, s.column)
			return true
		}
	case traceAction:
		prefix := a.pos
		fn := g.logFunc

		rules := make([]parseFunc, len(a.args))
		for i, r := range a.args {
			rules[i] = buildAction(g, r, index)
		}

		return func(i *parserInput, s *parserState) bool {
			fn("%v: Trace() starting, at offset %v\n", prefix, s.offset)
			result := true

			var s1 parserState
			copyState(s, &s1)
			s1.trace = true
			for _, v := range rules {
				if !v(i, &s1) {
					result = false
					break
				}
			}
			if result {
				s1.trace = false
				fn("%v: Trace() ending, at offset %v\n", prefix, s1.offset)
				mergeState(s, &s1)
			} else {
				s.choiceExit = s1.choiceExit
				fn("%v: Trace() failing, at offset %v\n", prefix, s.offset)
			}
			return result
		}
	case callAction:
		prefix := a.pos
		name := a.name
		idx := index[name]
		fn := g.logFunc
		return func(i *parserInput, s *parserState) bool {
			if s.trace {
				fn("%v: Call(%q) starting, at offset %v\n", prefix, name, s.offset)
			}

			rule := i.rules[idx] // can't move this out to runtime unless we reorder defs
			oldChoice := s.choiceExit
			s.choiceExit = false

			out := rule(i, s)
			s.choiceExit = oldChoice

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
		ws := g.config.whitespace
		return func(i *parserInput, s *parserState) bool {
			for {
				if !acceptAny(i, s, ws) {
					break
				}
			}
			return true
		}
	case endOfLineAction, newlineAction:
		nl := g.config.newlines
		return func(i *parserInput, s *parserState) bool {
			return acceptAny(i, s, nl)
		}

	case startOfLineAction:
		return func(i *parserInput, s *parserState) bool {
			return s.column == 0
		}
	case startOfFileAction:
		return func(i *parserInput, s *parserState) bool {
			return s.offset == 0
		}
	case endOfFileAction:
		return func(i *parserInput, s *parserState) bool {
			return s.offset == i.length
		}

	case runeAction:
		return func(i *parserInput, s *parserState) bool {
			if atEnd(i, s) {
				return false
			}
			_, n := peekRune(i, s)
			advanceState(i, s, n)
			return true
		}
	case byteAction:
		return func(i *parserInput, s *parserState) bool {
			if atEnd(i, s) {
				return false
			}
			advanceState(i, s, 1)
			return true
		}
	case stringAction:
		return func(i *parserInput, s *parserState) bool {
			for _, v := range a.strings {
				if acceptString(i, s, v) {
					return true
				}
			}
			return false
		}
	case byteListAction, byteStringAction:
		return func(i *parserInput, s *parserState) bool {
			for _, v := range a.bytes {
				if acceptBytes(i, s, v) {
					return true
				}
			}
			return false
		}
	case peekStringAction:
		rules := make(map[string]parseFunc, len(a.stringSwitch))
		size := 0
		for i, r := range a.stringSwitch {
			rules[i] = buildAction(g, r, index)
			if len(i) > size {
				size = len(i)
			}
		}
		return func(i *parserInput, s *parserState) bool {
			if atEnd(i, s) {
				return false
			}
			r := peekString(i, s, size)

			if fn, ok := rules[r]; ok {
				return fn(i, s)
			}

			return false
		}
	case peekRuneAction:
		rules := make(map[rune]parseFunc, len(a.runeSwitch))
		for i, r := range a.runeSwitch {
			rules[i] = buildAction(g, r, index)
		}
		return func(i *parserInput, s *parserState) bool {
			if atEnd(i, s) {
				return false
			}
			r, _ := peekRune(i, s)

			if fn, ok := rules[r]; ok {
				return fn(i, s)
			}

			return false
		}
	case peekByteAction:
		rules := make(map[byte]parseFunc, len(a.byteSwitch))
		for i, r := range a.byteSwitch {
			rules[i] = buildAction(g, r, index)
		}
		return func(i *parserInput, s *parserState) bool {
			if atEnd(i, s) {
				return false
			}
			r := peekByte(i, s)

			if fn, ok := rules[r]; ok {
				return fn(i, s)
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
		return func(i *parserInput, s *parserState) bool {
			if atEnd(i, s) {
				return false
			}
			r, size := peekRune(i, s)
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
				advanceState(i, s, size)
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
		return func(i *parserInput, s *parserState) bool {
			if atEnd(i, s) {
				return false
			}
			r := peekByte(i, s)
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
				advanceState(i, s, 1)
				return true
			}

			return false
		}
	case optionalAction:
		rules := make([]parseFunc, len(a.args))
		for i, r := range a.args {
			rules[i] = buildAction(g, r, index)
		}
		return func(i *parserInput, s *parserState) bool {
			var s1 parserState
			copyState(s, &s1)
			for _, r := range rules {
				if !r(i, &s1) {
					s.choiceExit = s1.choiceExit
					return true
				}
			}
			s.choiceExit = s1.choiceExit
			mergeState(s, &s1)
			return true
		}
	case lookaheadAction:
		rules := make([]parseFunc, len(a.args))
		for i, r := range a.args {
			rules[i] = buildAction(g, r, index)
		}
		return func(i *parserInput, s *parserState) bool {
			var s1 parserState
			copyState(s, &s1)
			for _, r := range rules {
				if !r(i, &s1) {
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
			rules[i] = buildAction(g, r, index)
		}
		return func(i *parserInput, s *parserState) bool {
			var s1 parserState
			copyState(s, &s1)
			for _, r := range rules {
				if !r(i, &s1) {
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
			rules[i] = buildAction(g, r, index)
		}
		min_n := a.min
		max_n := a.max

		return func(i *parserInput, s *parserState) bool {
			c := 0
			var s1 parserState
			copyState(s, &s1)
			for {
				for _, r := range rules {
					if !r(i, &s1) {
						s.choiceExit = s1.choiceExit
						return c >= min_n
					}
				}
				if c >= min_n {
					mergeState(s, &s1)
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
		return func(i *parserInput, s *parserState) bool {
			s.choiceExit = true
			return true
		}
	case choiceAction:
		rules := make([]parseFunc, len(a.args))
		for i, r := range a.args {
			rules[i] = buildAction(g, r, index)
		}
		return func(i *parserInput, s *parserState) bool {
			for _, r := range rules {
				var s1 parserState
				copyState(s, &s1)
				s1.choiceExit = false
				if r(i, &s1) {
					s1.choiceExit = s.choiceExit
					mergeState(s, &s1)
					return true
				}
				trimCapture(s, &s1)
				if s1.choiceExit {
					break
				}
			}
			return false
		}
	case sequenceAction:
		rules := make([]parseFunc, len(a.args))
		for i, r := range a.args {
			rules[i] = buildAction(g, r, index)
		}
		return func(i *parserInput, s *parserState) bool {
			var s1 parserState
			copyState(s, &s1)
			for _, r := range rules {
				if !r(i, &s1) {
					s.choiceExit = s1.choiceExit
					return false
				}
			}
			mergeState(s, &s1)
			return true
		}
	case captureAction:
		rules := make([]parseFunc, len(a.args))
		for i, r := range a.args {
			rules[i] = buildAction(g, r, index)
		}
		return func(i *parserInput, s *parserState) bool {
			var s1 parserState
			startCapture(s, &s1)
			for _, r := range rules {
				if !r(i, &s1) {
					s.choiceExit = s1.choiceExit
					return false
				}
			}
			s.choiceExit = s1.choiceExit
			mergeCapture(s, a.name, &s1)
			return true
		}
	default:
		return func(i *parserInput, s *parserState) bool {
			return true
		}
	}
}

type Parser struct {
	rules    []parseFunc
	index    map[string]int
	start    string
	builders map[string]BuilderFunc
	grammar  *Grammar
	err      error
}

func (p *Parser) Err() error {
	return p.err
}

func (p *Parser) newParserInput(s string) *parserInput {
	config := p.grammar.config
	i := &parserInput{
		rules:        p.rules,
		buf:          s,
		length:       len(s),
		columnParser: config.columnParser,
		tabstop:      config.tabstop,
	}
	return i
}

func (p *Parser) ParseTree(s string) (*ParseTree, error) {
	if p.err != nil {
		return nil, p.err
	}
	input := p.newParserInput(s)
	state := &parserState{}
	rule := p.rules[p.index[p.start]]

	complete := rule(input, state) && atEnd(input, state)
	if complete {
		n := state.finalNode(p.start)
		return &ParseTree{root: n, buf: s, nodes: state.nodes}, nil
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
	rule := p.rules[p.index[p.start]]
	return p.testParseFunc(rule, accept, reject)
}

func (p *Parser) testRule(name string, accept []string, reject []string) bool {
	if p.err != nil {
		return false
	}
	i, ok := p.index[name]
	if !ok {
		return false
	}
	rule := p.rules[i]
	return p.testParseFunc(rule, accept, reject)
}

func (p *Parser) testParseFunc(rule parseFunc, accept []string, reject []string) bool {
	if p.err != nil {
		return false
	}
	for _, s := range accept {
		input := p.newParserInput(s)
		state := &parserState{}
		complete := rule(input, state) && atEnd(input, state)

		if !complete {
			return false
		}
	}
	for _, s := range reject {
		input := p.newParserInput(s)
		state := &parserState{}
		complete := rule(input, state) && atEnd(input, state)

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
