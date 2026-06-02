//go:build windows

package session

import (
	"os/exec"
	"strconv"
	"strings"
)

// findBrowsersWindows enumerates running browser processes via PowerShell
// Get-CimInstance Win32_Process (which exposes CommandLine — tasklist does
// not). It does NOT filter by path inside PowerShell: -like wildcard matching
// over backslash-heavy Windows paths is fragile (\, [, ], ?, * are all
// metacharacters). Instead it emits "PID\tCommandLine" for every candidate
// browser and applies the same resolved-path gate (matchDataDirCmdline) in Go
// that the Linux path uses, so matching semantics are identical cross-platform.
//
// PowerShell is on every modern Windows install; this avoids pulling in the
// WMI / OLE Go bindings just for one scan operation.
func findBrowsersWindows(dataDir string) []int {
	if dataDir == "" {
		return nil
	}

	const script = `Get-CimInstance Win32_Process -Filter "Name='chrome.exe' OR Name='brave.exe' OR Name='msedge.exe' OR Name='chromium.exe'" |
		Where-Object { $_.CommandLine } |
		ForEach-Object { "$($_.ProcessId)` + "`t" + `$($_.CommandLine)" }`

	out, err := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", script).Output()
	if err != nil {
		return nil
	}

	var pids []int

	for line := range strings.SplitSeq(string(out), "\n") {
		line = strings.TrimRight(line, "\r")

		pidStr, cmdline, ok := strings.Cut(line, "\t")
		if !ok {
			continue
		}

		pidStr = strings.TrimSpace(pidStr)

		pid, err := strconv.Atoi(pidStr)
		if err != nil || pid <= 0 {
			continue
		}

		if matchDataDirCmdline(cmdline, dataDir) {
			pids = append(pids, pid)
		}
	}

	return pids
}

// escapePowerShell escapes a string for safe LITERAL use inside a PowerShell
// double-quoted string that is later compared with the -like operator.
// Backticks, double quotes and $ are PowerShell string metacharacters;
// backslashes must be doubled so a Windows path is treated literally; and the
// -like wildcard metacharacters (* ? [ ]) are back-tick escaped so they match
// literally rather than as patterns.
//
// Retained for any future direct-PowerShell path comparison; the current
// findBrowsersWindows gates in Go and no longer interpolates the path, but the
// escaper is the documented-correct primitive and is unit tested.
func escapePowerShell(s string) string {
	r := strings.NewReplacer(
		"`", "``",
		`"`, "`\"",
		"$", "`$",
		`\`, `\\`,
		"*", "``*",
		"?", "``?",
		"[", "``[",
		"]", "``]",
	)

	return r.Replace(s)
}
