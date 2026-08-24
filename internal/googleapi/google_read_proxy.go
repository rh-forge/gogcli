package googleapi

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
)

var (
	errGoogleReadProxyEndpoint    = errors.New("google read proxy blocks request outside its configured endpoint")
	errGoogleReadProxyCredentials = errors.New("google read proxy blocks credentials in the request URL")
	errGoogleReadProxyAbsoluteURL = errors.New("GOG_GMAIL_READ_PROXY_URL must be an absolute URL")
	errGoogleReadProxyScheme      = errors.New("GOG_GMAIL_READ_PROXY_URL must use HTTP or HTTPS")
	errGoogleReadProxyLoopback    = errors.New("GOG_GMAIL_READ_PROXY_URL must use a loopback IP address")
	errGoogleReadProxyOrigin      = errors.New("GOG_GMAIL_READ_PROXY_URL must be an origin without credentials, path, query, or fragment")
)

var googleReadProxyCredentialHeaders = []string{
	"Authorization",
	"Cookie",
	"Proxy-Authorization",
	"X-Goog-Api-Key",
}

type googleReadProxyTransport struct {
	base         http.RoundTripper
	origin       string
	allowRequest func(*http.Request) error
}

func (t googleReadProxyTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	if request.URL.Scheme+"://"+request.URL.Host != t.origin {
		return nil, errGoogleReadProxyEndpoint
	}

	if request.URL.Query().Has("access_token") || request.URL.Query().Has("key") {
		return nil, errGoogleReadProxyCredentials
	}

	if err := t.allowRequest(request); err != nil {
		return nil, err
	}

	forwarded := request.Clone(request.Context())
	forwarded.Header = request.Header.Clone()

	for _, name := range googleReadProxyCredentialHeaders {
		forwarded.Header.Del(name)
	}

	response, err := t.base.RoundTrip(forwarded)
	if err != nil {
		return nil, fmt.Errorf("google read proxy request: %w", err)
	}

	return response, nil
}

func normalizeGoogleReadProxyURL(value string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(value))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", errGoogleReadProxyAbsoluteURL
	}

	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", errGoogleReadProxyScheme
	}

	ip := net.ParseIP(parsed.Hostname())
	if ip == nil || !ip.IsLoopback() {
		return "", errGoogleReadProxyLoopback
	}

	if parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" || (parsed.Path != "" && parsed.Path != "/") {
		return "", errGoogleReadProxyOrigin
	}

	return parsed.Scheme + "://" + parsed.Host + "/", nil
}

func newGoogleReadProxyClient(endpoint string, allowRequest func(*http.Request) error) (*http.Client, string, error) {
	normalized, err := normalizeGoogleReadProxyURL(endpoint)
	if err != nil {
		return nil, "", err
	}

	parsed, err := url.Parse(normalized)
	if err != nil {
		return nil, "", fmt.Errorf("parse normalized Google read proxy URL: %w", err)
	}

	client := &http.Client{
		Transport: googleReadProxyTransport{
			base:         newBaseTransport(),
			origin:       parsed.Scheme + "://" + parsed.Host,
			allowRequest: allowRequest,
		},
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	return client, normalized, nil
}
