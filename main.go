package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"go-uptime-monitor/internal/monitor"
)

type fileConfig struct {
	WebhookURL string           `json:"webhook_url"`
	Targets    []monitor.Target `json:"targets"`
}

func loadConfig(path string) (fileConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return fileConfig{}, err
	}
	var cfg fileConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return fileConfig{}, err
	}
	seen := make(map[string]bool)
	for i := range cfg.Targets {
		name := strings.TrimSpace(cfg.Targets[i].Name)
		if name == "" {
			return fileConfig{}, fmt.Errorf("target %d has a blank name", i+1)
		}
		if seen[name] {
			return fileConfig{}, fmt.Errorf("duplicate target name %q", name)
		}
		seen[name] = true
		cfg.Targets[i].Name = name
	}
	return cfg, nil
}

func main() {
	configPath := flag.String("config", "config.json", "path to config JSON file")
	listenAddr := flag.String("addr", ":8080", "address to serve /status on")
	flag.Parse()

	cfg, err := loadConfig(*configPath)
	if err != nil {
		log.Fatalf("loading config %s: %v", *configPath, err)
	}
	if len(cfg.Targets) == 0 {
		log.Fatalf("config %s defines no targets", *configPath)
	}

	var notifier monitor.Notifier = monitor.LogNotifier{}
	if cfg.WebhookURL != "" {
		notifier = monitor.NewWebhookNotifier(cfg.WebhookURL)
	}

	m := monitor.New(notifier, monitor.HTTPChecker)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	m.Start(ctx, cfg.Targets)
	log.Printf("watching %d target(s)", len(cfg.Targets))

	mux := http.NewServeMux()
	mux.HandleFunc("/status", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(m.Snapshot())
	})
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	srv := &http.Server{Addr: *listenAddr, Handler: mux}
	go func() {
		<-ctx.Done()
		srv.Close()
	}()

	log.Printf("serving /status on %s", *listenAddr)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("server error: %v", err)
	}
}
