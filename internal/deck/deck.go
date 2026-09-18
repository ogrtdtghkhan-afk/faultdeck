package deck

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"time"
)

var ErrRuleNotFound = errors.New("rule not found")

type Deck struct {
	mu           sync.Mutex
	config       Config
	target       *url.URL
	enabled      bool
	rules        []Rule
	logs         []Log
	stats        Stats
	totalMS      float64
	serial       uint64
	epoch        uint64
	startedAt    string
	scenarioName string
	transport    *http.Transport
}

func New(config Config) (*Deck, error) {
	if config.Version == "" {
		config.Version = Version
	}
	t := http.DefaultTransport.(*http.Transport).Clone()
	t.Proxy = nil
	t.ResponseHeaderTimeout = 30 * time.Second
	t.MaxIdleConns = 100
	t.MaxIdleConnsPerHost = 20
	d := &Deck{config: config, enabled: true, rules: []Rule{}, logs: []Log{}, startedAt: time.Now().UTC().Format(time.RFC3339), scenarioName: "My scenario", transport: t}
	saved, found, err := loadWorkspace(config.DataDir)
	if err != nil {
		d.Close()
		return nil, err
	}
	if found {
		u, rules, err := d.validateScenario(saved)
		if err != nil {
			d.Close()
			return nil, fmt.Errorf("%w: invalid saved workspace: %v", ErrPersistence, err)
		}
		for i := range rules {
			rules[i].ID = d.nextIDLocked()
		}
		d.target, d.enabled, d.rules, d.scenarioName = u, saved.Enabled, rules, saved.Name
		return d, nil
	}
	u, err := d.validateTarget(config.Upstream)
	if err != nil {
		d.Close()
		return nil, err
	}
	d.target = u
	if err := d.saveWorkspace(d.scenarioLocked()); err != nil {
		d.Close()
		return nil, err
	}
	return d, nil
}

func (d *Deck) Close() { d.transport.CloseIdleConnections() }

func (d *Deck) State() State {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.stateLocked()
}

func (d *Deck) stateLocked() State {
	rules := append([]Rule{}, d.rules...)
	logs := make([]Log, len(d.logs))
	for i := range d.logs {
		logs[i] = d.logs[len(d.logs)-1-i]
	}
	return State{Version: d.config.Version, ProxyURL: d.config.ProxyURL, DemoURL: d.config.DemoURL, Upstream: d.target.String(), Enabled: d.enabled, Persistent: d.config.DataDir != "", ProxyAuth: d.config.ProxyToken != "", StartedAt: d.startedAt, Rules: rules, Stats: d.stats, Logs: logs}
}

func (d *Deck) nextIDLocked() string { d.serial++; return fmt.Sprintf("r-%d", d.serial) }

