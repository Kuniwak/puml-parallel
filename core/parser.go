package core

import (
	"fmt"
	"strings"
)

type Parser struct {
	input string
	pos   int
	line  int
	col   int
}

func NewParser(input string) *Parser {
	return &Parser{
		input: input,
		pos:   0,
		line:  1,
		col:   1,
	}
}

func (p *Parser) Parse() (*Diagram, error) {
	diagram := &Diagram{
		States:   make(map[StateID]State),
		Edges:    []Edge{},
		EndEdges: []EndEdge{},
	}

	if !p.expectString("@startuml") {
		return nil, fmt.Errorf("expected @startuml at line %d, col %d", p.line, p.col)
	}

	if !p.expectNewlines() {
		return nil, fmt.Errorf("expected newline after @startuml at line %d, col %d", p.line, p.col)
	}

	// Parse states first
	for !p.isAtEnd() && !p.peekString("@enduml") && p.peekString("state") {
		state, err := p.parseState()
		if err != nil {
			return nil, err
		}
		diagram.States[state.ID] = state
		p.skipWhitespace()
	}

	// Skip any additional whitespace before start edge
	p.skipWhitespace()

	// Parse startEdge (required)
	if p.peekString("[*]") {
		startEdge, err := p.parseStartEdge()
		if err != nil {
			return nil, err
		}
		diagram.StartEdge = startEdge
		p.skipWhitespace()
	} else {
		return nil, fmt.Errorf("expected start edge [*] --> state at line %d, col %d", p.line, p.col)
	}

	// Parse edges and endEdges
	for !p.isAtEnd() && !p.peekString("@enduml") {
		if p.isStateID() {
			// Check if it's an end edge (state --> [*])
			if p.isEndEdge() {
				endEdge, err := p.parseEndEdge()
				if err != nil {
					return nil, err
				}
				diagram.EndEdges = append(diagram.EndEdges, endEdge)
			} else {
				// Regular edge (state --> state)
				edge, err := p.parseEdge()
				if err != nil {
					return nil, err
				}
				diagram.Edges = append(diagram.Edges, edge)
			}
		} else {
			p.skipLine()
		}
		p.skipWhitespace()
	}

	if !p.expectString("@enduml") {
		return nil, fmt.Errorf("expected @enduml at line %d, col %d", p.line, p.col)
	}

	return diagram, nil
}

func (p *Parser) parseState() (State, error) {
	if !p.expectString("state") {
		return State{}, fmt.Errorf("expected 'state' at line %d, col %d", p.line, p.col)
	}
	p.skipSpaces()

	name, err := p.parseStateName()
	if err != nil {
		return State{}, err
	}
	p.skipSpaces()

	if !p.expectString("as") {
		return State{}, fmt.Errorf("expected 'as' at line %d, col %d", p.line, p.col)
	}
	p.skipSpaces()

	id, err := p.parseID()
	if err != nil {
		return State{}, err
	}

	state := State{
		ID:   StateID(id),
		Name: name,
		Vars: []Var{},
	}

	if !p.expectNewlines() {
		return State{}, fmt.Errorf("expected newline after state declaration at line %d, col %d", p.line, p.col)
	}

	for !p.isAtEnd() && !p.peekString("@enduml") && !p.peekString("state") && !p.isStateID() {
		if p.peekString(string(state.ID)) {
			// Parse stateID : var
			_, err := p.parseID() // Should match state.ID
			if err != nil {
				return State{}, err
			}
			p.skipSpaces()
			if !p.expectChar(':') {
				return State{}, fmt.Errorf("expected ':' after state ID in variable declaration at line %d, col %d", p.line, p.col)
			}
			p.skipSpaces()
			varName, err := p.parseID()
			if err != nil {
				return State{}, err
			}
			state.Vars = append(state.Vars, Var(varName))
			if !p.expectNewlines() {
				return State{}, fmt.Errorf("expected newline after variable declaration at line %d, col %d", p.line, p.col)
			}
		} else {
			break
		}
	}

	return state, nil
}

