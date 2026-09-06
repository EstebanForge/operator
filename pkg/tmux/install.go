// Package tmux provides client communication and platform utilities for tmux.
package tmux

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

// InstallInfo contains platform-specific tmux installation commands.
type InstallInfo struct {
	Platform string `json:"platform"`
	Command  string `json:"command"`
}

// DetectInstallInfo detects the operating system and package manager to recommend an install command.
func DetectInstallInfo() InstallInfo {
	switch runtime.GOOS {
	case "darwin":
		return InstallInfo{
			Platform: "macOS",
			Command:  "brew install tmux",
		}
	case "linux":
		// Check common package managers in PATH
		if _, err := exec.LookPath("dnf"); err == nil {
			return InstallInfo{Platform: "Fedora / RHEL", Command: "sudo dnf install tmux"}
		}
		if _, err := exec.LookPath("apt-get"); err == nil {
			return InstallInfo{Platform: "Ubuntu / Debian", Command: "sudo apt update && sudo apt install tmux"}
		}
		if _, err := exec.LookPath("pacman"); err == nil {
			return InstallInfo{Platform: "Arch Linux", Command: "sudo pacman -S tmux"}
		}
		if _, err := exec.LookPath("apk"); err == nil {
			return InstallInfo{Platform: "Alpine Linux", Command: "sudo apk add tmux"}
		}
		if _, err := exec.LookPath("zypper"); err == nil {
			return InstallInfo{Platform: "openSUSE", Command: "sudo zypper install tmux"}
		}
		if _, err := exec.LookPath("brew"); err == nil {
			return InstallInfo{Platform: "Linux (Homebrew)", Command: "brew install tmux"}
		}

		// Fallback inspection of /etc/os-release
		if data, err := os.ReadFile("/etc/os-release"); err == nil {
			content := strings.ToLower(string(data))
			if strings.Contains(content, "fedora") || strings.Contains(content, "rhel") || strings.Contains(content, "centos") {
				return InstallInfo{Platform: "Fedora / RHEL", Command: "sudo dnf install tmux"}
			}
			if strings.Contains(content, "ubuntu") || strings.Contains(content, "debian") {
				return InstallInfo{Platform: "Ubuntu / Debian", Command: "sudo apt update && sudo apt install tmux"}
			}
			if strings.Contains(content, "arch") {
				return InstallInfo{Platform: "Arch Linux", Command: "sudo pacman -S tmux"}
			}
		}

		return InstallInfo{
			Platform: "Linux",
			Command:  "sudo apt install tmux  # or: sudo dnf install tmux",
		}
	case "freebsd":
		return InstallInfo{Platform: "FreeBSD", Command: "pkg install tmux"}
	case "openbsd":
		return InstallInfo{Platform: "OpenBSD", Command: "pkg_add tmux"}
	default:
		return InstallInfo{
			Platform: runtime.GOOS,
			Command:  "brew install tmux",
		}
	}
}

// MissingTmuxPrompt returns a formatted guidance message for terminal display.
func MissingTmuxPrompt() string {
	info := DetectInstallInfo()

	var sb strings.Builder
	sb.WriteString("Error: tmux binary not found in PATH.\n\n")
	sb.WriteString("'operator' requires tmux (>= 3.0 recommended) as its multiplexer backend engine.\n\n")
	_, _ = fmt.Fprintf(&sb, "To install tmux on your system (%s):\n", info.Platform)
	_, _ = fmt.Fprintf(&sb, "  $ %s\n\n", info.Command)
	sb.WriteString("Other platforms:\n")
	sb.WriteString("  • Fedora / RHEL:    sudo dnf install tmux\n")
	sb.WriteString("  • Ubuntu / Debian:  sudo apt update && sudo apt install tmux\n")
	sb.WriteString("  • macOS (Homebrew): brew install tmux\n")
	sb.WriteString("  • Arch Linux:       sudo pacman -S tmux\n")
	sb.WriteString("  • Alpine Linux:     sudo apk add tmux\n\n")
	sb.WriteString("After installing tmux, rerun 'operator'.")

	return sb.String()
}
