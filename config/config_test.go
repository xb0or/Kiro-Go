package config

import (
	"path/filepath"
	"testing"
)

func TestUpdateSettingsPatchPreservesOmittedAPIKeyFields(t *testing.T) {
	if err := Init(filepath.Join(t.TempDir(), "config.json")); err != nil {
		t.Fatalf("init config: %v", err)
	}
	if err := UpdateSettings("proxy-api-key", true, "admin-password"); err != nil {
		t.Fatalf("seed settings: %v", err)
	}

	if err := UpdateSettingsPatch(nil, nil, "new-admin-password"); err != nil {
		t.Fatalf("patch settings: %v", err)
	}

	if got := GetApiKey(); got != "proxy-api-key" {
		t.Fatalf("expected API key to be preserved, got %q", got)
	}
	if !IsApiKeyRequired() {
		t.Fatalf("expected requireApiKey to stay enabled")
	}
	if got := GetPassword(); got != "new-admin-password" {
		t.Fatalf("expected password to update, got %q", got)
	}
}

func TestUpdateSettingsPatchCanExplicitlyDisableAPIKey(t *testing.T) {
	if err := Init(filepath.Join(t.TempDir(), "config.json")); err != nil {
		t.Fatalf("init config: %v", err)
	}
	if err := UpdateSettings("proxy-api-key", true, "admin-password"); err != nil {
		t.Fatalf("seed settings: %v", err)
	}

	emptyKey := ""
	requireAPIKey := false
	if err := UpdateSettingsPatch(&emptyKey, &requireAPIKey, ""); err != nil {
		t.Fatalf("patch settings: %v", err)
	}

	if got := GetApiKey(); got != "" {
		t.Fatalf("expected API key to be cleared, got %q", got)
	}
	if IsApiKeyRequired() {
		t.Fatalf("expected requireApiKey to be disabled")
	}
	if got := GetPassword(); got != "admin-password" {
		t.Fatalf("expected password to be preserved, got %q", got)
	}
}

func TestClientModeDefaultsAndAccountOverride(t *testing.T) {
	if err := Init(filepath.Join(t.TempDir(), "config.json")); err != nil {
		t.Fatalf("init config: %v", err)
	}

	if got := GetClientMode(); got != ClientModeKiroIDE {
		t.Fatalf("expected default client mode kiro-ide, got %q", got)
	}
	if got := EffectiveClientMode(&Account{}); got != ClientModeKiroIDE {
		t.Fatalf("expected empty account to use global kiro-ide, got %q", got)
	}

	if err := UpdateClientMode(ClientModeKiroCLI); err != nil {
		t.Fatalf("update global client mode: %v", err)
	}
	if got := EffectiveClientMode(&Account{}); got != ClientModeKiroCLI {
		t.Fatalf("expected empty account to use global kiro-cli, got %q", got)
	}
	if got := EffectiveClientMode(&Account{ClientMode: ClientModeKiroIDE}); got != ClientModeKiroIDE {
		t.Fatalf("expected account override kiro-ide, got %q", got)
	}
}
