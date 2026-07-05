package skills

import (
	"reflect"
	"strings"
	"testing"
)

func TestNormalizeTargetsDefaultsAndWarnsOnceOnUnknown(t *testing.T) {
	var warn strings.Builder

	if got, want := NormalizeTargets("", &warn), []string{"agents", "claude", "cursor"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("expected default targets %v, got %v", want, got)
	}

	got := NormalizeTargets("bogus,claude,other", &warn)
	if want := []string{"claude"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("expected %v, got %v", want, got)
	}
	wantWarn := "Unknown install target 'bogus' in SKILL_INSTALL_TARGETS (expected agents, claude, cursor)\n"
	if warn.String() != wantWarn {
		t.Fatalf("expected one warning %q, got %q", wantWarn, warn.String())
	}
}

func TestTeamSkipsWhenClaudeNotInInstallTargets(t *testing.T) {
	cases := []struct {
		targets []string
		kind    Kind
		managed bool
	}{
		{[]string{"agents", "cursor"}, KindTeam, false},
		{[]string{"agents", "cursor"}, KindTeamHybrid, true},
		{[]string{"cursor"}, KindTeamHybrid, false},
		{[]string{"claude"}, KindTeam, true},
		{[]string{"claude"}, KindTeamHybrid, true},
		{[]string{"agents", "claude", "cursor"}, KindTeam, true},
	}
	for _, c := range cases {
		cfg := Config{Targets: c.targets}
		if got := cfg.TeamManaged(c.kind); got != c.managed {
			t.Errorf("TeamManaged(%s) with targets %v = %v, want %v", c.kind, c.targets, got, c.managed)
		}
	}
}

func TestNormalizeTargetsDeduplicatesAndLowercases(t *testing.T) {
	var warn strings.Builder

	got := NormalizeTargets(" Claude,AGENTS,claude ", &warn)

	if want := []string{"claude", "agents"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("expected deduped lowercase targets %v, got %v", want, got)
	}
	if warn.Len() != 0 {
		t.Fatalf("expected no warnings, got %q", warn.String())
	}
}
