package characters

import (
	"testing"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	"github.com/PuraFome/meuRPG/backend/internal/play/link"
)

// A recast that brings a dismissed steed back puts one more creature among the character's live ones, so it is held to the cap of 40 like any
// summon; a steed that is already live adds none.
func TestSteedRecastCountsAgainstTheCreatureCap(t *testing.T) {
	t.Parallel()
	rats := func(n int) []link.CreatureSpec {
		out := make([]link.CreatureSpec, n)
		for i := range out {
			out[i] = link.CreatureSpec{MonsterKey: "monster:rat", Attack: "full"}
		}
		return out
	}
	recast := link.Summon{Source: creatureSourceSteed, Creatures: []link.CreatureSpec{{MonsterKey: "monster:wolf", Attack: "full"}}}

	t.Run("a dismissed steed past the cap stays away", func(t *testing.T) {
		t.Parallel()
		tb := newCreatureTable(t)
		id := tb.steed(t, "Trovão")
		if _, err := tb.h.pool.Exec(t.Context(), `UPDATE character_creatures SET dismissed_at = now(), dismissed_reason = 'dismissed', hp_current = 0 WHERE id = $1`, id); err != nil {
			t.Fatal(err)
		}
		if _, err := tb.summonInTx(t, link.Summon{Source: "conjure_animals", Creatures: rats(maxCreaturesPerCharacter)}); err != nil {
			t.Fatalf("up to the limit: %v", err)
		}
		_, err := tb.summonInTx(t, recast)
		if b := blocked(t, "the recast of a dismissed steed at 40 live creatures", err); b.GetReason() != charactersv1.CharacterBlockedReason_CHARACTER_BLOCKED_REASON_CREATURE_LIMIT {
			t.Errorf("blocked = %v", b)
		}
		var dismissed bool
		if err := tb.h.pool.QueryRow(t.Context(), "SELECT dismissed_at IS NOT NULL FROM character_creatures WHERE id = $1", id).Scan(&dismissed); err != nil || !dismissed {
			t.Errorf("the steed is dismissed = %v (%v), want it left as it was", dismissed, err)
		}
	})

	t.Run("a live steed adds none", func(t *testing.T) {
		t.Parallel()
		tb := newCreatureTable(t)
		tb.steed(t, "Trovão")
		if _, err := tb.summonInTx(t, link.Summon{Source: "conjure_animals", Creatures: rats(maxCreaturesPerCharacter - 1)}); err != nil {
			t.Fatalf("up to the limit: %v", err)
		}
		if _, err := tb.summonInTx(t, recast); err != nil {
			t.Errorf("the recast of a live steed at 40 live creatures: %v", err)
		}
	})
}
