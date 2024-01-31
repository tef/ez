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
	grammarAction   = "Grammar"
	defineAction    = "Define"
	builderAction   = "Builder"
	recursiveAction = "Recursive"

	printAction = "Print"
	traceAction = "Trace"

	callAction = "Call"

	cornerAction   = "Corner"
	noCornerAction = "NoCorner"

	recurAction = "Recur"
	stumpAction = "Stump"

	choiceAction = "Choice"
	caseAction   = "Case"
	cutAction    = "Cut"

	doAction       = "Do"
	ruleAction     = "Rule"
	sequenceAction = "Sequence"

	optionalAction  = "Optional"
	repeatAction    = "Repeat"
	lookaheadAction = "Lookahead"
	rejectAction    = "Reject"
	captureAction   = "Capture"

	startOfFileAction = "StartOfFile"
	endOfFileAction   = "EndOfFile"

	startOfLineAction = "StartOfLine"
	endOfLineAction   = "EndOfLine"

	runeAction        = "Rune"
	runeRangeAction   = "Rune.Range"
	runeExceptAction  = "Rune.Except"
	stringAction      = "String"
	matchRuneAction   = "MatchRune"
	matchStringAction = "MatchString"

	spaceAction             = "Space"
	tabAction               = "Tab"
	whitespaceAction        = "Whitespace"
	newlineAction           = "Newline"
	whitespaceNewlineAction = "WhitespaceNewline"

	indentedBlockAction = "IndentedBlock"
	offsideBlockAction  = "OffsideBlock"
	indentAction        = "Indent"
	dedentAction        = "Dedent"

	byteAction       = "Byte"
	byteRangeAction  = "Byte.Range"
	byteExceptAction = "Byte.Except"

	matchByteAction  = "MatchByte"
	byteListAction   = "Bytes"
	byteStringAction = "ByteString"
)

var ParseError = errors.New("failed to parse")

var Whitespace = []string{" ", "\t"}
var Newline = []string{"\r\n", "\r", "\n"}

func Printf(format string, a ...any) {
	// used for g.LogFunc = ez.Printf
	fmt.Printf(format, a...)
}

