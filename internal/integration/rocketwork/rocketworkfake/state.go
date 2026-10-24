package rocketworkfake

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// load читает общее состояние из файла (если задан). Нет файла — текущее состояние в памяти.
func (f *Fake) load() error {
	if f.path == "" {
		return nil
	}
	raw, err := os.ReadFile(f.path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("rocketworkfake: read state: %w", err)
	}
	var st state
	if err := json.Unmarshal(raw, &st); err != nil {
		return fmt.Errorf("rocketworkfake: decode state: %w", err)
	}
	if st.Contractors == nil {
		st.Contractors = map[string]*Contractor{}
	}
	if st.Payments == nil {
		st.Payments = map[string]*Payment{}
	}
	f.st = st
	return nil
}

// save пишет состояние во временный файл и переименовывает его (атомарная замена).
func (f *Fake) save() error {
	if f.path == "" {
		return nil
	}
	raw, err := json.MarshalIndent(f.st, "", "  ")
	if err != nil {
		return fmt.Errorf("rocketworkfake: encode state: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(f.path), 0o755); err != nil {
		return fmt.Errorf("rocketworkfake: state dir: %w", err)
	}
	tmp := f.path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o600); err != nil {
		return fmt.Errorf("rocketworkfake: write state: %w", err)
	}
	if err := os.Rename(tmp, f.path); err != nil {
		return fmt.Errorf("rocketworkfake: replace state: %w", err)
	}
	return nil
}
