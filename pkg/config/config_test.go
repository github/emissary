package config

import (
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"os"
	"testing"
)

func TestHealthCheckListener(t *testing.T) {
	resetEnv()
	os.Setenv("EMISSARY_HEALTH_CHECK_LISTENER", "1.1.1.1:1234")
	os.Setenv("EMISSARY_IDENTIFIER", "spiffe://blah.net/test123")

	log := buildFakeLog()
	config, err := BuildConfig(log)

	assert.Nil(t, err)

	assert.Equal(t, "1.1.1.1:1234", config.GetHealthCheckListener())
}

func TestBadUri(t *testing.T) {
	resetEnv()
	os.Setenv("EMISSARY_LISTENER", "_$%^@#")

	log := buildFakeLog()
	_, err := BuildConfig(log)

	assert.Error(t, err)
	assert.Equal(t, "failed to parse listener uri: _$%^@#", err.Error())
}

func TestBadScheme(t *testing.T) {
	resetEnv()

	os.Setenv("EMISSARY_LISTENER", "bogus:///test123.sock")

	log := buildFakeLog()
	_, err := BuildConfig(log)

	assert.Error(t, err)
	assert.Equal(t, "unknown listener scheme: bogus", err.Error())
}

func TestMissingIdentifier(t *testing.T) {
	resetEnv()

	log := buildFakeLog()
	_, err := BuildConfig(log)

	assert.Error(t, err)
	assert.Equal(t, "no service identifier configured, please set EMISSARY_IDENTIFIER", err.Error())
}

func TestMissingInvalidIdentifier(t *testing.T) {
	resetEnv()

	os.Setenv("EMISSARY_IDENTIFIER", "spiffe://bogus/!@##$%#$%")

	log := buildFakeLog()
	_, err := BuildConfig(log)

	assert.Error(t, err)
	assert.Equal(t, "failed trying to validate audience spiffe id: spiffe://bogus/!@##$%#$%", err.Error())
}

func TestGoodIngressConfig(t *testing.T) {
	resetEnv()

	os.Setenv("EMISSARY_IDENTIFIER", "spiffe://bogus/test123")
	os.Setenv("EMISSARY_INGRESS_MAP", "{\"spiffe://bogus/adsf\": [{\"path\": \"test123\", \"methods\":[\"GET\",\"POST\"]}]}")

	log := buildFakeLog()
	config, err := BuildConfig(log)

	assert.Nil(t, err)
	assert.True(t, config.GetReady())
	assert.Equal(t, []string{"GET", "POST"}, config.GetIngressMapEntry("spiffe://bogus/adsf")[0].Methods)
}

func TestEmtpyIngressConfig(t *testing.T) {
	resetEnv()

	os.Setenv("EMISSARY_IDENTIFIER", "spiffe://bogus/test123")
	os.Setenv("EMISSARY_INGRESS_MAP", "{\"spiffe://bogus/adsf\": []}")

	log := buildFakeLog()
	config, err := BuildConfig(log)

	assert.Error(t, err)
	assert.False(t, config.GetReady())
	assert.Equal(t, "failed trying to validate ingress map, emtpy ACL: spiffe://bogus/adsf - []", err.Error())
}

func TestInvalidSpiffeID(t *testing.T) {
	resetEnv()

	os.Setenv("EMISSARY_IDENTIFIER", "spiffe://bogus/test123")
	os.Setenv("EMISSARY_INGRESS_MAP", "{\"spiffe://bogus/@#$@#$\": [{\"path\":\"/asdf\", \"methods\":[\"GET\"]}]}")

	log := buildFakeLog()
	config, err := BuildConfig(log)

	assert.Error(t, err)
	assert.False(t, config.GetReady())
	assert.Equal(t, "failed trying to validate ingress spiffeID: spiffe://bogus/@#$@#$", err.Error())
}

func TestInvalidIngressMapJSON(t *testing.T) {
	resetEnv()

	os.Setenv("EMISSARY_IDENTIFIER", "spiffe://bogus/test123")
	os.Setenv("EMISSARY_INGRESS_MAP", "}")

	log := buildFakeLog()
	config, err := BuildConfig(log)

	assert.Error(t, err)
	assert.False(t, config.GetReady())
	assert.Equal(t, "ingress map is not valid json: }", err.Error())
}

