package play

import (
	"testing"

	"connectrpc.com/connect"

	campaignsv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/campaigns/v1"
	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
	"github.com/PuraFome/meuRPG/backend/internal/authz"
)

func (a *armed) castByID(t *testing.T, u *user, id string) *playv1.OutsideCast {
	t.Helper()
	for _, c := range append(a.casts(t, u).GetActive(), a.casts(t, u).GetLog()...) {
		if c.GetId() == id {
			return c
		}
	}
	return nil
}

func (a *armed) interrupt(t *testing.T, u *user, castID string) error {
	t.Helper()
	_, err := u.casting.AbandonCast(t.Context(), connect.NewRequest(&playv1.AbandonCastRequest{CampaignId: a.campaignID, CastId: castID, IdempotencyKey: newKey()}))
	return err
}

func (a *armed) endSpell(t *testing.T, u *user, castID string) (*playv1.EndActiveSpellResponse, error) {
	t.Helper()
	res, err := u.casting.EndActiveSpell(t.Context(), connect.NewRequest(&playv1.EndActiveSpellRequest{CampaignId: a.campaignID, CastId: castID, IdempotencyKey: newKey()}))
	if err != nil {
		return nil, err
	}
	return res.Msg, nil
}

// onStage puts an NPC on the stage of the open session.
func (a *armed) onStage(t *testing.T, npc *charactersv1.Character) {
	t.Helper()
	if _, err := a.h.pool.Exec(t.Context(), `INSERT INTO stage_npcs (game_session_id, character_id, position, created_at)
		SELECT id, $2, 0, now() FROM game_sessions WHERE campaign_id = $1 AND ended_at IS NULL`, a.campaignID, npc.GetId()); err != nil {
		t.Fatalf("put the NPC on the stage: %v", err)
	}
}

// TestCastingOutside_ARitualTakesTenMinutesMoreAndSpendsNoSlot: Alarm is a ritual of
// 1 minute; as a ritual it takes 11 and spends no slot. A wizard casts it from the
// spellbook without preparing it; a cleric needs the ritual prepared, and a fighter
// has no ritual feature (SRD 5.1, "Rituals").
func TestCastingOutside_ARitualTakesTenMinutesMoreAndSpendsNoSlot(t *testing.T) {
	t.Parallel()
	a := newCastingParty(t)
	before := usedSlots(a.vitals(t, a.pens), 1)

	res := a.mustCastOut(t, a.ana, a.pens, alarmKey, nil, true, nil)
	cast := res.GetCast()
	if cast.GetStatus() != playv1.OutsideCastStatus_OUTSIDE_CAST_STATUS_CASTING || !cast.GetRitual() || cast.GetCastingMinutes() != 11 || !cast.GetConcentrating() {
		t.Fatalf("cast = %v, want a ritual being cast, 11 minutes, concentrating on the casting", cast)
	}
	if got := usedSlots(a.vitals(t, a.pens), 1); got != before {
		t.Errorf("slots used = %d, want %d: nothing is spent while casting", got, before)
	}
	// One cast at a time.
	_, err := a.castOut(t, a.ana, a.pens, alarmKey, nil, true, nil)
	wantCastingBlocked(t, "a second casting", err, playv1.CastingBlockedReason_CASTING_BLOCKED_REASON_CAST_IN_PROGRESS)
	// Only the master says the time has passed.
	_, err = a.finish(t, a.ana, cast.GetId())
	wantCode(t, "FinishCast as the caster's player", err, connect.CodePermissionDenied)

	done, err := a.finish(t, a.master, cast.GetId())
	if err != nil {
		t.Fatalf("FinishCast() error = %v", err)
	}
	// Alarm lasts 8 hours and needs no concentration: it stays, and the caster is free.
	if c := done.GetCast(); c.GetStatus() != playv1.OutsideCastStatus_OUTSIDE_CAST_STATUS_ACTIVE || c.GetConcentrating() || c.GetDurationSeconds() != 8*3600 ||
		c.GetRestEnds() != playv1.RestThatEnds_REST_THAT_ENDS_LONG {
		t.Errorf("finished cast = %v, want active for 8 hours, ended by a long rest, no concentration", c)
	}
	if got := usedSlots(a.vitals(t, a.pens), 1); got != before {
		t.Errorf("slots used = %d, want %d: a ritual spends none", got, before)
	}

	// Not every spell is a ritual, and not every class casts them.
	_, err = a.castOut(t, a.ana, a.pens, magicMissileSpell, nil, true, nil)
	wantCastingBlocked(t, "a ritual of a spell without the tag", err, playv1.CastingBlockedReason_CASTING_BLOCKED_REASON_NOT_A_RITUAL)
	_, err = a.castOut(t, a.ana, a.pens, alarmKey, slotOfLevel(1), true, nil)
	wantCode(t, "a ritual with a slot", err, connect.CodeInvalidArgument)
	_, err = a.castOut(t, a.caio, a.toren, alarmKey, nil, true, nil)
	wantCastingBlocked(t, "a fighter's ritual", err, playv1.CastingBlockedReason_CASTING_BLOCKED_REASON_CLASS_CANNOT_RITUAL)
	// The cleric has Detect Magic prepared; the wizard's Identify is only in her book.
	_, err = a.castOut(t, a.bia, a.bri, identifyKey, nil, true, nil)
	wantCastingBlocked(t, "a ritual the cleric does not have", err, playv1.CastingBlockedReason_CASTING_BLOCKED_REASON_CLASS_CANNOT_RITUAL)
	if r := a.mustCastOut(t, a.bia, a.bri, detectMagicKey, nil, true, nil).GetCast(); r.GetCastingMinutes() != 10 {
		t.Errorf("Detect Magic as a ritual takes %d minutes, want 10 (an action + 10)", r.GetCastingMinutes())
	}
}

