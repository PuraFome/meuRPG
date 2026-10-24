package play

import (
	"slices"
	"testing"

	"connectrpc.com/connect"

	campaignsv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/campaigns/v1"
	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	mapsv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/maps/v1"
	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
)

// The table rule "Os jogadores veem a CD dos testes de resistência" (RN-24, RN-10): by default only the
// master reads the DC of a saving throw, and the caster's player the DC of their own spell. With the rule
// on, the player who rolls the save reads its DC as well, for every save the app asks of them: a zone, a
// lasting effect at the start or the end of a turn, a spell cast by an NPC or by another character, a
// trap. Nobody who does not roll gains anything. These tests are the rule's, place by place; the leak
// test (internal/leaktest) holds the needle that follows the DC through every answer.

// saveDC is the DC the effect tests give the effect: no other number of these answers is 23.
const saveDC = 23

func showSaveDC(r *campaignsv1.TableRules) { r.ShowSaveDc = true }

// holdPersonOnToren is the fixture of the effect tests: Pensantus's Hold Person (DC 23) on Toren, Toren's
// turn about to end, so his player is asked the end-of-turn save. The caster is Pensantus (Ana's).
func holdPersonOnToren(t *testing.T, on bool) (*armed, *playv1.Encounter, *playv1.ReactionWindow) {
	t.Helper()
	a := newCasters(t)
	if on {
		a.setRules(t, showSaveDC)
	}
	e := a.closeFight(t)
	a.mustAddEffect(t, e, holdPerson, []string{"Toren"}, func(r *playv1.AddLastingEffectRequest) {
		a.rounds(t, 10, "Toren")(r)
		r.CasterId, r.SaveDc = a.id(t, "Pensantus"), new(int32(saveDC))
		r.PlayerVisible, r.Audience = new(true), playv1.EffectAudience_EFFECT_AUDIENCE_ALL // the card is on every sheet
	})
	a.passTo(t, e, "Toren")
	if _, err := a.endTurnRaw(t, a.master, e, true); err != nil {
		t.Fatalf("EndTurn() error = %v", err)
	}
	w := a.windowOf(t, a.caio, playv1.ReactionKind_REACTION_KIND_EFFECT_SAVE)
	if w == nil {
		t.Fatalf("Toren's windows = %v, want the save of Hold Person", a.windows(t, a.caio))
	}
	return a, e, w
}

// dcOf reads an optional DC as a number: 0 when the answer carries none.
func dcOf(p *int32) int32 {
	if p == nil {
		return 0
	}
	return *p
}

// effectLogDCs are the DCs the effect lines of the source in the log of u carry.
func effectLogDCs(l *playv1.ListCombatLogResponse, source string) []int32 {
	var out []int32
	for _, r := range l.GetRounds() {
		for _, en := range r.GetEntries() {
			if en.GetKind() == playv1.CombatLogKind_COMBAT_LOG_KIND_EFFECT && en.GetEffect().GetSourceKey() == source && en.GetEffect().Dc != nil {
				out = append(out, en.GetEffect().GetDc())
			}
		}
	}
	return out
}

// An effect's end-of-turn save, rule off: the roller reads no DC, the master does.
func TestSaveDCOffTheRollerOfAnEffectSaveReadsNoDC(t *testing.T) {
	t.Parallel()
	a, e, w := holdPersonOnToren(t, false)
	if got := w.GetEffectSave().Dc; got != nil {
		t.Errorf("Toren's player reads the DC %d of the effect save, want none: the rule is off", *got)
	}
	if got := dcOf(a.windowOf(t, a.master, playv1.ReactionKind_REACTION_KIND_EFFECT_SAVE).GetEffectSave().Dc); got != saveDC {
		t.Errorf("the master reads the DC %d, want %d", got, saveDC)
	}
	res, err := a.rollEffectSave(t, a.caio, e, w.GetId(), effectSaveFace(5))
	if err != nil {
		t.Fatalf("RollEffectSave() error = %v", err)
	}
	if res.GetResult().Dc != nil {
		t.Errorf("the roller's result carries the DC %d, want none", *res.GetResult().Dc)
	}
	if got := effectLogDCs(a.log(t, a.caio, e), holdPerson); len(got) != 0 {
		t.Errorf("Toren's player reads the DCs %v in the log, want none", got)
	}
}