func TestInvalidIngressMap1(t *testing.T) {
	resetEnv()

	os.Setenv("EMISSARY_IDENTIFIER", "spiffe://bogus/test123")
	os.Setenv("EMISSARY_INGRESS_MAP", "{\"spiffe://bogus/asdf\": [{\"methods\":[\"GET\"]}]}")

	log := buildFakeLog()
	config, err := BuildConfig(log)

	assert.Error(t, err)
	assert.False(t, config.GetReady())
	assert.Equal(t, "failed trying to validate ingress map: spiffe://bogus/asdf - [{ [GET]}] ingress map entry does not contain a path", err.Error())
}

func TestInvalidIngressMap2(t *testing.T) {
	resetEnv()

	os.Setenv("EMISSARY_IDENTIFIER", "spiffe://bogus/test123")
	os.Setenv("EMISSARY_INGRESS_MAP", "{\"spiffe://bogus/asdf\": 123}")

	log := buildFakeLog()
	config, err := BuildConfig(log)

	assert.Error(t, err)
	assert.False(t, config.GetReady())
	assert.Equal(t, "failed trying to validate ingress map, emtpy ACL: spiffe://bogus/asdf - []", err.Error())
}

func TestInvalidEgressMapIdentifier(t *testing.T) {
	resetEnv()

	os.Setenv("EMISSARY_IDENTIFIER", "spiffe://bogus/test123")
	os.Setenv("EMISSARY_INGRESS_MAP", "{\"spiffe://bogus/asdfadsf\": [{\"path\":\"/asdf\", \"methods\":[\"GET\"]}]}")
	os.Setenv("EMISSARY_EGRESS_MAP", "{\"test.blah.net\":\"spiffe://bogus/@#$@#$@#$\"}")

	log := buildFakeLog()
	config, err := BuildConfig(log)

	assert.Error(t, err)
	assert.False(t, config.GetReady())
	assert.Equal(t, "failed trying to validate egress spiffeID: spiffe://bogus/@#$@#$@#$", err.Error())
}

func TestInvalidEgressMapJSON1(t *testing.T) {
	resetEnv()

	os.Setenv("EMISSARY_IDENTIFIER", "spiffe://bogus/test123")
	os.Setenv("EMISSARY_INGRESS_MAP", "{\"spiffe://bogus/asdfadsf\": [{\"path\":\"/asdf\", \"methods\":[\"GET\"]}]}")
	os.Setenv("EMISSARY_EGRESS_MAP", "{123: \"spiffe://bogus/asdfasdfadsf\"}")

	log := buildFakeLog()
	config, err := BuildConfig(log)

	assert.Error(t, err)
	assert.False(t, config.GetReady())
	assert.Equal(t, "egress map is not valid json: {123: \"spiffe://bogus/asdfasdfadsf\"}", err.Error())
}

func TestInvalidEgressMapJSON2(t *testing.T) {
	resetEnv()

	os.Setenv("EMISSARY_IDENTIFIER", "spiffe://bogus/test123")
	os.Setenv("EMISSARY_INGRESS_MAP", "[{\"spiffe://bogus/asdf\":[]}]")
	os.Setenv("EMISSARY_EGRESS_MAP", "}")

	log := buildFakeLog()
	config, err := BuildConfig(log)

	assert.Error(t, err)
	assert.False(t, config.GetReady())
	assert.Equal(t, "egress map is not valid json: }", err.Error())
}

func TestBadDogPortBuildConfig(t *testing.T) {
	resetEnv()

	os.Setenv("EMISSARY_LISTENER", "unix:///test123.sock")
	os.Setenv("EMISSARY_SPIRE_SOCKET", "unix:///agent.sock")
	os.Setenv("EMISSARY_IDENTIFIER", "spiffe://bogus/appname")
	os.Setenv("DOGSTATSD_ENABLED", "true")
	os.Setenv("DOGSTATSD_PORT", "asdf")

	log := buildFakeLog()
	config, err := BuildConfig(log)

	assert.Error(t, err)
	assert.False(t, config.GetReady())
	assert.Equal(t, "failed trying to set up dogstatsd port: 0", err.Error())
}

