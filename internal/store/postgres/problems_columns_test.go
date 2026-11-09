package postgres

import (
	"reflect"
	"strings"
	"testing"

	"github.com/tahsinarafat/aioj/internal/model"
)

// problemColumns is shared by the practice read and the contest read. It used to
// be two hand-maintained copies, and the contest copy lost `constraints` — the
// Constraints block simply never appeared on a contest problem. This test fails
// when the shared list falls behind the model, so the next added column cannot
// be silently dropped either.
func TestProblemColumnsCoverTheWholeModel(t *testing.T) {
	// Computed per-request, not stored in the problems table.
	skip := map[string]bool{"language_limits": true}

	typ := reflect.TypeOf(model.Problem{})
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		name := strings.Split(field.Tag.Get("json"), ",")[0]
		if name == "" || name == "-" || skip[name] {
			continue
		}
		if !strings.Contains(problemColumns, "p."+name) {
			t.Errorf("model.Problem.%s is not selected by problemColumns (missing p.%s)", field.Name, name)
		}
	}
}