// Rule on: the roller reads the DC in the question, the card of the effect on them, the answer and the log line; the master
// as before; a player who does not roll reads none of it.
func TestSaveDCOnTheRollerOfAnEffectSaveReadsTheDC(t *testing.T) {
	t.Parallel()
	a, e, w := holdPersonOnToren(t, true)
	if got := dcOf(w.GetEffectSave().Dc); got != saveDC {
		t.Errorf("Toren's player reads the DC %d of the effect save, want %d", got, saveDC)
	}
	if got := dcOf(a.windowOf(t, a.master, playv1.ReactionKind_REACTION_KIND_EFFECT_SAVE).GetEffectSave().Dc); got != saveDC {
		t.Errorf("the master reads the DC %d, want %d", got, saveDC)
	}
	if card := cardOf(t, byLabel(t, a.get(t, a.caio), "Toren"), holdPerson); card == nil || dcOf(card.GetEndSave().Dc) != saveDC {
		t.Errorf("the roller reads the card %v, want the DC %d", card, saveDC)
	}
	res, err := a.rollEffectSave(t, a.caio, e, w.GetId(), effectSaveFace(5))
	if err != nil {
		t.Fatalf("RollEffectSave() error = %v", err)
	}
	if got := dcOf(res.GetResult().Dc); got != saveDC {
		t.Errorf("the roller's result carries the DC %d, want %d", got, saveDC)
	}
	if got := effectLogDCs(a.log(t, a.caio, e), holdPerson); !slices.Equal(got, []int32{saveDC}) {
		t.Errorf("Toren's player reads the DCs %v in the log, want [%d]", got, saveDC)
	}
	// Brisa's player rolls nothing: no question, no DC on any card, no DC in the log.
	if ws := a.windows(t, a.bia); len(ws) != 0 {
		t.Errorf("a bystander reads the windows %v", ws)
	}
	enc := a.get(t, a.bia)
	for _, c := range enc.GetCombatants() {
		for _, f := range c.GetEffects() {
			if f.GetEndSave().Dc != nil || f.GetStartSave().Dc != nil {
				t.Errorf("a bystander reads the DC on the card %v of %s", f, c.GetLabel())
			}
		}
	}
	if got := effectLogDCs(a.log(t, a.bia, e), holdPerson); len(got) != 0 {
		t.Errorf("a bystander reads the DCs %v in the log", got)
	}
}

// The start-of-turn save of an effect (Web): the same, for the other phase.
func TestSaveDCOnTheStartOfTurnSaveOfAnEffect(t *testing.T) {
	t.Parallel()
	for _, on := range []bool{false, true} {
		a := newCasters(t)
		if on {
			a.setRules(t, showSaveDC)
		}
		e := a.closeFight(t)
		a.mustAddEffect(t, e, webSpell, []string{"Toren"}, func(r *playv1.AddLastingEffectRequest) {
			a.rounds(t, 10, "Pensantus")(r)
			r.CasterId, r.SaveDc = a.id(t, "Pensantus"), new(int32(saveDC))
		})
		for range 8 { // a whole round: the helper that passes turns skips the saves
			if _, err := a.endTurnRaw(t, a.master, e, true); err != nil {
				t.Fatalf("EndTurn() error = %v", err)
			}
			if a.get(t, a.master).GetCurrentCombatantId() == a.id(t, "Toren") {
				break
			}
		}
		w := a.windowOf(t, a.caio, playv1.ReactionKind_REACTION_KIND_EFFECT_SAVE)
		if w == nil || w.GetEffectSave().GetPhase() != playv1.EffectPhase_EFFECT_PHASE_START {
			t.Fatalf("rule %v: Toren's windows = %v, want the start-of-turn save of Web", on, a.windows(t, a.caio))
		}
		want := int32(0)
		if on {
			want = saveDC
		}
		if got := dcOf(w.GetEffectSave().Dc); got != want {
			t.Errorf("rule %v: Toren's player reads the DC %d, want %d", on, got, want)
		}
		if got := dcOf(a.windowOf(t, a.master, playv1.ReactionKind_REACTION_KIND_EFFECT_SAVE).GetEffectSave().Dc); got != saveDC {
			t.Errorf("rule %v: the master reads the DC %d, want %d", on, got, saveDC)
		}
	}
}

