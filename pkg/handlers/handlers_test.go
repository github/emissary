package handlers

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"net/http"
	"net/http/httptest"

	"github.com/github/emissary/mocks"
	"github.com/github/emissary/pkg/config"
)

func TestHealthCheckHandler(t *testing.T) {
	ctx := context.Background()
	resetEnv()
	log := buildFakeLog()

	req, err := http.NewRequest("GET", "/", nil)
	if err != nil {
		t.Fatal(err)
	}

	healthMock := &mocks.HealthCheck{}

	healthMock.On("RunCheck", ctx).Return(true, nil)

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		HealthHandler(ctx, log, healthMock, w, r)
	})

	handler.ServeHTTP(rr, req)

	assert.Equal(t, 200, rr.Code)
}

func TestBadHealthCheckHandler(t *testing.T) {
	ctx := context.Background()
	resetEnv()
	log := buildFakeLog()

	req, err := http.NewRequest("GET", "/", nil)
	if err != nil {
		t.Fatal(err)
	}

	healthMock := &mocks.HealthCheck{}

	healthMock.On("RunCheck", ctx).Return(false, nil)

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		HealthHandler(ctx, log, healthMock, w, r)
	})

	handler.ServeHTTP(rr, req)

	assert.Equal(t, 503, rr.Code)
}

func TestErrorHealthCheckHandler(t *testing.T) {
	ctx := context.Background()
	resetEnv()
	log := buildFakeLog()

	req, err := http.NewRequest("GET", "/", nil)
	if err != nil {
		t.Fatal(err)
	}
	req = req.WithContext(ctx)

	healthMock := &mocks.HealthCheck{}

	healthMock.On("RunCheck", ctx).Return(true, fmt.Errorf("error"))

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		HealthHandler(r.Context(), log, healthMock, w, r)
	})

	handler.ServeHTTP(rr, req)

	assert.Equal(t, 500, rr.Code)
}

func TestBadModeAuthHandler(t *testing.T) {
	ctx := context.Background()
	resetEnv()

	os.Setenv("EMISSARY_LISTENER", "tcp://localhost:8080")
	os.Setenv("EMISSARY_SPIRE_SOCKET", "unix:///agent.sock")
	os.Setenv("EMISSARY_IDENTIFIER", "spiffe://bogus/appname")

	log := buildFakeLog()
	config, err := config.BuildConfig(log)
	assert.Nil(t, err)

	req, err := http.NewRequest("GET", "/", nil)
	if err != nil {
		t.Fatal(err)
	}

	req.Header.Add("x-emissary-mode", "badmode")

	jwtsvidMock := &mocks.JWTSVID{}

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		AuthHandler(ctx, log, jwtsvidMock, config, w, r)
	})

	handler.ServeHTTP(rr, req)

	assert.Equal(t, 412, rr.Code)
}

// happy path
func TestIngressModeAuthHandler(t *testing.T) {
	ctx := context.Background()
	resetEnv()

	os.Setenv("EMISSARY_LISTENER", "tcp://localhost:8080")
	os.Setenv("EMISSARY_SPIRE_SOCKET", "unix:///agent.sock")
	os.Setenv("EMISSARY_IDENTIFIER", "spiffe://bogus/appname")
	os.Setenv("EMISSARY_INGRESS_MAP", "{\"spiffe://goodsubject\":[{\"path\":\"/asdf\", \"methods\":[\"GET\"]}]}")

	log := buildFakeLog()
	config, err := config.BuildConfig(log)
	assert.Nil(t, err)

	req, err := http.NewRequest("GET", "/asdf", nil)
	if err != nil {
		t.Fatal(err)
	}
	req = req.WithContext(ctx)
	req.Header.Add("x-emissary-mode", "ingress")
	req.Header.Add("x-emissary-auth", "bearer validsvid")

	jwtsvidMock := &mocks.JWTSVID{}

	jwtsvidMock.On("Validate", ctx, mock.Anything, mock.Anything).Return(true, "spiffe://goodsubject", nil)

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		AuthHandler(r.Context(), log, jwtsvidMock, config, w, r)
	})

	handler.ServeHTTP(rr, req)

	assert.Equal(t, 200, rr.Code)
	assert.Equal(t, "success", rr.Header()["X-Emissary-Auth-Status"][0])
}

