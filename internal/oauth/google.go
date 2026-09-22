package oauth

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

// GoogleConfig allows overriding userinfo URL for tests.
type GoogleConfig struct {
	UserInfoURL string
}

type GoogleProvider struct {
	cfg *oauth2.Config
	gc  GoogleConfig
}

func NewGoogleProvider(c GoogleConfig) *GoogleProvider {
	cfg := &oauth2.Config{
		Endpoint: google.Endpoint,
		Scopes:   []string{"openid", "email", "profile"},
	}
	if c.UserInfoURL == "" {
		c.UserInfoURL = "https://openidconnect.googleapis.com/v1/userinfo"
	}
	return &GoogleProvider{cfg: cfg, gc: c}
}

func (p *GoogleProvider) Name() string           { return "google" }
func (p *GoogleProvider) Config() *oauth2.Config { return p.cfg }

func (p *GoogleProvider) FetchUser(_ context.Context, client *http.Client) (*UserInfo, error) {
	var u struct {
		Sub           string `json:"sub"`
		Email         string `json:"email"`
		EmailVerified bool   `json:"email_verified"`
		Name          string `json:"name"`
		Picture       string `json:"picture"`
	}
	if err := getJSON(client, p.gc.UserInfoURL, &u); err != nil {
		return nil, err
	}
	return &UserInfo{
		ProviderUserID: u.Sub,
		Username:       sanitizeUsername(deriveUsername(u.Email, u.Name)),
		Email:          u.Email,
		AvatarURL:      u.Picture,
	}, nil
}

func deriveUsername(email, name string) string {
	if name != "" {
		return name
	}
	if i := strings.IndexByte(email, '@'); i > 0 {
		return email[:i]
	}
	return fmt.Sprintf("user_%d", time.Now().Unix())
}
