package leaktest

import (
	"testing"
	"time"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// The heal line of Vampiric Touch (RN-20, ruling B): every landed hit writes it, with 0 when nothing heals, and its numbers
// are the master's and the caster's player's alone. Another player reads the line with no numbers, so a heal of 0 and a heal
// of 8 look the same to them; when the target is hidden, no player reads the line (a player's combatant is never hidden, so
// the caster cannot be).
//
// Toren casts it as a wizard at the skeleton, hurt (so the heal is 8: half of 16 necrotic) or at full hit points (so the
// heal is 0). The heal of 8 is the canary, a number searched only in the fields named "amount"; the hit's own 16 is public
// to the table and is not one.

const (
	kindDrainHeal  = "drain-heal-amount"
	drainSpellKey  = "spell:vampiric-touch"
	drainHealValue = 8
)

// drainLinesOf are the heal lines of Toren's Vampiric Touch that p reads in the log.
func (rt *rollsTable) drainLinesOf(p *person) []*playv1.CombatLogEntry {
	var out []*playv1.CombatLogEntry
	for _, r := range rt.logOf(p).GetRounds() {
		for _, en := range r.GetEntries() {
			if en.GetKind() == playv1.CombatLogKind_COMBAT_LOG_KIND_ACTION && en.GetKey() == drainSpellKey && en.GetActorId() == rt.ids[labelToren] {
				out = append(out, en)
			}
		}
	}
	return out
}

// newest is the line of lines that is not one of seen, and records it.
func newest(t *testing.T, who string, lines []*playv1.CombatLogEntry, seen map[string]bool) *playv1.CombatLogEntry {
	t.Helper()
	var out *playv1.CombatLogEntry
	for _, l := range lines {
		if !seen[l.GetId()] {
			if out != nil {
				t.Fatalf("%s reads two new heal lines", who)
			}
			out, seen[l.GetId()] = l, true
		}
	}
	if out == nil {
		t.Fatalf("%s reads no new heal line", who)
	}
	return out
}

// drainStep is one cast of Vampiric Touch: how hurt the caster is, who is hidden when the damage lands, the slot and
// the necrotic faces (they add up to 16 whatever the slot).
type drainStep struct {
	name  string
	hurt  bool
	hide  string // "" or "Esqueleto": a player's combatant is never hidden, so the caster stays in the open
	slot  int32
	faces []int
}

func TestTheHealLineOfADrainSpellIsTheMastersAndTheCastersPlayersAlone(t *testing.T) {
	rt := newRollsTableWith(t, func(rt *rollsTable) {
		// Toren becomes a wizard with Vampiric Touch: three slots of the 3rd level and three of the 4th.
		sheet := rt.toren.GetSheet()
		full := sheet.GetFull()
		full.Classes = []*charactersv1.ClassLevel{{ClassKey: "class:wizard", Level: 11}}
		full.FeatureChoiceKeys = nil
		full.KnownSpellKeys, full.PreparedSpellKeys = []string{drainSpellKey}, []string{drainSpellKey}
		rt.toren = must(rt.master.characters.UpdateCharacter(rt.t.Context(), rq(&charactersv1.UpdateCharacterRequest{
			CampaignId: rt.campaign, CharacterId: rt.toren.GetId(), Revision: rt.toren.GetRevision(), Name: rt.toren.GetName(), Sheet: sheet,
		}))).GetCharacter()
	})
	w, m, caio, ana := rt.world, rt.master, rt.caio, rt.ana
	ctx := t.Context()
	rt.buildRows()
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("the table: %v", r)
		}
	}()
	w.secrets.add(&canary{needle: itoa(drainHealValue), kind: kindDrainHeal, numeric: true, value: drainHealValue, fields: []string{"amount"}, readers: names([]*person{caio})})
	watchers := map[*person]*streamWatcher{m: w.watchStream(m), ana: w.watchStream(ana), caio: w.watchStream(caio)}
	rt.ask(t, "the combat begins")

	hide := func(label string, hidden bool) {
		must(m.combat.SetCombatantHidden(ctx, rq(&playv1.SetCombatantHiddenRequest{
			CampaignId: w.campaign, EncounterId: rt.encounter.GetId(), CombatantId: rt.ids[label], IdempotencyKey: newKey(), Hidden: hidden,
		})))
	}
	// The heal lines each reader holds after the steps so far: the master all of them, the caster's player the ones that were not secret, the other player the same lines with no numbers.
	var master, owner, other int
	seenBy := map[*person]map[string]bool{m: {}, caio: {}}
	steps := []drainStep{
		{"hurt caster, in the open", true, "", 3, []int{6, 5, 5}},
		{"caster at full hit points, in the open", false, "", 3, []int{6, 5, 5}},
		{"hurt caster, hidden target", true, "Esqueleto", 3, []int{6, 5, 5}},
		{"caster at full hit points, hidden target", false, "Esqueleto", 4, []int{6, 5, 3, 2}},
	}
	for _, s := range steps {
		rt.toTurn(labelToren)
		vitals := func() *playv1.CharacterVitals {
			for _, v := range must(m.play.GetLiveSession(ctx, rq(&playv1.GetLiveSessionRequest{CampaignId: w.campaign}))).GetVitals() {
				if v.GetCharacterId() == w.toren.GetId() {
					return v
				}
			}
			t.Fatalf("no vitals for Toren")
			return nil
		}
		current := vitals().GetHitPointsMax()
		if s.hurt {
			current = 1
		}
		must(m.play.AdjustCharacterVitals(ctx, rq(&playv1.AdjustCharacterVitalsRequest{
			CampaignId: w.campaign, CharacterId: w.toren.GetId(), IdempotencyKey: newKey(), ExpectedRevision: proto.Int32(vitals().GetRevision()), HitPointsCurrent: proto.Int32(current),
		})))
		// The skeleton lives through every hit: the master gives it its hit points and temporary ones again.
		must(m.combat.AdjustCombatantHitPoints(ctx, rq(&playv1.AdjustCombatantHitPointsRequest{
			CampaignId: w.campaign, EncounterId: rt.encounter.GetId(), CombatantId: rt.ids["Esqueleto"], IdempotencyKey: newKey(),
			Change: &playv1.AdjustCombatantHitPointsRequest_HitPoints{HitPoints: 13}, HitPointsTemporary: proto.Int32(100),
		})))
		cast := must(caio.combat.CastSpell(ctx, rq(&playv1.CastSpellRequest{
			CampaignId: w.campaign, EncounterId: rt.encounter.GetId(), CasterId: rt.ids[labelToren], SpellKey: drainSpellKey, Slot: &playv1.SpellSlot{Level: s.slot},
			Targets: []*playv1.SpellTarget{{CombatantId: rt.ids["Esqueleto"]}}, IdempotencyKey: newKey(), D20Faces: []int32{19},
		})))
		rt.keepReply(t, caio, s.name+": the cast", cast)
		id := cast.GetCast().GetTargets()[0].GetPendingDamageId()
		if id == "" {
			t.Fatalf("%s: the attack opened no damage: %v", s.name, cast.GetCast())
		}
		if s.hide != "" {
			hide(s.hide, true)
		}
		w.dice.ScriptFor(t, 6, s.faces...)
		rolled := rt.damage(m, id, func(r *playv1.RollDamageRequest) {
			r.ExtrasChosen, r.Roll = true, &playv1.RollDamageRequest_RollInApp{RollInApp: true}
		})
		rt.keepReply(t, m, s.name+": the damage", rolled)
		if s.hide != "" {
			hide(s.hide, false)
		}
		rt.ask(t, s.name)

		want := int32(0)
		if s.hurt {
			want = drainHealValue
		}
		master++
		if s.hide == "" {
			owner++
			other++
		}
		lines := rt.drainLinesOf(m)
		if len(lines) != master {
			t.Fatalf("%s: the master reads %d heal lines, want %d: the canaries are unreachable", s.name, len(lines), master)
		}
		if got := newest(t, "the master", lines, seenBy[m]).GetDamage(); got == nil || !got.GetHealing() || got.GetAmount() != want {
			t.Errorf("%s: the master reads %v, want a heal of %d", s.name, got, want)
		}
		lines = rt.drainLinesOf(caio)
		if len(lines) != owner {
			t.Fatalf("%s: the caster's player reads %d heal lines, want %d", s.name, len(lines), owner)
		}
		if s.hide == "" {
			if got := newest(t, "the caster's player", lines, seenBy[caio]).GetDamage(); got == nil || !got.GetHealing() || got.GetAmount() != want {
				t.Errorf("%s: the caster's player reads %v, want a heal of %d", s.name, got, want)
			}
		}
		lines = rt.drainLinesOf(ana)
		if len(lines) != other {
			t.Fatalf("%s: the other player reads %d heal lines, want %d", s.name, len(lines), other)
		}
		for _, line := range lines {
			if line.GetDamage() != nil {
				t.Errorf("%s: the other player reads the numbers of a heal: %v", s.name, line.GetDamage())
			}
		}
		rt.endTurn(caio)
	}

	must(m.combat.EndEncounter(ctx, rq(&playv1.EndEncounterRequest{CampaignId: w.campaign, EncounterId: rt.encounter.GetId(), IdempotencyKey: newKey()})))
	must(m.play.EndGameSession(ctx, rq(&playv1.EndGameSessionRequest{CampaignId: w.campaign, GameSessionId: w.session})))
	for p, sw := range watchers {
		select {
		case <-sw.done:
		case <-time.After(30 * time.Second):
			t.Fatalf("%s's stream did not end with the session", p.name)
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
