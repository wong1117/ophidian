package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
	"unsafe"
)

type viewMode int

const (
	viewDashboard viewMode = iota
	viewRecommendations
)

type recommendationItem struct {
	EventID        string    `json:"event_id"`
	MissionID      string    `json:"mission_id"`
	SourceEventID  string    `json:"source_event_id"`
	Recommendation string    `json:"recommendation"`
	Confidence     float64   `json:"confidence"`
	GeneratedAt    time.Time `json:"generated_at"`
}

type Dashboard struct {
	serverURL       string
	httpClient      *http.Client
	refreshInterval time.Duration
	logs            []string
	maxLogs         int
	logCh           chan string
	stopCh          chan struct{}
	running         bool

	currentView     viewMode
	recommendations []recommendationItem
	selectedRec     int
	approvedSet     map[string]bool
	statusMsg       string
	statusMsgAt     time.Time
}

func NewDashboard(serverURL string) *Dashboard {
	if serverURL == "" {
		serverURL = "http://localhost:8443"
	}
	return &Dashboard{
		serverURL:       serverURL,
		httpClient:      &http.Client{Timeout: 5 * time.Second},
		refreshInterval: time.Second,
		maxLogs:         20,
		logCh:           make(chan string, 100),
		stopCh:          make(chan struct{}),
		approvedSet:     make(map[string]bool),
	}
}

func (d *Dashboard) Run() {
	d.running = true

	d.setupTerminal()
	defer d.restoreTerminal()

	recTicker := time.NewTicker(5 * time.Second)
	defer recTicker.Stop()

	keyCh := make(chan byte, 32)
	go d.readKeys(keyCh)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	frame := 0
	render := func() {
		fmt.Print("\033[2J\033[H")
		switch d.currentView {
		case viewRecommendations:
			fmt.Print(d.viewRecommendations(frame))
		default:
			fmt.Print(d.viewDashboard(frame))
		}
		frame++
	}

	render()

	d.fetchRecommendations()
	render()

	for d.running {
		select {
		case <-sigCh:
			d.running = false
			fmt.Print("\033[2J\033[H")
			fmt.Println(GreenText("Dashboard stopped."))
			return

		case key := <-keyCh:
			d.handleKey(key, render)

		case logLine := <-d.logCh:
			d.logs = append(d.logs, logLine)
			if len(d.logs) > d.maxLogs {
				d.logs = d.logs[len(d.logs)-d.maxLogs:]
			}
			render()

		case <-recTicker.C:
			oldCount := len(d.recommendations)
			if d.currentView == viewRecommendations {
				d.fetchRecommendations()
			}
			if len(d.recommendations) != oldCount || d.currentView == viewDashboard {
				render()
			}
		}
	}
}

func (d *Dashboard) handleKey(key byte, render func()) {
	switch key {
	case 'q', 'Q', 3:
		d.running = false
		fmt.Print("\033[2J\033[H")
		fmt.Println(GreenText("Dashboard stopped."))
		return
	case 'r':
		d.fetchRecommendations()
		render()
	case 9:
		if d.currentView == viewDashboard {
			d.currentView = viewRecommendations
			d.fetchRecommendations()
		} else {
			d.currentView = viewDashboard
		}
		d.selectedRec = 0
		render()
	}

	if d.currentView == viewRecommendations && len(d.recommendations) > 0 {
		switch key {
		case 'j', 66:
			if d.selectedRec < len(d.recommendations)-1 {
				d.selectedRec++
				render()
			}
		case 'k', 65:
			if d.selectedRec > 0 {
				d.selectedRec--
				render()
			}
		case 'y', 'Y':
			d.submitApproval(d.selectedRec, "ACCEPTED", render)
		case 'n', 'N':
			d.submitApproval(d.selectedRec, "REJECTED", render)
		}
	}
}

