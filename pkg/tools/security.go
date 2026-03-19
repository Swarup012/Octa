package tools

import (
	"os"
	"path/filepath"
	"strings"
)

// dangerZonePaths are system paths that are always blocked, even with approval.
var dangerZonePaths = []string{
	"/etc/passwd",
	"/etc/shadow",
	"/etc/sudoers",
	"/boot",
	"/proc",
	"/sys",
	"/dev",
	"/root",
}

// dangerZoneHomePatterns are home directory patterns that are always blocked, even with approval.
var dangerZoneHomePatterns = []string{
	".ssh",
	".gnupg",
	".config",
}

// isDangerZone checks if a path is in a protected zone.
// Danger zones are always blocked, even with elevated permissions.
func isDangerZone(path string) bool {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return false
	}

	// Check standard danger zones
	for _, danger := range dangerZonePaths {
		if strings.HasPrefix(absPath, danger) {
			return true
		}
	}

	// Check home dotfiles
	homeDir, err := os.UserHomeDir()
	if err == nil {
		for _, pattern := range dangerZoneHomePatterns {
			if strings.HasPrefix(absPath, filepath.Join(homeDir, pattern)) {
				return true
			}
		}
	}

	return false
}