// TestCastingOutside_ALongCastFailsWhenInterruptedAndGoesOnInACombat: a spell that takes
// minutes is cast in two steps, spends the slot only when it completes, fails without
// spending it when the concentration breaks, and a combat that begins does not cancel it
// (the caster spends the action each turn): the combatant carries the concentration
// (SRD 5.1, "Longer Casting Times").
func TestCastingOutside_ALongCastFailsWhenInterruptedAndGoesOnInACombat(t *testing.T) {
	t.Parallel()
	a := newCastingParty(t)
	tor := []*charactersv1.Character{a.toren}

	start := a.mustCastOut(t, a.bia, a.bri, prayerKey, slotOfLevel(2), false, tor).GetCast()
	if start.GetStatus() != playv1.OutsideCastStatus_OUTSIDE_CAST_STATUS_CASTING || start.GetCastingMinutes() != 10 || usedSlots(a.vitals(t, a.bri), 2) != 0 {
		t.Fatalf("cast = %v, want 10 minutes being cast and no slot spent", start)
	}
	if err := a.interrupt(t, a.bia, start.GetId()); err != nil {
		t.Fatalf("InterruptCast() error = %v", err)
	}
	if c := a.castByID(t, a.master, start.GetId()); c.GetStatus() != playv1.OutsideCastStatus_OUTSIDE_CAST_STATUS_FAILED || c.GetEndReason() != playv1.OutsideCastEnd_OUTSIDE_CAST_END_INTERRUPTED {
		t.Errorf("interrupted cast = %v, want failed", c)
	}
	if got := usedSlots(a.vitals(t, a.bri), 2); got != 0 {
		t.Errorf("slots used = %d: a casting that fails spends none", got)
	}
	_, err := a.finish(t, a.master, start.GetId())
	wantCastingBlocked(t, "FinishCast on a failed cast", err, playv1.CastingBlockedReason_CASTING_BLOCKED_REASON_CAST_NOT_GOING)

	// Again, and this time a combat starts before the time passes.
	start = a.mustCastOut(t, a.bia, a.bri, prayerKey, slotOfLevel(2), false, tor).GetCast()
	e := a.start(t, plan{
		npcs: []*playv1.Participant{{CharacterId: a.goblin.GetId()}}, npcRolls: []int{2},
		players: map[string]int32{"Toren": 12, "Brisa": 11, "Pensantus": 5}, reveal: []string{"Goblin"},
		at: map[string][2]int32{"Toren": {3, 3}, "Goblin": {4, 3}, "Brisa": {6, 5}},
	})
	if c := a.castByID(t, a.master, start.GetId()); c.GetStatus() != playv1.OutsideCastStatus_OUTSIDE_CAST_STATUS_CASTING {
		t.Errorf("cast when the combat began = %v, want it going on", c)
	}
	if got := byLabel(t, e, "Brisa").GetConcentrationSpell(); got != prayerKey {
		t.Errorf("Brisa concentrates on %q in the combat, want the casting of %s", got, prayerKey)
	}
	_, err = a.finish(t, a.master, start.GetId())
	wantCastingBlocked(t, "FinishCast in a combat", err, playv1.CastingBlockedReason_CASTING_BLOCKED_REASON_IN_COMBAT)
	// A character in a combat casts there.
	_, err = a.castOut(t, a.bia, a.bri, cureWounds, slotOfLevel(1), false, tor)
	wantCastingBlocked(t, "a cast outside while in a combat", err, playv1.CastingBlockedReason_CASTING_BLOCKED_REASON_IN_COMBAT)

	// The concentration survives the combat: it ends, and the cast is still going.
	if _, err := a.master.combat.EndEncounter(t.Context(), connect.NewRequest(&playv1.EndEncounterRequest{CampaignId: a.campaignID, EncounterId: e.GetId(), IdempotencyKey: newKey()})); err != nil {
		t.Fatalf("EndEncounter() error = %v", err)
	}
	if c := a.castByID(t, a.master, start.GetId()); c.GetStatus() != playv1.OutsideCastStatus_OUTSIDE_CAST_STATUS_CASTING || !c.GetConcentrating() {
		t.Errorf("cast after the combat = %v, want it going and concentrating", c)
	}
	a.hurt(t, a.toren, 10)
	done, err := a.finish(t, a.master, start.GetId())
	if err != nil {
		t.Fatalf("FinishCast() error = %v", err)
	}
	if usedSlots(a.vitals(t, a.bri), 2) != 1 || done.GetCast().GetStatus() != playv1.OutsideCastStatus_OUTSIDE_CAST_STATUS_ENDED {
		t.Errorf("finished = %v, slots used %d; want the slot spent now and an instantaneous spell over", done.GetCast(), usedSlots(a.vitals(t, a.bri), 2))
	}
	if a.vitals(t, a.toren).GetHitPointsCurrent() <= 10 {
		t.Errorf("Prayer of Healing healed nobody")
	}
}