func TestGoodUnixBuildConfig(t *testing.T) {
	resetEnv()

	os.Setenv("EMISSARY_LISTENER", "unix:///test123.sock")
	os.Setenv("EMISSARY_SPIRE_SOCKET", "unix:///agent.sock")
	os.Setenv("EMISSARY_IDENTIFIER", "spiffe://bogus/appname")
	os.Setenv("EMISSARY_INGRESS_MAP", "{\"spiffe://bogus/app1\":[{\"path\":\"/asdf/123\", \"methods\":[\"GET\"]}],  \"spiffe://bogus/app3\":[{\"path\":\"/test/123\",\"methods\":[\"GET\",\"POST\"]}]}")
	os.Setenv("EMISSARY_EGRESS_MAP", "{\"test.blah.net\":\"spiffe://bogus/test\"}")

	log := buildFakeLog()
	config, err := BuildConfig(log)

	assert.Nil(t, err)

	assert.True(t, config.GetReady())
	assert.Equal(t, "/test123.sock", config.GetListener())
	assert.Equal(t, "unix", config.GetScheme())

	assert.Equal(t, "unix:///agent.sock", config.GetWorkloadSocketPath())

	assert.Equal(t, "spiffe://bogus/appname", config.GetIdentifier())

	assert.Nil(t, config.GetIngressMapEntry("spiffe://bogus/nomatch"))

	assert.Equal(t, "", config.GetEgressMapEntry("notmatch.net"))

	assert.Equal(t, 1, len(config.GetIngressMapEntry("spiffe://bogus/app1")))
	assert.Equal(t, "/asdf/123", config.GetIngressMapEntry("spiffe://bogus/app1")[0].Path)

	assert.Equal(t, 0, len(config.GetIngressMapEntry("spiffe://bogus/app2")))

	assert.Equal(t, 1, len(config.GetIngressMapEntry("spiffe://bogus/app3")))
	assert.Equal(t, "/test/123", config.GetIngressMapEntry("spiffe://bogus/app3")[0].Path)
	assert.Equal(t, []string{"GET", "POST"}, config.GetIngressMapEntry("spiffe://bogus/app3")[0].Methods)

	assert.Equal(t, "spiffe://bogus/test", config.GetEgressMapEntry("test.blah.net"))

	assert.False(t, config.GetDogstatsdEnabled())
}

func TestGoodTcpBuildConfig(t *testing.T) {
	resetEnv()

	os.Setenv("EMISSARY_LISTENER", "tcp://localhost:8080")
	os.Setenv("EMISSARY_SPIRE_SOCKET", "unix:///agent.sock")
	os.Setenv("EMISSARY_IDENTIFIER", "spiffe://bogus/appname")
	os.Setenv("EMISSARY_INGRESS_MAP", "{\"spiffe://bogus/app1\":[{\"path\":\"/asdf/123\", \"methods\":[\"GET\"]}],  \"spiffe://bogus/app3\":[{\"path\":\"/test/1\",\"methods\":[\"GET\",\"POST\"]}, {\"path\":\"/test/2\",\"methods\":[\"POST\",\"GET\"]}]}")
	os.Setenv("EMISSARY_EGRESS_MAP", "{\"test.blah.net\":\"spiffe://bogus/test\"}")
	os.Setenv("DOGSTATSD_ENABLED", "true")
	os.Setenv("DOGSTATSD_HOST", "host123")
	os.Setenv("DOGSTATSD_PORT", "10000")

	log := buildFakeLog()
	config, err := BuildConfig(log)

	assert.Nil(t, err)

	assert.True(t, config.GetReady())

	assert.Equal(t, "localhost:8080", config.GetListener())
	assert.Equal(t, "tcp", config.GetScheme())

	assert.Equal(t, "unix:///agent.sock", config.GetWorkloadSocketPath())

	assert.Equal(t, "spiffe://bogus/appname", config.GetIdentifier())

	assert.Nil(t, config.GetIngressMapEntry("spiffe://bogus/asdfasdfasdfasdf"))

	assert.Equal(t, 1, len(config.GetIngressMapEntry("spiffe://bogus/app1")))

	// ACLs should be ordered
	assert.Equal(t, "/test/1", config.GetIngressMapEntry("spiffe://bogus/app3")[0].Path)
	assert.Equal(t, "GET", config.GetIngressMapEntry("spiffe://bogus/app3")[0].Methods[0])
	assert.Equal(t, "POST", config.GetIngressMapEntry("spiffe://bogus/app3")[0].Methods[1])
	assert.Equal(t, "/test/2", config.GetIngressMapEntry("spiffe://bogus/app3")[1].Path)
	assert.Equal(t, "POST", config.GetIngressMapEntry("spiffe://bogus/app3")[1].Methods[0])
	assert.Equal(t, "GET", config.GetIngressMapEntry("spiffe://bogus/app3")[1].Methods[1])

	assert.Equal(t, "spiffe://bogus/test", config.GetEgressMapEntry("test.blah.net"))

	assert.Equal(t, "", config.GetEgressMapEntry("nomatch.net"))

	assert.True(t, config.GetDogstatsdEnabled())
	assert.Equal(t, "host123", config.GetDogstatsdHost())
	assert.Equal(t, 10000, config.GetDogstatsdPort())

	assert.Equal(t, "0.0.0.0:9191", config.GetHealthCheckListener())
}

