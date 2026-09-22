package oauth

import (
	"context"
	"net/http"

	"golang.org/x/oauth2"
)

// UserInfo is the normalized identity returned by an OAuth provider.
type UserInfo struct {
	ProviderUserID string
	Username       string
	Email          string
	AvatarURL      string
	Raw            map[string]any
}

// Provider abstracts an OAuth2 identity provider.
type Provider interface {
	Name() string
	Config() *oauth2.Config
	FetchUser(ctx context.Context, client *http.Client) (*UserInfo, error)
}
