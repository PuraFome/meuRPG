package play

import (
	"errors"
	"testing"

	"connectrpc.com/connect"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
)

// Characterization of the casts outside a combat and of the casting reads, written
// before the functions behind them were split into phases: each refusal with its code
// and message, the Life Domain's healing at the levels the party fixture does not
// reach, and what each viewer reads. These tests need the database
// (MEURPG_TEST_DATABASE_URL).

// wantCastRefusal fails unless err is a Connect error with this code and exactly this message.
func wantCastRefusal(t *testing.T, call string, err error, code connect.Code, msg string) {
	t.Helper()
	if err == nil {
		t.Fatalf("%s succeeded, want %v %q", call, code, msg)
	}
	var ce *connect.Error
	if !errors.As(err, &ce) || ce.Code() != code || ce.Message() != msg {
		t.Fatalf("%s error = %v, want %v %q", call, err, code, msg)
	}
}

// newCastingHighParty is the party of newCastingParty with Brisa a level 17 cleric of the Life
// Domain (Supreme Healing, Blessed Healer and the 6th-circle Heal), for the healing cases the
// level 3 Brisa cannot reach.
func newCastingHighParty(t *testing.T) *armed {
	t.Helper()
	return newArmedWith(t, func(a *armed) {
		a.toren = a.caio.hero(t, a.campaignID, "Toren", "class:fighter", "race:human", 5,
			&rulesv1.AbilityScores{Strength: 16, Dexterity: 13, Constitution: 14, Intelligence: 10, Wisdom: 10, Charisma: 8}, []string{battleaxe}, nil)
		a.pens = a.ana.castingHero(t, a.campaignID, "Pensantus", classLevel("class:wizard", 5, ""),
			&rulesv1.AbilityScores{Strength: 10, Dexterity: 14, Constitution: 12, Intelligence: 16, Wisdom: 10, Charisma: 8}, []string{fireBolt},
			[]string{mageArmorKey, magicMissileSpell}, []string{mageArmorKey, magicMissileSpell}, "")
		a.bri = a.bia.castingHero(t, a.campaignID, "Brisa", classLevel("class:cleric", 17, "subclass:life"),
			&rulesv1.AbilityScores{Strength: 10, Dexterity: 12, Constitution: 14, Intelligence: 10, Wisdom: 20, Charisma: 10}, nil, nil,
			[]string{cureWounds, healSpell, falseLife}, "")
	})
}

// castCharEnemy creates an NPC the master may aim a cast at (it is not on the stage).
func (a *armed) castCharEnemy(t *testing.T, name string) *charactersv1.Character {
	t.Helper()
	sheet := &charactersv1.CharacterSheet{Content: &charactersv1.CharacterSheet_Full{Full: &charactersv1.FullSheet{
		BaseScores: &rulesv1.AbilityScores{Strength: 10, Dexterity: 12, Constitution: 12, Intelligence: 16, Wisdom: 10, Charisma: 10}, RaceKey: "race:human",
		Classes: []*charactersv1.ClassLevel{classLevel("class:wizard", 5, "")}, KnownSpellKeys: []string{mageArmorKey, magicMissileSpell}, PreparedSpellKeys: []string{mageArmorKey, magicMissileSpell},
	}}}
	res, err := a.master.characters.CreateCharacter(t.Context(), connect.NewRequest(&charactersv1.CreateCharacterRequest{
		CampaignId: a.campaignID, Kind: charactersv1.CharacterKind_CHARACTER_KIND_ENEMY, Name: name, Sheet: sheet,
	}))
	if err != nil {
		t.Fatalf("CreateCharacter(%s) error = %v", name, err)
	}
	return res.Msg.GetCharacter()
}