// TestCastingOutside_ConcentrationIsOneSpellAtATimeAndGoesIntoACombat: casting another
// concentration spell ends the first (SRD 5.1, "Duration"); the combatant carries the
// concentration in, and a spell whose concentration broke in the fight ends with it.
func TestCastingOutside_ConcentrationIsOneSpellAtATimeAndGoesIntoACombat(t *testing.T) {
	t.Parallel()
	a := newCastingParty(t)
	tor := []*charactersv1.Character{a.toren}
	first := a.mustCastOut(t, a.bia, a.bri, blessKey, slotOfLevel(1), false, tor).GetCast()
	if first.GetStatus() != playv1.OutsideCastStatus_OUTSIDE_CAST_STATUS_ACTIVE || !first.GetConcentrating() {
		t.Fatalf("Bless = %v, want active and concentrating", first)
	}
	o, err := a.bia.casting.GetCastOptions(t.Context(), connect.NewRequest(&playv1.GetCastOptionsRequest{CampaignId: a.campaignID, CharacterId: a.bri.GetId()}))
	if err != nil || o.Msg.GetConcentrating().GetId() != first.GetId() {
		t.Fatalf("GetCastOptions() = %v, %v; want the concentration on the sheet", o, err)
	}
	// The master sees the concentration too.
	if got := len(a.casts(t, a.master).GetActive()); got != 1 {
		t.Fatalf("active casts = %d, want 1", got)
	}
	res := a.mustCastOut(t, a.bia, a.bri, detectMagicKey, slotOfLevel(1), false, nil)
	if len(res.GetEndedCastIds()) != 1 || res.GetEndedCastIds()[0] != first.GetId() {
		t.Errorf("ended = %v, want Bless", res.GetEndedCastIds())
	}
	if c := a.castByID(t, a.master, first.GetId()); c.GetStatus() != playv1.OutsideCastStatus_OUTSIDE_CAST_STATUS_ENDED || c.GetEndReason() != playv1.OutsideCastEnd_OUTSIDE_CAST_END_CONCENTRATION || c.GetConcentrating() {
		t.Errorf("Bless after Detect Magic = %v, want ended by concentration", c)
	}

	e := a.start(t, plan{
		npcs: []*playv1.Participant{{CharacterId: a.goblin.GetId()}}, npcRolls: []int{2},
		players: map[string]int32{"Toren": 12, "Brisa": 11, "Pensantus": 5}, reveal: []string{"Goblin"},
		at: map[string][2]int32{"Toren": {3, 3}, "Goblin": {4, 3}, "Brisa": {6, 5}},
	})
	if got := byLabel(t, e, "Brisa").GetConcentrationSpell(); got != detectMagicKey {
		t.Fatalf("Brisa's concentration in the combat = %q, want Detect Magic", got)
	}
	// It breaks in the fight: the spell ends when the combat does.
	if _, err := a.bia.combat.EndConcentration(t.Context(), connect.NewRequest(&playv1.EndConcentrationRequest{
		CampaignId: a.campaignID, EncounterId: e.GetId(), CombatantId: byLabel(t, e, "Brisa").GetId(), IdempotencyKey: newKey(),
	})); err != nil {
		t.Fatalf("EndConcentration() error = %v", err)
	}
	if _, err := a.master.combat.EndEncounter(t.Context(), connect.NewRequest(&playv1.EndEncounterRequest{CampaignId: a.campaignID, EncounterId: e.GetId(), IdempotencyKey: newKey()})); err != nil {
		t.Fatalf("EndEncounter() error = %v", err)
	}
	if c := a.castByID(t, a.master, res.GetCast().GetId()); c.GetStatus() != playv1.OutsideCastStatus_OUTSIDE_CAST_STATUS_ENDED {
		t.Errorf("Detect Magic after a broken concentration = %v, want ended", c)
	}
}

