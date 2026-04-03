// Package soql provides a SOQL (Salesforce Object Query Language) parser.
package soql

// TokenType represents the type of a lexical token.
type TokenType int

const (
	// Special tokens
	TokenIllegal TokenType = iota
	TokenEOF
	TokenWhitespace

	// Literals
	TokenIdent  // field names, object names
	TokenString // 'quoted string'
	TokenNumber // 123, 45.67

	// Keywords
	TokenSelect
	TokenFrom
	TokenWhere
	TokenAnd
	TokenOr
	TokenNot
	TokenIn
	TokenLike
	TokenNull
	TokenTrue
	TokenFalse
	TokenOrderBy
	TokenOrder
	TokenBy
	TokenAsc
	TokenDesc
	TokenLimit
	TokenOffset
	TokenNullsFirst
	TokenNullsLast
	TokenNulls
	TokenFirst
	TokenLast

	// Operators
	TokenEquals       // =
	TokenNotEquals    // != or <>
	TokenLessThan     // <
	TokenLessEqual    // <=
	TokenGreaterThan  // >
	TokenGreaterEqual // >=

	// Delimiters
	TokenComma       // ,
	TokenDot         // .
	TokenLeftParen   // (
	TokenRightParen  // )
	TokenLeftBrace   // {
	TokenRightBrace  // }
)

// Token represents a lexical token.
type Token struct {
	Type    TokenType
	Literal string
	Pos     int // position in input
}

// String returns the string representation of the token type.
func (t TokenType) String() string {
	switch t {
	case TokenIllegal:
		return "ILLEGAL"
	case TokenEOF:
		return "EOF"
	case TokenWhitespace:
		return "WHITESPACE"
	case TokenIdent:
		return "IDENT"
	case TokenString:
		return "STRING"
	case TokenNumber:
		return "NUMBER"
	case TokenSelect:
		return "SELECT"
	case TokenFrom:
		return "FROM"
	case TokenWhere:
		return "WHERE"
	case TokenAnd:
		return "AND"
	case TokenOr:
		return "OR"
	case TokenNot:
		return "NOT"
	case TokenIn:
		return "IN"
	case TokenLike:
		return "LIKE"
	case TokenNull:
		return "NULL"
	case TokenTrue:
		return "TRUE"
	case TokenFalse:
		return "FALSE"
	case TokenOrder:
		return "ORDER"
	case TokenBy:
		return "BY"
	case TokenAsc:
		return "ASC"
	case TokenDesc:
		return "DESC"
	case TokenLimit:
		return "LIMIT"
	case TokenOffset:
		return "OFFSET"
	case TokenNulls:
		return "NULLS"
	case TokenFirst:
		return "FIRST"
	case TokenLast:
		return "LAST"
	case TokenEquals:
		return "="
	case TokenNotEquals:
		return "!="
	case TokenLessThan:
		return "<"
	case TokenLessEqual:
		return "<="
	case TokenGreaterThan:
		return ">"
	case TokenGreaterEqual:
		return ">="
	case TokenComma:
		return ","
	case TokenDot:
		return "."
	case TokenLeftParen:
		return "("
	case TokenRightParen:
		return ")"
	default:
		return "UNKNOWN"
	}
}

// keywords maps keyword strings to token types.
var keywords = map[string]TokenType{
	"SELECT": TokenSelect,
	"FROM":   TokenFrom,
	"WHERE":  TokenWhere,
	"AND":    TokenAnd,
	"OR":     TokenOr,
	"NOT":    TokenNot,
	"IN":     TokenIn,
	"LIKE":   TokenLike,
	"NULL":   TokenNull,
	"TRUE":   TokenTrue,
	"FALSE":  TokenFalse,
	"ORDER":  TokenOrder,
	"BY":     TokenBy,
	"ASC":    TokenAsc,
	"DESC":   TokenDesc,
	"LIMIT":  TokenLimit,
	"OFFSET": TokenOffset,
	"NULLS":  TokenNulls,
	"FIRST":  TokenFirst,
	"LAST":   TokenLast,
}

// LookupIdent returns the token type for an identifier.
func LookupIdent(ident string) TokenType {
	if tok, ok := keywords[ident]; ok {
		return tok
	}
	return TokenIdent
}