func TextMode() *textMode {
	return &textMode{
		tabstop:         8,
		stringsReserved: []string{"\r\n", "\r", "\n", "\t"},
		actionsDisabled: []string{
			byteAction,
			byteRangeAction,
			byteExceptAction,
			byteListAction,
			byteStringAction,
			matchByteAction,
		},
	}
}
func StringMode() *stringMode {
	return &stringMode{
		actionsDisabled: []string{
			indentedBlockAction,
			offsideBlockAction,
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
			tabAction,
			spaceAction,
			whitespaceAction,
			newlineAction,
			whitespaceNewlineAction,
			startOfLineAction,
			endOfLineAction,
			runeRangeAction,
			runeExceptAction,
			indentedBlockAction,
			offsideBlockAction,
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

type grammarError struct {
	pos     *filePosition
	message string
	fatal   bool
}

func (e *grammarError) Error() string {
	return fmt.Sprintf("%v: error in %v(), %v", e.pos, e.pos.action, e.message)
}

func errorSummary(pos *filePosition, errors []*grammarError) error {
	if len(errors) == 0 {
		return nil
	} else if len(errors) == 1 {
		return errors[0]
	}

	report := make([]string, len(errors)+1)

	report[0] = fmt.Sprintf("%d in total", len(errors))

	for i, v := range errors {
		report[i+1] = v.Error()
	}

	message := strings.Join(report, "\n ")

	return &grammarError{
		pos:     pos,
		message: message,
	}
}

//
//  	Grammar Modes
//

type GrammarMode interface {
	grammarConfig() *grammarConfig
}

type binaryMode struct {
	actionsDisabled []string
}

func (m *binaryMode) grammarConfig() *grammarConfig {
	return &grammarConfig{
		name:            "binary mode",
		actionsDisabled: m.actionsDisabled,
		tabstop:         1,
	}
}

type stringMode struct {
	actionsDisabled []string
}

func (m *stringMode) grammarConfig() *grammarConfig {
	return &grammarConfig{
		name:            "string mode",
		actionsDisabled: m.actionsDisabled,
		tabstop:         1,
	}
}

type textMode struct {
	tabstop         int
	actionsDisabled []string
	stringsReserved []string
}

func (m *textMode) grammarConfig() *grammarConfig {
	return &grammarConfig{
		name:            "text mode",
		actionsDisabled: m.actionsDisabled,
		stringsReserved: m.stringsReserved,
		tabstop:         m.tabstop,
	}
}

func (m *textMode) Tabstop(t int) *textMode {
	m.tabstop = t
	return m
}

// parseAction is a representation of a bit of a grammar

type parseAction struct {
	// this is a 'wide style' variant
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

	zeroWidth bool
	terminal  bool
	// regular?
	//

	recursiveNames []string

	precedence int
}

func (a *parseAction) walk(stub func(*parseAction)) {
	if a == nil {
		return
	}
	if a.args != nil {
		for _, c := range a.args {
			c.walk(stub)
		}
	}
	stub(a)
}

func (a *parseAction) leftCalls() []string {
	if a == nil {
		return []string{}
	}

	switch a.kind {
	case choiceAction:
		out := []string{}
		for _, v := range a.args {
			for _, j := range v.leftCalls() {
				out = append(out, j)
			}
		}
		return out

	case traceAction,
		doAction, caseAction, ruleAction, sequenceAction,
		optionalAction, repeatAction,
		lookaheadAction, captureAction, rejectAction,
		matchRuneAction, matchStringAction, matchByteAction:

		out := []string{}
		for _, v := range a.args {
			for _, j := range v.leftCalls() {
				out = append(out, j)
			}
			if !v.zeroWidth {
				break
			}
		}

		return out

	case callAction, recurAction:
		return []string{a.name}
	}

	return []string{}

}

func (a *parseAction) setTerminal() {

	allTerminal := true
	if a.args != nil {
		for _, c := range a.args {
			c.setTerminal()
			allTerminal = allTerminal && c.terminal
		}
	}

	switch a.kind {
	case traceAction:
		a.terminal = allTerminal
	case printAction:
		a.terminal = true

	case callAction:
		a.terminal = false
	case recurAction:
		a.terminal = false

	case cornerAction:
		a.terminal = false
	case noCornerAction:
		a.terminal = true

	case doAction, sequenceAction, caseAction, ruleAction:
		a.terminal = allTerminal
	case choiceAction:
		a.terminal = allTerminal
	case cutAction:
		a.terminal = true
	case optionalAction:
		a.terminal = allTerminal
	case repeatAction:
		a.terminal = allTerminal
	case lookaheadAction:
		a.terminal = allTerminal
	case rejectAction:
		a.terminal = allTerminal
	case captureAction:
		a.terminal = allTerminal

	case matchRuneAction, matchStringAction:
		a.terminal = allTerminal
	case matchByteAction:
		a.terminal = allTerminal

	case startOfFileAction, endOfFileAction:
		a.terminal = true
	case startOfLineAction, endOfLineAction:
		a.terminal = true

	case runeAction, stringAction:
		a.terminal = true
	case runeRangeAction, runeExceptAction:
		a.terminal = true

	case spaceAction, tabAction:
		a.terminal = true
	case whitespaceAction, newlineAction, whitespaceNewlineAction:
		a.terminal = true

	case indentedBlockAction, offsideBlockAction:
		a.terminal = false
	case indentAction, dedentAction:
		a.terminal = false

	case byteAction, byteListAction, byteStringAction:
		a.terminal = true
	case byteRangeAction, byteExceptAction:
		a.terminal = true
	default:
		a.terminal = false
	}
}

func (a *parseAction) setZeroWidth(rules map[string]bool) bool {
	old := a.zeroWidth

	allZw := true
	anyZw := false
	if a.args != nil {
		for _, c := range a.args {
			c.setZeroWidth(rules)
			allZw = allZw && c.zeroWidth
			anyZw = anyZw || c.zeroWidth
		}
		// empty list should be true
	}

	anyZw = anyZw || allZw

	switch a.kind {

	case printAction:
		a.zeroWidth = true
	case traceAction:
		a.zeroWidth = allZw

	case callAction:
		a.zeroWidth = rules[a.name]
	case recurAction:
		a.zeroWidth = rules[a.name]
	case cornerAction, noCornerAction:
		a.zeroWidth = true

	case doAction, caseAction, ruleAction, sequenceAction:
		a.zeroWidth = allZw
	case choiceAction:
		a.zeroWidth = anyZw
	case cutAction:
		a.zeroWidth = true
	case repeatAction:
		a.zeroWidth = a.min == 0 || allZw
	case lookaheadAction:
		a.zeroWidth = true
	case rejectAction:
		a.zeroWidth = true
	case captureAction:
		a.zeroWidth = allZw
	case optionalAction:
		a.zeroWidth = true

	case matchStringAction:
		allZw = true
		anyZw = false
		if a.stringSwitch != nil {
			for _, c := range a.stringSwitch {
				c.setZeroWidth(rules)
				allZw = allZw && c.zeroWidth
				anyZw = anyZw || c.zeroWidth
			}
			// empty list should be true
		}

		anyZw = anyZw || allZw
		a.zeroWidth = anyZw

	case matchRuneAction:
		allZw = true
		anyZw = false
		if a.runeSwitch != nil {
			for _, c := range a.runeSwitch {
				c.setZeroWidth(rules)
				allZw = allZw && c.zeroWidth
				anyZw = anyZw || c.zeroWidth
			}
			// empty list should be true
		}

		anyZw = anyZw || allZw
		a.zeroWidth = anyZw

	case matchByteAction:
		allZw = true
		anyZw = false
		if a.byteSwitch != nil {
			for _, c := range a.byteSwitch {
				c.setZeroWidth(rules)
				allZw = allZw && c.zeroWidth
				anyZw = anyZw || c.zeroWidth
			}
			// empty list should be true
		}

		anyZw = anyZw || allZw
		a.zeroWidth = anyZw

	case startOfFileAction, endOfFileAction:
		a.zeroWidth = true
	case startOfLineAction:
		a.zeroWidth = true
	case endOfLineAction:
		a.zeroWidth = false

	case runeAction, stringAction:
		a.zeroWidth = false
	case runeRangeAction, runeExceptAction:
		a.zeroWidth = false

	case spaceAction, tabAction:
		a.zeroWidth = false
	case whitespaceAction:
		a.zeroWidth = a.min == 0
	case newlineAction:
		a.zeroWidth = false
	case whitespaceNewlineAction:
		a.zeroWidth = true

	case indentedBlockAction, offsideBlockAction:
		a.zeroWidth = allZw
	case indentAction, dedentAction:
		a.zeroWidth = true // an indented block or an offsideBlock can be zw

	case byteAction, byteListAction, byteStringAction:
		a.zeroWidth = false
	case byteRangeAction, byteExceptAction:
		a.zeroWidth = false
	default:
		a.zeroWidth = false
	}

	return old != a.zeroWidth
}

// Builder

type nodeBuilder struct {
	kind   string
	parent *nodeBuilder
	rule   *string
	args   []*parseAction
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

	builderPos map[string]*filePosition
	rulePos    map[string]*filePosition

	nb *nodeBuilder
	//err    error
	errors []*grammarError
	n      int
}

func (g *G) grammarConfig() *grammarConfig {
	if g.configmode == nil {
		g.configmode = g.Mode
		g.grammar.config = g.configmode.grammarConfig()
	}
	return g.grammar.config
}

func (g *G) addError(pos *filePosition, args ...any) {
	msg := fmt.Sprint(args...)
	err := &grammarError{
		message: msg,
		pos:     pos,
	}
	g.errors = append(g.errors, err)
}

func (g *G) addErrorf(pos *filePosition, s string, args ...any) {
	msg := fmt.Sprintf(s, args...)
	err := &grammarError{
		message: msg,
		pos:     pos,
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
	if g.nb == nil {
		g.addError(pos, "must call method inside BuildParser()")
		return true
	}
	if !g.nb.inRule() {
		g.addError(pos, "must call method inside Define()")
		return true
	}
	if g.Mode == nil {
		g.addError(pos, "Mode must be set first")
		return true
	}
	if config := g.grammarConfig(); !config.actionAllowed(kind) {
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

func (g *G) buildArgs(kind string, stub func()) []*parseAction {
	var rule *string
	oldNb := g.nb
	if oldNb != nil {
		rule = oldNb.rule
	}
	newNb := &nodeBuilder{kind: kind, rule: rule, parent: oldNb}
	g.nb = newNb
	stub()
	g.nb = oldNb
	return newNb.args
}

func (g *G) buildRule(rule string, stub func()) []*parseAction {
	oldNb := g.nb
	newNb := &nodeBuilder{kind: defineAction, rule: &rule, parent: oldNb}
	g.nb = newNb
	stub()
	g.nb = oldNb
	return newNb.args
}

type DefineOptions struct {
	name string
	g    *G
	a    *parseAction
	p    *filePosition
}

func (db DefineOptions) Do(stub func()) {
	db.g.defineSequence(db.name, db.p, db.a, stub)
}

func (db DefineOptions) Choice(options ...func()) {
	db.g.defineChoice(db.name, db.p, db.a, options)
}

func (db DefineOptions) Recursive(names ...string) DefineBlock {
	return db.g.defineRecursive(db.name, db.p, db.a, names)
}

type DefineBlock struct {
	name string
	g    *G
	a    *parseAction
	p    *filePosition
}

func (db DefineBlock) Do(stub func()) {
	db.g.defineSequence(db.name, db.p, db.a, stub)
}

func (db DefineBlock) Choice(options ...func()) {
	db.g.defineChoice(db.name, db.p, db.a, options)
}

func (g *G) Define(name string) DefineOptions {
	p := g.markPosition(defineAction)
	do := DefineOptions{g: g, p: p, name: name}

	if g.grammar == nil {
		return do
	} else if g.nb == nil {
		g.addError(p, "must call Define inside BuildGrammar()")
		return do
	} else if g.nb.inRule() {
		g.addError(p, "cant call Define() inside Define()")
		return do
	}

	if oldPos, ok := g.rulePos[name]; ok {
		g.addErrorf(p, "cant redefine %q, already defined at %v", name, oldPos)
		return do
	}

	g.rulePos[name] = p

	a := &parseAction{kind: ruleAction, args: nil, pos: p}
	do.a = a
	g.grammar.rules[name] = a
	g.grammarConfig().names = append(g.grammarConfig().names, name)
	return do
}

func (g *G) defineSequence(name string, definePos *filePosition, a *parseAction, stub func()) {
	p := g.markPosition(defineAction)

	if a == nil || g.grammar == nil {
		return
	} else if g.nb == nil {
		g.addError(p, "must call define inside grammar")
		return
	} else if g.nb.inRule() {
		g.addError(p, "cant call define inside define")
		return
	}

	if p.n-definePos.n != 1 {
		g.addError(p, "called in wrong position")
		return
	}

	if stub == nil {
		g.addError(p, "cant call Do() with nil")
		return
	}

	a.args = g.buildRule(name, stub)
}

func (g *G) defineChoice(name string, definePos *filePosition, a *parseAction, options []func()) {
	p := g.markPosition(defineAction)

	if a == nil || g.grammar == nil {
		return
	} else if g.nb == nil {
		g.addError(p, "must call define inside grammar")
		return
	} else if g.nb.inRule() {
		g.addError(p, "cant call define inside define")
		return
	}

	if p.n-definePos.n != 1 {
		g.addError(p, "called in wrong position")
		return
	}

	if len(options) == 0 {
		g.addError(p, "cant call .Choice() with nil")
		return
	}

	a.args = g.buildRule(name, func() {
		args := make([]*parseAction, len(options))
		for i, stub := range options {
			if stub == nil {
				g.addError(p, "cant call .Choice() with nil")
			} else {
				stubArgs := g.buildArgs(choiceAction, stub)
				args[i] = &parseAction{kind: caseAction, pos: p, args: stubArgs}
			}
		}
		c := &parseAction{kind: choiceAction, args: args, pos: p}
		g.nb.append(c)
	})
}

func (g *G) defineRecursive(name string, definePos *filePosition, a *parseAction, names []string) DefineBlock {
	p := g.markPosition(defineAction)

	db := DefineBlock{g: g, a: a, p: p, name: name}

	if a == nil || g.grammar == nil {
		return db
	} else if g.nb == nil {
		g.addError(p, "must call Recursive inside grammar")
		return db
	} else if g.nb.inRule() {
		g.addError(p, "cant call Recursive() inside a rule")
		return db
	}

	if p.n-definePos.n != 1 {
		g.addError(p, "called in wrong position")
		return db
	}

	if names == nil || len(names) == 0 {
		names = []string{name}
	} else {
		found := false
		for _, n := range names {
			if n == name {
				found = true
				break
			}
		}

		if !found {
			g.addError(p, "Recursive Rules must include own name in list")
		}
	}

	a.recursiveNames = names
	return db
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
		return
	}

	args := g.buildArgs(traceAction, stub)

	a := &parseAction{kind: traceAction, args: args, pos: p}
	g.nb.append(a)
}

func (g *G) Space() {
	p := g.markPosition(spaceAction)
	if g.shouldExit(p, spaceAction) {
		return
	}
	a := &parseAction{kind: spaceAction, pos: p}
	g.nb.append(a)
}

func (g *G) Tab() {
	p := g.markPosition(tabAction)
	if g.shouldExit(p, tabAction) {
		return
	}
	a := &parseAction{kind: tabAction, pos: p}
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

func (g *G) WhitespaceNewline() {
	p := g.markPosition(whitespaceAction)
	if g.shouldExit(p, whitespaceAction) {
		return
	}
	a := &parseAction{kind: whitespaceNewlineAction, pos: p}
	g.nb.append(a)
}

type WhitespaceOptions struct {
	g *G
	a *parseAction
	p *filePosition
}

func (wo WhitespaceOptions) Min(min int) {
	wo.g.whitespaceMin(wo.p, wo.a, min)
}

func (wo WhitespaceOptions) Max(max int) {
	wo.g.whitespaceMax(wo.p, wo.a, max)
}

func (wo WhitespaceOptions) MinMax(min int, max int) {
	wo.g.whitespaceMax(wo.p, wo.a, max)
}

func (wo WhitespaceOptions) Width(width int) {
	wo.g.whitespaceWidth(wo.p, wo.a, width)
}

func (g *G) Whitespace() WhitespaceOptions {
	p := g.markPosition(whitespaceAction)
	wo := WhitespaceOptions{g: g, p: p}

	if g.shouldExit(p, whitespaceAction) {
		return wo
	}
	a := &parseAction{kind: whitespaceAction, pos: p}
	g.nb.append(a)
	wo.a = a
	return wo
}

func (g *G) whitespaceMin(whitespacePos *filePosition, a *parseAction, min int) {
	p := g.markPosition(whitespaceAction)
	if a == nil || g.shouldExit(p, a.kind) {
		return
	}

	if p.n-whitespacePos.n != 1 {
		g.addError(p, "called in wrong position")
		return
	}

	a.min = min
}
func (g *G) whitespaceMax(whitespacePos *filePosition, a *parseAction, max int) {
	p := g.markPosition(whitespaceAction)
	if a == nil || g.shouldExit(p, a.kind) {
		return
	}

	if p.n-whitespacePos.n != 1 {
		g.addError(p, "called in wrong position")
		return
	}

	a.max = max
}
func (g *G) whitespaceMinMax(whitespacePos *filePosition, a *parseAction, min int, max int) {
	p := g.markPosition(whitespaceAction)
	if a == nil || g.shouldExit(p, a.kind) {
		return
	}

	if p.n-whitespacePos.n != 1 {
		g.addError(p, "called in wrong position")
		return
	}

	a.min = min
	a.max = max
}
func (g *G) whitespaceWidth(whitespacePos *filePosition, a *parseAction, width int) {
	p := g.markPosition(whitespaceAction)
	if a == nil || g.shouldExit(p, a.kind) {
		return
	}

	if p.n-whitespacePos.n != 1 {
		g.addError(p, "called in wrong position")
		return
	}

	a.min = width
	a.max = width
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

func (g *G) Indent() {
	p := g.markPosition(indentAction)
	if g.shouldExit(p, indentAction) {
		return
	}

	a := &parseAction{kind: indentAction, pos: p}
	g.nb.append(a)
}

func (g *G) OffsideBlock(stub func()) {
	p := g.markPosition(offsideBlockAction)
	if g.shouldExit(p, offsideBlockAction) {
		return
	} else if stub == nil {
		g.addError(p, "cant OffsideBlock() with nil")
		return
	}

	args := g.buildArgs(offsideBlockAction, stub)
	a := &parseAction{kind: offsideBlockAction, args: args, pos: p}
	g.nb.append(a)
}

func (g *G) IndentedBlock(stub func()) {
	p := g.markPosition(indentedBlockAction)
	if g.shouldExit(p, indentedBlockAction) {
		return
	} else if stub == nil {
		g.addError(p, "cant call IndentedBlock() with nil")
		return
	}

	args := g.buildArgs(indentedBlockAction, stub)
	a := &parseAction{kind: indentedBlockAction, args: args, pos: p}
	g.nb.append(a)
}

func (g *G) MatchString(stubs map[string]func()) {
	p := g.markPosition(matchStringAction)
	if g.shouldExit(p, matchStringAction) {
		return
	} else if stubs == nil {
		g.addError(p, "cant call MatchString() with nil map")
		return
	}

	args := make(map[string]*parseAction, len(stubs))
	for c, stub := range stubs {
		if !utf8.ValidString(c) {
			g.addErrorf(p, "MatchString(%q) contains invalid UTF-8", c)
		} else if c == "" {
			g.addErrorf(p, "MatchString(%q) is empty string", c)
		}

		for _, b := range g.grammarConfig().stringsReserved {
			if strings.Index(c, b) > -1 {
				g.addErrorf(p, "MatchString(%q) contains reserved string %q", c, b)
			}
		}
		if stub == nil {
			g.addError(p, "cant call MatchString() with nil function")
			return
		} else {
			stubArgs := g.buildArgs(matchStringAction, stub)
			args[c] = &parseAction{kind: caseAction, pos: p, args: stubArgs}
		}
	}
	a := &parseAction{kind: matchStringAction, stringSwitch: args, pos: p}
	g.nb.append(a)
}

func (g *G) MatchRune(stubs map[rune]func()) {
	p := g.markPosition(matchRuneAction)
	if g.shouldExit(p, matchRuneAction) {
		return
	} else if stubs == nil {
		g.addError(p, "cant call MatchRune() with nil map")
		return
	}

	args := make(map[rune]*parseAction, len(stubs))
	for c, stub := range stubs {
		if stub == nil {
			g.addError(p, "cant call MatchRune() with nil function")
		} else {
			stubArgs := g.buildArgs(matchRuneAction, stub)
			args[c] = &parseAction{kind: caseAction, pos: p, args: stubArgs}
		}
	}
	a := &parseAction{kind: matchRuneAction, runeSwitch: args, pos: p}
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
		} else if v == "" {
			g.addErrorf(p, "String(%q) is empty string", v)
		}
		for _, b := range g.grammarConfig().stringsReserved {
			if strings.Index(v, b) > -1 {
				g.addErrorf(p, "String(%q) contains reserved string %q", v, b)
			}
		}

	}

	a := &parseAction{kind: stringAction, strings: s, pos: p}
	g.nb.append(a)
}

func (g *G) MatchByte(stubs map[byte]func()) {
	p := g.markPosition(matchByteAction)
	if g.shouldExit(p, matchByteAction) {
		return
	} else if stubs == nil {
		g.addError(p, "cant call MatchByte() with nil map")
		return
	}

	args := make(map[byte]*parseAction, len(stubs))
	for c, stub := range stubs {
		if stub == nil {
			g.addError(p, "cant call MatchByte() with nil function")
		} else {
			stubArgs := g.buildArgs(matchByteAction, stub)
			args[c] = &parseAction{kind: caseAction, pos: p, args: stubArgs}
		}
	}
	a := &parseAction{kind: matchByteAction, byteSwitch: args, pos: p}
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
		if v == "" {
			g.addErrorf(p, "ByteString(%q) is empty string", v)
		}
		b[i] = []byte(v)

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

	for _, v := range s {
		if len(v) == 0 {
			g.addErrorf(p, "Bytes(%q) is empty", v)
		}

	}

	a := &parseAction{kind: byteListAction, bytes: s, pos: p}
	g.nb.append(a)
}

type RuneOption struct {
	g *G
	a *parseAction
	p *filePosition
}

func (ro RuneOption) Range(s ...string) {
	ro.g.runeRange(ro.p, ro.a, s)
}

func (ro RuneOption) Except(s ...string) {
	ro.g.runeExcept(ro.p, ro.a, s)
}

func (g *G) Rune() RuneOption {
	p := g.markPosition(runeAction)
	ro := RuneOption{g: g, p: p}
	if g.shouldExit(p, runeAction) {
		return ro
	}

	a := &parseAction{kind: runeAction, pos: p}
	g.nb.append(a)
	ro.a = a
	return ro
}

func (g *G) runeRange(repeatPos *filePosition, a *parseAction, s []string) {
	p := g.markPosition(runeRangeAction)
	if a == nil || g.shouldExit(p, runeRangeAction) {
		return
	}

	if p.n-repeatPos.n != 1 {
		g.addError(p, "called in wrong position")
		return
	}

	if len(s) == 0 {
		g.addError(p, "missing operand")
		return
	}

	args := make([]string, len(s))
	for i, v := range s {
		r := []rune(v)
		if !(len(r) == 1 || (len(r) == 3 && r[1] == '-' && r[0] < r[2])) {
			g.addError(p, "invalid range", v)
		}
		args[i] = v
	}
	*a = parseAction{kind: runeRangeAction, ranges: args, pos: p}
}

func (g *G) runeExcept(repeatPos *filePosition, a *parseAction, s []string) {
	p := g.markPosition(runeExceptAction)
	if a == nil || g.shouldExit(p, runeExceptAction) {
		return
	}

	if p.n-repeatPos.n != 1 {
		g.addError(p, "called in wrong position")
		return
	}

	if len(s) == 0 {
		g.addError(p, "missing operand")
		return
	}

	args := make([]string, len(s))
	for i, v := range s {
		r := []rune(v)
		if !(len(r) == 1 || (len(r) == 3 && r[1] == '-' && r[0] < r[2])) {
			g.addError(p, "invalid range", v)
		}
		args[i] = v
	}
	*a = parseAction{kind: runeExceptAction, ranges: args, pos: p, inverted: true}
}

type ByteOption struct {
	g *G
	a *parseAction
	p *filePosition
}

func (ro ByteOption) Range(s ...string) {
	ro.g.byteRange(ro.p, ro.a, s)
}

func (ro ByteOption) Except(s ...string) {
	ro.g.byteExcept(ro.p, ro.a, s)
}

func (g *G) Byte() ByteOption {
	p := g.markPosition(byteAction)
	bo := ByteOption{g: g, p: p}

	if g.shouldExit(p, byteAction) {
		return bo
	}

	a := &parseAction{kind: byteAction, pos: p}
	g.nb.append(a)
	bo.a = a
	return bo
}

func (g *G) byteRange(repeatPos *filePosition, a *parseAction, s []string) {
	p := g.markPosition(byteRangeAction)
	if a == nil || g.shouldExit(p, byteRangeAction) {
		return
	}

	if p.n-repeatPos.n != 1 {
		g.addError(p, "called in wrong position")
		return
	}

	if len(s) == 0 {
		g.addError(p, "missing operand")
		return
	}

	args := make([]string, len(s))
	for i, v := range s {
		r := []byte(v)
		if !(len(r) == 1 || (len(r) == 3 && r[1] == '-' && r[0] < r[2])) {
			g.addError(p, "invalid range", v)
		}
		args[i] = v
	}
	*a = parseAction{kind: byteRangeAction, ranges: args, pos: p}
}

func (g *G) byteExcept(repeatPos *filePosition, a *parseAction, s []string) {
	p := g.markPosition(byteExceptAction)
	if a == nil || g.shouldExit(p, byteExceptAction) {
		return
	}

	if p.n-repeatPos.n != 1 {
		g.addError(p, "called in wrong position")
		return
	}

	if len(s) == 0 {
		g.addError(p, "missing operand")
		return
	}

	args := make([]string, len(s))
	for i, v := range s {
		r := []byte(v)
		if !(len(r) == 1 || (len(r) == 3 && r[1] == '-' && r[0] < r[2])) {
			g.addError(p, "invalid range", v)
		}
		args[i] = v
	}
	*a = parseAction{kind: byteExceptAction, ranges: args, pos: p, inverted: true}
}

func (g *G) Call(name string) {
	p := g.markPosition(callAction)
	if g.shouldExit(p, callAction) {
		return
	}
	a := &parseAction{kind: callAction, name: name, pos: p}
	g.nb.append(a)
}

func (g *G) Recur(name string) {
	p := g.markPosition(recurAction)
	if g.shouldExit(p, recurAction) {
		return
	}
	a := &parseAction{kind: recurAction, name: name, pos: p}
	g.nb.append(a)
}

func (g *G) Stump(name string) {
	p := g.markPosition(stumpAction)
	if g.shouldExit(p, stumpAction) {
		return
	}
	a := &parseAction{kind: stumpAction, name: name, pos: p}
	g.nb.append(a)
}

func (g *G) NoCorner(name string, precedence int) {
	p := g.markPosition(noCornerAction)
	if g.shouldExit(p, noCornerAction) {
		return
	}
	a := &parseAction{kind: noCornerAction, pos: p, precedence: precedence, name: name}
	g.nb.append(a)
}

func (g *G) Corner(name string, precedence int) {
	p := g.markPosition(cornerAction)
	if g.shouldExit(p, cornerAction) {
		return
	}
	a := &parseAction{kind: cornerAction, pos: p, precedence: precedence, name: name}
	g.nb.append(a)
}
func (g *G) Do(stub func()) {
	p := g.markPosition(doAction)
	if g.shouldExit(p, doAction) {
		return
	} else if stub == nil {
		g.addError(p, "cant call Sequence() with nil")
		return
	}

	args := g.buildArgs(doAction, stub)
	a := &parseAction{kind: doAction, pos: p, args: args}
	g.nb.append(a)
}

func (g *G) Capture(name string, stub func()) {
	p := g.markPosition(captureAction)
	if g.shouldExit(p, captureAction) {
		return
	} else if stub == nil {
		g.addError(p, "cant call Capture() with nil")
		return
	}

	args := g.buildArgs(captureAction, stub)

	a := &parseAction{kind: captureAction, name: name, args: args, pos: p}
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
		} else {
			stubArgs := g.buildArgs(choiceAction, stub)
			args[i] = &parseAction{kind: caseAction, pos: p, args: stubArgs}
		}
	}
	a := &parseAction{kind: choiceAction, args: args, pos: p}
	g.nb.append(a)
}

func (g *G) Lookahead(stub func()) {
	p := g.markPosition(lookaheadAction)
	if g.shouldExit(p, lookaheadAction) {
		return
	} else if stub == nil {
		g.addError(p, "cant call Lookahead() with nil")
		return
	}
	args := g.buildArgs(lookaheadAction, stub)
	a := &parseAction{kind: lookaheadAction, args: args, pos: p}
	g.nb.append(a)
}

func (g *G) Reject(stub func()) {
	p := g.markPosition(rejectAction)
	if g.shouldExit(p, rejectAction) {
		return
	} else if stub == nil {
		g.addError(p, "cant call Reject() with nil")
		return
	}
	args := g.buildArgs(rejectAction, stub)
	a := &parseAction{kind: rejectAction, args: args, pos: p}
	g.nb.append(a)
}

type RepeatOptions struct {
	g *G
	a *parseAction
	p *filePosition
}

func (ro RepeatOptions) Min(min int) RepeatBlock {
	return ro.g.repeatMin(ro.p, ro.a, min)
}

func (ro RepeatOptions) Max(max int) RepeatBlock {
	return ro.g.repeatMax(ro.p, ro.a, max)
}

func (ro RepeatOptions) MinMax(min int, max int) RepeatBlock {
	return ro.g.repeatMinMax(ro.p, ro.a, min, max)
}

func (ro RepeatOptions) N(n int) RepeatBlock {
	return ro.g.repeatN(ro.p, ro.a, n)
}

func (ro RepeatOptions) Do(stub func()) {
	ro.g.repeatSequence(ro.p, ro.a, stub)
}

func (ro RepeatOptions) Choice(options ...func()) {
	ro.g.repeatChoice(ro.p, ro.a, options)
}

type RepeatBlock struct {
	g *G
	a *parseAction
	p *filePosition
}

func (ro RepeatBlock) Do(stub func()) {
	ro.g.repeatSequence(ro.p, ro.a, stub)
}

func (ro RepeatBlock) Choice(options ...func()) {
	ro.g.repeatChoice(ro.p, ro.a, options)
}

func (g *G) Repeat() RepeatOptions {
	p := g.markPosition(repeatAction)
	ro := RepeatOptions{g: g, p: p}

	if g.shouldExit(p, repeatAction) {
		return ro
	}

	a := &parseAction{kind: repeatAction, pos: p}
	g.nb.append(a)
	ro.a = a
	return ro

}

func (g *G) repeatMin(repeatPos *filePosition, a *parseAction, min int) RepeatBlock {
	p := g.markPosition(repeatAction)
	rb := RepeatBlock{g: g, a: a, p: p}
	if a == nil || g.shouldExit(p, a.kind) {
		return rb
	}

	if p.n-repeatPos.n != 1 {
		g.addError(p, "called in wrong position")
		return rb
	}
	a.min = min
	return rb
}

func (g *G) repeatMax(repeatPos *filePosition, a *parseAction, max int) RepeatBlock {
	p := g.markPosition(repeatAction)
	rb := RepeatBlock{g: g, a: a, p: p}
	if a == nil || g.shouldExit(p, a.kind) {
		return rb
	}

	if p.n-repeatPos.n != 1 {
		g.addError(p, "called in wrong position")
		return rb
	}
	a.max = max
	return rb
}

func (g *G) repeatMinMax(repeatPos *filePosition, a *parseAction, min int, max int) RepeatBlock {
	p := g.markPosition(repeatAction)
	rb := RepeatBlock{g: g, a: a, p: p}
	if a == nil || g.shouldExit(p, a.kind) {
		return rb
	}

	if p.n-repeatPos.n != 1 {
		g.addError(p, "called in wrong position")
		return rb
	}
	a.min = min
	a.max = max
	return rb
}

func (g *G) repeatN(repeatPos *filePosition, a *parseAction, n int) RepeatBlock {
	p := g.markPosition(repeatAction)
	rb := RepeatBlock{g: g, a: a, p: p}
	if a == nil || g.shouldExit(p, a.kind) {
		return rb
	}

	if p.n-repeatPos.n != 1 {
		g.addError(p, "called in wrong position")
		return rb
	}
	a.min = n
	a.max = n
	return rb
}

func (g *G) repeatSequence(repeatPos *filePosition, a *parseAction, stub func()) {
	p := g.markPosition(repeatAction)
	if a == nil || g.shouldExit(p, a.kind) {
		return
	}

	if p.n-repeatPos.n != 1 {
		g.addError(p, "called in wrong position")
		return
	}

	if stub == nil {
		g.addError(p, "cant call Do() with nil")
		return
	}

	a.args = g.buildArgs(repeatAction, stub)
}

func (g *G) repeatChoice(repeatPos *filePosition, a *parseAction, options []func()) {
	p := g.markPosition(repeatAction)
	if a == nil || g.shouldExit(p, a.kind) {
		return
	}

	if p.n-repeatPos.n != 1 {
		g.addError(p, "called in wrong position")
		return
	}

	if len(options) == 0 {
		g.addError(p, "cant call Choice() with nil")
		return
	}

	args := make([]*parseAction, len(options))
	for i, stub := range options {
		if stub == nil {
			g.addError(p, "cant call Choice() with nil")
		} else {
			stubArgs := g.buildArgs(choiceAction, stub)
			args[i] = &parseAction{kind: caseAction, pos: p, args: stubArgs}
		}
	}
	c := &parseAction{kind: choiceAction, args: args, pos: p}
	a.args = []*parseAction{c}
}

type OptionalBlock struct {
	g *G
	a *parseAction
	p *filePosition
}

func (ro OptionalBlock) Do(stub func()) {
	ro.g.optionalSequence(ro.p, ro.a, stub)
}

func (ro OptionalBlock) Choice(options ...func()) {
	ro.g.optionalChoice(ro.p, ro.a, options)
}

func (g *G) Optional() OptionalBlock {
	p := g.markPosition(optionalAction)
	ob := OptionalBlock{g: g, p: p}

	if g.shouldExit(p, optionalAction) {
		return ob
	}

	a := &parseAction{kind: optionalAction, pos: p}
	g.nb.append(a)
	ob.a = a
	return ob
}

func (g *G) optionalSequence(optionalPos *filePosition, a *parseAction, stub func()) {
	p := g.markPosition(optionalAction)
	if a == nil || g.shouldExit(p, a.kind) {
		return
	}

	if p.n-optionalPos.n != 1 {
		g.addError(p, "called in wrong position")
		return
	}

	if stub == nil {
		g.addError(p, "cant call Do() with nil")
		return
	}

	a.args = g.buildArgs(optionalAction, stub)
}

func (g *G) optionalChoice(optionalPos *filePosition, a *parseAction, options []func()) {
	p := g.markPosition(optionalAction)
	if a == nil || g.shouldExit(p, a.kind) {
		return
	}

	if p.n-optionalPos.n != 1 {
		g.addError(p, "called in wrong position")
		return
	}

	if len(options) == 0 {
		g.addError(p, "cant call Choice() with nil")
		return
	}

	args := make([]*parseAction, len(options))
	for i, stub := range options {
		if stub == nil {
			g.addError(p, "cant call Choice() with nil")
		} else {
			stubArgs := g.buildArgs(caseAction, stub)
			args[i] = &parseAction{kind: caseAction, pos: p, args: stubArgs}
		}
	}
	c := &parseAction{kind: choiceAction, args: args, pos: p}
	a.args = []*parseAction{c}
}

func (g *G) Builder(name string, stub any) {
	p := g.markPosition(builderAction)
	if g.nb == nil {
		g.addError(p, "must call Builder() inside grammar")
		return
	} else if g.nb.inRule() {
		g.addError(p, "cant call Builder() inside define")
		return
	} else if stub == nil {
		g.addError(p, "cant call Builder() with nil")
		return
	}

	if _, ok := g.grammar.builders[name]; ok {
		oldPos := g.builderPos[name]
		g.addErrorf(p, "cant redefine %q, already defined at %v", name, oldPos)
	} else {
		g.builderPos[name] = p

		// pass bytes in byte mode?

		switch stub.(type) {
		case func(string, []any) (any, error):
			g.grammar.builders[name] = stub
		default:
			g.addError(p, "builder has wrong type signature. try func(string, []any) (any, error).")
		}
	}

}

//
// Grammar
//

type grammarConfig struct {
	name            string
	actionsDisabled []string
	stringsReserved []string
	tabstop         int
	start           string
	startIdx        int
	index           map[string]int
	logFunc         func(string, ...any)
	names           []string
}

func (c *grammarConfig) actionAllowed(s string) bool {
	for _, v := range c.actionsDisabled {
		if v == s {
			return false
		}
	}
	return true
}

/// ---

type Grammar struct {
	config   *grammarConfig
	rules    map[string]*parseAction
	builders map[string]any

	pos *filePosition //
	Err error
}

func (g *Grammar) Parser() *Parser {
	if g.Err != nil {
		p := &Parser{err: g.Err}
		return p
	}

	rules := make([]parseFunc, len(g.rules))
	for i, n := range g.config.names {
		rule := g.rules[n]
		rules[i] = buildRule(g.config, n, rule)
	}

	p := &Parser{
		rules:    rules,
		config:   g.config,
		builders: g.builders,
	}
	return p
}

func buildGrammar(pos *filePosition, mode GrammarMode, stub func(*G)) *Grammar {
	g := &Grammar{}
	g.builders = make(map[string]any, 0)
	g.rules = make(map[string]*parseAction, 0)
	g.pos = pos

	bg := &G{
		grammar:    g,
		n:          1,
		nb:         &nodeBuilder{kind: grammarAction},
		LogFunc:    Printf,
		Mode:       mode,
		rulePos:    make(map[string]*filePosition, 0),
		builderPos: make(map[string]*filePosition, 0),
	}

	if stub == nil {
		bg.addError(g.pos, "cant call BuildGrammar() with nil")
	}

	stub(bg)

	// if only one rule, set it as the starting rule

	if bg.Start == "" && len(g.rules) == 1 {
		for k := range g.rules {
			bg.Start = k
			break
		}
	}

	// grammars must have a start rule

	if bg.Start == "" {
		bg.addError(g.pos, "starting rule undefined")
	} else if _, ok := g.rules[bg.Start]; !ok {
		bg.addErrorf(g.pos, "starting rule %q is missing", bg.Start)
	}

	// mark down all g.Call() and g.Capture()

	capturePos := make(map[string][]*filePosition, 0)
	callPos := make(map[string][]*filePosition, 0)

	for _, rule := range g.rules {
		rule.walk(func(a *parseAction) {
			switch a.kind {
			case captureAction:
				capturePos[a.name] = append(capturePos[a.name], a.pos)

			case callAction:
				callPos[a.name] = append(callPos[a.name], a.pos)
			case recurAction:
				callPos[a.name] = append(callPos[a.name], a.pos)
			}
		})
	}

	// ensure each g.Call() has a rule

	for name, pos := range callPos {
		if _, ok := g.rules[name]; !ok {
			for _, p := range pos {
				bg.addErrorf(p, "missing rule %q", name)
			}
		}
	}

	// ensure each rule gets called at least once

	for name := range g.rules {
		if name != bg.Start && callPos[name] == nil {
			p := bg.rulePos[name]
			bg.addErrorf(p, "unused rule %q", name)
		}
	}

	// and every builder has a matching g.Capture()

	if len(g.builders) > 0 {
		for name, _ := range g.builders {
			if _, ok := capturePos[name]; !ok {
				p := bg.builderPos[name]
				bg.addErrorf(p, "missing capture %q for builder", name)
			}
		}

		for name, pos := range capturePos {
			if _, ok := bg.builderPos[name]; !ok {
				for _, p := range pos {
					bg.addErrorf(p, "missing builder %q for capture", name)
				}
			}
		}
	}

	// mark all terminal rules

	for _, rule := range g.rules {
		rule.setTerminal()
	}

	// mark all zero width rules, using a closure algorithm
	// repeat until no new rules marked

	// in theory we could do a SCC analysis and avoid
	// retesting, but that sounds like a lot of work

	n := 1

	zwMap := make(map[string]bool, len(g.rules))
	for n > 0 {
		n = 0
		for name, rule := range g.rules {
			if rule.setZeroWidth(zwMap) {
				n++
			}
			zwMap[name] = rule.zeroWidth
		}
	}

	// check nullable rules have a '?' at the end of name

	for name, rule := range g.rules {
		nullable := name[len(name)-1] == '?'
		if nullable {
			if !rule.zeroWidth {
				p := bg.rulePos[name]
				bg.addErrorf(p, "nullable rule %q not actually nullable", name)
			}
		} else {
			if rule.zeroWidth {
				p := bg.rulePos[name]
				bg.addErrorf(p, "rule %q is nullable but not marked", name)
			}
		}
	}

	// left recursion check

	directCalls := make(map[string][]string)

	for name, rule := range g.rules {
		directCalls[name] = rule.leftCalls()
	}

	indirectCalls := make(map[string][]string)

	for name, _ := range g.rules {
		seen := make(map[string]bool)
		seen[name] = true

		indirects := make([]string, 0)

		var walk func(string)
		walk = func(s string) {
			for _, v := range directCalls[s] {
				indirects = append(indirects, v)
				if _, ok := seen[v]; !ok {
					seen[v] = true
					walk(v)
				}
			}

		}
		walk(name)
		// cycles := make([]string, 0)

		indirectCalls[name] = indirects
	}

	mutualCalls := make(map[string][]string)

	for name, _ := range g.rules {
		mutuals := make([]string, 0)

		for _, rule_corner := range indirectCalls[name] {
			if name == rule_corner {
				mutuals = append(mutuals, name)
			} else {
				for _, n := range indirectCalls[rule_corner] {
					if n == name {
						mutuals = append(mutuals, name)
						break
					}
				}
			}

		}
		mutualCalls[name] = mutuals
	}

	for name, rule := range g.rules {

		mutuals := mutualCalls[name]

		if rule.recursiveNames != nil && len(rule.recursiveNames) > 0 {
			missing := make([]string, 0)

			for _, i := range rule.recursiveNames {
				found := false
				for _, r := range mutuals {
					if i == r {
						found = true
						break
					}

				}

				if !found {
					missing = append(missing, i)
				}
			}
			if len(missing) > 0 {
				p := bg.rulePos[name]
				bg.addErrorf(p, "%s is not left recursive with %v, but is defined to be", name, missing)
			}

			missing = make([]string, 0)

			for _, i := range mutuals {
				found := false
				for _, r := range rule.recursiveNames {
					if i == r {
						found = true
						break
					}

				}

				if !found {
					missing = append(missing, i)
				}
			}
			if len(missing) > 0 {
				p := bg.rulePos[name]
				bg.addErrorf(p, "%s is left recursive with %v, but not defined to be", name, missing)
			}
			if len(rule.recursiveNames) != 1 || rule.recursiveNames[0] != name {
				p := bg.rulePos[name]
				bg.addErrorf(p, "%s is mutually left recursive, and this is currently unsupported", name)
			}

		} else if len(mutuals) == 1 && mutuals[0] == name {
			p := bg.rulePos[name]
			bg.addErrorf(p, "%s is directly left recursive, and specified not to be", name)
		} else if len(mutuals) > 0 {
			p := bg.rulePos[name]
			bg.addErrorf(p, "%s is mutually left recursive with %v, and specified not to be", name, mutualCalls[name])
			break
		}
	}

	err := errorSummary(pos, bg.errors)

	if err != nil {
		return &Grammar{Err: err}
	}

	index := make(map[string]int, len(g.config.names))

	for i, n := range g.config.names {
		index[n] = i
	}

	g.config.start = bg.Start
	g.config.startIdx = index[bg.Start]
	g.config.logFunc = bg.LogFunc
	g.config.index = index

	return g
}

//
//  	Parser
//

type parseFunc func(*parserState) bool

type parserCorner struct {
	name   string
	offset int
	state  *parserState
	nodes  []Node

	precedence int
}

type parserInput struct {
	rules   []parseFunc
	starts  map[int]int // XXX no column check
	corner  *parserCorner
	buf     string
	length  int
	nodes   []Node
	tabstop int

	inside map[int]int

	// these dont get set/used as much
	trace bool
	// this needs to be preserved even when a rule fails
	choiceExit bool
}

type parserState struct {
	i *parserInput

	offset int
	column int

	lineStart  int // offset
	lineNumber int
	lineIndent int // column

	numNodes     int
	lastSibling  int
	countSibling int

	matchIndent parseFunc

	precedence int
}

func atEnd(s *parserState) bool {
	return s.offset >= s.i.length
}

func peekByte(s *parserState) byte {
	return s.i.buf[s.offset]
}

func peekString(s *parserState, n int) string {
	end := s.offset + n
	if end > s.i.length {
		end = s.i.length
	}
	return s.i.buf[s.offset:end]
}

func peekBytes(s *parserState, n int) []byte {
	end := s.offset + n
	if end > s.i.length {
		end = s.i.length
	}
	return []byte(s.i.buf[s.offset:end])
}

func peekRune(s *parserState) (rune, int) {
	return utf8.DecodeRuneInString(s.i.buf[s.offset:])
}

func advanceState(s *parserState, length int) {
	newOffset := s.offset + length

	// in theory this should check whitespace and newlines, but
	// in practice: no

	for i := s.offset; i < newOffset; i++ {
		switch s.i.buf[i] {
		case byte('\t'):
			width := 1
			if s.i.tabstop > 1 {
				width = s.i.tabstop - (s.column % s.i.tabstop)
			}
			s.column += width
		case byte('\r'):
			s.column = 0
			s.lineIndent = 0
			s.lineStart = i + 1
			s.lineNumber++
		case byte('\n'):
			s.column = 0
			s.lineIndent = 0
			s.lineStart = i + 1
			if i == 0 || (s.i.buf[i-1] != byte('\r')) {
				s.lineNumber++
			}
		default:
			s.column += 1
		}
	}

	s.offset = newOffset
}

func acceptString(s *parserState, v string) bool {
	length_v := len(v)
	b := peekString(s, length_v)
	if b == v {
		advanceState(s, length_v)
		return true
	}
	return false
}

func acceptBytes(s *parserState, v []byte) bool {
	length_v := len(v)
	b := peekString(s, length_v)
	if b == string(v) {
		advanceState(s, length_v)
		return true
	}
	return false
}

func acceptAny(s *parserState, o []string) bool {
	for _, v := range o {
		if acceptString(s, v) {
			return true
		}
	}
	return false
}

func acceptWhitespace(s *parserState, minWidth int, maxWidth int) bool {
	column := s.column
	w := 0
	c := 0
outer:
	for i := s.offset; i < s.i.length; i++ {
		b := s.i.buf[i]

		if b == byte('\t') {
			tabWidth := s.i.tabstop - (column % s.i.tabstop)
			if w+tabWidth > maxWidth {
				// to handle matching part of a tabstop
				// i.e Ws(4), Ws(4) meeting a tab
				// we advance the offset up to the tabstop
				// but advance the column, so that the next
				// calculation of the tabstop is correct

				partialTabWidth := maxWidth - w
				advanceState(s, c) // advance up to tab
				s.column += partialTabWidth
				return true
			} else {
				column += tabWidth
				w += tabWidth
				c += 1
			}
		} else if b == byte(' ') {
			column += 1
			w += 1
			c += 1
		} else {
			break outer
		}

		if maxWidth > 0 && w >= maxWidth {
			break
		}
	}
	if w >= minWidth && (maxWidth == 0 || w <= maxWidth) {
		advanceState(s, c)
		return true
	}
	return false
}

func acceptWhitespaceOrNewline(s *parserState) bool {
	c := 0
outer:
	for i := s.offset; i < s.i.length; i++ {
		switch s.i.buf[i] {
		case byte('\t'), byte(' '), byte('\r'), byte('\n'):
			c += 1
		default:
			break outer
		}
	}
	if c > 0 {
		advanceState(s, c)
		return true
	}
	return false
}

func acceptNewline(s *parserState) bool {
	b := peekByte(s)
	if b == byte('\n') {
		advanceState(s, 1)
		return true
	} else if b == byte('\r') {
		advanceState(s, 1)
		if s.offset >= s.i.length {
			return true
		}
		b = peekByte(s)
		if b == byte('\n') {
			advanceState(s, 1)
		}
		return true
	}
	return false
}

func copyState(s *parserState, into *parserState) {
	*into = *s
}

func mergeState(s *parserState, new *parserState) {
	*s = *new
}

func trimState(s *parserState, new *parserState) {
	s.i.nodes = s.i.nodes[:s.numNodes]
}

func startCapture(s *parserState, st *parserState) {
	*st = *s
	st.countSibling = 0
	st.lastSibling = 0
}

func mergeCapture(s *parserState, name string, new *parserState) {
	nextSibling := new.lastSibling
	lastSibling := 0
	nodes := new.i.nodes

	for i := 0; i < new.countSibling; i++ {
		nodeSibling := nodes[nextSibling].sibling

		nodes[nextSibling].sibling = lastSibling
		nodes[nextSibling].nsibling = i

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

	new.i.nodes = append(new.i.nodes[:new.numNodes], node)
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
			nodeSibling := s.i.nodes[nextSibling].sibling

			s.i.nodes[nextSibling].sibling = lastSibling
			s.i.nodes[nextSibling].nsibling = i

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
		s.i.nodes = append(s.i.nodes[:s.numNodes], node)
		// s.children = []int{}
		s.countSibling = 0
		s.lastSibling = s.numNodes
		s.numNodes = s.numNodes + 1
		return s.numNodes - 1
	}
}

func startCorner(s *parserState, s1 *parserState) {
	*s1 = *s
	s1.countSibling = 0
	s1.lastSibling = 0
	s1.precedence = 0
}

func pluckCorner(name string, s *parserState, s1 *parserState) {
	nodes := []Node{}
	for i := s.numNodes; i < s1.numNodes; i++ {
		nodes = append(nodes, s.i.nodes[i])
	}

	c := &parserCorner{
		name:       name,
		state:      s1,
		offset:     s.offset,
		nodes:      nodes,
		precedence: s1.precedence,
	}

	s.i.corner = c
}

func applyCorner(s *parserState) {
	c := s.i.corner
	s1 := c.state

	s.offset = s1.offset
	s.column = s1.column

	s.lineStart = s.lineStart
	s.lineNumber = s.lineNumber
	s.lineIndent = s.lineIndent

	for _, n := range c.nodes {
		n.sibling = s.lastSibling
		n.nsibling = s.countSibling

		s.i.nodes = append(s.i.nodes[:s.numNodes], n)

		s.lastSibling = s.numNodes
		s.countSibling = s.countSibling + 1
		s.numNodes = s.numNodes + 1
	}

	s.i.corner = nil
}

func buildRule(c *grammarConfig, name string, a *parseAction) parseFunc {
	if a == nil {
		// when a func() stub has no rules
		return func(s *parserState) bool {
			return true
		}
	}

	switch a.kind {
	case ruleAction:
		rules := make([]parseFunc, len(a.args))
		for i, r := range a.args {
			rules[i] = buildAction(c, r)
		}

		if len(rules) == 0 {
			return func(s *parserState) bool { return true }
		}

		idx := c.index[name]

		if a.recursiveNames == nil || len(a.recursiveNames) == 0 {
			return func(s *parserState) bool {
				// exit if corner?

				var s1 parserState
				copyState(s, &s1)
				oldChoice := s1.i.choiceExit
				oldStart := s1.i.starts[idx]
				s1.i.choiceExit = false
				s1.i.starts[idx] = s1.offset

				for _, r := range rules {
					if !r(&s1) {
						s.i.choiceExit = oldChoice
						s1.i.starts[idx] = oldStart
						return false
					}
				}
				s1.i.choiceExit = oldChoice
				s1.i.starts[idx] = oldStart
				mergeState(s, &s1)
				return true
			}
		} else {
			return func(s *parserState) bool {
				oldChoice := s.i.choiceExit
				oldStart := s.i.starts[idx]

				// if there's already a corner, and it is us
				// we can grow it (or in our recursiveNames

				// s.precedence = s.i.inside[idx]

				var s1 parserState
				startCorner(s, &s1)
				s.i.choiceExit = false
				s.i.starts[idx] = s.offset

				for _, r := range rules {
					if !r(&s1) {
						s.i.choiceExit = oldChoice
						s.i.starts[idx] = oldStart
						return false
					}
				}

				pluckCorner(name, s, &s1)
				//fmt.Println("found seed", s.i.corner.precedence)
			growCorner:
				for true {
					var s1 parserState
					startCorner(s, &s1)
					for _, r := range rules {
						if !r(&s1) {
							break growCorner
						}
					}
					if s.i.corner != nil { // shouldn't happen if NoRecur used in rules
						break growCorner
					}
					pluckCorner(name, s, &s1)
					// fmt.Println("grown seed", s.i.corner.precedence)
				}
				// fmt.Println("done", s.i.corner.precedence)
				applyCorner(s)

				s.i.choiceExit = oldChoice
				s.i.starts[idx] = oldStart

				return true
			}
		}

	default:
		return func(s *parserState) bool {
			return false
		}
	}
}

func buildAction(c *grammarConfig, a *parseAction) parseFunc {
	if a == nil {
		// when a func() stub has no rules
		return func(s *parserState) bool {
			return true
		}
	}
	switch a.kind {
	case printAction:
		prefix := a.pos
		fn := c.logFunc
		return func(s *parserState) bool {
			msg := fmt.Sprint(a.message...)
			fn("%v: Print(%q) called, at line %v, col %v\n", prefix, msg, s.lineNumber, s.column)
			if s.offset < s.i.length {
				fn("next char: %q\n", s.i.buf[s.offset])
			}
			return true
		}
	case traceAction:
		prefix := a.pos
		fn := c.logFunc

		rules := make([]parseFunc, len(a.args))
		for i, r := range a.args {
			rules[i] = buildAction(c, r)
		}

		return func(s *parserState) bool {
			oldTrace := s.i.trace

			if !oldTrace {
				fn("%v: Trace() starting, at line %v, col %v\n", prefix, s.lineNumber, s.column)
			}
			result := true

			var s1 parserState
			copyState(s, &s1)
			s1.i.trace = true
			for _, v := range rules {
				if !v(&s1) {
					result = false
					break
				}
			}

			if !oldTrace {
				s1.i.trace = false
				if result {
					fn("%v: Trace() ending, at line %v, col %v\n", prefix, s1.lineNumber, s1.column)
					mergeState(s, &s1)
				} else {
					fn("%v: Trace() failing, at line %v, col %v\n", prefix, s.lineNumber, s.column)
				}
			} else if result {
				mergeState(s, &s1)
			}

			return result
		}
	case cornerAction:
		name := a.name
		idx := c.index[name]
		precedence := a.precedence
		return func(s *parserState) bool {
			if s.i.inside[idx] > precedence {
				return false
			}
			// fmt.Println(s.i.inside, precedence)

			// case 1. parse for corner, we're left, fine
			// case 2. parse for non corner, left rec is optional, should be fine
			// case 3. parse for non corner, optional prefix matches, so no left rec
			// case 4. parse for non corner, direct left rec, fails then

			// we can't check s.i.corner unless we know _absolutely_
			// left rec is always direct

			s.precedence = precedence
			return true
		}
	case noCornerAction:
		name := a.name
		idx := c.index[name]
		precedence := a.precedence
		return func(s *parserState) bool {
			if s.i.inside[idx] > precedence {
				return false
			}

			if s.i.corner == nil || s.i.corner.offset != s.offset {
				// fmt.Println("set", precedence, "inside", s.i.inside)
				s.precedence = precedence
				return true
			}
			return false
		}

	case recurAction, stumpAction:
		prefix := a.pos
		name := a.name
		idx := c.index[name]
		fn := c.logFunc
		var rule parseFunc

		isStump := a.kind == stumpAction

		return func(s *parserState) bool {
			if rule == nil {
				rule = s.i.rules[idx] // can't move this out to runtime unless we reorder defs
			}

			out := false

			if off, ok := s.i.starts[idx]; ok && off == s.offset {
				// we are the left most rule, and we have no seed rule to match
				if s.i.trace {
					fn("%v: Left Recur(%q) starting, at line %v, col %v\n", prefix, name, s.lineNumber, s.column)
				}

				out = false

				precedence := s.precedence
				if isStump {
					precedence++
				}

				if s.i.corner != nil && s.i.corner.precedence >= precedence {
					if s.i.corner.name == name && s.i.corner.offset == s.offset {
						applyCorner(s)
						// fmt.Println("set", precedence, "inside", s.i.inside)
						if s.i.trace {
							fn("%v: Left Recur(%q) returning, at line %v, col %v\n", prefix, name, s.lineNumber, s.column)
						}
						return true
					}

				}

				if s.i.trace {
					fn("%v: Left Recur(%q) failing, at line %v, col %v\n", prefix, name, s.lineNumber, s.column)
				}
				return false

			} else if s.i.corner == nil {
				// we are not the left most rule
				if s.i.trace {
					fn("%v: Call Recur(%q) starting, at line %v, col %v\n", prefix, name, s.lineNumber, s.column)
				}

				oldInside, ok := s.i.inside[idx]

				p := s.precedence

				if isStump {
					p++
				}

				if p >= oldInside {
					s.i.inside[idx] = p
				}

				//fmt.Println("recur", p)

				out = rule(s)

				if ok {
					s.i.inside[idx] = oldInside
				} else {
					delete(s.i.inside, idx)
				}

				if s.i.trace {
					if out {
						fn("%v: Call Recur(%q) returning, at line %v, col %v\n", prefix, name, s.lineNumber, s.column)
					} else {
						fn("%v: Call Recur(%q) failing, at line %v, col %v\n", prefix, name, s.lineNumber, s.column)
					}
				}
				return out
			}
			return false
		}
	case callAction:
		prefix := a.pos
		name := a.name
		idx := c.index[name]
		fn := c.logFunc
		var rule parseFunc
		return func(s *parserState) bool {
			if s.i.trace {
				fn("%v: Call(%q) starting, at line %v, col %v\n", prefix, name, s.lineNumber, s.column)
			}

			if rule == nil {
				rule = s.i.rules[idx] // can't move this out to runtime unless we reorder defs
			}

			out := rule(s)
			if s.i.trace {
				if out {
					fn("%v: Call(%q) exiting, at line %v, col %v\n", prefix, name, s.lineNumber, s.column)
				} else {
					fn("%v: Call(%q) failing, at line %v, col %v\n", prefix, name, s.lineNumber, s.column)
				}
			}
			return out
		}

	case offsideBlockAction:
		rules := make([]parseFunc, len(a.args))
		for i, r := range a.args {
			rules[i] = buildAction(c, r)
		}
		return func(s *parserState) bool {
			var s1 parserState
			copyState(s, &s1)

			oldMatch := s.matchIndent

			width := s.column - s.lineIndent

			newMatch := func(s *parserState) bool {
				if oldMatch != nil && !oldMatch(s) {
					return false
				}

				if width == 0 {
					return true
				}
				return acceptWhitespace(s, width, width)
			}

			s1.matchIndent = newMatch

			for _, r := range rules {
				if !r(&s1) {
					return false
				}
			}

			s1.matchIndent = oldMatch
			mergeState(s, &s1)
			return true
		}

	case indentedBlockAction:
		rules := make([]parseFunc, len(a.args))
		for i, r := range a.args {
			rules[i] = buildAction(c, r)
		}
		return func(s *parserState) bool {
			var s1 parserState
			copyState(s, &s1)

			oldMatch := s.matchIndent

			newMatch := func(s *parserState) bool {
				if oldMatch != nil && !oldMatch(s) {
					return false
				}

				start := s.offset
				// try and find whitespace
				if !acceptWhitespace(s, 1, 0) {
					return false
				}

				prefix := s.i.buf[start:s.offset]

				s.matchIndent = func(s *parserState) bool {
					return (oldMatch == nil || oldMatch(s)) && acceptString(s, prefix)
				}

				return true
			}

			s1.matchIndent = newMatch

			for _, r := range rules {
				if !r(&s1) {
					return false
				}
			}
			s1.matchIndent = s.matchIndent
			mergeState(s, &s1)
			return true
		}

	case indentAction:
		return func(s *parserState) bool {
			if s.matchIndent == nil {
				s.lineIndent = s.column
				return true
			} else if s.matchIndent(s) {
				s.lineIndent = s.column
				return true
			}
			return false
		}

	// case dedentAction

	case spaceAction:
		return func(s *parserState) bool {
			if atEnd(s) {
				return false
			}
			return acceptString(s, " ")
		}
	case tabAction:
		return func(s *parserState) bool {
			if atEnd(s) {
				return false
			}
			return acceptString(s, "\t")
		}

	case whitespaceAction:
		min_n := a.min
		max_n := a.max
		return func(s *parserState) bool {
			if atEnd(s) {
				return true
			}
			acceptWhitespace(s, min_n, max_n)
			return true
		}

	case whitespaceNewlineAction:
		return func(s *parserState) bool {
			if atEnd(s) {
				return true
			}
			acceptWhitespaceOrNewline(s)
			return true
		}

	case newlineAction:
		return func(s *parserState) bool {
			if atEnd(s) {
				return false // eof is not a newline
			}
			return acceptNewline(s)
		}

	case endOfLineAction:
		return func(s *parserState) bool {
			if atEnd(s) {
				return true // eof is eol
			}
			return acceptNewline(s)
		}

	case startOfLineAction:
		return func(s *parserState) bool {
			return s.lineStart == s.offset
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
			if atEnd(s) {
				return false
			}
			_, n := peekRune(s)
			advanceState(s, n)
			return true
		}
	case byteAction:
		return func(s *parserState) bool {
			if atEnd(s) {
				return false
			}
			advanceState(s, 1)
			return true
		}
	case stringAction:
		return func(s *parserState) bool {
			for _, v := range a.strings {
				if acceptString(s, v) {
					return true
				}
			}
			return false
		}
	case byteListAction, byteStringAction:
		return func(s *parserState) bool {
			for _, v := range a.bytes {
				if acceptBytes(s, v) {
					return true
				}
			}
			return false
		}
	case matchStringAction:
		rules := make(map[string]parseFunc, len(a.stringSwitch))
		size := 0
		for i, r := range a.stringSwitch {
			rules[i] = buildAction(c, r)
			if len(i) > size {
				size = len(i)
			}
		}
		return func(s *parserState) bool {
			if atEnd(s) {
				return false
			}
			r := peekString(s, size)

			if fn, ok := rules[r]; ok {
				return fn(s)
			}

			return false
		}
	case matchRuneAction:
		rules := make(map[rune]parseFunc, len(a.runeSwitch))
		for i, r := range a.runeSwitch {
			rules[i] = buildAction(c, r)
		}
		return func(s *parserState) bool {
			if atEnd(s) {
				return false
			}
			r, _ := peekRune(s)

			if fn, ok := rules[r]; ok {
				return fn(s)
			}

			return false
		}
	case matchByteAction:
		rules := make(map[byte]parseFunc, len(a.byteSwitch))
		for i, r := range a.byteSwitch {
			rules[i] = buildAction(c, r)
		}
		return func(s *parserState) bool {
			if atEnd(s) {
				return false
			}
			r := peekByte(s)

			if fn, ok := rules[r]; ok {
				return fn(s)
			}

			return false
		}
	case runeExceptAction, runeRangeAction:
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
			if atEnd(s) {
				return false
			}
			r, size := peekRune(s)
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
				advanceState(s, size)
				return true
			}

			return false
		}
	case byteExceptAction, byteRangeAction:
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
			if atEnd(s) {
				return false
			}
			r := peekByte(s)
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
				advanceState(s, 1)
				return true
			}

			return false
		}
	case optionalAction:
		rules := make([]parseFunc, len(a.args))
		for i, r := range a.args {
			rules[i] = buildAction(c, r)
		}
		return func(s *parserState) bool {
			var s1 parserState
			copyState(s, &s1)
			for _, r := range rules {
				if !r(&s1) {
					return true
				}
			}
			mergeState(s, &s1)
			return true
		}
	case lookaheadAction:
		rules := make([]parseFunc, len(a.args))
		for i, r := range a.args {
			rules[i] = buildAction(c, r)
		}
		return func(s *parserState) bool {
			var s1 parserState
			copyState(s, &s1)
			for _, r := range rules {
				if !r(&s1) {
					return false
				}
			}
			return true
		}
	case rejectAction:
		rules := make([]parseFunc, len(a.args))
		for i, r := range a.args {
			rules[i] = buildAction(c, r)
		}
		return func(s *parserState) bool {
			var s1 parserState
			copyState(s, &s1)
			for _, r := range rules {
				if !r(&s1) {
					return true
				}
			}
			return false
		}

	case repeatAction:
		rules := make([]parseFunc, len(a.args))
		for i, r := range a.args {
			rules[i] = buildAction(c, r)
		}
		min_n := a.min
		max_n := a.max

		return func(s *parserState) bool {
			c := 0
			var s1 parserState
			copyState(s, &s1)
			for {
				start := s1.offset

				for _, r := range rules {
					if !r(&s1) {
						return c >= min_n
					}
				}

				if s1.offset == start {
					// reject zero width matches
					break
				}

				c++

				if c >= min_n {
					mergeState(s, &s1)
				}

				if max_n != 0 && c >= max_n {
					break
				}
			}

			return c >= min_n
		}

	case cutAction:
		return func(s *parserState) bool {
			s.i.choiceExit = true
			return true
		}
	case choiceAction:
		rules := make([]parseFunc, len(a.args))
		for i, r := range a.args {
			rules[i] = buildAction(c, r)
		}
		return func(s *parserState) bool {
			oldExit := s.i.choiceExit
			oldCorner := s.i.corner
			for _, r := range rules {
				var s1 parserState
				copyState(s, &s1)
				s.i.corner = oldCorner
				s1.i.choiceExit = false
				if r(&s1) {
					mergeState(s, &s1)
					s.i.choiceExit = oldExit
					return true
				}
				trimState(s, &s1)
				if s1.i.choiceExit {
					break
				}
			}
			s.i.corner = oldCorner
			s.i.choiceExit = oldExit
			return false
		}

	case doAction, caseAction, sequenceAction:
		rules := make([]parseFunc, len(a.args))
		for i, r := range a.args {
			rules[i] = buildAction(c, r)
		}

		if len(rules) == 0 {
			return func(s *parserState) bool { return true }
		}

		if len(rules) == 1 {
			return rules[0]
		}

		return func(s *parserState) bool {
			var s1 parserState
			copyState(s, &s1)
			for _, r := range rules {
				if !r(&s1) {
					return false
				}
			}
			mergeState(s, &s1)
			return true
		}
	case captureAction:
		rules := make([]parseFunc, len(a.args))
		for i, r := range a.args {
			rules[i] = buildAction(c, r)
		}
		return func(s *parserState) bool {
			var s1 parserState
			startCapture(s, &s1)
			for _, r := range rules {
				if !r(&s1) {
					return false
				}
			}
			mergeCapture(s, a.name, &s1)
			return true
		}
	default:
		return func(s *parserState) bool {
			return true
		}
	}
}