// TestCastingOutside_MageArmorSetsTheBaseArmorClassForCombat: Mage Armor makes the base AC
// 13 + Dexterity for a creature that wears no armor; a combat that starts later uses it, and
// ending the spell takes it back (SRD 5.1, Mage Armor).
func TestCastingOutside_MageArmorSetsTheBaseArmorClassForCombat(t *testing.T) {
	t.Parallel()
	a := newCastingParty(t)
	res := a.mustCastOut(t, a.ana, a.pens, mageArmorKey, slotOfLevel(1), false, []*charactersv1.Character{a.toren})
	cast := res.GetCast()
	if cast.GetStatus() != playv1.OutsideCastStatus_OUTSIDE_CAST_STATUS_ACTIVE || cast.GetDurationSeconds() != 8*3600 {
		t.Fatalf("Mage Armor = %v, want active for 8 hours", cast)
	}
	// Toren has DEX 13 + 1 (human) = 14 (+2): 13 + 2 = 15 against his 12.
	var ac int32
	for _, c := range a.casts(t, a.caio).GetActive() {
		ac = c.GetTargets()[0].GetArmorClass()
	}
	if ac != 15 {
		t.Fatalf("armor class given = %d, want 15", ac)
	}
	e := a.start(t, plan{
		npcs: []*playv1.Participant{{CharacterId: a.goblin.GetId()}}, npcRolls: []int{2},
		players: map[string]int32{"Toren": 12, "Pensantus": 11, "Brisa": 5}, reveal: []string{"Goblin"},
		at: map[string][2]int32{"Toren": {3, 3}, "Goblin": {4, 3}, "Pensantus": {6, 5}, "Brisa": {7, 5}},
	})
	armorOfToren := func() int {
		cs, err := a.h.svc.queries.ListCombatants(t.Context(), e.GetId())
		if err != nil {
			t.Fatal(err)
		}
		for _, c := range cs {
			if c.Label == "Toren" {
				sheet, err := a.h.svc.sheetOf(t.Context(), nil, a.campaignID, c)
				if err != nil {
					t.Fatal(err)
				}
				return sheet.ArmorClass
			}
		}
		return 0
	}
	if got := armorOfToren(); got != 15 {
		t.Errorf("Toren's armor class in the combat = %d, want 15 with Mage Armor", got)
	}
	if _, err := a.master.combat.EndEncounter(t.Context(), connect.NewRequest(&playv1.EndEncounterRequest{CampaignId: a.campaignID, EncounterId: e.GetId(), IdempotencyKey: newKey()})); err != nil {
		t.Fatalf("EndEncounter() error = %v", err)
	}
	// The caster's player ends their own spell; Toren's armor class goes back.
	if _, err := a.endSpell(t, a.caio, cast.GetId()); connect.CodeOf(err) != connect.CodeNotFound && connect.CodeOf(err) != connect.CodePermissionDenied {
		t.Errorf("EndSpell by another player error = %v, want a refusal", err)
	}
	if _, err := a.endSpell(t, a.ana, cast.GetId()); err != nil {
		t.Fatalf("EndSpell() error = %v", err)
	}
	if c := a.castByID(t, a.master, cast.GetId()); c.GetStatus() != playv1.OutsideCastStatus_OUTSIDE_CAST_STATUS_ENDED || c.GetEndReason() != playv1.OutsideCastEnd_OUTSIDE_CAST_END_DISMISSED {
		t.Errorf("Mage Armor after EndSpell = %v, want dismissed", c)
	}
	var n int
	if err := a.h.pool.QueryRow(t.Context(), `SELECT count(*) FROM combatants WHERE mage_armor_ac IS NOT NULL AND encounter_id IN (SELECT id FROM encounters WHERE status <> 'ended')`).Scan(&n); err != nil || n != 0 {
		t.Errorf("combatants with Mage Armor = %d, %v; want none", n, err)
	}
	// Ending it again changes nothing.
	if _, err := a.endSpell(t, a.ana, cast.GetId()); err != nil {
		t.Errorf("EndSpell again error = %v, want nothing to change", err)
	}
}

// TestCastingOutside_MageArmorAsksForNoArmor: the spell asks for a creature that wears no
// armor (SRD 5.1).
func TestCastingOutside_MageArmorAsksForNoArmor(t *testing.T) {
	t.Parallel()
	a := newCastingParty(t)
	extra := a.h.newUser("Dani")
	a.h.join(a.master, a.campaignID, extra)
	armored := extra.castingHero(t, a.campaignID, "Ilha", classLevel("class:fighter", 2, ""),
		&rulesv1.AbilityScores{Strength: 14, Dexterity: 14, Constitution: 12, Intelligence: 10, Wisdom: 10, Charisma: 8}, nil, nil, nil, leatherArmorKey)
	_, err := a.castOut(t, a.ana, a.pens, mageArmorKey, slotOfLevel(1), false, []*charactersv1.Character{armored})
	wantCastingBlocked(t, "Mage Armor on a creature in armor", err, playv1.CastingBlockedReason_CASTING_BLOCKED_REASON_TARGET_WEARS_ARMOR)
	if got := usedSlots(a.vitals(t, a.pens), 1); got != 0 {
		t.Errorf("slots used = %d: a refused cast spends none", got)
	}
}

