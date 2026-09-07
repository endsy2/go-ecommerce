package config

import (
	"bufio"
	"errors"
	"io/fs"
	"os"
	"strings"
)

// LoadDotEnv reads a .env file and sets any variable not already present in the
// environment. Call it before Load.
//
// This is a deliberately small stdlib replacement for github.com/joho/godotenv:
// it keeps the "configure via .env in development" workflow without adding a
// dependency. It handles KEY=value, # comments, blank lines, `export ` prefixes
// and quoted values — not the full shell grammar, which .env files should not
// be using anyway.
//
// A missing file is not an error: in production the environment is populated by
// the orchestrator and no .env exists.
//
// Real environment variables always win, so `HTTP_PORT=9000 go run ./cmd/api`
// overrides the file without editing it.
func LoadDotEnv(path string) error {
	f, err := os.Open(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimPrefix(line, "export ")

		key, value, found := strings.Cut(line, "=")
		if !found {
			continue
		}
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}

		value = strings.TrimSpace(value)
		value = unquote(value)

		// Do not clobber a variable the environment already set.
		if _, exists := os.LookupEnv(key); exists {
			continue
		}
		if err := os.Setenv(key, value); err != nil {
			return err
		}
	}
	return scanner.Err()
}

// unquote strips one matching pair of surrounding quotes. An unquoted value also
// has any trailing ` # comment` removed; a quoted one keeps its content verbatim,
// so a password containing '#' survives if it is quoted.
func unquote(v string) string {
	if len(v) >= 2 {
		if (v[0] == '"' && v[len(v)-1] == '"') || (v[0] == '\'' && v[len(v)-1] == '\'') {
			return v[1 : len(v)-1]
		}
	}
	if i := strings.Index(v, " #"); i >= 0 {
		v = strings.TrimSpace(v[:i])
	}
	return v
}