// TestCastingOutside_TheLifeDomainHealsAtHighLevels: at level 17 Supreme Healing makes the
// dice their highest number, Disciple of Life adds to each target, Blessed Healer heals the
// caster once when another creature regained hit points (and merges into the caster's own
// line when the caster is a target too), an NPC target is only narrated, and Heal restores
// a fixed number.
func TestCastingOutside_TheLifeDomainHealsAtHighLevels(t *testing.T) {
	t.Parallel()
	a := newCastingHighParty(t)
	hiero := a.bri
	goblin := a.castCharEnemy(t, "Mago Inimigo")
	a.hurt(t, a.toren, 1)
	a.hurt(t, hiero, 1)

	// Cure Wounds at the 1st level: 1d8 as its highest number (8) + WIS 5 = 13, Disciple of
	// Life +3 = 16 to Toren; Blessed Healer gives the caster 2 + 1 = 3 as its own line.
	res := a.mustCastOut(t, a.bia, hiero, cureWounds, slotOfLevel(1), false, []*charactersv1.Character{a.toren})
	cast := res.GetCast()
	if cast.GetDiceCount() != 1 || cast.GetDiceSides() != 8 || len(cast.GetFaces()) != 1 || cast.GetFaces()[0] != 8 || cast.GetRollTotal() != 13 {
		t.Errorf("roll = %dd%d %v = %d, want Supreme Healing: 1d8 [8] = 13", cast.GetDiceCount(), cast.GetDiceSides(), cast.GetFaces(), cast.GetRollTotal())
	}
	if physical := cast.GetPhysical(); physical {
		t.Errorf("a roll that was not made is physical = %v", physical)
	}
	if got := a.vitals(t, a.toren).GetHitPointsCurrent(); got != 1+16 {
		t.Errorf("Toren = %d, want %d (1 + 13 + Disciple of Life 3)", got, 1+16)
	}
	if got := a.vitals(t, hiero).GetHitPointsCurrent(); got != 1+3 {
		t.Errorf("the caster = %d, want %d (1 + Blessed Healer 3)", got, 1+3)
	}
	master := a.castByID(t, a.master, cast.GetId())
	if len(master.GetTargets()) != 2 {
		t.Fatalf("targets as the master = %v, want Toren and the caster's Blessed Healer line", master.GetTargets())
	}
	if tg := master.GetTargets()[0]; tg.GetCharacterId() != a.toren.GetId() || tg.GetAmount() != 16 || tg.GetExtraHitPoints() != 3 || tg.GetHitPointsBefore() != 1 || tg.GetHitPointsAfter() != 17 {
		t.Errorf("first target = %v, want Toren healed 16 (3 of it Disciple of Life) from 1 to 17", tg)
	}
	if tg := master.GetTargets()[1]; tg.GetCharacterId() != hiero.GetId() || tg.GetAmount() != 3 || tg.GetExtraHitPoints() != 0 || tg.GetHitPointsBefore() != 1 || tg.GetHitPointsAfter() != 4 {
		t.Errorf("second target = %v, want the caster's Blessed Healer line of 3, from 1 to 4", tg)
	}

	// The caster is also a listed target (the master is not held to the number of targets): Blessed
	// Healer merges into the caster's own line.
	a.hurt(t, hiero, 1)
	a.hurt(t, a.toren, 1)
	res = a.mustCastOut(t, a.master, hiero, cureWounds, slotOfLevel(1), false, []*charactersv1.Character{a.toren, hiero})
	got := a.castByID(t, a.master, res.GetCast().GetId()).GetTargets()
	if len(got) != 2 {
		t.Fatalf("targets = %v, want Toren and the caster in one line each", got)
	}
	if tg := got[1]; tg.GetCharacterId() != hiero.GetId() || tg.GetAmount() != 16+3 || tg.GetHitPointsAfter() != 20 || tg.GetExtraHitPoints() != 3 {
		t.Errorf("the caster's line = %v, want 16 from the spell + 3 from Blessed Healer, after 20", tg)
	}

	// An NPC among the targets is narrated and changes nothing; the master aims at it.
	res = a.mustCastOut(t, a.master, hiero, cureWounds, slotOfLevel(1), false, []*charactersv1.Character{goblin})
	if tg := res.GetCast().GetTargets()[0]; tg.GetNpc() != true || tg.GetName() != "Mago Inimigo" || tg.GetEffect() != playv1.CastEffect_CAST_EFFECT_NARRATED {
		t.Errorf("target = %v, want the NPC narrated", tg)
	}
	if !res.GetCast().GetSecret() {
		t.Errorf("a cast aimed at an NPC the players do not see = %v, want a secret one", res.GetCast())
	}

	// Heal restores a fixed 70 and rolls no dice (the roll is not asked for).
	a.hurt(t, a.toren, 2)
	res, err := a.castOut(t, a.bia, hiero, healSpell, slotOfLevel(6), false, []*charactersv1.Character{a.toren}, func(r *playv1.CastSpellOutsideCombatRequest) { r.Roll = nil })
	if err != nil {
		t.Fatalf("Heal error = %v", err)
	}
	if res.GetCast().GetDiceCount() != 0 || res.GetCast().GetRollTotal() != 70 {
		t.Errorf("Heal = %v, want a fixed 70 and no dice", res.GetCast())
	}
	if got := a.vitals(t, a.toren).GetHitPointsCurrent(); got != a.vitals(t, a.toren).GetHitPointsMax() {
		t.Errorf("Toren = %d, want the maximum", got)
	}
}

