package deck

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"testing"
)

func persistenceConfig(dir string) Config {
	return Config{Upstream: "http://127.0.0.1:8000", ProxyURL: "http://127.0.0.1:7332", AdminURL: "http://127.0.0.1:7331", DataDir: dir}
}

func persistenceRule(name string) Rule {
	return Rule{Name: name, Enabled: true, Method: "GET", Path: "/api/orders", Type: "status", StatusCode: 503, Every: 1, Limit: 2}
}

func openPersistentDeck(t *testing.T, config Config) *Deck {
	t.Helper()
	d, err := New(config)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(d.Close)
	return d
}

func readWorkspaceForTest(t *testing.T, dir string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, "workspace.json"))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestPersistenceRestartCRUDAndConfiguration(t *testing.T) {
	config := persistenceConfig(filepath.Join(t.TempDir(), "workspace"))
	d := openPersistentDeck(t, config)
	if !d.State().Persistent {
		t.Fatal("disk-backed workspace not reported as persistent")
	}
	if len(readWorkspaceForTest(t, config.DataDir)) == 0 {
		t.Fatal("initial workspace not written")
	}
	first, err := d.AddRule(persistenceRule("first"))
	if err != nil {
		t.Fatal(err)
	}
	second, err := d.AddRule(persistenceRule("second"))
	if err != nil {
		t.Fatal(err)
	}
	updated := persistenceRule("edited first")
	updated.StatusCode = 429
	if _, err := d.UpdateRule(first.ID, updated); err != nil {
		t.Fatal(err)
	}
	if err := d.DeleteRule(second.ID); err != nil {
		t.Fatal(err)
	}
	upstream, enabled := "https://example.test/base", false
	if _, err := d.Configure(&upstream, &enabled); err != nil {
		t.Fatal(err)
	}
	d.Close()
	restarted := openPersistentDeck(t, config)
	state := restarted.State()
	if state.Upstream != upstream || state.Enabled || len(state.Rules) != 1 || state.Rules[0].Name != "edited first" || state.Rules[0].StatusCode != 429 {
		t.Fatalf("configuration/CRUD not restored: %+v", state)
	}
	if _, err := restarted.UpdateRule("missing", updated); !errors.Is(err, ErrRuleNotFound) {
		t.Fatalf("missing update returned %v", err)
	}
	if err := restarted.DeleteRule("missing"); !errors.Is(err, ErrRuleNotFound) {
		t.Fatalf("missing delete returned %v", err)
	}
}

func TestPersistenceImportOmitsRuntimeAndSecrets(t *testing.T) {
	config := persistenceConfig(t.TempDir())
	config.AdminUser, config.AdminPassword, config.ProxyToken = "private-admin", "secret-password-value", "secret-proxy-token-value"
	d := openPersistentDeck(t, config)
	rule := persistenceRule("imported")
	rule.ID, rule.Hits, rule.Matched = "caller-supplied-id", 10, 20
	scenario := Scenario{Version: 1, Name: "Saved scenario", Upstream: "https://example.test/backend", Enabled: true, Rules: []Rule{rule}}
	if err := d.Import(scenario); err != nil {
		t.Fatal(err)
	}
	before := readWorkspaceForTest(t, config.DataDir)
	_, selected, epoch := d.selectRule("GET", "/api/orders")
	if selected == nil {
		t.Fatal("imported rule not active")
	}
	d.record(Log{Path: "/api/orders", Status: 503, Fault: "status"}, epoch)
	if !bytes.Equal(before, readWorkspaceForTest(t, config.DataDir)) {
		t.Fatal("request processing wrote runtime state to disk")
	}
	for _, forbidden := range []string{`"id"`, `"hits"`, `"matched"`, `"logs"`, `"stats"`, config.AdminUser, config.AdminPassword, config.ProxyToken} {
		if bytes.Contains(before, []byte(forbidden)) {
			t.Fatalf("workspace contains runtime/private data %q", forbidden)
		}
	}
	d.Close()
	restarted := openPersistentDeck(t, config)
	state := restarted.State()
	if state.Stats != (Stats{}) || len(state.Logs) != 0 || len(state.Rules) != 1 || state.Rules[0].Matched != 0 || state.Rules[0].Hits != 0 || state.Rules[0].ID == "" || state.Rules[0].ID == rule.ID {
		t.Fatalf("runtime state not fresh: %+v", state)
	}
	if got := restarted.Export(); got.Name != scenario.Name || got.Upstream != scenario.Upstream || !got.Enabled {
		t.Fatalf("scenario not restored: %+v", got)
	}
	if !state.ProxyAuth {
		t.Fatal("proxy auth state missing")
	}
}