// zoneDC is the DC of the zone as the master reads it.
func zoneDC(t *testing.T, c *cave, spell string) int32 {
	t.Helper()
	return dcOf(c.oneZone(t, c.master, spell).SaveDc)
}

// A zone's save, a player's spell: Pensantus's Cloudkill catches Toren at the start of his turn.
func TestSaveDCTheZoneSaveOfAPlayersSpell(t *testing.T) {
	t.Parallel()
	for _, on := range []bool{false, true} {
		c := zoneCave(t)
		if on {
			c.setRules(t, showSaveDC)
		}
		e := c.zoneFight(t)
		c.mustMove(t, c.master, "Toren", 16, 7)
		c.passTo(t, e, "Pensantus")
		c.mustCastArea(t, c.ana, "Pensantus", cloudkillKey, slotOfLevel(5), at(15, 7), nil)
		c.passRaw(t, "Toren")
		w := c.windowOf(t, c.caio, playv1.ReactionKind_REACTION_KIND_EFFECT_SAVE)
		if w == nil {
			t.Fatalf("rule %v: Toren's player has no question", on)
		}
		want, dc := int32(0), zoneDC(t, c, cloudkillKey)
		if on {
			want = dc
		}
		if dc == 0 {
			t.Fatal("the master reads no DC on the zone")
		}
		if got := dcOf(w.GetEffectSave().Dc); got != want {
			t.Errorf("rule %v: Toren's player reads the zone DC %d, want %d", on, got, want)
		}
		if got := dcOf(c.windowOf(t, c.master, playv1.ReactionKind_REACTION_KIND_EFFECT_SAVE).GetEffectSave().Dc); got != dc {
			t.Errorf("rule %v: the master reads the zone DC %d, want %d", on, got, dc)
		}
		if ws := c.windows(t, c.bia); len(ws) != 0 {
			t.Errorf("rule %v: a bystander reads the windows %v", on, ws)
		}
		if z := c.oneZone(t, c.bia, cloudkillKey); z.SaveDc != nil {
			t.Errorf("rule %v: a bystander reads the zone DC %d", on, *z.SaveDc)
		}
	}
}

// A zone of an NPC's spell: the master plays the druid, and Brisa's player, whose character stands in the beam and rolls the save,
// reads the DC only with the rule on.
func TestSaveDCTheZoneSaveOfAnNPCsSpell(t *testing.T) {
	t.Parallel()
	for _, on := range []bool{false, true} {
		c := newCaveWith(t, 5, []string{thunderwave})
		if on {
			c.setRules(t, showSaveDC)
		}
		e := c.druidFight(t, "Druida")
		c.mustCastArea(t, c.master, "Druida", moonbeamKey, slotOfLevel(2), at(3, 9), nil)
		dc := zoneDC(t, c, moonbeamKey)
		var w *playv1.ReactionWindow
		for range 6 { // the turns pass until the beam asks Brisa's save at the start of her turn
			if w = c.windowOf(t, c.bia, playv1.ReactionKind_REACTION_KIND_EFFECT_SAVE); w != nil {
				break
			}
			if _, err := c.endTurnRaw(t, c.master, e, true); err != nil {
				t.Fatalf("EndTurn() error = %v", err)
			}
		}
		if w == nil {
			t.Fatalf("rule %v: Brisa's player has no question: the beam is over her", on)
		}
		want := int32(0)
		if on {
			want = dc
		}
		if got := dcOf(w.GetEffectSave().Dc); got != want || dc == 0 {
			t.Errorf("rule %v: Brisa's player reads the DC %d of an NPC's zone, want %d (the master's is %d)", on, got, want, dc)
		}
		for who, u := range map[string]*user{"Pensantus's player": c.ana, "Toren's player": c.caio} {
			if z := zonesOf(c.get(t, u), moonbeamKey); len(z) != 0 && z[0].SaveDc != nil {
				t.Errorf("rule %v: %s reads the DC %d of the zone", on, who, *z[0].SaveDc)
			}
			if ws := c.windows(t, u); len(ws) != 0 {
				t.Errorf("rule %v: %s reads the windows %v", on, who, ws)
			}
		}
	}
}

