package cli

import (
	"fmt"
	"strings"
	"time"
)

type Color int

const (
	Reset  Color = 0
	Red    Color = 31
	Green  Color = 32
	Yellow Color = 33
	Blue   Color = 34
	Cyan   Color = 36
	White  Color = 37
	Bold   Color = 1
)

func C(c Color, s string) string    { return fmt.Sprintf("\033[%dm%s\033[0m", c, s) }
func BoldText(s string) string       { return C(Bold, s) }
func GreenText(s string) string      { return C(Green, s) }
func RedText(s string) string        { return C(Red, s) }
func YellowText(s string) string     { return C(Yellow, s) }
func CyanText(s string) string       { return C(Cyan, s) }
func BlueText(s string) string       { return C(Blue, s) }

func Box(title string, lines []string, width int) string {
	var sb strings.Builder
	border := strings.Repeat("─", width-2)
	sb.WriteString(fmt.Sprintf("┌%s┐\n", border))
	sb.WriteString(fmt.Sprintf("│ %-*s │\n", width-4, BoldText(title)))
	sb.WriteString(fmt.Sprintf("├%s┤\n", border))
	for _, line := range lines {
		t := line
		if len(t) > width-4 { t = t[:width-4] }
		sb.WriteString(fmt.Sprintf("│ %-*s │\n", width-4, t))
	}
	sb.WriteString(fmt.Sprintf("└%s┘\n", border))
	return sb.String()
}

func ProgressBar(label string, current, total int, width int) string {
	if total <= 0 { total = 1 }
	pct := float64(current) / float64(total)
	filled := int(pct * float64(width))
	empty := width - filled
	bar := strings.Repeat("█", filled) + strings.Repeat("░", empty)
	return fmt.Sprintf("%s [%s] %d/%d (%.0f%%)", label, bar, current, total, pct*100)
}

func Table(headers []string, rows [][]string) string {
	if len(headers) == 0 { return "" }
	colWidths := make([]int, len(headers))
	for i, h := range headers { colWidths[i] = len(h) }
	for _, row := range rows {
		for i, cell := range row {
			if i < len(colWidths) && len(cell) > colWidths[i] { colWidths[i] = len(cell) }
		}
	}
	var sb strings.Builder
	pad := 2
	totalW := 1
	for _, w := range colWidths { totalW += w + pad + 1 }
	sb.WriteString(strings.Repeat("─", totalW) + "\n")
	for i, row := range rows {
		sb.WriteString("│")
		for j, cell := range row {
			w := 0
			if j < len(colWidths) { w = colWidths[j] }
			sb.WriteString(fmt.Sprintf(" %-*s │", w, cell))
		}
		sb.WriteString("\n")
		if i == 0 { sb.WriteString(strings.Repeat("─", totalW) + "\n") }
	}
	sb.WriteString(strings.Repeat("─", totalW) + "\n")
	return sb.String()
}

func Spinner(i int) string {
	frames := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	return frames[i%len(frames)]
}

func DashboardHeader(title string) {
	fmt.Print("\033[2J\033[H")
	fmt.Printf("%s %s", BoldText(CyanText("◆")), BoldText(title))
	fmt.Printf("  %s %s\n", C(White, time.Now().Format("15:04:05")), BlueText("v1.0"))
	fmt.Println(strings.Repeat("─", 80))
}

func StatusLine(status string, healthy bool) string {
	if healthy { return fmt.Sprintf("  %s  %s", GreenText("●"), status) }
	return fmt.Sprintf("  %s  %s", RedText("●"), status)
}

type MetricRow struct {
	Name  string
	Value string
	Color Color
}

func MetricPanel(title string, metrics []MetricRow) string {
	var sb strings.Builder
	sb.WriteString(Box(title, nil, 40))
	for _, m := range metrics {
		sb.WriteString(fmt.Sprintf("  %-20s %s\n", m.Name, C(m.Color, m.Value)))
	}
	return sb.String()
}

func LogLine(level, msg string) string {
	c := White
	switch strings.ToUpper(level) {
	case "ERROR": c = Red
	case "WARN": c = Yellow
	case "INFO": c = Green
	case "DEBUG": c = Cyan
	}
	return fmt.Sprintf("%s [%s] %s", time.Now().Format("15:04:05"), C(c, strings.ToUpper(level)), msg)
}