func TestIngressModeBadPathAuthHandler(t *testing.T) {
	ctx := context.Background()
	os.Setenv("EMISSARY_LISTENER", "tcp://localhost:8080")
	os.Setenv("EMISSARY_SPIRE_SOCKET", "unix:///agent.sock")
	os.Setenv("EMISSARY_IDENTIFIER", "spiffe://bogus/appname")
	os.Setenv("EMISSARY_INGRESS_MAP", "{\"spiffe://goodsubject\":[{\"path\":\"/asdf\", \"methods\":[\"GET\"]}]}")

	log := buildFakeLog()
	config, err := config.BuildConfig(log)
	assert.Nil(t, err)

	req, err := http.NewRequest("GET", "/nomatch", nil)
	if err != nil {
		t.Fatal(err)
	}
	req = req.WithContext(ctx)
	req.Header.Add("x-emissary-mode", "ingress")
	req.Header.Add("x-emissary-auth", "bearer validsvid")

	jwtsvidMock := &mocks.JWTSVID{}

	jwtsvidMock.On("Validate", ctx, mock.Anything, mock.Anything).Return(true, "spiffe://goodsubject", nil)

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		AuthHandler(r.Context(), log, jwtsvidMock, config, w, r)
	})

	handler.ServeHTTP(rr, req)

	assert.Equal(t, 403, rr.Code)
	assert.Equal(t, "failure", rr.Header()["X-Emissary-Auth-Status"][0])
}

func TestIngressModeBadMethodAuthHandler(t *testing.T) {
	ctx := context.Background()
	os.Setenv("EMISSARY_LISTENER", "tcp://localhost:8080")
	os.Setenv("EMISSARY_SPIRE_SOCKET", "unix:///agent.sock")
	os.Setenv("EMISSARY_IDENTIFIER", "spiffe://bogus/appname")
	os.Setenv("EMISSARY_INGRESS_MAP", "{\"spiffe://goodsubject\":[{\"path\":\"/asdf\", \"methods\":[\"GET\"]}]}")

	log := buildFakeLog()
	config, err := config.BuildConfig(log)
	assert.Nil(t, err)

	req, err := http.NewRequest("HEAD", "/asdf", nil)
	if err != nil {
		t.Fatal(err)
	}
	req = req.WithContext(ctx)
	req.Header.Add("x-emissary-mode", "ingress")
	req.Header.Add("x-emissary-auth", "bearer validsvid")

	jwtsvidMock := &mocks.JWTSVID{}

	jwtsvidMock.On("Validate", ctx, mock.Anything, mock.Anything).Return(true, "spiffe://goodsubject", nil)

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		AuthHandler(r.Context(), log, jwtsvidMock, config, w, r)
	})

	handler.ServeHTTP(rr, req)

	assert.Equal(t, 403, rr.Code)
	assert.Equal(t, "failure", rr.Header()["X-Emissary-Auth-Status"][0])
}

func TestNoAccessIngressModeAuthHandler(t *testing.T) {
	ctx := context.Background()
	resetEnv()

	os.Setenv("EMISSARY_LISTENER", "tcp://localhost:8080")
	os.Setenv("EMISSARY_SPIRE_SOCKET", "unix:///agent.sock")
	os.Setenv("EMISSARY_IDENTIFIER", "spiffe://bogus/appname")
	os.Setenv("EMISSARY_INGRESS_MAP", "{\"spiffe://goodsubject\":[{\"path\":\"/asdf\", \"methods\":[\"GET\"]}]}")

	log := buildFakeLog()
	config, err := config.BuildConfig(log)
	assert.Nil(t, err)

	req, err := http.NewRequest("GET", "/asdf", nil)
	if err != nil {
		t.Fatal(err)
	}
	req = req.WithContext(ctx)
	req.Header.Add("x-emissary-mode", "ingress")
	req.Header.Add("x-emissary-auth", "bearer validsvid")

	jwtsvidMock := &mocks.JWTSVID{}

	jwtsvidMock.On("Validate", ctx, mock.Anything, mock.Anything).Return(true, "notallowed", nil)

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		AuthHandler(r.Context(), log, jwtsvidMock, config, w, r)
	})

	handler.ServeHTTP(rr, req)

	assert.Equal(t, 403, rr.Code)
	assert.Equal(t, "failure", rr.Header()["X-Emissary-Auth-Status"][0])
}

