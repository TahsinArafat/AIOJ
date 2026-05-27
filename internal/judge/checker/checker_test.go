package checker

import "testing"

func TestExact(t *testing.T) {
	c := ExactChecker{}
	if r := c.Check(nil, []byte("a\n"), []byte("a\n")); !r.Passed {
		t.Fatal("fail")
	}
	if r := c.Check(nil, []byte("a"), []byte("  a  ")); !r.Passed {
		t.Fatal("trim fail")
	}
	if r := c.Check(nil, []byte("a"), []byte("b")); r.Passed {
		t.Fatal("should fail")
	}
}

func TestLines(t *testing.T) {
	c := LinesChecker{}
	if r := c.Check(nil, []byte("1\n2"), []byte("1\n2")); !r.Passed {
		t.Fatal("fail")
	}
	if r := c.Check(nil, []byte("1\n2"), []byte("1\n3")); r.Passed {
		t.Fatal("should fail")
	}
}

func TestFloat(t *testing.T) {
	c := FloatChecker{Epsilon: 1e-4}
	if r := c.Check(nil, []byte("3.14159"), []byte("3.14158")); !r.Passed {
		t.Fatal("epsilon fail")
	}
}

func TestGetChecker(t *testing.T) {
	if _, ok := GetChecker("").(ExactChecker); !ok {
		t.Fatal("default not exact")
	}
	if _, ok := GetChecker("float").(FloatChecker); !ok {
		t.Fatal("not float")
	}
}