// castEntryDCs are the DCs the SPELL_CAST lines of the log of u carry on the save of the target.
func castEntryDCs(l *playv1.ListCombatLogResponse, targetID string) []int32 {
	var out []int32
	for _, r := range l.GetRounds() {
		for _, en := range r.GetEntries() {
			if en.GetKind() != playv1.CombatLogKind_COMBAT_LOG_KIND_SPELL_CAST {
				continue
			}
			for _, tg := range en.GetSpell().GetTargets() {
				if tg.GetTargetId() == targetID && tg.GetSave() != nil {
					out = append(out, tg.GetSave().GetDc())
				}
			}
		}
	}
	return out
}

// A spell cast by another character: Pensantus's Hold Person on Toren. His player rolls nothing (the app rolls the save), but
// reads the roll, and with the rule on the DC; the caster's player reads it either way; Brisa's never.
func TestSaveDCASpellCastByAnotherCharacter(t *testing.T) {
	t.Parallel()
	for _, on := range []bool{false, true} {
		a := newCasters(t)
		if on {
			a.setRules(t, showSaveDC)
		}
		e := a.castersFight(t, 1)
		a.h.roller.queue(1) // Toren's d20: the save fails
		a.mustCast(t, a.ana, e, "Pensantus", holdPerson, slotOfLevel(2), a.at(t, "Toren"), noCastRoll)
		toren := a.id(t, "Toren")
		want := int32(0)
		if on {
			want = 14 // 8 + 2 + 4: Pensantus is a gnome with Intelligence 16, level 3
		}
		if got := castEntryDCs(a.log(t, a.caio, e), toren); !slices.Equal(got, []int32{want}) {
			t.Errorf("rule %v: Toren's player reads the DCs %v, want [%d]", on, got, want)
		}
		if got := castEntryDCs(a.log(t, a.ana, e), toren); !slices.Equal(got, []int32{14}) {
			t.Errorf("rule %v: the caster's player reads the DCs %v, want [14]", on, got)
		}
		if got := castEntryDCs(a.log(t, a.master, e), toren); !slices.Equal(got, []int32{14}) {
			t.Errorf("rule %v: the master reads the DCs %v, want [14]", on, got)
		}
		if got := castEntryDCs(a.log(t, a.bia, e), toren); !slices.Equal(got, []int32{0}) {
			t.Errorf("rule %v: a bystander reads the DCs %v, want [0]", on, got)
		}
	}
}

