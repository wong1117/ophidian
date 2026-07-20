package github

import (
	"context"
	"net/http"
)

type Client struct {
	token string
	http  *http.Client
}

func NewClient(token string) *Client {
	return &Client{
		token: token,
		http:  &http.Client{},
	}
}

func (c *Client) SearchExploits(ctx context.Context, query string) ([]string, error) {
	return nil, nil
}