// TestCastingOutside_ARestEndsWhatItsLengthCovers: a long rest (8 hours) ends spells of 8
// hours or less and not longer ones; a short rest only those of an hour or less.
func TestCastingOutside_ARestEndsWhatItsLengthCovers(t *testing.T) {
	t.Parallel()
	a := newCastingParty(t)
	mage := a.mustCastOut(t, a.ana, a.pens, mageArmorKey, slotOfLevel(1), false, []*charactersv1.Character{a.toren}).GetCast()
	bless := a.mustCastOut(t, a.bia, a.bri, blessKey, slotOfLevel(1), false, []*charactersv1.Character{a.toren}).GetCast()
	rest := func(minutes int) []string {
		var ended []string
		m := authz.Membership{CampaignID: a.campaignID, UserID: a.master.id, Role: authz.RoleMaster}
		_, err := a.h.svc.write(t.Context(), combatWrite{m: m, key: newKey(), kind: eventSpellCastEnded}, func(c *combatTx) (any, error) {
			var err error
			ended, _, err = a.h.svc.endSpellsAtRest(t.Context(), c, minutes)
			return nil, err
		})
		if err != nil {
			t.Fatalf("endSpellsAtRest(%d) error = %v", minutes, err)
		}
		return ended
	}
	if got := rest(60); len(got) != 1 || got[0] != bless.GetId() {
		t.Errorf("a short rest ended %v, want only Bless (1 minute)", got)
	}
	if got := rest(480); len(got) != 1 || got[0] != mage.GetId() {
		t.Errorf("a long rest ended %v, want Mage Armor (8 hours)", got)
	}
	if c := a.castByID(t, a.master, mage.GetId()); c.GetEndReason() != playv1.OutsideCastEnd_OUTSIDE_CAST_END_REST {
		t.Errorf("Mage Armor = %v, want ended by the rest", c)
	}
}

// TestRN10_CastingOutsideHidesWhatThePlayersMaySee: a player reads the casts of the party
// and of the NPCs on the stage; an NPC that is not on the stage casts for the master alone,
// is never a target a player can pick, and what a target regained is its own player's.
func TestRN10_CastingOutsideHidesWhatThePlayersMaySee(t *testing.T) {
	t.Parallel()
	a := newCastingParty(t)
	sheet := &charactersv1.CharacterSheet{Content: &charactersv1.CharacterSheet_Full{Full: &charactersv1.FullSheet{
		BaseScores: &rulesv1.AbilityScores{Strength: 10, Dexterity: 12, Constitution: 12, Intelligence: 16, Wisdom: 10, Charisma: 10}, RaceKey: "race:human",
		Classes: []*charactersv1.ClassLevel{classLevel("class:wizard", 5, "")}, KnownSpellKeys: []string{mageArmorKey, magicMissileSpell}, PreparedSpellKeys: []string{mageArmorKey, magicMissileSpell},
	}}}
	res, err := a.master.characters.CreateCharacter(t.Context(), connect.NewRequest(&charactersv1.CreateCharacterRequest{
		CampaignId: a.campaignID, Kind: charactersv1.CharacterKind_CHARACTER_KIND_ENEMY, Name: "LEAKCANARY-npc-1", Sheet: sheet,
	}))
	if err != nil {
		t.Fatalf("CreateCharacter(NPC) error = %v", err)
	}
	mage := res.Msg.GetCharacter()

	// A player cannot cast for an NPC, nor pick one the master has not shown: both are not found.
	_, err = a.castOut(t, a.ana, mage, mageArmorKey, slotOfLevel(1), false, []*charactersv1.Character{a.toren})
	wantCode(t, "a player casting for an NPC", err, connect.CodeNotFound)
	_, err = a.castOut(t, a.ana, a.pens, magicMissileSpell, slotOfLevel(1), false, []*charactersv1.Character{mage})
	wantCode(t, "a player aiming at a hidden NPC", err, connect.CodeNotFound)
	_, err = a.casting(t, a.ana, mage)
	wantCode(t, "options of an NPC as a player", err, connect.CodeNotFound)

	// The master casts for the hidden NPC: the players read nothing, the master reads it.
	hidden := a.mustCastOut(t, a.master, mage, mageArmorKey, slotOfLevel(1), false, []*charactersv1.Character{a.toren}).GetCast()
	if !hidden.GetSecret() || !hidden.GetCasterIsNpc() {
		t.Fatalf("cast = %v, want a secret cast by an NPC", hidden)
	}
	for _, p := range []*user{a.ana, a.bia, a.caio} {
		all := a.casts(t, p)
		for _, c := range append(all.GetActive(), all.GetLog()...) {
			if c.GetId() == hidden.GetId() || c.GetCasterName() == "LEAKCANARY-npc-1" {
				t.Errorf("player %s reads the hidden NPC's cast: %v", p.id, c)
			}
		}
		_, err := p.casting.AbandonCast(t.Context(), connect.NewRequest(&playv1.AbandonCastRequest{CampaignId: a.campaignID, CastId: hidden.GetId(), IdempotencyKey: newKey()}))
		wantCode(t, "InterruptCast of a hidden cast", err, connect.CodeNotFound)
	}
	if c := a.castByID(t, a.master, hidden.GetId()); c == nil || c.GetCasterName() != "LEAKCANARY-npc-1" {
		t.Errorf("the master's read of the hidden cast = %v (the positive control)", c)
	}

	// Shown on the stage, the NPC's casts are the table's, and it is a target the players may pick.
	a.onStage(t, mage)
	shown := a.mustCastOut(t, a.master, mage, magicMissileSpell, slotOfLevel(1), false, []*charactersv1.Character{a.toren}).GetCast()
	if shown.GetSecret() {
		t.Errorf("cast by an NPC on the stage = %v, want a public one", shown)
	}
	if c := a.castByID(t, a.ana, shown.GetId()); c == nil || c.GetCasterName() != "LEAKCANARY-npc-1" {
		t.Errorf("a player reads %v of the cast by an NPC on the stage, want it", c)
	}
	// A player names an NPC by its place on the stage, never by its character ID (RN-20).
	var place string
	if err := a.h.pool.QueryRow(t.Context(), `SELECT id FROM stage_npcs WHERE character_id = $1`, mage.GetId()).Scan(&place); err != nil {
		t.Fatal(err)
	}
	_, err = a.castOut(t, a.ana, a.pens, magicMissileSpell, slotOfLevel(1), false, []*charactersv1.Character{mage})
	wantCode(t, "aiming at an NPC by its character ID", err, connect.CodeNotFound)
	res2, err := a.castOut(t, a.ana, a.pens, magicMissileSpell, slotOfLevel(1), false, nil, func(r *playv1.CastSpellOutsideCombatRequest) { r.TargetIds = []string{place} })
	if err != nil {
		t.Fatalf("aiming at an NPC on the stage error = %v", err)
	}
	if got := res2.GetCast().GetTargets()[0]; got.GetCharacterId() != place || got.GetName() != "LEAKCANARY-npc-1" {
		t.Errorf("target = %v, want the NPC by its place on the stage", got)
	}
	if got := a.castByID(t, a.ana, shown.GetId()).GetCasterId(); got != place {
		t.Errorf("a player reads the NPC caster as %q, want its place on the stage %q", got, place)
	}
	if got := a.castByID(t, a.master, shown.GetId()).GetCasterId(); got != mage.GetId() {
		t.Errorf("the master reads the NPC caster as %q, want its character ID", got)
	}
	// What Toren regained is for Toren's player and the master only.
	a.hurt(t, a.toren, 3)
	healed := a.mustCastOut(t, a.bia, a.bri, cureWounds, slotOfLevel(1), false, []*charactersv1.Character{a.toren}).GetCast()
	if tg := a.castByID(t, a.ana, healed.GetId()).GetTargets()[0]; tg.GetAmount() != 0 || tg.GetHitPointsAfter() != 0 {
		t.Errorf("another player reads %v, want the numbers left out", tg)
	}
	if tg := a.castByID(t, a.caio, healed.GetId()).GetTargets()[0]; tg.GetAmount() == 0 {
		t.Errorf("the target's player reads %v, want what he regained", tg)
	}
}

