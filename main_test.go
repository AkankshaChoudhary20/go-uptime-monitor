package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadConfigTargetNames(t *testing.T) {
	tests := []struct {
		name    string
		config  string
		wantErr string
	}{
		{"unknown root field", `{"webhok_url":"https://example.com","targets":[]}`, "unknown field"},
		{"unknown target field", `{"targets":[{"name":"api","timout":5}]}`, "unknown field"},
		{"trailing object", `{"targets":[]} {"targets":[]}`, "exactly one"},
		{"trailing garbage", `{"targets":[]} invalid`, "exactly one"},
		{"blank", `{"targets":[{"name":""}]}`, "blank name"},
		{"whitespace", `{"targets":[{"name":" \t\n "}]}`, "blank name"},
		{"missing", `{"targets":[{"url":"https://example.com"}]}`, "blank name"},
		{"duplicate", `{"targets":[{"name":"api"},{"name":"api"}]}`, "duplicate target name"},
		{"trimmed duplicate", `{"targets":[{"name":"api"},{"name":" api "}]}`, "duplicate target name"},
		{"valid", `{"targets":[{"name":" api "},{"name":"web"}]}`, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.json")
			if err := os.WriteFile(path, []byte(tt.config), 0600); err != nil {
				t.Fatal(err)
			}
			cfg, err := loadConfig(path)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("got %v, want error containing %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if cfg.Targets[0].Name != "api" || cfg.Targets[1].Name != "web" {
				t.Fatalf("unexpected target names: %+v", cfg.Targets)
			}
		})
	}
}
