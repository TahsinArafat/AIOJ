package handler

import "net/http"

// CSRFToken hands the client a csrf cookie.
//
// The double-submit cookie is minted by the CSRF middleware on any safe
// request, so this handler does no work of its own — the value comes from the
// middleware inspecting the response. It exists as an explicit, cheap endpoint
// because the SPA is served by nginx: a cold visitor landing straight on
// /register or /login has made no /api/* call yet, so no cookie exists, and
// the registration POST was rejected with a raw "csrf cookie missing". The
// client now primes here before any unauthenticated write.
//
// Safe method, public, returns 204.
func (h *AuthHandler) CSRFToken(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusNoContent)
}
