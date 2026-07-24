package crawler

import (
	"fmt"
	"strings"

	"golang.org/x/net/html"
)

type Crawler struct {
	client   *HTTPAttacker
	maxDepth int
	visited  map[string]bool
}

type CrawledPage struct {
	URL         string
	Method      string
	StatusCode  int
	Title       string
	Server      string
	BodyPreview string
	Forms       []CrawledForm
	Links       []string
}

type CrawledForm struct {
	Action string
	Method string
	Inputs []CrawledInput
}

func (f CrawledForm) InputNames() []string {
	names := make([]string, len(f.Inputs))
	for i, input := range f.Inputs {
		names[i] = input.Name
	}
	return names
}

type CrawledInput struct {
	Name string
	Type string
}

func NewCrawler(maxDepth int) *Crawler {
	if maxDepth <= 0 {
		maxDepth = 3
	}
	return &Crawler{
		client:   NewHTTPAttacker(),
		maxDepth: maxDepth,
		visited:  make(map[string]bool),
	}
}

func (c *Crawler) Crawl(targetURL string) (*CrawledPage, error) {
	if !strings.HasPrefix(targetURL, "http") {
		targetURL = "http://" + targetURL
	}

	if c.visited[targetURL] {
		return nil, fmt.Errorf("already crawled: %s", targetURL)
	}
	c.visited[targetURL] = true

	resp, err := c.client.SendRaw("GET", targetURL, "", nil)
	if err != nil {
		return nil, fmt.Errorf("crawl %s: %w", targetURL, err)
	}

	page := &CrawledPage{
		URL:         targetURL,
		Method:      "GET",
		StatusCode:  resp.StatusCode,
		Server:      resp.Headers["Server"],
		BodyPreview: truncate(resp.Body, 1000),
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 500 {
		return page, nil
	}

	page.Title = extractTitle(resp.Body)
	page.Forms = extractForms(targetURL, resp.Body)
	page.Links = extractLinks(targetURL, resp.Body)

	return page, nil
}

func (c *Crawler) CrawlDepth(targetURL string) []CrawledPage {
	var pages []CrawledPage
	queue := []string{targetURL}
	c.visited = make(map[string]bool)

	for depth := 0; depth < c.maxDepth && len(queue) > 0; depth++ {
		var nextQueue []string
		for _, url := range queue {
			page, err := c.Crawl(url)
			if err != nil {
				continue
			}
			pages = append(pages, *page)
			for _, link := range page.Links {
				if !c.visited[link] {
					nextQueue = append(nextQueue, link)
				}
			}
		}
		queue = nextQueue
	}

	return pages
}

func extractTitle(body string) string {
	tokenizer := html.NewTokenizer(strings.NewReader(body))
	inTitle := false
	for {
		tt := tokenizer.Next()
		switch tt {
		case html.ErrorToken:
			return ""
		case html.StartTagToken, html.SelfClosingTagToken:
			tagName, _ := tokenizer.TagName()
			if string(tagName) == "title" {
				inTitle = true
			}
		case html.TextToken:
			if inTitle {
				return strings.TrimSpace(string(tokenizer.Text()))
			}
		case html.EndTagToken:
			tagName, _ := tokenizer.TagName()
			if string(tagName) == "title" {
				return ""
			}
		}
	}
}

func extractForms(baseURL string, body string) []CrawledForm {
	var forms []CrawledForm
	tokenizer := html.NewTokenizer(strings.NewReader(body))

	var currentForm *CrawledForm
	inForm := false

	for {
		tt := tokenizer.Next()
		switch tt {
		case html.ErrorToken:
			return forms
		case html.StartTagToken, html.SelfClosingTagToken:
			tagName, _ := tokenizer.TagName()
			switch string(tagName) {
			case "form":
				currentForm = &CrawledForm{Method: "GET"}
				for _, attr := range getAttrs(tokenizer) {
					switch strings.ToLower(attr.Key) {
					case "action":
						currentForm.Action = resolveURL(baseURL, attr.Val)
					case "method":
						currentForm.Method = strings.ToUpper(attr.Val)
					}
				}
				inForm = true
			case "input":
				if inForm && currentForm != nil {
					input := CrawledInput{Name: "", Type: "text"}
					for _, attr := range getAttrs(tokenizer) {
						switch strings.ToLower(attr.Key) {
						case "name":
							input.Name = attr.Val
						case "type":
							input.Type = attr.Val
						}
					}
					if input.Name != "" {
						currentForm.Inputs = append(currentForm.Inputs, input)
					}
				}
			case "textarea":
				if inForm && currentForm != nil {
					input := CrawledInput{Name: "", Type: "textarea"}
					for _, attr := range getAttrs(tokenizer) {
						if strings.ToLower(attr.Key) == "name" {
							input.Name = attr.Val
						}
					}
					if input.Name != "" {
						currentForm.Inputs = append(currentForm.Inputs, input)
					}
				}
			}
		case html.EndTagToken:
			tagName, _ := tokenizer.TagName()
			if string(tagName) == "form" && currentForm != nil {
				forms = append(forms, *currentForm)
				currentForm = nil
				inForm = false
			}
		}
	}
}

type htmlAttr struct{ Key, Val string }

func getAttrs(tokenizer *html.Tokenizer) []htmlAttr {
	var attrs []htmlAttr
	for {
		key, val, more := tokenizer.TagAttr()
		attrs = append(attrs, htmlAttr{string(key), string(val)})
		if !more {
			break
		}
	}
	return attrs
}

func extractLinks(baseURL string, body string) []string {
	var links []string
	seen := make(map[string]bool)
	tokenizer := html.NewTokenizer(strings.NewReader(body))

	for {
		tt := tokenizer.Next()
		if tt == html.ErrorToken {
			break
		}
		if tt != html.StartTagToken && tt != html.SelfClosingTagToken {
			continue
		}
		tagName, _ := tokenizer.TagName()
		switch string(tagName) {
		case "a", "link":
			for _, attr := range getAttrs(tokenizer) {
				if strings.ToLower(attr.Key) == "href" {
					link := resolveURL(baseURL, attr.Val)
					if link != "" && !seen[link] && isValidURL(link) {
						seen[link] = true
						links = append(links, link)
					}
				}
			}
		case "script":
			for _, attr := range getAttrs(tokenizer) {
				if strings.ToLower(attr.Key) == "src" {
					link := resolveURL(baseURL, attr.Val)
					if link != "" && !seen[link] {
						seen[link] = true
						links = append(links, link)
					}
				}
			}
		}
	}
	return links
}

func resolveURL(base, ref string) string {
	if ref == "" || strings.HasPrefix(ref, "#") || strings.HasPrefix(ref, "javascript:") || strings.HasPrefix(ref, "mailto:") {
		return ""
	}
	if strings.HasPrefix(ref, "http://") || strings.HasPrefix(ref, "https://") {
		return ref
	}
	if strings.HasPrefix(ref, "//") {
		return "https:" + ref
	}

	base = strings.TrimSuffix(base, "/")
	base = strings.TrimSuffix(base, "/index.html")
	base = strings.TrimSuffix(base, "/index.php")

	if strings.HasPrefix(ref, "/") {
		baseRoot := base
		if idx := strings.Index(base[8:], "/"); idx >= 0 {
			baseRoot = base[:8+idx]
		}
		return baseRoot + ref
	}
	return base + "/" + ref
}

func isValidURL(url string) bool {
	return strings.HasPrefix(url, "http://") || strings.HasPrefix(url, "https://")
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
