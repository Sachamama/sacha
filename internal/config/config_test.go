package config

import (
	"path/filepath"
	"testing"
)

func TestSaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	want := &Config{
		DefaultProfile: "dev",
		DefaultRegion:  "us-east-1",
		LastProfile:    "staging",
		LastRegion:     "us-west-2",
		LastService:    "cloudwatch-logs",
	}

	if err := Save(path, want); err != nil {
		t.Fatalf("save: %v", err)
	}

	got, err := Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	if *got != *want {
		t.Fatalf("config mismatch: got %+v want %+v", got, want)
	}
}

func TestResolvePrecedence(t *testing.T) {
	fileCfg := &Config{
		DefaultProfile: "file-profile",
		DefaultRegion:  "file-region",
		LastProfile:    "file-last-profile",
		LastRegion:     "file-last-region",
		LastService:    "file-service",
	}

	flags := Flags{
		Profile: "flag-profile",
		Region:  "flag-region",
		Service: "flag-service",
	}
	env := Env{
		Profile: "env-profile",
		Region:  "env-region",
	}

	// CLI flags take priority over everything.
	runtime := Resolve(flags, env, fileCfg)

	if runtime.Profile != flags.Profile {
		t.Fatalf("profile precedence failed, got %s", runtime.Profile)
	}
	if runtime.Region != flags.Region {
		t.Fatalf("region precedence failed, got %s", runtime.Region)
	}
	if runtime.Service != flags.Service {
		t.Fatalf("service precedence failed, got %s", runtime.Service)
	}

	// Env takes priority when flags are empty.
	runtime = Resolve(Flags{}, env, fileCfg)
	if runtime.Profile != env.Profile {
		t.Fatalf("profile env precedence failed, got %s", runtime.Profile)
	}
	if runtime.Region != env.Region {
		t.Fatalf("region env precedence failed, got %s", runtime.Region)
	}

	// LastProfile takes priority over DefaultProfile.
	runtime = Resolve(Flags{}, Env{}, fileCfg)
	if runtime.Profile != fileCfg.LastProfile {
		t.Fatalf("profile last-used precedence failed, got %s", runtime.Profile)
	}
	if runtime.Region != fileCfg.LastRegion {
		t.Fatalf("region config precedence failed, got %s", runtime.Region)
	}

	// DefaultProfile used when LastProfile is empty.
	fileCfgNoLast := &Config{
		DefaultProfile: "file-profile",
		DefaultRegion:  "file-region",
	}
	runtime = Resolve(Flags{}, Env{}, fileCfgNoLast)
	if runtime.Profile != fileCfgNoLast.DefaultProfile {
		t.Fatalf("profile default precedence failed, got %s", runtime.Profile)
	}
}

func TestResolveEndpointPrecedence(t *testing.T) {
	// Flag takes priority over env.
	runtime := Resolve(
		Flags{Endpoint: "http://flag:4566"},
		Env{Endpoint: "http://env:4566"},
		nil,
	)
	if runtime.Endpoint != "http://flag:4566" {
		t.Fatalf("endpoint flag precedence failed, got %s", runtime.Endpoint)
	}

	// Env used when flag is empty.
	runtime = Resolve(Flags{}, Env{Endpoint: "http://env:4566"}, nil)
	if runtime.Endpoint != "http://env:4566" {
		t.Fatalf("endpoint env precedence failed, got %s", runtime.Endpoint)
	}

	// Empty when neither set.
	runtime = Resolve(Flags{}, Env{}, nil)
	if runtime.Endpoint != "" {
		t.Fatalf("endpoint should be empty, got %s", runtime.Endpoint)
	}
}
