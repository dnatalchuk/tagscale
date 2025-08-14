package aws

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewClientUsesBackgroundWhenContextNil(t *testing.T) {
	os.Setenv("AWS_EC2_METADATA_DISABLED", "true")
	t.Cleanup(func() { os.Unsetenv("AWS_EC2_METADATA_DISABLED") })

	client, err := NewClient(nil, "us-east-1", "")
	require.NoError(t, err)
	require.NotNil(t, client)
}
