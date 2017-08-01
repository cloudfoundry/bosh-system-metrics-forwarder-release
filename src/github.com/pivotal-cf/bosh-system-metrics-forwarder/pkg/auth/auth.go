package auth

import (
	"encoding/json"
	"fmt"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type Auth struct {
	httpClient *http.Client
	infoAddr   string
}

func New(infoAddr string) *Auth {
	return &Auth{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		infoAddr: infoAddr,
	}
}

type infoResponse struct {
	UserAuthentication struct {
		AuthType string `json:"type"`
		Options struct {
			Url string `json:"url"`
		} `json:"options"`
	} `json:"user_authentication"`
}

type authResponse struct {
	AccessToken string `json:"access_token"`
}

func (a *Auth) getServerAddr() (string, error) {
	resp, err := a.httpClient.Get(fmt.Sprintf("%s/info", a.infoAddr))
	if err != nil {
		return "", err
	}

	if resp.StatusCode != http.StatusOK {
		return "", errors.New(fmt.Sprintf("info endpoint returned bad status code: %d", resp.StatusCode))
	}

	defer resp.Body.Close()

	var info infoResponse
	decoder := json.NewDecoder(resp.Body)
	err = decoder.Decode(&info)
	if err != nil {
		return "", err
	}

	return info.UserAuthentication.Options.Url, nil
}

func (a *Auth) GetToken(clientId, clientSecret string) (string, error) {
	addr, err := a.getServerAddr()
	if err != nil {
		return "", err
	}

	form := url.Values{}
	form.Set("client_id", clientId)
	form.Set("client_secret", clientSecret)
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
		return "", errors.New(fmt.Sprintf("auth endpoint returned bad status code: %d", resp.StatusCode))
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
