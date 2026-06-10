package app

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	tea "github.com/charmbracelet/bubbletea"
	awsx "github.com/sachamama/sacha/internal/aws"
	"github.com/sachamama/sacha/internal/cache"
	appconfig "github.com/sachamama/sacha/internal/config"
)

// mockService implements awsx.Service for testing.
type mockService struct{}

func (mockService) Name() string  { return "mock" }
func (mockService) Title() string { return "Mock Service" }
func (mockService) Init(_ context.Context, _ aws.Config, _ awsx.ServiceOptions) (tea.Model, error) {
	return mockModel{}, nil
}

// mockModel implements tea.Model for testing.
type mockModel struct{}

func (mockModel) Init() tea.Cmd                         { return nil }
func (m mockModel) Update(tea.Msg) (tea.Model, tea.Cmd) { return m, nil }
func (mockModel) View() string                          { return "mock" }

// newTestModel creates a Model with the given region for testing.
func newTestModel(t *testing.T, region string) Model {
	t.Helper()

	loader := awsx.Loader{}
	// We use a fake loader that just returns a config with the given region.
	// This is only needed if changeRegion is called.

	services := map[string]awsx.Service{
		"mock": mockService{},
	}

	runtime := appconfig.RuntimeConfig{
		Region:  region,
		Service: "mock",
	}

	cfg := aws.Config{
		Region: region,
	}

	m := Model{
		loader:   loader,
		services: services,
		runtime:  runtime,
		cfg:      cfg,
		cache:    cache.New(),
	}
	m.regionSelector = newOptionSelectorWithViews(awsRegions, commonRegions)
	m.serviceSelector = newOptionSelector(serviceNames(services))

	// Activate the mock service
	if err := m.activateService("mock"); err != nil {
		t.Fatalf("activateService: %v", err)
	}

	return m
}

func TestModelRegionPreservedThroughUpdateCycle(t *testing.T) {
	m := newTestModel(t, "eu-west-1")

	// Verify initial state
	if m.runtime.Region != "eu-west-1" {
		t.Fatalf("initial region = %q, want %q", m.runtime.Region, "eu-west-1")
	}

	// Simulate Bubble Tea lifecycle: Update is a value receiver, so each call
	// gets a copy, modifies it, and returns the new state.

	// Step 1: WindowSizeMsg
	result, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = result.(Model)
	if m.runtime.Region != "eu-west-1" {
		t.Fatalf("after WindowSizeMsg: region = %q, want %q", m.runtime.Region, "eu-west-1")
	}

	// Step 2: Press 'r' to open region selector
	result, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	m = result.(Model)
	if m.runtime.Region != "eu-west-1" {
		t.Fatalf("after pressing r: region = %q, want %q", m.runtime.Region, "eu-west-1")
	}
	if !m.regionSelector.active {
		t.Fatal("region selector should be active after pressing r")
	}
	if m.regionSelector.current() != "eu-west-1" {
		t.Errorf("region selector cursor: current() = %q, want %q", m.regionSelector.current(), "eu-west-1")
	}

	// Step 3: Press Esc to close region selector
	result, _ = m.Update(tea.KeyMsg{Type: tea.KeyEscape})
	m = result.(Model)
	if m.runtime.Region != "eu-west-1" {
		t.Fatalf("after Esc: region = %q, want %q", m.runtime.Region, "eu-west-1")
	}
	if m.regionSelector.active {
		t.Fatal("region selector should be inactive after Esc")
	}

	// Step 4: Simulate some service messages (e.g., log groups loaded)
	// These are unknown message types that go to m.service.Update
	type dummyMsg struct{}
	result, _ = m.Update(dummyMsg{})
	m = result.(Model)
	if m.runtime.Region != "eu-west-1" {
		t.Fatalf("after service msg: region = %q, want %q", m.runtime.Region, "eu-west-1")
	}

	// Step 5: Press 'r' again to reopen region selector
	result, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	m = result.(Model)
	if m.runtime.Region != "eu-west-1" {
		t.Fatalf("after pressing r again: region = %q, want %q", m.runtime.Region, "eu-west-1")
	}
	if !m.regionSelector.active {
		t.Fatal("region selector should be active after pressing r again")
	}
	if m.regionSelector.current() != "eu-west-1" {
		t.Errorf("region selector cursor on reopen: current() = %q, want %q", m.regionSelector.current(), "eu-west-1")
	}
}

