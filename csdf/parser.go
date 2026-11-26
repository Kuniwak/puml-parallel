package csdf

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/Kuniwak/puml-parallel/v6/csdf/clause"
	"github.com/Kuniwak/puml-parallel/v6/csdf/ident"
	"github.com/Kuniwak/puml-parallel/v6/csdf/types"
)

const (
	ignoreBeginMarker = "CSDF-IGNORE-BEGIN"
	ignoreEndMarker   = "CSDF-IGNORE-END"
	observeMarker     = "CSDF-OBSERVE"
)

// sourceParser reads the concrete syntax tree of a diagram.
type sourceParser struct {
	input string
	pos   int
	line  int
	col   int
	// sink is the body the comments, CSDF-IGNORE regions and CSDF-OBSERVE
	// lines read go to: the body being read.
	sink *[]Element
}

// newSourceParser reads the concrete syntax tree of a diagram; its clauses are
// read as written, for the reader of their meaning (coreio's,
// csdf/clause).
func newSourceParser(input string) *sourceParser {
	return &sourceParser{
		input: input,
		pos:   0,
		line:  1,
		col:   1,
		sink:  &[]Element{},
	}
}

func (p *sourceParser) here() Pos { return Pos{Line: p.line, Col: p.col} }

// comparePos orders positions as the source does.
func comparePos(a, b Pos) int {
	if a.Line != b.Line {
		return a.Line - b.Line
	}
	return a.Col - b.Col
}

func (p *sourceParser) parse() (*SyntaxTree, error) {
	tree := &SyntaxTree{Pos: p.here()}
	p.sink = &tree.Body

	if !p.expectString("@startuml") {
		return nil, fmt.Errorf("csdf.Parser.Parse: expected @startuml at line %d, col %d", p.line, p.col)
	}

	if err := p.skipInlineTrivia(); err != nil {
		return nil, fmt.Errorf("csdf.Parser.Parse: %w", err)
	}
	// The diagram name carries no meaning, but the rest of the line is kept
	// verbatim so that printing the diagram round-trips it.
	nameStart := p.pos
	for !p.isAtEnd() && p.peek() != '\n' {
		p.advance()
	}
	tree.Name = strings.TrimSpace(p.input[nameStart:p.pos])
	if !p.expectNewlines() {
		return nil, fmt.Errorf("csdf.Parser.Parse: expected newline after @startuml at line %d, col %d", p.line, p.col)
	}

	if err := p.parseBody(&tree.Body, nil); err != nil {
		return nil, fmt.Errorf("csdf.Parser.Parse: %w", err)
	}

	if !p.expectString("@enduml") {
		return nil, fmt.Errorf("csdf.Parser.Parse: expected @enduml at line %d, col %d", p.line, p.col)
	}
	return tree, nil
}

// parseBody reads elements into body up to @enduml, or, for the block of
// the state opened (nil at the top), up to the "}" that closes it, which it
// consumes.
func (p *sourceParser) parseBody(body *[]Element, opened *StateSyntax) error {
	outer := p.sink
	p.sink = body
	defer func() {
		p.sink = outer
		// A comment inside a line is read before the element of the line.
		slices.SortStableFunc(*body, func(a, b Element) int { return comparePos(a.Position(), b.Position()) })
	}()
	for {
		if err := p.skipTrivia(); err != nil {
			return err
		}
		if p.isAtEnd() {
			if opened != nil {
				return fmt.Errorf("the block of the state %s at line %d, col %d is not closed", opened.ID, opened.Line, opened.Col)
			}
			return nil
		}
		if opened == nil && p.peekString("@enduml") {
			return nil
		}
		if opened != nil && p.peek() == '}' {
			p.advance()
			if err := p.skipInlineTrivia(); err != nil {
				return err
			}
			if !p.isAtEnd() && !p.expectNewlines() {
				return fmt.Errorf("expected newline after '}' at line %d, col %d", p.line, p.col)
			}
			return nil
		}
		e, err := p.parseElement()
		if err != nil {
			return err
		}
		*body = append(*body, e)
	}
}

func (p *sourceParser) parseElement() (Element, error) {
	switch {
	case p.peek() == '!':
		return p.parsePragma()
	case p.peekString("state"):
		return p.parseState()
	case p.peekString("note"):
		return p.parseNote()
	case p.peekString("[*]"):
		return p.parseStartEdge()
	}
	isEdge, err := p.isEdge()
	if err != nil {
		return nil, err
	}
	if isEdge {
		isEndEdge, err := p.isEndEdge()
		if err != nil {
			return nil, err
		}
		if isEndEdge {
			return p.parseEndEdge()
		}
		return p.parseEdge()
	}
	isStateVar, err := p.isStateVar()
	if err != nil {
		return nil, err
	}
	if isStateVar {
		return p.parseStateVar()
	}
	return nil, fmt.Errorf("unexpected syntax at line %d, col %d", p.line, p.col)
}

