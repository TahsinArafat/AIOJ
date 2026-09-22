package oauth

import (
	"context"
	"fmt"
	"net/http"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/github"
)

// GitHubConfig allows overriding API URLs for tests.
type GitHubConfig struct {
	UserURL   string
	EmailsURL string
}

type GitHubProvider struct {
	cfg *oauth2.Config
	ghc GitHubConfig
}

func NewGitHubProvider(c GitHubConfig) *GitHubProvider {
	cfg := &oauth2.Config{Endpoint: github.Endpoint, Scopes: []string{"user:email"}}
	if c.UserURL == "" {
		c.UserURL = "https://api.github.com/user"
		c.EmailsURL = "https://api.github.com/user/emails"
	}
	return &GitHubProvider{cfg: cfg, ghc: c}
}

func (p *GitHubProvider) Name() string           { return "github" }
func (p *GitHubProvider) Config() *oauth2.Config { return p.cfg }

func (p *GitHubProvider) FetchUser(_ context.Context, client *http.Client) (*UserInfo, error) {
	var u struct {
		ID        int    `json:"id"`
		Login     string `json:"login"`
		Name      string `json:"name"`
		AvatarURL string `json:"avatar_url"`
	}
	if err := getJSON(client, p.ghc.UserURL, &u); err != nil {
		return nil, err
	}
	var emails []struct {
		Email    string `json:"email"`
		Primary  bool   `json:"primary"`
		Verified bool   `json:"verified"`
	}
	if err := getJSON(client, p.ghc.EmailsURL, &emails); err != nil {
		return nil, err
	}
	email := ""
	for _, e := range emails {
		if e.Primary && e.Verified {
			email = e.Email
			break
		}
	}
	if email == "" && len(emails) > 0 {
		email = emails[0].Email
	}
	return &UserInfo{
		ProviderUserID: fmt.Sprintf("%d", u.ID),
		Username:       sanitizeUsername(u.Login),
		Email:          email,
		AvatarURL:      u.AvatarURL,
	}, nil
}

func getJSON(client *http.Client, url string, out any) error {
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("oauth: %s returned %d", url, resp.StatusCode)
	}
	return jsonDecode(resp.Body, out)
}
