package main

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEnvVarsOverrideDefaults(t *testing.T) {
	t.Setenv("TAGSCALE_OUTPUT", "json")
	t.Setenv("TAGSCALE_REGION", "us-west-2")

	cli := NewCLI()
	require.Equal(t, "json", cli.PersistentFlags().Lookup("output").Value.String())
	require.Equal(t, "us-west-2", cli.PersistentFlags().Lookup("region").Value.String())
}

func TestEnvVarOutputAffectsVersion(t *testing.T) {
	t.Setenv("TAGSCALE_OUTPUT", "json")

	cli := NewCLI()
	var buf bytes.Buffer
	cli.SetOut(&buf)
	cli.SetArgs([]string{"version"})
	require.NoError(t, cli.Execute())
	require.Equal(t, fmt.Sprintf("{\"version\":\"%s\"}\n", Version), buf.String())
}
