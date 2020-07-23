package spire

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"google.golang.org/grpc/metadata"
	"testing"

	"github.com/golang/protobuf/jsonpb"
	structpb "github.com/golang/protobuf/ptypes/struct"

	"github.com/spiffe/go-spiffe/proto/spiffe/workload"

	"github.com/github/emissary/mocks"
)

func TestPrepareContext(t *testing.T) {
	var ctx context.Context

	newCtx, fun := prepareContext(ctx)
	fun()

	md, ok := metadata.FromOutgoingContext(newCtx)
	assert.NotNil(t, ok)

	assert.Equal(t, metadata.New(map[string]string{"workload.spiffe.io": "true"}), md)
}

func TestGetSubject(t *testing.T) {
	claims := make(map[string]string)
	claims["aud"] = "spiffe://domain.test/app123"
	claims["sub"] = "spiffe://domain.test/test123"

	valuesJSON, err := json.Marshal(claims)
	assert.Nil(t, err)

	valuesPB := new(structpb.Struct)
	jsonpb.Unmarshal(bytes.NewReader(valuesJSON), valuesPB)

	response := &workload.ValidateJWTSVIDResponse{
		SpiffeId: "spiffe://testid",
		Claims:   valuesPB,
	}

	subject, err := getSubject(response)

	assert.Nil(t, err)
	assert.Equal(t, "spiffe://domain.test/test123", subject)
}

func TestBadGetSubject(t *testing.T) {
	claims := make(map[string]string)
	claims["aud"] = "spiffe://domain.test/app123"
	claims["notsub"] = "spiffe://domain.test/test123"

	valuesJSON, err := json.Marshal(claims)
	assert.Nil(t, err)

	valuesPB := new(structpb.Struct)
	jsonpb.Unmarshal(bytes.NewReader(valuesJSON), valuesPB)

	response := &workload.ValidateJWTSVIDResponse{
		SpiffeId: "spiffe://testid",
		Claims:   valuesPB,
	}

	subject, err := getSubject(response)

	assert.NotNil(t, err)
	assert.Equal(t, "", subject)
	assert.Equal(t, "jwt missing subject claim", err.Error())
}

func TestBadSubGetSubject(t *testing.T) {
	claims := make(map[string]int)
	claims["sub"] = 1234

	valuesJSON, err := json.Marshal(claims)
	assert.Nil(t, err)

	valuesPB := new(structpb.Struct)
	jsonpb.Unmarshal(bytes.NewReader(valuesJSON), valuesPB)

	response := &workload.ValidateJWTSVIDResponse{
		SpiffeId: "spiffe://testid",
		Claims:   valuesPB,
	}

	subject, err := getSubject(response)

	assert.NotNil(t, err)
	assert.Equal(t, "", subject)
	assert.Equal(t, "jwt subject claim not a string", err.Error())
}

func TestGetSVID(t *testing.T) {
	resp := &workload.JWTSVIDResponse{
		Svids: []*workload.JWTSVID{
			{
				SpiffeId: "spiffe://domain.test/test1",
				Svid:     "SVIDTIME",
			},
		},
	}

	svid, err := getSVID(resp)

	assert.Nil(t, err)
	assert.Equal(t, "SVIDTIME", svid)
}

func TestGetSVIDNoSVID(t *testing.T) {
	resp := &workload.JWTSVIDResponse{
		Svids: []*workload.JWTSVID{},
	}

	svid, err := getSVID(resp)

	assert.NotNil(t, err)
	assert.Equal(t, "", svid)
	assert.Equal(t, "expecting 1 svid in jwt-svid response. found 0", err.Error())
}

func TestGetSVIDMultiple(t *testing.T) {
	resp := &workload.JWTSVIDResponse{
		Svids: []*workload.JWTSVID{
			{
				SpiffeId: "spiffe://domain.test/test1",
				Svid:     "SVIDTIME",
			},
			{
				SpiffeId: "spiffe://domain.test/test2",
				Svid:     "SVIDTIME2",
			},
		},
	}

	_, err := getSVID(resp)

	assert.NotNil(t, err)
	assert.Equal(t, "expecting 1 svid in jwt-svid response. found 2", err.Error())
}

func TestAuthClientFetch(t *testing.T) {
	ctx := context.Background()
	egressAudience := "spiffe://domain.test/receive1"
	spiffeID := "spiffe://domain.test/test1"
	resp := &workload.JWTSVIDResponse{
		Svids: []*workload.JWTSVID{
			{
				SpiffeId: spiffeID,
				Svid:     "SVIDTIME",
			},
		},
	}
	jwtRequest := &workload.JWTSVIDRequest{
		Audience: []string{egressAudience},
		SpiffeId: spiffeID,
	}

	client := &mocks.SpiffeWorkloadAPIClient{}
	client.On("FetchJWTSVID", mock.Anything, jwtRequest).Return(resp, nil)

	ac := AuthClient{
		Ready:        true,
		spiffeClient: client,
	}

	svid, err := ac.Fetch(ctx, spiffeID, egressAudience)

	assert.Nil(t, err)
	assert.Equal(t, "SVIDTIME", svid)
}

func TestAuthClientFetchNoSVID(t *testing.T) {
	ctx := context.Background()
	egressAudience := "spiffe://domain.test/receive1"
	spiffeID := ""
	resp := &workload.JWTSVIDResponse{
		Svids: []*workload.JWTSVID{},
	}
	jwtRequest := &workload.JWTSVIDRequest{
		Audience: []string{egressAudience},
		SpiffeId: spiffeID,
	}

	client := &mocks.SpiffeWorkloadAPIClient{}
	client.On("FetchJWTSVID", mock.Anything, jwtRequest).Return(resp, nil)

	ac := AuthClient{
		Ready:        true,
		spiffeClient: client,
	}

	svid, err := ac.Fetch(ctx, spiffeID, egressAudience)

	assert.NotNil(t, err)
	assert.Equal(t, "", svid)
	assert.Equal(t, "bad fetch from workload api: failed to parse jwt-svid response from spire-agent: expecting 1 svid in jwt-svid response. found 0", err.Error())
}
