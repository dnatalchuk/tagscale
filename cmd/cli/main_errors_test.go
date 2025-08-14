package main

import (
	"bytes"
	"regexp"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestScanCmdErrorBubblesUp(t *testing.T) {
	cli := NewCLI()
	var errBuf bytes.Buffer
	cli.SetErr(&errBuf)
	cli.SetArgs([]string{"scan", "--range", "bad"})
	err := cli.Execute()
	require.Error(t, err)
	require.Contains(t, err.Error(), "Invalid range")
	require.NotContains(t, err.Error(), "command failed")
	require.NotContains(t, errBuf.String(), "command failed")
	require.False(t, regexp.MustCompile(`\d{4}/\d{2}/\d{2}`).MatchString(errBuf.String()))
}

func TestSummaryCmdErrorBubblesUp(t *testing.T) {
	cli := NewCLI()
	var errBuf bytes.Buffer
	cli.SetErr(&errBuf)
	cli.SetArgs([]string{"summary", "--range", "bad"})
	err := cli.Execute()
	require.Error(t, err)
	require.Contains(t, err.Error(), "Invalid range")
	require.NotContains(t, err.Error(), "command failed")
	require.NotContains(t, errBuf.String(), "command failed")
	require.False(t, regexp.MustCompile(`\d{4}/\d{2}/\d{2}`).MatchString(errBuf.String()))
}