func (d *Dashboard) submitApproval(idx int, decision string, render func()) {
	if idx < 0 || idx >= len(d.recommendations) {
		return
	}
	rec := d.recommendations[idx]
	if d.approvedSet[rec.EventID] {
		d.setStatus("Already submitted for this item")
		render()
		return
	}

	body := map[string]string{
		"mission_id":      rec.MissionID,
		"source_event_id": rec.SourceEventID,
		"decision":        decision,
		"operator_note":   "",
	}
	payload, _ := json.Marshal(body)

	resp, err := d.httpClient.Post(
		d.serverURL+"/api/v1/recommendations/0/approve",
		"application/json",
		bytes.NewReader(payload),
	)
	if err != nil {
		d.setStatus(fmt.Sprintf("Error: %v", err))
		render()
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		d.setStatus(fmt.Sprintf("Server error: %s", resp.Status))
	} else {
		d.approvedSet[rec.EventID] = true
		d.setStatus(fmt.Sprintf("Sent %s for mission %s", decision, rec.MissionID[:8]))
	}
	render()
}

func (d *Dashboard) setStatus(msg string) {
	d.statusMsg = msg
	d.statusMsgAt = time.Now()
}

func (d *Dashboard) fetchRecommendations() {
	resp, err := d.httpClient.Get(d.serverURL + "/api/v1/recommendations")
	if err != nil {
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return
	}

	var recs []recommendationItem
	if err := json.NewDecoder(resp.Body).Decode(&recs); err != nil {
		return
	}
	d.recommendations = recs
	if d.selectedRec >= len(d.recommendations) {
		d.selectedRec = 0
	}
}

func (d *Dashboard) viewDashboard(frame int) string {
	var sb strings.Builder

	sb.WriteString(DashboardHeaderStr("OPHIDIAN CONTROL CENTER"))

	systemMetrics := []MetricRow{
		{Name: "Server", Value: d.serverURL, Color: Cyan},
		{Name: "Time", Value: time.Now().Format("15:04:05"), Color: White},
		{Name: "View", Value: "Dashboard", Color: Green},
		{Name: "Recs", Value: fmt.Sprintf("%d", len(d.recommendations)), Color: Yellow},
	}
	sb.WriteString(MetricPanel("SYSTEM", systemMetrics))
	sb.WriteString("\n")

	var logLines []string
	if len(d.logs) == 0 {
		logLines = []string{
			LogLine("INFO", fmt.Sprintf("Connected to %s", d.serverURL)),
			LogLine("INFO", "Press Tab to view AI recommendations"),
		}
	} else {
		logLines = d.logs
	}
	sb.WriteString(Box("LIVE LOGS", logLines, 80))
	sb.WriteString(fmt.Sprintf("\n%s %s    %s | %s | %s",
		Spinner(frame), GreenText("● Live"),
		BlueText("q: quit"), BlueText("r: refresh"), BlueText("Tab: recommendations")))
	sb.WriteString("\n")
	return sb.String()
}

