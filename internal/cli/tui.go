package cli

import (
	"fmt"
	"os"
	"os/signal"
	"strings"
	"time"
)

type Dashboard struct {
	refreshInterval time.Duration
	running         bool
	logs            []string
	maxLogs         int
}

func NewDashboard() *Dashboard {
	return &Dashboard{
		refreshInterval: time.Second,
		maxLogs:         20,
	}
}

func (d *Dashboard) Run() {
	d.running = true
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt)

	ticker := time.NewTicker(d.refreshInterval)
	defer ticker.Stop()

	frame := 0
	for d.running {
		select {
		case <-sigCh:
			d.running = false
			fmt.Print("\033[2J\033[H")
			fmt.Println(GreenText("Dashboard stopped."))
			return
		case <-ticker.C:
			d.render(frame)
			frame++
		}
	}
}

func (d *Dashboard) render(frame int) {
	DashboardHeader("OPHIDIAN CONTROL CENTER")

	systemMetrics := []MetricRow{
		{Name: "Status", Value: "RUNNING", Color: Green},
		{Name: "Uptime", Value: "5m 32s", Color: White},
		{Name: "CPU", Value: "23%", Color: Yellow},
		{Name: "Memory", Value: "156MB", Color: White},
		{Name: "Goroutines", Value: "42", Color: Cyan},
	}
	fmt.Print(MetricPanel("SYSTEM", systemMetrics))
	fmt.Println()

	queueMetrics := []MetricRow{
		{Name: "Pending", Value: "3", Color: Yellow},
		{Name: "In-flight", Value: "1", Color: White},
		{Name: "Dead-lettered", Value: "0", Color: Green},
		{Name: "Delayed", Value: "2", Color: Cyan},
	}
	fmt.Print(MetricPanel("QUEUES", queueMetrics))

	workerMetrics := []MetricRow{
		{Name: "Total", Value: "5", Color: Green},
		{Name: "Idle", Value: "2", Color: White},
		{Name: "Busy", Value: "1", Color: Yellow},
		{Name: "Offline", Value: "0", Color: Green},
	}
	fmt.Print(MetricPanel("WORKERS", workerMetrics))
	fmt.Println()

	var logLines []string
	if len(d.logs) == 0 {
		logLines = []string{
			LogLine("INFO", "Server started on :8443"),
			LogLine("INFO", "PostgreSQL connection established"),
			LogLine("INFO", "Redis cache connected"),
			LogLine("DEBUG", "Worker pool initialized with 5 workers"),
		}
	} else {
		logLines = d.logs
	}
	fmt.Println(Box("LIVE LOGS", logLines, 80))

	if len(d.logs) > 0 { d.logs = d.logs[:0] }
	fmt.Printf("\n%s %s    %s", Spinner(frame), GreenText("● Live"), BlueText("Ctrl+C to exit"))
	time.Sleep(d.refreshInterval)
}

func (d *Dashboard) AddLog(level, msg string) {
	d.logs = append(d.logs, LogLine(level, msg))
	if len(d.logs) > d.maxLogs { d.logs = d.logs[len(d.logs)-d.maxLogs:] }
}

func (d *Dashboard) Stop() { d.running = false }

func RunDashboard() {
	d := NewDashboard()
	d.AddLog("INFO", "Dashboard started")
	d.Run()
}

func RunWorkflowMonitor(workflowName string) {
	fmt.Printf("\n%s Monitoring workflow: %s\n", BoldText("⚡"), CyanText(workflowName))
	fmt.Println(strings.Repeat("─", 60))

	nodes := []string{"recon", "scan", "exploit", "post-exploit", "report"}
	for i, node := range nodes {
		status := "✓"
		if i == 0 {
			status = GreenText("✓")
			time.Sleep(500 * time.Millisecond)
		} else if i < 4 {
			for j := 0; j < 3; j++ {
				fmt.Printf("\r  %s %s %s", fmt.Sprintf("[%d/%d]", i, len(nodes)), node, Spinner(j))
				time.Sleep(200 * time.Millisecond)
			}
			status = GreenText("✓")
		}
		fmt.Printf("\r  %s %s %s\n", fmt.Sprintf("[%d/%d]", i+1, len(nodes)), node, status)
	}
	fmt.Printf("\n%s Workflow %s completed!\n", GreenText("✓"), CyanText(workflowName))
}

func RunEventViewer() {
	fmt.Printf("\n%s Live Event Stream\n", BoldText("📡"))
	fmt.Println(strings.Repeat("─", 80))

	colors := []Color{Green, Cyan, White, Red, Yellow}
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
	_ = colors
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