func (a *armed) casting(t *testing.T, u *user, c *charactersv1.Character) (*playv1.GetCastOptionsResponse, error) {
	t.Helper()
	res, err := u.casting.GetCastOptions(t.Context(), connect.NewRequest(&playv1.GetCastOptionsRequest{CampaignId: a.campaignID, CharacterId: c.GetId()}))
	if err != nil {
		return nil, err
	}
	return res.Msg, nil
}

// TestCastingOutside_TheRequestIsIdempotentAndHonoursTheDiceMode: the same key again
// repeats the answer and spends nothing more; the same key for another cast, or by another
// person, is refused; the campaign's dice setting binds a player (RN-18).
func TestCastingOutside_TheRequestIsIdempotentAndHonoursTheDiceMode(t *testing.T) {
	t.Parallel()
	a := newCastingParty(t)
	a.hurt(t, a.toren, 4)
	tor := []*charactersv1.Character{a.toren}
	key := newKey()
	set := func(r *playv1.CastSpellOutsideCombatRequest) { r.IdempotencyKey = key }
	first := a.mustCastOut(t, a.bia, a.bri, cureWounds, slotOfLevel(1), false, tor, set)
	again := a.mustCastOut(t, a.bia, a.bri, cureWounds, slotOfLevel(1), false, tor, set)
	if first.GetCast().GetId() != again.GetCast().GetId() || usedSlots(a.vitals(t, a.bri), 1) != 1 || a.eventCount(t, "spell_cast_outside") != 1 {
		t.Errorf("a retry made a second cast: %v / %v, slots used %d", first.GetCast().GetId(), again.GetCast().GetId(), usedSlots(a.vitals(t, a.bri), 1))
	}
	_, err := a.castOut(t, a.bia, a.bri, healingWord, slotOfLevel(1), false, tor, set)
	wantCode(t, "the same key for another cast", err, connect.CodeInvalidArgument)
	_, err = a.castOut(t, a.master, a.bri, cureWounds, slotOfLevel(1), false, tor, set)
	wantCode(t, "the same key by another person", err, connect.CodeInvalidArgument)

	// Physical dice: the sum is typed, and the app's roll is refused.
	a.setPhysical(t, a.bia)
	a.forceDice(t, campaignsv1.DiceMode_DICE_MODE_PHYSICAL)
	_, err = a.castOut(t, a.bia, a.bri, cureWounds, slotOfLevel(1), false, tor)
	wantBlockedBy(t, "roll_in_app under physical dice", err, playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_WRONG_DICE_MODE)
	_, err = a.castOut(t, a.bia, a.bri, cureWounds, slotOfLevel(1), false, tor, func(r *playv1.CastSpellOutsideCombatRequest) {
		r.Roll = &playv1.CastSpellOutsideCombatRequest_TypedSum{TypedSum: 9}
	})
	wantCode(t, "a sum above the dice", err, connect.CodeInvalidArgument)
	res := a.mustCastOut(t, a.bia, a.bri, cureWounds, slotOfLevel(1), false, tor, func(r *playv1.CastSpellOutsideCombatRequest) {
		r.Roll = &playv1.CastSpellOutsideCombatRequest_TypedSum{TypedSum: 5}
	})
	if !res.GetCast().GetPhysical() || res.GetCast().GetRollTotal() != 8 {
		t.Errorf("cast = %v, want the typed 5 on a physical die, 5 + WIS 3 = 8", res.GetCast())
	}
	_, err = a.castOut(t, a.bia, a.bri, cureWounds, slotOfLevel(1), false, tor, func(r *playv1.CastSpellOutsideCombatRequest) { r.Roll = nil })
	wantCode(t, "no roll for a spell that rolls dice", err, connect.CodeInvalidArgument)
}