// parsePragma reads a line of PlantUML's preprocessor, "!name arg".
func (p *sourceParser) parsePragma() (*PragmaSyntax, error) {
	at := p.here()
	p.advance() // !
	start := p.pos
	for !p.isAtEnd() && p.peek() != ' ' && p.peek() != '\t' && p.peek() != '\n' && p.peek() != '\r' {
		p.advance()
	}
	name := p.input[start:p.pos]
	if name == "" {
		return nil, fmt.Errorf("expected a preprocessor directive after '!' at line %d, col %d", at.Line, at.Col)
	}
	start = p.pos
	for !p.isAtEnd() && p.peek() != '\n' {
		p.advance()
	}
	arg := strings.TrimSpace(p.input[start:p.pos])
	p.expectNewlines()
	return &PragmaSyntax{Pos: at, Name: name, Arg: arg}, nil
}

func (p *sourceParser) parseState() (*StateSyntax, error) {
	at := p.here()
	if !p.expectString("state") {
		return nil, fmt.Errorf("csdf.Parser.parseState: expected 'state' at line %d, col %d", p.line, p.col)
	}
	if err := p.skipInlineTrivia(); err != nil {
		return nil, fmt.Errorf("csdf.Parser.parseState: %w", err)
	}

	label, err := p.parseStateName()
	if err != nil {
		return nil, fmt.Errorf("csdf.Parser.parseState: %w", err)
	}
	if err := p.skipInlineTrivia(); err != nil {
		return nil, fmt.Errorf("csdf.Parser.parseState: %w", err)
	}

	if !p.expectString("as") {
		return nil, fmt.Errorf("csdf.Parser.parseState: expected 'as' at line %d, col %d", p.line, p.col)
	}
	if err := p.skipInlineTrivia(); err != nil {
		return nil, fmt.Errorf("csdf.Parser.parseState: %w", err)
	}

	id, err := p.parseName()
	if err != nil {
		return nil, fmt.Errorf("csdf.Parser.parseState: %w", err)
	}
	state := &StateSyntax{Pos: at, ID: id, Label: label}

	if err := p.skipInlineTrivia(); err != nil {
		return nil, fmt.Errorf("csdf.Parser.parseState: %w", err)
	}
	if p.expectString("<<") {
		start := p.pos
		for !p.isAtEnd() && p.peek() != '\n' && !p.peekString(">>") {
			p.advance()
		}
		state.Stereotype = strings.TrimSpace(p.input[start:p.pos])
		if !p.expectString(">>") || state.Stereotype == "" {
			return nil, fmt.Errorf("csdf.Parser.parseState: expected '<<stereotype>>' at line %d, col %d", p.line, p.col)
		}
		if err := p.skipInlineTrivia(); err != nil {
			return nil, fmt.Errorf("csdf.Parser.parseState: %w", err)
		}
	}
	if p.expectChar('{') {
		state.Composite = true
		if err := p.skipInlineTrivia(); err != nil {
			return nil, fmt.Errorf("csdf.Parser.parseState: %w", err)
		}
		if !p.expectNewlines() {
			return nil, fmt.Errorf("csdf.Parser.parseState: expected newline after '{' at line %d, col %d", p.line, p.col)
		}
		if err := p.parseBody(&state.Body, state); err != nil {
			return nil, fmt.Errorf("csdf.Parser.parseState: %w", err)
		}
		return state, nil
	}
	if !p.expectNewlines() {
		return nil, fmt.Errorf("csdf.Parser.parseState: expected newline after state declaration at line %d, col %d", p.line, p.col)
	}
	return state, nil
}

// parseStateVar reads "<state> : <name>" or "<state> : <name> ; <type>".
func (p *sourceParser) parseStateVar() (*StateVarSyntax, error) {
	at := p.here()
	state, err := p.parseID()
	if err != nil {
		return nil, fmt.Errorf("csdf.Parser.parseStateVar: %w", err)
	}
	if err := p.skipInlineTrivia(); err != nil {
		return nil, fmt.Errorf("csdf.Parser.parseStateVar: %w", err)
	}
	if !p.expectChar(':') {
		return nil, fmt.Errorf("csdf.Parser.parseStateVar: expected ':' after state ID in variable declaration at line %d, col %d", p.line, p.col)
	}
	if err := p.skipInlineTrivia(); err != nil {
		return nil, fmt.Errorf("csdf.Parser.parseStateVar: %w", err)
	}
	name, err := p.parseName()
	if err != nil {
		return nil, fmt.Errorf("csdf.Parser.parseStateVar: %w", err)
	}
	if err := p.skipInlineTrivia(); err != nil {
		return nil, fmt.Errorf("csdf.Parser.parseStateVar: %w", err)
	}

	var typ string
	if p.expectChar(';') {
		if err := p.skipInlineTrivia(); err != nil {
			return nil, fmt.Errorf("csdf.Parser.parseStateVar: %w", err)
		}
		typ, err = p.parseUntilSemicolon()
		if err != nil {
			return nil, fmt.Errorf("csdf.Parser.parseStateVar: %w", err)
		}
		if p.peek() == ';' {
			return nil, fmt.Errorf("csdf.Parser.parseStateVar: unexpected ';' in variable type at line %d, col %d", p.line, p.col)
		}
	}
	if !p.expectNewlines() {
		return nil, fmt.Errorf("csdf.Parser.parseStateVar: expected newline after variable declaration at line %d, col %d", p.line, p.col)
	}
	return &StateVarSyntax{Pos: at, State: state, Name: name, Type: typ}, nil
}