func TestNoSVIDIngressAuthHandler(t *testing.T) {
	ctx := context.Background()
	resetEnv()

	os.Setenv("EMISSARY_LISTENER", "tcp://localhost:8080")
	os.Setenv("EMISSARY_SPIRE_SOCKET", "unix:///agent.sock")
	os.Setenv("EMISSARY_IDENTIFIER", "spiffe://bogus/appname")
	os.Setenv("EMISSARY_INGRESS_MAP", "{\"spiffe://goodsubject\":[{\"path\":\"/asdf\", \"methods\":[\"GET\"]}]}")

	log := buildFakeLog()
	config, err := config.BuildConfig(log)
	assert.Nil(t, err)

	req, err := http.NewRequest("GET", "/", nil)
	if err != nil {
		t.Fatal(err)
	}
	req = req.WithContext(ctx)
	req.Header.Add("x-emissary-mode", "ingress")

	jwtsvidMock := &mocks.JWTSVID{}

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		AuthHandler(r.Context(), log, jwtsvidMock, config, w, r)
	})

	handler.ServeHTTP(rr, req)

	assert.Equal(t, 403, rr.Code)
	assert.Equal(t, "failure", rr.Header()["X-Emissary-Auth-Status"][0])
}

func TestValidateErrorIngressModeAuthHandler(t *testing.T) {
	ctx := context.Background()
	resetEnv()

	os.Setenv("EMISSARY_LISTENER", "tcp://localhost:8080")
	os.Setenv("EMISSARY_SPIRE_SOCKET", "unix:///agent.sock")
	os.Setenv("EMISSARY_IDENTIFIER", "spiffe://bogus/appname")
	os.Setenv("EMISSARY_INGRESS_MAP", "{\"spiffe://goodsubject\":[{\"path\":\"/asdf\", \"methods\":[\"GET\"]}]}")

	log := buildFakeLog()
	config, err := config.BuildConfig(log)
	assert.Nil(t, err)

	req, err := http.NewRequest("GET", "/", nil)
	if err != nil {
		t.Fatal(err)
	}
	req = req.WithContext(ctx)
	req.Header.Add("x-emissary-mode", "ingress")
	req.Header.Add("x-emissary-auth", "bearer validsvid")

	jwtsvidMock := &mocks.JWTSVID{}

	jwtsvidMock.On("Validate", ctx, mock.Anything, mock.Anything).Return(true, "", fmt.Errorf("bad validate"))

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		AuthHandler(r.Context(), log, jwtsvidMock, config, w, r)
	})

	handler.ServeHTTP(rr, req)

	assert.Equal(t, 403, rr.Code)
	assert.Equal(t, "failure", rr.Header()["X-Emissary-Auth-Status"][0])
}

func TestInvalidSVIDIngressModeAuthHandler(t *testing.T) {
	ctx := context.Background()
	resetEnv()

	os.Setenv("EMISSARY_LISTENER", "tcp://localhost:8080")
	os.Setenv("EMISSARY_SPIRE_SOCKET", "unix:///agent.sock")
	os.Setenv("EMISSARY_IDENTIFIER", "spiffe://bogus/appname")
	os.Setenv("EMISSARY_INGRESS_MAP", "{\"spiffe://goodsubject\":[{\"path\":\"/asdf\", \"methods\":[\"GET\"]}]}")

	log := buildFakeLog()
	config, err := config.BuildConfig(log)
	assert.Nil(t, err)

	req, err := http.NewRequest("GET", "/", nil)
	if err != nil {
		t.Fatal(err)
	}
	req = req.WithContext(ctx)
	req.Header.Add("x-emissary-mode", "ingress")
	req.Header.Add("x-emissary-auth", "bearer validsvid")

	jwtsvidMock := &mocks.JWTSVID{}

	jwtsvidMock.On("Validate", ctx, mock.Anything, mock.Anything).Return(false, "test123", nil)

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		AuthHandler(r.Context(), log, jwtsvidMock, config, w, r)
	})

	handler.ServeHTTP(rr, req)

	assert.Equal(t, 403, rr.Code)
	assert.Equal(t, "failure", rr.Header()["X-Emissary-Auth-Status"][0])
}