type Parser struct {
	rules    []parseFunc
	config   *grammarConfig
	builders map[string]any
	err      error
}

func (p *Parser) Err() error {
	return p.err
}

func (p *Parser) newParserState(s string) *parserState {
	i := &parserInput{
		rules:   p.rules,
		buf:     s,
		length:  len(s),
		tabstop: p.config.tabstop,
		nodes:   make([]Node, 128),
		trace:   false,
		starts:  make(map[int]int, len(p.rules)),
		inside:  make(map[int]int, len(p.rules)),
	}
	return &parserState{i: i}
}

func (p *Parser) ParseTree(s string) (*ParseTree, error) {
	if p.err != nil {
		return nil, p.err
	}
	state := p.newParserState(s)
	rule := p.rules[p.config.startIdx]

	complete := rule(state) && atEnd(state)
	if complete {
		n := state.finalNode(p.config.start)
		return &ParseTree{root: n, buf: s, nodes: state.i.nodes}, nil
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
	rule := p.rules[p.config.startIdx]
	return p.testParseFunc(rule, accept, reject)
}

func (p *Parser) testRule(name string, accept []string, reject []string) bool {
	if p.err != nil {
		return false
	}
	i, ok := p.config.index[name]
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
		state := p.newParserState(s)
		complete := rule(state) && atEnd(state)

		if !complete {
			return false
		}
	}
	for _, s := range reject {
		state := p.newParserState(s)
		complete := rule(state) && atEnd(state)

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

func (t *ParseTree) Build(builders map[string]any) (any, error) {
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
		fn := builders[n.name]
		switch v := fn.(type) {
		case func(string, []any) (any, error):
			return v(t.buf[n.start:n.end], args)
		default:
			return nil, errors.New("no builder")
		}
	}
	return build(t.root)
}
