package http

import (
	"net/http"
	"time"
)

type Client struct {
	http.Client
}

func NewClient(timeout time.Duration) *Client {
	return &Client{
		Client: http.Client{
			Timeout: timeout,
		},
	}
}
