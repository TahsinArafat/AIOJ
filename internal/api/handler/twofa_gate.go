package handler

import (
	"context"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/tahsinarafat/aioj/internal/auth"
	"github.com/tahsinarafat/aioj/internal/model"
	"github.com/tahsinarafat/aioj/internal/store"
)

// twoFAChallengeTTL is how long the second-factor challenge stays valid.
const twoFAChallengeTTL = 5 * time.Minute

// writeTwoFAChallenge writes a 2FA challenge response and reports whether it
// did so; callers must return without issuing tokens when it reports true.
//
// Every login entry point has to run this. The onsite contest login used to
// mint access and refresh tokens directly, so an account with TOTP enabled
// could be signed into with the password alone through that route.
func writeTwoFAChallenge(ctx context.Context, w http.ResponseWriter, jwt *auth.JWTManager, secrets store.TOTPSecretStore, userID string) bool {
	if jwt == nil || secrets == nil || userID == "" {
		return false
	}
	secret, err := secrets.Get(ctx, userID)
	if err != nil || secret == nil || !secret.Enabled {
		return false
	}
	challenge, err := jwt.GenerateChallengeToken(userID, uuid.NewString(), twoFAChallengeTTL)
	if err != nil {
		http.Error(w, "failed to generate challenge", http.StatusInternalServerError)
		return true
	}
	respondJSON(w, http.StatusOK, model.AuthResponse{
		Requires2FA: true,
		ChallengeID: challenge,
	})
	return true
}
