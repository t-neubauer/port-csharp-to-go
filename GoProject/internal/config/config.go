package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Options struct {
	Addr               string
	DefaultMaxAttempts int
	LeaseDuration      time.Duration
	RetryBackoff       time.Duration
	WorkerPollInterval time.Duration
	WorkerEnabled      bool
}

func Load() (Options, error) {
	options := Options{
		Addr:               getString("JOBDISPATCH_ADDR", ":8080"),
		DefaultMaxAttempts: 3,
		LeaseDuration:      5 * time.Minute,
		RetryBackoff:       30 * time.Second,
		WorkerPollInterval: 15 * time.Minute,
	}

	var err error
	if options.DefaultMaxAttempts, err = getPositiveInt("JobDispatch__DefaultMaxAttempts", options.DefaultMaxAttempts); err != nil {
		return Options{}, err
	}
	if options.LeaseDuration, err = getPositiveDuration("JobDispatch__LeaseDuration", options.LeaseDuration, false); err != nil {
		return Options{}, err
	}
	if options.RetryBackoff, err = getPositiveDuration("JobDispatch__RetryBackoff", options.RetryBackoff, true); err != nil {
		return Options{}, err
	}
	if options.WorkerPollInterval, err = getPositiveDuration("JobDispatch__WorkerPollInterval", options.WorkerPollInterval, false); err != nil {
		return Options{}, err
	}
	if options.WorkerEnabled, err = getBool("JobDispatch__WorkerEnabled", false); err != nil {
		return Options{}, err
	}
	return options, nil
}

func getString(name, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		return value
	}
	return fallback
}

func getPositiveInt(name string, fallback int) (int, error) {
	value := getString(name, strconv.Itoa(fallback))
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		return 0, fmt.Errorf("%s must be a positive integer", name)
	}
	return parsed, nil
}

func getPositiveDuration(name string, fallback time.Duration, allowZero bool) (time.Duration, error) {
	value := getString(name, fallback.String())
	parsed, err := time.ParseDuration(value)
	if err != nil {
		parts := strings.Split(value, ":")
		if len(parts) == 3 {
			var hours, minutes, seconds int
			_, err = fmt.Sscanf(value, "%d:%d:%d", &hours, &minutes, &seconds)
			parsed = time.Duration(hours)*time.Hour +
				time.Duration(minutes)*time.Minute +
				time.Duration(seconds)*time.Second
		}
	}
	if err != nil || parsed < 0 || (!allowZero && parsed == 0) {
		return 0, fmt.Errorf("%s must be a valid duration", name)
	}
	return parsed, nil
}

func getBool(name string, fallback bool) (bool, error) {
	value := getString(name, strconv.FormatBool(fallback))
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return false, fmt.Errorf("%s must be true or false", name)
	}
	return parsed, nil
}