// TestCastingOutside_FalseLifeGivesTheCasterTemporaryHitPoints: a spell that reaches only
// the caster takes the caster by itself, rolls its pool (1d4 + 4), gives temporary hit
// points that do not stack, and refuses targets and a missing roll.
func TestCastingOutside_FalseLifeGivesTheCasterTemporaryHitPoints(t *testing.T) {
	t.Parallel()
	a := newCastingHighParty(t)
	hiero := a.bri

	_, err := a.castOut(t, a.bia, hiero, falseLife, slotOfLevel(1), false, []*charactersv1.Character{a.toren})
	wantCastRefusal(t, "False Life with a target", err, connect.CodeInvalidArgument, "this spell reaches the caster alone: leave the targets out")
	_, err = a.castOut(t, a.bia, hiero, falseLife, slotOfLevel(1), false, nil, func(r *playv1.CastSpellOutsideCombatRequest) { r.Roll = nil })
	wantCastRefusal(t, "False Life with no roll", err, connect.CodeInvalidArgument, "set roll_in_app or pool_sum: this spell rolls a pool of dice")

	a.h.roller.queue(3)
	cast := a.mustCastOut(t, a.bia, hiero, falseLife, slotOfLevel(1), false, nil).GetCast()
	if cast.GetRollTotal() != 7 || cast.GetDiceCount() != 1 || cast.GetDiceSides() != 4 {
		t.Errorf("roll = %dd%d = %d, want 1d4 + 4 = 7", cast.GetDiceCount(), cast.GetDiceSides(), cast.GetRollTotal())
	}
	if v := a.vitals(t, hiero); v.GetHitPointsTemporary() != 7 {
		t.Errorf("temporary hit points = %d, want 7", v.GetHitPointsTemporary())
	}
	tg := a.castByID(t, a.master, cast.GetId()).GetTargets()
	if len(tg) != 1 || tg[0].GetCharacterId() != hiero.GetId() || tg[0].GetEffect() != playv1.CastEffect_CAST_EFFECT_TEMPORARY_HIT_POINTS || tg[0].GetAmount() != 7 {
		t.Errorf("targets = %v, want the caster with 7 temporary hit points", tg)
	}
	// A smaller roll does not take the larger away: the second cast gives nothing more.
	a.h.roller.queue(1)
	again := a.mustCastOut(t, a.bia, hiero, falseLife, slotOfLevel(1), false, nil).GetCast()
	if v := a.vitals(t, hiero); v.GetHitPointsTemporary() != 7 {
		t.Errorf("temporary hit points after a lower roll = %d, want 7", v.GetHitPointsTemporary())
	}
	if tg := a.castByID(t, a.master, again.GetId()).GetTargets()[0]; tg.GetAmount() != 0 || tg.GetHitPointsBefore() != 7 || tg.GetHitPointsAfter() != 7 {
		t.Errorf("second target = %v, want nothing gained from 7 to 7", tg)
	}
}