func (p *sourceParser) parseStateName() (string, error) {
	if !p.expectChar('"') {
		return "", fmt.Errorf("csdf.Parser.parseStateName: expected '\"' at line %d, col %d", p.line, p.col)
	}

	var result strings.Builder
	for !p.isAtEnd() && p.peek() != '"' {
		if p.peek() == '\\' {
			p.advance()
			if p.isAtEnd() {
				return "", fmt.Errorf("csdf.Parser.parseStateName: unexpected end of input in string at line %d, col %d", p.line, p.col)
			}
			switch p.peek() {
			case '\\':
				result.WriteByte('\\')
			case '"':
				result.WriteByte('"')
			default:
				result.WriteByte('\\')
				result.WriteByte(p.peek())
			}
		} else {
			result.WriteByte(p.peek())
		}
		p.advance()
	}

	if !p.expectChar('"') {
		return "", fmt.Errorf("csdf.Parser.parseStateName: expected closing '\"' at line %d, col %d", p.line, p.col)
	}

	return result.String(), nil
}

func (p *sourceParser) parseStartEdge() (*StartEdgeSyntax, error) {
	at := p.here()
	if !p.expectString("[*]") {
		return nil, fmt.Errorf("csdf.Parser.parseStartEdge: expected '[*]' at line %d, col %d", p.line, p.col)
	}
	if err := p.skipInlineTrivia(); err != nil {
		return nil, fmt.Errorf("csdf.Parser.parseStartEdge: %w", err)
	}

	if !p.expectString("-->") {
		return nil, fmt.Errorf("csdf.Parser.parseStartEdge: expected '-->' at line %d, col %d", p.line, p.col)
	}
	if err := p.skipInlineTrivia(); err != nil {
		return nil, fmt.Errorf("csdf.Parser.parseStartEdge: %w", err)
	}

	dst, err := p.parseName()
	if err != nil {
		return nil, fmt.Errorf("csdf.Parser.parseStartEdge: expected destination state ID after '-->' in start edge at line %d, col %d", p.line, p.col)
	}
	if err := p.skipInlineTrivia(); err != nil {
		return nil, fmt.Errorf("csdf.Parser.parseStartEdge: %w", err)
	}

	edge := &StartEdgeSyntax{Pos: at, Dst: dst}
	if p.peek() == ':' {
		p.advance() // consume ':'
		if err := p.skipInlineTrivia(); err != nil {
			return nil, fmt.Errorf("csdf.Parser.parseStartEdge: %w", err)
		}
		edge.Post, err = p.parseClause(at.Line, "post", p.parseUntilNewline)
		if err != nil {
			return nil, fmt.Errorf("csdf.Parser.parseStartEdge: %w", err)
		}
	}

	if !p.expectNewlines() {
		return nil, fmt.Errorf("csdf.Parser.parseStartEdge: expected newline after start edge declaration at line %d, col %d", p.line, p.col)
	}
	return edge, nil
}

func (p *sourceParser) parseEdge() (*EdgeSyntax, error) {
	at := p.here()
	src, err := p.parseName()
	if err != nil {
		return nil, fmt.Errorf("csdf.Parser.parseEdge: %w", err)
	}
	if err := p.skipInlineTrivia(); err != nil {
		return nil, fmt.Errorf("csdf.Parser.parseEdge: %w", err)
	}

	if !p.expectString("-->") {
		return nil, fmt.Errorf("csdf.Parser.parseEdge: expected '-->' at line %d, col %d", p.line, p.col)
	}
	if err := p.skipInlineTrivia(); err != nil {
		return nil, fmt.Errorf("csdf.Parser.parseEdge: %w", err)
	}

	dst, err := p.parseName()
	if err != nil {
		return nil, fmt.Errorf("csdf.Parser.parseEdge: %w", err)
	}
	if err := p.skipInlineTrivia(); err != nil {
		return nil, fmt.Errorf("csdf.Parser.parseEdge: %w", err)
	}

	if !p.expectChar(':') {
		return nil, fmt.Errorf("csdf.Parser.parseEdge: expected ':' at line %d, col %d", p.line, p.col)
	}
	if err := p.skipInlineTrivia(); err != nil {
		return nil, fmt.Errorf("csdf.Parser.parseEdge: %w", err)
	}

	event, err := p.parseEvent()
	if err != nil {
		return nil, fmt.Errorf("csdf.Parser.parseEdge: %w", err)
	}
	if err := p.skipInlineTrivia(); err != nil {
		return nil, fmt.Errorf("csdf.Parser.parseEdge: %w", err)
	}

	edge := &EdgeSyntax{Pos: at, Src: src, Dst: dst, Event: event}
	if p.peek() == ';' {
		p.advance() // consume first ';'
		if err := p.skipInlineTrivia(); err != nil {
			return nil, fmt.Errorf("csdf.Parser.parseEdge: %w", err)
		}
		edge.Guard, err = p.parseClause(at.Line, "guard", p.parseUntilSemicolon)
		if err != nil {
			return nil, fmt.Errorf("csdf.Parser.parseEdge: %w", err)
		}
		if err := p.skipInlineTrivia(); err != nil {
			return nil, fmt.Errorf("csdf.Parser.parseEdge: %w", err)
		}

		if p.peek() == ';' {
			p.advance() // consume second ';'
			if err := p.skipInlineTrivia(); err != nil {
				return nil, fmt.Errorf("csdf.Parser.parseEdge: %w", err)
			}
			edge.Post, err = p.parseClause(at.Line, "post", p.parseUntilNewline)
			if err != nil {
				return nil, fmt.Errorf("csdf.Parser.parseEdge: %w", err)
			}
		}
	}

	if !p.expectNewlines() {
		return nil, fmt.Errorf("csdf.Parser.parseEdge: expected newline after edge declaration at line %d, col %d", p.line, p.col)
	}
	return edge, nil
}

