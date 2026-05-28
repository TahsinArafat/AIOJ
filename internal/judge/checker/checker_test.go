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
	if _, ok := GetChecker("", 0).(ExactChecker); !ok {
		t.Fatal("default not exact")
	}
	if _, ok := GetChecker("float", 1e-6).(FloatChecker); !ok {
		t.Fatal("not float")
	}
}

func TestFloatAbsoluteChecker(t *testing.T) {
	c := NewFloatAbsoluteChecker(1e-6)

	r := c.Check(nil, []byte("3.14159"), []byte("3.14159"))
	if !r.Passed {
		t.Error("exact match should pass")
	}

	r = c.Check(nil, []byte("1.0"), []byte("1.0000005"))
	if !r.Passed {
		t.Error("within tolerance should pass")
	}

	r = c.Check(nil, []byte("3.14159"), []byte("3.15"))
	if r.Passed {
		t.Error("outside tolerance should fail")
	}

	r = c.Check(nil, []byte("1 2 3"), []byte("1 2"))
	if r.Passed {
		t.Error("token count mismatch should fail")
	}
}

func TestFloatRelativeChecker(t *testing.T) {
	c := NewFloatRelativeChecker(0.01)

	r := c.Check(nil, []byte("100.0"), []byte("100.5"))
	if !r.Passed {
		t.Error("0.5% relative error should pass")
	}

	r = c.Check(nil, []byte("100.0"), []byte("115.0"))
	if r.Passed {
		t.Error("15% relative error should fail")
	}
}

func TestSortedChecker(t *testing.T) {
	c := &SortedChecker{}

	r := c.Check(nil, []byte("3\n1\n2"), []byte("1\n2\n3"))
	if !r.Passed {
		t.Error("same lines different order should pass")
	}

	r = c.Check(nil, []byte("3\n1\n2"), []byte("1\n2\n4"))
	if r.Passed {
		t.Error("different content should fail")
	}

	r = c.Check(nil, []byte("3\n1\n2"), []byte("1\n2\n3\n4"))
	if r.Passed {
		t.Error("different line count should fail")
	}
}

func TestUnorderedChecker(t *testing.T) {
	c := &UnorderedChecker{}

	r := c.Check(nil, []byte("a b c"), []byte("c a b"))
	if !r.Passed {
		t.Error("same tokens different order should pass")
	}

	r = c.Check(nil, []byte("a a b"), []byte("a b b"))
	if r.Passed {
		t.Error("different multisets should fail")
	}
}

func TestByteIdenticalChecker(t *testing.T) {
	c := &ByteIdenticalChecker{}

	r := c.Check(nil, []byte("hello\n"), []byte("hello\n"))
	if !r.Passed {
		t.Error("identical bytes should pass")
	}

	r = c.Check(nil, []byte("hello"), []byte("hello\n"))
	if r.Passed {
		t.Error("trailing newline difference should fail")
	}
}
