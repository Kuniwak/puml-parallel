package main

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
		States: make(map[StateID]State),
		Edges:  []Edge{},
	}

	if !p.expectString("@startuml") {
		return nil, fmt.Errorf("expected @startuml at line %d, col %d", p.line, p.col)
	}
	p.skipWhitespace()

	for !p.isAtEnd() && !p.peekString("@enduml") {
		if p.peekString("state") {
			state, err := p.parseState()
			if err != nil {
				return nil, err
			}
			diagram.States[state.ID] = state
		} else if p.isStateID() {
			edge, err := p.parseEdge()
			if err != nil {
				return nil, err
			}
			diagram.Edges = append(diagram.Edges, edge)
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
	p.skipWhitespace()

	name, err := p.parseStateName()
	if err != nil {
		return State{}, err
	}
	p.skipWhitespace()

	if !p.expectString("as") {
		return State{}, fmt.Errorf("expected 'as' at line %d, col %d", p.line, p.col)
	}
	p.skipWhitespace()

	id, err := p.parseID()
	if err != nil {
		return State{}, err
	}

	state := State{
		ID:   StateID(id),
		Name: name,
		Vars: []Var{},
	}

	p.skipWhitespace()

	for !p.isAtEnd() && !p.peekString("@enduml") && !p.peekString("state") && !p.isStateID() {
		if p.peekString(string(state.ID) + ":") {
			p.expectString(string(state.ID) + ":")
			p.skipWhitespace()
			varName, err := p.parseID()
			if err != nil {
				return State{}, err
			}
			state.Vars = append(state.Vars, Var(varName))
			p.skipWhitespace()
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

func (p *Parser) parseEdge() (Edge, error) {
	src, err := p.parseID()
	if err != nil {
		return Edge{}, err
	}
	p.skipWhitespace()

	if !p.expectString("-->") {
		return Edge{}, fmt.Errorf("expected '-->' at line %d, col %d", p.line, p.col)
	}
	p.skipWhitespace()

	dst, err := p.parseID()
	if err != nil {
		return Edge{}, err
	}
	p.skipWhitespace()

	if !p.expectChar(':') {
		return Edge{}, fmt.Errorf("expected ':' at line %d, col %d", p.line, p.col)
	}
	p.skipWhitespace()

	event, err := p.parseEvent()
	if err != nil {
		return Edge{}, err
	}
	p.skipWhitespace()

	if !p.expectChar(';') {
		return Edge{}, fmt.Errorf("expected ';' at line %d, col %d", p.line, p.col)
	}
	p.skipWhitespace()

	guard := p.parseUntilSemicolon()
	p.skipWhitespace()

	if !p.expectChar(';') {
		return Edge{}, fmt.Errorf("expected ';' at line %d, col %d", p.line, p.col)
	}
	p.skipWhitespace()

	post := p.parseUntilNewline()

	return Edge{
		Src:   StateID(src),
		Dst:   StateID(dst),
		Event: event,
		Guard: guard,
		Post:  post,
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
		p.skipWhitespace()

		if p.peek() != ')' {
			param, err := p.parseID()
			if err != nil {
				return Event{}, err
			}
			event.Params = append(event.Params, Var(param))
			p.skipWhitespace()

			for p.peek() == ',' {
				p.advance()
				p.skipWhitespace()
				param, err := p.parseID()
				if err != nil {
					return Event{}, err
				}
				event.Params = append(event.Params, Var(param))
				p.skipWhitespace()
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
	
	p.skipWhitespace()
	return p.peekString("-->")
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

func (p *Parser) skipLine() {
	for !p.isAtEnd() && p.peek() != '\n' {
		p.advance()
	}
	if !p.isAtEnd() {
		p.advance()
	}
}