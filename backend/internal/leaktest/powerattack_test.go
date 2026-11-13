package leaktest

import (
	"strings"
	"testing"
	"time"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
)

// A power attack (RollAttackRequest.power_attack_key): Caio's paladin trades attack roll for damage with a table feat. The
// feat's name and the two numbers are the attacker's player's and the master's: the roll, the damage parts, the log line
// and the master's own answers have them. Another player reads that a trade was made; for a feat the master retired
// from the players they read "Outra fonte" in the part and in the log, and never the name, the key or a number (RN-10,
// RN-23). The retired feat's name is the canary "Zorvak4321"; the numbers are searched as numbers, in the fields that
// name them (the penalty, the damage bonus, the flat number of a part), never as digits in the text of an answer.

const (
	hiddenPowerName = "Zorvak4321"
	// The numbers of the retired feat and of the visible one; the totals of the rolls are not canaries.
	hiddenPenalty, hiddenBonus   = 9, 17
	visiblePenalty, visibleBonus = 3, 8

	kindPowerName         = "power-attack-name"
	kindPowerKey          = "power-attack-key"
	kindPowerPenalty      = "power-attack-penalty"
	kindPowerBonus        = "power-attack-damage-bonus"
	kindVisiblePenalty    = "power-attack-visible-penalty"
	kindVisibleBonus      = "power-attack-visible-damage-bonus"
	otherSource           = "Outra fonte"
	torenToHitBeforeTrade = 5 // Toren's proficiency (2) and Strength (+3)
)

// powerFeat gives Toren a table feat with a power attack for melee weapons and returns its key.
func (rt *rollsTable) powerFeat(name string, penalty, bonus int32) string {
	ctx := rt.t.Context()
	entry := must(rt.master.table.CreateTableEntry(ctx, rq(&rulesv1.CreateTableEntryRequest{
		CampaignId: rt.campaign, Body: &rulesv1.CreateTableEntryRequest_TableFeat{TableFeat: &rulesv1.TableFeat{
			NamePt: name, DescPt: []string{"Um talento da mesa."},
			Effects: []*rulesv1.TableEffect{{Type: "power_attack", AttackPenalty: penalty, DamageBonus: bonus, Weapon: "melee"}},
		}},
	}))).GetEntry()
	sheet := rt.toren.GetSheet()
	sheet.GetFull().FeatKeys = append(sheet.GetFull().GetFeatKeys(), entry.GetKey())
	rt.toren = must(rt.master.characters.UpdateCharacter(ctx, rq(&charactersv1.UpdateCharacterRequest{
		CampaignId: rt.campaign, CharacterId: rt.toren.GetId(), Revision: rt.toren.GetRevision(), Name: rt.toren.GetName(), Sheet: sheet,
	}))).GetCharacter()
	return entry.GetKey()
}

// keepReply keeps an answer the table got directly, as the matrix keeps its reads, and looks for the canaries a player may not
// read in it.
func (rt *rollsTable) keepReply(t *testing.T, p *person, what string, msg proto.Message) {
	t.Helper()
	body, err := protojson.Marshal(msg)
	if err != nil {
		t.Fatalf("marshal %s: %v", what, err)
	}
	r := reply{status: 200, body: body, msg: msg}
	rt.got.keep(p, r)
	if p == rt.master {
		return
	}
	for _, f := range rt.inspect(p, r, nil) {
		t.Errorf("%s: %s: %s", p.name, what, f)
	}
}

// powerPartsOf lists the damage parts and the rolls of the parts of the power attack anywhere in an answer.
func powerPartsOf(msg proto.Message) (parts []*playv1.DamagePart, rolls []*playv1.DamagePartRoll) {
	walk(msg.ProtoReflect(), "answer", func(_ string, _ protoreflect.Message, fd protoreflect.FieldDescriptor, v protoreflect.Value) {
		if fd.Message() == nil || fd.IsMap() {
			return
		}
		switch m := v.Message().Interface().(type) {
		case *playv1.DamagePart:
			if m.GetKey() == "power_attack" {
				parts = append(parts, m)
			}
		case *playv1.DamagePartRoll:
			if m.GetPartKey() == "power_attack" {
				rolls = append(rolls, m)
			}
		}
	})
	return parts, rolls
}

