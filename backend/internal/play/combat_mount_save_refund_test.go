package play

import (
	"testing"

	"connectrpc.com/connect"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// The question kept for a rider's save (Lucky, the Bardic die) that spent the Inspiration goes with its window, and the Inspiration goes
// back with it (hold_forget.go): the roll never happened. RollMountSave takes no spend of the Inspiration, so the spend is written the way
// the paths that take it write it: the hold's favor_spent and the character's row gone.

// askedSave sits Toren on the Goblin, asks his save and rolls it so that the question is kept (a die, or a halfling's 1), then marks the
// hold as having spent the Inspiration. It returns the hold.
func (f *favorFight) askedSave(t *testing.T, die bool) string {
	t.Helper()
	f.execSQL(t, `UPDATE combatants SET mounted_on_id = $1, grid_col = m.grid_col, grid_row = m.grid_row FROM combatants m WHERE m.id = $1 AND combatants.id = $2`, f.id(t, "Goblin"), f.id(t, "Toren"))
	face := int32(1)
	if die {
		f.giveCombatantDie(t, "Toren", f.e)
		face = 3
	}
	f.askSave(t, "Toren", causePushed)
	res, err := f.caio.combat.RollMountSave(t.Context(), connect.NewRequest(&playv1.RollMountSaveRequest{
		CampaignId: f.campaignID, EncounterId: f.e.GetId(), RiderId: f.id(t, "Toren"), Roll: &playv1.RollMountSaveRequest_D20Face{D20Face: face}, IdempotencyKey: newKey(),
	}))
	if err != nil || res.Msg.GetHeldRollOffer().GetHoldId() == "" {
		t.Fatalf("RollMountSave(%d) = %v, %v, want the question of the die or of Lucky", face, res.Msg, err)
	}
	id := res.Msg.GetHeldRollOffer().GetHoldId()
	f.give(t, f.toren)
	f.execSQL(t, `DELETE FROM character_favor WHERE campaign_id = $1`, f.campaignID)
	f.execSQL(t, `UPDATE inspiration_holds SET favor_spent = true WHERE id = $1`, id)
	if f.favorHolds(t) || !f.holdFavorOf(t, id) {
		t.Fatal("the setup did not leave a hold that spent the Inspiration")
	}
	return id
}

// defeatToren puts the rider at 0 (a player's hit points are its vitals') and lets the combat settle with a change of the Goblin's.
func (f *favorFight) defeatToren(t *testing.T) {
	t.Helper()
	f.execSQL(t, `UPDATE combatants SET defeated = true WHERE id = $1`, f.id(t, "Toren"))
	if _, err := f.adjustHP(t, f.e, "Goblin", damageHP(1)); err != nil {
		t.Fatalf("AdjustCombatantHitPoints() error = %v", err)
	}
}

func TestMountedCombat_TheInspirationGoesBackWhenTheWindowOfAHeldSaveCloses(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name  string
		close func(t *testing.T, f *favorFight)
	}{
		{"the rider is defeated", func(t *testing.T, f *favorFight) {
			t.Helper()
			f.defeatToren(t)
		}},
		{"the push is undone", func(t *testing.T, f *favorFight) {
			t.Helper()
			f.execSQL(t, `UPDATE combatants SET grid_col = grid_col + 1 WHERE id = $1 OR mounted_on_id = $1`, f.id(t, "Goblin"))
			if _, err := f.adjustHP(t, f.e, "Goblin", damageHP(1)); err != nil {
				t.Fatalf("AdjustCombatantHitPoints() error = %v", err)
			}
		}},
		{"the encounter ends", func(t *testing.T, f *favorFight) {
			t.Helper()
			if _, err := f.master.combat.EndEncounter(t.Context(), connect.NewRequest(&playv1.EndEncounterRequest{CampaignId: f.campaignID, EncounterId: f.e.GetId(), IdempotencyKey: newKey()})); err != nil {
				t.Fatalf("EndEncounter() error = %v", err)
			}
		}},
	} {
		for _, kept := range []struct {
			name string
			die  bool
		}{{"a Bardic die", true}, {"a Lucky 1", false}} {
			t.Run(tt.name+" with "+kept.name, func(t *testing.T) {
				t.Parallel()
				f := newHalflingFavorFight(t)
				id := f.askedSave(t, kept.die)
				tt.close(t, f)
				f.wantGivenBack(t, tt.name, id)
			})
		}
	}
	t.Run("the prone is undone", func(t *testing.T) {
		t.Parallel()
		f := newHalflingFavorFight(t)
		f.execSQL(t, `UPDATE combatants SET conditions = ARRAY['condition:prone'] WHERE id = $1`, f.id(t, "Toren"))
		f.execSQL(t, `UPDATE combatants SET mounted_on_id = $1, grid_col = m.grid_col, grid_row = m.grid_row FROM combatants m WHERE m.id = $1 AND combatants.id = $2`, f.id(t, "Goblin"), f.id(t, "Toren"))
		f.askSave(t, "Toren", causeProne)
		res, err := f.caio.combat.RollMountSave(t.Context(), connect.NewRequest(&playv1.RollMountSaveRequest{
			CampaignId: f.campaignID, EncounterId: f.e.GetId(), RiderId: f.id(t, "Toren"), Roll: &playv1.RollMountSaveRequest_D20Face{D20Face: 1}, IdempotencyKey: newKey(),
		}))
		if err != nil || res.Msg.GetHeldRollOffer().GetHoldId() == "" {
			t.Fatalf("RollMountSave(1) = %v, %v, want the question of Lucky", res.Msg, err)
		}
		id := res.Msg.GetHeldRollOffer().GetHoldId()
		f.give(t, f.toren)
		f.execSQL(t, `DELETE FROM character_favor WHERE campaign_id = $1`, f.campaignID)
		f.execSQL(t, `UPDATE inspiration_holds SET favor_spent = true WHERE id = $1`, id)
		f.execSQL(t, `UPDATE combatants SET conditions = '{}' WHERE id = $1`, f.id(t, "Toren"))
		if _, err := f.adjustHP(t, f.e, "Goblin", damageHP(1)); err != nil {
			t.Fatalf("AdjustCombatantHitPoints() error = %v", err)
		}
		f.wantGivenBack(t, "the prone taken off", id)
	})
	t.Run("a hold that spent nothing gives nothing back", func(t *testing.T) {
		t.Parallel()
		f := newHalflingFavorFight(t)
		id := f.askedSave(t, false)
		f.execSQL(t, `UPDATE inspiration_holds SET favor_spent = false WHERE id = $1`, id)
		f.defeatToren(t)
		if f.holdExists(t, id) || f.rows(t) != 0 {
			t.Errorf("the hold is there = %v, the Inspiration rows = %d, want the hold gone and nothing given", f.holdExists(t, id), f.rows(t))
		}
	})
}
