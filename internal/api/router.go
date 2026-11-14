package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/tahsinarafat/aioj/internal/api/middleware"
	"github.com/tahsinarafat/aioj/internal/auth"
	"github.com/tahsinarafat/aioj/internal/feature"
)

// NewRouter builds the HTTP handler for the API.
//
// This function does two things: assemble the base middleware chain, and call
// each domain's mount function in the order that matters for chi's routing.
// Every route registration lives in its own mount<Domain> method — see
// router_builder.go for the shared value they receive and router_*.go for the
// domains themselves.
//
// The order of the mount calls is part of the contract. chi matches static
// segments before patterns within a router, but two mounts that both claim a
// prefix are resolved by registration order, so moving a call can silently
// re-point a route. TestRouteTable pins the resulting table.
func NewRouter(d Deps, jwtManager *auth.JWTManager) http.Handler {
	b := &routerBuilder{
		Deps: &d,
		jwt:  jwtManager,
		rl:   middleware.NewRateLimiter(),
	}

	r := chi.NewRouter()
	r.Use(b.base()...)

	b.mountPublic(r)
	b.mountAuth(r)
	b.mountUsers(r)
	b.mountProblems(r)
	b.mountPublicAPIv1(r)
	b.mountSubmissions(r)
	b.mountContests(r)
	b.mountContestExtras(r)
	b.mountVJudge(r)
	b.mountAdmin(r)
	b.mountMiscTopLevel(r)
	b.mountVirtualGymHacks(r)
	b.mountSocial(r)
	b.mountContent(r)
	b.mountOrgsTraining(r)
	b.mountAI(r)

	// Serve uploads through the media handler, not http.FileServer: FileServer
	// lists the directory for /media/ and serves whatever happens to be in it.
	if b.Optional.Media != nil {
		r.With(b.gate(feature.KeyMediaUploads)).Handle("/media/*", http.HandlerFunc(b.Optional.Media.Serve))
	}

	return r
}
