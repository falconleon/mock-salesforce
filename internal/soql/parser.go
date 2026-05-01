package soql

import (
	"fmt"
	"strconv"
	"strings"
)

// Parser parses SOQL queries.
type Parser struct {
	lexer     *Lexer
	curToken  Token
	peekToken Token
	errors    []string
}

// NewParser creates a new parser for the given input.
func NewParser(input string) *Parser {
	p := &Parser{lexer: NewLexer(input)}
	// Read two tokens to initialize curToken and peekToken
	p.nextToken()
	p.nextToken()
	return p
}

// Parse parses the input and returns a SelectStatement.
func (p *Parser) Parse() (*SelectStatement, error) {
	stmt := &SelectStatement{}

	// Expect SELECT
	if !p.expectCurrent(TokenSelect) {
		return nil, fmt.Errorf("expected SELECT, got %s", p.curToken.Type)
	}
	p.nextToken()

	// Parse fields (and any parent-child subqueries) in the SELECT list
	if err := p.parseSelectList(stmt); err != nil {
		return nil, err
	}

	// Expect FROM
	if !p.expectCurrent(TokenFrom) {
		return nil, fmt.Errorf("expected FROM, got %s at position %d", p.curToken.Type, p.curToken.Pos)
	}
	p.nextToken()

	// Parse object name
	if !p.expectCurrent(TokenIdent) {
		return nil, fmt.Errorf("expected object name, got %s", p.curToken.Type)
	}
	stmt.Object = p.curToken.Literal
	p.nextToken()

	// Parse optional WHERE
	if p.curTokenIs(TokenWhere) {
		p.nextToken()
		where, err := p.parseWhereClause()
		if err != nil {
			return nil, err
		}
		stmt.Where = where
	}

	// Parse optional GROUP BY
	if p.curTokenIs(TokenGroup) {
		p.nextToken()
		if !p.expectCurrent(TokenBy) {
			return nil, fmt.Errorf("expected BY after GROUP, got %s", p.curToken.Type)
		}
		p.nextToken()
		groupBy, err := p.parseGroupBy()
		if err != nil {
			return nil, err
		}
		stmt.GroupBy = groupBy
	}

	// Parse optional HAVING
	if p.curTokenIs(TokenHaving) {
		p.nextToken()
		cond, err := p.parseCondition()
		if err != nil {
			return nil, err
		}
		stmt.Having = &WhereClause{Condition: cond}
	}

	// Parse optional ORDER BY
	if p.curTokenIs(TokenOrder) {
		p.nextToken()
		if !p.expectCurrent(TokenBy) {
			return nil, fmt.Errorf("expected BY after ORDER, got %s", p.curToken.Type)
		}
		p.nextToken()
		orderBy, err := p.parseOrderBy()
		if err != nil {
			return nil, err
		}
		stmt.OrderBy = orderBy
	}

	// Parse optional LIMIT
	if p.curTokenIs(TokenLimit) {
		p.nextToken()
		if !p.expectCurrent(TokenNumber) {
			return nil, fmt.Errorf("expected number after LIMIT, got %s", p.curToken.Type)
		}
		limit, _ := strconv.Atoi(p.curToken.Literal)
		stmt.Limit = &limit
		p.nextToken()
	}

	// Parse optional OFFSET
	if p.curTokenIs(TokenOffset) {
		p.nextToken()
		if !p.expectCurrent(TokenNumber) {
			return nil, fmt.Errorf("expected number after OFFSET, got %s", p.curToken.Type)
		}
		offset, _ := strconv.Atoi(p.curToken.Literal)
		stmt.Offset = &offset
		p.nextToken()
	}

	return stmt, nil
}

// parseSelectList parses the SELECT list, populating Fields and SubQueries on stmt.
// A parenthesised SELECT in the list is a parent-child subquery.
func (p *Parser) parseSelectList(stmt *SelectStatement) error {
	for {
		if p.curTokenIs(TokenLeftParen) && p.peekTokenIs(TokenSelect) {
			sub, err := p.parseSubQuery()
			if err != nil {
				return err
			}
			stmt.SubQueries = append(stmt.SubQueries, *sub)
		} else {
			field, err := p.parseSelectField()
			if err != nil {
				return err
			}
			stmt.Fields = append(stmt.Fields, field)
		}

		if !p.curTokenIs(TokenComma) {
			break
		}
		p.nextToken() // consume comma
	}

	return nil
}

