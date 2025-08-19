package dropbox

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"blogsync2/pkg/config"
	"blogsync2/pkg/token"
)

type Auth struct {
	config  config.DropboxConfig
	storage *token.SecureStorage
}

type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token,omitempty"`
	Scope        string `json:"scope,omitempty"`
}

func NewAuth(config config.DropboxConfig, storage *token.SecureStorage) *Auth {
	return &Auth{
		config:  config,
		storage: storage,
	}
}

func (a *Auth) GetAuthorizationURL(state string) (string, error) {
	baseURL := "https://www.dropbox.com/oauth2/authorize"
	params := url.Values{}
	params.Add("response_type", "code")
	params.Add("client_id", a.config.AppKey)
	params.Add("redirect_uri", a.config.RedirectURI)
	params.Add("state", state)
	params.Add("force_reapprove", "false")
	params.Add("disable_signup", "false")

	return baseURL + "?" + params.Encode(), nil
}

func (a *Auth) ExchangeCodeForToken(code string) error {
	data := url.Values{}
	data.Set("code", code)
	data.Set("grant_type", "authorization_code")
	data.Set("client_id", a.config.AppKey)
	data.Set("client_secret", a.config.AppSecret)
	data.Set("redirect_uri", a.config.RedirectURI)

	resp, err := http.PostForm("https://api.dropboxapi.com/oauth2/token", data)
	if err != nil {
		return fmt.Errorf("failed to exchange code for token: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("token exchange failed: %s", string(body))
	}

	var tokenResp TokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return fmt.Errorf("failed to decode token response: %w", err)
	}

	expiresAt := time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second)

	return a.storage.SaveToken(tokenResp.AccessToken, tokenResp.RefreshToken, expiresAt)
}

func (a *Auth) GetValidAccessToken() (string, error) {
	tokenData, err := a.storage.LoadToken()
	if err != nil {
		return "", fmt.Errorf("failed to load token: %w", err)
	}

	// If token expires within 5 minutes, try to refresh it
	if time.Now().Add(5*time.Minute).After(tokenData.ExpiresAt) && tokenData.RefreshToken != "" {
		if err := a.refreshToken(tokenData.RefreshToken); err != nil {
			return "", fmt.Errorf("failed to refresh token: %w", err)
		}
		// Reload token after refresh
		tokenData, err = a.storage.LoadToken()
		if err != nil {
			return "", fmt.Errorf("failed to reload token after refresh: %w", err)
		}
	}

	return tokenData.AccessToken, nil
}

func (a *Auth) HasValidToken() bool {
	return a.storage.HasValidToken()
}

func (a *Auth) refreshToken(refreshToken string) error {
	data := url.Values{}
	data.Set("grant_type", "refresh_token")
	data.Set("refresh_token", refreshToken)
	data.Set("client_id", a.config.AppKey)
	data.Set("client_secret", a.config.AppSecret)

	resp, err := http.PostForm("https://api.dropboxapi.com/oauth2/token", data)
	if err != nil {
		return fmt.Errorf("failed to refresh token: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("token refresh failed: %s", string(body))
	}

	var tokenResp TokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return fmt.Errorf("failed to decode refresh token response: %w", err)
	}

	expiresAt := time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second)

	// If no new refresh token is provided, use the existing one
	newRefreshToken := tokenResp.RefreshToken
	if newRefreshToken == "" {
		newRefreshToken = refreshToken
	}

	return a.storage.SaveToken(tokenResp.AccessToken, newRefreshToken, expiresAt)
}
