package auth_test

import (
	"errors"
	"testing"

	"net/http"
	"net/http/httptest"
	"net/http/httputil"

	"fmt"
	"sync"

	. "github.com/onsi/gomega"

	"github.com/cloudfoundry/bosh-system-metrics-forwarder/pkg/auth"
)

func TestToken(t *testing.T) {
	RegisterTestingT(t)

	expectedToken := "test-access-token"
	sas := newSpyAuthServer(validAuthResponse(expectedToken), 200)
	testAuthServer := httptest.NewServer(sas)
	defer testAuthServer.Close()

	clientId := "system-metrics-server"
	clientSecret := "asddnbhkasdhasd123"
	addresser := newSpyAddresser(testAuthServer.URL, nil)

	client := auth.New(addresser, clientId, clientSecret, nil)
	token, err := client.Token()

	Expect(err).ToNot(HaveOccurred())
	Expect(token).To(Equal(expectedToken))

	receivedRequest := sas.lastRequest
	Expect(receivedRequest.ParseForm()).ToNot(HaveOccurred())

	Expect(receivedRequest.Method).To(Equal(http.MethodPost))
	Expect(receivedRequest.URL.Path).To(Equal("/oauth/token"))
	Expect(receivedRequest.Form.Get("response_type")).To(Equal("token"))
	Expect(receivedRequest.Form.Get("grant_type")).To(Equal("client_credentials"))
	Expect(receivedRequest.Form.Get("client_id")).To(Equal(clientId))
	Expect(receivedRequest.Form.Get("client_secret")).To(Equal(clientSecret))
}

func TestTokenWithFailingAddresser(t *testing.T) {
	RegisterTestingT(t)

	addresser := newSpyAddresser("", errors.New("some-err"))

	client := auth.New(addresser, "unused", "unused", nil)
	_, err := client.Token()

	Expect(err).To(HaveOccurred())
}

func TestTokenWithFailingAuthRequest(t *testing.T) {
	RegisterTestingT(t)

	sas := newSpyAuthServer("unused", 500)
	testAuthServer := httptest.NewServer(sas)
	defer testAuthServer.Close()

	addresser := newSpyAddresser(testAuthServer.URL, nil)

	client := auth.New(addresser, "unused", "unused", nil)
	_, err := client.Token()

	Expect(err).To(HaveOccurred())
}

func TestTokenWithUnparseableAuthRequest(t *testing.T) {
	RegisterTestingT(t)

	sas := newSpyAuthServer("wont-parse-json", 200)
	testAuthServer := httptest.NewServer(sas)
	defer testAuthServer.Close()

	addresser := newSpyAddresser(testAuthServer.URL, nil)

	client := auth.New(addresser, "unused", "unused", nil)
	_, err := client.Token()

	Expect(err).To(HaveOccurred())
}

func validAuthResponse(token string) string {
	responseTemplate := `{"access_token":"%s","token_type":"bearer","expires_in":43199,"scope":"bosh.system_metrics.read","jti":"c5368216ebeb43f8803f22e8e3f4ce88"}`
	return fmt.Sprintf(responseTemplate, token)
}

type spyAuthServer struct {
	body   string
	status int

	mu          sync.Mutex
	lastRequest *http.Request
}

func newSpyAuthServer(body string, status int) *spyAuthServer {
	return &spyAuthServer{
		body:   body,
		status: status,
	}
}

func (a *spyAuthServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	a.mu.Lock()
	defer a.mu.Unlock()

	httputil.DumpRequest(r, true)
	a.lastRequest = r

	w.WriteHeader(a.status)
	w.Write([]byte(a.body))
}

type spyAddresser struct {
	url string
	err error
}

func newSpyAddresser(url string, err error) *spyAddresser {
	return &spyAddresser{
		url: url,
		err: err,
	}
}

func (a *spyAddresser) Addr() (string, error) {
	return a.url, a.err
}
