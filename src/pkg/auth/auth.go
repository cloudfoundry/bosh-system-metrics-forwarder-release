package auth

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type addresser interface {
	Addr() (string, error)
}

type Auth struct {
	httpClient   *http.Client
	addrProvider addresser
	authAddr     string
	clientID     string
	clientSecret string
}

// New returns a new Auth.
func New(a addresser, clientID string, clientSecret string, tlsConfig *tls.Config) *Auth {
	return &Auth{
		httpClient: &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: tlsConfig,
			},
			Timeout: 30 * time.Second,
		},
		addrProvider: a,
		clientID:     clientID,
		clientSecret: clientSecret,
	}
}

type authResponse struct {
	AccessToken string `json:"access_token"`
}

// Token returns the token provided by the auth endpoint.
// It returns an error if the  request fails or the response cannot be decoded.
func (a *Auth) Token() (string, error) {
	addr, err := a.addrProvider.Addr()
	if err != nil {
		return "", err
	}

	form := url.Values{}
	form.Set("client_id", a.clientID)
	form.Set("client_secret", a.clientSecret)
	form.Set("grant_type", "client_credentials")
	form.Set("response_type", "token")

	resp, err := a.httpClient.Post(
		fmt.Sprintf("%s/oauth/token", addr),
		"application/x-www-form-urlencoded",
		strings.NewReader(form.Encode()),
	)
	if err != nil {
		return "", err
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("auth endpoint returned bad status code: %d", resp.StatusCode)
	}
	defer resp.Body.Close()

	var auth authResponse
	decoder := json.NewDecoder(resp.Body)
	err = decoder.Decode(&auth)
	if err != nil {
		return "", err
	}

	return auth.AccessToken, nil
}
