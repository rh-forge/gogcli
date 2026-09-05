package googleapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
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
		{name: "hostname", value: "http://localhost:8080/", want: "http://localhost:8080/"},
		{name: "governed base", value: "https://host.containers.internal:18081", want: "https://host.containers.internal:18081/"},
		{name: "remote", value: "https://proxy.example/", want: "https://proxy.example/"},
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

func TestGmailReadProxyPresentsTheCallerBearer(t *testing.T) {
	t.Parallel()

	requestSeen := make(chan *http.Request, 1)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requestSeen <- request.Clone(request.Context())

		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"id":"message-1"}`))
	}))
	t.Cleanup(server.Close)

	service, err := newGmailReadProxy(context.Background(), server.URL, " openshell:resolve:gmail-read-proxy:0123 ")
	if err != nil {
		t.Fatalf("newGmailReadProxy: %v", err)
	}
	call := service.Users.Messages.Get("me", "message-1")
	call.Header().Set("Authorization", "Bearer set-by-the-process")

	if _, err := call.Do(); err != nil {
		t.Fatalf("Messages.Get: %v", err)
	}

	// The caller credential toward the proxy is what the proxy sees, whatever
	// the process tried to attach.
	if got := (<-requestSeen).Header.Get("Authorization"); got != "Bearer openshell:resolve:gmail-read-proxy:0123" {
		t.Errorf("Authorization = %q, want the caller bearer", got)
	}
}

func TestGmailReadProxyRequiresTheCallerBearer(t *testing.T) {
	t.Parallel()

	for _, bearer := range []string{"", "   "} {
		if _, err := newGmailReadProxy(context.Background(), "http://127.0.0.1:18081/", bearer); err == nil {
			t.Fatalf("newGmailReadProxy(bearer=%q) succeeded, want error", bearer)
		}
	}
}