// parseClause reads a clause by read, written on line as the which of that
// line, into its syntax tree.
func (p *sourceParser) parseClause(line int, which string, read func() (string, error)) (*ClauseSyntax, error) {
	at := p.here()
	text, err := read()
	if err != nil {
		return nil, err
	}
	return clauseAt(at, line, which, text)
}

// clauseAt reads text, a clause written at pos on line as the which of that
// line, into its syntax tree. A clause the clause grammar refuses is tagged
// with that line, not with where the parser stopped, which is past it; the
// column is 0, since the clause's own position is not the line's.
func clauseAt(pos Pos, line int, which, text string) (*ClauseSyntax, error) {
	s, err := clause.ParseSyntax(text)
	if err != nil {
		return nil, &ParseError{Line: line, err: fmt.Errorf("line %d, the %s: %w", line, which, err)}
	}
	return &ClauseSyntax{Pos: pos, Text: text, Syntax: s}, nil
}

func (p *sourceParser) parseEvent() (EventSyntax, error) {
	at := p.here()
	text, err := p.parseUntilSemicolon()
	if err != nil {
		return EventSyntax{}, fmt.Errorf("csdf.Parser.parseEvent: %w", err)
	}
	if text == "" {
		return EventSyntax{}, fmt.Errorf("csdf.Parser.parseEvent: expected event after ':' in edge at line %d, col %d", p.line, p.col)
	}
	e, err := eventSyntax(at, text)
	if err != nil {
		return EventSyntax{}, fmt.Errorf("csdf.Parser.parseEvent: line %d: %w", p.line, err)
	}
	return e, nil
}

// eventSyntax reads text, an event written at pos.
func eventSyntax(pos Pos, text string) (EventSyntax, error) {
	e, err := ParseEvent(text)
	if err != nil {
		return EventSyntax{}, err
	}
	out := EventSyntax{Pos: pos, Text: text, Channel: e.Channel()}
	for _, f := range e.Fields() {
		out.Fields = append(out.Fields, FieldSyntax{Dir: string(f.Dir), Name: f.Name})
	}
	return out, nil
}

func (p *sourceParser) parseID() (string, error) {
	var result strings.Builder

	if p.isAtEnd() || !ident.IsRune(p.peekRune()) {
		return "", fmt.Errorf("csdf.Parser.parseID: expected identifier at line %d, col %d", p.line, p.col)
	}

	for !p.isAtEnd() {
		r, size := p.peekRuneSize()
		if !ident.IsRune(r) {
			break
		}
		result.WriteRune(r)
		for i := 0; i < size; i++ {
			p.advance()
		}
	}

	return result.String(), nil
}

// parseName reads an identifier that names something, refusing one that
// ident.CheckName refuses. The probes read with parseID, which only reads.
func (p *sourceParser) parseName() (string, error) {
	line, col := p.line, p.col
	name, err := p.parseID()
	if err != nil {
		return "", err
	}
	if err := ident.CheckName(name); err != nil {
		return "", fmt.Errorf("csdf.Parser.parseName: line %d, col %d: %w", line, col, err)
	}
	return name, nil
}

func (p *sourceParser) parseUntilSemicolon() (string, error) {
	return p.parseUntil(';', '\n')
}

func (p *sourceParser) parseUntilNewline() (string, error) {
	return p.parseUntil('\n')
}

func (p *sourceParser) parseUntil(stops ...byte) (string, error) {
	var result strings.Builder
	var lastWritten byte

	for !p.isAtEnd() && !containsByte(stops, p.peek()) {
		if p.peekString("/'") {
			needsSeparator := result.Len() > 0 && lastWritten != ' ' && lastWritten != '\t' && lastWritten != '\r' && lastWritten != '\n'
			if err := p.skipBlockComment(); err != nil {
				return "", fmt.Errorf("csdf.Parser.parseUntil: %w", err)
			}
			if needsSeparator && !p.isAtEnd() && !containsByte(stops, p.peek()) &&
				p.peek() != ' ' && p.peek() != '\t' && p.peek() != '\r' && p.peek() != '\n' {
				result.WriteByte(' ')
				lastWritten = ' '
			}
			continue
		}

		c := p.peek()
		result.WriteByte(c)
		p.advance()
		lastWritten = c

		// \; is part of the clause rather than the end of it, and \\ is a
		// backslash that escapes nothing after it (docs/SYNTAX.md). A clause
		// language's quotes are not the tools', so they protect nothing.
		if c == '\\' && !p.isAtEnd() && (p.peek() == ';' || p.peek() == '\\') {
			lastWritten = p.peek()
			result.WriteByte(lastWritten)
			p.advance()
		}
	}
	return strings.TrimSpace(result.String()), nil
}

