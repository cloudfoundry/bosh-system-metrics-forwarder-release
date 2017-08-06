package auth_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"sync"
	"testing"

	. "github.com/onsi/gomega"
	"github.com/pivotal-cf/bosh-system-metrics-forwarder/pkg/auth"
)

func TestAuthServerAddrMakesRequestToInfoServer(t *testing.T) {
	RegisterTestingT(t)

	authURL := "https://some-url.com"
	sis := newSpyInfoServer(validInfoResponse(authURL), 200)
	infoServer := httptest.NewServer(sis)
	defer infoServer.Close()

	addrProvider := auth.NewAddressProvider(infoServer.URL, nil)
	a, err := addrProvider.Addr()
	Expect(err).ToNot(HaveOccurred())
	Expect(a).To(Equal(authURL))
	receivedRequest := sis.lastRequest

	Expect(receivedRequest.Method).To(Equal(http.MethodGet))
	Expect(receivedRequest.URL.Path).To(Equal("/info"))
}

func TestAuthServerAddrCachesAddress(t *testing.T) {
	RegisterTestingT(t)

	authURL := "https://some-url.com"
	sis := newSpyInfoServer(validInfoResponse(authURL), 200)
	infoServer := httptest.NewServer(sis)
	defer infoServer.Close()

	addrProvider := auth.NewAddressProvider(infoServer.URL, nil)
	a1, err := addrProvider.Addr()
	Expect(err).ToNot(HaveOccurred())
	a2, err := addrProvider.Addr()
	Expect(err).ToNot(HaveOccurred())

	Expect(sis.CallCount).To(Equal(1))
	Expect(a1).To(Equal(a2))
	Expect(a1).To(Equal(authURL))
}

func TestAuthServerWithFailingRequest(t *testing.T) {
	RegisterTestingT(t)

	sis := newSpyInfoServer(validInfoResponse(""), 500)
	infoServer := httptest.NewServer(sis)
	defer infoServer.Close()

	addrProvider := auth.NewAddressProvider(infoServer.URL, nil)
	_, err := addrProvider.Addr()

	Expect(err).To(HaveOccurred())
}

func TestAuthServerWithUnparseableInfoRequest(t *testing.T) {
	RegisterTestingT(t)

	sis := newSpyInfoServer("wont-parse-json", 200)
	infoServer := httptest.NewServer(sis)
	defer infoServer.Close()

	addrProvider := auth.NewAddressProvider(infoServer.URL, nil)
	_, err := addrProvider.Addr()

	Expect(err).To(HaveOccurred())
}

func validInfoResponse(authAddr string) string {
	responseTemplate := `{"name":"Bosh Lite Director","uuid":"3f20c4a3-0ef0-4443-8f39-efef33f502a7","version":"262.3.0 (00000000)","user":null,"cpi":"warden_cpi","user_authentication":{"type":"uaa","options":{"url":"%0s","urls":["%0s"]}},"features":{"dns":{"status":false,"extras":{"domain_name":"bosh"}},"compiled_package_cache":{"status":false,"extras":{"provider":null}},"snapshots":{"status":false},"config_server":{"status":false,"extras":{"urls":[]}}}}`
	return fmt.Sprintf(responseTemplate, authAddr)
}

type spyInfoServer struct {
	body      string
	status    int
	CallCount int

	mu          sync.Mutex
	lastRequest *http.Request
}

func newSpyInfoServer(body string, status int) *spyInfoServer {
	return &spyInfoServer{
		body:   body,
		status: status,
	}
}

func (a *spyInfoServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.CallCount++

	httputil.DumpRequest(r, true)
	a.lastRequest = r

	w.WriteHeader(a.status)
	w.Write([]byte(a.body))
}
