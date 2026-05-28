package checker

import (
	"math"
	"strconv"
	"strings"
)

// FloatRelativeChecker checks |expected - actual| / max(|expected|, 1) <= epsilon.
type FloatRelativeChecker struct {
	Epsilon float64
}

func NewFloatRelativeChecker(epsilon float64) *FloatRelativeChecker {
	if epsilon <= 0 {
		epsilon = 1e-6
	}
	return &FloatRelativeChecker{Epsilon: epsilon}
}

func (c *FloatRelativeChecker) Name() string { return "float_relative" }

func (c *FloatRelativeChecker) Check(input, expected, actual []byte) *Result {
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
		denom := math.Max(math.Abs(e), 1.0)
		if math.Abs(e-a)/denom > c.Epsilon {
			return &Result{Passed: false, Score: 0, Message: "value outside relative tolerance"}
		}
	}
	return &Result{Passed: true, Score: 100, Message: "OK"}
}
