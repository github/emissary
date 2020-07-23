package handlers

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/github/emissary/pkg/config"
	"github.com/github/emissary/pkg/spire"
	"github.com/github/emissary/pkg/stats"
)

const (
	// JWTsvidHeaderKey is the header we use for carrying the JWT-SVID
	JWTsvidHeaderKey = "x-emissary-auth"
	// JWTModeHeader is the header we use to indicate the direction of request flow
	JWTModeHeader = "x-emissary-mode"
	bearerStr     = "bearer "
	// AuthStatusHeaderKey is set to success or failure on ingress requests
	AuthStatusHeaderKey = "x-emissary-auth-status"
)

// HealthHandler is an http handler that is used for health checking emissary, it makes a call to spire-agent to ensure it's responding
func HealthHandler(ctx context.Context, log *logrus.Logger, h spire.HealthCheck, w http.ResponseWriter, r *http.Request) error {
	userAgent := r.Header.Get("User-Agent")
	method := r.Method
	path := r.URL.Path
	log.WithFields(logrus.Fields{
		"method":      method,
		"path":        path,
		"user_agent":  userAgent,
		"host_header": r.Host,
	}).Debug("health check request received")

	start := time.Now()
	healthy, err := h.RunCheck(ctx)
	took := time.Since(start)
	stats.Timing("health_check_time", took, []string{})

	// spire-agent is unavailable in some way
	if !healthy {
		stats.IncrFail("health_check_request", []string{"type:unhealthy"})
		log.Errorf("health check failure: %v", err)
		w.WriteHeader(http.StatusServiceUnavailable)
		fmt.Fprintf(w, "spire-agent is unhealthy\n")
		return nil
	}

	// there was some sort of error communicating with spire-agent
	if err != nil {
		stats.IncrFail("health_check_request", []string{"type:error"})
		log.Errorf("health check failure: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "contacting spire-agent had an error\n")
		return nil
	}

	// all clear
	stats.IncrSuccess("health_check_request", []string{})
	log.Debugf("health check success")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "emissary and spire-agent are live\n")
	return nil
}

// AuthHandler is an http handler that calls to the appropriate function depending on direction of request flow
func AuthHandler(ctx context.Context, log *logrus.Logger, jwtClient spire.JWTSVID, c config.Config, w http.ResponseWriter, r *http.Request) error {
	modeHeader := r.Header.Get(JWTModeHeader)
	method := r.Method
	path := r.URL.Path

	log.WithFields(logrus.Fields{
		"method":      method,
		"path":        path,
		"mode_header": modeHeader,
		"host_header": r.Host,
	}).Info("request received")

	switch modeHeader {
	case "ingress":
		start := time.Now()
		ingressMode(ctx, log, jwtClient, c, w, r)
		took := time.Since(start)
		stats.Timing("request_time", took, []string{"mode:ingress"})
	case "egress":
		start := time.Now()
		egressMode(ctx, log, jwtClient, c, w, r)
		took := time.Since(start)
		stats.Timing("request_time", took, []string{"mode:egress"})
	default:
		log.WithFields(logrus.Fields{
			"mode": modeHeader,
		}).Error("unknown mode, request denied")
		stats.IncrFail("request", []string{"mode:" + modeHeader})

		// unknown mode
		w.WriteHeader(http.StatusPreconditionFailed)
	}

	return nil
}

// ingresMode validates a JWT on the way into a service and ensures the origin of the request is allowed to communicate with a service
func ingressMode(ctx context.Context, log *logrus.Logger, jwtClient spire.JWTSVID, c config.Config, w http.ResponseWriter, r *http.Request) {
	jwtHeader := r.Header.Get(JWTsvidHeaderKey)
	svid, hasSVID := parseJWTHeader(jwtHeader)

	// deny any request that doesn't include a JWT-SVID
	if !hasSVID {
		log.WithFields(logrus.Fields{
			"mode": "ingress",
		}).Error("request does not contain a jwt header, request denied")
		stats.IncrFail("request", []string{"mode:ingress", "has_svid:false"})

		setIngressFailureResponse(w)
		return
	}

	start := time.Now()
	jwtStatus, subject, err := jwtClient.Validate(ctx, svid, c.GetIdentifier())
	took := time.Since(start)

	stats.Timing("validate_time", took, []string{"remote:" + subject, "mode:ingress"})

	// deny any request where there is an error validating the JWT-SVID
	if err != nil {
		log.WithFields(logrus.Fields{
			"svid":   logSVID(svid),
			"error":  err,
			"remote": subject,
			"mode":   "ingress",
		}).Error("svid validation error, request denied")
		stats.IncrFail("request", []string{"remote:" + subject, "mode:ingress"})

		setIngressFailureResponse(w)
		return
	}

	// deny any request that contains an invalid JWT-SVID
	if !jwtStatus {
		log.WithFields(logrus.Fields{
			"svid":    logSVID(svid),
			"remote":  subject,
			"mode":    "ingress",
			"subject": subject,
		}).Error("svid not valid, request denied")
		stats.IncrFail("request", []string{"remote:" + subject, "mode:ingress"})

		setIngressFailureResponse(w)
		return
	}

	// deny any request that originates from a disallowed spiffe id
	if c.GetIngressMapEntry(subject) == nil {
		log.WithFields(logrus.Fields{
			"svid":    logSVID(svid),
			"remote":  subject,
			"mode":    "ingress",
			"subject": subject,
		}).Error("subject not an allowed spiffeID, request denied")
		stats.IncrFail("request", []string{"remote:" + subject, "mode:ingress"})

		setIngressFailureResponse(w)
		return
	}

	result, aclIndex, err := isPathAndMethodAllowed(c.GetIngressMapEntry(subject), r.Method, r.URL.Path)
	if result != true {
		log.WithFields(logrus.Fields{
			"svid":    logSVID(svid),
			"remote":  subject,
			"mode":    "ingress",
			"subject": subject,
			"err":     err,
		}).Error(err)
		stats.IncrFail("request", []string{"remote:" + subject, "mode:ingress"})

		setIngressFailureResponse(w)
		return
	}

	log.WithFields(logrus.Fields{
		"svid":      logSVID(svid),
		"remote":    subject,
		"mode":      "ingress",
		"acl_index": aclIndex,
	}).Info("svid allowed, request accepted")
	stats.IncrSuccess("request", []string{"remote:" + subject, "mode:ingress"})

	// if all the above checks pass the request is approved
	setIngressSuccessResponse(w)
}