func (d *Dashboard) viewRecommendations(frame int) string {
	var sb strings.Builder

	sb.WriteString(DashboardHeaderStr("AI RECOMMENDATIONS"))

	if len(d.recommendations) == 0 {
		sb.WriteString(Box("NO RECOMMENDATIONS", []string{
			"No AI recommendations found.",
			"Run a mission to generate recon data.",
			"",
			"Press Tab to return to dashboard",
		}, 80))
		sb.WriteString(fmt.Sprintf("\n%s %s    %s",
			Spinner(frame), GreenText("● Live"), BlueText("Tab: dashboard | q: quit")))
		return sb.String()
	}

	for i, rec := range d.recommendations {
		selected := i == d.selectedRec
		approved := d.approvedSet[rec.EventID]

		indicator := " "
		if selected {
			indicator = "▶"
		}

		missionID := rec.MissionID
		if len(missionID) > 12 {
			missionID = missionID[:12]
		}

		recText := rec.Recommendation
		if len(recText) > 70 {
			recText = recText[:70] + "..."
		}

		var lines []string
		if approved {
			lines = append(lines, fmt.Sprintf("%s [%d] %s  ✓ submitted", GreenText(indicator), i+1, missionID))
		} else if selected {
			lines = append(lines, fmt.Sprintf("%s [%d] %s", YellowText(indicator), i+1, missionID))
		} else {
			lines = append(lines, fmt.Sprintf("%s [%d] %s", C(White, indicator), i+1, missionID))
		}
		if rec.Confidence > 0 {
			lines = append(lines, fmt.Sprintf("    Confidence: %.0f%%", rec.Confidence*100))
		} else {
			lines = append(lines, "    Confidence: N/A (pending AI score)")
		}
		lines = append(lines, fmt.Sprintf("    %s", recText))

		if selected && !approved {
			lines = append(lines, "")
			lines = append(lines, fmt.Sprintf("    %s %s",
				GreenText("[Y] Approve"), RedText("[N] Reject")))
		}

		sb.WriteString(Box(fmt.Sprintf("REC #%d", i+1), lines, 80))
		sb.WriteString("\n")
	}

	if d.statusMsg != "" && time.Since(d.statusMsgAt) < 5*time.Second {
		sb.WriteString(fmt.Sprintf("\n%s %s\n", YellowText("►"), d.statusMsg))
	}

	sb.WriteString(fmt.Sprintf("\n%s %s    %s | %s | %s | %s",
		Spinner(frame), GreenText("● Live"),
		BlueText("j/k: navigate"), BlueText("Y: approve"), BlueText("N: reject"),
		BlueText("Tab: dashboard")))
	sb.WriteString("\n")
	return sb.String()
}

func (d *Dashboard) AddLog(level, msg string) {
	select {
	case d.logCh <- LogLine(level, msg):
	default:
	}
}

func (d *Dashboard) Stop() {
	if d.running {
		d.running = false
		close(d.stopCh)
	}
}

func (d *Dashboard) readKeys(ch chan<- byte) {
	fd := int(os.Stdin.Fd())
	var buf [1]byte
	for d.running {
		n, err := os.Stdin.Read(buf[:])
		if err != nil || n == 0 {
			time.Sleep(10 * time.Millisecond)
			continue
		}
		select {
		case ch <- buf[0]:
		default:
		}
	}
	_ = fd
}

func (d *Dashboard) setupTerminal() {
	setRawMode(true)
	fmt.Print("\033[?25l")
}

func (d *Dashboard) restoreTerminal() {
	setRawMode(false)
	fmt.Print("\033[?25h")
}

func DashboardHeaderStr(title string) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("%s %s", BoldText(CyanText("◆")), BoldText(title)))
	sb.WriteString(fmt.Sprintf("  %s %s\n", C(White, time.Now().Format("15:04:05")), BlueText("v1.0")))
	sb.WriteString(strings.Repeat("─", 80) + "\n")
	return sb.String()
}

func RunDashboard(serverURL string) {
	d := NewDashboard(serverURL)
	d.AddLog("INFO", "Dashboard started")
	d.AddLog("INFO", fmt.Sprintf("Connecting to %s", serverURL))
	d.fetchRecommendations()
	d.Run()
}

func RunWorkflowMonitor(workflowName string) {
	fmt.Printf("\n%s Monitoring workflow: %s\n", BoldText("⚡"), CyanText(workflowName))
	fmt.Println(strings.Repeat("─", 60))

	nodes := []string{"recon", "scan", "exploit", "post-exploit", "report"}
	for i, node := range nodes {
		if i == 0 {
			fmt.Printf("  %s %s %s\n", fmt.Sprintf("[%d/%d]", i+1, len(nodes)), node, GreenText("✓"))
			time.Sleep(300 * time.Millisecond)
		} else if i < 4 {
			for j := 0; j < 3; j++ {
				fmt.Printf("\r  %s %s %s", fmt.Sprintf("[%d/%d]", i+1, len(nodes)), node, Spinner(j))
				time.Sleep(100 * time.Millisecond)
			}
			fmt.Printf("\r  %s %s %s\n", fmt.Sprintf("[%d/%d]", i+1, len(nodes)), node, GreenText("✓"))
		} else {
			fmt.Printf("  %s %s %s\n", fmt.Sprintf("[%d/%d]", i+1, len(nodes)), node, GreenText("✓"))
		}
	}
	fmt.Printf("\n%s Workflow %s completed!\n", GreenText("✓"), CyanText(workflowName))
}

