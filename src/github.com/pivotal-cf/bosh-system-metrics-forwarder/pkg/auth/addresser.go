package auth

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type infoResponse struct {
	UserAuthentication struct {
		AuthType string `json:"type"`
		Options  struct {
			Url string `json:"url"`
		} `json:"options"`
	} `json:"user_authentication"`
}

type AddressProvider struct {
	infoURL    string
	httpClient *http.Client
	authAddr   string
}

func NewAddressProvider(url string, c *tls.Config) *AddressProvider {
	return &AddressProvider{
		infoURL: url,
		httpClient: &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: c,
			},
			Timeout: 30 * time.Second,
		},
	}
}

func (a *AddressProvider) Addr() (string, error) {
	if a.authAddr != "" {
		return a.authAddr, nil
	}
	resp, err := a.httpClient.Get(fmt.Sprintf("%s/info", a.infoURL))
	if err != nil {
		return "", err
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("info endpoint returned bad status code: %d", resp.StatusCode)
	}
	defer resp.Body.Close()

	var info infoResponse
	decoder := json.NewDecoder(resp.Body)
	err = decoder.Decode(&info)
	if err != nil {
		return "", err
	}
	a.authAddr = info.UserAuthentication.Options.Url

	return a.authAddr, nil
}
