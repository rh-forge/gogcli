package googleapi

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

var (
	errGoogleReadProxyAbsoluteURL = errors.New("GOG_GMAIL_READ_PROXY_URL must be an absolute URL")
	errGoogleReadProxyScheme      = errors.New("GOG_GMAIL_READ_PROXY_URL must use HTTP or HTTPS")
	errGoogleReadProxyOrigin      = errors.New("GOG_GMAIL_READ_PROXY_URL must be an origin without credentials, path, query, or fragment")
	errGoogleReadProxyBearer      = errors.New("GOG_GMAIL_READ_PROXY_URL requires GOG_ACCESS_TOKEN (the caller credential toward the read proxy)")
)

// googleReadProxyTransport presents the caller's credential toward the read
// proxy on every request. That credential (GOG_ACCESS_TOKEN) is not a Google
// token: in a governed sandbox it is an OpenShell provider placeholder the
// supervisor substitutes in transit, otherwise the proxy's static bearer.
// The proxy authenticates the caller with it and holds the real Google
// credential itself.
//
// Nothing else is enforced here. Which destinations, methods and paths a
// request may reach is the read proxy's and the sandbox network policy's
// decision, applied to every process; this client holds no Google
// credential that could leak, and the HTTP client already drops
// Authorization on cross-host redirects.
type googleReadProxyTransport struct {
	base   http.RoundTripper
	bearer string
}

func (t googleReadProxyTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	forwarded := request.Clone(request.Context())
	forwarded.Header = request.Header.Clone()
	forwarded.Header.Set("Authorization", "Bearer "+t.bearer)

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

	if parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" || (parsed.Path != "" && parsed.Path != "/") {
		return "", errGoogleReadProxyOrigin
	}

	return parsed.Scheme + "://" + parsed.Host + "/", nil
}

func newGoogleReadProxyClient(endpoint, bearer string) (*http.Client, string, error) {
	normalized, err := normalizeGoogleReadProxyURL(endpoint)
	if err != nil {
		return nil, "", err
	}

	if strings.TrimSpace(bearer) == "" {
		return nil, "", errGoogleReadProxyBearer
	}

	client := &http.Client{
		Transport: googleReadProxyTransport{
			base:   newBaseTransport(),
			bearer: strings.TrimSpace(bearer),
		},
	}

	return client, normalized, nil
}
