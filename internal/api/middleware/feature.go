package middleware

import (
	"encoding/json"
	"net/http"

	"github.com/tahsinarafat/aioj/internal/auth"
	"github.com/tahsinarafat/aioj/internal/feature"
)

// FeatureGate rejects requests for a feature the admin has restricted.
//
// The gate resolves the caller itself rather than trusting the route to have
// run an auth middleware first. Most gated groups mix public and authenticated
// routes (problems, contests, gym, …), and mounting an optional-auth middleware
// just to feed the gate would change what every handler in those groups sees.
// Validating the bearer token here keeps the blast radius to the gate: when the
// route already authenticated, the claims are on the context and the token is
// not read twice.
//
// Denials are 403 rather than 404. Hiding the route would make a
// misconfiguration indistinguishable from a typo in a link, and the point of
// the gate is a policy decision the caller is allowed to understand. The body
// carries a machine-readable reason so the SPA can say "sign in" instead of
// "forbidden".
func FeatureGate(jwtManager *auth.JWTManager, resolver *feature.Resolver, key string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			principal := PrincipalFromRequest(r)
			if principal.Anonymous && jwtManager != nil {
				if token := bearerToken(r); token != "" {
					if claims, err := jwtManager.ValidateToken(token); err == nil {
						principal = principalFor(claims)
					}
				}
			}

			decision := resolver.Decide(r.Context(), key, principal)
			if decision.Allowed {
				next.ServeHTTP(w, r)
				return
			}

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"error":   featureDenialMessage(decision.Reason),
				"feature": key,
				"reason":  decision.Reason,
				// The admin's own explanation, when they wrote one.
				"message": decision.Message,
			})
		})
	}
}

// PrincipalFromRequest builds the feature principal for the caller. Anonymous
// requests (no claims) are represented rather than rejected, because some
// features are legitimately open to visitors.
func PrincipalFromRequest(r *http.Request) feature.Principal {
	if claims := GetUserClaims(r); claims != nil {
		return principalFor(claims)
	}
	return feature.Principal{Anonymous: true}
}

func principalFor(claims *auth.Claims) feature.Principal {
	return feature.Principal{UserID: claims.UserID, Role: claims.Role}
}

func featureDenialMessage(reason string) string {
	switch reason {
	case feature.ReasonDisabled:
		return "this feature is disabled"
	case feature.ReasonLoginRequired:
		return "sign in to use this feature"
	case feature.ReasonRole:
		return "your account is not allowed to use this feature"
	case feature.ReasonRating:
		return "this feature is not available at your rating"
	default:
		return "this feature is not available"
	}
}
