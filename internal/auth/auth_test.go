package auth

import "testing"

func TestHashPassword(t *testing.T) {
	tests := []struct {
		name     string
		password string
		wantErr  bool
	}{
		{"normal password", "password123", false},
		{"short password", "a", false},
		{"long password", "thisisaverylongpasswordthatexceedstypicallimits12345", false},
		{"empty password", "", false},
		{"special chars", "!@#$%^&*()_+-=[]{}\\|;:'\",.<>/?", false},
		{"unicode password", "пароль123", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hash, err := HashPassword(tt.password)
			if (err != nil) != tt.wantErr {
				t.Errorf("HashPassword() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if hash == "" {
				t.Error("HashPassword() returned empty hash")
			}
			// Hash should be different from password
			if hash == tt.password {
				t.Error("HashPassword() returned plaintext password")
			}
		})
	}
}

func TestCheckPassword(t *testing.T) {
	password := "mysecretpassword"
	hash, err := HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword() failed: %v", err)
	}

	tests := []struct {
		name     string
		password string
		expected bool
	}{
		{"correct password", password, true},
		{"wrong password", "wrongpassword", false},
		{"empty password", "", false},
		{"similar password", "mysecretpasswor", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CheckPassword(tt.password, hash)
			if got != tt.expected {
				t.Errorf("CheckPassword(%q, hash) = %v, want %v", tt.password, got, tt.expected)
			}
		})
	}
}

func TestHashPasswordConsistency(t *testing.T) {
	password := "consistent"
	hash1, err1 := HashPassword(password)
	hash2, err2 := HashPassword(password)

	if err1 != nil || err2 != nil {
		t.Fatal("HashPassword() should not error on valid input")
	}

	// Each hash should be different (bcrypt uses random salt)
	if hash1 == hash2 {
		t.Error("HashPassword() should produce different hashes due to salt")
	}

	// Both should verify correctly
	if !CheckPassword(password, hash1) {
		t.Error("CheckPassword() failed for hash1")
	}
	if !CheckPassword(password, hash2) {
		t.Error("CheckPassword() failed for hash2")
	}
}

func TestCheckPasswordInvalidHash(t *testing.T) {
	if CheckPassword("anything", "invalid-hash-format") {
		t.Error("CheckPassword() should return false for invalid hash")
	}
}