// TestCastingOutside_TheRequestAndThePlanAreRefusedWithTheirMessages: the shape of the
// request, the spell and the slot against the sheet, and the number of targets.
func TestCastingOutside_TheRequestAndThePlanAreRefusedWithTheirMessages(t *testing.T) {
	t.Parallel()
	a := newCastingParty(t)
	tor := []*charactersv1.Character{a.toren}
	inApp := func(v bool) func(*playv1.CastSpellOutsideCombatRequest) {
		return func(r *playv1.CastSpellOutsideCombatRequest) {
			r.Roll = &playv1.CastSpellOutsideCombatRequest_RollInApp{RollInApp: v}
		}
	}
	for name, tc := range map[string]struct {
		u      *user
		caster *charactersv1.Character
		spell  string
		slot   *playv1.SpellSlot
		ritual bool
		tgts   []*charactersv1.Character
		edit   func(*playv1.CastSpellOutsideCombatRequest)
		code   connect.Code
		msg    string
	}{
		"roll_in_app false":    {a.bia, a.bri, cureWounds, slotOfLevel(1), false, tor, inApp(false), connect.CodeInvalidArgument, "roll_in_app must be true"},
		"an empty spell key":   {a.ana, a.pens, "", nil, false, nil, nil, connect.CodeInvalidArgument, "spell_key must name one of the caster's spells"},
		"a spell key too long": {a.ana, a.pens, string(make([]byte, 101)), nil, false, nil, nil, connect.CodeInvalidArgument, "spell_key must name one of the caster's spells"},
		"a malformed caster":   {a.ana, a.pens, magicMissileSpell, slotOfLevel(1), false, nil, func(r *playv1.CastSpellOutsideCombatRequest) { r.CasterCharacterId = "nope" }, connect.CodeNotFound, "character not found"},
		"a malformed target":   {a.ana, a.pens, magicMissileSpell, slotOfLevel(1), false, nil, func(r *playv1.CastSpellOutsideCombatRequest) { r.TargetIds = []string{"nope"} }, connect.CodeNotFound, "character not found"},
		"a repeated target": {a.ana, a.pens, magicMissileSpell, slotOfLevel(1), false, nil, func(r *playv1.CastSpellOutsideCombatRequest) {
			r.TargetIds = []string{a.toren.GetId(), a.toren.GetId()}
		}, connect.CodeInvalidArgument, "target_ids[1] repeats a character"},
		"a target that does not exist": {a.ana, a.pens, magicMissileSpell, slotOfLevel(1), false, nil, func(r *playv1.CastSpellOutsideCombatRequest) {
			r.TargetIds = []string{newKey()}
		}, connect.CodeNotFound, "character not found"},
		"a spell the sheet does not know": {a.ana, a.pens, "spell:nonexistent", slotOfLevel(1), false, nil, nil, connect.CodeInvalidArgument, "spell_key is not one of the caster's spells"},
		"a slot below the spell's level":  {a.bia, a.bri, aidSpell, slotOfLevel(1), false, tor, nil, connect.CodeInvalidArgument, "slot must be a spell slot of at least the spell's level"},
		"no slot for a levelled spell":    {a.bia, a.bri, cureWounds, nil, false, tor, nil, connect.CodeInvalidArgument, "slot must be a spell slot of at least the spell's level"},
		"a ritual with a slot":            {a.ana, a.pens, alarmKey, slotOfLevel(1), true, nil, nil, connect.CodeInvalidArgument, "a ritual is cast with no slot"},
		"a spell that needs a target":     {a.bia, a.bri, cureWounds, slotOfLevel(1), false, nil, nil, connect.CodeInvalidArgument, "this spell needs a target"},
		"more targets than the spell takes": {a.bia, a.bri, cureWounds, slotOfLevel(1), false, []*charactersv1.Character{a.toren, a.pens}, nil, connect.CodeInvalidArgument,
			"this spell takes at most 1 targets with this slot"},
		"a pick for a spell that is not a summon": {a.bia, a.bri, cureWounds, slotOfLevel(1), false, tor, func(r *playv1.CastSpellOutsideCombatRequest) {
			r.Summon = &playv1.SummonChoice{CreatureKeys: []string{"monster:owl"}}
		}, connect.CodeInvalidArgument, "summon is for a summoning spell, and a summoning spell needs it"},
	} {
		_, err := a.castOut(t, tc.u, tc.caster, tc.spell, tc.slot, tc.ritual, tc.tgts, func(r *playv1.CastSpellOutsideCombatRequest) {
			if tc.edit != nil {
				tc.edit(r)
			}
		})
		if tc.msg == "character not found" && connect.CodeOf(err) == connect.CodeNotFound {
			continue // errCharacterNotFound's message is not this test's to freeze
		}
		wantCastRefusal(t, name, err, tc.code, tc.msg)
	}
	// Nothing was spent or written by the refusals.
	if n := a.eventCount(t, eventSpellCastOutside); n != 0 {
		t.Errorf("spell_cast_outside events = %d after refusals, want 0", n)
	}
}