// powerMarkers are the markers of the power attacks on the lines of Toren's attacks, newest first.
func (rt *rollsTable) powerMarkers(p *person) []*playv1.CombatLogPowerAttack {
	var out []*playv1.CombatLogPowerAttack
	for _, r := range rt.logOf(p).GetRounds() {
		for _, en := range r.GetEntries() {
			if en.GetKind() == playv1.CombatLogKind_COMBAT_LOG_KIND_ATTACK && en.GetActorId() == rt.ids[labelToren] && en.GetPowerAttack() != nil {
				out = append(out, en.GetPowerAttack())
			}
		}
	}
	return out
}

func TestAPowerAttackIsTheAttackersAndTheMastersAlone(t *testing.T) {
	var hiddenKey, visibleKey, visibleName string
	rt := newRollsTableWith(t, func(rt *rollsTable) {
		hiddenKey = rt.powerFeat(hiddenPowerName, hiddenPenalty, hiddenBonus)
		visibleName = rt.secrets.public("vis-GolpeVisivel")
		visibleKey = rt.powerFeat(visibleName, visiblePenalty, visibleBonus)
	})
	w, m, caio, ana := rt.world, rt.master, rt.caio, rt.ana
	rt.buildRows()
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("the table: %v", r)
		}
	}()
	// The names and the keys are the owner's (the visible name is public: the other players read it, by the marker and the part);
	// the numbers are searched in the fields that carry them.
	w.secrets.add(&canary{needle: hiddenPowerName, kind: kindPowerName, readers: names([]*person{caio})})
	w.secrets.add(&canary{needle: hiddenKey, kind: kindPowerKey, readers: names([]*person{caio})})
	w.secrets.add(&canary{needle: visibleKey, kind: kindPowerKey + "-visible", readers: names([]*person{caio})})
	w.secrets.add(&canary{needle: "9", kind: kindPowerPenalty, numeric: true, value: hiddenPenalty, fields: []string{"penalty"}, readers: names([]*person{caio})})
	w.secrets.add(&canary{needle: "17", kind: kindPowerBonus, numeric: true, value: hiddenBonus, fields: []string{"damage_bonus", "flat"}, readers: names([]*person{caio})})
	w.secrets.add(&canary{needle: "3", kind: kindVisiblePenalty, numeric: true, value: visiblePenalty, fields: []string{"penalty"}, readers: names([]*person{caio})})
	w.secrets.add(&canary{needle: "8", kind: kindVisibleBonus, numeric: true, value: visibleBonus, fields: []string{"damage_bonus", "flat"}, readers: names([]*person{caio})})
	for _, label := range []string{labelToren, labelPens, "Esqueleto", "Arbusto", "Bandido", "Lobo"} {
		w.secrets.id("power-combatant", rt.ids[label], ana, caio)
	}
	watchers := map[*person]*streamWatcher{m: w.watchStream(m), ana: w.watchStream(ana), caio: w.watchStream(caio)}
	// The master retires the first feat from the players: the sheet keeps it and the trade still works.
	must(m.table.ArchiveTableEntry(t.Context(), rq(&rulesv1.ArchiveTableEntryRequest{CampaignId: w.campaign, Key: hiddenKey})))

	rt.toTurn(labelToren)
	rt.ask(t, "the combat begins")
	typed := func(r *playv1.RollDamageRequest) {
		r.ExtrasChosen, r.Roll = true, &playv1.RollDamageRequest_TypedSum{TypedSum: 3}
	}

	// 1. A visible feature: Caio's roll counts the penalty and lists the part with the name and the number.
	visible := rt.mustAttack(caio, labelToren, dagger, "Esqueleto", []int32{15}, func(r *playv1.RollAttackRequest) { r.PowerAttackKey = visibleKey })
	if d := visible.GetRoll().GetD20(); visible.GetPendingDamage() == nil || d.GetModifier() != torenToHitBeforeTrade-visiblePenalty {
		t.Fatalf("Caio's attack = %v, want a hit with the modifier counting the penalty", visible.GetRoll())
	}
	rt.keepReply(t, caio, "the roll of the visible trade", visible)
	if parts, _ := powerPartsOf(visible); len(parts) != 1 || parts[0].GetLabelPt() != visibleName || parts[0].GetFlat() != visibleBonus || parts[0].GetSourceKey() != visibleKey {
		t.Errorf("Caio reads the parts %v, want the visible feature's name, number and key", parts)
	}
	rt.ask(t, "the visible trade hit")
	rt.checkOthersRead(t, ana, visibleName)
	rolled := rt.damage(caio, visible.GetPendingDamage().GetId(), typed)
	rt.keepReply(t, caio, "the damage of the visible trade", rolled)
	rt.ask(t, "the visible trade's damage")
	rt.checkOthersRead(t, ana, visibleName)
	for who, p := range map[string]*person{"the master": m, "Caio": caio} {
		if got := rt.powerMarkers(p); len(got) != 1 || got[0].GetKey() != visibleKey || got[0].GetNamePt() != visibleName || got[0].GetPenalty() != visiblePenalty || got[0].GetDamageBonus() != visibleBonus || got[0].GetHidden() {
			t.Errorf("%s reads the markers %v, want the visible feature with %d and %d (the positive control)", who, got, visiblePenalty, visibleBonus)
		}
	}
	// Another player reads that a trade was made, with the feature's name and no key or number.
	if got := rt.powerMarkers(ana); len(got) != 1 || got[0].GetNamePt() != visibleName || got[0].GetKey() != "" || got[0].GetPenalty() != 0 || got[0].GetDamageBonus() != 0 || got[0].GetHidden() {
		t.Errorf("Ana reads the markers %v, want the visible feature's name alone", got)
	}
	rt.endTurn(caio)

	// 2. The retired feature: Caio and the master read the name and the numbers; Ana reads "Outra fonte" and no number. The master
	// rolls the damage this time, as the positive control of its own reply.
	rt.toTurn(labelToren)
	hidden := rt.mustAttack(caio, labelToren, dagger, "Bandido", []int32{19}, func(r *playv1.RollAttackRequest) { r.PowerAttackKey = hiddenKey })
	if d := hidden.GetRoll().GetD20(); hidden.GetPendingDamage() == nil || d.GetModifier() != torenToHitBeforeTrade-hiddenPenalty {
		t.Fatalf("Caio's attack = %v, want a hit with the modifier counting the penalty", hidden.GetRoll())
	}
	rt.keepReply(t, caio, "the roll of the retired trade", hidden)
	if parts, _ := powerPartsOf(hidden); len(parts) != 1 || parts[0].GetLabelPt() != hiddenPowerName || parts[0].GetFlat() != hiddenBonus || parts[0].GetSourceKey() != hiddenKey {
		t.Errorf("Caio reads the parts %v, want the retired feature's name, number and key", parts)
	}
	rt.ask(t, "the retired trade hit")
	rt.checkOthersRead(t, ana, otherSource)
	// Ana cannot roll Caio's damage; the refusal says nothing of the trade.
	if _, err := ana.combat.RollDamage(t.Context(), rq(&playv1.RollDamageRequest{
		CampaignId: w.campaign, EncounterId: rt.encounter.GetId(), PendingDamageId: hidden.GetPendingDamage().GetId(), IdempotencyKey: newKey(),
		ExtrasChosen: true, Roll: &playv1.RollDamageRequest_TypedSum{TypedSum: 3},
	})); err == nil {
		t.Error("Ana rolled Caio's damage")
	} else if strings.Contains(err.Error(), hiddenPowerName) || strings.Contains(err.Error(), hiddenKey) {
		t.Errorf("Ana's RollDamage on Caio's hit = %v, want a refusal that names nothing", err)
	}
	masterRolled := rt.damage(m, hidden.GetPendingDamage().GetId(), typed)
	rt.keepReply(t, m, "the damage of the retired trade, rolled by the master", masterRolled)
	if parts, rolls := powerPartsOf(masterRolled); len(parts) != 1 || parts[0].GetLabelPt() != hiddenPowerName || parts[0].GetFlat() != hiddenBonus || len(rolls) != 1 || rolls[0].GetFlat() != hiddenBonus || rolls[0].GetLabelPt() != hiddenPowerName {
		t.Errorf("the master reads the parts %v and the rolls %v, want the retired feature's name and number (the positive control)", parts, rolls)
	}
	rt.ask(t, "the retired trade's damage")
	rt.checkOthersRead(t, ana, otherSource)
	for who, p := range map[string]*person{"the master": m, "Caio": caio} {
		got := rt.powerMarkers(p)
		if len(got) != 2 || got[0].GetKey() != hiddenKey || got[0].GetNamePt() != hiddenPowerName || got[0].GetPenalty() != hiddenPenalty || got[0].GetDamageBonus() != hiddenBonus || got[0].GetHidden() {
			t.Errorf("%s reads the markers %v, want the retired feature with %d and %d first (the positive control)", who, got, hiddenPenalty, hiddenBonus)
		}
		// The log's damage line lists the part with its number.
		if line := rt.attackLine(p, labelToren, "Bandido"); line == nil || !hasPowerPart(line.GetDamage().GetParts(), hiddenPowerName, hiddenBonus) {
			t.Errorf("%s reads the line %v, want the part of the retired feature with its number (the positive control)", who, line)
		}
	}
	got := rt.powerMarkers(ana)
	if len(got) != 2 || got[0].GetNamePt() != otherSource || !got[0].GetHidden() || got[0].GetKey() != "" || got[0].GetPenalty() != 0 || got[0].GetDamageBonus() != 0 {
		t.Errorf("Ana reads the markers %v, want %q for the retired feature and no number", got, otherSource)
	}
	if line := rt.attackLine(ana, labelToren, "Bandido"); line == nil || len(line.GetDamage().GetParts()) != 0 {
		t.Errorf("Ana reads the line %v, want the damage with no part", line)
	}
	rt.endTurn(caio)

	// The end: the stream told every player of the steps, and carried the numbers and the name to the master and Caio alone.
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
			r := reply{status: 200, body: body, msg: ev}
			rt.got.keep(p, r)
			if p == m {
				continue
			}
			for _, f := range w.inspect(p, r, nil) {
				t.Errorf("%s: event %s: %s\n\tevent: %s", p.name, eventCase(ev), f, shorten(body))
			}
		}
	}
	t.Run("canaries are reachable", func(t *testing.T) { checkCanariesAreReachable(t, w, rt.got) })
}

