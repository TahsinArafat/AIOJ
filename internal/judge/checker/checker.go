package checker

import (
	"bytes"
	"math"
	"strconv"
	"strings"
)

type Result struct {
	Passed  bool
	Score   int
	Message string
}

type Checker interface {
	Name() string
	Check(input, expected, actual []byte) *Result
}

type ExactChecker struct{}

func (ExactChecker) Name() string { return "exact" }

func (ExactChecker) Check(_, expected, actual []byte) *Result {
	if bytes.Equal(bytes.TrimSpace(expected), bytes.TrimSpace(actual)) {
		return &Result{Passed: true, Score: 100, Message: "OK"}
	}
	return &Result{Passed: false, Score: 0, Message: "output mismatch"}
}

type LinesChecker struct{}

func (LinesChecker) Name() string { return "lines" }

func (LinesChecker) Check(_, expected, actual []byte) *Result {
	exp := strings.Split(string(bytes.TrimSpace(expected)), "\n")
	act := strings.Split(string(bytes.TrimSpace(actual)), "\n")
	if len(exp) != len(act) {
		return &Result{Passed: false, Score: 0, Message: "line count mismatch"}
	}
	for i := range exp {
		if strings.TrimSpace(exp[i]) != strings.TrimSpace(act[i]) {
			return &Result{Passed: false, Score: 0, Message: "line " + strconv.Itoa(i+1) + " differs"}
		}
	}
	return &Result{Passed: true, Score: 100, Message: "OK"}
}

type FloatChecker struct{ Epsilon float64 }

func (f FloatChecker) Name() string { return "float" }

func (f FloatChecker) Check(_, expected, actual []byte) *Result {
	exp := strings.Fields(string(bytes.TrimSpace(expected)))
	act := strings.Fields(string(bytes.TrimSpace(actual)))
	if len(exp) != len(act) {
		return &Result{Passed: false, Score: 0, Message: "token count mismatch"}
	}
	eps := f.Epsilon
	if eps == 0 {
		eps = 1e-6
	}
	for i := range exp {
		ev, _ := strconv.ParseFloat(exp[i], 64)
		av, _ := strconv.ParseFloat(act[i], 64)
		if math.Abs(ev-av) > eps {
			return &Result{Passed: false, Score: 0, Message: "diff at " + strconv.Itoa(i+1)}
		}
	}
	return &Result{Passed: true, Score: 100, Message: "OK"}
}

func GetChecker(name string) Checker {
	switch name {
	case "lines":
		return LinesChecker{}
	case "float":
		return FloatChecker{Epsilon: 1e-6}
	default:
		return ExactChecker{}
	}
}
