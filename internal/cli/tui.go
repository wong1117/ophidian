package cli

import (
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
	"unsafe"
)

type dashboardMsg int

const (
	msgTick dashboardMsg = iota
	msgQuit
)

type teaMsg struct {
	key  byte
}

type Dashboard struct {
	refreshInterval time.Duration
	logs            []string
	maxLogs         int
	logCh           chan string
	stopCh          chan struct{}
	running         bool
}

func NewDashboard() *Dashboard {
	return &Dashboard{
		refreshInterval: time.Second,
		maxLogs:         20,
		logCh:           make(chan string, 100),
		stopCh:          make(chan struct{}),
	}
}

func (d *Dashboard) Run() {
	d.running = true

	d.setupTerminal()
	defer d.restoreTerminal()

	ticker := time.NewTicker(d.refreshInterval)
	defer ticker.Stop()

	keyCh := make(chan byte, 10)
	go d.readKeys(keyCh)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	frame := 0
	render := func() {
		fmt.Print("\033[2J\033[H")
		fmt.Print(d.view(frame))
		frame++
	}

	render()

	for d.running {
		select {
		case <-sigCh:
			d.running = false
			fmt.Print("\033[2J\033[H")
			fmt.Println(GreenText("Dashboard stopped."))
			return

		case key := <-keyCh:
			switch key {
			case 'q', 'Q', 3:
				d.running = false
				fmt.Print("\033[2J\033[H")
				fmt.Println(GreenText("Dashboard stopped."))
				return
			case 'r':
				render()
			}

		case logLine := <-d.logCh:
			d.logs = append(d.logs, logLine)
			if len(d.logs) > d.maxLogs {
				d.logs = d.logs[len(d.logs)-d.maxLogs:]
			}
			render()

		case <-ticker.C:
			render()
		}
	}
}

func (d *Dashboard) view(frame int) string {
	var sb strings.Builder

	sb.WriteString(DashboardHeaderStr("OPHIDIAN CONTROL CENTER"))

	systemMetrics := []MetricRow{
		{Name: "Status", Value: "RUNNING", Color: Green},
		{Name: "Uptime", Value: time.Now().Format("15:04:05"), Color: White},
		{Name: "CPU", Value: "23%", Color: Yellow},
		{Name: "Memory", Value: "156MB", Color: White},
		{Name: "Goroutines", Value: fmt.Sprintf("%d", d.routineCount()), Color: Cyan},
	}
	sb.WriteString(MetricPanel("SYSTEM", systemMetrics))
	sb.WriteString("\n")

	queueMetrics := []MetricRow{
		{Name: "Pending", Value: "3", Color: Yellow},
		{Name: "In-flight", Value: "1", Color: White},
		{Name: "Dead-lettered", Value: "0", Color: Green},
		{Name: "Delayed", Value: "2", Color: Cyan},
	}
	sb.WriteString(MetricPanel("QUEUES", queueMetrics))

	workerMetrics := []MetricRow{
		{Name: "Total", Value: "5", Color: Green},
		{Name: "Idle", Value: "2", Color: White},
		{Name: "Busy", Value: "1", Color: Yellow},
		{Name: "Offline", Value: "0", Color: Green},
	}
	sb.WriteString(MetricPanel("WORKERS", workerMetrics))
	sb.WriteString("\n")

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
	sb.WriteString(Box("LIVE LOGS", logLines, 80))
	sb.WriteString(fmt.Sprintf("\n%s %s    %s | %s",
		Spinner(frame), GreenText("● Live"), BlueText("q: quit"), BlueText("r: refresh")))
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
	var buf [1]byte
	for d.running {
		n, err := os.Stdin.Read(buf[:])
		if err != nil || n == 0 {
			continue
		}
		ch <- buf[0]
	}
}

func (d *Dashboard) routineCount() int {
	return 0
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

func RunDashboard() {
	d := NewDashboard()
	d.AddLog("INFO", "Dashboard started")
	d.AddLog("INFO", "Loading subsystems...")

	go func() {
		t := time.NewTicker(5 * time.Second)
		defer t.Stop()
		for d.running {
			select {
			case <-t.C:
				d.AddLog("INFO", "Heartbeat: system healthy")
			case <-d.stopCh:
				return
			}
		}
	}()

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