func TestPersistenceRejectsInvalidWorkspace(t *testing.T) {
	valid := `{"version":1,"name":"Saved","upstream":"http://127.0.0.1:8000","enabled":true,"rules":[]}`
	cases := map[string][]byte{
		"empty":       {},
		"malformed":   []byte(`{"version":`),
		"unknown":     []byte(strings.Replace(valid, `"rules":[]`, `"rules":[],"typo":true`, 1)),
		"trailing":    []byte(valid + `{}`),
		"null":        []byte(`null`),
		"version":     []byte(strings.Replace(valid, `"version":1`, `"version":2`, 1)),
		"credentials": []byte(strings.Replace(valid, "http://127.0.0.1:8000", "http://user:password@example.test", 1)),
		"duplicate":   []byte(strings.Replace(valid, `"enabled":true`, `"enabled":true,"enabled":false`, 1)),
		"utf8":        append([]byte(valid[:len(valid)-1]), 0xff, '}'),
		"too-large":   []byte(valid + strings.Repeat(" ", maxWorkspaceBytes)),
		"bad-rule":    []byte(strings.Replace(valid, `"rules":[]`, `"rules":[{"name":"bad","method":"GET","path":"/x","type":"unknown"}]`, 1)),
	}
	for name, data := range cases {
		t.Run(name, func(t *testing.T) {
			config := persistenceConfig(t.TempDir())
			path := filepath.Join(config.DataDir, "workspace.json")
			if err := os.WriteFile(path, data, 0600); err != nil {
				t.Fatal(err)
			}
			d, err := New(config)
			if d != nil {
				d.Close()
			}
			if !errors.Is(err, ErrPersistence) {
				t.Fatalf("invalid saved workspace accepted or misclassified: %v", err)
			}
			if !bytes.Equal(data, readWorkspaceForTest(t, config.DataDir)) {
				t.Fatal("invalid workspace was overwritten")
			}
		})
	}
}

func TestPersistenceFailedMutationsLeaveMemoryAndFileIntact(t *testing.T) {
	for _, operation := range []string{"add", "update", "delete", "configure", "import"} {
		t.Run(operation, func(t *testing.T) {
			config := persistenceConfig(t.TempDir())
			d := openPersistentDeck(t, config)
			rule, err := d.AddRule(persistenceRule("original"))
			if err != nil {
				t.Fatal(err)
			}
			_, _, epoch := d.selectRule("GET", "/api/orders")
			d.record(Log{Status: 503, Fault: "status"}, epoch)
			before, beforeSerial := d.State(), d.serial
			oldFile := readWorkspaceForTest(t, config.DataDir)
			// A path occupied by a regular file cannot hold a temporary workspace
			// on any platform. The existing durable workspace stays untouched.
			blocked := filepath.Join(config.DataDir, "not-a-directory")
			if err := os.WriteFile(blocked, []byte("block writes"), 0600); err != nil {
				t.Fatal(err)
			}
			d.config.DataDir = blocked
			switch operation {
			case "add":
				_, err = d.AddRule(persistenceRule("new"))
			case "update":
				_, err = d.UpdateRule(rule.ID, persistenceRule("changed"))
			case "delete":
				err = d.DeleteRule(rule.ID)
			case "configure":
				upstream, enabled := "http://example.test", false
				_, err = d.Configure(&upstream, &enabled)
			case "import":
				err = d.Import(Scenario{Version: 1, Name: "changed", Upstream: "http://example.test", Rules: []Rule{persistenceRule("replacement")}})
			}
			if !errors.Is(err, ErrPersistence) {
				t.Fatalf("write failure not classified: %v", err)
			}
			if !reflect.DeepEqual(before, d.State()) || beforeSerial != d.serial || d.Export().Name != "My scenario" {
				t.Fatalf("failed %s changed in-memory state", operation)
			}
			if !bytes.Equal(oldFile, readWorkspaceForTest(t, config.DataDir)) {
				t.Fatal("failed write changed the previous workspace")
			}
		})
	}
}

