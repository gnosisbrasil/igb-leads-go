package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

// GoogleProfile carries the fields the Node passport flow used:
// id, displayName, email and avatar.
type GoogleProfile struct {
	ID        string
	Email     string
	Name      string
	FirstName string
	LastName  string
	AvatarURL string
}

// GoogleOAuth replaces passport-google-oauth20 for the login and
// account-linking flows.
type GoogleOAuth struct {
	cfg *oauth2.Config
}

func NewGoogleOAuth(clientID, clientSecret, redirectURL string) *GoogleOAuth {
	return &GoogleOAuth{
		cfg: &oauth2.Config{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			RedirectURL:  redirectURL,
			Scopes:       []string{"email", "profile"},
			Endpoint:     google.Endpoint,
		},
	}
}

// AuthURL builds the consent URL (login flow, no state like passport).
func (g *GoogleOAuth) AuthURL() string {
	return g.cfg.AuthCodeURL("", oauth2.AccessTypeOffline)
}

// ConnectURL builds the consent URL for account linking.
func (g *GoogleOAuth) ConnectURL(userID string) string {
	return g.cfg.AuthCodeURL("connect:"+userID, oauth2.AccessTypeOffline, oauth2.SetAuthURLParam("prompt", "consent"))
}

// ExchangeProfile trades the code for tokens and fetches the userinfo.
func (g *GoogleOAuth) ExchangeProfile(ctx context.Context, code string) (*GoogleProfile, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	tok, err := g.cfg.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("google exchange: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://www.googleapis.com/oauth2/v3/userinfo", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+tok.AccessToken)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("google userinfo: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("google userinfo: status %d", resp.StatusCode)
	}
	var u struct {
		Sub        string `json:"sub"`
		Email      string `json:"email"`
		Name       string `json:"name"`
		GivenName  string `json:"given_name"`
		FamilyName string `json:"family_name"`
		Picture    string `json:"picture"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&u); err != nil {
		return nil, fmt.Errorf("google userinfo: %w", err)
	}
	return &GoogleProfile{
		ID: u.Sub, Email: u.Email, Name: u.Name,
		FirstName: u.GivenName, LastName: u.FamilyName, AvatarURL: u.Picture,
	}, nil
}