// happy path
func TestEgressModeAuthHandler(t *testing.T) {
	ctx := context.Background()
	resetEnv()

	os.Setenv("EMISSARY_LISTENER", "tcp://localhost:8080")
	os.Setenv("EMISSARY_SPIRE_SOCKET", "unix:///agent.sock")
	os.Setenv("EMISSARY_IDENTIFIER", "spiffe://bogus/appname")
	os.Setenv("EMISSARY_EGRESS_MAP", "{\"testhandler.blah.net\":\"spiffe://bogus/test\"}")

	log := buildFakeLog()
	config, err := config.BuildConfig(log)
	assert.Nil(t, err)

	assert.Nil(t, err)
	req, err := http.NewRequest("GET", "/", nil)
	if err != nil {
		t.Fatal(err)
	}
	req = req.WithContext(ctx)

	req.Header.Add("x-emissary-mode", "egress")
	req.Host = "testhandler.blah.net"

	jwtsvidMock := &mocks.JWTSVID{}

	jwtsvidMock.On("Fetch", ctx, mock.Anything, mock.Anything).Return("thisisavalidsvid", nil)

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		AuthHandler(r.Context(), log, jwtsvidMock, config, w, r)
	})

	handler.ServeHTTP(rr, req)

	// go will camel case header names automatically
	assert.Equal(t, "bearer thisisavalidsvid", rr.HeaderMap["X-Emissary-Auth"][0])
	assert.Equal(t, 200, rr.Code)
}

func TestHasSVIDEgressModeAuthHandler(t *testing.T) {
	resetEnv()

	os.Setenv("EMISSARY_LISTENER", "tcp://localhost:8080")
	os.Setenv("EMISSARY_SPIRE_SOCKET", "unix:///agent.sock")
	os.Setenv("EMISSARY_IDENTIFIER", "spiffe://bogus/appname")
	os.Setenv("EMISSARY_EGRESS_MAP", "{\"testhandler.blah.net\":\"spiffe://bogus/test\"}")

	log := buildFakeLog()
	config, err := config.BuildConfig(log)
	assert.Nil(t, err)

	req, err := http.NewRequest("GET", "/", nil)
	if err != nil {
		t.Fatal(err)
	}

	req.Header.Add("x-emissary-mode", "egress")
	req.Header.Add("x-emissary-auth", "bearer validsvid")

	jwtsvidMock := &mocks.JWTSVID{}

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		AuthHandler(r.Context(), log, jwtsvidMock, config, w, r)
	})

	handler.ServeHTTP(rr, req)

	assert.Equal(t, 403, rr.Code)
}

func TestMissingHostEgressModeAuthHandler(t *testing.T) {
	resetEnv()

	os.Setenv("EMISSARY_LISTENER", "tcp://localhost:8080")
	os.Setenv("EMISSARY_SPIRE_SOCKET", "unix:///agent.sock")
	os.Setenv("EMISSARY_IDENTIFIER", "spiffe://bogus/appname")
	os.Setenv("EMISSARY_EGRESS_MAP", "{\"testhandler.blah.net\":\"spiffe://bogus/test\"}")

	log := buildFakeLog()
	config, err := config.BuildConfig(log)
	assert.Nil(t, err)

	req, err := http.NewRequest("GET", "/", nil)
	if err != nil {
		t.Fatal(err)
	}

	req.Header.Add("x-emissary-mode", "egress")

	jwtsvidMock := &mocks.JWTSVID{}

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		AuthHandler(r.Context(), log, jwtsvidMock, config, w, r)
	})

	handler.ServeHTTP(rr, req)

	assert.Equal(t, 403, rr.Code)
}

