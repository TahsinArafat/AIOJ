package plagiarism

import "testing"

func TestLCSLength(t *testing.T) {
	tests := []struct {
		name     string
		a        []string
		b        []string
		expected int
	}{
		{"empty", nil, nil, 0},
		{"identical", []string{"a", "b", "c"}, []string{"a", "b", "c"}, 3},
		{"disjoint", []string{"a", "b"}, []string{"x", "y"}, 0},
		{"partial", []string{"a", "b", "c", "d"}, []string{"a", "x", "c", "y"}, 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := LCSLength(tt.a, tt.b)
			if got != tt.expected {
				t.Errorf("LCSLength() = %d, want %d", got, tt.expected)
			}
		})
	}
}

func TestLCSSimilarity(t *testing.T) {
	tests := []struct {
		name     string
		a        []string
		b        []string
		expected float64
	}{
		{"empty", nil, nil, 0.0},
		{"identical", []string{"a", "b"}, []string{"a", "b"}, 1.0},
		{"disjoint", []string{"a"}, []string{"b"}, 0.0},
		{"half", []string{"a", "b"}, []string{"a", "x"}, 0.5},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := LCSSimilarity(tt.a, tt.b)
			if got != tt.expected {
				t.Errorf("LCSSimilarity() = %f, want %f", got, tt.expected)
			}
		})
	}
}