// npcWizard creates an NPC wizard of level 5 (Intelligence 16: save DC 14) who prepared Burning Hands.
func (u *user) npcWizard(t *testing.T, campaignID, name string) *charactersv1.Character {
	t.Helper()
	sheet := &charactersv1.CharacterSheet{Content: &charactersv1.CharacterSheet_Full{Full: &charactersv1.FullSheet{
		BaseScores: &rulesv1.AbilityScores{Strength: 10, Dexterity: 14, Constitution: 12, Intelligence: 16, Wisdom: 10, Charisma: 10}, RaceKey: "race:human", Classes: []*charactersv1.ClassLevel{{ClassKey: "class:wizard", Level: 5}},
		PreparedSpellKeys: []string{burningHands},
	}}}
	res, err := u.characters.CreateCharacter(t.Context(), connect.NewRequest(&charactersv1.CreateCharacterRequest{
		CampaignId: campaignID, Kind: charactersv1.CharacterKind_CHARACTER_KIND_BOSS, Name: name, Sheet: sheet,
	}))
	if err != nil {
		t.Fatalf("CreateCharacter(%s) error = %v", name, err)
	}
	return res.Msg.GetCharacter()
}

// A spell cast by an NPC: the master plays the wizard Mago, whose Burning Hands catches Toren.
func TestSaveDCASpellCastByAnNPC(t *testing.T) {
	t.Parallel()
	for _, on := range []bool{false, true} {
		c := newCaveWith(t, 5, []string{burningHands})
		if on {
			c.setRules(t, showSaveDC)
		}
		mago := c.master.npcWizard(t, c.campaignID, "Mago")
		e := c.start(t, plan{
			npcs:     []*playv1.Participant{{CharacterId: mago.GetId()}, {CharacterId: c.ogre.GetId()}},
			npcRolls: []int{20, 2},
			players:  map[string]int32{"Toren": 5, "Pensantus": 4, "Brisa": 3},
			reveal:   []string{"Mago", "Ogro"},
			at:       map[string][2]int32{"Mago": {5, 7}, "Toren": {6, 7}, "Brisa": {3, 9}, "Pensantus": {12, 7}, "Ogro": {20, 7}},
		})
		c.h.roller.queue(1)
		c.mustCastArea(t, c.master, "Mago", burningHands, slotOfLevel(1), nil, toward(1, 0))
		toren := c.id(t, "Toren")
		want := int32(0)
		if on {
			want = 14
		}
		if got := castEntryDCs(c.log(t, c.caio, e), toren); !slices.Equal(got, []int32{want}) {
			t.Errorf("rule %v: Toren's player reads the DCs %v of the NPC's spell, want [%d]", on, got, want)
		}
		if got := castEntryDCs(c.log(t, c.master, e), toren); !slices.Equal(got, []int32{14}) {
			t.Errorf("rule %v: the master reads the DCs %v, want [14]", on, got)
		}
		for who, u := range map[string]*user{"Pensantus's player": c.ana, "Brisa's player": c.bia} {
			if got := castEntryDCs(c.log(t, u, e), toren); !slices.Equal(got, []int32{0}) {
				t.Errorf("rule %v: %s reads the DCs %v of an NPC's spell on Toren, want [0]", on, who, got)
			}
		}
	}
}

// trapDCs are the DCs the trap lines of the log of u carry for the target.
func trapDCs(l *playv1.ListCombatLogResponse, targetID string) []int32 {
	var out []int32
	for _, r := range l.GetRounds() {
		for _, en := range r.GetEntries() {
			if en.GetKind() != playv1.CombatLogKind_COMBAT_LOG_KIND_TRAP_TRIGGERED {
				continue
			}
			for _, cc := range en.GetTrap().GetCaught() {
				if cc.GetTargetId() == targetID && cc.GetSave() != nil {
					out = append(out, cc.GetSave().GetDc())
				}
			}
		}
	}
	return out
}