func TestMissingMappingEgressModeAuthHandler(t *testing.T) {
	resetEnv()

	os.Setenv("EMISSARY_LISTENER", "tcp://localhost:8080")
	os.Setenv("EMISSARY_SPIRE_SOCKET", "unix:///agent.sock")
	os.Setenv("EMISSARY_IDENTIFIER", "spiffe://bogus/appname")
	os.Setenv("EMISSARY_EGRESS_MAP", "{\"testhandler.blah.net\":\"spiffe://bogus/test\"}")

	log := buildFakeLog()
	config, err := config.BuildConfig(log)
	assert.Nil(t, err)

	req, err := http.NewRequest("GET", "/", nil)
	if err != nil {
		t.Fatal(err)
	}

	req.Header.Add("x-emissary-mode", "egress")
	req.Host = "nomatch"

	jwtsvidMock := &mocks.JWTSVID{}

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		AuthHandler(r.Context(), log, jwtsvidMock, config, w, r)
	})

	handler.ServeHTTP(rr, req)

	assert.Equal(t, 403, rr.Code)
}

func TestBadFetchEgressModeAuthHandler(t *testing.T) {
	ctx := context.Background()
	resetEnv()

	os.Setenv("EMISSARY_LISTENER", "tcp://localhost:8080")
	os.Setenv("EMISSARY_SPIRE_SOCKET", "unix:///agent.sock")
	os.Setenv("EMISSARY_IDENTIFIER", "spiffe://bogus/appname")
	os.Setenv("EMISSARY_EGRESS_MAP", "{\"testhandler.blah.net\":\"spiffe://bogus/test\"}")

	log := buildFakeLog()
	config, err := config.BuildConfig(log)
	assert.Nil(t, err)

	req, err := http.NewRequest("GET", "/", nil)
	if err != nil {
		t.Fatal(err)
	}

	req.Header.Add("x-emissary-mode", "egress")
	req.Host = "testhandler.blah.net"

	jwtsvidMock := &mocks.JWTSVID{}

	jwtsvidMock.On("Fetch", ctx, mock.Anything, mock.Anything).Return("", fmt.Errorf("bad fetch"))

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		AuthHandler(r.Context(), log, jwtsvidMock, config, w, r)
	})

	handler.ServeHTTP(rr, req)

	assert.Equal(t, 403, rr.Code)
}

func TestIsPathAndMethodAllowed(t *testing.T) {
	acls := []config.ACL{
		config.ACL{
			Path:    "/path1",
			Methods: []string{"GET", "POST", "PUT"},
		},
		config.ACL{
			Path:    "/path2",
			Methods: []string{"PATCH", "GET", "HEAD"},
		},
		config.ACL{
			Path:    "/path3",
			Methods: []string{"OPTIONS", "PUT", "GET"},
		},
	}

	status1, aclIndex1, err1 := isPathAndMethodAllowed(acls, "GET", "/path1")
	assert.True(t, status1)
	assert.Nil(t, err1)
	assert.Equal(t, 0, aclIndex1)

	status2, aclIndex2, err2 := isPathAndMethodAllowed(acls, "PUT", "/path1/something/deep")
	assert.True(t, status2)
	assert.Nil(t, err2)
	assert.Equal(t, 0, aclIndex2)

	status3, aclIndex3, err3 := isPathAndMethodAllowed(acls, "PATCH", "/path2/123123")
	assert.True(t, status3)
	assert.Nil(t, err3)
	assert.Equal(t, 1, aclIndex3)

	status4, aclIndex4, err4 := isPathAndMethodAllowed(acls, "BADMETHOD", "/path2/123123")
	assert.False(t, status4)
	assert.Equal(t, "no matching ACL: path match: true, method match: false", err4.Error())
	assert.Equal(t, 65535, aclIndex4)

	status5, aclIndex5, err5 := isPathAndMethodAllowed(acls, "OPTIONS", "/path3123123123")
	assert.True(t, status5)
	assert.Nil(t, err5)
	assert.Equal(t, 2, aclIndex5)

	status6, aclIndex6, err6 := isPathAndMethodAllowed(acls, "GET", "/pathasdfadsf")
	assert.False(t, status6)
	assert.Equal(t, "no matching ACL: path match: false, method match: unknown", err6.Error())
	assert.Equal(t, 65535, aclIndex6)
}

