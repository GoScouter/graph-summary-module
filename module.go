package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/GoScouter/sdk"
	"github.com/GoScouter/sdk/style"
)

var scanNamespace = sdk.ModuleNamespace{Name: "scan", Author: "internal"}

type SgsModule struct{}

type report struct {
	Summary Summary `json:"summary"`
	Path    string  `json:"path"`
}

func (m *SgsModule) Info() sdk.ModuleInfo {
	deps := make([]sdk.ModuleNamespace, 0)
	deps = append(deps, scanNamespace)

	return sdk.ModuleInfo{
		Name:         "sgs",
		Author:       "goscouter",
		Description:  "Renders the scan result as a browsable HTML graph",
		Dependencies: deps,
	}
}

func (m *SgsModule) Scout(target string, args []string) (json.RawMessage, error) {
	fs := flag.NewFlagSet("web-summary", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	out := fs.String("out", "", "path for the generated HTML page (default gs-scan-<host>.html)")
	if err := fs.Parse(args); err != nil {
		return nil, err
	}

	client, err := sdk.NewClient()
	if err != nil {
		return nil, err
	}
	defer client.Close()

	data, err := client.AskModule(scanNamespace)
	if err != nil {
		return nil, err
	}

	if strings.TrimSpace(data) == "" || strings.TrimSpace(data) == "null" {
		return nil, fmt.Errorf("web-summary: %s:%s produced no result to render", scanNamespace.Author, scanNamespace.Name)
	}

	var result Result
	if err := json.Unmarshal([]byte(data), &result); err != nil {
		return nil, fmt.Errorf("web-summary: unreadable scan result: %w", err)
	}

	if result.Target == "" {
		result.Target = target
	}

	html, summary := Page(result)

	path := *out
	if path == "" {
		path = fmt.Sprintf("gs-scan-%s.html", filename(summary.Host))
	}

	if err := os.WriteFile(path, []byte(html), 0o644); err != nil {
		return nil, fmt.Errorf("web-summary: writing %s: %w", path, err)
	}

	if abs, err := filepath.Abs(path); err == nil {
		path = abs
	}

	return json.Marshal(report{Summary: summary, Path: path})
}

const labelWidth = 10

func (m *SgsModule) Render(raw json.RawMessage) string {
	var r report
	if err := json.Unmarshal(raw, &r); err != nil {
		return style.Failuref("web-summary: unreadable result: %v", err) + "\r\n"
	}

	return style.Section("WEB SUMMARY",
		style.Field(style.Green("[+]")+style.Gray(" Target"), labelWidth, style.White(r.Summary.Target)),
		style.Field(style.Green("[+]")+style.Gray(" Subdomains"), labelWidth, style.White(fmt.Sprintf("%d discovered", r.Summary.Subdomains))),
		style.Field(style.Green("[+]")+style.Gray(" Reachable"), labelWidth, style.White(strconv.Itoa(r.Summary.Reachable))),
		style.Field(style.Green("[+]")+style.Gray(" Page"), labelWidth, style.Cyan(r.Path)),
	)
}

func filename(host string) string {
	safe := strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			return r
		case r == '.', r == '-', r == '_':
			return r
		default:
			return '-'
		}
	}, host)

	safe = strings.Trim(safe, "-.")
	if safe == "" {
		return "target"
	}

	return safe
}
