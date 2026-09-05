package cmd

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/openclaw/gogcli/internal/app"
)

func TestGmailReadProxyCommandPresentsOnlyTheCallerBearer(t *testing.T) {
	for _, envName := range []string{"GOG_GMAIL_READ_PROXY_URL", "GOG_GMAIL_BASE_URL"} {
		t.Run(envName, func(t *testing.T) {
			requestSeen := make(chan *http.Request, 1)
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				requestSeen <- request.Clone(request.Context())
				writer.Header().Set("Content-Type", "application/json")
				_, _ = writer.Write([]byte(`{"id":"message-1"}`))
			}))
			t.Cleanup(server.Close)
			t.Setenv("GOG_GMAIL_READ_PROXY_URL", "")
			t.Setenv("GOG_GMAIL_BASE_URL", "")
			t.Setenv(envName, server.URL)
			t.Setenv("GOG_ACCESS_TOKEN", "openshell:resolve:gmail-read-proxy:0123")

			result := executeWithTestRuntime(t, []string{
				"--json", "--account", "proxy@localhost", "gmail", "get", "message-1",
			}, &app.Runtime{ServicesManaged: true})
			if result.err != nil {
				t.Fatalf("gmail get: %v\nstderr=%s", result.err, result.stderr)
			}

			request := <-requestSeen
			// No Google OAuth happened (the account has no stored token); the
			// request carries exactly the caller credential from the
			// environment, which the governed proxy authenticates.
			if got := request.Header.Get("Authorization"); got != "Bearer openshell:resolve:gmail-read-proxy:0123" {
				t.Fatalf("Authorization = %q, want the caller bearer", got)
			}
		})
	}
}

func TestGmailReadProxyURLFromEnv(t *testing.T) {
	t.Setenv("GOG_GMAIL_READ_PROXY_URL", "http://127.0.0.1:18079/")
	t.Setenv("GOG_GMAIL_BASE_URL", "http://127.0.0.1:28079/")
	if got := gmailReadProxyURLFromEnv(); got != "http://127.0.0.1:18079/" {
		t.Fatalf("gmailReadProxyURLFromEnv() = %q, want new variable", got)
	}

	t.Setenv("GOG_GMAIL_READ_PROXY_URL", "")
	if got := gmailReadProxyURLFromEnv(); got != "http://127.0.0.1:28079/" {
		t.Fatalf("gmailReadProxyURLFromEnv() = %q, want legacy fallback", got)
	}
}