// egressMode fetches a JWT-SVID from spire-agent and injects the JWT on the way out of a service
func egressMode(ctx context.Context, log *logrus.Logger, jwtClient spire.JWTSVID, c config.Config, w http.ResponseWriter, r *http.Request) {
	jwtHeader := r.Header.Get(JWTsvidHeaderKey)
	svid, hasSVID := parseJWTHeader(jwtHeader)

	// deny any request if it already contains a JWT-SVID, this may be a spoof attempt
	if hasSVID {
		log.WithFields(logrus.Fields{
			"svid": logSVID(svid),
			"mode": "egress",
		}).Error("request already contains a jwt header, request denied")
		stats.IncrFail("request", []string{"mode:egress", "has_svid:true"})

		setEgressFailureResponse(w)
		return
	}

	// deny any request that does not contain a host header, we don't know where to send it
	if r.Host == "" {
		log.WithFields(logrus.Fields{
			"mode": "egress",
		}).Error("request does not contain host header, denying request")
		stats.IncrFail("request", []string{"mode:egress", "host:false"})

		setEgressFailureResponse(w)
		return
	}

	egressAudience := c.GetEgressMapEntry(r.Host)

	// deny any request if we don't have a cooresponding matching spiffe id for the contents of the host header
	if egressAudience == "" {
		log.WithFields(logrus.Fields{
			"host_header": r.Host,
			"mode":        "egress",
			"remote":      egressAudience,
		}).Error("no matching spiffeid for host header, request denied")
		stats.IncrFail("request", []string{"remote:" + egressAudience, "mode:egress"})

		setEgressFailureResponse(w)
		return
	}

	log.WithFields(logrus.Fields{
		"host_header": r.Host,
		"mode":        "egress",
		"remote":      egressAudience,
	}).Info("egress mapping success")

	start := time.Now()
	jwtSVID, err := jwtClient.Fetch(ctx, c.GetIdentifier(), egressAudience)
	took := time.Since(start)
	stats.Timing("fetch_time", took, []string{"remote:" + egressAudience, "mode:egress"})

	// deny any request that results in an error while fetching the JWT-SVID from spire-agent
	if err != nil {
		log.WithFields(logrus.Fields{
			"host_header": r.Host,
			"mode":        "egress",
			"remote":      egressAudience,
			"error":       err,
		}).Error("error fetching jwt svid, request denied")
		stats.IncrFail("request", []string{"remote:" + egressAudience, "mode:egress"})

		setEgressFailureResponse(w)
		return
	}

	log.WithFields(logrus.Fields{
		"svid":        logSVID(jwtSVID),
		"host_header": r.Host,
		"mode":        "egress",
		"remote":      egressAudience,
	}).Info("injected svid header, request accepted")
	stats.IncrSuccess("request", []string{"remote:" + egressAudience, "mode:egress"})

	// if all the above checks pass the request is approved and the JWT-SVID is injected
	setEgressSuccessResponse(w, jwtSVID)
}

// the request is allowed by the first ACL to match
func isPathAndMethodAllowed(acl []config.ACL, method string, path string) (bool, int, error) {
	var pathMatch = false
	var methMatch = "unknown"

	for aclIndex, entry := range acl {
		if strings.HasPrefix(path, entry.Path) {
			pathMatch = true
			for _, value := range entry.Methods {
				if value == method {
					return true, aclIndex, nil
				}
			}
			methMatch = "false"
		}
	}

	// if we get here the either the path or methods didnt match
	// pathMatch can be true or false depending on the first if statement
	// method match is always false or unknown since either:
	// a) it was never reached because the path didnt match (i.e unknown) or ..
	// b) the method indeed was tested and did not match
	// we return 65535 as the unlikely negative case for the ACL index rather than 0 which is very likely
	return false, 65535, fmt.Errorf("no matching ACL: path match: %v, method match: %v", pathMatch, methMatch)
}

func setIngressFailureResponse(w http.ResponseWriter) {
	w.Header().Set(AuthStatusHeaderKey, "failure")
	w.WriteHeader(http.StatusForbidden)
}

func setIngressSuccessResponse(w http.ResponseWriter) {
	w.Header().Set(AuthStatusHeaderKey, "success")
	w.WriteHeader(http.StatusOK)
}

func setEgressFailureResponse(w http.ResponseWriter) {
	w.WriteHeader(http.StatusForbidden)
}

func setEgressSuccessResponse(w http.ResponseWriter, jwtSVID string) {
	w.Header().Set(JWTsvidHeaderKey, fmt.Sprintf("%s%s", bearerStr, jwtSVID))
	w.WriteHeader(http.StatusOK)
}

// get token from svid
func parseJWTHeader(header string) (string, bool) {
	suffix := strings.TrimPrefix(header, bearerStr)
	if suffix == header {
		return "", false
	}
	return suffix, true
}

func logSVID(s string) string {
	var maxLen = 25
	if len(s) > maxLen {
		return fmt.Sprintf("%v...", string(s[0:maxLen]))
	}
	return fmt.Sprintf("%v...", s)
}
