package leaktest

import (
	"testing"
	"time"

	"google.golang.org/protobuf/encoding/protojson"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
)

// The damage a table spell adds to the caster's weapon hits, and the save it asks of the target (#577). Toren's paladin
// casts the spell, hits a skeleton and rolls the damage: the effect's die, its dice, its save ability and DC, the spell's
// name and key are the caster's player's and the master's. The save line the target's roll leaves in the log is
// read by the master and by Toren's player, who reads the outcome and the condition but never the NPC's die, total or DC;
// Ana reads no line of it, and nothing of the effect before or after the roll (RN-10). The twin is the same spell with the
// audience of all players: the attacker's player reads the same line, and the numbers stay the master's.
//
// The canaries: the sentence of the part (the save's ability), the spell's name and key (public: the cast names the spell to
// every player, and the effect's lines are read with it), the dice of the effect (4 d12, searched as numbers in the fields that
// carry a die or a count) and the DC of the NPC's save (searched as a number in the fields that name a DC).

const (
	hitEffectSides, hitEffectDice = 12, 4
	hitEffectCharisma             = 20 // Toren's modifier +5, with the proficiency +2: the spell save DC is 15
	hitEffectDC                   = 8 + 2 + 5
	hitEffectSaveFace             = 4 // the skeleton's save: it fails

	kindHitEffectName  = "hit-effect-name"
	kindHitEffectKey   = "hit-effect-key"
	kindHitEffectSides = "hit-effect-die"
	kindHitEffectDice  = "hit-effect-dice"
	kindHitEffectDC    = "hit-effect-dc"
	kindHitEffectNote  = "hit-effect-note"

	// effectNote is the sentence of the part before the roll: the trigger, the save's ability and the condition.
	effectNote = "Depois do dano, o alvo faz um teste de resistência de Sabedoria"
)

// hitEffectSpell makes Toren a caster of a table spell that adds 4d12 to his weapon hits and asks the target for a Wisdom
// save, and returns its key: its audience is the master's choice.
func (rt *rollsTable) hitEffectSpell(name string, audience rulesv1.TableEffectAudience) string {
	ctx := rt.t.Context()
	entry := must(rt.master.table.CreateTableEntry(ctx, rq(&rulesv1.CreateTableEntryRequest{
		CampaignId: rt.campaign, Body: &rulesv1.CreateTableEntryRequest_TableSpell{TableSpell: &rulesv1.TableSpell{
			NamePt: name, Level: 1, SchoolKey: "school:evocation",
			CastingTime:   &rulesv1.TableSpellCastingTime{Unit: rulesv1.CastingTimeUnit_CASTING_TIME_UNIT_BONUS_ACTION, Amount: 1},
			Range:         &rulesv1.TableSpellRange{Kind: rulesv1.SpellRangeKind_SPELL_RANGE_KIND_SELF},
			Duration:      &rulesv1.TableSpellDuration{Kind: rulesv1.SpellDurationKind_SPELL_DURATION_KIND_TIMED, Amount: 1, Unit: rulesv1.SpellDurationUnit_SPELL_DURATION_UNIT_MINUTE, UpTo: true},
			Components:    &rulesv1.TableSpellComponents{Verbal: true},
			Concentration: true,
			ClassKeys:     []string{"class:paladin"}, DescPt: []string{"Uma magia da mesa."},
			Target: &rulesv1.TableSpellTarget{Kind: rulesv1.TableSpellTargetKind_TABLE_SPELL_TARGET_KIND_SELF},
			Effect: &rulesv1.TableSpellEffect{Visibility: audience, Modifiers: []*rulesv1.TableEffectModifier{{
				Kind: rulesv1.TableEffectModifierKind_TABLE_EFFECT_MODIFIER_KIND_WEAPON_HIT_DAMAGE, Trigger: rulesv1.TableHitTrigger_TABLE_HIT_TRIGGER_EACH_HIT,
				Die: hitEffectSides, Dice: hitEffectDice, MaxDice: hitEffectDice + 6,
				Save: "wis", OnFailConditions: []string{"condition:frightened"}, ConditionDuration: rulesv1.TableHitConditionDuration_TABLE_HIT_CONDITION_DURATION_UNTIL_SPELL_END,
			}}},
		}},
	}))).GetEntry()
	sheet := rt.toren.GetSheet()
	full := sheet.GetFull()
	full.KnownSpellKeys, full.PreparedSpellKeys = []string{entry.GetKey()}, []string{entry.GetKey()}
	full.BaseScores.Charisma = hitEffectCharisma
	rt.toren = must(rt.master.characters.UpdateCharacter(ctx, rq(&charactersv1.UpdateCharacterRequest{
		CampaignId: rt.campaign, CharacterId: rt.toren.GetId(), Revision: rt.toren.GetRevision(), Name: rt.toren.GetName(), Sheet: sheet,
	}))).GetCharacter()
	return entry.GetKey()
}

