package play

import (
	"slices"
	"strings"
	"testing"

	"google.golang.org/protobuf/encoding/protojson"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
)

// A condition with no outward sign (Envenenado) is the owner's and the master's (RN-10): the label
// in the encounter and the combat log, is named to nobody else.
func TestRN10_AnOwnerOnlyConditionIsInNoReadOfAnotherPlayer(t *testing.T) {
	t.Parallel()
	a := newCasters(t)
	e := a.castersFightNPCFirst(t)
	const needle = "Envenenado" // the Portuguese name of condition:poisoned; no other text of these reads has it

	both := []string{"condition:poisoned", "condition:prone"}
	// On an NPC and on Pensantus, a character of ana's: caio owns neither.
	for _, who := range []string{"Goblin", "Pensantus"} {
		var err error
		if e, err = a.conditions(t, a.master, e, who, both, true, false); err != nil {
			t.Fatalf("SetCombatantConditions(%s) error = %v", who, err)
		}
	}

	read := func(u *user) string {
		return protojson.Format(a.get(t, u)) + protojson.Format(a.log(t, u, e))
	}
	// The positive controls: the master and the owner read the condition and the log line.
	if got := read(a.master); !strings.Contains(got, needle) || !strings.Contains(got, "condition:poisoned") {
		t.Errorf("the master's read does not hold %s: the needle is unreachable", needle)
	}
	if got := protojson.Format(byLabel(t, a.get(t, a.ana), "Pensantus")); !strings.Contains(got, needle) {
		t.Errorf("the owner's read of their own character does not hold %s", needle)
	}
	// Another player reads neither the condition nor a log line that names it, on the NPC or on a character that is not theirs.
	if got := read(a.caio); strings.Contains(got, needle) || strings.Contains(got, "poisoned") {
		t.Errorf("a player who owns neither reads the owner-only condition:\n%s", got)
	}
	for _, line := range logEntries(a.log(t, a.caio, e)) {
		if line.GetKind() == playv1.CombatLogKind_COMBAT_LOG_KIND_CONDITIONS_CHANGED && !slices.Equal(line.GetConditions(), []string{"condition:prone"}) {
			t.Errorf("a player's log line carries conditions %v, want Prone alone", line.GetConditions())
		}
	}
	// A change of nothing but an owner-only condition is no line for the others.
	before := len(logEntries(a.log(t, a.caio, e)))
	if _, err := a.conditions(t, a.master, e, "Goblin", []string{"condition:poisoned", "condition:prone", "condition:frightened"}, true, false); err != nil {
		t.Fatalf("SetCombatantConditions(frightened) error = %v", err)
	}
	if after := len(logEntries(a.log(t, a.caio, e))); after != before {
		t.Errorf("a player's log went from %d to %d lines for a change they cannot read", before, after)
	}
}

// The label of an effect that gives conditions is the names of the ones the reader may read
// (RN-10): a catalog or table effect with Envenenado and Derrubado reads "Derrubado" to another
// player, and no label at all when it gives Envenenado alone.
func TestRN10_TheLabelOfAnEffectNamesOnlyTheConditionsTheViewerMayRead(t *testing.T) {
	t.Parallel()
	content, err := testRules()
	if err != nil {
		t.Fatalf("rules.LoadSRD() error = %v", err)
	}
	owner, other := "user-owner", "user-other"
	target := playdb.Combatant{ID: "c1", UserID: &owner}
	names := map[string]string{"condition:poisoned": "Envenenado", "condition:prone": "Derrubado"}
	view := func(user string, master bool) *effectViewer {
		return &effectViewer{
			s: &Service{}, content: content, d: &encounterData{}, byID: map[string]playdb.Combatant{"c1": target},
			names: func(k string) string { return names[k] }, v: combatViewer{master: master, userID: user},
		}
	}
	both := playdb.CombatantState{CombatantID: "c1", ConditionKeys: []string{"condition:poisoned", "condition:prone"}}
	alone := playdb.CombatantState{CombatantID: "c1", ConditionKeys: []string{"condition:poisoned"}}
	for _, c := range []struct {
		who         string
		ev          *effectViewer
		both, alone string
	}{
		{"the master", view("", true), "Envenenado, Derrubado", "Envenenado"},
		{"the owner", view(owner, false), "Envenenado, Derrubado", "Envenenado"},
		{"another player", view(other, false), "Derrubado", ""},
	} {
		if got := c.ev.labelFor(both); got != c.both {
			t.Errorf("%s reads %q for Envenenado and Derrubado, want %q", c.who, got, c.both)
		}
		if got := c.ev.labelFor(alone); got != c.alone {
			t.Errorf("%s reads %q for Envenenado alone, want %q", c.who, got, c.alone)
		}
	}
	if got := view("", true).labelForOthers(both); got != "Derrubado" {
		t.Errorf("the master's \"Os jogadores veem\" = %q, want Derrubado", got)
	}
}
