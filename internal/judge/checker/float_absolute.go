package checker

import (
	"math"
	"strconv"
	"strings"
)

// FloatAbsoluteChecker checks |expected - actual| <= epsilon for each token.
type FloatAbsoluteChecker struct {
	Epsilon float64
}

func NewFloatAbsoluteChecker(epsilon float64) *FloatAbsoluteChecker {
	if epsilon <= 0 {
		epsilon = 1e-6
	}
	return &FloatAbsoluteChecker{Epsilon: epsilon}
}

func (c *FloatAbsoluteChecker) Name() string { return "float_absolute" }

func (c *FloatAbsoluteChecker) Check(input, expected, actual []byte) *Result {
	expTokens := strings.Fields(string(expected))
	actTokens := strings.Fields(string(actual))
	if len(expTokens) != len(actTokens) {
		return &Result{Passed: false, Score: 0, Message: "token count mismatch"}
	}
	for i := range expTokens {
		e, err1 := strconv.ParseFloat(expTokens[i], 64)
		a, err2 := strconv.ParseFloat(actTokens[i], 64)
		if err1 != nil || err2 != nil {
			if expTokens[i] != actTokens[i] {
				return &Result{Passed: false, Score: 0, Message: "non-numeric token mismatch"}
			}
			continue
		}
		if math.Abs(e-a) > c.Epsilon {
			return &Result{Passed: false, Score: 0, Message: "value outside absolute tolerance"}
		}
	}
	return &Result{Passed: true, Score: 100, Message: "OK"}
}