// hasHitEffectPart says a damage line lists the part of the hit effect with its dice.
func hasHitEffectPart(parts []*playv1.DamagePartRoll, key string, dice int32) bool {
	for _, p := range parts {
		if p.GetPartKey() == key && p.GetHitEffect() && p.GetDiceCount() == dice {
			return true
		}
	}
	return false
}

// saveLines are the save lines of a hit effect in the log, as p reads them.
func (rt *rollsTable) saveLines(p *person) []*playv1.CombatLogEntry {
	var out []*playv1.CombatLogEntry
	for _, r := range rt.logOf(p).GetRounds() {
		for _, en := range r.GetEntries() {
			if en.GetKind() == playv1.CombatLogKind_COMBAT_LOG_KIND_HIT_RIDER {
				out = append(out, en)
			}
		}
	}
	return out
}

// hitEffectTable plays the hit: Toren casts the spell, hits the skeleton and rolls the damage, and every read is asked after
// each step. hidden says the spell's audience is its owner's alone.
func hitEffectTable(t *testing.T, hidden bool) {
	t.Helper()
	var key, name string
	audience := rulesv1.TableEffectAudience_TABLE_EFFECT_AUDIENCE_PUBLIC
	if hidden {
		audience = rulesv1.TableEffectAudience_TABLE_EFFECT_AUDIENCE_OWNER
	}
	rt := newRollsTableWith(t, func(rt *rollsTable) {
		// The cast and the concentration name the spell to every player, whatever the audience of its effect: the name and the key
		// are public, and the positive control that the effect's own lines are read with them.
		name = rt.secrets.public(kindHitEffectName)
		key = rt.hitEffectSpell(name, audience)
	})
	w, m, caio, ana := rt.world, rt.master, rt.caio, rt.ana
	rt.buildRows()
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("the table: %v", r)
		}
	}()
	w.secrets.id(kindHitEffectKey, key, caio, ana)
	w.secrets.add(&canary{needle: effectNote, kind: kindHitEffectNote, readers: names([]*person{caio})})
	// The dice of the effect and the DC of the save are the master's and the caster's player's: the player reads the dice, never the DC.
	w.secrets.add(&canary{needle: itoa(hitEffectSides), kind: kindHitEffectSides, numeric: true, value: hitEffectSides, fields: []string{"dice_sides"}, readers: names([]*person{caio})})
	w.secrets.add(&canary{needle: itoa(hitEffectDice), kind: kindHitEffectDice, numeric: true, value: hitEffectDice, fields: []string{"dice_count"}, readers: names([]*person{caio})})
	w.secrets.add(&canary{needle: itoa(hitEffectDC), kind: kindHitEffectDC, numeric: true, value: hitEffectDC, fields: []string{"dc"}})
	for _, label := range []string{labelToren, labelPens, "Esqueleto", "Arbusto", "Bandido", "Lobo"} {
		w.secrets.id("hit-effect-combatant", rt.ids[label], ana, caio)
	}
	watchers := map[*person]*streamWatcher{m: w.watchStream(m), ana: w.watchStream(ana), caio: w.watchStream(caio)}

	rt.toTurn(labelToren)
	rt.ask(t, "the combat begins")
	cast := must(caio.combat.CastSpell(t.Context(), rq(&playv1.CastSpellRequest{
		CampaignId: w.campaign, EncounterId: rt.encounter.GetId(), CasterId: rt.ids[labelToren], SpellKey: key, Slot: &playv1.SpellSlot{Level: 1},
		IdempotencyKey: newKey(), Roll: &playv1.CastSpellRequest_RollInApp{RollInApp: true},
	})))
	rt.keepReply(t, caio, "the cast", cast)
	rt.ask(t, "the spell is cast")

	hit := rt.mustAttack(caio, labelToren, dagger, "Esqueleto", []int32{19}, nil)
	rt.keepReply(t, caio, "the hit", hit)
	if hit.GetPendingDamage() == nil {
		t.Fatalf("Toren's attack = %v, want a hit", hit.GetRoll())
	}
	rt.ask(t, "the hit waits for its damage")

	w.dice.ScriptFor(t, 4, 1)
	w.dice.ScriptFor(t, hitEffectSides, 1, 1, 1, 1) // the dice stay low: the skeleton lives, and so its save is rolled
	w.dice.ScriptFor(t, 20, hitEffectSaveFace)
	rolled := rt.damage(caio, hit.GetPendingDamage().GetId(), func(r *playv1.RollDamageRequest) {
		r.ExtrasChosen, r.Roll = true, &playv1.RollDamageRequest_RollInApp{RollInApp: true}
	})
	rt.keepReply(t, caio, "the damage", rolled)
	rt.ask(t, "the damage is rolled")

	// The master's line is the positive control: the NPC's die, the DC, the spell and its save.
	lines := rt.saveLines(m)
	if len(lines) != 1 || !lines[0].GetHitRider().GetNumbers() || lines[0].GetHitRider().GetD20() != hitEffectSaveFace || lines[0].GetHitRider().GetDc() != hitEffectDC ||
		lines[0].GetHitRider().GetSaveAbility() != "wis" || lines[0].GetKey() != key || lines[0].GetKeyNamePt() != name {
		t.Fatalf("the master reads %v, want the save line with the die (%d), the DC (%d), the ability and the spell: the canaries are unreachable", lines, hitEffectSaveFace, hitEffectDC)
	}
	// Toren's player reads the outcome, the condition, the ability and the spell, never the NPC's numbers.
	lines = rt.saveLines(caio)
	if len(lines) != 1 {
		t.Fatalf("Caio reads %d save lines, want the one of his own hit", len(lines))
	}
	r := lines[0].GetHitRider()
	if r.GetResult() != playv1.HitRiderLogResult_HIT_RIDER_LOG_RESULT_FAILED || r.GetConditionNamePt() == "" || r.GetSaveAbility() != "wis" || r.GetSaveAbilityNamePt() != "Sabedoria" {
		t.Errorf("Caio reads %v, want a failed save, the condition it gave and the ability", r)
	}
	if r.GetNumbers() || r.GetD20() != 0 || r.GetSaveTotal() != 0 || r.GetDc() != 0 || r.GetTreatedAs() != 0 {
		t.Errorf("Caio reads the NPC's numbers: %v", r)
	}
	if lines[0].GetKey() != key || lines[0].GetKeyNamePt() != name {
		t.Errorf("Caio reads the spell %q %q, want %q %q", lines[0].GetKey(), lines[0].GetKeyNamePt(), key, name)
	}
	// The attack line lists the effect's part with its dice for the master and Toren's player (the positive control); Ana's lists none.
	for who, p := range map[string]*person{"the master": m, "Caio": caio} {
		if line := rt.attackLine(p, labelToren, "Esqueleto"); line == nil || !hasHitEffectPart(line.GetDamage().GetParts(), key, hitEffectDice) {
			t.Errorf("%s reads the line %v, want the part of the effect with its dice (the positive control)", who, line)
		}
	}
	if line := rt.attackLine(ana, labelToren, "Esqueleto"); line == nil || len(line.GetDamage().GetParts()) != 0 {
		t.Errorf("Ana reads the line %v, want the damage with no part", line)
	}
	// Ana reads no line of the effect, whoever may read the spell.
	if got := rt.saveLines(ana); len(got) != 0 {
		t.Errorf("Ana reads %d save lines of Toren's hit, want none", len(got))
	}
	rt.endTurn(caio)

	must(m.combat.EndEncounter(t.Context(), rq(&playv1.EndEncounterRequest{CampaignId: w.campaign, EncounterId: rt.encounter.GetId(), IdempotencyKey: newKey()})))
	must(m.play.EndGameSession(t.Context(), rq(&playv1.EndGameSessionRequest{CampaignId: w.campaign, GameSessionId: w.session})))
	for p, sw := range watchers {
		select {
		case <-sw.done:
		case <-time.After(30 * time.Second):
			t.Fatalf("%s's stream did not end with the session", p.name)
		}
		if len(sw.events) < 2 {
			t.Errorf("%s's stream heard %d events of all this, so it proves nothing", p.name, len(sw.events))
		}
		for _, ev := range sw.events {
			body, err := protojson.Marshal(ev)
			if err != nil {
				t.Fatalf("marshal an event: %v", err)
			}
			rep := reply{status: 200, body: body, msg: ev}
			rt.got.keep(p, rep)
			if p == m {
				continue
			}
			for _, f := range w.inspect(p, rep, nil) {
				t.Errorf("%s: event %s: %s\n\tevent: %s", p.name, eventCase(ev), f, shorten(body))
			}
		}
	}
	t.Run("canaries are reachable", func(t *testing.T) { checkCanariesAreReachable(t, w, rt.got) })
}

func TestAHitEffectOfAHiddenSpellIsTheAttackersAndTheMastersAlone(t *testing.T) {
	hitEffectTable(t, true)
}

// The twin: the same spell, public. The attacker's player reads the same line; the dice and the DC stay as private.
func TestAHitEffectOfAPublicSpellKeepsItsNumbersFromThePlayers(t *testing.T) {
	hitEffectTable(t, false)
}
