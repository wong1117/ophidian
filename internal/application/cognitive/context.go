package cognitive

type CrawledPage struct {
	URL         string
	Method      string
	StatusCode  int
	Title       string
	Server      string
	BodyPreview string
	Forms       []DiscoveredForm
	Links       []string
}

type DiscoveredForm struct {
	Action string
	Method string
	Inputs []FormInput
}

func (f DiscoveredForm) InputNames() []string {
	names := make([]string, len(f.Inputs))
	for i, input := range f.Inputs {
		names[i] = input.Name
	}
	return names
}

type FormInput struct {
	Name string
	Type string
}

type ActionAttempt struct {
	Method      string
	URL         string
	Payload     string
	StatusCode  int
	BodyPreview string
	Success     bool
	Analysis    string
}

type AIDecision struct {
	Reasoning          string
	Action             string
	TargetURL          string
	Method             string
	PayloadType        string
	Payload            string
	Confidence         float64
	ExpectedIndicators []string
}

type MatchedCVE struct {
	CVE         string
	Description string
	CVSS        float64
	Severity    string
	Platform    string
	MatchReason string
}

type SessionState struct {
	Active     bool
	Cookies    []string
	SourceURL  string
	Indicators []string
}

type StolenSession struct {
	Token      string
	TokenType  string
	SourceURL  string
	CapturedAt string
}

type AttackContext struct {
	Target                  string
	TechStack               []string
	CrawledPages            []CrawledPage
	Attempts                []ActionAttempt
	MatchingCVEs            []MatchedCVE
	Session                 *SessionState
	StolenSessions          []StolenSession
	VulnerabilityIndicators []string
	SecurityHeaders         []string
	MissingHeaders          []string
	SSLInfo                 string
	Subdomains              []string
	PreviousDecisions       []AIDecision
}

func NewAttackContext(target string) *AttackContext {
	return &AttackContext{Target: target}
}

func (c *AttackContext) AddPage(page CrawledPage) {
	c.CrawledPages = append(c.CrawledPages, page)
	for _, form := range page.Forms {
		for _, link := range page.Links {
			_ = form
			_ = link
		}
	}
}

func (c *AttackContext) AddAttempt(a ActionAttempt) {
	c.Attempts = append(c.Attempts, a)
}

func (c *AttackContext) AddDecision(d AIDecision) {
	c.PreviousDecisions = append(c.PreviousDecisions, d)
}

func (c *AttackContext) AddTech(t string) {
	for _, existing := range c.TechStack {
		if existing == t {
			return
		}
	}
	c.TechStack = append(c.TechStack, t)
}

func (c *AttackContext) TotalAttempts() int { return len(c.Attempts) }
func (c *AttackContext) SuccessCount() int {
	count := 0
	for _, a := range c.Attempts {
		if a.Success {
			count++
		}
	}
	return count
}
