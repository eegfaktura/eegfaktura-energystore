package mqttclient

import (
	"errors"
	"testing"

	"at.ourproject/energystore/internal/testsupport/logcapture"
	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/stretchr/testify/assert"
)

// clientWithSecret stands for paho's client, whose printed form holds the connection options.
type clientWithSecret struct {
	mqtt.Client
	Password string
}

func TestConnectionLostLogsOnlyTheError(t *testing.T) {
	out := logcapture.Capture(t)

	clientOptions().OnConnectionLost(&clientWithSecret{Password: "s3cret"}, errors.New("broker gone"))

	line := out.String()
	assert.Contains(t, line, `"error.message":"broker gone"`)
	assert.NotContains(t, line, "s3cret")
	assert.NotContains(t, line, "Password")
}
