package model

import "testing"

// The per-artifact scope is policy, and the whole point of it is that the
// artifact which INVENTS a problem asks for more than the ones that rewrite part
// of an existing one. Each case below would have failed before the split, when
// every artifact accepted every field.
func TestArtifactScopeGatesByWhatTheArtifactProduces(t *testing.T) {
	cases := map[string]struct {
		accepts    []BriefField
		rejects    []BriefField
		offeredFor bool
	}{
		// The one scope that invents a problem needs the inputs that define one.
		"full_problem": {
			accepts:    []BriefField{FieldTopics, FieldRating, FieldRatingRange, FieldTestCaseCount, FieldHasSubtasks},
			offeredFor: false,
		},
		// A statement rewrite takes its difficulty and topic from the problem.
		"statement": {
			accepts:    []BriefField{FieldAudienceLevel, FieldStoryTone, FieldForbiddenTechniques},
			rejects:    []BriefField{FieldTopics, FieldRating, FieldTestCaseCount, FieldHasSubtasks},
			offeredFor: true,
		},
		"tests": {
			accepts:    []BriefField{FieldTestCaseCount, FieldHasSubtasks, FieldConstraintGuidance},
			rejects:    []BriefField{FieldTopics, FieldRating, FieldAudienceLevel, FieldStoryTone},
			offeredFor: true,
		},
		"solution": {
			accepts:    []BriefField{FieldForbiddenTechniques, FieldConstraintGuidance},
			rejects:    []BriefField{FieldTopics, FieldRating, FieldTestCaseCount, FieldAudienceLevel},
			offeredFor: true,
		},
		// The reported case: an editorial needs no topic, rating or test count.
		"editorial": {
			accepts:    []BriefField{FieldAudienceLevel, FieldStoryTone},
			rejects:    []BriefField{FieldTopics, FieldRating, FieldTestCaseCount, FieldHasSubtasks, FieldFormatProfile},
			offeredFor: true,
		},
	}

	for id, tc := range cases {
		spec, ok := LookupArtifact(id)
		if !ok {
			t.Fatalf("%s: not in the catalog", id)
		}
		if spec.OfferedForExisting != tc.offeredFor {
			t.Errorf("%s: OfferedForExisting = %v, want %v", id, spec.OfferedForExisting, tc.offeredFor)
		}
		for _, f := range tc.accepts {
			if !spec.Accepts(f) {
				t.Errorf("%s: should accept %s", id, f)
			}
		}
		for _, f := range tc.rejects {
			if spec.Accepts(f) {
				t.Errorf("%s: should NOT accept %s", id, f)
			}
		}
	}
}

// Only the artifact that invents a problem requires anything, and only topics.
func TestOnlyTheInventingArtifactRequiresTopics(t *testing.T) {
	full, _ := LookupArtifact(string(ArtifactFullProblem))
	if !full.Requires(FieldTopics) {
		t.Error("the full package must require topics: without them there is nothing to build around")
	}
	if full.Requires(FieldRating) {
		t.Error("a rating has a default, so it is not required even of the full package")
	}
	for _, id := range []ArtifactType{ArtifactStatement, ArtifactTests, ArtifactSolution, ArtifactEditorial} {
		spec, _ := LookupArtifact(string(id))
		if spec.Requires(FieldTopics) {
			t.Errorf("%s must not require topics: it works from the existing problem", id)
		}
	}
}

// Every artifact must accept the free-form steering field, or a setter would
// have no way to say anything the form does not offer.
func TestEveryArtifactAcceptsTheAdditionalPrompt(t *testing.T) {
	for _, spec := range ArtifactSpecs() {
		if !spec.Accepts(FieldAdditionalPrompt) {
			t.Errorf("%s does not accept %s", spec.ID, FieldAdditionalPrompt)
		}
	}
}

// Each artifact's field list must be free of duplicates: the same field listed
// twice renders its control twice, and `join` exists to prevent that.
func TestArtifactFieldsHaveNoDuplicates(t *testing.T) {
	for _, spec := range ArtifactSpecs() {
		seen := map[BriefField]bool{}
		for _, f := range spec.Fields {
			if seen[f] {
				t.Errorf("%s lists %s twice", spec.ID, f)
			}
			seen[f] = true
		}
	}
}

// An unknown artifact resolves to the WIDEST scope, never a narrower one:
// dropping input the caller supplied is the silent failure this table exists to
// prevent. Validation rejects the unknown type separately.
func TestUnknownArtifactFallsBackToTheWidestScope(t *testing.T) {
	if IsArtifact("nonsense") {
		t.Error("IsArtifact must reject an unknown type")
	}
	if _, ok := LookupArtifact("nonsense"); ok {
		t.Error("LookupArtifact must not resolve an unknown type")
	}

	fields := ArtifactFields("nonsense")
	full := ArtifactFields(string(ArtifactFullProblem))
	if len(fields) != len(full) {
		t.Fatalf("fallback has %d fields, want the full package's %d", len(fields), len(full))
	}
	for i := range fields {
		if fields[i] != full[i] {
			t.Errorf("field %d: got %s, want %s", i, fields[i], full[i])
		}
	}
}

// ArtifactFields must hand back a copy: a caller that appends to the slice it
// was given would otherwise mutate the catalog for every other caller.
func TestArtifactFieldsReturnsACopy(t *testing.T) {
	first := ArtifactFields(string(ArtifactEditorial))
	first[0] = FieldTopics
	second := ArtifactFields(string(ArtifactEditorial))
	if second[0] == FieldTopics {
		t.Fatal("ArtifactFields leaked the catalog: a caller mutated the shared slice")
	}
}

func TestCatalogCoversEveryDeclaredArtifact(t *testing.T) {
	declared := []ArtifactType{ArtifactFullProblem, ArtifactStatement, ArtifactTests, ArtifactSolution, ArtifactEditorial}
	if len(ArtifactSpecs()) != len(declared) {
		t.Fatalf("catalog has %d entries, want %d", len(ArtifactSpecs()), len(declared))
	}
	for _, id := range declared {
		if _, ok := LookupArtifact(string(id)); !ok {
			t.Errorf("%s is declared but not in the catalog", id)
		}
	}
}
