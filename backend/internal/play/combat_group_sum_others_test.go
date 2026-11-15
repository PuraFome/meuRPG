package play

import (
	"strings"
	"testing"

	"google.golang.org/protobuf/encoding/protojson"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// removalLines are the removal lines of a combat log as one user reads them.
func removalLines(t *testing.T, g *groupHit, u *user) []*playv1.CombatLogEntry {
	t.Helper()
	var out []*playv1.CombatLogEntry
	for _, r := range g.a.log(t, u, g.e).GetRounds() {
		for _, en := range r.GetEntries() {
			if en.GetKind() == playv1.CombatLogKind_COMBAT_LOG_KIND_DAMAGE_PART_REMOVED {
				out = append(out, en)
			}
		}
	}
	return out
}

// #835 decision: another player reads that the master took the part out, by the name the roll line gave it, and never why.
func TestAnotherPlayerReadsWhatTheMasterTookOutOfAGroupButNotWhy(t *testing.T) {
	t.Parallel()
	g := newGroupHit(t, 12)
	if _, err := g.takeOut(t, "sneak-attack", "segredo do mestre"); err != nil {
		t.Fatalf("RemoveDamagePart() error = %v", err)
	}
	attacker := removalLines(t, g, g.a.caio)
	if len(attacker) != 1 || attacker[0].GetKeyNamePt() == "" || attacker[0].GetReason() != "segredo do mestre" {
		t.Fatalf("the attacker reads %v, want one line with the name and the reason", attacker)
	}
	other := removalLines(t, g, g.a.ana)
	if len(other) != 1 {
		t.Fatalf("another player reads %d removal lines, want 1", len(other))
	}
	if other[0].GetKeyNamePt() != attacker[0].GetKeyNamePt() || other[0].GetReason() != "" || other[0].GetKey() != "" {
		t.Errorf("another player reads name %q reason %q key %q, want the name %q and no reason, no key", other[0].GetKeyNamePt(), other[0].GetReason(), other[0].GetKey(), attacker[0].GetKeyNamePt())
	}
}

// A part the roll line hides from the other players is "Outra fonte" in the removal line too, with no key and no name.
func TestAnotherPlayerReadsAHiddenPartTheMasterTookOutAsAnotherSource(t *testing.T) {
	t.Parallel()
	g := newGroupHit(t, 12)
	if _, err := g.a.h.pool.Exec(t.Context(), `UPDATE pending_damages
		SET parts = (SELECT jsonb_agg(CASE WHEN p->>'key' = 'sneak-attack' THEN p || '{"hide": true, "label": "Marca Secreta"}' ELSE p END) FROM jsonb_array_elements(parts) p),
		    part_rolls = (SELECT jsonb_agg(CASE WHEN p->>'key' = 'sneak-attack' THEN p || '{"hide": true, "label": "Marca Secreta"}' ELSE p END) FROM jsonb_array_elements(part_rolls) p)
		WHERE id = $1`, g.id); err != nil {
		t.Fatalf("hide the part: %v", err)
	}
	if _, err := g.takeOut(t, "sneak-attack", "segredo do mestre"); err != nil {
		t.Fatalf("RemoveDamagePart() error = %v", err)
	}
	if mine := removalLines(t, g, g.a.master); len(mine) != 1 || mine[0].GetKeyNamePt() != "Marca Secreta" || mine[0].GetKey() != "sneak-attack" {
		t.Fatalf("the master reads %v, want the part's own name and key", mine)
	}
	other := removalLines(t, g, g.a.ana)
	if len(other) != 1 || other[0].GetKeyNamePt() != otherSourceTx || other[0].GetKey() != "" || other[0].GetReason() != "" {
		t.Errorf("another player reads %v, want Outra fonte with no key and no reason", other)
	}
	if b, err := protojson.Marshal(g.a.log(t, g.a.ana, g.e)); err != nil || strings.Contains(string(b), "Marca Secreta") || strings.Contains(string(b), "sneak-attack") {
		t.Errorf("another player's log carries the hidden part's name or key (%v): %s", err, b)
	}
}
