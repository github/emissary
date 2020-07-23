package stats

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConfigure(t *testing.T) {
	var err error

	assert.Equal(t, NopClient{}, Client(), "Client() should be a no-op implementation before Configure()")

	err = Client().Gauge("foo", 1, nil, 1)
	assert.NoError(t, err, "Sending a metric to a nil client should not throw an error")

	err = Configure("127.0.0.1", 8125)
	assert.Nil(t, err, "Configure should not return an error")
	assert.NotNil(t, Client(), "Client should not be nil after configure")
}
