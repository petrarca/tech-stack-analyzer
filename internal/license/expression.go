package license

import "strings"

// This file implements a small SPDX license-expression parser supporting the
// AND, OR and WITH operators and parenthesized grouping, per the SPDX license
// expression syntax (https://spdx.github.io/spdx-spec/SPDX-license-expressions/).
//
// Precedence (lowest to highest binding): OR, AND, WITH. Parentheses override.
// The parser is intentionally lenient: an unrecognised token is treated as a
// license identifier so malformed input degrades to a best-effort result rather
// than failing.

// Operator is a compound-expression conjunction.
type Operator string

const (
	// OpOr is the SPDX OR operator (choice of licenses).
	OpOr Operator = "OR"
	// OpAnd is the SPDX AND operator (combined requirements).
	OpAnd Operator = "AND"
	// OpWith is the SPDX WITH operator (a license with an exception).
	OpWith Operator = "WITH"
)

// Expression is a parsed SPDX license expression: either a simple license id or
// a compound expression joined by an operator.
type Expression interface {
	// Licenses returns every license identifier appearing in the expression,
	// in left-to-right order (exceptions from WITH are not included).
	Licenses() []string
	// String renders the expression back to its canonical SPDX form.
	String() string
}

// SimpleExpr is a single license identifier (optionally a "<license> WITH
// <exception>" pair, which SPDX treats as a simple expression for categorisation).
type SimpleExpr struct {
	License   string
	Exception string // set when this is a "<license> WITH <exception>" expression
}

// Licenses implements Expression.
func (e SimpleExpr) Licenses() []string { return []string{e.License} }

// String implements Expression.
func (e SimpleExpr) String() string {
	if e.Exception != "" {
		return e.License + " WITH " + e.Exception
	}
	return e.License
}

// CompoundExpr joins two sub-expressions with AND or OR.
type CompoundExpr struct {
	Op    Operator
	Left  Expression
	Right Expression
}

// Licenses implements Expression.
func (e CompoundExpr) Licenses() []string {
	return append(e.Left.Licenses(), e.Right.Licenses()...)
}

// String implements Expression.
func (e CompoundExpr) String() string {
	return "(" + e.Left.String() + " " + string(e.Op) + " " + e.Right.String() + ")"
}

// ParseExpression parses an SPDX license expression into an AST. It returns nil
// for an empty/whitespace-only input. License identifiers are returned verbatim
// (not normalised); callers normalise via Normalizer as needed.
func ParseExpression(input string) Expression {
	tokens := tokenizeExpression(input)
	if len(tokens) == 0 {
		return nil
	}
	p := &exprParser{tokens: tokens}
	expr := p.parseOr()
	if p.pos < len(p.tokens) {
		// Trailing tokens that did not parse (malformed input): fall back to a
		// simple expression of the whole original string so nothing is lost.
		return SimpleExpr{License: strings.TrimSpace(input)}
	}
	return expr
}

// tokenizeExpression splits an expression into license-id and operator/paren
// tokens. Operators are matched case-insensitively and emitted upper-cased.
func tokenizeExpression(input string) []string {
	// Surround parentheses with spaces so a simple field split separates them.
	input = strings.ReplaceAll(input, "(", " ( ")
	input = strings.ReplaceAll(input, ")", " ) ")
	fields := strings.Fields(input)

	tokens := make([]string, 0, len(fields))
	for _, f := range fields {
		switch strings.ToUpper(f) {
		case "AND", "OR", "WITH", "(", ")":
			tokens = append(tokens, strings.ToUpper(f))
		default:
			tokens = append(tokens, f)
		}
	}
	return tokens
}

// exprParser is a recursive-descent parser over the token stream.
type exprParser struct {
	tokens []string
	pos    int
}

func (p *exprParser) peek() string {
	if p.pos < len(p.tokens) {
		return p.tokens[p.pos]
	}
	return ""
}

func (p *exprParser) next() string {
	t := p.peek()
	p.pos++
	return t
}

// parseOr parses the lowest-precedence level (OR-joined terms).
func (p *exprParser) parseOr() Expression {
	left := p.parseAnd()
	for p.peek() == "OR" {
		p.next()
		right := p.parseAnd()
		left = CompoundExpr{Op: OpOr, Left: left, Right: right}
	}
	return left
}

// parseAnd parses AND-joined terms (binds tighter than OR).
func (p *exprParser) parseAnd() Expression {
	left := p.parseWith()
	for p.peek() == "AND" {
		p.next()
		right := p.parseWith()
		left = CompoundExpr{Op: OpAnd, Left: left, Right: right}
	}
	return left
}

// parseWith parses a primary optionally followed by "WITH <exception>".
func (p *exprParser) parseWith() Expression {
	left := p.parsePrimary()
	if p.peek() == "WITH" {
		p.next()
		exception := p.next() // exception identifier
		if simple, ok := left.(SimpleExpr); ok {
			simple.Exception = exception
			return simple
		}
	}
	return left
}

// parsePrimary parses a parenthesized group or a single license identifier.
func (p *exprParser) parsePrimary() Expression {
	if p.peek() == "(" {
		p.next() // consume "("
		inner := p.parseOr()
		if p.peek() == ")" {
			p.next() // consume ")"
		}
		return inner
	}
	return SimpleExpr{License: p.next()}
}
