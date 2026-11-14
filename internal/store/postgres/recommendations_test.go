package postgres

import (
	"context"
	"testing"
)

// The recommendation POLICY is tested in internal/recommend, where it lives. What
// is asserted here is the store's own contract: it must refuse to query without a
// connection rather than panic.
func TestGetRecommendations_NilDB_ReturnsError(t *testing.T) {
	s := &ProblemStore{db: nil}
	_, err := s.GetRecommendations(context.Background(), "user-1", 1200)
	if err == nil {
		t.Fatal("expected an error when the store has no connection, got nil")
	}
}