func (d *Deck) AddRule(r Rule) (Rule, error) {
	if err := validateRule(&r); err != nil {
		return Rule{}, err
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if len(d.rules) >= 100 {
		return Rule{}, fmt.Errorf("maximum 100 rules")
	}
	r.ID = fmt.Sprintf("r-%d", d.serial+1)
	candidate := d.scenarioLocked()
	candidate.Rules = append(candidate.Rules, r)
	if err := d.saveWorkspace(candidate); err != nil {
		return Rule{}, err
	}
	d.serial++
	d.rules = candidate.Rules
	return r, nil
}

// Configure persists a complete candidate before making either setting visible.
func (d *Deck) Configure(upstream *string, enabled *bool) (State, error) {
	var target *url.URL
	if upstream != nil {
		var err error
		target, err = d.validateTarget(*upstream)
		if err != nil {
			return State{}, err
		}
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	candidate := d.scenarioLocked()
	if target != nil {
		candidate.Upstream = target.String()
	}
	if enabled != nil {
		candidate.Enabled = *enabled
	}
	if err := d.saveWorkspace(candidate); err != nil {
		return State{}, err
	}
	if target != nil {
		d.target = target
	}
	d.enabled = candidate.Enabled
	return d.stateLocked(), nil
}

func (d *Deck) UpdateRule(id string, r Rule) (Rule, error) {
	if err := validateRule(&r); err != nil {
		return Rule{}, err
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	for i := range d.rules {
		if d.rules[i].ID != id {
			continue
		}
		r.ID = id
		candidate := d.scenarioLocked()
		candidate.Rules[i] = r
		if err := d.saveWorkspace(candidate); err != nil {
			return Rule{}, err
		}
		d.rules = candidate.Rules
		return r, nil
	}
	return Rule{}, ErrRuleNotFound
}

func (d *Deck) DeleteRule(id string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	for i := range d.rules {
		if d.rules[i].ID != id {
			continue
		}
		candidate := d.scenarioLocked()
		candidate.Rules = append(candidate.Rules[:i], candidate.Rules[i+1:]...)
		if err := d.saveWorkspace(candidate); err != nil {
			return err
		}
		d.rules = candidate.Rules
		return nil
	}
	return ErrRuleNotFound
}

func (d *Deck) validateScenario(s Scenario) (*url.URL, []Rule, error) {
	if s.Version != 1 {
		return nil, nil, fmt.Errorf("unsupported scenario version (expected 1)")
	}
	if len(s.Rules) > 100 {
		return nil, nil, fmt.Errorf("maximum 100 rules")
	}
	if len(s.Name) > 120 {
		return nil, nil, fmt.Errorf("scenario name is too long")
	}
	u, err := d.validateTarget(s.Upstream)
	if err != nil {
		return nil, nil, err
	}
	rules := append([]Rule{}, s.Rules...)
	for i := range rules {
		if err := validateRule(&rules[i]); err != nil {
			return nil, nil, fmt.Errorf("rule %d: %w", i+1, err)
		}
	}
	return u, rules, nil
}

func (d *Deck) Import(s Scenario) error {
	u, rules, err := d.validateScenario(s)
	if err != nil {
		return err
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	for i := range rules {
		rules[i].ID = fmt.Sprintf("r-%d", d.serial+uint64(i)+1)
	}
	candidate := Scenario{Version: 1, Name: s.Name, Upstream: u.String(), Enabled: s.Enabled, Rules: rules}
	if err := d.saveWorkspace(candidate); err != nil {
		return err
	}
	d.serial += uint64(len(rules))
	d.target, d.enabled, d.rules, d.scenarioName = u, s.Enabled, rules, s.Name
	d.resetLocked()
	return nil
}

func (d *Deck) Export() Scenario {
	d.mu.Lock()
	defer d.mu.Unlock()
	s := d.scenarioLocked()
	for i := range s.Rules {
		s.Rules[i].Matched, s.Rules[i].Hits = 0, 0
	}
	return s
}

// scenarioLocked copies live configuration, retaining runtime fields in the copy
// so unrelated rules keep their counters when one rule is edited. The persistence
// writer removes those runtime fields only from its own separate copy.
func (d *Deck) scenarioLocked() Scenario {
	return Scenario{Version: 1, Name: d.scenarioName, Upstream: d.target.String(), Enabled: d.enabled, Rules: append([]Rule{}, d.rules...)}
}

func (d *Deck) resetLocked() {
	d.epoch++
	d.logs, d.stats, d.totalMS = []Log{}, Stats{}, 0
	for i := range d.rules {
		d.rules[i].Matched, d.rules[i].Hits = 0, 0
	}
}

func (d *Deck) Reset() { d.mu.Lock(); defer d.mu.Unlock(); d.resetLocked() }

func (d *Deck) selectRule(method, path string) (*url.URL, *Rule, uint64) {
	d.mu.Lock()
	defer d.mu.Unlock()
	target := *d.target
	if d.enabled {
		for i := range d.rules {
			r := &d.rules[i]
			if !r.Enabled || (r.Method != "*" && r.Method != method) || !pathMatches(r.Path, path) {
				continue
			}
			r.Matched++
			if (r.Limit > 0 && r.Hits >= uint64(r.Limit)) || r.Matched%uint64(r.Every) != 0 {
				continue
			}
			r.Hits++
			copy := *r
			return &target, &copy, d.epoch
		}
	}
	return &target, nil, d.epoch
}

func (d *Deck) record(entry Log, epoch uint64) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if epoch != d.epoch {
		return
	} // A reset excludes requests started before it.
	d.serial++
	entry.ID = d.serial
	d.stats.Requests++
	if entry.Fault != "" {
		d.stats.Injected++
	}
	if entry.Status == 0 || entry.Status >= 400 || entry.Error != "" {
		d.stats.Errors++
	}
	d.totalMS += entry.DurationMS
	d.stats.AvgDurationMS = d.totalMS / float64(d.stats.Requests)
	if len(d.logs) == 200 {
		copy(d.logs, d.logs[1:])
		d.logs = d.logs[:199]
	}
	d.logs = append(d.logs, entry)
}
