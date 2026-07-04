package main

import (
	"bytes"
	"fmt"
	"os/exec"
	"runtime"
	"slices"
	"strconv"
	"strings"
)

// ProcessInfo holds our combined system metrics
type ProcessInfo struct {
	Port    string
	AppName string
	PID     string
	Memory  string
}

func main() {
	fmt.Println("🔍 Scanning system ports, application names, and memory...")

	var processes []ProcessInfo
	var err error

	// Detect the OS and call the matching helper function
	if runtime.GOOS == "windows" {
		processes, err = getWindowsPorts()
	} else {
		processes, err = getMacLinuxPorts()
	}

	if err != nil {
		fmt.Printf("Error scanning system: %v\n", err)
		return
	}

	// Print the simplistic parsed results directly to console
	if len(processes) == 0 {
		fmt.Println("No active ports found.")
		return
	}

	fmt.Printf("\n%-10s %-25s %-10s %-12s\n", "PORT", "APPLICATION NAME", "PID", "RAM USAGE")
	fmt.Println(strings.Repeat("-", 60))
	for _, p := range processes {
		fmt.Printf("%-10s %-25s %-10s %-12s\n", p.Port, p.AppName, p.PID, p.Memory)
	}
}

// 🪟 Windows Implementation
func getWindowsPorts() ([]ProcessInfo, error) {
	var list []ProcessInfo

	// 1. Run netstat to find listening TCP ports and their associated PIDs
	cmd := exec.Command("cmd", "/c", "netstat -ano -p tcp")
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return nil, err
	}

	lines := strings.Split(out.String(), "\n")
	for _, line := range lines {
		if !strings.Contains(line, "LISTENING") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 5 {
			continue
		}

		// Extract Port and PID from netstat line
		localAddr := fields[1] // e.g., "127.0.0.1:8080" or "[::]:3000"
		pid := fields[4]
		idx := strings.LastIndex(localAddr, ":")
		if idx == -1 {
			continue
		}
		port := localAddr[idx+1:]

		// 2. Query tasklist to get the specific Application Name and Memory usage for this PID
		taskCmd := exec.Command("tasklist", "/FI", fmt.Sprintf("PID eq %s", pid), "/FO", "CSV", "/NH")
		var taskOut bytes.Buffer
		taskCmd.Stdout = &taskOut
		_ = taskCmd.Run()

		// Parse the output string (format: "cmd.exe","1234","Console","1","4,124 K")
		taskLine := strings.TrimSpace(taskOut.String())
		if taskLine == "" || strings.Contains(taskLine, "No tasks") {
			continue
		}

		taskFields := strings.Split(taskLine, "\",\"")
		if len(taskFields) < 5 {
			continue
		}

		appName := strings.Trim(taskFields[0], "\"")
		memory := strings.Trim(taskFields[4], "\"\r\n ")

		list = append(list, ProcessInfo{
			Port:    port,
			AppName: appName,
			PID:     pid,
			Memory:  memory,
		})
	}
	return list, nil
}

// 🍏 🐧 macOS and Linux Implementation
func GetMacLinuxPorts() ([]ProcessInfo, error) {
	var list []ProcessInfo
	var pids map[string]struct{} = map[string]struct{}{}
	// Run standard lsof to get clean columns: Command, PID, User, FD, Type, Device, Size/Off, Node, Name
	cmd := exec.Command("lsof", "-iTCP", "-sTCP:LISTEN", "-P", "-n")
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return nil, err
	}

	lines := strings.Split(out.String(), "\n")
	for i, line := range lines {
		// Skip the very first index line (the header row)
		if i == 0 || strings.TrimSpace(line) == "" {
			continue
		}

		// Split columns cleanly ignoring variable spaces
		fields := strings.Fields(line)
		if len(fields) < 9 {
			continue
		}

		appName := fields[0] // e.g., "go" or "dummy"
		pid := fields[1]     // Process ID
		rawName := fields[8] // e.g., "*:9000" or "127.0.0.1:8080"

		// Extract the port number cleanly after the trailing colon
		idx := strings.LastIndex(rawName, ":")
		if idx == -1 {
			continue
		}
		if _, found := pids[pid]; found {
			continue
		}
		pids[pid] = struct{}{}

		port := rawName[idx+1:]

		// Query memory (RAM) allocation details via process stats utility
		memCmd := exec.Command("ps", "-o", "rss=", "-p", pid)
		var memOut bytes.Buffer
		memCmd.Stdout = &memOut
		_ = memCmd.Run()

		kbStr := strings.TrimSpace(memOut.String())
		var memDisplay = "0.0 MB"
		if kbStr != "" {
			var kb int
			_, fmtErr := fmt.Sscanf(kbStr, "%d", &kb)
			if fmtErr == nil {
				memDisplay = fmt.Sprintf("%.1f MB", float64(kb)/1024.0)
			}
		}

		list = append(list, ProcessInfo{
			Port:    port,
			AppName: appName,
			PID:     pid,
			Memory:  memDisplay,
		})
	}
	slices.SortFunc(list, func(a, b ProcessInfo) int {
		portA, _ := strconv.Atoi(a.Port)
		portB, _ := strconv.Atoi(b.Port)

		return portA - portB
	})

	return list, nil
}