func TestOverlappingIsPathAndMethodAllowed(t *testing.T) {
	acls := []config.ACL{
		config.ACL{
			Path:    "/pathx",
			Methods: []string{"GET"},
		},
		config.ACL{
			Path:    "/path",
			Methods: []string{"PATCH"},
		},
		config.ACL{
			Path:    "/pathy",
			Methods: []string{"OPTIONS"},
		},
		config.ACL{
			Path:    "/",
			Methods: []string{"PUT"},
		},
	}

	status1, aclIndex1, err1 := isPathAndMethodAllowed(acls, "GET", "/pathx")
	assert.True(t, status1)
	assert.Nil(t, err1)
	assert.Equal(t, 0, aclIndex1)

	status2, aclIndex2, err2 := isPathAndMethodAllowed(acls, "PATCH", "/pathasdfadsf")
	assert.True(t, status2)
	assert.Nil(t, err2)
	assert.Equal(t, 1, aclIndex2)

	// /pathy and PATCH should never be reached since /path will always match first
	status3, aclIndex3, err3 := isPathAndMethodAllowed(acls, "PATCH", "/pathy")
	assert.True(t, status3)
	assert.Nil(t, err3)
	assert.Equal(t, 1, aclIndex3)

	// /pathy and OPTIONS should be reached since /path and PATCH wont match first
	status4, aclIndex4, err4 := isPathAndMethodAllowed(acls, "OPTIONS", "/pathy")
	assert.True(t, status4)
	assert.Nil(t, err4)
	assert.Equal(t, 2, aclIndex4)

	// slash should match but the method wont
	status5, aclIndex5, err5 := isPathAndMethodAllowed(acls, "GET", "/catchall")
	assert.False(t, status5)
	assert.Equal(t, "no matching ACL: path match: true, method match: false", err5.Error())
	assert.Equal(t, 65535, aclIndex5)

	status6, aclIndex6, err6 := isPathAndMethodAllowed(acls, "PUT", "/catchall")
	assert.True(t, status6)
	assert.Nil(t, err6)
	assert.Equal(t, 3, aclIndex6)

	status7, aclIndex7, err7 := isPathAndMethodAllowed(acls, "PUT", "/pathy/asdf")
	assert.True(t, status7)
	assert.Nil(t, err7)
	assert.Equal(t, 3, aclIndex7)

	status8, aclIndex8, err8 := isPathAndMethodAllowed(acls, "PUT", "/pathx/asdf")
	assert.True(t, status8)
	assert.Nil(t, err8)
	assert.Equal(t, 3, aclIndex8)
}

func TestOverlappingOrderingIsPathAndMethodAllowed1(t *testing.T) {
	acls := []config.ACL{
		config.ACL{
			Path:    "/pathx",
			Methods: []string{"GET"},
		},
		config.ACL{
			Path:    "/pathy",
			Methods: []string{"OPTIONS"},
		},
		config.ACL{
			Path:    "/path",
			Methods: []string{"PATCH"},
		},
		config.ACL{
			Path:    "/",
			Methods: []string{"PUT"},
		},
	}

	status1, aclIndex1, err1 := isPathAndMethodAllowed(acls, "GET", "/pathx")
	assert.True(t, status1)
	assert.Nil(t, err1)
	assert.Equal(t, 0, aclIndex1)

	status2, aclIndex2, err2 := isPathAndMethodAllowed(acls, "PATCH", "/pathasdfadsf")
	assert.True(t, status2)
	assert.Nil(t, err2)
	assert.Equal(t, 2, aclIndex2)

	// /pathy and PATCH should never be reached since /path will always match first
	status3, aclIndex3, err3 := isPathAndMethodAllowed(acls, "PATCH", "/pathy")
	assert.True(t, status3)
	assert.Nil(t, err3)
	assert.Equal(t, 2, aclIndex3)

	// /pathy and OPTIONS should be reached since /path and PATCH wont match first
	status4, aclIndex4, err4 := isPathAndMethodAllowed(acls, "OPTIONS", "/pathy")
	assert.True(t, status4)
	assert.Nil(t, err4)
	assert.Equal(t, 1, aclIndex4)

	// slash should match but the method wont
	status5, aclIndex5, err5 := isPathAndMethodAllowed(acls, "GET", "/catchall")
	assert.False(t, status5)
	assert.Equal(t, "no matching ACL: path match: true, method match: false", err5.Error())
	assert.Equal(t, 65535, aclIndex5)

	status6, aclIndex6, err6 := isPathAndMethodAllowed(acls, "PUT", "/catchall")
	assert.True(t, status6)
	assert.Nil(t, err6)
	assert.Equal(t, 3, aclIndex6)

	status7, aclIndex7, err7 := isPathAndMethodAllowed(acls, "PUT", "/pathy/asdf")
	assert.True(t, status7)
	assert.Nil(t, err7)
	assert.Equal(t, 3, aclIndex7)

	status8, aclIndex8, err8 := isPathAndMethodAllowed(acls, "PUT", "/pathx/asdf")
	assert.True(t, status8)
	assert.Nil(t, err8)
	assert.Equal(t, 3, aclIndex8)
}

