package auth

import "testing"

func TestValidatePasswordStrength(t *testing.T) {
	cases := []struct {
		pwd     string
		wantErr bool
	}{
		{"short", true},
		{"alllowercasebut12chars", true},
		{"NoDigits!ButTwelveChars", true},
		{"nodigitsbut12chars", true},
		{"Valid1Pass!xy", false},
		{"Valid1Pass!with more chars", false},
		{"validpass!2025", true},
	}
	for _, c := range cases {
		err := ValidatePasswordStrength(c.pwd)
		if (err != nil) != c.wantErr {
			t.Errorf("ValidatePasswordStrength(%q) err=%v, wantErr=%v", c.pwd, err, c.wantErr)
		}
	}
}

func TestIsCommonPassword(t *testing.T) {
	if !IsCommonPassword("password123") {
		t.Error("expected 'password123' to be common")
	}
	if IsCommonPassword("Xq!9pL@2mZ$vN") {
		t.Error("expected long random string not to be common")
	}
}
