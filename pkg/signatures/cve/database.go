package cve

type Database struct {
	entries map[string]Entry
}

type Entry struct {
	ID          string
	Description string
	CVSS        float64
	CPE         string
	Exploit     bool
}

func NewDatabase() *Database {
	return &Database{
		entries: make(map[string]Entry),
	}
}
