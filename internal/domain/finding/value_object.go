package finding

type CVSS float64

func (c CVSS) IsValid() bool {
	return c >= 0 && c <= 10
}

func (c CVSS) Severity() string {
	switch {
	case c >= 9.0:
		return "CRITICAL"
	case c >= 7.0:
		return "HIGH"
	case c >= 4.0:
		return "MEDIUM"
	case c > 0:
		return "LOW"
	default:
		return "INFO"
	}
}

type CWE string

type ConfidenceLevel string

const (
	ConfidenceLow      ConfidenceLevel = "LOW"
	ConfidenceMedium   ConfidenceLevel = "MEDIUM"
	ConfidenceHigh     ConfidenceLevel = "HIGH"
	ConfidenceConfirmed ConfidenceLevel = "CONFIRMED"
)
