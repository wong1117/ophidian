package ollama

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

type Client struct {
	baseURL    string
	httpClient *http.Client
	model      string
}

type Config struct {
	Host   string
	Port   int
	Model  string
	Timeout int
}

func NewClient(cfg Config) *Client {
	return &Client{
		baseURL:    fmt.Sprintf("http://%s:%d", cfg.Host, cfg.Port),
		httpClient: &http.Client{},
		model:      cfg.Model,
	}
}

type GenerateRequest struct {
	Model     string   `json:"model"`
	Prompt    string   `json:"prompt"`
	System    string   `json:"system,omitempty"`
	Stream    bool     `json:"stream"`
	Options   Options  `json:"options,omitempty"`
}

type Options struct {
	Temperature float64 `json:"temperature"`
	TopP        float64 `json:"top_p"`
	TopK        int     `json:"top_k"`
	MaxTokens   int     `json:"num_predict"`
}

type GenerateResponse struct {
	Response string `json:"response"`
	Done     bool   `json:"done"`
}

func (c *Client) Generate(ctx context.Context, prompt string) (string, error) {
	req := GenerateRequest{
		Model:  c.model,
		Prompt: prompt,
		Stream: false,
	}
	resp, err := c.doRequest(ctx, "/api/generate", req)
	if err != nil {
		return "", err
	}
	return resp.Response, nil
}

func (c *Client) doRequest(ctx context.Context, endpoint string, req interface{}) (*GenerateResponse, error) {
	return nil, nil
}
