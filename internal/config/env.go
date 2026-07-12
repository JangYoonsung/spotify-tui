package config

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

// loadEnvFile parses simple KEY=VALUE lines (# comments, blank lines
// ignored) and calls os.Setenv for any key not already set in the real
// environment — real env vars always win over the file. Missing file is
// not an error.
func loadEnvFile(path string) error {
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(strings.Trim(strings.TrimSpace(value), `"'`))
		if _, alreadySet := os.LookupEnv(key); !alreadySet {
			_ = os.Setenv(key, value)
		}
	}
	return scanner.Err()
}

// LoadEnv looks for a .env file, first in the current working directory,
// then in ~/.config/spotify-tui-go/.env — the latter matters because this
// tool is typically launched from a cmux dock.json control with an
// unspecified/irrelevant cwd, not from the repo directory.
func LoadEnv() error {
	if err := loadEnvFile(".env"); err != nil {
		return err
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return nil //nolint:nilerr // best-effort secondary lookup, cwd .env already tried
	}
	return loadEnvFile(filepath.Join(home, ".config", "spotify-tui-go", ".env"))
}
