package license

import (
	"reflect"
	"testing"
)

func TestParseExpression_Licenses(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want []string
	}{
		{"empty", "", nil},
		{"whitespace", "   ", nil},
		{"single", "MIT", []string{"MIT"}},
		{"or", "MIT OR Apache-2.0", []string{"MIT", "Apache-2.0"}},
		{"and", "MIT AND Apache-2.0", []string{"MIT", "Apache-2.0"}},
		{"lowercase operators", "mit or apache-2.0", []string{"mit", "apache-2.0"}},
		{"with exception", "Apache-2.0 WITH LLVM-exception", []string{"Apache-2.0"}},
		{"parenthesized", "(MIT OR BSD-2-Clause) AND Apache-2.0", []string{"MIT", "BSD-2-Clause", "Apache-2.0"}},
		{"mixed precedence", "MIT OR Apache-2.0 AND GPL-3.0-only", []string{"MIT", "Apache-2.0", "GPL-3.0-only"}},
		{"nested parens", "((MIT OR ISC) AND (BSD-3-Clause OR Apache-2.0))", []string{"MIT", "ISC", "BSD-3-Clause", "Apache-2.0"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expr := ParseExpression(tt.in)
			if tt.want == nil {
				if expr != nil {
					t.Fatalf("expected nil expression, got %v", expr)
				}
				return
			}
			got := expr.Licenses()
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Licenses() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseExpression_Precedence(t *testing.T) {
	// "MIT OR Apache-2.0 AND GPL-3.0-only" must bind AND tighter than OR:
	// MIT OR (Apache-2.0 AND GPL-3.0-only).
	expr := ParseExpression("MIT OR Apache-2.0 AND GPL-3.0-only")
	compound, ok := expr.(CompoundExpr)
	if !ok || compound.Op != OpOr {
		t.Fatalf("expected top-level OR, got %T %v", expr, expr)
	}
	right, ok := compound.Right.(CompoundExpr)
	if !ok || right.Op != OpAnd {
		t.Fatalf("expected right side to be AND, got %T %v", compound.Right, compound.Right)
	}
}

func TestSimpleExpr_WithException(t *testing.T) {
	expr := ParseExpression("Apache-2.0 WITH LLVM-exception")
	simple, ok := expr.(SimpleExpr)
	if !ok {
		t.Fatalf("expected SimpleExpr, got %T", expr)
	}
	if simple.License != "Apache-2.0" || simple.Exception != "LLVM-exception" {
		t.Errorf("got license=%q exception=%q", simple.License, simple.Exception)
	}
	if simple.String() != "Apache-2.0 WITH LLVM-exception" {
		t.Errorf("String() = %q", simple.String())
	}
}

func TestExpression_String(t *testing.T) {
	expr := ParseExpression("MIT OR Apache-2.0")
	if s := expr.String(); s != "(MIT OR Apache-2.0)" {
		t.Errorf("String() = %q, want (MIT OR Apache-2.0)", s)
	}
}
