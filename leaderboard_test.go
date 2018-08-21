package leaderboard

import (
	"testing"
)

func TestFoo(t *testing.T) {
	tests := []struct {
		a, b, c, expected float64
	}{
		{1,2,3,6},
	}
	for _, tt := range tests {
		actual := tt.a + tt.b + tt.c
		if !calc.Eqf(actual, tt.expected) {
			t.Errorf("Foo(%.3f, %.3f, %.3f): got: %.3f, want: %.3f",
				tt.a, tt.b, tt.c, actual, tt.expected)
		}
	}
}
