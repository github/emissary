// Package stats is a very lightweight wrapper around the dogstatsd client, which gives
// us a package-level function
package stats

import (
	"fmt"
	"time"

	"github.com/DataDog/datadog-go/statsd"
)

var client StatsClient = NopClient{}

type StatsClient interface {
	Timing(name string, took time.Duration, tags []string, v float64) error
	Incr(name string, tags []string, rate float64) error
	Gauge(name string, value float64, tags []string, rate float64) error
}

type NopClient struct{}

func (NopClient) Timing(_ string, _ time.Duration, _ []string, _ float64) error { return nil }
func (NopClient) Incr(_ string, _ []string, _ float64) error                    { return nil }
func (NopClient) Gauge(_ string, _ float64, _ []string, _ float64) error        { return nil }

// Configure sets up statsd
func Configure(host string, port int) error {
	statsd, err := statsd.NewBuffered(fmt.Sprintf("%s:%d", host, port), 100)
	if err != nil {
		return fmt.Errorf("error configuring statsd client: %v", err)
	}
	statsd.Namespace = "emissary."
	client = statsd

	return nil
}

// Client returns the statsd client
func Client() StatsClient {
	return client
}

// Timing run a function and record timing
func Timing(name string, took time.Duration, tags []string) {
	client.Timing(name, took, tags, 1)
}

// Incr increment a thing
func Incr(key string, tags []string) {
	client.Incr(key, tags, 1)
}

// IncrSuccess increment success
func IncrSuccess(prefix string, tags []string) {
	var key string

	key = prefix + ".success"

	client.Incr(key, tags, 1)
}

// IncrFail increment success
func IncrFail(prefix string, tags []string) {
	var key string

	key = prefix + ".fail"

	client.Incr(key, tags, 1)
}
