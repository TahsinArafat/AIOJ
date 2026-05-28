package checker

import "bytes"

// ByteIdenticalChecker does byte-exact comparison with NO whitespace trimming.
type ByteIdenticalChecker struct{}

func (c *ByteIdenticalChecker) Name() string { return "byte_identical" }

func (c *ByteIdenticalChecker) Check(input, expected, actual []byte) *Result {
	if bytes.Equal(expected, actual) {
		return &Result{Passed: true, Score: 100, Message: "OK"}
	}
	return &Result{Passed: false, Score: 0, Message: "byte mismatch"}
}