// parseSelectField parses a SELECT-list field, which may be a regular field,
// a relationship field, or an aggregate function call, optionally followed by
// an alias (`AS alias` or bare `alias`).
func (p *Parser) parseSelectField() (Field, error) {
	field, err := p.parseFieldOrAggregate()
	if err != nil {
		return Field{}, err
	}
	// Optional alias.
	if p.curTokenIs(TokenAs) {
		p.nextToken()
		if !p.curTokenIs(TokenIdent) {
			return Field{}, fmt.Errorf("expected alias after AS, got %s", p.curToken.Type)
		}
		field.Alias = p.curToken.Literal
		p.nextToken()
	} else if p.curTokenIs(TokenIdent) {
		field.Alias = p.curToken.Literal
		p.nextToken()
	}
	return field, nil
}

// parseFieldOrAggregate parses either a plain (relationship) field or an
// aggregate function call: AGG(field) or COUNT().
func (p *Parser) parseFieldOrAggregate() (Field, error) {
	if !p.curTokenIs(TokenIdent) {
		return Field{}, fmt.Errorf("expected field name, got %s", p.curToken.Type)
	}
	if p.peekTokenIs(TokenLeftParen) {
		upper := strings.ToUpper(p.curToken.Literal)
		if isAggregateName(upper) {
			p.nextToken() // consume function name
			p.nextToken() // consume '('
			f := Field{Aggregate: upper}
			if p.curTokenIs(TokenRightParen) {
				if upper != "COUNT" {
					return Field{}, fmt.Errorf("%s requires an argument", upper)
				}
				p.nextToken() // consume ')'
				return f, nil
			}
			inner, err := p.parseField()
			if err != nil {
				return Field{}, err
			}
			f.Name = inner.Name
			f.Relation = inner.Relation
			if !p.curTokenIs(TokenRightParen) {
				return Field{}, fmt.Errorf("expected ) after %s argument, got %s", upper, p.curToken.Type)
			}
			p.nextToken()
			return f, nil
		}
	}
	return p.parseField()
}

// isAggregateName reports whether the upper-case identifier is a recognized
// SOQL aggregate function name.
func isAggregateName(s string) bool {
	switch s {
	case "COUNT", "COUNT_DISTINCT", "SUM", "AVG", "MIN", "MAX":
		return true
	}
	return false
}

// parseGroupBy parses a comma-separated field list after GROUP BY.
func (p *Parser) parseGroupBy() ([]Field, error) {
	var fields []Field
	for {
		f, err := p.parseField()
		if err != nil {
			return nil, err
		}
		fields = append(fields, f)
		if !p.curTokenIs(TokenComma) {
			break
		}
		p.nextToken()
	}
	return fields, nil
}

// parseSubQuery parses a parent-child subquery: ( SELECT ... FROM Rel [WHERE ...] [ORDER BY ...] [LIMIT N] [OFFSET N] ).
// Nested subqueries are not supported.
func (p *Parser) parseSubQuery() (*SubQuery, error) {
	if !p.expectCurrent(TokenLeftParen) {
		return nil, fmt.Errorf("expected ( for subquery, got %s", p.curToken.Type)
	}
	p.nextToken()

	if !p.expectCurrent(TokenSelect) {
		return nil, fmt.Errorf("expected SELECT inside subquery, got %s", p.curToken.Type)
	}
	p.nextToken()

	sub := &SubQuery{}

	// Parse field list (no nested subqueries)
	for {
		if p.curTokenIs(TokenLeftParen) {
			return nil, fmt.Errorf("nested subqueries are not supported")
		}
		field, err := p.parseField()
		if err != nil {
			return nil, err
		}
		sub.Fields = append(sub.Fields, field)
		if !p.curTokenIs(TokenComma) {
			break
		}
		p.nextToken() // consume comma
	}

	if !p.expectCurrent(TokenFrom) {
		return nil, fmt.Errorf("expected FROM in subquery, got %s", p.curToken.Type)
	}
	p.nextToken()

	if !p.expectCurrent(TokenIdent) {
		return nil, fmt.Errorf("expected relationship name in subquery, got %s", p.curToken.Type)
	}
	sub.Relationship = p.curToken.Literal
	p.nextToken()

	if p.curTokenIs(TokenWhere) {
		p.nextToken()
		where, err := p.parseWhereClause()
		if err != nil {
			return nil, err
		}
		sub.Where = where
	}

	if p.curTokenIs(TokenOrder) {
		p.nextToken()
		if !p.expectCurrent(TokenBy) {
			return nil, fmt.Errorf("expected BY after ORDER in subquery, got %s", p.curToken.Type)
		}
		p.nextToken()
		orderBy, err := p.parseOrderBy()
		if err != nil {
			return nil, err
		}
		sub.OrderBy = orderBy
	}

	if p.curTokenIs(TokenLimit) {
		p.nextToken()
		if !p.expectCurrent(TokenNumber) {
			return nil, fmt.Errorf("expected number after LIMIT in subquery, got %s", p.curToken.Type)
		}
		limit, _ := strconv.Atoi(p.curToken.Literal)
		sub.Limit = &limit
		p.nextToken()
	}

	if p.curTokenIs(TokenOffset) {
		p.nextToken()
		if !p.expectCurrent(TokenNumber) {
			return nil, fmt.Errorf("expected number after OFFSET in subquery, got %s", p.curToken.Type)
		}
		offset, _ := strconv.Atoi(p.curToken.Literal)
		sub.Offset = &offset
		p.nextToken()
	}

	if !p.expectCurrent(TokenRightParen) {
		return nil, fmt.Errorf("expected ) to close subquery, got %s", p.curToken.Type)
	}
	p.nextToken()

	return sub, nil
}

