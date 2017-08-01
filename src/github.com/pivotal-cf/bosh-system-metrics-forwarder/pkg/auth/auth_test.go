package auth_test

import (
	"testing"

	"net/http"
	"net/http/httptest"
	"net/http/httputil"

	"fmt"
	"sync"

	. "github.com/onsi/gomega"
	
	"github.com/pivotal-cf/bosh-system-metrics-forwarder/pkg/auth"
)

func TestGetToken(t *testing.T) {
	RegisterTestingT(t)

	expectedToken := "test-access-token"
	sas := newSpyAuthServer(validAuthResponse(expectedToken), 200)
	testAuthServer := httptest.NewServer(sas)
	defer testAuthServer.Close()

	sis := newSpyInfoServer(validInfoResponse(testAuthServer.URL), 200)
	testInfoServer := httptest.NewServer(sis)
	defer testInfoServer.Close()
	directorAddr := testInfoServer.URL

	clientId := "system-metrics-server"
	clientSecret := "asddnbhkasdhasd123"

	client := auth.New(directorAddr)
	token, err := client.GetToken(clientId, clientSecret)

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

func TestGetToken_withFailingInfoRequest(t *testing.T) {
	RegisterTestingT(t)

	sis := newSpyInfoServer(validInfoResponse(""), 500)
	testInfoServer := httptest.NewServer(sis)
	defer testInfoServer.Close()
	directorAddr := testInfoServer.URL

	client := auth.New(directorAddr)
	_, err := client.GetToken("unused", "unused")

	Expect(err).To(HaveOccurred())
}

func TestGetToken_withUnparseableInfoRequest(t *testing.T) {
	RegisterTestingT(t)

	sis := newSpyInfoServer("wont-parse-json", 200)
	testInfoServer := httptest.NewServer(sis)
	defer testInfoServer.Close()
	directorAddr := testInfoServer.URL

	client := auth.New(directorAddr)
	_, err := client.GetToken("unused", "unused")

	Expect(err).To(HaveOccurred())
}

func TestGetToken_withFailingAuthRequest(t *testing.T) {
	RegisterTestingT(t)

	sas := newSpyAuthServer("unused", 500)
	testAuthServer := httptest.NewServer(sas)
	defer testAuthServer.Close()

	sis := newSpyInfoServer(validInfoResponse(testAuthServer.URL), 200)
	testInfoServer := httptest.NewServer(sis)
	defer testInfoServer.Close()
	directorAddr := testInfoServer.URL

	client := auth.New(directorAddr)
	_, err := client.GetToken("unused", "unused")

	Expect(err).To(HaveOccurred())
}

func TestGetToken_withUnparseableAuthRequest(t *testing.T) {
	RegisterTestingT(t)

	sas := newSpyAuthServer("wont-parse-json", 200)
	testAuthServer := httptest.NewServer(sas)
	defer testAuthServer.Close()

	sis := newSpyInfoServer(validInfoResponse(testAuthServer.URL), 200)
	testInfoServer := httptest.NewServer(sis)
	defer testInfoServer.Close()
	directorAddr := testInfoServer.URL

	client := auth.New(directorAddr)
	_, err := client.GetToken("unused", "unused")

	Expect(err).To(HaveOccurred())
}

func validAuthResponse(token string) string {
	responseTemplate := `{"access_token":"%s","token_type":"bearer","expires_in":43199,"scope":"bosh.system_metrics.read","jti":"c5368216ebeb43f8803f22e8e3f4ce88"}`
	return fmt.Sprintf(responseTemplate, token)
}

func validInfoResponse(authAddr string) string {
	responseTemplate := `{"name":"Bosh Lite Director","uuid":"3f20c4a3-0ef0-4443-8f39-efef33f502a7","version":"262.3.0 (00000000)","user":null,"cpi":"warden_cpi","user_authentication":{"type":"uaa","options":{"url":"%0s","urls":["%0s"]}},"features":{"dns":{"status":false,"extras":{"domain_name":"bosh"}},"compiled_package_cache":{"status":false,"extras":{"provider":null}},"snapshots":{"status":false},"config_server":{"status":false,"extras":{"urls":[]}}}}`
	return fmt.Sprintf(responseTemplate, authAddr)
}

type spyAuthServer struct {
	mu          sync.Mutex
	lastRequest *http.Request
	body        string
	status      int
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

type spyInfoServer struct {
	body   string
	status int
}

func newSpyInfoServer(body string, status int) *spyInfoServer {
	return &spyInfoServer{
		body:   body,
		status: status,
	}
}

func (a *spyInfoServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/info" {
		w.WriteHeader(a.status)
		w.Write([]byte(a.body))
	}
}
