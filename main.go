package main

import (
	"bytes"
	"fmt"
	"os/exec"
	"slices"
	"strconv"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

type ProcessInfo struct {
	Port    string
	AppName string
	PID     string
	Memory  string
}

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

func main() {
	a := app.New()
	w := a.NewWindow("BetterTM - Port Scanner")
	searchEntry := widget.NewEntry()
	searchEntry.SetPlaceHolder("Search by App Name or Port...")
	data, _ := GetMacLinuxPorts()
	footer := widget.NewLabel(strconv.Itoa(len(data)) + " processes found")
	var table *widget.Table
	table = widget.NewTable(
		func() (int, int) {
			// rows, columns (+1 for button column)
			return len(data), 5
		},
		func() fyne.CanvasObject {
			// Default object for every cell
			return container.NewMax(
				widget.NewLabel(""),
				widget.NewButton("", nil),
			)
		},
		func(id widget.TableCellID, obj fyne.CanvasObject) {
			c := obj.(*fyne.Container)

			label := c.Objects[0].(*widget.Label)
			button := c.Objects[1].(*widget.Button)

			if id.Col < 4 {
				if id.Col == 0 {
					label.SetText(data[id.Row].Port)
				} else if id.Col == 1 {
					label.SetText(data[id.Row].AppName)
				} else if id.Col == 2 {
					label.SetText(data[id.Row].PID)
				} else if id.Col == 3 {
					label.SetText(data[id.Row].Memory)
				}
				label.Show()
				button.Hide()
			} else {
				label.Hide()
				button.Show()
				button.SetText("KILL")
				row := id.Row
				button.OnTapped = func() {
					_ = exec.Command("kill", "-9", data[row].PID).Run()
					fmt.Println(data)
					data = append(data[:row], data[row+1:]...)
					table.Refresh()
					fmt.Println(data)
				}
			}
		},
	)

	table.SetColumnWidth(0, 80)
	table.SetColumnWidth(1, 200)
	table.SetColumnWidth(2, 80)
	table.SetColumnWidth(3, 80)
	table.SetColumnWidth(4, 80)

	searchEntry.OnChanged = func(text string) {
		nData, err := GetMacLinuxPorts()

		if text == "" {
			if err == nil {
				data = nData
				footer.SetText(strconv.Itoa(len(data)) + " processes found")
			}
		} else {
			newData := []ProcessInfo{}
			for _, p := range nData {
				if strings.Contains(strings.ToLower(p.AppName), strings.ToLower(text)) || strings.Contains(p.Port, text) {
					newData = append(newData, p)
				}
			}
			data = newData
			footer.SetText(strconv.Itoa(len(data)) + " results found")
		}
		table.Refresh()
	}

	go func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()

		for range ticker.C {
			// Fetch new system snapshot
			searchText := searchEntry.Text
			searchEntry.OnChanged(searchText) // Reapply search filter after refreshing data
			// freshData, err := GetMacLinuxPorts()
			// if err == nil {
			// 	// Safely overwrite the global slice map
			// 	data = freshData

			// 	// Tell the UI table component to repaint itself on screen
			// 	table.Refresh()
			// }
		}
	}()
	mainLayout := container.NewBorder(searchEntry, footer, nil, nil, table)

	w.SetContent(mainLayout)
	// w.SetContent(table)
	w.Resize(fyne.NewSize(550, 520))
	w.ShowAndRun()
}
