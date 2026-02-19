package app

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestOptionSelectorOpenHighlightsCurrentRegion(t *testing.T) {
	tests := []struct {
		name           string
		current        string
		wantCursor     int
		wantViewMode   viewMode
		wantInFiltered bool
	}{
		{
			name:           "us-east-1 (first common region)",
			current:        "us-east-1",
			wantCursor:     0,
			wantViewMode:   viewCommon,
			wantInFiltered: true,
		},
		{
			name:           "eu-west-1 (middle of common regions)",
			current:        "eu-west-1",
			wantCursor:     4,
			wantViewMode:   viewCommon,
			wantInFiltered: true,
		},
		{
			name:           "ap-northeast-1 (last common region)",
			current:        "ap-northeast-1",
			wantCursor:     7,
			wantViewMode:   viewCommon,
			wantInFiltered: true,
		},
		{
			name:         "sa-east-1 (not in common, falls back to all)",
			current:      "sa-east-1",
			wantViewMode: viewAll,
		},
		{
			name:         "empty string",
			current:      "",
			wantCursor:   0,
			wantViewMode: viewCommon,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := newOptionSelectorWithViews("Select Region", awsRegions, commonRegions)
			s.open(awsRegions, tt.current)

			if s.viewMode != tt.wantViewMode {
				t.Errorf("viewMode = %d, want %d", s.viewMode, tt.wantViewMode)
			}

			if tt.wantInFiltered {
				if s.cursor != tt.wantCursor {
					t.Errorf("cursor = %d, want %d", s.cursor, tt.wantCursor)
				}
				if s.current() != tt.current {
					t.Errorf("current() = %q, want %q", s.current(), tt.current)
				}
			}

			// For non-common regions that fall back to All view, verify cursor points to the correct region
			if tt.wantViewMode == viewAll && tt.current != "" {
				if s.current() != tt.current {
					t.Errorf("current() = %q, want %q", s.current(), tt.current)
				}
			}
		})
	}
}

func TestOptionSelectorOpenCloseReopenPreservesRegion(t *testing.T) {
	// Simulate: open region selector, close with Esc, reopen — cursor should be on same region
	s := newOptionSelectorWithViews("Select Region", awsRegions, commonRegions)

	// First open: eu-west-1
	s.open(awsRegions, "eu-west-1")
	if s.current() != "eu-west-1" {
		t.Fatalf("first open: current() = %q, want %q", s.current(), "eu-west-1")
	}

	// Close with Esc
	choice, _ := s.update(tea.KeyMsg{Type: tea.KeyEscape})
	if choice != "" {
		t.Fatalf("Esc should return empty choice, got %q", choice)
	}
	if s.active {
		t.Fatal("selector should be inactive after Esc")
	}

	// Reopen with same region
	s.open(awsRegions, "eu-west-1")
	if !s.active {
		t.Fatal("selector should be active after open")
	}
	if s.current() != "eu-west-1" {
		t.Errorf("reopen: current() = %q, want %q", s.current(), "eu-west-1")
	}
	if s.cursor != 4 {
		t.Errorf("reopen: cursor = %d, want 4", s.cursor)
	}
}

func TestOptionSelectorSelectThenReopenShowsNewRegion(t *testing.T) {
	s := newOptionSelectorWithViews("Select Region", awsRegions, commonRegions)

	// Open and select eu-central-1 (index 5 in common)
	s.open(awsRegions, "us-east-1")
	if s.current() != "us-east-1" {
		t.Fatalf("initial: current() = %q, want %q", s.current(), "us-east-1")
	}

	// Navigate down to eu-central-1 (index 5)
	for i := 0; i < 5; i++ {
		s.update(tea.KeyMsg{Type: tea.KeyDown})
	}
	if s.current() != "eu-central-1" {
		t.Fatalf("after nav: current() = %q, want %q", s.current(), "eu-central-1")
	}

	// Select it
	choice, _ := s.update(tea.KeyMsg{Type: tea.KeyEnter})
	if choice != "eu-central-1" {
		t.Fatalf("Enter should return %q, got %q", "eu-central-1", choice)
	}

	// Reopen with the new region
	s.open(awsRegions, "eu-central-1")
	if s.current() != "eu-central-1" {
		t.Errorf("reopen: current() = %q, want %q", s.current(), "eu-central-1")
	}
}

func TestOptionSelectorNonCommonRegionReopenPreservesRegion(t *testing.T) {
	s := newOptionSelectorWithViews("Select Region", awsRegions, commonRegions)

	// Open with sa-east-1 (not in common set, should switch to All view)
	s.open(awsRegions, "sa-east-1")
	if s.viewMode != viewAll {
		t.Fatalf("viewMode = %d, want viewAll (%d)", s.viewMode, viewAll)
	}
	if s.current() != "sa-east-1" {
		t.Fatalf("first open: current() = %q, want %q", s.current(), "sa-east-1")
	}

	// Close and reopen
	s.update(tea.KeyMsg{Type: tea.KeyEscape})
	s.open(awsRegions, "sa-east-1")
	if s.viewMode != viewAll {
		t.Errorf("reopen: viewMode = %d, want viewAll", s.viewMode)
	}
	if s.current() != "sa-east-1" {
		t.Errorf("reopen: current() = %q, want %q", s.current(), "sa-east-1")
	}
}

func TestOptionSelectorFilterResetOnOpen(t *testing.T) {
	s := newOptionSelectorWithViews("Select Region", awsRegions, commonRegions)

	// Open and type a filter
	s.open(awsRegions, "eu-west-1")
	s.input.SetValue("eu")
	s.applyFilter()

	// Should have fewer items
	if len(s.filtered) >= len(commonRegions) {
		t.Fatalf("filter should reduce items, got %d", len(s.filtered))
	}

	// Close and reopen — filter should be cleared
	s.update(tea.KeyMsg{Type: tea.KeyEscape})
	s.open(awsRegions, "eu-west-1")

	if s.input.Value() != "" {
		t.Errorf("input should be cleared on open, got %q", s.input.Value())
	}
	if len(s.filtered) != len(commonRegions) {
		t.Errorf("filtered should have all common regions (%d), got %d", len(commonRegions), len(s.filtered))
	}
	if s.current() != "eu-west-1" {
		t.Errorf("current() = %q, want %q", s.current(), "eu-west-1")
	}
}