// A trap's save: the statue (DC 17) fires on Toren; his player reads the DC only with the rule on, nobody else's player ever.
func TestSaveDCATrapsSave(t *testing.T) {
	t.Parallel()
	for _, on := range []bool{false, true} {
		r := newTrapRig(t)
		if on {
			r.setRules(t, showSaveDC)
		}
		statue := r.trap(t, "Estátua de Fogo", 15, 12, func(s *mapsv1.TrapSpec) {
			s.Trigger = rulesv1.TrapTrigger_TRAP_TRIGGER_MANUAL
			s.Effect = &rulesv1.TrapEffect{
				Save: &rulesv1.TrapSaveEffect{
					Ability: rulesv1.Ability_ABILITY_DEXTERITY, Dc: 17, AppliesTo: rulesv1.TrapSaveApplies_TRAP_SAVE_APPLIES_CAUGHT,
					OnFail: &rulesv1.TrapOnFail{Damage: []*rulesv1.TrapDamage{{Dice: "2d6", DamageTypeKey: "damage-type:fire"}}},
					OnPass: rulesv1.TrapPassOutcome_TRAP_PASS_OUTCOME_HALF,
				},
				Targets: rulesv1.TrapTargets_TRAP_TARGETS_MANUAL,
			}
		})
		e := r.fight(t)
		r.h.roller.queue(5, 3, 4)
		if _, err := r.fireByHand(t, statue, r.id(t, "Toren")); err != nil {
			t.Fatalf("FireTrap() error = %v", err)
		}
		toren := r.id(t, "Toren")
		want := int32(0)
		if on {
			want = 17
		}
		if got := trapDCs(r.log(t, r.caio, e), toren); !slices.Equal(got, []int32{want}) {
			t.Errorf("rule %v: Toren's player reads the trap DCs %v, want [%d]", on, got, want)
		}
		if got := trapDCs(r.log(t, r.master, e), toren); !slices.Equal(got, []int32{17}) {
			t.Errorf("rule %v: the master reads the trap DCs %v, want [17]", on, got)
		}
		for who, u := range map[string]*user{"Pensantus's player": r.ana, "Brisa's player": r.bia} {
			if got := trapDCs(r.log(t, u, e), toren); len(got) != 0 && !slices.Equal(got, []int32{0}) {
				t.Errorf("rule %v: %s reads the trap DCs %v on Toren, want none", on, who, got)
			}
		}
	}
}

// The caster's own player keeps the DC of their spell with the rule off: Pensantus's Cloudkill, left on her own square, asks her
// player the save at the start of her turn, and the DC is on the question, as on her sheet.
func TestSaveDCTheCastersPlayerKeepsTheDCWithTheRuleOff(t *testing.T) {
	t.Parallel()
	c := zoneCave(t)
	e := c.zoneFight(t)
	c.passTo(t, e, "Pensantus")
	c.mustCastArea(t, c.ana, "Pensantus", cloudkillKey, slotOfLevel(5), at(12, 7), nil)
	c.passRaw(t, "Pensantus")
	w := c.windowOf(t, c.ana, playv1.ReactionKind_REACTION_KIND_EFFECT_SAVE)
	if w == nil {
		t.Fatalf("Pensantus's windows = %v, want the save of her own cloud", c.windows(t, c.ana))
	}
	if got, want := dcOf(w.GetEffectSave().Dc), zoneDC(t, c, cloudkillKey); got == 0 || got != want {
		t.Errorf("the caster's player reads the DC %d of her own spell, want %d", got, want)
	}
}

// The line of a rider's save (Open Hand, Stunning Strike) names the roll to a target's player, and the DC only
// when the table shows the DC of a save.
func TestSaveDCTheLineOfARidersSave(t *testing.T) {
	t.Parallel()
	r := &riderEvent{Choice: choiceStun, Result: riderFailed, D20: 4, Total: 6, DC: 17}
	if got := riderLogProto(r, true, false); !got.GetNumbers() || got.GetD20() != 4 || got.GetDc() != 0 {
		t.Errorf("the target's player reads %v with the rule off, want the roll and no DC", got)
	}
	if got := riderLogProto(r, true, true); got.GetDc() != 17 {
		t.Errorf("the target's player reads the DC %d with the rule on, want 17", got.GetDc())
	}
	if got := riderLogProto(r, false, false); got.GetNumbers() || got.GetDc() != 0 {
		t.Errorf("another player reads %v, want no numbers", got)
	}
}
