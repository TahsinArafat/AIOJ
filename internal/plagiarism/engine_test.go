package plagiarism

import "testing"

func TestNormalizeCode_StripsComments(t *testing.T) {
	code := "int x = 5; // this is a comment"
	normalized := NormalizeCode(code)
	if normalized == code {
		t.Error("expected comments to be stripped")
	}
}

func TestNormalizeCode_ReplacesStrings(t *testing.T) {
	code := `printf("hello world");`
	normalized := NormalizeCode(code)
	expected := "printf( STR );"
	if normalized != expected {
		t.Errorf("NormalizeCode() = %q, want %q", normalized, expected)
	}
}

func TestNormalizeCode_ReplacesNumbers(t *testing.T) {
	code := "int x = 42;"
	normalized := NormalizeCode(code)
	expected := "int x = NUM ;"
	if normalized != expected {
		t.Errorf("NormalizeCode() = %q, want %q", normalized, expected)
	}
}

func TestNormalizeCode_BlockComments(t *testing.T) {
	code := "int /* foo */ x = 5;"
	normalized := NormalizeCode(code)
	expected := "int x = NUM ;"
	if normalized != expected {
		t.Errorf("NormalizeCode() = %q, want %q", normalized, expected)
	}
}

func TestTokenize(t *testing.T) {
	code := "int main() { return 0; }"
	tokens := Tokenize(code)
	if len(tokens) == 0 {
		t.Error("expected non-empty token list")
	}
}

func TestTokenize_Empty(t *testing.T) {
	tokens := Tokenize("")
	if len(tokens) != 0 {
		t.Errorf("expected 0 tokens, got %d", len(tokens))
	}
}

func TestCompareCodes_Identical(t *testing.T) {
	code := "int main() { return 0; }"
	sim := CompareCodes(code, code)
	if sim < 0.99 {
		t.Errorf("expected similarity ~1.0, got %f", sim)
	}
}

func TestCompareCodes_Different(t *testing.T) {
	code1 := "int main() { int a = 1; return a; }"
	code2 := "int main() { int b = 2; return b + 3; }"
	sim := CompareCodes(code1, code2)
	if sim > 0.95 {
		t.Errorf("expected similarity < 0.95 (different logic), got %f", sim)
	}
	if sim < 0.1 {
		t.Errorf("expected similarity > 0.1 (some structure shared), got %f", sim)
	}
}

func TestCompareCodes_RenamedVariables(t *testing.T) {
	code1 := "int solve() { int x = readInt(); return x * 2; }"
	code2 := "int calc() { int y = readInt(); return y * 2; }"
	sim := CompareCodes(code1, code2)
	if sim < 0.5 {
		t.Errorf("expected similarity > 0.5 (same structure, renamed vars), got %f", sim)
	}
}