func TestDogstatsDDefaultBuildConfig(t *testing.T) {
	resetEnv()

	os.Setenv("EMISSARY_LISTENER", "tcp://localhost:8080")
	os.Setenv("EMISSARY_SPIRE_SOCKET", "unix:///agent.sock")
	os.Setenv("EMISSARY_IDENTIFIER", "spiffe://bogus/appname")
	os.Setenv("EMISSARY_INGRESS_MAP", "{\"spiffe://bogus/app1\":[{\"path\":\"/asdf/123\", \"methods\":[\"GET\"]}],  \"spiffe://bogus/app3\":[{\"path\":\"/test/123\",\"methods\":[\"GET\",\"POST\"]}]}")
	os.Setenv("EMISSARY_EGRESS_MAP", "{\"test.blah.net\":\"spiffe://bogus/test\"}")
	os.Setenv("DOGSTATSD_ENABLED", "asdfasdf")

	log := buildFakeLog()
	config, err := BuildConfig(log)

	assert.Nil(t, err)
	assert.True(t, config.GetReady())
	assert.False(t, config.GetDogstatsdEnabled())
}

func TestLogging(t *testing.T) {
	resetEnv()

	os.Setenv("EMISSARY_LOG_LEVEL", "debug")

	log, err := SetupLogging()

	assert.Nil(t, err)
	assert.Equal(t, "debug", log.GetLevel().String())
}

func TestBadLogLevel(t *testing.T) {
	resetEnv()

	os.Setenv("EMISSARY_LOG_LEVEL", "asdf")

	log, err := SetupLogging()

	assert.Nil(t, log)
	assert.Error(t, err)
	assert.Equal(t, "failed to set log level: not a valid logrus Level: \"asdf\"", err.Error())
}

func TestBadValidateSpiffeID(t *testing.T) {
	status := validateSpiffeID("spiffe://bogus/!@##$%#$%")
	assert.False(t, status)
}

func TestGoodValidateSpiffeID(t *testing.T) {
	status := validateSpiffeID("spiffe://bogus/goodstuff")
	assert.True(t, status)
}

func TestMissingPathValidateIngressMapEntry(t *testing.T) {
	var methods = []string{"GET"}
	acl := ACL{
		Methods: methods,
	}
	status, err := validateIngressMapEntry(acl)
	assert.False(t, status)
	assert.Error(t, err)
	assert.Equal(t, "ingress map entry does not contain a path", err.Error())
}

func TestMissingMethodsValidateIngressMapEntry(t *testing.T) {
	acl := ACL{
		Path: "/asdf",
	}
	status, err := validateIngressMapEntry(acl)
	assert.False(t, status)
	assert.Error(t, err)
	assert.Equal(t, "ingress map entry does not contain any methods", err.Error())
}

func TestZeroMethodsValidateIngressMapEntry(t *testing.T) {
	var methods = []string{}
	acl := ACL{
		Path:    "/asdf",
		Methods: methods,
	}
	status, err := validateIngressMapEntry(acl)
	assert.False(t, status)
	assert.Error(t, err)
	assert.Equal(t, "ingress map entry does not contain any methods", err.Error())
}

func TestNothingValidateIngressMapEntry(t *testing.T) {
	acl := ACL{}
	status, err := validateIngressMapEntry(acl)
	assert.False(t, status)
	assert.Error(t, err)
	assert.Equal(t, "ingress map entry does not contain a path", err.Error())
}

func buildFakeLog() *logrus.Logger {
	log := logrus.New()
	log.SetLevel(logrus.DebugLevel)
	return log
}

func resetEnv() {
	os.Setenv("EMISSARY_LISTENER", "")
	os.Setenv("EMISSARY_SPIRE_SOCKET", "")
	os.Setenv("EMISSARY_IDENTIFIER", "")
	os.Setenv("EMISSARY_INGRESS_MAP", "")
	os.Setenv("EMISSARY_EGRESS_MAP", "")
	os.Setenv("EMISSARY_HEALTH_CHECK_LISTENER", "")
	os.Setenv("DOGSTATSD_ENABLED", "")
	os.Setenv("DOGSTATSD_HOST", "")
	os.Setenv("DOGSTATSD_PORT", "")
}
