package skills

import (
	"reflect"
	"strings"
	"testing"
)

func TestNormalizeTargetsDefaultsAndWarnsOnceOnUnknown(t *testing.T) {
	var warn strings.Builder

	if got, want := NormalizeTargets("", &warn), []Target{"agents", "claude"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("expected default targets %v, got %v", want, got)
	}

	got := NormalizeTargets("bogus,claude,other", &warn)
	if want := []Target{"claude"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("expected %v, got %v", want, got)
	}
	wantWarn := "Unknown install target 'bogus' in SKILL_INSTALL_TARGETS (expected agents, claude)\n"
	if warn.String() != wantWarn {
		t.Fatalf("expected one warning %q, got %q", wantWarn, warn.String())
	}
}

func TestNormalizeTargetsDeduplicatesAndLowercases(t *testing.T) {
	var warn strings.Builder

	got := NormalizeTargets(" Claude,AGENTS,claude ", &warn)

	if want := []Target{"claude", "agents"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("expected deduped lowercase targets %v, got %v", want, got)
	}
	if warn.Len() != 0 {
		t.Fatalf("expected no warnings, got %q", warn.String())
	}
}
