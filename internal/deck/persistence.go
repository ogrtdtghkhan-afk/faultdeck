package deck

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"unicode/utf8"
)

// ErrPersistence distinguishes storage failures from invalid user configuration.
var ErrPersistence = errors.New("workspace persistence failed")

const maxWorkspaceBytes = 1 << 20

func loadWorkspace(dataDir string) (Scenario, bool, error) {
	if dataDir == "" {
		return Scenario{}, false, nil
	}
	path := filepath.Join(dataDir, "workspace.json")
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return Scenario{}, false, nil
	}
	if err != nil {
		return Scenario{}, false, fmt.Errorf("%w: inspect workspace: %v", ErrPersistence, err)
	}
	if !info.Mode().IsRegular() {
		return Scenario{}, false, fmt.Errorf("%w: workspace must be a regular file, not a directory or symbolic link", ErrPersistence)
	}
	f, err := os.Open(path)
	if err != nil {
		return Scenario{}, false, fmt.Errorf("%w: open workspace: %v", ErrPersistence, err)
	}
	defer f.Close()
	info, err = f.Stat()
	if err != nil || !info.Mode().IsRegular() {
		return Scenario{}, false, fmt.Errorf("%w: workspace must be a readable regular file", ErrPersistence)
	}
	data, err := io.ReadAll(io.LimitReader(f, maxWorkspaceBytes+1))
	if err != nil {
		return Scenario{}, false, fmt.Errorf("%w: read workspace: %v", ErrPersistence, err)
	}
	if len(data) > maxWorkspaceBytes {
		return Scenario{}, false, fmt.Errorf("%w: workspace exceeds 1 MiB", ErrPersistence)
	}
	if !utf8.Valid(data) {
		return Scenario{}, false, fmt.Errorf("%w: workspace is not valid UTF-8", ErrPersistence)
	}
	var scenario Scenario
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&scenario); err != nil {
		return Scenario{}, false, fmt.Errorf("%w: invalid workspace JSON: %v", ErrPersistence, err)
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return Scenario{}, false, fmt.Errorf("%w: workspace must contain exactly one JSON value", ErrPersistence)
	}
	if err := uniqueJSONKeys(json.NewDecoder(bytes.NewReader(data))); err != nil {
		return Scenario{}, false, fmt.Errorf("%w: invalid workspace JSON: %v", ErrPersistence, err)
	}
	return scenario, true, nil
}

// encoding/json accepts duplicate object keys; workspace configuration rejects
// them so a file cannot silently select a different value depending on its reader.
func uniqueJSONKeys(decoder *json.Decoder) error {
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	switch token {
	case json.Delim('{'):
		seen := make(map[string]bool)
		for decoder.More() {
			key, err := decoder.Token()
			if err != nil {
				return err
			}
			name, ok := key.(string)
			if !ok || seen[name] {
				return fmt.Errorf("duplicate or invalid object key")
			}
			seen[name] = true
			if err := uniqueJSONKeys(decoder); err != nil {
				return err
			}
		}
		_, err = decoder.Token()
		return err
	case json.Delim('['):
		for decoder.More() {
			if err := uniqueJSONKeys(decoder); err != nil {
				return err
			}
		}
		_, err = decoder.Token()
		return err
	default:
		return nil
	}
}

// saveWorkspace is called while holding d.mu (or before New publishes the Deck).
// It persists only scenario configuration, never logs, counters, or Config fields.
// Nothing in the caller's candidate is mutated until replacement succeeds.
func (d *Deck) saveWorkspace(scenario Scenario) error {
	if d.config.DataDir == "" {
		return nil
	}
	scenario.Rules = append([]Rule{}, scenario.Rules...)
	for i := range scenario.Rules {
		scenario.Rules[i].ID = ""
		scenario.Rules[i].Matched, scenario.Rules[i].Hits = 0, 0
	}
	data, err := json.MarshalIndent(scenario, "", "  ")
	if err != nil {
		return fmt.Errorf("%w: encode workspace: %v", ErrPersistence, err)
	}
	data = append(data, '\n')
	if len(data) > maxWorkspaceBytes {
		return fmt.Errorf("%w: workspace exceeds 1 MiB", ErrPersistence)
	}
	if err := replaceWorkspace(d.config.DataDir, data); err != nil {
		return fmt.Errorf("%w: save workspace: %v", ErrPersistence, err)
	}
	return nil
}

// The old workspace is never deleted before replacement. A failed write, sync,
// close, or rename leaves it intact and removes the temporary file. Existing
// directory permissions are respected; newly created directories are private.
func replaceWorkspace(dataDir string, data []byte) error {
	if err := os.MkdirAll(dataDir, 0700); err != nil {
		return err
	}
	f, err := os.CreateTemp(dataDir, ".workspace-*.tmp")
	if err != nil {
		return err
	}
	tempPath := f.Name()
	defer os.Remove(tempPath)
	if err := f.Chmod(0600); err != nil {
		f.Close()
		return err
	}
	if _, err := f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(tempPath, filepath.Join(dataDir, "workspace.json"))
}