// TestCastingOutside_GuardCastDecidesWhoMayCastForWhomAndWhatTheMasterSees: the guard's
// answers for each caller and each kind of target.
func TestCastingOutside_GuardCastDecidesWhoMayCastForWhomAndWhatTheMasterSees(t *testing.T) {
	t.Parallel()
	a := newCastingParty(t)
	mage := a.castCharEnemy(t, "Mago Oculto")
	// A player may not cast for an NPC, nor for another player's character.
	_, err := a.castOut(t, a.ana, mage, magicMissileSpell, slotOfLevel(1), false, []*charactersv1.Character{a.toren})
	wantCode(t, "a player casting for an NPC", err, connect.CodeNotFound)
	_, err = a.castOut(t, a.caio, a.pens, magicMissileSpell, slotOfLevel(1), false, []*charactersv1.Character{a.toren})
	wantCastRefusal(t, "another player's character", err, connect.CodePermissionDenied, "only the character's player or the master may cast for it")
	// The master casts for any of them, and a cast for a hidden NPC is the master's alone.
	secret := a.mustCastOut(t, a.master, a.pens, magicMissileSpell, slotOfLevel(1), false, []*charactersv1.Character{mage}).GetCast()
	if !secret.GetSecret() || secret.GetCasterIsNpc() {
		t.Errorf("master's cast for Pensantus at a hidden NPC = %v, want secret, caster a player", secret)
	}
	open := a.mustCastOut(t, a.master, a.pens, magicMissileSpell, slotOfLevel(1), false, []*charactersv1.Character{a.toren}).GetCast()
	if open.GetSecret() {
		t.Errorf("master's cast at a player's character = %v, want a public one", open)
	}
	if got := len(a.casts(t, a.ana).GetLog()); got != 1 {
		t.Errorf("casts a player reads = %d, want only the public one", got)
	}
	// The NPC caster is not a character a player picks as a target, and a made-up id is not found.
	_, err = a.castOut(t, a.ana, a.pens, magicMissileSpell, slotOfLevel(1), false, []*charactersv1.Character{mage})
	wantCode(t, "a player aiming at a hidden NPC", err, connect.CodeNotFound)
}