func TestOverlappingOrderingIsPathAndMethodAllowed2(t *testing.T) {
	acls := []config.ACL{
		config.ACL{
			Path:    "/pathx",
			Methods: []string{"GET", "PATCH"},
		},
		config.ACL{
			Path:    "/pathy",
			Methods: []string{"OPTIONS"},
		},
		config.ACL{
			Path:    "/",
			Methods: []string{"PUT"},
		},
		config.ACL{
			Path:    "/path",
			Methods: []string{"PATCH", "GET"},
		},
	}

	status1, aclIndex1, err1 := isPathAndMethodAllowed(acls, "GET", "/pathx")
	assert.True(t, status1)
	assert.Nil(t, err1)
	assert.Equal(t, 0, aclIndex1)

	status2, aclIndex2, err2 := isPathAndMethodAllowed(acls, "PATCH", "/pathasdfadsf")
	assert.True(t, status2)
	assert.Nil(t, err2)
	assert.Equal(t, 3, aclIndex2)

	// /pathy and PATCH should never be reached since /path will always match first
	status3, aclIndex3, err3 := isPathAndMethodAllowed(acls, "PATCH", "/pathy")
	assert.True(t, status3)
	assert.Nil(t, err3)
	assert.Equal(t, 3, aclIndex3)

	// /pathy and OPTIONS should be reached since /path and PATCH wont match first
	status4, aclIndex4, err4 := isPathAndMethodAllowed(acls, "OPTIONS", "/pathy")
	assert.True(t, status4)
	assert.Nil(t, err4)
	assert.Equal(t, 1, aclIndex4)

	// slash should match but the method wont
	status5, aclIndex5, err5 := isPathAndMethodAllowed(acls, "GET", "/catchall")
	assert.False(t, status5)
	assert.Equal(t, "no matching ACL: path match: true, method match: false", err5.Error())
	assert.Equal(t, 65535, aclIndex5)

	status6, aclIndex6, err6 := isPathAndMethodAllowed(acls, "PUT", "/catchall")
	assert.True(t, status6)
	assert.Nil(t, err6)
	assert.Equal(t, 2, aclIndex6)

	status7, aclIndex7, err7 := isPathAndMethodAllowed(acls, "PUT", "/pathy/asdf")
	assert.True(t, status7)
	assert.Nil(t, err7)
	assert.Equal(t, 2, aclIndex7)

	status8, aclIndex8, err8 := isPathAndMethodAllowed(acls, "PUT", "/pathx/asdf")
	assert.True(t, status8)
	assert.Nil(t, err8)
	assert.Equal(t, 2, aclIndex8)

	status9, aclIndex9, err9 := isPathAndMethodAllowed(acls, "GET", "/pathx/asdf")
	assert.True(t, status9)
	assert.Nil(t, err9)
	assert.Equal(t, 0, aclIndex9)

	status10, aclIndex10, err10 := isPathAndMethodAllowed(acls, "GET", "/pathy/asdf")
	assert.True(t, status10)
	assert.Nil(t, err10)
	assert.Equal(t, 3, aclIndex10)
}

func TestLogSVID(t *testing.T) {
	assert.Equal(t, "aaaaaaaaaaaaaaaaaaaaaaaaa...", logSVID("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"))
	assert.Equal(t, "a...", logSVID("a"))
}

func TestGoodParseJWTHeader(t *testing.T) {
	suffix, status := parseJWTHeader("bearer asdf")
	assert.True(t, status)
	assert.Equal(t, "asdf", suffix)
}

func TestBadParseJWTHeader(t *testing.T) {
	suffix, status := parseJWTHeader("bogus asdf")
	assert.False(t, status)
	assert.Equal(t, "", suffix)
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
