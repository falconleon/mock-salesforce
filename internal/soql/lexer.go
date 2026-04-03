package soql

import (
	"strings"
	"unicode"
)

// Lexer tokenizes SOQL input.
type Lexer struct {
	input   string
	pos     int  // current position
	readPos int  // next position to read
	ch      byte // current character
}

// NewLexer creates a new lexer for the given input.
func NewLexer(input string) *Lexer {
	l := &Lexer{input: input}
	l.readChar()
	return l
}

// readChar reads the next character.
func (l *Lexer) readChar() {
	if l.readPos >= len(l.input) {
		l.ch = 0 // EOF
	} else {
		l.ch = l.input[l.readPos]
	}
	l.pos = l.readPos
	l.readPos++
}

// peekChar returns the next character without advancing.
func (l *Lexer) peekChar() byte {
	if l.readPos >= len(l.input) {
		return 0
	}
	return l.input[l.readPos]
}

// NextToken returns the next token.
func (l *Lexer) NextToken() Token {
	l.skipWhitespace()

	var tok Token
	tok.Pos = l.pos

	switch l.ch {
	case '=':
		tok = Token{Type: TokenEquals, Literal: "=", Pos: l.pos}
	case '!':
		if l.peekChar() == '=' {
			l.readChar()
			tok = Token{Type: TokenNotEquals, Literal: "!=", Pos: l.pos - 1}
		} else {
			tok = Token{Type: TokenIllegal, Literal: string(l.ch), Pos: l.pos}
		}
	case '<':
		if l.peekChar() == '=' {
			l.readChar()
			tok = Token{Type: TokenLessEqual, Literal: "<=", Pos: l.pos - 1}
		} else if l.peekChar() == '>' {
			l.readChar()
			tok = Token{Type: TokenNotEquals, Literal: "<>", Pos: l.pos - 1}
		} else {
			tok = Token{Type: TokenLessThan, Literal: "<", Pos: l.pos}
		}
	case '>':
		if l.peekChar() == '=' {
			l.readChar()
			tok = Token{Type: TokenGreaterEqual, Literal: ">=", Pos: l.pos - 1}
		} else {
			tok = Token{Type: TokenGreaterThan, Literal: ">", Pos: l.pos}
		}
	case ',':
		tok = Token{Type: TokenComma, Literal: ",", Pos: l.pos}
	case '.':
		tok = Token{Type: TokenDot, Literal: ".", Pos: l.pos}
	case '(':
		tok = Token{Type: TokenLeftParen, Literal: "(", Pos: l.pos}
	case ')':
		tok = Token{Type: TokenRightParen, Literal: ")", Pos: l.pos}
	case '\'':
		tok.Type = TokenString
		tok.Literal = l.readString()
		return tok
	case 0:
		tok = Token{Type: TokenEOF, Literal: "", Pos: l.pos}
	default:
		if isLetter(l.ch) || l.ch == '_' {
			tok.Literal = l.readIdentifier()
			tok.Type = LookupIdent(strings.ToUpper(tok.Literal))
			return tok
		} else if isDigit(l.ch) || (l.ch == '-' && isDigit(l.peekChar())) {
			tok.Literal = l.readNumber()
			tok.Type = TokenNumber
			return tok
		} else {
			tok = Token{Type: TokenIllegal, Literal: string(l.ch), Pos: l.pos}
		}
	}

	l.readChar()
	return tok
}

// skipWhitespace skips whitespace characters.
func (l *Lexer) skipWhitespace() {
	for l.ch == ' ' || l.ch == '\t' || l.ch == '\n' || l.ch == '\r' {
		l.readChar()
	}
}

// readIdentifier reads an identifier.
func (l *Lexer) readIdentifier() string {
	pos := l.pos
	for isLetter(l.ch) || isDigit(l.ch) || l.ch == '_' {
		l.readChar()
	}
	return l.input[pos:l.pos]
}

// readNumber reads a number (integer or decimal).
func (l *Lexer) readNumber() string {
	pos := l.pos
	if l.ch == '-' {
		l.readChar()
	}
	for isDigit(l.ch) {
		l.readChar()
	}
	if l.ch == '.' && isDigit(l.peekChar()) {
		l.readChar() // consume '.'
		for isDigit(l.ch) {
			l.readChar()
		}
	}
	return l.input[pos:l.pos]
}

// readString reads a single-quoted string.
func (l *Lexer) readString() string {
	l.readChar() // skip opening quote
	pos := l.pos
	for l.ch != '\'' && l.ch != 0 {
		if l.ch == '\\' && l.peekChar() == '\'' {
			l.readChar() // skip escape
		}
		l.readChar()
	}
	str := l.input[pos:l.pos]
	if l.ch == '\'' {
		l.readChar() // skip closing quote
	}
	return str
}

func isLetter(ch byte) bool {
	return unicode.IsLetter(rune(ch))
}

func isDigit(ch byte) bool {
	return ch >= '0' && ch <= '9'
}
