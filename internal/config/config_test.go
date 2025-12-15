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

	// Now test env takes priority when flags are empty.
	runtime = Resolve(Flags{}, env, fileCfg)
	if runtime.Profile != env.Profile {
		t.Fatalf("profile env precedence failed, got %s", runtime.Profile)
	}
	if runtime.Region != env.Region {
		t.Fatalf("region env precedence failed, got %s", runtime.Region)
	}

	// Config fallback.
	runtime = Resolve(Flags{}, Env{}, fileCfg)
	if runtime.Profile != fileCfg.DefaultProfile {
		t.Fatalf("profile config precedence failed, got %s", runtime.Profile)
	}
	if runtime.Region != fileCfg.LastRegion {
		t.Fatalf("region config precedence failed, got %s", runtime.Region)
	}
}
