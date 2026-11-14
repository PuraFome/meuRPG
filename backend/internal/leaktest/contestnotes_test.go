package leaktest

import (
	"slices"
	"strings"
	"testing"

	"google.golang.org/protobuf/proto"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1/playv1connect"
	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
)

// The notes behind a contest's roll follow the hiding rule (RN-10, RN-22): a source the master hides from the roller (an effect
// with no outward sign) is read by the roller as "Outra fonte", the way a hidden effect's bonus is, and only the master reads
// what it is. The effect here gives its target the disadvantage on Strength and Dexterity checks, so it sits behind a roll's mode
// and the note it makes is the kind that names it ("Efeito ativo"). The roller's contest start (Pensantus's, rolled by Ana),
// the contest Toren answers and his Hide (Caio's) each carry a note of it, and the roller must read none that names it.
//
// The canary is the label the note had before it followed the rule. The positive control is the master's reads, which name it.

const hiddenEffectNoteWord = "Efeito ativo"

// putHiddenCheckEffect puts a table effect on the combatant that the players may not read: the disadvantage on Strength and
// Dexterity checks.
func (w *world) putHiddenCheckEffect(target *playv1.Combatant) {
	w.t.Helper()
	ctx := w.t.Context()
	spell := &rulesv1.TableSpell{
		NamePt: "Marca de teste", Level: 1, SchoolKey: "school:evocation",
		CastingTime: &rulesv1.TableSpellCastingTime{Unit: rulesv1.CastingTimeUnit_CASTING_TIME_UNIT_ACTION, Amount: 1},
		Range:       &rulesv1.TableSpellRange{Kind: rulesv1.SpellRangeKind_SPELL_RANGE_KIND_RANGED, DistanceFt: 60},
		Duration:    &rulesv1.TableSpellDuration{Kind: rulesv1.SpellDurationKind_SPELL_DURATION_KIND_TIMED, Amount: 1, Unit: rulesv1.SpellDurationUnit_SPELL_DURATION_UNIT_MINUTE},
		Components:  &rulesv1.TableSpellComponents{Verbal: true, Somatic: true},
		ClassKeys:   []string{"class:wizard"}, DescPt: []string{"Um efeito de teste da mesa."},
		Target: &rulesv1.TableSpellTarget{Kind: rulesv1.TableSpellTargetKind_TABLE_SPELL_TARGET_KIND_CREATURE},
		Effect: &rulesv1.TableSpellEffect{
			Visibility: rulesv1.TableEffectAudience_TABLE_EFFECT_AUDIENCE_PUBLIC,
			Modifiers: []*rulesv1.TableEffectModifier{{
				Kind: rulesv1.TableEffectModifierKind_TABLE_EFFECT_MODIFIER_KIND_CHECK_DISADVANTAGE, Abilities: []string{"str", "dex"},
			}},
		},
	}
	entry := must(w.master.table.CreateTableEntry(ctx, rq(&rulesv1.CreateTableEntryRequest{
		CampaignId: w.campaign, Body: &rulesv1.CreateTableEntryRequest_TableSpell{TableSpell: spell},
	}))).GetEntry()
	hidden := false
	must(w.master.lasting.AddLastingEffect(ctx, rq(&playv1.AddLastingEffectRequest{
		CampaignId: w.campaign, EncounterId: w.encounter.GetId(), IdempotencyKey: newKey(), CatalogKey: entry.GetKey(),
		TargetIds: []string{target.GetId()}, CasterId: w.combatant(w.pens).GetId(),
		Duration:      &playv1.EffectDurationChoice{Kind: playv1.EffectDurationKind_EFFECT_DURATION_KIND_ROUNDS, Rounds: 10, AnchorCombatantId: w.combatant(w.pens).GetId()},
		PlayerVisible: &hidden,
	})))
}