func TestPersistenceAtomicReplacementAndCleanup(t *testing.T) {
	dir := t.TempDir()
	if err := replaceWorkspace(dir, []byte("old")); err != nil {
		t.Fatal(err)
	}
	if err := replaceWorkspace(dir, []byte("new")); err != nil {
		t.Fatalf("replace existing file on %s: %v", runtime.GOOS, err)
	}
	if got := string(readWorkspaceForTest(t, dir)); got != "new" {
		t.Fatalf("replacement contains %q", got)
	}
	// A nonempty directory forces rename to fail after writing and syncing the
	// temporary file, including on Windows; its contents must survive unchanged.
	blockedDir := filepath.Join(t.TempDir(), "workspace.json")
	if err := os.Mkdir(blockedDir, 0700); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(blockedDir, "keep")
	if err := os.WriteFile(marker, []byte("preserve"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := replaceWorkspace(filepath.Dir(blockedDir), []byte("replacement")); err == nil {
		t.Fatal("rename over a directory succeeded")
	}
	if data, err := os.ReadFile(marker); err != nil || string(data) != "preserve" {
		t.Fatalf("rename failure destroyed the destination: %q %v", data, err)
	}
	if temps, err := filepath.Glob(filepath.Join(filepath.Dir(blockedDir), ".workspace-*.tmp")); err != nil || len(temps) != 0 {
		t.Fatalf("temporary files leaked: %v %v", temps, err)
	}
}

func TestPersistenceInitialWriteFailure(t *testing.T) {
	blocked := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(blocked, []byte("not a directory"), 0600); err != nil {
		t.Fatal(err)
	}
	if d, err := New(persistenceConfig(blocked)); !errors.Is(err, ErrPersistence) {
		if d != nil {
			d.Close()
		}
		t.Fatalf("initial storage failure not reported: %v", err)
	}
}

func TestPersistenceRejectsNonRegularWorkspace(t *testing.T) {
	config := persistenceConfig(t.TempDir())
	if err := os.Mkdir(filepath.Join(config.DataDir, "workspace.json"), 0700); err != nil {
		t.Fatal(err)
	}
	if d, err := New(config); !errors.Is(err, ErrPersistence) {
		if d != nil {
			d.Close()
		}
		t.Fatalf("workspace directory not rejected: %v", err)
	}
}

func TestPersistenceEditsKeepUnrelatedRuntimeCounters(t *testing.T) {
	d := openPersistentDeck(t, persistenceConfig(t.TempDir()))
	first, err := d.AddRule(persistenceRule("already used"))
	if err != nil {
		t.Fatal(err)
	}
	d.selectRule("GET", "/api/orders")
	second, err := d.AddRule(persistenceRule("editable"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d.UpdateRule(second.ID, persistenceRule("edited")); err != nil {
		t.Fatal(err)
	}
	if err := d.DeleteRule(second.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := d.Configure(nil, nil); err != nil {
		t.Fatal(err)
	}
	state := d.State()
	if len(state.Rules) != 1 || state.Rules[0].ID != first.ID || state.Rules[0].Matched != 1 || state.Rules[0].Hits != 1 {
		t.Fatalf("unrelated edits reset runtime counters: %+v", state.Rules)
	}
	if data := readWorkspaceForTest(t, d.config.DataDir); bytes.Contains(data, []byte(`"matched"`)) || bytes.Contains(data, []byte(`"hits"`)) {
		t.Fatal("later configuration save included live counters")
	}
}

func TestPersistencePermissionsAndReadOnlyFailure(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows permissions are managed by ACLs, not Unix mode bits")
	}
	config := persistenceConfig(filepath.Join(t.TempDir(), "private"))
	d := openPersistentDeck(t, config)
	for path, want := range map[string]os.FileMode{config.DataDir: 0700, filepath.Join(config.DataDir, "workspace.json"): 0600} {
		info, err := os.Stat(path)
		if err != nil || info.Mode().Perm() != want {
			t.Fatalf("permissions for %s: info=%v err=%v want=%o", path, info, err, want)
		}
	}
	oldFile, before := readWorkspaceForTest(t, config.DataDir), d.State()
	if err := os.Chmod(config.DataDir, 0500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(config.DataDir, 0700) })
	probe, probeErr := os.CreateTemp(config.DataDir, "permission-probe")
	if probeErr == nil {
		probe.Close()
		os.Remove(probe.Name())
		t.Skip("test process can bypass directory write permissions")
	}
	if _, err := d.AddRule(persistenceRule("cannot save")); !errors.Is(err, ErrPersistence) {
		t.Fatalf("readonly directory error: %v", err)
	}
	if !reflect.DeepEqual(before, d.State()) || !bytes.Equal(oldFile, readWorkspaceForTest(t, config.DataDir)) {
		t.Fatal("readonly write failure modified state")
	}
}

func TestPersistenceConcurrentSaves(t *testing.T) {
	config := persistenceConfig(t.TempDir())
	d := openPersistentDeck(t, config)
	var workers sync.WaitGroup
	for worker := range 4 {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for i := range 5 {
				name := fmt.Sprintf("worker-%d-rule-%d", worker, i)
				if _, err := d.AddRule(persistenceRule(name)); err != nil {
					t.Errorf("concurrent save: %v", err)
				}
				_, _, epoch := d.selectRule("GET", "/api/orders")
				d.record(Log{Status: 503, Fault: "status"}, epoch)
				_ = d.State()
			}
		}()
	}
	workers.Wait()
	var saved Scenario
	if err := json.Unmarshal(readWorkspaceForTest(t, config.DataDir), &saved); err != nil {
		t.Fatalf("concurrent save left malformed JSON: %v", err)
	}
	if len(saved.Rules) != 20 || len(d.State().Rules) != 20 {
		t.Fatalf("concurrent updates lost: disk=%d memory=%d", len(saved.Rules), len(d.State().Rules))
	}
	restarted := openPersistentDeck(t, config)
	seen := make(map[string]bool)
	for _, rule := range restarted.State().Rules {
		if seen[rule.Name] || rule.Hits != 0 || rule.Matched != 0 {
			t.Fatalf("invalid restored rule: %+v", rule)
		}
		seen[rule.Name] = true
	}
}
