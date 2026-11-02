package mqttclient

import (
	"bytes"
	"errors"
	"os"
	"testing"

	"at.ourproject/energystore/internal/logging"
	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// clientWithSecret stands for paho's client, whose printed form holds the connection options.
type clientWithSecret struct {
	mqtt.Client
	Password string
}

func TestConnectionLostLogsOnlyTheError(t *testing.T) {
	var out bytes.Buffer
	require.NoError(t, logging.Configure(nil, &out))
	t.Cleanup(func() { _ = logging.Configure(os.Environ(), os.Stdout) })

	clientOptions().OnConnectionLost(&clientWithSecret{Password: "s3cret"}, errors.New("broker gone"))

	line := out.String()
	assert.Contains(t, line, `"error.message":"broker gone"`)
	assert.NotContains(t, line, "s3cret")
	assert.NotContains(t, line, "Password")
}
