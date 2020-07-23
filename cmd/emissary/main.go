package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/github/emissary/pkg/config"
	"github.com/github/emissary/pkg/handlers"
	"github.com/github/emissary/pkg/spire"
	"github.com/github/emissary/pkg/stats"
)

const (
	readTimeout  = 5 * time.Second
	writeTimeout = 10 * time.Second
	idleTimeout  = 120 * time.Second
)

func startServer(log *logrus.Logger, c config.Config) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	authClient, err := spire.NewAuthClient(ctx, c.GetWorkloadSocketPath())
	if err != nil {
		return fmt.Errorf("failed to create auth client: %v", err)
	}
	if !authClient.Ready {
		return fmt.Errorf("failed to create auth client")
	}

	authListener, err := net.Listen(c.GetScheme(), c.GetListener())
	if err != nil {
		return fmt.Errorf("failed to listen for auth server: %v", err)
	}

	healthListener, err := net.Listen("tcp", c.GetHealthCheckListener())
	if err != nil {
		return fmt.Errorf("failed to listen for auth server: %v", err)
	}

	authHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := handlers.AuthHandler(r.Context(), log, authClient, c, w, r); err != nil {
			log.Errorf("auth http server had a problem: %v", err)
		}
	})

	healthConfig, err := spire.NewHealthConfig(c.GetWorkloadSocketPath())
	if err != nil {
		return fmt.Errorf("failed to create auth client: %v", err)
	}

	healthHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := handlers.HealthHandler(r.Context(), log, healthConfig, w, r); err != nil {
			log.Errorf("health check http server had a problem: %v", err)
		}
	})

	authServer := &http.Server{
		Handler:      authHandler,
		ReadTimeout:  readTimeout,
		WriteTimeout: writeTimeout,
		IdleTimeout:  idleTimeout,
	}

	healthServer := &http.Server{
		Handler:      healthHandler,
		ReadTimeout:  readTimeout,
		WriteTimeout: writeTimeout,
		IdleTimeout:  idleTimeout,
	}

	channel := make(chan os.Signal, 1)
	signal.Notify(channel, os.Interrupt, os.Kill, syscall.SIGTERM)

	go func(channel chan os.Signal) {
		sig := <-channel
		log.Printf("caught signal (%s), shutting down", sig)
		authServer.Close()
		healthServer.Close()
		os.Exit(0)
	}(channel)

	go func() error {
		if err := healthServer.Serve(healthListener); err != nil {
			return fmt.Errorf("failed to run health check server: %v", err)
		}
		return nil
	}()

	if err := authServer.Serve(authListener); err != nil {
		return fmt.Errorf("failed to run auth server: %v", err)
	}

	return nil
}

func main() {
	log, err := config.SetupLogging()
	if err != nil {
		fmt.Println("error setting up logs:", err)
		os.Exit(1)
	}

	builtConfig, err := config.BuildConfig(log)
	if err != nil {
		log.Fatalln(err)
	}
	if !builtConfig.GetReady() {
		log.Fatalln("fail to build config")
	}

	if builtConfig.GetDogstatsdEnabled() {
		if err := stats.Configure(builtConfig.GetDogstatsdHost(), builtConfig.GetDogstatsdPort()); err != nil {
			log.Fatalln("error configuring dogstatsd", err)
		}
	}

	stats.Client().Gauge("start", 1, nil, 1)

	if err := startServer(log, builtConfig); err != nil {
		log.Fatalln(err)
	}
}