func (p *sourceParser) peekRune() rune {
	r, _ := p.peekRuneSize()
	return r
}

func (p *sourceParser) peekRuneSize() (rune, int) {
	if p.isAtEnd() {
		return 0, 0
	}
	return utf8.DecodeRuneInString(p.input[p.pos:])
}

// probe is a copy of p that reads ahead without moving p or adding to
// what p has read.
func (p *sourceParser) probe() *sourceParser {
	probe := *p
	probe.sink = &[]Element{}
	return &probe
}

func (p *sourceParser) isEdge() (bool, error) {
	probe := p.probe()
	_, err := probe.parseID()
	if err != nil {
		return false, nil
	}
	if err := probe.skipInlineTrivia(); err != nil {
		return false, fmt.Errorf("csdf.Parser.isEdge: %w", err)
	}
	return probe.peekString("-->"), nil
}

func (p *sourceParser) isEndEdge() (bool, error) {
	probe := p.probe()
	_, err := probe.parseID()
	if err != nil {
		return false, nil
	}
	if err := probe.skipInlineTrivia(); err != nil {
		return false, fmt.Errorf("csdf.Parser.isEndEdge: %w", err)
	}
	if !probe.expectString("-->") {
		return false, nil
	}
	if err := probe.skipInlineTrivia(); err != nil {
		return false, fmt.Errorf("csdf.Parser.isEndEdge: %w", err)
	}
	return probe.peekString("[*]"), nil
}

func (p *sourceParser) isStateVar() (bool, error) {
	probe := p.probe()
	if _, err := probe.parseID(); err != nil {
		return false, nil
	}
	if err := probe.skipInlineTrivia(); err != nil {
		return false, fmt.Errorf("csdf.Parser.isStateVar: %w", err)
	}
	return probe.peek() == ':', nil
}

func (p *sourceParser) peek() byte {
	if p.isAtEnd() {
		return 0
	}
	return p.input[p.pos]
}

// advance moves past one byte; the column counts code points, so a byte
// that continues one does not move it.
func (p *sourceParser) advance() byte {
	if p.isAtEnd() {
		return 0
	}
	c := p.input[p.pos]
	p.pos++
	switch {
	case c == '\n':
		p.line++
		p.col = 1
	case utf8.RuneStart(c):
		p.col++
	}
	return c
}

func (p *sourceParser) isAtEnd() bool {
	return p.pos >= len(p.input)
}

func (p *sourceParser) expectChar(expected byte) bool {
	if p.isAtEnd() || p.peek() != expected {
		return false
	}
	p.advance()
	return true
}

func (p *sourceParser) expectString(expected string) bool {
	if p.pos+len(expected) > len(p.input) {
		return false
	}
	if p.input[p.pos:p.pos+len(expected)] != expected {
		return false
	}
	for i := 0; i < len(expected); i++ {
		p.advance()
	}
	return true
}

func (p *sourceParser) peekString(expected string) bool {
	if p.pos+len(expected) > len(p.input) {
		return false
	}
	return p.input[p.pos:p.pos+len(expected)] == expected
}

// skipTrivia moves past whitespace, comments, CSDF-IGNORE regions and
// CSDF-OBSERVE lines, adding all but whitespace to the body being read.
func (p *sourceParser) skipTrivia() error {
	for {
		for !p.isAtEnd() && (p.peek() == ' ' || p.peek() == '\t' || p.peek() == '\n' || p.peek() == '\r') {
			p.advance()
		}
		if p.peekString("/'") {
			if err := p.skipBlockComment(); err != nil {
				return fmt.Errorf("csdf.Parser.skipTrivia: %w", err)
			}
			continue
		}
		if p.peek() == '\'' {
			if p.lineCommentBody() == ignoreBeginMarker {
				if err := p.skipIgnoreRegion(); err != nil {
					return fmt.Errorf("csdf.Parser.skipTrivia: %w", err)
				}
				continue
			}
			if body, ok := p.observeBody(); ok {
				if err := p.readObserve(body); err != nil {
					return fmt.Errorf("csdf.Parser.skipTrivia: %w", err)
				}
				continue
			}
			p.readLineComment()
			continue
		}
		return nil
	}
}

// readLineComment reads the line comment at the current position.
func (p *sourceParser) readLineComment() {
	at := p.here()
	end := p.pos + 1
	for end < len(p.input) && p.input[end] != '\n' {
		end++
	}
	*p.sink = append(*p.sink, &CommentSyntax{Pos: at, Text: strings.TrimRight(p.input[p.pos+1:end], " \t\r")})
	p.skipLine()
}

// lineCommentBody returns the trimmed text of the line comment at the current
// position (the leading "'" excluded). The Parser is not advanced.
func (p *sourceParser) lineCommentBody() string {
	end := p.pos + 1
	for end < len(p.input) && p.input[end] != '\n' {
		end++
	}
	return strings.TrimSpace(p.input[p.pos+1 : end])
}