func (p *Parser) parseStateName() (string, error) {
	if !p.expectChar('"') {
		return "", fmt.Errorf("expected '\"' at line %d, col %d", p.line, p.col)
	}

	var result strings.Builder
	for !p.isAtEnd() && p.peek() != '"' {
		if p.peek() == '\\' {
			p.advance()
			if p.isAtEnd() {
				return "", fmt.Errorf("unexpected end of input in string at line %d, col %d", p.line, p.col)
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
		return "", fmt.Errorf("expected closing '\"' at line %d, col %d", p.line, p.col)
	}

	return result.String(), nil
}

func (p *Parser) parseStartEdge() (StartEdge, error) {
	if !p.expectString("[*]") {
		return StartEdge{}, fmt.Errorf("expected '[*]' at line %d, col %d", p.line, p.col)
	}
	p.skipSpaces()

	if !p.expectString("-->") {
		return StartEdge{}, fmt.Errorf("expected '-->' at line %d, col %d", p.line, p.col)
	}
	p.skipSpaces()

	dst, err := p.parseID()
	if err != nil {
		return StartEdge{}, err
	}
	p.skipSpaces()

	post := "true" // Default value when post is omitted
	if p.peek() == ':' {
		p.advance() // consume ':'
		p.skipSpaces()
		post = p.parseUntilNewline()
	}

	if !p.expectNewlines() {
		return StartEdge{}, fmt.Errorf("expected newline after start edge declaration at line %d, col %d", p.line, p.col)
	}

	return StartEdge{
		Dst:  StateID(dst),
		Post: post,
	}, nil
}

func (p *Parser) parseEdge() (Edge, error) {
	src, err := p.parseID()
	if err != nil {
		return Edge{}, err
	}
	p.skipSpaces()

	if !p.expectString("-->") {
		return Edge{}, fmt.Errorf("expected '-->' at line %d, col %d", p.line, p.col)
	}
	p.skipSpaces()

	dst, err := p.parseID()
	if err != nil {
		return Edge{}, err
	}
	p.skipSpaces()

	if !p.expectChar(':') {
		return Edge{}, fmt.Errorf("expected ':' at line %d, col %d", p.line, p.col)
	}
	p.skipSpaces()

	event, err := p.parseEvent()
	if err != nil {
		return Edge{}, err
	}
	p.skipSpaces()

	guard := "true" // Default value when guard is omitted
	post := "true"  // Default value when post is omitted

	if p.peek() == ';' {
		p.advance() // consume first ';'
		p.skipSpaces()
		guard = p.parseUntilSemicolon()
		p.skipSpaces()

		if p.peek() == ';' {
			p.advance() // consume second ';'
			p.skipSpaces()
			post = p.parseUntilNewline()
		}
	}

	if !p.expectNewlines() {
		return Edge{}, fmt.Errorf("expected newline after edge declaration at line %d, col %d", p.line, p.col)
	}

	return Edge{
		Src:   StateID(src),
		Dst:   StateID(dst),
		Event: event,
		Guard: guard,
		Post:  post,
	}, nil
}

func (p *Parser) parseEndEdge() (EndEdge, error) {
	src, err := p.parseID()
	if err != nil {
		return EndEdge{}, err
	}
	p.skipSpaces()

	if !p.expectString("-->") {
		return EndEdge{}, fmt.Errorf("expected '-->' at line %d, col %d", p.line, p.col)
	}
	p.skipSpaces()

	if !p.expectString("[*]") {
		return EndEdge{}, fmt.Errorf("expected '[*]' at line %d, col %d", p.line, p.col)
	}
	p.skipSpaces()

	if !p.expectChar(':') {
		return EndEdge{}, fmt.Errorf("expected ':' at line %d, col %d", p.line, p.col)
	}
	p.skipSpaces()

	event, err := p.parseEvent()
	if err != nil {
		return EndEdge{}, err
	}
	p.skipSpaces()

	guard := "true" // Default value when guard is omitted
	if p.peek() == ';' {
		p.advance() // consume ';'
		p.skipSpaces()
		guard = p.parseUntilNewline()
	}

	if !p.expectNewlines() {
		return EndEdge{}, fmt.Errorf("expected newline after end edge declaration at line %d, col %d", p.line, p.col)
	}

	return EndEdge{
		Src:   StateID(src),
		Event: event,
		Guard: guard,
	}, nil
}

func (p *Parser) parseEvent() (Event, error) {
	eventID, err := p.parseID()
	if err != nil {
		return Event{}, err
	}

	event := Event{
		ID:     EventID(eventID),
		Params: []Var{},
	}

	if p.peek() == '(' {
		p.advance()
		p.skipSpaces()

		if p.peek() != ')' {
			param, err := p.parseID()
			if err != nil {
				return Event{}, err
			}
			event.Params = append(event.Params, Var(param))
			p.skipSpaces()

			for p.peek() == ',' {
				p.advance()
				p.skipSpaces()
				param, err := p.parseID()
				if err != nil {
					return Event{}, err
				}
				event.Params = append(event.Params, Var(param))
				p.skipSpaces()
			}
		}

		if !p.expectChar(')') {
			return Event{}, fmt.Errorf("expected ')' at line %d, col %d", p.line, p.col)
		}
	}

	return event, nil
}


func (p *Parser) parseID() (string, error) {
	var result strings.Builder

	if p.isAtEnd() || !p.isIDChar(p.peek()) {
		return "", fmt.Errorf("expected identifier at line %d, col %d", p.line, p.col)
	}

	for !p.isAtEnd() && p.isIDChar(p.peek()) {
		result.WriteByte(p.peek())
		p.advance()
	}

	return result.String(), nil
}

func (p *Parser) parseUntilSemicolon() string {
	var result strings.Builder
	for !p.isAtEnd() && p.peek() != ';' && p.peek() != '\n' {
		result.WriteByte(p.peek())
		p.advance()
	}
	return strings.TrimSpace(result.String())
}

func (p *Parser) parseUntilNewline() string {
	var result strings.Builder
	for !p.isAtEnd() && p.peek() != '\n' {
		result.WriteByte(p.peek())
		p.advance()
	}
	return strings.TrimSpace(result.String())
}

func (p *Parser) isIDChar(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_' || c == '-'
}

func (p *Parser) isStateID() bool {
	saved := p.pos
	savedLine := p.line
	savedCol := p.col

	defer func() {
		p.pos = saved
		p.line = savedLine
		p.col = savedCol
	}()

	_, err := p.parseID()
	if err != nil {
		return false
	}

	p.skipSpaces()
	return p.peekString("-->")
}

func (p *Parser) isEndEdge() bool {
	saved := p.pos
	savedLine := p.line
	savedCol := p.col

	defer func() {
		p.pos = saved
		p.line = savedLine
		p.col = savedCol
	}()

	_, err := p.parseID()
	if err != nil {
		return false
	}

	p.skipSpaces()
	if !p.peekString("-->") {
		return false
	}
	p.expectString("-->")
	p.skipSpaces()
	return p.peekString("[*]")
}

func (p *Parser) peek() byte {
	if p.isAtEnd() {
		return 0
	}
	return p.input[p.pos]
}

func (p *Parser) advance() byte {
	if p.isAtEnd() {
		return 0
	}
	c := p.input[p.pos]
	p.pos++
	if c == '\n' {
		p.line++
		p.col = 1
	} else {
		p.col++
	}
	return c
}

func (p *Parser) isAtEnd() bool {
	return p.pos >= len(p.input)
}

func (p *Parser) expectChar(expected byte) bool {
	if p.isAtEnd() || p.peek() != expected {
		return false
	}
	p.advance()
	return true
}

func (p *Parser) expectString(expected string) bool {
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

func (p *Parser) peekString(expected string) bool {
	if p.pos+len(expected) > len(p.input) {
		return false
	}
	return p.input[p.pos:p.pos+len(expected)] == expected
}

func (p *Parser) skipWhitespace() {
	for !p.isAtEnd() && (p.peek() == ' ' || p.peek() == '\t' || p.peek() == '\n' || p.peek() == '\r') {
		p.advance()
	}
}

func (p *Parser) skipSpaces() {
	for !p.isAtEnd() && (p.peek() == ' ' || p.peek() == '\t') {
		p.advance()
	}
}

func (p *Parser) expectNewlines() bool {
	count := 0
	for !p.isAtEnd() && (p.peek() == '\n' || p.peek() == '\r') {
		if p.peek() == '\n' {
			count++
		}
		p.advance()
	}
	return count > 0
}

func (p *Parser) skipLine() {
	for !p.isAtEnd() && p.peek() != '\n' {
		p.advance()
	}
	if !p.isAtEnd() {
		p.advance()
	}
}