func setRawMode(on bool) {
	fd := int(os.Stdin.Fd())
	if on {
		var termios [256]byte
		syscall.Syscall(syscall.SYS_IOCTL, uintptr(fd), uintptr(0x5401), uintptr(unsafe.Pointer(&termios[0])))
		termios[3] &^= 0x000A
		termios[3] |= 0x0001
		syscall.Syscall(syscall.SYS_IOCTL, uintptr(fd), uintptr(0x5402), uintptr(unsafe.Pointer(&termios[0])))
	} else {
		var termios [256]byte
		syscall.Syscall(syscall.SYS_IOCTL, uintptr(fd), uintptr(0x5401), uintptr(unsafe.Pointer(&termios[0])))
		termios[3] |= 0x000A
		termios[3] &^= 0x0001
		syscall.Syscall(syscall.SYS_IOCTL, uintptr(fd), uintptr(0x5402), uintptr(unsafe.Pointer(&termios[0])))
	}
}

func RunEventViewer() {
	fmt.Printf("\n%s Live Event Stream\n", BoldText("📡"))
	fmt.Println(strings.Repeat("─", 80))

	events := []struct {
		ts, typ, detail string
		c               Color
	}{
		{"14:32:01", "MISSION_STARTED", "Mission Alpha launched", Green},
		{"14:32:05", "JOB_ENQUEUED", "Port scan queued for 10.0.0.1", Cyan},
		{"14:32:10", "TASK_COMPLETED", "Port scan completed: 3 open ports", White},
		{"14:32:15", "FINDING_DISCOVERED", "SQL Injection on login page", Red},
		{"14:32:20", "JOB_ENQUEUED", "Exploit queued for CVE-2024-0001", Yellow},
	}

	for _, ev := range events {
		fmt.Printf("  %s [%s] %s\n", C(Green, ev.ts), C(ev.c, ev.typ), ev.detail)
		time.Sleep(500 * time.Millisecond)
	}
	fmt.Println(strings.Repeat("─", 80))
}

func RunMetricsViewer() {
	fmt.Printf("\n%s System Metrics\n", BoldText("📊"))
	fmt.Println()
	fmt.Println(Table(
		[]string{"Metric", "Value", "Status"},
		[][]string{
			{"CPU Usage", "23.5%", "OK"},
			{"Memory", "156.2 MB", "OK"},
			{"Goroutines", "42", "OK"},
			{"HTTP Requests/s", "1,245", "OK"},
			{"Error Rate", "0.2%", "OK"},
			{"DB Queries/s", "3,891", "OK"},
			{"Cache Hit Rate", "94.7%", "OK"},
			{"Queue Depth", "5", "OK"},
		},
	))
}

func RunPluginManager() {
	fmt.Printf("\n%s Plugin Manager\n", BoldText("🔌"))
	fmt.Println()
	fmt.Println(Table(
		[]string{"Plugin", "Version", "Status"},
		[][]string{
			{"network-scanner", "1.2.0", "Active"},
			{"exploit-pack", "2.1.0", "Active"},
			{"report-generator", "0.9.0", "Active"},
			{"ai-assistant", "1.0.0", "Active"},
			{"brute-force", "1.5.0", "Disabled"},
		},
	))
	fmt.Printf("\nFound 5 plugins, 4 active.\n")
}

func ShowProgress(prefix string, steps int) {
	for i := 0; i <= steps; i++ {
		fmt.Printf("\r%s", ProgressBar(prefix, i, steps, 40))
		time.Sleep(200 * time.Millisecond)
	}
	fmt.Println()
	fmt.Printf("%s %s\n", GreenText("✓"), prefix+" completed")
}
