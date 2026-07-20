package shodan

import (
	"context"
	"net/http"
)

type Client struct {
	apiKey string
	http   *http.Client
}

func NewClient(apiKey string) *Client {
	return &Client{
		apiKey: apiKey,
		http:   &http.Client{},
	}
}

func (c *Client) SearchHost(ctx context.Context, ip string) (map[string]interface{}, error) {
	return nil, nil
}
