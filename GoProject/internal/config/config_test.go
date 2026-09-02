package config

import (
	"os"
	"testing"
	"time"
)

func TestLoadUsesDefaults(t *testing.T) {
	clearConfigEnvironment(t)
	options, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if options.DefaultMaxAttempts != 3 || options.LeaseDuration != 5*time.Minute ||
		options.RetryBackoff != 30*time.Second || options.WorkerPollInterval != 15*time.Minute ||
		options.WorkerEnabled {
		t.Fatalf("unexpected defaults: %+v", options)
	}
}

func TestLoadParsesEnvironmentConfiguration(t *testing.T) {
	clearConfigEnvironment(t)
	t.Setenv("JobDispatch__DefaultMaxAttempts", "5")
	t.Setenv("JobDispatch__LeaseDuration", "2m")
	t.Setenv("JobDispatch__RetryBackoff", "0")
	t.Setenv("JobDispatch__WorkerPollInterval", "250ms")
	t.Setenv("JobDispatch__WorkerEnabled", "true")

	options, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if options.DefaultMaxAttempts != 5 || options.LeaseDuration != 2*time.Minute ||
		options.RetryBackoff != 0 || options.WorkerPollInterval != 250*time.Millisecond ||
		!options.WorkerEnabled {
		t.Fatalf("unexpected parsed options: %+v", options)
	}
}

func TestLoadRejectsInvalidEnvironmentConfiguration(t *testing.T) {
	clearConfigEnvironment(t)
	t.Setenv("JobDispatch__WorkerPollInterval", "0")
	if _, err := Load(); err == nil {
		t.Fatal("expected invalid worker interval error")
	}
}

func clearConfigEnvironment(t *testing.T) {
	t.Helper()
	for _, name := range []string{
		"JOBDISPATCH_ADDR",
		"JobDispatch__DefaultMaxAttempts",
		"JobDispatch__LeaseDuration",
		"JobDispatch__RetryBackoff",
		"JobDispatch__WorkerPollInterval",
		"JobDispatch__WorkerEnabled",
	} {
		if value, ok := os.LookupEnv(name); ok {
			t.Setenv(name, "")
			_ = value
		}
	}
}