// TestCastingView_ListSpellCastsFiltersByCharacterForEachViewer: character_id limits the
// list to the casts the character cast or was a target of; a player names an NPC by its
// place on the stage, and an unseen NPC or a made-up id is not found.
func TestCastingView_ListSpellCastsFiltersByCharacterForEachViewer(t *testing.T) {
	t.Parallel()
	a := newCastingParty(t)
	a.hurt(t, a.toren, 3)
	healed := a.mustCastOut(t, a.bia, a.bri, cureWounds, slotOfLevel(1), false, []*charactersv1.Character{a.toren}).GetCast()
	own := a.mustCastOut(t, a.ana, a.pens, fireBolt, nil, false, nil).GetCast()
	mage := a.castCharEnemy(t, "Mago da Cena")
	a.onStage(t, mage)
	staged := a.mustCastOut(t, a.master, mage, magicMissileSpell, slotOfLevel(1), false, []*charactersv1.Character{a.toren}).GetCast()

	list := func(u *user, id string) (*playv1.ListSpellCastsResponse, error) {
		res, err := u.casting.ListSpellCasts(t.Context(), connect.NewRequest(&playv1.ListSpellCastsRequest{CampaignId: a.campaignID, CharacterId: id}))
		if err != nil {
			return nil, err
		}
		return res.Msg, nil
	}
	ids := func(r *playv1.ListSpellCastsResponse) map[string]bool {
		out := map[string]bool{}
		for _, c := range r.GetLog() {
			out[c.GetId()] = true
		}
		return out
	}
	// The master filters by the character ID of anyone.
	r, err := list(a.master, a.toren.GetId())
	if err != nil {
		t.Fatalf("ListSpellCasts(Toren) as the master error = %v", err)
	}
	if got := ids(r); len(got) != 2 || !got[healed.GetId()] || !got[staged.GetId()] {
		t.Errorf("casts about Toren = %v, want the heal and the NPC's missile", got)
	}
	r, _ = list(a.master, mage.GetId())
	if got := ids(r); len(got) != 1 || !got[staged.GetId()] {
		t.Errorf("casts about the NPC for the master = %v, want its own cast", got)
	}
	// A player filters by a player character, or by the NPC's place on the stage.
	r, err = list(a.ana, a.pens.GetId())
	if err != nil {
		t.Fatalf("ListSpellCasts(Pensantus) as a player error = %v", err)
	}
	if got := ids(r); len(got) != 1 || !got[own.GetId()] {
		t.Errorf("casts about Pensantus = %v, want its Fire Bolt", got)
	}
	var place string
	if err := a.h.pool.QueryRow(t.Context(), `SELECT id FROM stage_npcs WHERE character_id = $1`, mage.GetId()).Scan(&place); err != nil {
		t.Fatal(err)
	}
	r, err = list(a.ana, place)
	if err != nil {
		t.Fatalf("ListSpellCasts(place) as a player error = %v", err)
	}
	if got := ids(r); len(got) != 1 || !got[staged.GetId()] {
		t.Errorf("casts about the NPC by its place = %v, want its cast", got)
	}
	// The NPC's own character ID, an unseen NPC and a made-up id are not found for a player.
	hidden := a.castCharEnemy(t, "Mago Escondido")
	for name, id := range map[string]string{"the staged NPC's character id": mage.GetId(), "a hidden NPC": hidden.GetId(), "a made-up id": newKey(), "a malformed id": "nope"} {
		_, err := list(a.ana, id)
		wantCode(t, name, err, connect.CodeNotFound)
	}
	// What a player reads of a cast that is not about their character, by viewer.
	for _, tc := range []struct {
		u    *user
		id   string
		want string
	}{{a.caio, a.toren.GetId(), "Toren"}, {a.ana, a.toren.GetId(), "Toren"}} {
		if _, err := list(tc.u, tc.id); err != nil {
			t.Errorf("ListSpellCasts(%s) as %s error = %v", tc.want, tc.u.id, err)
		}
	}
}