// TestCastingOutside_TheSlotAndTheSpellAreTheSheets: a cantrip takes no slot, a spell not on
// the sheet is refused, a player casts only for their own character, and the options say what
// is castable.
func TestCastingOutside_TheSlotAndTheSpellAreTheSheets(t *testing.T) {
	t.Parallel()
	a := newCastingParty(t)
	_, err := a.castOut(t, a.ana, a.pens, fireBolt, slotOfLevel(1), false, nil)
	wantCode(t, "a cantrip with a slot", err, connect.CodeInvalidArgument)
	_, err = a.castOut(t, a.ana, a.pens, cureWounds, slotOfLevel(1), false, []*charactersv1.Character{a.toren})
	wantCode(t, "a spell not on the sheet", err, connect.CodeInvalidArgument)
	_, err = a.castOut(t, a.ana, a.pens, detectMagicKey, slotOfLevel(1), false, nil) // in the book, not prepared
	wantCode(t, "a spell that is not prepared", err, connect.CodeInvalidArgument)
	_, err = a.castOut(t, a.bia, a.bri, cureWounds, slotOfLevel(1), false, nil)
	wantCode(t, "a healing spell with no target", err, connect.CodeInvalidArgument)
	_, err = a.castOut(t, a.ana, a.pens, mageArmorKey, slotOfLevel(5), false, []*charactersv1.Character{a.toren})
	wantCode(t, "a slot the wizard does not have", err, connect.CodeInvalidArgument)
	_, err = a.castOut(t, a.caio, a.pens, mageArmorKey, slotOfLevel(1), false, []*charactersv1.Character{a.toren})
	wantCode(t, "another player's character", err, connect.CodePermissionDenied)
	res := a.mustCastOut(t, a.ana, a.pens, fireBolt, nil, false, nil)
	if res.GetCast().GetSlotLevel() != 0 || res.GetCast().GetStatus() != playv1.OutsideCastStatus_OUTSIDE_CAST_STATUS_ENDED {
		t.Errorf("Fire Bolt = %v, want a cast with no slot", res.GetCast())
	}

	o, err := a.casting(t, a.ana, a.pens)
	if err != nil {
		t.Fatalf("GetCastOptions() error = %v", err)
	}
	by := map[string]*playv1.CastingSpell{}
	for _, s := range o.GetSpells() {
		by[s.GetSpell().GetKey()] = s
	}
	if s := by[alarmKey]; s == nil || s.GetCanCast() || !s.GetRitualAllowed() || s.GetRitualMinutes() != 11 || s.GetCastingMinutes() != 1 {
		t.Errorf("Alarm = %v, want a ritual of 11 minutes only (1 minute + 10, not prepared)", s)
	}
	if s := by[mageArmorKey]; s == nil || !s.GetCanCast() || s.GetEffect() != playv1.CastingEffectKind_CASTING_EFFECT_KIND_ARMOR_CLASS || s.GetDurationSeconds() != 8*3600 {
		t.Errorf("Mage Armor = %v, want castable, armor class, 8 hours", s)
	}
	if s := by[detectMagicKey]; s == nil || s.GetCanCast() || !s.GetRitualAllowed() {
		t.Errorf("Detect Magic = %v, want a ritual from the book", s)
	}
	_, err = a.casting(t, a.caio, a.pens)
	wantCode(t, "options of another player's character", err, connect.CodePermissionDenied)
}

