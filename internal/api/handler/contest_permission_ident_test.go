package handler

import (
	"context"
	"errors"
	"testing"

	"github.com/tahsinarafat/aioj/internal/model"
)

/**
 * Adding a judge or moderator answered 500 for every realistic input.
 *
 * The reported bug: the "Moderators" tab has a box labelled "User ID", so people
 * type a username. That string went straight into
 *
 *     INSERT INTO contest_permissions(contest_id, user_id, ...)
 *
 * where user_id is a uuid column. Postgres rejected it with 22P02 "invalid input
 * syntax for type uuid" and the handler turned that into a bare 500. A UUID that
 * parses but belongs to nobody failed the foreign key the same way — so both
 * plausible inputs (a name, and a stale id) produced an error the operator could
 * not act on.
 *
 * The fix resolves the reference to a real account first, which makes "no such
 * user" a 404. These tests pin the resolution itself, since that is the part that
 * must never regress into passing a raw string through.
 */

const permTestUserID = "0f0e0d0c-0b0a-4908-8706-050403020100"

// refStub answers the two lookups resolveUserRef makes. It models one known
// account, by id and by name, and nothing else.
type refStub struct {
	id       string
	username string
	byIDErr  error
	byNameEr error
	// lookedUp records which path was taken, so a test can prove a UUID is not
	// silently treated as a username.
	lookedUp string
}

func (s *refStub) GetByID(_ context.Context, id string) (*model.User, error) {
	s.lookedUp = "id"
	if s.byIDErr != nil {
		return nil, s.byIDErr
	}
	if id != s.id {
		return nil, nil
	}
	return &model.User{ID: s.id, Username: s.username}, nil
}

func (s *refStub) GetByUsername(_ context.Context, name string) (*model.User, error) {
	s.lookedUp = "username"
	if s.byNameEr != nil {
		return nil, s.byNameEr
	}
	if name != s.username {
		return nil, nil
	}
	return &model.User{ID: s.id, Username: s.username}, nil
}

func TestResolveUserRefAcceptsAUsername(t *testing.T) {
	stub := &refStub{id: permTestUserID, username: "alice"}

	got, err := resolveUserRef(context.Background(), stub, "alice")
	if err != nil {
		t.Fatalf("a known username must resolve, got error: %v", err)
	}
	if got != permTestUserID {
		t.Fatalf("resolved to %q, want the account id %q", got, permTestUserID)
	}
}

// The exact input that produced the 500. If this ever returns the raw string
// again, the uuid column fails and we are back to the reported bug.
func TestResolveUserRefNeverReturnsAnUnknownName(t *testing.T) {
	stub := &refStub{id: permTestUserID, username: "alice"}

	got, err := resolveUserRef(context.Background(), stub, "nobody")
	if err != nil {
		t.Fatalf("an unknown name is not a lookup failure: %v", err)
	}
	if got != "" {
		t.Fatalf("an unknown name must resolve to nothing, got %q", got)
	}
}

func TestResolveUserRefAcceptsAUserID(t *testing.T) {
	stub := &refStub{id: permTestUserID, username: "alice"}

	got, err := resolveUserRef(context.Background(), stub, permTestUserID)
	if err != nil {
		t.Fatalf("a known id must resolve: %v", err)
	}
	if got != permTestUserID {
		t.Fatalf("resolved to %q, want %q", got, permTestUserID)
	}
	if stub.lookedUp != "id" {
		t.Fatalf("a UUID must be looked up by id, took the %q path", stub.lookedUp)
	}
}

// An id that parses but names nobody used to reach the foreign key and 500.
func TestResolveUserRefRejectsAStaleUUID(t *testing.T) {
	stub := &refStub{id: permTestUserID, username: "alice"}

	got, err := resolveUserRef(context.Background(), stub, "11111111-1111-1111-1111-111111111111")
	if err != nil {
		t.Fatalf("a stale id is not a lookup failure: %v", err)
	}
	if got != "" {
		t.Fatalf("a stale id must resolve to nothing, got %q", got)
	}
}

func TestResolveUserRefTrimsWhitespace(t *testing.T) {
	stub := &refStub{id: permTestUserID, username: "alice"}

	got, err := resolveUserRef(context.Background(), stub, "  alice  ")
	if err != nil {
		t.Fatalf("pasted whitespace must not break the lookup: %v", err)
	}
	if got != permTestUserID {
		t.Fatalf("resolved to %q, want %q", got, permTestUserID)
	}
}

// A genuine database failure is not "user not found": it must stay a 500 so an
// outage is not reported as a typo.
func TestResolveUserRefSurfacesLookupFailures(t *testing.T) {
	boom := errors.New("connection refused")
	stub := &refStub{id: permTestUserID, username: "alice", byNameEr: boom}

	if _, err := resolveUserRef(context.Background(), stub, "alice"); !errors.Is(err, boom) {
		t.Fatalf("want the underlying error, got %v", err)
	}
}

// With no user store wired in, an unresolvable reference must still not reach the
// uuid column.
func TestResolveUserRefWithoutAStoreRejectsNonUUIDs(t *testing.T) {
	got, err := resolveUserRef(context.Background(), nil, "alice")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "" {
		t.Fatalf("a non-UUID must not pass through without a store, got %q", got)
	}

	got, err = resolveUserRef(context.Background(), nil, permTestUserID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != permTestUserID {
		t.Fatalf("a UUID should pass through, got %q", got)
	}
}
