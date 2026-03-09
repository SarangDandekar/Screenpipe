package detector

import (
	"fmt"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/SarangDandekar/Screenpipe/meeting-coach/models"
)

type NetworkMonitor struct{}

func NewNetworkMonitor() *NetworkMonitor {
	return &NetworkMonitor{}
}

func (nm *NetworkMonitor) CheckProcess(pid int) models.NetworkStatus {
	status := models.NetworkStatus{
		Timestamp: time.Now(),
	}
	status.ProcessAlive = nm.isProcessAlive(pid)
	if !status.ProcessAlive {
		return status
	}
	allPIDs := nm.findAllPIDsByName("chrome.exe")
	if len(allPIDs) == 0 {
		allPIDs = []int{pid}
	}
	netstatOutput := nm.getNetstatOutput()
	for _, p := range allPIDs {
		tcp, udp := nm.countConnectionsForPID(netstatOutput, p)
		status.TCPCount += tcp
		status.UDPCount += udp
	}
	// Meeting has UDP only if there are non-mDNS UDP connections
	status.HasUDP = status.UDPCount > 0
	return status
}

func (nm *NetworkMonitor) getNetstatOutput() string {
	cmd := exec.Command("netstat", "-ano")
	output, err := cmd.Output()
	if err != nil {
		return ""
	}
	return string(output)
}

func (nm *NetworkMonitor) countConnectionsForPID(netstatOutput string, pid int) (int, int) {
	tcpCount := 0
	udpCount := 0
	pidStr := strconv.Itoa(pid)
	lines := strings.Split(netstatOutput, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		fields := strings.Fields(trimmed)
		if len(fields) < 4 {
			continue
		}
		lastField := fields[len(fields)-1]
		if lastField != pidStr {
			continue
		}
		if strings.Contains(trimmed, "ESTABLISHED") {
			tcpCount++
		}
		if fields[0] == "UDP" {
			localAddr := fields[1]
			port := nm.extractPort(localAddr)
			// Skip mDNS (5353) and port 0 — only count real media UDP
			if port > 0 && port != 5353 {
				udpCount++
			}
		}
	}
	return tcpCount, udpCount
}

func (nm *NetworkMonitor) extractPort(address string) int {
	// Handle IPv6 format [::]:5353
	if strings.Contains(address, "]:") {
		parts := strings.Split(address, "]:")
		if len(parts) == 2 {
			p, err := strconv.Atoi(parts[1])
			if err == nil {
				return p
			}
		}
		return 0
	}
	// Handle IPv4 format 0.0.0.0:5353
	parts := strings.Split(address, ":")
	if len(parts) >= 2 {
		p, err := strconv.Atoi(parts[len(parts)-1])
		if err == nil {
			return p
		}
	}
	return 0
}

func (nm *NetworkMonitor) isProcessAlive(pid int) bool {
	switch runtime.GOOS {
	case "windows":
		cmd := exec.Command("tasklist", "/FI", fmt.Sprintf("PID eq %d", pid), "/NH")
		output, err := cmd.Output()
		if err != nil {
			return false
		}
		return !strings.Contains(string(output), "No tasks")
	default:
		cmd := exec.Command("kill", "-0", strconv.Itoa(pid))
		return cmd.Run() == nil
	}
}

func (nm *NetworkMonitor) findAllPIDsByName(name string) []int {
	cmd := exec.Command("tasklist", "/FI", fmt.Sprintf("IMAGENAME eq %s", name), "/FO", "CSV", "/NH")
	output, err := cmd.Output()
	if err != nil {
		return nil
	}
	var pids []int
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, line := range lines {
		parts := strings.Split(line, ",")
		if len(parts) >= 2 {
			pidStr := strings.Trim(parts[1], "\" \r")
			p, err := strconv.Atoi(pidStr)
			if err == nil && p > 0 {
				pids = append(pids, p)
			}
		}
	}
	return pids
}

func (nm *NetworkMonitor) FindProcessPID(appName string) (int, error) {
	processNames := nm.getProcessSearchTerms(appName)
	for _, pname := range processNames {
		p, err := nm.findPIDByName(pname)
		if err == nil && p > 0 {
			return p, nil
		}
	}
	return 0, fmt.Errorf("process not found for app: %s", appName)
}

func (nm *NetworkMonitor) getProcessSearchTerms(appName string) []string {
	lower := strings.ToLower(appName)
	if runtime.GOOS == "windows" {
		if strings.Contains(lower, "chrome") || strings.Contains(lower, "google") {
			return []string{"chrome.exe"}
		}
		if strings.Contains(lower, "firefox") {
			return []string{"firefox.exe"}
		}
		if strings.Contains(lower, "edge") {
			return []string{"msedge.exe"}
		}
		if strings.Contains(lower, "zoom") {
			return []string{"Zoom.exe", "CptHost.exe"}
		}
		if strings.Contains(lower, "teams") {
			return []string{"ms-teams.exe", "Teams.exe"}
		}
		if strings.Contains(lower, "slack") {
			return []string{"slack.exe"}
		}
		if strings.Contains(lower, "discord") {
			return []string{"Discord.exe"}
		}
	}
	return []string{"chrome", "Google Chrome", "firefox", "zoom", "Teams"}
}

func (nm *NetworkMonitor) findPIDByName(name string) (int, error) {
	if runtime.GOOS == "windows" {
		cmd := exec.Command("tasklist", "/FI", fmt.Sprintf("IMAGENAME eq %s", name), "/FO", "CSV", "/NH")
		output, err := cmd.Output()
		if err != nil {
			return 0, err
		}
		outputStr := strings.TrimSpace(string(output))
		if outputStr == "" || strings.Contains(outputStr, "No tasks") {
			return 0, fmt.Errorf("no process found: %s", name)
		}
		lines := strings.Split(outputStr, "\n")
		for _, line := range lines {
			parts := strings.Split(line, ",")
			if len(parts) >= 2 {
				pidStr := strings.Trim(parts[1], "\" \r")
				p, err := strconv.Atoi(pidStr)
				if err == nil && p > 0 {
					return p, nil
				}
			}
		}
		return 0, fmt.Errorf("could not parse PID for: %s", name)
	}
	cmd := exec.Command("pgrep", "-i", "-f", name)
	output, err := cmd.Output()
	if err != nil {
		return 0, err
	}
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(lines) == 0 || lines[0] == "" {
		return 0, fmt.Errorf("no process found")
	}
	return strconv.Atoi(strings.TrimSpace(lines[0]))
}
