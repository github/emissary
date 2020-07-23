package spire

import (
	"context"
	"fmt"
	"net"
	"time"

	structpb "github.com/golang/protobuf/ptypes/struct"
	"github.com/spiffe/go-spiffe/proto/spiffe/workload"
	api_workload "github.com/spiffe/spire/api/workload"
	api_workload_dial "github.com/spiffe/spire/api/workload/dial"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// JWTSVID is the interface for the JWT stuff
type JWTSVID interface {
	Fetch(ctx context.Context, spiffeID string, egressAudience string) (string, error)
	Validate(ctx context.Context, svid string, identifier string) (bool, string, error)
}

// AuthClient is the the struct that carries the data needed to communicate with spire-agent in the http handler
type AuthClient struct {
	Ready        bool
	spiffeClient workload.SpiffeWorkloadAPIClient
}

// HealthCheck interface to the health check stuff
type HealthCheck interface {
	RunCheck(ctx context.Context) (bool, error)
}

// HealthConfig is the config for the health check
type HealthConfig struct {
	socketPath string
}

// NewHealthConfig do the thing
func NewHealthConfig(socketPath string) (HealthConfig, error) {
	return HealthConfig{
		socketPath: socketPath,
	}, nil
}

// RunCheck checks to see if spire-agent is running
func (h HealthConfig) RunCheck(ctx context.Context) (bool, error) {
	client := getHealthCheckClient(h.socketPath)
	defer client.Stop()

	errCh := make(chan error, 1)
	go func() {
		errCh <- client.Start()
	}()

	select {
	case err := <-errCh:
		// https://godoc.org/google.golang.org/grpc/codes
		if status.Code(err) > 0 {
			return false, fmt.Errorf("spire-agent is unavailable: %v", err)
		}
	case <-client.UpdateChan():
	}

	return true, nil
}

// NewAuthClient returns AuthClient for communicating with spire-agent
func NewAuthClient(ctx context.Context, socketPath string) (AuthClient, error) {
	spiffeClient, err := getSpiffeClient(ctx, socketPath)
	if err != nil {
		return AuthClient{}, fmt.Errorf("fail to create spiffe client: %v", err)
	}

	return AuthClient{
		Ready:        true,
		spiffeClient: spiffeClient,
	}, nil
}

// Fetch uses AuthClient to fetch JWT-SVIDs from spire-agent, returns a JWT-SVID
func (s AuthClient) Fetch(ctx context.Context, spiffeID string, egressAudience string) (string, error) {
	svid, err := s.fetchJWTSVIDViaWorkloadAPI(ctx, spiffeID, egressAudience)
	if err != nil {
		return "", fmt.Errorf("bad fetch from workload api: %v", err)
	}
	return svid, nil
}

// Validate uses AuthClient to validate JWT-SVIDs with spire-agent, also returns subject contained in the JWT-SVID
func (s AuthClient) Validate(ctx context.Context, svid string, identifier string) (bool, string, error) {
	subject, err := s.validateJWTSVIDViaWorkloadAPI(ctx, svid, identifier)
	if err != nil {
		return false, "", fmt.Errorf("invalid jwt signature: %v", err)
	}

	return true, subject, nil
}

// a wrapper around the spiffe client's FetchJWTSVID method
func (s AuthClient) fetchJWTSVIDViaWorkloadAPI(ctx context.Context, spiffeID string, egressAudience string) (string, error) {
	ctx, cancel := prepareContext(ctx)
	defer cancel()

	jwtRequest := &workload.JWTSVIDRequest{
		Audience: []string{egressAudience},
		SpiffeId: spiffeID,
	}

	jwtSVID, err := s.spiffeClient.FetchJWTSVID(ctx, jwtRequest)
	if err != nil {
		return "", fmt.Errorf("spiffe client failed to fetch jwt-svid from spire-agent: %v", err)
	}

	svid, err := getSVID(jwtSVID)
	if err != nil {
		return "", fmt.Errorf("failed to parse jwt-svid response from spire-agent: %v", err)
	}

	return svid, nil
}

// a wrapper around the spiffe client's ValidateJWTSVID method
func (s AuthClient) validateJWTSVIDViaWorkloadAPI(ctx context.Context, svid string, identifier string) (string, error) {
	ctx, cancel := prepareContext(ctx)
	defer cancel()

	jwtRequest := &workload.ValidateJWTSVIDRequest{
		Audience: identifier,
		Svid:     svid,
	}

	response, err := s.spiffeClient.ValidateJWTSVID(ctx, jwtRequest)
	if err != nil {
		// unwrap invalid argument error
		// https://github.com/spiffe/spire/blob/3a23cf4ae75fc440a9fffe7bee3775aec0cad16c/pkg/agent/endpoints/workload/handler.go#L151
		if s, ok := status.FromError(err); ok {
			if s.Code() == codes.InvalidArgument {
				return "", fmt.Errorf(s.Message())
			}
		}
		return "", err
	}

	subject, err := getSubject(response)
	if err != nil {
		return "", err
	}

	return subject, nil
}

func getSpiffeClient(ctx context.Context, socketPath string) (workload.SpiffeWorkloadAPIClient, error) {
	conn, err := api_workload_dial.Dial(ctx, &net.UnixAddr{
		Name: socketPath,
		Net:  "unix",
	})
	if err != nil {
		return nil, err
	}
	return workload.NewSpiffeWorkloadAPIClient(conn), nil
}

func getHealthCheckClient(socketPath string) api_workload.X509Client {
	addr := &net.UnixAddr{
		Name: socketPath,
		Net:  "unix",
	}

	client := api_workload.NewX509Client(&api_workload.X509ClientConfig{
		Addr:        addr,
		FailOnError: true,
		Timeout:     5 * time.Second,
	})

	return client
}

func getSubject(response *workload.ValidateJWTSVIDResponse) (string, error) {
	subjectField := response.Claims.GetFields()["sub"]
	if subjectField == nil {
		return "", fmt.Errorf("jwt missing subject claim")
	}

	subject, ok := subjectField.Kind.(*structpb.Value_StringValue)
	if !ok {
		return "", fmt.Errorf("jwt subject claim not a string")
	}

	return subject.StringValue, nil
}

func getSVID(jwtsvid *workload.JWTSVIDResponse) (string, error) {
	if len(jwtsvid.Svids) != 1 {
		return "", fmt.Errorf("expecting 1 svid in jwt-svid response. found %d", len(jwtsvid.Svids))
	}

	return jwtsvid.Svids[0].GetSvid(), nil
}

func prepareContext(ctx context.Context) (context.Context, func()) {
	header := metadata.Pairs("workload.spiffe.io", "true")
	ctx = metadata.NewOutgoingContext(ctx, header)
	return ctx, func() {}
}
