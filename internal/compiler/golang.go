package compiler

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// checkGoVersion executes 'go version' to check if Go is installed,
// returning a stripped version string (e.g. "1.26.3") or an error.
func checkGoVersion() (string, error) {
	_, err := exec.LookPath("go")
	if err != nil {
		return "", fmt.Errorf("go toolchain is not installed or not in PATH")
	}

	cmd := exec.Command("go", "version")
	outputBytes, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to execute 'go version': %w", err)
	}

	output := strings.TrimSpace(string(outputBytes))
	fields := strings.Fields(output)
	if len(fields) >= 3 {
		rawVersion := fields[2]
		cleanVersion := strings.TrimPrefix(rawVersion, "go")
		return cleanVersion, nil
	}

	return "", fmt.Errorf("unexpected go version output format: %q", output)
}

// isGoVersionSupported compares the installed version against a minimum required
// version slice-by-slice as integers to verify if it is supported.
func isGoVersionSupported(installedStr, minRequiredStr string) bool {
	installedParts := strings.Split(installedStr, ".")
	requiredParts := strings.Split(minRequiredStr, ".")

	maxLen := max(len(requiredParts), len(installedParts))

	for i := range maxLen {
		var installedNum, requiredNum int
		var err error

		if i < len(installedParts) {
			installedNum, err = strconv.Atoi(installedParts[i])
			if err != nil {
				installedNum = 0
			}
		}

		if i < len(requiredParts) {
			requiredNum, err = strconv.Atoi(requiredParts[i])
			if err != nil {
				requiredNum = 0
			}
		}

		if installedNum > requiredNum {
			return true
		}
		if installedNum < requiredNum {
			return false
		}
	}

	return true
}