// observeBody is the text after the observe marker of the line comment at
// the current position, the marker being "CSDF-OBSERVE" followed by a tab or
// a space; ok is false for a line comment the marker does not start, which
// is a plain one. The Parser is not advanced.
func (p *sourceParser) observeBody() (body string, ok bool) {
	end := p.pos + 1
	for end < len(p.input) && p.input[end] != '\n' {
		end++
	}
	rest := strings.TrimLeft(p.input[p.pos+1:end], " \t")
	rest, ok = strings.CutPrefix(rest, observeMarker)
	if !ok || rest == "" || (rest[0] != ' ' && rest[0] != '\t') {
		return "", false
	}
	return rest, true
}

// readObserve reads body, the rest of an observe line at the current
// position, as a view or a variable, and consumes the line.
func (p *sourceParser) readObserve(body string) error {
	at := p.here()
	o, err := parseObserve(strings.TrimRight(body, " \t\r"), at)
	if err != nil {
		return err
	}
	*p.sink = append(*p.sink, o)
	p.skipLine()
	return nil
}

// parseObserve reads the body of an observe line, written at pos:
// "<view> = <state>, …", "<view> =" for a view of no state, or
// "<variable>", separated by tabs and spaces only.
func parseObserve(body string, pos Pos) (*ObserveSyntax, error) {
	name, states, isView := strings.Cut(body, "=")
	name = strings.Trim(name, " \t")
	if err := ident.Check(name); err != nil {
		return nil, fmt.Errorf("the CSDF-OBSERVE line at line %d: expected \"<view> = <state>, …\" or \"<variable>\": %w", pos.Line, err)
	}
	o := &ObserveSyntax{Pos: pos, Name: name, View: isView}
	if !isView {
		return o, nil
	}
	o.States = []string{}
	if strings.Trim(states, " \t") == "" {
		return o, nil
	}
	for _, s := range strings.Split(states, ",") {
		s = strings.Trim(s, " \t")
		if err := ident.Check(s); err != nil {
			return nil, fmt.Errorf("the CSDF-OBSERVE line at line %d: the view %s: expected a state: %w", pos.Line, name, err)
		}
		o.States = append(o.States, s)
	}
	return o, nil
}

// skipIgnoreRegion reads the lines from the "' CSDF-IGNORE-BEGIN" marker
// (current position) through the matching "' CSDF-IGNORE-END" marker,
// inclusive, keeping the lines between them.
func (p *sourceParser) skipIgnoreRegion() error {
	at := p.here()
	region := &IgnoreSyntax{Pos: at, Lines: []string{}}
	p.skipLine() // consume the begin-marker line
	for !p.isAtEnd() {
		start := p.pos
		for !p.isAtEnd() && (p.peek() == ' ' || p.peek() == '\t') {
			p.advance()
		}
		if p.peek() == '\'' && p.lineCommentBody() == ignoreEndMarker {
			p.skipLine() // consume the end-marker line
			*p.sink = append(*p.sink, region)
			return nil
		}
		p.skipLine()
		region.Lines = append(region.Lines, strings.TrimRight(p.input[start:p.pos], "\r\n"))
	}
	return fmt.Errorf("csdf.Parser.skipIgnoreRegion: unterminated CSDF-IGNORE region at line %d, col %d", at.Line, at.Col)
}

func (p *sourceParser) skipInlineTrivia() error {
	for {
		for !p.isAtEnd() && (p.peek() == ' ' || p.peek() == '\t') {
			p.advance()
		}
		if !p.peekString("/'") {
			return nil
		}
		if err := p.skipBlockComment(); err != nil {
			return fmt.Errorf("csdf.Parser.skipInlineTrivia: %w", err)
		}
	}
}

// skipBlockComment reads the block comment at the current position.
func (p *sourceParser) skipBlockComment() error {
	at := p.here()
	p.expectString("/'")
	start := p.pos
	for !p.isAtEnd() && !p.peekString("'/") {
		p.advance()
	}
	text := p.input[start:p.pos]
	if !p.expectString("'/") {
		return fmt.Errorf("csdf.Parser.skipBlockComment: unterminated block comment at line %d, col %d", at.Line, at.Col)
	}
	*p.sink = append(*p.sink, &CommentSyntax{Pos: at, Text: text, Block: true})
	return nil
}

func (p *sourceParser) expectNewlines() bool {
	count := 0
	for !p.isAtEnd() && (p.peek() == '\n' || p.peek() == '\r') {
		if p.peek() == '\n' {
			count++
		}
		p.advance()
	}
	return count > 0
}

func (p *sourceParser) skipLine() {
	for !p.isAtEnd() && p.peek() != '\n' {
		p.advance()
	}
	if !p.isAtEnd() {
		p.advance()
	}
}

// noteSides are the sides a note may be anchored on.
var noteSides = []string{"left", "right", "top", "bottom"}

