package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

const (
	AccessRead  = "read"
	AccessWrite = "write"
)

type State struct {
	Access string `json:"access"`
}

func Home(getenv func(string) string) (string, error) {
	if h := getenv("DTRACK_HOME"); h != "" {
		return h, nil
	}
	d, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("cannot find config directory (set DTRACK_HOME): %w", err)
	}
	return filepath.Join(d, "dtrack"), nil
}

func configPath(getenv func(string) string) (string, error) {
	home, err := Home(getenv)
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "config.json"), nil
}

func Load(getenv func(string) string) (*State, error) {
	path, err := configPath(getenv)
	if err != nil {
		return nil, err
	}
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return &State{Access: AccessRead}, nil
	}
	if err != nil {
		return nil, err
	}
	var st State
	if err := json.Unmarshal(b, &st); err != nil {
		return nil, fmt.Errorf("corrupt %s: %w", path, err)
	}
	// Legacy config.json "access":"write" is ignored; writes are per-command with --write + TTY.
	st.Access = AccessRead
	return &st, nil
}

func Save(getenv func(string) string, st *State) error {
	path, err := configPath(getenv)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	if st.Access == "" {
		st.Access = AccessRead
	}
	b, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o600)
}

// CanWrite is always false; local config does not grant write access.
func (st *State) CanWrite() bool { return false }