// TestCastingView_GetCastOptionsAnswersEachViewerItsOwn: the options of a character for
// its player and the master, the targets a caller may pick (an NPC on the stage by its
// place for a player, by its character ID for the master), the cast being made and the
// concentration going on, and the refusals.
func TestCastingView_GetCastOptionsAnswersEachViewerItsOwn(t *testing.T) {
	t.Parallel()
	a := newCastingParty(t)
	mage := a.castCharEnemy(t, "Mago da Cena")
	a.onStage(t, mage)
	var place string
	if err := a.h.pool.QueryRow(t.Context(), `SELECT id FROM stage_npcs WHERE character_id = $1`, mage.GetId()).Scan(&place); err != nil {
		t.Fatal(err)
	}
	casting := a.mustCastOut(t, a.ana, a.pens, alarmKey, nil, true, nil).GetCast()
	conc := a.mustCastOut(t, a.bia, a.bri, blessKey, slotOfLevel(1), false, []*charactersv1.Character{a.toren}).GetCast()

	byTarget := func(o *playv1.GetCastOptionsResponse) map[string]*playv1.CastingTarget {
		out := map[string]*playv1.CastingTarget{}
		for _, tg := range o.GetTargets() {
			out[tg.GetCharacterId()] = tg
		}
		return out
	}
	o, err := a.casting(t, a.ana, a.pens)
	if err != nil {
		t.Fatalf("GetCastOptions() as the player error = %v", err)
	}
	if o.GetInCombat() || o.GetWildShape() {
		t.Errorf("options = in combat %v, wild shape %v, want neither", o.GetInCombat(), o.GetWildShape())
	}
	if o.GetCasting().GetId() != casting.GetId() || o.GetConcentrating() != nil {
		t.Errorf("casting = %v, concentrating = %v, want the ritual being cast and no concentration", o.GetCasting().GetId(), o.GetConcentrating())
	}
	pt := byTarget(o)
	if tg := pt[place]; tg == nil || !tg.GetNpc() || tg.GetName() != "Mago da Cena" {
		t.Errorf("targets as a player = %v, want the NPC by its place %s", pt, place)
	}
	if pt[mage.GetId()] != nil {
		t.Errorf("a player reads the NPC's character ID among the targets: %v", pt)
	}
	for _, c := range []*charactersv1.Character{a.toren, a.pens, a.bri} {
		if pt[c.GetId()] == nil || pt[c.GetId()].GetNpc() {
			t.Errorf("targets as a player = %v, want the party's %s", pt, c.GetName())
		}
	}
	mt := byTarget(mustOptions(t, a, a.master, a.pens))
	if tg := mt[mage.GetId()]; tg == nil || mt[place] != nil {
		t.Errorf("targets as the master = %v, want the NPC by its character ID", mt)
	}
	// Brisa concentrates on Bless; the master reads the same options for her.
	ob := mustOptions(t, a, a.master, a.bri)
	if ob.GetConcentrating().GetId() != conc.GetId() || ob.GetCasting() != nil {
		t.Errorf("Brisa's options = casting %v, concentrating %v, want Bless", ob.GetCasting().GetId(), ob.GetConcentrating().GetId())
	}
	// The refusals: a made-up id, another player's character, an NPC for a player.
	for name, tc := range map[string]struct {
		u    *user
		id   string
		want connect.Code
	}{
		"a malformed id":          {a.ana, "nope", connect.CodeNotFound},
		"a made-up id":            {a.ana, newKey(), connect.CodeNotFound},
		"another player's sheet":  {a.caio, a.pens.GetId(), connect.CodePermissionDenied},
		"an NPC as a player":      {a.ana, mage.GetId(), connect.CodeNotFound},
		"a stranger to the party": {a.h.newUser("Eva"), a.pens.GetId(), connect.CodeNotFound},
	} {
		_, err := tc.u.casting.GetCastOptions(t.Context(), connect.NewRequest(&playv1.GetCastOptionsRequest{CampaignId: a.campaignID, CharacterId: tc.id}))
		wantCode(t, name, err, tc.want)
	}
}

func mustOptions(t *testing.T, a *armed, u *user, c *charactersv1.Character) *playv1.GetCastOptionsResponse {
	t.Helper()
	o, err := a.casting(t, u, c)
	if err != nil {
		t.Fatalf("GetCastOptions(%s) error = %v", c.GetName(), err)
	}
	return o
}

// TestCastingView_ReadsNeedAnOpenSession: with the session ended, the lists and the
// options answer that there is none.
func TestCastingView_ReadsNeedAnOpenSession(t *testing.T) {
	t.Parallel()
	a := newCastingParty(t)
	a.execSQL(t, `UPDATE game_sessions SET ended_at = now() WHERE campaign_id = $1 AND ended_at IS NULL`, a.campaignID)
	_, err := a.master.casting.ListSpellCasts(t.Context(), connect.NewRequest(&playv1.ListSpellCastsRequest{CampaignId: a.campaignID}))
	wantCode(t, "ListSpellCasts with no open session", err, connect.CodeFailedPrecondition)
	_, err = a.casting(t, a.master, a.pens)
	wantCode(t, "GetCastOptions with no open session", err, connect.CodeFailedPrecondition)
}
