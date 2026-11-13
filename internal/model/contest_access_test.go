package model

import "testing"

/**
 * The audience vocabulary, and the one value that has to be right.
 *
 * AccessOpen is load-bearing in a way the other two are not: it is what an
 * ABSENT key resolves to, and every round that predates this feature has an empty
 * access_config. Getting it wrong does not break a setting, it changes what every
 * existing round means — which is why it is asserted directly here rather than
 * only through a handler.
 *
 * The tests below are the three ways this goes wrong: the default silently
 * restricting rounds, a restriction widening sideways, and a value being stored
 * that no surface can honour.
 */

func TestAccessConfigAudienceResolution(t *testing.T) {
	t.Run("an empty config resolves everything to open", func(t *testing.T) {
		// The compatibility guarantee. Empty must mean "what the round did
		// yesterday", and yesterday was the password and group rules and nothing
		// else. Answering participants here would have locked every existing round
		// the moment this shipped.
		var empty AccessConfig
		for _, surface := range AccessSurfaceKeys {
			if got := empty.Audience(surface); got != AccessOpen {
				t.Errorf("empty.Audience(%q) = %q, want %q", surface, got, AccessOpen)
			}
		}
		if got := (AccessConfig{}).Audience(SurfaceStatement); got != AccessOpen {
			t.Errorf("a nil-key map resolved to %q, want %q", got, AccessOpen)
		}
	})

	t.Run("an explicit entry beats the base", func(t *testing.T) {
		cfg := AccessConfig{
			SurfaceParticipate: AccessParticipants,
			SurfaceStandings:   AccessPublic,
		}
		if got := cfg.Audience(SurfaceStandings); got != AccessPublic {
			t.Errorf("standings = %q, want %q — the explicit entry must win", got, AccessPublic)
		}
		if got := cfg.Audience(SurfaceStatement); got != AccessParticipants {
			t.Errorf("statement = %q, want %q — unset surfaces follow the base", got, AccessParticipants)
		}
	})

	t.Run("the base does not widen sideways", func(t *testing.T) {
		// The bug the whole axis exists to prevent: publishing one surface must
		// never publish another. Here participate is PUBLIC and a surface is
		// explicitly participants — the explicit entry must survive.
		cfg := AccessConfig{
			SurfaceParticipate: AccessPublic,
			SurfaceStatement:   AccessParticipants,
		}
		if got := cfg.Audience(SurfaceStatement); got != AccessParticipants {
			t.Errorf("statement = %q, want %q — a public base must not override an "+
				"explicit restriction", got, AccessParticipants)
		}
	})

	t.Run("an invalid audience is ignored rather than honoured", func(t *testing.T) {
		// A row written by hand or by an older build must not be able to make a
		// surface unreachable, and must not be read as a grant either.
		cfg := AccessConfig{SurfaceStatement: "bogus", SurfaceParticipate: AccessParticipants}
		if got := cfg.Audience(SurfaceStatement); got != AccessParticipants {
			t.Errorf("statement = %q, want the base %q — an unknown value must fall "+
				"through to the base, not be obeyed", got, AccessParticipants)
		}
	})

	t.Run("submit cannot require an entrant, by inheritance either", func(t *testing.T) {
		// The deadlock, and the inheritance path is the subtle half: nobody wrote
		// "participants" on submit here, it arrives from the base. Left uncoerced
		// it would block the first submission of every round.
		cfg := AccessConfig{SurfaceParticipate: AccessParticipants}
		if got := cfg.Audience(SurfaceSubmit); got != AccessOpen {
			t.Errorf("submit = %q, want %q — requiring an entrant to submit blocks the "+
				"first submission of every round", got, AccessOpen)
		}
	})
}

func TestNormalizeAccessConfig(t *testing.T) {
	t.Run("drops an unknown surface and an unknown audience", func(t *testing.T) {
		got := NormalizeAccessConfig(AccessConfig{
			SurfaceStatement: AccessPublic,
			"not_a_surface":  AccessPublic,
			SurfaceStandings: "bogus",
		})
		if len(got) != 1 || got[SurfaceStatement] != AccessPublic {
			t.Fatalf("got %v, want only statement=public", got)
		}
	})

	t.Run("coerces participants on submit to open", func(t *testing.T) {
		// Storing the restriction would store a rule the policy refuses to
		// enforce, and the settings row would disagree with the behaviour.
		got := NormalizeAccessConfig(AccessConfig{SurfaceSubmit: AccessParticipants})
		if got[SurfaceSubmit] != AccessOpen {
			t.Fatalf("submit = %q, want %q", got[SurfaceSubmit], AccessOpen)
		}
	})

	t.Run("keeps an explicit open", func(t *testing.T) {
		// Not redundant: open is the only way to opt one surface out of a
		// participants base. Dropping it would silently re-restrict the surface
		// the setter was opting out of.
		got := NormalizeAccessConfig(AccessConfig{
			SurfaceParticipate: AccessParticipants,
			SurfaceStatement:   AccessOpen,
		})
		if got[SurfaceStatement] != AccessOpen {
			t.Fatalf("statement = %q, want %q", got[SurfaceStatement], AccessOpen)
		}
		if got.Audience(SurfaceStatement) != AccessOpen {
			t.Fatal("the explicit open was re-restricted by the participants base")
		}
	})

	t.Run("an empty config normalizes to a non-nil empty map", func(t *testing.T) {
		if got := NormalizeAccessConfig(nil); got == nil {
			t.Fatal("nil came back; a caller indexing the result would panic")
		}
		if got := NormalizeAccessConfig(nil); len(got) != 0 {
			t.Fatalf("got %v, want an empty map", got)
		}
	})
}
