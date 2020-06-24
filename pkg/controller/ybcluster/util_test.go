package ybcluster

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestContainsString(t *testing.T) {

	cases := []struct {
		name     string
		slice    []string
		s        string
		expected bool
	}{
		{"string present in slice", []string{"b", "c", "z", "a"}, "a", true},
		{"string not present in slice", []string{"a", "b"}, "z", false},
		{"empty slice", []string{}, "a", false},
		{"nil slice", nil, "a", false},
	}

	for _, cas := range cases {
		t.Run(cas.name, func(t *testing.T) {
			got := containsString(cas.slice, cas.s)
			assert.Equal(t, cas.expected, got)
		})
	}
}

func TestRunWithShell(t *testing.T) {

	cases := []struct {
		name     string
		shell    string
		cmd      []string
		expected []string
	}{
		{"bash spaces in cmd elements", "bash", []string{"yb-admin", "--arg", "'test name,two'"}, []string{"bash", "-c", "yb-admin --arg 'test name,two'"}},
		{"sh single cmd element", "sh", []string{"ls"}, []string{"sh", "-c", "ls"}},
		{"sh empty cmd", "sh", []string{}, []string{"sh", "-c", ""}},
	}

	for _, cas := range cases {
		t.Run(cas.name, func(t *testing.T) {
			got := runWithShell(cas.shell, cas.cmd)
			assert.Equal(t, cas.expected, got)
		})
	}
}
