package config

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strconv"

	"github.com/sirupsen/logrus"
	"github.com/spiffe/spire/pkg/common/idutil"
)

const (
	defaultListener            = "unix:///run/emissary/sockets/auth.sock"
	defaultWorkloadSocketPath  = "/tmp/agent.sock"
	defaultDogstatsdEnabled    = false
	defaultDogstatsdHost       = "localhost"
	defaultDogstatsdPort       = 28125
	defaultHealthCheckListener = "0.0.0.0:9191"
)

// Config is the emissary config struct
type Config struct {
	ready               bool
	listener            string
	scheme              string
	workloadSocketPath  string
	ingressMap          map[string][]ACL
	identifier          string
	egressMap           map[string]string
	dogstatsdHost       string
	dogstatsdPort       int
	dogstatsdEnabled    bool
	healthCheckListener string
}

// ACL is the struct for each acl in an ingress map
type ACL struct {
	Path    string   `json:"path,omitempty"`
	Methods []string `json:"methods,omitempty"`
}

// BuildConfig returns Config which contains all the data needed for running emissary
func BuildConfig(log *logrus.Logger) (Config, error) {
	var listenerString = defaultListener
	if os.Getenv("EMISSARY_LISTENER") != "" {
		listenerString = os.Getenv("EMISSARY_LISTENER")
	}

	uri, err := url.Parse(listenerString)
	if err != nil {
		err := fmt.Errorf("failed to parse listener uri: %v", listenerString)
		return Config{}, err
	}

	var scheme = uri.Scheme

	var listener string
	switch uri.Scheme {
	case "unix":
		listener = uri.Path
	case "tcp":
		listener = uri.Host
	default:
		err := fmt.Errorf("unknown listener scheme: %v", uri.Scheme)
		return Config{}, err
	}

	var workloadSocketPath = defaultWorkloadSocketPath
	if os.Getenv("EMISSARY_SPIRE_SOCKET") != "" {
		workloadSocketPath = os.Getenv("EMISSARY_SPIRE_SOCKET")
	}

	var identifier = os.Getenv("EMISSARY_IDENTIFIER")
	if identifier == "" {
		err := fmt.Errorf("no service identifier configured, please set EMISSARY_IDENTIFIER")
		return Config{}, err
	}
	if !validateSpiffeID(identifier) {
		err := fmt.Errorf("failed trying to validate audience spiffe id: %v", identifier)
		return Config{}, err
	}

	var ingressMap = make(map[string][]ACL)
	if os.Getenv("EMISSARY_INGRESS_MAP") != "" {
		if !json.Valid([]byte(os.Getenv("EMISSARY_INGRESS_MAP"))) {
			return Config{}, fmt.Errorf("ingress map is not valid json: %v", os.Getenv("EMISSARY_INGRESS_MAP"))
		}

		json.Unmarshal([]byte(os.Getenv("EMISSARY_INGRESS_MAP")), &ingressMap)

		for key, value := range ingressMap {
			if !validateSpiffeID(key) {
				return Config{}, fmt.Errorf("failed trying to validate ingress spiffeID: %v", key)
			}

			if len(value) == 0 {
				return Config{}, fmt.Errorf("failed trying to validate ingress map, emtpy ACL: %v - %v", key, value)
			}

			for _, entry := range value {
				if status, err := validateIngressMapEntry(entry); !status {
					return Config{}, fmt.Errorf("failed trying to validate ingress map: %v - %v %v", key, value, err)
				}
			}
		}
	}

	var egressMap = make(map[string]string)
	if os.Getenv("EMISSARY_EGRESS_MAP") != "" {
		if !json.Valid([]byte(os.Getenv("EMISSARY_EGRESS_MAP"))) {
			return Config{}, fmt.Errorf("egress map is not valid json: %v", os.Getenv("EMISSARY_EGRESS_MAP"))
		}

		json.Unmarshal([]byte(os.Getenv("EMISSARY_EGRESS_MAP")), &egressMap)

		for _, value := range egressMap {
			if !validateSpiffeID(value) {
				return Config{}, fmt.Errorf("failed trying to validate egress spiffeID: %v", value)
			}
		}
	}

	var dogstatsdEnabled bool
	var dogstatsdHost = defaultDogstatsdHost
	var dogstatsdPort = defaultDogstatsdPort
	if os.Getenv("DOGSTATSD_ENABLED") != "" {
		switch os.Getenv("DOGSTATSD_ENABLED") {
		case "true":
			dogstatsdEnabled = true
			if os.Getenv("DOGSTATSD_HOST") != "" {
				dogstatsdHost = os.Getenv("DOGSTATSD_HOST")
			}
			if os.Getenv("DOGSTATSD_PORT") != "" {
				dogstatsdPort, err = strconv.Atoi(os.Getenv("DOGSTATSD_PORT"))
				if err != nil {
					return Config{}, fmt.Errorf("failed trying to set up dogstatsd port: %v", dogstatsdPort)
				}
			}
		default:
			dogstatsdEnabled = defaultDogstatsdEnabled
		}
	}

	var healthCheckListener = defaultHealthCheckListener
	if os.Getenv("EMISSARY_HEALTH_CHECK_LISTENER") != "" {
		healthCheckListener = os.Getenv("EMISSARY_HEALTH_CHECK_LISTENER")
	}

	// convert the resulting ingress and egress config into json for logging the config
	jsonIngress, err := json.Marshal(ingressMap)
	if err != nil {
		return Config{}, fmt.Errorf("failed trying to log ingress map: %v", ingressMap)
	}

	jsonEgress, err := json.Marshal(egressMap)
	if err != nil {
		return Config{}, fmt.Errorf("failed trying to log egress map: %v", egressMap)
	}

	log.WithFields(logrus.Fields{
		"listener":              listener,
		"health_check_listener": healthCheckListener,
		"scheme":                scheme,
		"spire_socket":          workloadSocketPath,
		"identifier":            identifier,
		"log_level":             log.GetLevel(),
		"ingress_mapping":       string(jsonIngress),
		"egress_mapping":        string(jsonEgress),
	}).Info("emissary configuration")

	if dogstatsdEnabled {
		log.WithFields(logrus.Fields{
			"dogstatsd_host": dogstatsdHost,
			"dogstatsd_port": dogstatsdPort,
		}).Info("dogstatsd configuration")
	}

	return Config{
		ready:               true,
		listener:            listener,
		scheme:              scheme,
		workloadSocketPath:  workloadSocketPath,
		identifier:          identifier,
		ingressMap:          ingressMap,
		egressMap:           egressMap,
		dogstatsdHost:       dogstatsdHost,
		dogstatsdPort:       dogstatsdPort,
		dogstatsdEnabled:    dogstatsdEnabled,
		healthCheckListener: healthCheckListener,
	}, nil
}

