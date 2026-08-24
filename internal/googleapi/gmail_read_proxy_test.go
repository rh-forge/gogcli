package googleapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"google.golang.org/api/gmail/v1"
)

func TestNormalizeGoogleReadProxyURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		value   string
		want    string
		wantErr bool
	}{
		{name: "IPv4", value: "http://127.0.0.1:8080", want: "http://127.0.0.1:8080/"},
		{name: "IPv6", value: "https://[::1]:8443/", want: "https://[::1]:8443/"},
		{name: "hostname", value: "http://localhost:8080/", wantErr: true},
		{name: "remote", value: "https://proxy.example/", wantErr: true},
		{name: "path", value: "http://127.0.0.1:8080/proxy", wantErr: true},
		{name: "credentials", value: "http://user:pass@127.0.0.1:8080/", wantErr: true},
		{name: "query", value: "http://127.0.0.1:8080/?token=value", wantErr: true},
		{name: "fragment", value: "http://127.0.0.1:8080/#fragment", wantErr: true},
		{name: "scheme", value: "ftp://127.0.0.1/", wantErr: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			got, err := normalizeGoogleReadProxyURL(test.value)
			if test.wantErr {
				if err == nil {
					t.Fatalf("normalizeGoogleReadProxyURL(%q) = %q, want error", test.value, got)
				}

				return
			}

			if err != nil {
				t.Fatalf("normalizeGoogleReadProxyURL(%q): %v", test.value, err)
			}

			if got != test.want {
				t.Fatalf("normalizeGoogleReadProxyURL(%q) = %q, want %q", test.value, got, test.want)
			}
		})
	}
}

func TestGmailReadProxySendsNoCredentials(t *testing.T) {
	t.Parallel()

	requestSeen := make(chan *http.Request, 1)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requestSeen <- request.Clone(request.Context())

		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"id":"message-1"}`))
	}))
	t.Cleanup(server.Close)

	service, err := newGmailReadProxy(context.Background(), server.URL)
	if err != nil {
		t.Fatalf("newGmailReadProxy: %v", err)
	}
	call := service.Users.Messages.Get("me", "message-1")
	call.Header().Set("Authorization", "Bearer must-not-leak")
	call.Header().Set("Cookie", "session=must-not-leak")
	call.Header().Set("Proxy-Authorization", "Basic must-not-leak")
	call.Header().Set("X-Goog-Api-Key", "must-not-leak")

	if _, err := call.Do(); err != nil {
		t.Fatalf("Messages.Get: %v", err)
	}

	request := <-requestSeen
	for _, name := range googleReadProxyCredentialHeaders {
		if got := request.Header.Get(name); got != "" {
			t.Errorf("%s = %q, want empty", name, got)
		}
	}
}

func TestGmailReadProxyRefusesRedirects(t *testing.T) {
	t.Parallel()

	var redirectedRequests atomic.Int32
	requestSeen := make(chan *http.Request, 1)
	redirected := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		redirectedRequests.Add(1)
	}))
	t.Cleanup(redirected.Close)

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requestSeen <- request.Clone(request.Context())

		writer.Header().Set("Location", redirected.URL)
		writer.WriteHeader(http.StatusFound)
	}))
	t.Cleanup(server.Close)

	service, err := newGmailReadProxy(context.Background(), server.URL)
	if err != nil {
		t.Fatalf("newGmailReadProxy: %v", err)
	}
	call := service.Users.Messages.Get("me", "message-1")
	call.Header().Set("Authorization", "Bearer must-not-leak")

	if _, err := call.Do(); err == nil {
		t.Fatal("Messages.Get succeeded through a redirect")
	}

	if got := (<-requestSeen).Header.Get("Authorization"); got != "" {
		t.Fatalf("redirecting endpoint received Authorization %q, want empty", got)
	}

	if got := redirectedRequests.Load(); got != 0 {
		t.Fatalf("redirect target received %d requests, want 0", got)
	}
}

func TestGmailReadProxyBlocksWritesBeforeNetwork(t *testing.T) {
	t.Parallel()

	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		requests.Add(1)
	}))
	t.Cleanup(server.Close)

	service, err := newGmailReadProxy(context.Background(), server.URL)
	if err != nil {
		t.Fatalf("newGmailReadProxy: %v", err)
	}

	err = service.Users.Messages.BatchDelete("me", &gmail.BatchDeleteMessagesRequest{Ids: []string{"message-1"}}).Do()
	if err == nil {
		t.Fatal("BatchDelete succeeded through a read proxy")
	}

	if got := requests.Load(); got != 0 {
		t.Fatalf("proxy received %d write requests, want 0", got)
	}
}