func TestTheNotesOfAContestRollFollowTheHidingRule(t *testing.T) {
	contests := []string{"/meurpg.play.v1.ContestService/", "/meurpg.play.v1.CombatService/ListCombatLog"}
	// healPens gets Pensantus up (the fixture left her down), as the favor rows do before she starts a contest.
	healPens := func(w *world) {
		must(w.master.play.AdjustCharacterVitals(w.t.Context(), rq(&playv1.AdjustCharacterVitalsRequest{
			CampaignId: w.campaign, CharacterId: w.pens.GetId(), IdempotencyKey: newKey(), ExpectedRevision: w.master.revisionOf(w, w.pens.GetId()), HitPointsCurrent: proto.Int32(5),
		})))
	}
	paths := []struct {
		name string
		// only limits the sweep to the reads of these services (healing Pensantus, which the other rows do not expect).
		only []string
		// roller is who rolls the note's roll, and target the combatant that carries the hidden effect.
		roller func(w *world) *person
		target func(w *world) *playv1.Combatant
		call   func(w *world, key string) (procedure string, req proto.Message)
	}{
		{"a contest start", contests, func(w *world) *person { return w.ana }, func(w *world) *playv1.Combatant { return w.combatant(w.pens) }, func(w *world, key string) (string, proto.Message) {
			healPens(w)
			w.toTurnOf(w.pens.GetId())
			return playv1connect.ContestServiceStartContestProcedure, &playv1.StartContestRequest{
				CampaignId: w.campaign, EncounterId: w.encounter.GetId(), IdempotencyKey: key, Roll: appRoll(),
				InitiatorId: w.combatant(w.pens).GetId(), TargetId: w.combatant(w.toren).GetId(), Purpose: playv1.ContestPurpose_CONTEST_PURPOSE_SHOVE,
			}
		}},
		{"a contest answer", contests, func(w *world) *person { return w.caio }, func(w *world) *playv1.Combatant { return w.combatant(w.toren) }, func(w *world, key string) (string, proto.Message) {
			healPens(w)
			w.toTurnOf(w.pens.GetId())
			started := must(w.master.contests.StartContest(w.t.Context(), rq(&playv1.StartContestRequest{
				CampaignId: w.campaign, EncounterId: w.encounter.GetId(), IdempotencyKey: newKey(), Roll: appRoll(),
				InitiatorId: w.combatant(w.pens).GetId(), TargetId: w.combatant(w.toren).GetId(), Purpose: playv1.ContestPurpose_CONTEST_PURPOSE_SHOVE,
			})))
			return playv1connect.ContestServiceRespondContestProcedure, &playv1.RespondContestRequest{
				CampaignId: w.campaign, EncounterId: w.encounter.GetId(), IdempotencyKey: key, ContestId: started.GetContest().GetId(),
				Skill: playv1.ContestSkill_CONTEST_SKILL_ATHLETICS, Roll: appRoll(),
			}
		}},
		{"a hide", nil, func(w *world) *person { return w.caio }, func(w *world) *playv1.Combatant { return w.combatant(w.toren) }, func(w *world, key string) (string, proto.Message) {
			w.toTurnOf(w.toren.GetId())
			return playv1connect.ContestServiceHideProcedure, &playv1.HideRequest{
				CampaignId: w.campaign, EncounterId: w.encounter.GetId(), IdempotencyKey: key, Roll: appRoll(),
				CombatantId: w.combatant(w.toren).GetId(),
			}
		}},
	}
	for _, p := range paths {
		t.Run(p.name, func(t *testing.T) {
			w := newWorld(t)
			word := w.secrets.add(&canary{needle: hiddenEffectNoteWord, kind: "hidden-effect-note"})
			roller := p.roller(w)
			w.putHiddenCheckEffect(p.target(w))
			w.hints.Settle(t)
			procedure, req := p.call(w, newKey())
			own := roller.call(procedure, req)
			if !own.ok() {
				t.Fatalf("%s's call on %s was refused: HTTP %d %s", roller.name, p.name, own.status, own.body)
			}
			got := newAnswers()
			got.keep(roller, own)
			w.hints.Settle(t)
			for _, r := range reads {
				if len(p.only) == 0 || slices.ContainsFunc(p.only, func(prefix string) bool { return strings.HasPrefix(r.procedure, prefix) }) {
					w.runRead(t, r, got)
				}
			}
			// The positive control: the master reads the note in full, in some read.
			if !got.holds(w.master, word) {
				t.Errorf("the master reads no %q of %s in any read: the control that it shows to the master is missing", hiddenEffectNoteWord, p.name)
			}
			for _, other := range []*person{w.caio, w.ana, w.pending, w.stranger} {
				if got.holds(other, word) {
					t.Errorf("%s reads %q of %s", other.name, hiddenEffectNoteWord, p.name)
				}
			}
		})
	}
}