// hasPowerPart says a damage line lists the power attack's part with the name and the number.
func hasPowerPart(parts []*playv1.DamagePartRoll, name string, flat int32) bool {
	for _, p := range parts {
		if p.GetPartKey() == "power_attack" && p.GetLabelPt() == name && p.GetFlat() == flat {
			return true
		}
	}
	return false
}

// checkOthersRead proves the parts and the rolls of the power attack that the combat shows another player name the source as
// they may read it and carry no number and no key.
//
// No read of another player carries the parts of a hit on an NPC (the combat lists none, the turn options of Caio's combatant
// are refused to Ana, and only the attacker and the master are answered with the damage), so these loops guard the routes that
// may come to carry them; the part and the roll as another player reads them are proved, with their positive controls, by
// TestThePowerAttackPartForEachReader and TestThePowerAttackRollForEachReader.
func (rt *rollsTable) checkOthersRead(t *testing.T, p *person, name string) {
	t.Helper()
	parts, rolls := powerPartsOf(rt.encounterOf(p))
	for _, part := range parts {
		if part.GetLabelPt() != name || part.GetFlat() != 0 || part.GetSourceKey() != "" || part.GetReasonPt() != "" {
			t.Errorf("%s reads the part %v in the combat, want %q and no number, key or reason", p.name, part, name)
		}
	}
	for _, roll := range rolls {
		if roll.GetLabelPt() != name || roll.GetFlat() != 0 {
			t.Errorf("%s reads the roll %v in the combat, want %q and no number", p.name, roll, name)
		}
	}
}
