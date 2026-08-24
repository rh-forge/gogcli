package googleapi

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"google.golang.org/api/gmail/v1"
	"google.golang.org/api/option"
)

var (
	errGmailReadProxyMethod   = errors.New("gmail read proxy blocks request method")
	errGmailReadProxyEndpoint = errors.New("gmail read proxy blocks request outside its configured endpoint")
)

func allowGmailReadProxyRequest(request *http.Request) error {
	if request.Method != http.MethodGet && request.Method != http.MethodHead {
		return fmt.Errorf("%w: %s", errGmailReadProxyMethod, request.Method)
	}

	if !strings.HasPrefix(request.URL.Path, "/gmail/v1/users/me/") {
		return errGmailReadProxyEndpoint
	}

	return nil
}

func newGmailReadProxy(ctx context.Context, endpoint string) (*gmail.Service, error) {
	client, normalized, err := newGoogleReadProxyClient(endpoint, allowGmailReadProxyRequest)
	if err != nil {
		return nil, err
	}

	service, err := gmail.NewService(
		ctx,
		option.WithEndpoint(normalized),
		option.WithHTTPClient(client),
		option.WithoutAuthentication(),
	)
	if err != nil {
		return nil, fmt.Errorf("create Gmail read proxy service: %w", err)
	}

	return service, nil
}
