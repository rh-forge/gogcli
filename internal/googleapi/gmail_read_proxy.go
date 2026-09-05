package googleapi

import (
	"context"
	"fmt"

	"google.golang.org/api/gmail/v1"
	"google.golang.org/api/option"
)

func newGmailReadProxy(ctx context.Context, endpoint, bearer string) (*gmail.Service, error) {
	client, normalized, err := newGoogleReadProxyClient(endpoint, bearer)
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
