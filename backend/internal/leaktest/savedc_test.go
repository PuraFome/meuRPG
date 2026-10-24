package leaktest

import (
	"strconv"
	"testing"

	"google.golang.org/protobuf/reflect/protoreflect"

	campaignsv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/campaigns/v1"
	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1/playv1connect"
)

// The table rule "Os jogadores veem a CD dos testes de resistência" (RN-24, RN-10): the DC of a
// saving throw is the master's. With the rule on, the player who rolls the save reads it too, in
// what they are asked and in what they are told; no other player ever does. The DC is a number
// no other number of these answers matches (37 for an effect, the fixture's trap save for a
// trap), searched in the numeric fields named "dc", and the positive controls show the number in
// the master's answer, and in the roller's when the rule is on.

// saveDCEffect is the DC the Hold Person of these tests asks.
const saveDCEffect = 37

// showSaveDC turns the table's rule on or off, as the master.
func (w *world) showSaveDC(on bool) {
	w.t.Helper()
	ctx := w.t.Context()
	rules := must(w.master.campaigns.GetTableRules(ctx, rq(&campaignsv1.GetTableRulesRequest{CampaignId: w.campaign}))).GetRules()
	rules.ShowSaveDc = on
	must(w.master.campaigns.SetTableRules(ctx, rq(&campaignsv1.SetTableRulesRequest{CampaignId: w.campaign, Rules: rules})))
}

// holdsNumber says whether the answer holds the canary's number in one of its fields.
func holdsNumber(r reply, c *canary) bool {
	if r.msg == nil {
		return false
	}
	found := false
	walk(r.msg.ProtoReflect(), "", func(_ string, _ protoreflect.Message, fd protoreflect.FieldDescriptor, v protoreflect.Value) {
		if isNumber(fd) && c.matchesNumber(fd, asInt(fd, v)) {
			found = true
		}
	})
	return found
}

// canaryOf finds the registered number of a kind.
func (w *world) canaryOf(kind string) *canary {
	w.t.Helper()
	for _, c := range w.secrets.list {
		if c.kind == kind && c.numeric {
			return c
		}
	}
	w.t.Fatalf("no number of the kind %q is registered", kind)
	return nil
}

// A Hold Person the Mago casts on Toren asks his player the saving throw at the end of his turn,
// against a DC of 37. Rule off: the DC is in no answer of a player. Rule on: Caio, who rolls it,
// reads it in the question, the card and the answer; Ana, who does not, never does.
func TestTheDCOfAnEffectSaveReachesOnlyItsRollerWhenTheTableShowsIt(t *testing.T) {
	for _, on := range []bool{false, true} {
		t.Run("rule "+strconv.FormatBool(on), func(t *testing.T) {
			w := newWorld(t)
			mage := w.reactionFight()
			w.secrets.number("effect-save-dc", saveDCEffect, "dc")
			dc := w.canaryOf("effect-save-dc")
			if on {
				w.showSaveDC(true)
				w.secrets.allowNeedle(strconv.Itoa(saveDCEffect), w.caio)
			}
			toren := w.combatant(w.toren)
			visible := true
			must(w.master.lasting.AddLastingEffect(t.Context(), rq(&playv1.AddLastingEffectRequest{
				CampaignId: w.campaign, EncounterId: w.encounter.GetId(), IdempotencyKey: newKey(), CatalogKey: "spell:hold-person",
				TargetIds: []string{toren.GetId()}, CasterId: mage.GetId(), SaveDc: new(int32(saveDCEffect)), PlayerVisible: &visible,
				Duration: &playv1.EffectDurationChoice{Kind: playv1.EffectDurationKind_EFFECT_DURATION_KIND_ROUNDS, Rounds: 10, AnchorCombatantId: toren.GetId()},
			})))
			must(w.master.combat.EndTurn(t.Context(), rq(&playv1.EndTurnRequest{
				CampaignId: w.campaign, EncounterId: w.encounter.GetId(), IdempotencyKey: newKey(), ExpectedCombatantId: toren.GetId(), DiscardPendingDamage: true,
			})))
			enc := must(w.master.combat.GetEncounter(t.Context(), rq(&playv1.GetEncounterRequest{CampaignId: w.campaign}))).GetEncounter()
			var window string
			for _, rw := range enc.GetReactionWindows() {
				if rw.GetKind() == playv1.ReactionKind_REACTION_KIND_EFFECT_SAVE {
					window = rw.GetId()
				}
			}
			if window == "" {
				t.Fatal("Toren's save is not asked")
			}

			roll := w.caio.call(playv1connect.LastingEffectServiceRollEffectSaveProcedure, &playv1.RollEffectSaveRequest{
				CampaignId: w.campaign, EncounterId: enc.GetId(), WindowId: window, IdempotencyKey: newKey(), Roll: &playv1.RollEffectSaveRequest_D20Face{D20Face: 5},
			})
			if !roll.ok() {
				t.Fatalf("Caio's save: status %d %s", roll.status, roll.body)
			}
			// What Caio is told (the answer, the log line, the card) and what Ana reads.
			for _, p := range []*person{w.ana, w.caio} {
				replies := w.reads(p)
				if p == w.caio {
					replies = append(replies, roll)
				}
				for _, r := range replies {
					for _, f := range w.inspect(p, r, nil) {
						t.Errorf("%s: %s", p.name, f)
					}
				}
			}
			// The positive controls: the master reads the number, and Caio does when the rule is on.
			masterHolds := false
			for _, r := range w.reads(w.master) {
				masterHolds = masterHolds || holdsNumber(r, dc)
			}
			if !masterHolds {
				t.Error("the master's reads do not hold the DC: the needle is unreachable")
			}
			caio := append(w.reads(w.caio), roll)
			got := false
			for _, r := range caio {
				got = got || holdsNumber(r, dc)
			}
			if got != on {
				t.Errorf("Caio reads the DC = %v, want %v (the rule is %v)", got, on, on)
			}
		})
	}
}