// TestCastingOutside_ReachIsMeasuredOnTheMapAndTheMasterJudgesWithoutOne: Cure Wounds is a
// touch spell (5 ft); on the current map a player's target farther than that is refused and
// the options say so, and without tokens nothing is refused (SRD 5.1, "Range").
func TestCastingOutside_ReachIsMeasuredOnTheMapAndTheMasterJudgesWithoutOne(t *testing.T) {
	t.Parallel()
	a := newCastingParty(t)
	a.hurt(t, a.toren, 5)
	tor := []*charactersv1.Character{a.toren}
	// No tokens: the master judges, and the options give no distance.
	o, err := a.casting(t, a.bia, a.bri)
	if err != nil {
		t.Fatal(err)
	}
	for _, tg := range o.GetTargets() {
		if tg.GetDistanceKnown() {
			t.Errorf("target %v has a distance without tokens", tg)
		}
	}
	a.mustCastOut(t, a.bia, a.bri, cureWounds, slotOfLevel(1), false, tor)

	// Brisa stands on the first column, Toren on the next one (1,5 m) and Pensantus three away.
	step := 10000 / gridColumns
	at := func(col int) int { return col*step + step/2 }
	if _, err := a.h.pool.Exec(t.Context(), `UPDATE game_sessions SET current_map_id = $2 WHERE campaign_id = $1 AND ended_at IS NULL`, a.campaignID, a.mapID); err != nil {
		t.Fatal(err)
	}
	a.h.placeToken(a.mapID, a.bri.GetId(), at(2), 5000)
	a.h.placeToken(a.mapID, a.toren.GetId(), at(3), 5000)
	a.h.placeToken(a.mapID, a.pens.GetId(), at(5), 5000)
	o, err = a.casting(t, a.bia, a.bri)
	if err != nil {
		t.Fatal(err)
	}
	dist := map[string]int32{}
	for _, tg := range o.GetTargets() {
		if tg.GetDistanceKnown() {
			dist[tg.GetName()] = tg.GetDistanceFt()
		}
	}
	if dist["Toren"] != 5 || dist["Pensantus"] != 15 {
		t.Errorf("distances = %v, want Toren 5 ft and Pensantus 15 ft", dist)
	}
	for _, s := range o.GetSpells() {
		if s.GetSpell().GetKey() != cureWounds {
			continue
		}
		for _, r := range s.GetReach() {
			want := r.GetCharacterId() != a.pens.GetId()
			if r.GetInRange() != want || (!want && r.GetReasonPt() != "Fora do alcance do toque (1,5 m).") {
				t.Errorf("reach of %s = %v, want in range %v with the touch reason", r.GetCharacterId(), r, want)
			}
		}
	}
	_, err = a.castOut(t, a.bia, a.bri, cureWounds, slotOfLevel(1), false, []*charactersv1.Character{a.pens})
	b := wantCastingBlocked(t, "a touch spell three squares away", err, playv1.CastingBlockedReason_CASTING_BLOCKED_REASON_TARGET_OUT_OF_REACH)
	if b.GetMissingFt() != 10 {
		t.Errorf("missing = %d ft, want 10", b.GetMissingFt())
	}
	a.mustCastOut(t, a.bia, a.bri, cureWounds, slotOfLevel(1), false, tor)
	// The master is never held to the reach.
	a.mustCastOut(t, a.master, a.bri, cureWounds, slotOfLevel(1), false, []*charactersv1.Character{a.pens})
}

// TestCastingOutside_DamageAsksForTheConcentrationSaveAndZeroEndsIt: the master's correction of
// the hit points of a character concentrating on a spell asks for a Constitution save, DC 10 or
// half the damage; at 0 hit points the concentration is over (SRD 5.1, "Duration").
func TestCastingOutside_DamageAsksForTheConcentrationSaveAndZeroEndsIt(t *testing.T) {
	t.Parallel()
	a := newCastingParty(t)
	bless := a.mustCastOut(t, a.bia, a.bri, blessKey, slotOfLevel(1), false, []*charactersv1.Character{a.toren}).GetCast()
	full := a.vitals(t, a.bri).GetHitPointsMax()
	adjust := func(hp int32) *playv1.AdjustCharacterVitalsResponse {
		res, err := a.master.play.AdjustCharacterVitals(t.Context(), connect.NewRequest(&playv1.AdjustCharacterVitalsRequest{
			CampaignId: a.campaignID, CharacterId: a.bri.GetId(), IdempotencyKey: newKey(), HitPointsCurrent: &hp,
		}))
		if err != nil {
			t.Fatalf("AdjustCharacterVitals() error = %v", err)
		}
		return res.Msg
	}
	if r := adjust(full - 6); r.GetConcentrationDc() != 10 || r.GetConcentrationCastId() != bless.GetId() {
		t.Errorf("6 damage = %v, want DC 10 on Bless", r)
	}
	if r := adjust(full); r.GetConcentrationDc() != 0 {
		t.Errorf("healing asked for a save: %v", r)
	}
	if r := adjust(1); r.GetConcentrationDc() != 11 { // half of 23, above 10
		t.Errorf("%d damage = DC %d, want 11", full-1, r.GetConcentrationDc())
	}
	if r := adjust(0); r.GetConcentrationDc() != 0 || r.GetConcentrationCastId() != bless.GetId() {
		t.Errorf("0 hit points = %v, want the concentration over, no save", r)
	}
	if c := a.castByID(t, a.master, bless.GetId()); c.GetStatus() != playv1.OutsideCastStatus_OUTSIDE_CAST_STATUS_ENDED || c.GetEndReason() != playv1.OutsideCastEnd_OUTSIDE_CAST_END_CONCENTRATION {
		t.Errorf("Bless after 0 hit points = %v, want ended by concentration", c)
	}
	// The effect Bless left on Toren went with it.
	var n int
	if err := a.h.pool.QueryRow(t.Context(), `SELECT count(*) FROM character_effects WHERE character_id = $1`, a.toren.GetId()).Scan(&n); err != nil || n != 0 {
		t.Errorf("Toren still has %d effects after the caster lost her concentration (%v)", n, err)
	}
}
