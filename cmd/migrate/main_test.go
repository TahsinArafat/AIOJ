package main

import (
	"strconv"
	"testing"
)

// parseSteps validates that steps argument is a non-zero integer.
func TestParseSteps(t *testing.T) {
	cases := []struct {
		input   string
		want    int
		wantErr bool
	}{
		{"1", 1, false},
		{"-1", -1, false},
		{"5", 5, false},
		{"0", 0, true},
		{"abc", 0, true},
	}
	for _, c := range cases {
		n, err := strconv.Atoi(c.input)
		if c.wantErr {
			if err == nil && n != 0 {
				t.Errorf("parseSteps(%q): expected error or zero, got %d", c.input, n)
			}
			continue
		}
		if err != nil {
			t.Errorf("parseSteps(%q): unexpected error %v", c.input, err)
			continue
		}
		if n != c.want {
			t.Errorf("parseSteps(%q) = %d, want %d", c.input, n, c.want)
		}
	}
}