// parseNote reads a note, floating ("note as <id>") or anchored ("note
// <side> of <state>"), and its lines up to "end note" (docs/SYNTAX.md,
// docs/PROMOTION.md).
func (p *sourceParser) parseNote() (*NoteSyntax, error) {
	at := p.here()
	p.expectString("note")
	if err := p.skipInlineTrivia(); err != nil {
		return nil, fmt.Errorf("csdf.Parser.parseNote: %w", err)
	}
	note := &NoteSyntax{Pos: at}
	if p.expectString("as") {
		if err := p.skipInlineTrivia(); err != nil {
			return nil, fmt.Errorf("csdf.Parser.parseNote: %w", err)
		}
		id, err := p.parseName()
		if err != nil {
			return nil, fmt.Errorf("csdf.Parser.parseNote: %w", err)
		}
		note.ID = id
	} else {
		for _, side := range noteSides {
			if p.expectString(side) {
				note.Side = side
				break
			}
		}
		if note.Side == "" {
			return nil, fmt.Errorf("csdf.Parser.parseNote: expected \"as <id>\" or \"<left|right|top|bottom> of <state>\" after note at line %d, col %d", p.line, p.col)
		}
		if err := p.skipInlineTrivia(); err != nil {
			return nil, fmt.Errorf("csdf.Parser.parseNote: %w", err)
		}
		if !p.expectString("of") {
			return nil, fmt.Errorf("csdf.Parser.parseNote: expected 'of' at line %d, col %d", p.line, p.col)
		}
		if err := p.skipInlineTrivia(); err != nil {
			return nil, fmt.Errorf("csdf.Parser.parseNote: %w", err)
		}
		of, err := p.parseName()
		if err != nil {
			return nil, fmt.Errorf("csdf.Parser.parseNote: %w", err)
		}
		note.Of = of
	}
	if err := p.skipInlineTrivia(); err != nil {
		return nil, fmt.Errorf("csdf.Parser.parseNote: %w", err)
	}
	if !p.expectNewlines() {
		return nil, fmt.Errorf("csdf.Parser.parseNote: expected newline after the note's head at line %d, col %d", p.line, p.col)
	}

	directed, opened := false, false
	outer := p.sink
	p.sink = &note.Lines
	defer func() { p.sink = outer }()
	for {
		for !p.isAtEnd() && (p.peek() == ' ' || p.peek() == '\t') {
			p.advance()
		}
		if p.isAtEnd() {
			return nil, fmt.Errorf("csdf.Parser.parseNote: unterminated note at line %d, col %d", at.Line, at.Col)
		}
		lineAt := p.here()
		if p.peek() == '\'' {
			p.readLineComment()
			continue
		}
		text, err := p.parseUntil('\n')
		if err != nil {
			return nil, fmt.Errorf("csdf.Parser.parseNote: %w", err)
		}
		p.expectNewlines()
		switch {
		case text == "":
			continue
		case strings.HasPrefix(text, "end") && strings.TrimSpace(strings.TrimPrefix(text, "end")) == "note":
			return note, nil
		}
		if !opened {
			directed, opened = opensDirective(text), true
		}
		line, err := noteLine(lineAt, text, directed)
		if err != nil {
			return nil, fmt.Errorf("csdf.Parser.parseNote: %w", err)
		}
		note.Lines = append(note.Lines, line)
	}
}

// opensDirective reports whether a note line opens a directive.
func opensDirective(text string) bool {
	for _, w := range []string{"assume", "channel", "sync", "constrain"} {
		if text == w || strings.HasPrefix(text, w+" ") {
			return true
		}
	}
	return false
}

var (
	syncRe       = regexp.MustCompile(`^sync\s+(` + ident.Pattern + `)\s*:\s*([^;]*?)\s*(?:;\s*(.*))?$`)
	syncTargetRe = regexp.MustCompile(`^(` + ident.Pattern + `)\((` + ident.Pattern + `)\)(?:\.(` + ident.Pattern + `))?$`)
)

// noteLine reads text, a line of a note written at pos: a directive when
// the note is directed, prose otherwise.
func noteLine(pos Pos, text string, directed bool) (Element, error) {
	if !directed || !opensDirective(text) {
		return &ProseSyntax{Pos: pos, Text: text}, nil
	}
	word, rest, _ := strings.Cut(text, " ")
	rest = strings.TrimSpace(rest)
	switch word {
	case "assume":
		return &AssumeSyntax{Pos: pos, Text: rest}, nil
	case "channel":
		return channelSyntax(pos, rest), nil
	case "sync":
		return syncSyntax(pos, text)
	}
	return constrainSyntax(pos, rest)
}

// channelSyntax reads what follows "channel ", written at pos.
func channelSyntax(pos Pos, text string) *ChannelSyntax {
	name, written, hasTypes := strings.Cut(text, ":")
	c := &ChannelSyntax{Pos: pos, Text: text, Name: strings.TrimSpace(name), Fields: []string{}}
	if !hasTypes {
		return c
	}
	for _, f := range splitFieldTypes(written) {
		c.Fields = append(c.Fields, strings.TrimSpace(f))
	}
	return c
}

