package play

import (
	"strings"
	"testing"

	"google.golang.org/protobuf/encoding/protojson"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
)

// The combat log's slot view per viewer: the resource of an innate use (the key and the uses a day of a
// stat block) is the master's and the caster's owner's; every other viewer reads that the cast was innate.

// slotViewOfLog is the slot the viewer reads on the first log entry of the kind that carries a spell, and the
// whole log as text.
func slotViewOfLog(t *testing.T, a *armed, u *user, e *playv1.Encounter, kind playv1.CombatLogKind) (*playv1.SpellSlot, string) {
	t.Helper()
	res := a.log(t, u, e)
	var slot *playv1.SpellSlot
	for _, r := range res.GetRounds() {
		for _, en := range r.GetEntries() {
			if en.GetKind() == kind && slot == nil {
				slot = en.GetSpell().GetSlot()
			}
		}
	}
	return slot, protojson.Format(res)
}

// wantInnateView checks the viewers of the log of an innate cast: the master reads the resource, a player
// reads the marker and never the key nor the count (canaries: "innate_" and the uses a day, 3).
func wantInnateView(t *testing.T, a *armed, e *playv1.Encounter, kind playv1.CombatLogKind, resource string) {
	t.Helper()
	slot, _ := slotViewOfLog(t, a, a.master, e, kind)
	if slot.GetResource() != resource { // positive control: the master reads it
		t.Fatalf("the master reads slot.resource = %q, want %q", slot.GetResource(), resource)
	}
	for name, u := range map[string]*user{"Caio": a.caio, "Ana": a.ana} {
		slot, text := slotViewOfLog(t, a, u, e, kind)
		if slot == nil {
			t.Fatalf("%s reads no slot on the entry, want the innate marker", name)
		}
		if slot.GetResource() != innateSlotMarker {
			t.Errorf("%s reads slot.resource = %q, want the marker %q", name, slot.GetResource(), innateSlotMarker)
		}
		if strings.Contains(text, "innate_") || strings.Contains(text, "_3_") || strings.Contains(text, "_day") {
			t.Errorf("%s's log carries the key or the uses a day of an innate use:\n%s", name, text)
		}
	}
}

// TestTheLogsReactionLineKeepsAnInnateUseFromPlayers: a visible Couatl casts Shield from an innate use
// and the reaction line of the log shows it.
func TestTheLogsReactionLineKeepsAnInnateUseFromPlayers(t *testing.T) {
	t.Parallel()
	a, e, _ := couatlHit(t)
	w := a.windowOf(t, a.master, playv1.ReactionKind_REACTION_KIND_SHIELD)
	a.mustAnswer(t, a.master, e, w.GetId(), innateUse(couatlShield))
	wantInnateView(t, a, a.get(t, a.master), playv1.CombatLogKind_COMBAT_LOG_KIND_REACTION, couatlShield)
}

// TestTheLogsCastLineKeepsAnInnateUseFromPlayers: the cast line of the log, built from a cast of a visible NPC
// with an innate use (no cast of today stores one; the reaction is the path that does), for each viewer.
func TestTheLogsCastLineKeepsAnInnateUseFromPlayers(t *testing.T) {
	t.Parallel()
	owner, other := "user-owner", "user-other"
	couatl := playdb.Combatant{ID: "couatl", Label: "Couatl", UserID: &owner}
	byID := map[string]playdb.Combatant{couatl.ID: couatl}
	e := &logEntry{ev: actionEvent{Actor: couatl.ID, Slot: &slotRef{Level: 1, Resource: couatlShield}}}
	for name, tc := range map[string]struct {
		v    combatViewer
		want string
	}{
		"master":         {combatViewer{master: true}, couatlShield},
		"caster's owner": {combatViewer{userID: owner}, couatlShield},
		"other player":   {combatViewer{userID: other}, innateSlotMarker},
	} {
		out := e.spellView(tc.v, byID)
		if got := out.GetSlot().GetResource(); got != tc.want {
			t.Errorf("%s reads slot.resource = %q, want %q", name, got, tc.want)
		}
		if text := protojson.Format(out); tc.want == innateSlotMarker && (strings.Contains(text, "innate_") || strings.Contains(text, "_3_")) {
			t.Errorf("%s's cast line carries the key or the uses a day of an innate use:\n%s", name, text)
		}
	}
}

// TestSlotProtoGivesTheResourceToTheMasterAndTheCastersPlayerOnly covers the viewers a table of the
// database cannot make: the caster's own player.
func TestSlotProtoGivesTheResourceToTheMasterAndTheCastersPlayerOnly(t *testing.T) {
	t.Parallel()
	owner, other := "user-owner", "user-other"
	caster := playdb.Combatant{ID: "c1", UserID: &owner}
	slot := &slotRef{Level: 1, Resource: "innate_3_day_shield"}
	for _, tc := range []struct {
		name string
		v    combatViewer
		want string
	}{
		{"master", combatViewer{master: true}, "innate_3_day_shield"},
		{"caster's owner", combatViewer{userID: owner}, "innate_3_day_shield"},
		{"other player", combatViewer{userID: other}, innateSlotMarker},
	} {
		if got := slotProto(slot, tc.v, caster).GetResource(); got != tc.want {
			t.Errorf("%s reads slot.resource = %q, want %q", tc.name, got, tc.want)
		}
	}
	if got := slotProto(&slotRef{Level: 2}, combatViewer{userID: other}, caster); got.GetResource() != "" || got.GetLevel() != 2 {
		t.Errorf("a slot with no resource reads %v to a player, want level 2 and no resource", got)
	}
	if slotProto(nil, combatViewer{}, caster) != nil {
		t.Error("no slot reads a slot")
	}
}