func TestModelChangeRegionUpdatesRuntimeRegion(t *testing.T) {
	// Create a model that supports changing regions via a fake loader
	services := map[string]awsx.Service{
		"mock": mockService{},
	}

	runtime := appconfig.RuntimeConfig{
		Region:  "us-east-1",
		Service: "mock",
	}

	cfg := aws.Config{
		Region: "us-east-1",
	}

	m := Model{
		loader:   newFakeLoader(),
		services: services,
		runtime:  runtime,
		cfg:      cfg,
		cache:    cache.New(),
	}
	m.regionSelector = newOptionSelectorWithViews(awsRegions, commonRegions)
	m.serviceSelector = newOptionSelector(serviceNames(services))
	if err := m.activateService("mock"); err != nil {
		t.Fatalf("activateService: %v", err)
	}

	// Give it a size
	result, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = result.(Model)

	// Open region selector
	result, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	m = result.(Model)

	// Navigate down to eu-west-1 (index 4 in common regions)
	for i := 0; i < 4; i++ {
		result, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
		m = result.(Model)
	}
	if m.regionSelector.current() != "eu-west-1" {
		t.Fatalf("navigated to: %q, want %q", m.regionSelector.current(), "eu-west-1")
	}

	// Select eu-west-1
	result, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = result.(Model)

	// Verify region was changed
	if m.runtime.Region != "eu-west-1" {
		t.Fatalf("after selection: runtime.Region = %q, want %q", m.runtime.Region, "eu-west-1")
	}
	if m.cfg.Region != "eu-west-1" {
		t.Fatalf("after selection: cfg.Region = %q, want %q", m.cfg.Region, "eu-west-1")
	}

	// Reopen region selector — should show eu-west-1
	result, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	m = result.(Model)
	if m.regionSelector.current() != "eu-west-1" {
		t.Errorf("reopen after change: current() = %q, want %q", m.regionSelector.current(), "eu-west-1")
	}
}

func TestModelRegionSelectorFallsBackToCfgRegion(t *testing.T) {
	// Simulate a scenario where runtime.Region is empty but cfg.Region is set.
	// The region selector should use cfg.Region as fallback.
	services := map[string]awsx.Service{
		"mock": mockService{},
	}

	runtime := appconfig.RuntimeConfig{
		Region:  "", // empty — simulates edge case
		Service: "mock",
	}

	cfg := aws.Config{
		Region: "eu-west-1", // SDK config has the region
	}

	m := Model{
		loader:   newFakeLoader(),
		services: services,
		runtime:  runtime,
		cfg:      cfg,
		cache:    cache.New(),
	}
	m.regionSelector = newOptionSelectorWithViews(awsRegions, commonRegions)
	m.serviceSelector = newOptionSelector(serviceNames(services))
	if err := m.activateService("mock"); err != nil {
		t.Fatalf("activateService: %v", err)
	}

	// Give it a size
	result, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = result.(Model)

	// Open region selector — should fall back to cfg.Region
	result, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	m = result.(Model)

	if m.regionSelector.current() != "eu-west-1" {
		t.Errorf("region selector should fall back to cfg.Region: current() = %q, want %q", m.regionSelector.current(), "eu-west-1")
	}
}

func TestModelChangeRegionRollsBackOnFailure(t *testing.T) {
	// Test that changeRegion rolls back m.cfg and m.runtime.Region if activateService fails
	services := map[string]awsx.Service{
		"mock": mockService{},
	}

	runtime := appconfig.RuntimeConfig{
		Region:  "eu-west-1",
		Service: "mock",
	}

	cfg := aws.Config{
		Region: "eu-west-1",
	}

	m := Model{
		loader:   newFakeLoader(),
		services: services,
		runtime:  runtime,
		cfg:      cfg,
		cache:    cache.New(),
	}
	m.regionSelector = newOptionSelectorWithViews(awsRegions, commonRegions)
	m.serviceSelector = newOptionSelector(serviceNames(services))
	if err := m.activateService("mock"); err != nil {
		t.Fatalf("activateService: %v", err)
	}

	// Try to change to a region — changeRegion("") should fail
	_, err := m.changeRegion("")
	if err == nil {
		t.Fatal("changeRegion('') should fail")
	}

	// Region should be preserved
	if m.runtime.Region != "eu-west-1" {
		t.Errorf("runtime.Region should be preserved after failed change: got %q, want %q", m.runtime.Region, "eu-west-1")
	}
	if m.cfg.Region != "eu-west-1" {
		t.Errorf("cfg.Region should be preserved after failed change: got %q, want %q", m.cfg.Region, "eu-west-1")
	}
}

