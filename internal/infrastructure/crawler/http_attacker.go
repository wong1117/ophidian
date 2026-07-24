package crawler

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"time"

	"github.com/ophidian/ophidian/internal/application/opsec"
)

type HTTPAttacker struct {
	client       *http.Client
	History      []HTTPTransaction
	jitterEngine *opsec.JitterEngine
}

type HTTPTransaction struct {
	Iteration int
	Request   HTTPRequestDetail
	Response  HTTPResponseDetail
	Success   bool
	Error     string
}

type HTTPRequestDetail struct {
	URL     string
	Method  string
	Headers map[string]string
	Body    string
}

type HTTPResponseDetail struct {
	StatusCode int
	Headers    map[string]string
	Body       string
	Duration   time.Duration
	Length     int
}

func NewHTTPAttacker() *HTTPAttacker {
	jar, _ := cookiejar.New(nil)
	return &HTTPAttacker{
		client: &http.Client{
			Jar:     jar,
			Timeout: 30 * time.Second,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) >= 5 {
					return http.ErrUseLastResponse
				}
				return nil
			},
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
				DialContext:     (&net.Dialer{Timeout: 10 * time.Second}).DialContext,
			},
		},
		jitterEngine: opsec.NewJitterEngine(opsec.JitterConfig{
			Enabled:         true,
			MinDelay:        500 * time.Millisecond,
			MaxDelay:        3 * time.Second,
			SleepMasking:    true,
			SleepMaskType:   opsec.SleepMaskCalculations,
			TimingRandomize: true,
		}),
	}
}

func (a *HTTPAttacker) Execute(req HTTPRequestDetail) (*HTTPResponseDetail, error) {
	a.jitterEngine.Sleep(context.Background())
	startedAt := time.Now()

	httpReq, err := http.NewRequest(req.Method, req.URL, strings.NewReader(req.Body))
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}

	httpReq.Header.Set("User-Agent", "Mozilla/5.0 (compatible; Ophidian3/1.0)")
	httpReq.Header.Set("Accept", "text/html,application/json,*/*")

	for key, value := range req.Headers {
		httpReq.Header.Set(key, value)
	}

	resp, err := a.client.Do(httpReq)
	elapsed := time.Since(startedAt)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 32768))
	body := strings.TrimSpace(string(bodyBytes))

	headers := make(map[string]string)
	for key := range resp.Header {
		headers[key] = resp.Header.Get(key)
	}

	detail := &HTTPResponseDetail{
		StatusCode: resp.StatusCode,
		Headers:    headers,
		Body:       body,
		Duration:   elapsed,
		Length:     len(bodyBytes),
	}

	a.History = append(a.History, HTTPTransaction{
		Request:  req,
		Response: *detail,
		Success:  resp.StatusCode >= 200 && resp.StatusCode < 400,
	})

	return detail, nil
}

func (a *HTTPAttacker) SubmitForm(url string, method string, params map[string]string) (*HTTPResponseDetail, error) {
	var bodyStr string
	if method == "" {
		method = "POST"
	}

	header := map[string]string{"Content-Type": "application/x-www-form-urlencoded"}
	var parts []string
	for key, value := range params {
		parts = append(parts, fmt.Sprintf("%s=%s", key, value))
	}
	bodyStr = strings.Join(parts, "&")

	return a.Execute(HTTPRequestDetail{
		URL:     url,
		Method:  method,
		Headers: header,
		Body:    bodyStr,
	})
}

func (a *HTTPAttacker) SendRaw(method, url, body string, headers map[string]string) (*HTTPResponseDetail, error) {
	return a.Execute(HTTPRequestDetail{
		URL:     url,
		Method:  method,
		Headers: headers,
		Body:    body,
	})
}

func (a *HTTPAttacker) GetHistory() []HTTPTransaction {
	return a.History
}

func (a *HTTPAttacker) GetCookies(urlStr string) []*http.Cookie {
	u, err := url.Parse(urlStr)
	if err != nil {
		return nil
	}
	return a.client.Jar.Cookies(u)
}

func (a *HTTPAttacker) HasSessionFor(urlStr string) bool {
	return len(a.GetCookies(urlStr)) > 0
}
