package runner

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/ophidian/ophidian/internal/domain/execution"
)

type HTTPProbePlugin struct {
	client         *http.Client
	defaultTimeout time.Duration
}

func NewHTTPProbePlugin() *HTTPProbePlugin {
	return &HTTPProbePlugin{
		client: &http.Client{
			Timeout: 30 * time.Second,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) >= 3 {
					return http.ErrUseLastResponse
				}
				return nil
			},
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
				DialContext:     (&net.Dialer{Timeout: 10 * time.Second}).DialContext,
			},
		},
		defaultTimeout: 2 * time.Minute,
	}
}

func (p *HTTPProbePlugin) Name() string {
	return "http-probe"
}

func (p *HTTPProbePlugin) Run(ctx context.Context, req execution.ToolRequest) (*execution.ToolResult, error) {
	target := strings.TrimSpace(req.Target)
	if target == "" {
		return nil, fmt.Errorf("http-probe plugin: target is empty")
	}

	timeout := req.Options.Timeout
	if timeout <= 0 {
		timeout = p.defaultTimeout
	}
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	urls := []string{"http://" + target, "https://" + target}
	if strings.HasPrefix(target, "http://") || strings.HasPrefix(target, "https://") {
		urls = []string{target}
	}

	startedAt := time.Now().UTC()
	var artifacts []execution.ToolArtifact
	var summaries []string

	for _, url := range urls {
		artifact, summary := p.probeURL(ctx, url)
		if artifact != nil {
			artifacts = append(artifacts, *artifact)
		}
		summaries = append(summaries, summary)
	}
	completedAt := time.Now().UTC()

	if len(summaries) == 0 {
		summaries = append(summaries, fmt.Sprintf("http-probe: could not reach %s", target))
	}

	return &execution.ToolResult{
		Evidence:  strings.Join(summaries, "\n"),
		Artifacts: artifacts,
		Metadata: map[string]string{
			"tool":         p.Name(),
			"target":       target,
			"duration":     completedAt.Sub(startedAt).String(),
			"completed_at": completedAt.Format(time.RFC3339),
		},
		Statistics: execution.ToolStatistics{
			TargetsScanned: 1,
			Findings:       len(artifacts),
		},
	}, nil
}

func (p *HTTPProbePlugin) probeURL(ctx context.Context, url string) (*execution.ToolArtifact, string) {
	httpReq, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Sprintf("%s: request error: %v", url, err)
	}
	httpReq.Header.Set("User-Agent", "Mozilla/5.0 (compatible; Ophidian3/1.0; +http://ophidian.local)")
	httpReq.Header.Set("Accept", "text/html,application/json,*/*")

	startedAt := time.Now().UTC()
	resp, err := p.client.Do(httpReq)
	elapsed := time.Since(startedAt)

	if err != nil {
		return nil, fmt.Sprintf("%s: error (%s): %v", url, elapsed.Round(time.Millisecond), err)
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 8192))
	bodyPreview := strings.TrimSpace(string(bodyBytes))
	if len(bodyPreview) > 2048 {
		bodyPreview = bodyPreview[:2048] + "..."
	}

	reqHeaders := formatRequestHeaders(httpReq)
	respHeaders := formatResponseHeaders(resp)

	sslInfo := p.getSSLInfo(url)

	summary := fmt.Sprintf("%s [%d] %s (%s) %d bytes",
		url, resp.StatusCode, http.StatusText(resp.StatusCode),
		elapsed.Round(time.Millisecond), len(bodyBytes))

	metadata := map[string]string{
		"url":              url,
		"status_code":      strconv.Itoa(resp.StatusCode),
		"status_text":      http.StatusText(resp.StatusCode),
		"server":           resp.Header.Get("Server"),
		"content_type":     resp.Header.Get("Content-Type"),
		"content_length":   strconv.Itoa(len(bodyBytes)),
		"response_time":    elapsed.Round(time.Millisecond).String(),
		"request_headers":  reqHeaders,
		"response_headers": respHeaders,
		"body_preview":     bodyPreview,
	}

	missingHeaders := p.checkSecurityHeaders(resp)
	if len(missingHeaders) > 0 {
		metadata["missing_security_headers"] = strings.Join(missingHeaders, ", ")
	}

	if sslInfo != "" {
		metadata["ssl_info"] = sslInfo
	}

	artifact := &execution.ToolArtifact{
		Name:     url,
		Type:     "http_transaction",
		Metadata: metadata,
	}

	return artifact, summary
}

func (p *HTTPProbePlugin) checkSecurityHeaders(resp *http.Response) []string {
	required := map[string]string{
		"Content-Security-Policy":   "CSP",
		"X-Content-Type-Options":    "XCTO",
		"X-Frame-Options":           "XFO",
		"Strict-Transport-Security": "HSTS",
		"X-XSS-Protection":          "XXSSP",
	}

	var missing []string
	for header, short := range required {
		if resp.Header.Get(header) == "" {
			missing = append(missing, short)
		}
	}
	return missing
}

func (p *HTTPProbePlugin) getSSLInfo(url string) string {
	if !strings.HasPrefix(url, "https://") {
		return ""
	}
	host := strings.TrimPrefix(url, "https://")
	if idx := strings.Index(host, "/"); idx >= 0 {
		host = host[:idx]
	}
	if idx := strings.Index(host, ":"); idx >= 0 {
		host = host[:idx]
	}

	conn, err := tls.DialWithDialer(
		&net.Dialer{Timeout: 5 * time.Second},
		"tcp",
		host+":443",
		&tls.Config{InsecureSkipVerify: true},
	)
	if err != nil {
		return fmt.Sprintf("SSL: error: %v", err)
	}
	defer conn.Close()

	certs := conn.ConnectionState().PeerCertificates
	if len(certs) == 0 {
		return ""
	}
	cert := certs[0]
	parts := []string{
		fmt.Sprintf("Subject=%s", cert.Subject.CommonName),
		fmt.Sprintf("Issuer=%s", cert.Issuer.CommonName),
		fmt.Sprintf("NotAfter=%s", cert.NotAfter.Format("2006-01-02")),
	}
	return strings.Join(parts, ", ")
}

func formatRequestHeaders(req *http.Request) string {
	var parts []string
	parts = append(parts, fmt.Sprintf("%s %s %s", req.Method, req.URL.RequestURI(), req.Proto))
	parts = append(parts, fmt.Sprintf("Host: %s", req.Host))
	for key, vals := range req.Header {
		for _, v := range vals {
			parts = append(parts, fmt.Sprintf("%s: %s", key, v))
		}
	}
	return strings.Join(parts, "\n")
}

func formatResponseHeaders(resp *http.Response) string {
	var parts []string
	parts = append(parts, fmt.Sprintf("%s %s", resp.Proto, resp.Status))
	for key, vals := range resp.Header {
		for _, v := range vals {
			parts = append(parts, fmt.Sprintf("%s: %s", key, v))
		}
	}
	return strings.Join(parts, "\n")
}