// parseField parses a single field (possibly with relationship).
func (p *Parser) parseField() (Field, error) {
	if !p.expectCurrent(TokenIdent) {
		return Field{}, fmt.Errorf("expected field name, got %s", p.curToken.Type)
	}

	field := Field{Name: p.curToken.Literal}
	p.nextToken()

	// Check for relationship (e.g., Owner.Name)
	if p.curTokenIs(TokenDot) {
		p.nextToken()
		if !p.expectCurrent(TokenIdent) {
			return Field{}, fmt.Errorf("expected field name after dot, got %s", p.curToken.Type)
		}
		field.Relation = field.Name
		field.Name = p.curToken.Literal
		p.nextToken()
	}

	return field, nil
}

// parseWhereClause parses the WHERE clause conditions.
func (p *Parser) parseWhereClause() (*WhereClause, error) {
	cond, err := p.parseCondition()
	if err != nil {
		return nil, err
	}
	return &WhereClause{Condition: cond}, nil
}

// parseCondition parses a condition (handles AND/OR precedence).
func (p *Parser) parseCondition() (Condition, error) {
	left, err := p.parseSimpleCondition()
	if err != nil {
		return nil, err
	}

	// Handle AND/OR
	for p.curTokenIs(TokenAnd) || p.curTokenIs(TokenOr) {
		op := p.curToken.Literal
		p.nextToken()
		right, err := p.parseSimpleCondition()
		if err != nil {
			return nil, err
		}
		left = &LogicalCondition{
			Operator: strings.ToUpper(op),
			Left:     left,
			Right:    right,
		}
	}

	return left, nil
}

// parseSimpleCondition parses a single comparison or IN condition.
func (p *Parser) parseSimpleCondition() (Condition, error) {
	// Handle NOT
	if p.curTokenIs(TokenNot) {
		p.nextToken()
		cond, err := p.parseSimpleCondition()
		if err != nil {
			return nil, err
		}
		return &NotCondition{Condition: cond}, nil
	}

	// Handle parenthesized conditions
	if p.curTokenIs(TokenLeftParen) {
		p.nextToken()
		cond, err := p.parseCondition()
		if err != nil {
			return nil, err
		}
		if !p.expectCurrent(TokenRightParen) {
			return nil, fmt.Errorf("expected ), got %s", p.curToken.Type)
		}
		p.nextToken()
		return cond, nil
	}

	// Parse field (allow aggregate function for HAVING)
	field, err := p.parseFieldOrAggregate()
	if err != nil {
		return nil, err
	}

	// Handle IN / NOT IN
	not := false
	if p.curTokenIs(TokenNot) {
		not = true
		p.nextToken()
	}
	if p.curTokenIs(TokenIn) {
		p.nextToken()
		values, err := p.parseInValues()
		if err != nil {
			return nil, err
		}
		return &InCondition{Field: field, Values: values, Not: not}, nil
	}
	if not {
		return nil, fmt.Errorf("expected IN after NOT, got %s", p.curToken.Type)
	}

	// Handle LIKE
	if p.curTokenIs(TokenLike) {
		p.nextToken()
		value, err := p.parseValue()
		if err != nil {
			return nil, err
		}
		return &ComparisonCondition{Field: field, Operator: "LIKE", Value: value}, nil
	}

	// Parse operator
	op, err := p.parseOperator()
	if err != nil {
		return nil, err
	}

	// Parse value
	value, err := p.parseValue()
	if err != nil {
		return nil, err
	}

	return &ComparisonCondition{Field: field, Operator: op, Value: value}, nil
}

