package ybcluster

import (
	"strings"
)

// containsString checks if string s is in the slice. Function
// definition is from Kubebuilder project licensed under Apache
// License, Version 2.0 https://git.io/Jf5E2
func containsString(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}

// runWithShell wraps the given cmd slice in a new slice representing
// a command like 'shell -c "cmd arg1 arg2"'
func runWithShell(shell string, cmd []string) []string {
	return []string{shell, "-c", strings.Join(cmd, " ")}
}
