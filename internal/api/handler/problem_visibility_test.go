package handler

import "testing"

func TestVisibleForCreator(t *testing.T) {
	cases := []struct {
		name      string
		requested bool
		role      string
		want      bool
	}{
		{name: "admin can publish", requested: true, role: "admin", want: true},
		{name: "setter can publish", requested: true, role: "setter", want: true},
		{name: "ordinary user cannot publish", requested: true, role: "user", want: false},
		{name: "admin can keep private", requested: false, role: "admin", want: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := visibleForCreator(tc.requested, tc.role); got != tc.want {
				t.Fatalf("visibleForCreator(%v, %q) = %v, want %v", tc.requested, tc.role, got, tc.want)
			}
		})
	}
}