// The question itself, while it waits: the window of the end-of-turn save is read by its roller,
// who reads the DC only with the rule on, and by nobody else.
func TestTheQuestionOfAnEffectSaveCarriesTheDCOnlyForItsRollerWhenTheTableShowsIt(t *testing.T) {
	for _, on := range []bool{false, true} {
		t.Run("rule "+strconv.FormatBool(on), func(t *testing.T) {
			w := newWorld(t)
			mage := w.reactionFight()
			w.secrets.number("effect-save-dc", saveDCEffect, "dc")
			dc := w.canaryOf("effect-save-dc")
			if on {
				w.showSaveDC(true)
				w.secrets.allowNeedle(strconv.Itoa(saveDCEffect), w.caio)
			}
			toren := w.combatant(w.toren)
			must(w.master.lasting.AddLastingEffect(t.Context(), rq(&playv1.AddLastingEffectRequest{
				CampaignId: w.campaign, EncounterId: w.encounter.GetId(), IdempotencyKey: newKey(), CatalogKey: "spell:hold-person",
				TargetIds: []string{toren.GetId()}, CasterId: mage.GetId(), SaveDc: new(int32(saveDCEffect)),
				Duration: &playv1.EffectDurationChoice{Kind: playv1.EffectDurationKind_EFFECT_DURATION_KIND_ROUNDS, Rounds: 10, AnchorCombatantId: toren.GetId()},
			})))
			must(w.master.combat.EndTurn(t.Context(), rq(&playv1.EndTurnRequest{
				CampaignId: w.campaign, EncounterId: w.encounter.GetId(), IdempotencyKey: newKey(), ExpectedCombatantId: toren.GetId(), DiscardPendingDamage: true,
			})))
			for _, p := range []*person{w.ana, w.caio} {
				for _, r := range w.reads(p) {
					for _, f := range w.inspect(p, r, nil) {
						t.Errorf("%s: %s", p.name, f)
					}
				}
			}
			if !holdsNumber(w.reads(w.master)[0], dc) {
				t.Error("the master's combat does not hold the DC of the question: the needle is unreachable")
			}
			if got := holdsNumber(w.reads(w.caio)[0], dc); got != on {
				t.Errorf("Caio's combat holds the DC = %v, want %v", got, on)
			}
			if holdsNumber(w.reads(w.ana)[0], dc) {
				t.Error("Ana's combat holds the DC of Toren's save")
			}
		})
	}
}

// A trap that fires on Toren asks his save against its DC. The master fires it by hand: the log
// carries the DC for the master, and for Caio, whose character rolls it, only with the rule on.
func TestTheDCOfATrapSaveReachesOnlyItsRollerWhenTheTableShowsIt(t *testing.T) {
	for _, on := range []bool{false, true} {
		t.Run("rule "+strconv.FormatBool(on), func(t *testing.T) {
			w := newWorld(t)
			dc := w.canaryOf("trap-save-dc")
			if on {
				w.showSaveDC(true)
				w.secrets.allowNeedle(strconv.Itoa(trapSave), w.caio)
			}
			fireByHand(w, w.combatant(w.toren).GetId())
			firedTrapIsPublic(w)
			for _, p := range []*person{w.ana, w.caio} {
				for _, f := range w.inspect(p, trapLog(w, p), nil) {
					t.Errorf("%s: %s", p.name, f)
				}
			}
			if !holdsNumber(trapLog(w, w.master), dc) {
				t.Error("the master's log does not hold the trap's DC: the needle is unreachable")
			}
			if got := holdsNumber(trapLog(w, w.caio), dc); got != on {
				t.Errorf("Caio's log holds the trap's DC = %v, want %v", got, on)
			}
			if holdsNumber(trapLog(w, w.ana), dc) {
				t.Error("Ana's log holds the DC of the save Toren rolls")
			}
		})
	}
}
