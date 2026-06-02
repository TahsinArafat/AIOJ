package handler

import "testing"

func TestNormalizeOutput(t *testing.T) {
	cases := []struct {
		input, want string
	}{
		{"hello\r\n", "hello"},
		{"  hello  \n", "hello"},
		{"a\nb\n", "a\nb"},
		{"", ""},
	}
	for _, c := range cases {
		got := normalizeOutput(c.input)
		if got != c.want {
			t.Errorf("normalizeOutput(%q) = %q, want %q", c.input, got, c.want)
		}
	}
}