func TestNewModelSyncsRuntimeRegionFromConfig(t *testing.T) {
	// When runtime.Region is empty but cfg.Region is set, NewModel should sync them
	services := map[string]awsx.Service{
		"mock": mockService{},
	}
	runtime := appconfig.RuntimeConfig{
		Region:  "", // empty
		Service: "mock",
	}
	cfg := aws.Config{
		Region: "eu-west-1",
	}

	m, err := NewModel(newFakeLoader(), services, runtime, cfg, nil, nil)
	if err != nil {
		t.Fatalf("NewModel: %v", err)
	}

	if m.runtime.Region != "eu-west-1" {
		t.Errorf("NewModel should sync runtime.Region from cfg.Region: got %q, want %q", m.runtime.Region, "eu-west-1")
	}
}

func TestModelChangeRegionPersistsImmediately(t *testing.T) {
	// Region selection must be written to disk as soon as it changes, so it
	// survives abnormal termination rather than only a clean exit.
	services := map[string]awsx.Service{
		"mock": mockService{},
	}

	runtime := appconfig.RuntimeConfig{
		Region:  "us-east-1",
		Service: "mock",
	}
	cfg := aws.Config{Region: "us-east-1"}

	var saved []appconfig.RuntimeConfig
	persist := func(rt appconfig.RuntimeConfig) error {
		saved = append(saved, rt)
		return nil
	}

	m := Model{
		loader:   newFakeLoader(),
		services: services,
		runtime:  runtime,
		cfg:      cfg,
		cache:    cache.New(),
		persist:  persist,
	}
	m.regionSelector = newOptionSelectorWithViews(awsRegions, commonRegions)
	m.serviceSelector = newOptionSelector(serviceNames(services))
	if err := m.activateService("mock"); err != nil {
		t.Fatalf("activateService: %v", err)
	}

	if _, err := m.changeRegion("eu-west-1"); err != nil {
		t.Fatalf("changeRegion: %v", err)
	}

	if len(saved) != 1 {
		t.Fatalf("persist called %d times, want 1", len(saved))
	}
	if saved[0].Region != "eu-west-1" {
		t.Errorf("persisted region = %q, want %q", saved[0].Region, "eu-west-1")
	}
}

func TestModelChangeRegionFailureDoesNotPersist(t *testing.T) {
	// A failed region change must not write anything to disk.
	services := map[string]awsx.Service{
		"mock": mockService{},
	}
	runtime := appconfig.RuntimeConfig{Region: "us-east-1", Service: "mock"}

	var calls int
	m := Model{
		loader:   newFakeLoader(),
		services: services,
		runtime:  runtime,
		cfg:      aws.Config{Region: "us-east-1"},
		cache:    cache.New(),
		persist:  func(appconfig.RuntimeConfig) error { calls++; return nil },
	}
	m.regionSelector = newOptionSelectorWithViews(awsRegions, commonRegions)
	m.serviceSelector = newOptionSelector(serviceNames(services))
	if err := m.activateService("mock"); err != nil {
		t.Fatalf("activateService: %v", err)
	}

	if _, err := m.changeRegion(""); err == nil {
		t.Fatal("changeRegion('') should fail")
	}
	if calls != 0 {
		t.Errorf("persist called %d times on failure, want 0", calls)
	}
}

// newFakeLoader returns an awsx.Loader that produces aws.Config with the requested region.
func newFakeLoader() awsx.Loader {
	return awsx.NewTestLoader(func(ctx context.Context, optFns ...func(*config.LoadOptions) error) (aws.Config, error) {
		var opts config.LoadOptions
		for _, fn := range optFns {
			if err := fn(&opts); err != nil {
				return aws.Config{}, err
			}
		}
		return aws.Config{
			Region: opts.Region,
		}, nil
	}, "")
}
