package censys

import (
	"context"
	"net/http"
)

type Client struct {
	apiID    string
	apiSecret string
	http     *http.Client
}

func NewClient(apiID, apiSecret string) *Client {
	return &Client{
		apiID:    apiID,
		apiSecret: apiSecret,
		http:     &http.Client{},
	}
}

func (c *Client) SearchHost(ctx context.Context, ip string) (map[string]interface{}, error) {
	return nil, nil
}