func (c Config) GetReady() bool {
	return c.ready
}

func (c Config) GetListener() string {
	return c.listener
}

func (c Config) GetScheme() string {
	return c.scheme
}

func (c Config) GetWorkloadSocketPath() string {
	return c.workloadSocketPath
}

func (c Config) GetIdentifier() string {
	return c.identifier
}

func (c Config) GetIngressMapEntry(spiffeid string) []ACL {
	return c.ingressMap[spiffeid]
}

func (c Config) GetEgressMapEntry(spiffeid string) string {
	return c.egressMap[spiffeid]
}

func (c Config) GetDogstatsdHost() string {
	return c.dogstatsdHost
}

func (c Config) GetDogstatsdPort() int {
	return c.dogstatsdPort
}

func (c Config) GetDogstatsdEnabled() bool {
	return c.dogstatsdEnabled
}

func (c Config) GetHealthCheckListener() string {
	return c.healthCheckListener
}

// SetupLogging sets up and configures logging
func SetupLogging() (*logrus.Logger, error) {
	log := logrus.New()
	log.SetOutput(os.Stdout)
	log.SetLevel(logrus.InfoLevel)
	//log.SetReportCaller(true)

	if os.Getenv("EMISSARY_LOG_LEVEL") != "" {
		level, err := logrus.ParseLevel(os.Getenv("EMISSARY_LOG_LEVEL"))
		if err != nil {
			return nil, fmt.Errorf("failed to set log level: %v", err)
		}

		log.SetLevel(level)
	}
	return log, nil
}

// make sure spiffe ids have the right format
func validateSpiffeID(spiffeID string) bool {
	if err := idutil.ValidateSpiffeID(spiffeID, idutil.AllowAny()); err != nil {
		return false
	}
	return true
}

// all ingress map entries need a identifier and an acl, each acl requires a path and methods
func validateIngressMapEntry(entry ACL) (bool, error) {
	pathExists := entry.Path
	methodsExists := entry.Methods
	methodsSize := len(entry.Methods)

	if pathExists == "" {
		return false, fmt.Errorf("ingress map entry does not contain a path")
	}

	if methodsExists == nil {
		return false, fmt.Errorf("ingress map entry does not contain any methods")
	}

	if methodsSize == 0 {
		return false, fmt.Errorf("ingress map entry does not contain any methods")
	}

	return true, nil
}