// syncSyntax reads a sync directive, "sync <event> : <map>(<id>)[.<event>],
// … [; guard]", written at pos.
func syncSyntax(pos Pos, text string) (*SyncSyntax, error) {
	m := syncRe.FindStringSubmatch(text)
	if m == nil {
		return nil, fmt.Errorf("line %d: %q: expected \"sync <event> : <map>(<id>), … ; <guard>\"", pos.Line, text)
	}
	s := &SyncSyntax{Pos: pos, Event: m[1]}
	for _, t := range strings.Split(m[2], ",") {
		tm := syncTargetRe.FindStringSubmatch(strings.TrimSpace(t))
		if tm == nil {
			return nil, fmt.Errorf("line %d: sync %s: %q: expected map(id) or map(id).event", pos.Line, m[1], strings.TrimSpace(t))
		}
		s.Targets = append(s.Targets, SyncTargetSyntax{Map: tm[1], ID: tm[2], Event: tm[3]})
	}
	if _, guard, ok := strings.Cut(text, ";"); ok {
		g, err := clauseAt(pos, pos.Line, "guard", strings.TrimSpace(guard))
		if err != nil {
			return nil, fmt.Errorf("sync %s: %w", m[1], err)
		}
		s.Guard = g
	}
	return s, nil
}

// constrainSyntax reads what follows "constrain ", "<event> ; <guard>",
// written at pos.
func constrainSyntax(pos Pos, text string) (*ConstrainSyntax, error) {
	head, guard, hasGuard := strings.Cut(text, ";")
	e, err := eventSyntax(pos, strings.TrimSpace(head))
	if err != nil {
		return nil, fmt.Errorf("line %d: constrain %s: %w", pos.Line, strings.TrimSpace(head), err)
	}
	c := &ConstrainSyntax{Pos: pos, Event: e}
	if hasGuard {
		g, err := clauseAt(pos, pos.Line, "guard", strings.TrimSpace(guard))
		if err != nil {
			return nil, fmt.Errorf("constrain %s: %w", strings.TrimSpace(head), err)
		}
		c.Guard = g
	}
	return c, nil
}

// ParseChannelDecl reads what follows "channel " in a channel declaration
// written at line: "<name>" or "<name> : <type>.<type>…", one type per field.
func ParseChannelDecl(text string, line int) (ChannelDecl, error) {
	name, written, hasTypes := strings.Cut(text, ":")
	name = strings.TrimSpace(name)
	if name == "" {
		return ChannelDecl{}, fmt.Errorf("the channel declaration at line %d names no channel", line)
	}
	if err := ident.Check(name); err != nil {
		return ChannelDecl{}, fmt.Errorf("the channel declaration at line %d: expected \"<name> : <type>.<type>\": %w", line, err)
	}
	decl := ChannelDecl{Name: name, Fields: []types.Type{}, Line: line}
	if !hasTypes {
		return decl, nil
	}
	for _, written := range splitFieldTypes(written) {
		t, err := types.Parse(strings.TrimSpace(written))
		if err != nil {
			return ChannelDecl{}, fmt.Errorf("the channel declaration of %s at line %d: %w", name, line, err)
		}
		decl.Fields = append(decl.Fields, t)
	}
	return decl, nil
}

// splitFieldTypes splits text at each . outside parentheses.
func splitFieldTypes(text string) []string {
	var out []string
	depth, start := 0, 0
	for i, r := range text {
		switch r {
		case '(':
			depth++
		case ')':
			depth--
		case '.':
			if depth == 0 {
				out = append(out, text[start:i])
				start = i + 1
			}
		}
	}
	return append(out, text[start:])
}

func (p *sourceParser) parseEndEdge() (*EndEdgeSyntax, error) {
	at := p.here()
	src, err := p.parseName()
	if err != nil {
		return nil, fmt.Errorf("csdf.Parser.parseEndEdge: %w", err)
	}
	if err := p.skipInlineTrivia(); err != nil {
		return nil, fmt.Errorf("csdf.Parser.parseEndEdge: %w", err)
	}

	if !p.expectString("-->") {
		return nil, fmt.Errorf("csdf.Parser.parseEndEdge: expected '-->' at line %d, col %d", p.line, p.col)
	}
	if err := p.skipInlineTrivia(); err != nil {
		return nil, fmt.Errorf("csdf.Parser.parseEndEdge: %w", err)
	}

	if !p.expectString("[*]") {
		return nil, fmt.Errorf("csdf.Parser.parseEndEdge: expected '[*]' at line %d, col %d", p.line, p.col)
	}
	if err := p.skipInlineTrivia(); err != nil {
		return nil, fmt.Errorf("csdf.Parser.parseEndEdge: %w", err)
	}

	edge := &EndEdgeSyntax{Pos: at, Src: src}
	if p.expectChar(':') {
		if err := p.skipInlineTrivia(); err != nil {
			return nil, fmt.Errorf("csdf.Parser.parseEndEdge: %w", err)
		}
		edge.Guard, err = p.parseClause(at.Line, "guard", p.parseUntilSemicolon)
		if err != nil {
			return nil, fmt.Errorf("csdf.Parser.parseEndEdge: %w", err)
		}
		if p.peek() == ';' {
			return nil, fmt.Errorf("csdf.Parser.parseEndEdge: unexpected ';' in end edge guard at line %d, col %d", p.line, p.col)
		}
	}

	if !p.expectNewlines() {
		return nil, fmt.Errorf("csdf.Parser.parseEndEdge: expected newline after end edge declaration at line %d, col %d", p.line, p.col)
	}
	return edge, nil
}

func containsByte(values []byte, target byte) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