// parseOperator parses a comparison operator.
func (p *Parser) parseOperator() (string, error) {
	var op string
	switch p.curToken.Type {
	case TokenEquals:
		op = "="
	case TokenNotEquals:
		op = "!="
	case TokenLessThan:
		op = "<"
	case TokenLessEqual:
		op = "<="
	case TokenGreaterThan:
		op = ">"
	case TokenGreaterEqual:
		op = ">="
	default:
		return "", fmt.Errorf("expected operator, got %s", p.curToken.Type)
	}
	p.nextToken()
	return op, nil
}

// parseValue parses a literal value (string, number, boolean, null, date literal).
func (p *Parser) parseValue() (any, error) {
	switch p.curToken.Type {
	case TokenString:
		val := p.curToken.Literal
		p.nextToken()
		return val, nil
	case TokenNumber:
		val := p.curToken.Literal
		p.nextToken()
		if strings.Contains(val, ".") {
			f, _ := strconv.ParseFloat(val, 64)
			return f, nil
		}
		i, _ := strconv.Atoi(val)
		return i, nil
	case TokenTrue:
		p.nextToken()
		return true, nil
	case TokenFalse:
		p.nextToken()
		return false, nil
	case TokenNull:
		p.nextToken()
		return nil, nil
	case TokenIdent:
		name := strings.ToUpper(p.curToken.Literal)
		if isSimpleDateLiteral(name) {
			p.nextToken()
			return DateLiteral{Name: name}, nil
		}
		if isParamDateLiteral(name) {
			p.nextToken()
			if !p.curTokenIs(TokenColon) {
				return nil, fmt.Errorf("expected ':' after %s, got %s", name, p.curToken.Type)
			}
			p.nextToken()
			if !p.curTokenIs(TokenNumber) {
				return nil, fmt.Errorf("expected number after %s:, got %s", name, p.curToken.Type)
			}
			n, err := strconv.Atoi(p.curToken.Literal)
			if err != nil || n < 0 {
				return nil, fmt.Errorf("invalid parameter for %s: %s", name, p.curToken.Literal)
			}
			p.nextToken()
			return DateLiteral{Name: name, N: n}, nil
		}
		return nil, fmt.Errorf("expected value, got identifier %q", p.curToken.Literal)
	default:
		return nil, fmt.Errorf("expected value, got %s (%s)", p.curToken.Type, p.curToken.Literal)
	}
}

// parseInValues parses the values in an IN clause.
func (p *Parser) parseInValues() ([]any, error) {
	if !p.expectCurrent(TokenLeftParen) {
		return nil, fmt.Errorf("expected ( after IN, got %s", p.curToken.Type)
	}
	p.nextToken()

	var values []any
	for {
		val, err := p.parseValue()
		if err != nil {
			return nil, err
		}
		values = append(values, val)

		if !p.curTokenIs(TokenComma) {
			break
		}
		p.nextToken() // consume comma
	}

	if !p.expectCurrent(TokenRightParen) {
		return nil, fmt.Errorf("expected ) after IN values, got %s", p.curToken.Type)
	}
	p.nextToken()

	return values, nil
}

// parseOrderBy parses ORDER BY fields.
func (p *Parser) parseOrderBy() ([]OrderByField, error) {
	var fields []OrderByField

	for {
		field, err := p.parseFieldOrAggregate()
		if err != nil {
			return nil, err
		}

		orderField := OrderByField{Field: field}

		// Check for ASC/DESC
		if p.curTokenIs(TokenAsc) {
			orderField.Descending = false
			p.nextToken()
		} else if p.curTokenIs(TokenDesc) {
			orderField.Descending = true
			p.nextToken()
		}

		// Check for NULLS FIRST/LAST
		if p.curTokenIs(TokenNulls) {
			p.nextToken()
			if p.curTokenIs(TokenFirst) {
				nullsFirst := true
				orderField.NullsFirst = &nullsFirst
				p.nextToken()
			} else if p.curTokenIs(TokenLast) {
				nullsFirst := false
				orderField.NullsFirst = &nullsFirst
				p.nextToken()
			}
		}

		fields = append(fields, orderField)

		if !p.curTokenIs(TokenComma) {
			break
		}
		p.nextToken() // consume comma
	}

	return fields, nil
}

// Helper methods

func (p *Parser) nextToken() {
	p.curToken = p.peekToken
	p.peekToken = p.lexer.NextToken()
}

func (p *Parser) curTokenIs(t TokenType) bool {
	return p.curToken.Type == t
}

func (p *Parser) peekTokenIs(t TokenType) bool {
	return p.peekToken.Type == t
}

func (p *Parser) expectCurrent(t TokenType) bool {
	return p.curToken.Type == t
}
