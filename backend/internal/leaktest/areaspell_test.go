package leaktest

import (
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	campaignsv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/campaigns/v1"
	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	mapsv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/maps/v1"
	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
)

// An area spell a player casts that hits a hidden creature (RN-10, RN-20). The effect
// is the creature's like anybody's, and until the master reveals it nothing a player
// receives names or counts it: not the preview, the cast's answer, the combat, the
// log, the turn options, nor the stream. After a reveal the creature is on the map,
// but the line of the cast still does not say it was hit. The master's own copies are
// the positive control: the scan finds the needles there.

// areaLeak is the marker a hidden creature's name carries, and what else names it.
type areaLeak struct {
	name, characterID, combatantID, question string
}

func (l areaLeak) needles() []string {
	out := []string{l.name, l.characterID, l.combatantID}
	if l.question != "" {
		out = append(out, l.question)
	}
	return out
}

func jsonOf(m proto.Message) string {
	b, err := protojson.Marshal(m)
	if err != nil {
		panic(err)
	}
	return string(b)
}

// namesAny returns the needles found in the text.
func namesAny(text string, needles []string) []string {
	var out []string
	for _, n := range needles {
		if n != "" && strings.Contains(text, n) {
			out = append(out, n)
		}
	}
	return out
}

func TestAnAreaSpellThatHitsAHiddenCreatureNamesItToNoPlayerBeforeTheReveal(t *testing.T) {
	w := &world{stack: newStack(t), secrets: newSecrets(), pts: map[string]*mapsv1.MapPoint{}, puzzles: map[string]*playv1.Puzzle{}, notes: map[string]string{}}
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("building the fixture: %v", r)
		}
	}()
	ctx := t.Context()
	m := w.master
	w.buildPeople()
	w.imgMap = w.image("area-map-image", 240, 160)
	w.fogMap = must(m.maps.CreateMap(ctx, rq(&mapsv1.CreateMapRequest{CampaignId: w.campaign, Name: w.secrets.public("area-map-name"), ImageId: w.imgMap}))).GetMap().GetId()
	must(m.maps.SetMapRevealed(ctx, rq(&mapsv1.SetMapRevealedRequest{CampaignId: w.campaign, MapId: w.fogMap, Revealed: true})))
	must(m.maps.SetMapGrid(ctx, rq(&mapsv1.SetMapGridRequest{CampaignId: w.campaign, MapId: w.fogMap, Columns: 24})))
	w.session = must(m.play.StartGameSession(ctx, rq(&playv1.StartGameSessionRequest{CampaignId: w.campaign}))).GetGameSession().GetId()
	must(m.play.SetCurrentMap(ctx, rq(&playv1.SetCurrentMapRequest{CampaignId: w.campaign, MapId: w.fogMap})))

	spells := []string{"spell:fireball"}
	wizard := must(w.ana.characters.CreateCharacter(ctx, rq(&charactersv1.CreateCharacterRequest{
		CampaignId: w.campaign, Kind: charactersv1.CharacterKind_CHARACTER_KIND_PLAYER, Name: "Pensantus",
		Sheet: &charactersv1.CharacterSheet{Content: &charactersv1.CharacterSheet_Full{Full: &charactersv1.FullSheet{
			BaseScores: &rulesv1.AbilityScores{Strength: 10, Dexterity: 14, Constitution: 12, Intelligence: 16, Wisdom: 10, Charisma: 8},
			RaceKey:    "race:gnome", Classes: []*charactersv1.ClassLevel{{ClassKey: "class:wizard", Level: 5}},
			KnownSpellKeys: spells, PreparedSpellKeys: spells,
		}}},
	}))).GetCharacter()
	toren := w.pc(w.caio, "Toren", "race:human")
	seen, seenName := w.npc("area-seen", &charactersv1.BasicSheet{HitPointsMax: 40, ArmorClass: 12, SpeedFt: 30})
	hidden, hiddenName := w.npc("area-hidden", &charactersv1.BasicSheet{HitPointsMax: 40, ArmorClass: 12, SpeedFt: 30})
	w.allow("area-seen-name", w.ana, w.caio)
	w.secrets.id("area-hidden-npc", hidden.GetId())

	rules := must(m.campaigns.GetTableRules(ctx, rq(&campaignsv1.GetTableRulesRequest{CampaignId: w.campaign}))).GetRules()
	rules.HiddenAreaHits = campaignsv1.HiddenAreaHitRule_HIDDEN_AREA_HIT_RULE_ASK
	must(m.campaigns.SetTableRules(ctx, rq(&campaignsv1.SetTableRulesRequest{CampaignId: w.campaign, Rules: rules})))

	revealed := false
	e := must(m.combat.StartEncounter(ctx, rq(&playv1.StartEncounterRequest{
		CampaignId: w.campaign, IdempotencyKey: newKey(), Name: w.secrets.public("area-encounter"),
		Participants: []*playv1.Participant{{CharacterId: seen.GetId(), Hidden: &revealed}, {CharacterId: hidden.GetId()}},
	}))).GetEncounter()
	var hiddenCombatant, casterCombatant string
	for _, c := range e.GetCombatants() {
		switch c.GetCharacterId() {
		case hidden.GetId():
			hiddenCombatant = c.GetId()
		case wizard.GetId():
			casterCombatant = c.GetId()
		}
		if c.GetKind() == playv1.CombatantKind_COMBATANT_KIND_PLAYER {
			must(m.combat.SubmitInitiative(ctx, rq(&playv1.SubmitInitiativeRequest{
				CampaignId: w.campaign, EncounterId: e.GetId(), CombatantId: c.GetId(), IdempotencyKey: newKey(),
				Roll: &playv1.SubmitInitiativeRequest_D20Face{D20Face: 12},
			})))
		}
	}
	place := func(characterID string, col, row int32) {
		for _, c := range e.GetCombatants() {
			if c.GetCharacterId() == characterID {
				must(m.combat.MoveCombatant(ctx, rq(&playv1.MoveCombatantRequest{CampaignId: w.campaign, EncounterId: e.GetId(), CombatantId: c.GetId(), IdempotencyKey: newKey(), Col: col, Row: row})))
			}
		}
	}
	place(wizard.GetId(), 3, 5)
	place(toren.GetId(), 2, 6)
	place(seen.GetId(), 10, 5)
	place(hidden.GetId(), 11, 5)
	e = must(m.combat.BeginCombat(ctx, rq(&playv1.BeginCombatRequest{CampaignId: w.campaign, EncounterId: e.GetId(), IdempotencyKey: newKey()}))).GetEncounter()
	for range 20 { // until the wizard's turn
		cur := must(m.combat.GetEncounter(ctx, rq(&playv1.GetEncounterRequest{CampaignId: w.campaign}))).GetEncounter()
		if cur.GetCurrentCombatantId() == casterCombatant {
			break
		}
		must(m.combat.EndTurn(ctx, rq(&playv1.EndTurnRequest{CampaignId: w.campaign, EncounterId: e.GetId(), IdempotencyKey: newKey(), ExpectedCombatantId: cur.GetCurrentCombatantId(), ExpectedRound: cur.GetRound()})))
	}
	leak := areaLeak{name: hiddenName, characterID: hidden.GetId(), combatantID: hiddenCombatant}

	watchers := map[*person]*streamWatcher{m: w.watchStream(m), w.ana: w.watchStream(w.ana), w.caio: w.watchStream(w.caio)}
	players := []*person{w.ana, w.caio}

	// What each player reads of the combat, as text.
	readAll := func(p *person) string {
		var b strings.Builder
		b.WriteString(jsonOf(must(p.combat.GetEncounter(ctx, rq(&playv1.GetEncounterRequest{CampaignId: w.campaign})))))
		b.WriteString(jsonOf(must(p.combat.ListCombatLog(ctx, rq(&playv1.ListCombatLogRequest{CampaignId: w.campaign, EncounterId: e.GetId()})))))
		if p == w.ana {
			b.WriteString(jsonOf(must(p.combat.GetTurnOptions(ctx, rq(&playv1.GetTurnOptionsRequest{CampaignId: w.campaign, EncounterId: e.GetId(), CombatantId: casterCombatant})))))
		}
		return b.String()
	}
	area := func() *playv1.PreviewSpellAreaRequest {
		return &playv1.PreviewSpellAreaRequest{
			CampaignId: w.campaign, EncounterId: e.GetId(), CasterId: casterCombatant, SpellKey: "spell:fireball", Slot: &playv1.SpellSlot{Level: 3},
			Area: &playv1.PreviewSpellAreaRequest_Origin{Origin: &playv1.SpellOrigin{Col: 10, Row: 5}},
		}
	}

	// Before the cast: the preview names the visible creature and not the hidden one.
	preview := jsonOf(must(w.ana.combat.PreviewSpellArea(ctx, rq(area()))))
	if found := namesAny(preview, leak.needles()); len(found) > 0 {
		t.Errorf("the player's preview names %v", found)
	}
	if !strings.Contains(preview, seenName) {
		t.Errorf("the player's preview does not list the visible creature, so it cannot prove anything: %s", preview)
	}
	masterPreview := jsonOf(must(m.combat.PreviewSpellArea(ctx, rq(area()))))
	if !strings.Contains(masterPreview, hiddenCombatant) {
		t.Fatalf("the master's preview does not list the hidden creature (the positive control): %s", masterPreview)
	}

	cast := must(w.ana.combat.CastSpell(ctx, rq(&playv1.CastSpellRequest{
		CampaignId: w.campaign, EncounterId: e.GetId(), CasterId: casterCombatant, SpellKey: "spell:fireball", Slot: &playv1.SpellSlot{Level: 3}, IdempotencyKey: newKey(),
		Area: &playv1.CastSpellRequest_Origin{Origin: &playv1.SpellOrigin{Col: 10, Row: 5}},
	})))
	pending := must(m.combat.GetEncounter(ctx, rq(&playv1.GetEncounterRequest{CampaignId: w.campaign}))).GetEncounter().GetPendingHiddenReveals()
	if len(pending) != 1 {
		t.Fatalf("the master has %d questions, want 1 (the positive control: the spell hit a hidden creature)", len(pending))
	}
	leak.question = pending[0].GetId()

	if found := namesAny(jsonOf(cast), leak.needles()); len(found) > 0 {
		t.Errorf("the cast's answer to the player names %v: %s", found, jsonOf(cast))
	}
	for _, p := range players {
		if found := namesAny(readAll(p), leak.needles()); len(found) > 0 {
			t.Errorf("%s reads %v before the reveal", p.name, found)
		}
	}
	masterText := jsonOf(must(m.combat.GetEncounter(ctx, rq(&playv1.GetEncounterRequest{CampaignId: w.campaign}))))
	if !strings.Contains(masterText, leak.combatantID) || !strings.Contains(masterText, leak.question) {
		t.Errorf("the master's combat does not name the hidden creature and the question, so the scan proves nothing")
	}

	// A player who answers the question gets the same refusal for the real id and for any other.
	answer := func(id string) error {
		_, err := w.ana.combat.ResolveHiddenReveal(ctx, rq(&playv1.ResolveHiddenRevealRequest{CampaignId: w.campaign, EncounterId: e.GetId(), PendingRevealId: id, Reveal: true, IdempotencyKey: newKey()}))
		return err
	}
	errReal, errFake := answer(leak.question), answer(newKey())
	if errReal == nil || errFake == nil || connect.CodeOf(errReal) != connect.CodePermissionDenied || errReal.Error() != errFake.Error() || connect.CodeOf(errFake) != connect.CodeOf(errReal) {
		t.Errorf("a player's answer: real id %v, other id %v; want the same permission_denied", errReal, errFake)
	}

	// The master reveals it: the creature is on the players' map, and the line of the cast
	// still does not say it was hit.
	must(m.combat.ResolveHiddenReveal(ctx, rq(&playv1.ResolveHiddenRevealRequest{CampaignId: w.campaign, EncounterId: e.GetId(), PendingRevealId: leak.question, Reveal: true, IdempotencyKey: newKey()})))
	after := must(w.ana.combat.GetEncounter(ctx, rq(&playv1.GetEncounterRequest{CampaignId: w.campaign}))).GetEncounter()
	if !strings.Contains(jsonOf(after), leak.combatantID) {
		t.Errorf("the revealed creature is not in the player's combat")
	}
	for _, p := range players {
		log := must(p.combat.ListCombatLog(ctx, rq(&playv1.ListCombatLogRequest{CampaignId: w.campaign, EncounterId: e.GetId()})))
		for _, r := range log.GetRounds() {
			for _, en := range r.GetEntries() {
				if en.GetKind() == playv1.CombatLogKind_COMBAT_LOG_KIND_SPELL_CAST && strings.Contains(jsonOf(en), leak.combatantID) {
					t.Errorf("%s's line of the cast names the creature it hit after the reveal: %s", p.name, jsonOf(en))
				}
			}
		}
	}
	if strings.Contains(jsonOf(cast), leak.combatantID) {
		t.Error("the cast's answer names the creature")
	}

	// The stream: the players heard nothing that names it, and nothing of the question.
	must(m.play.EndGameSession(ctx, rq(&playv1.EndGameSessionRequest{CampaignId: w.campaign, GameSessionId: w.session})))
	for p, sw := range watchers {
		select {
		case <-sw.done:
		case <-time.After(30 * time.Second):
			t.Fatalf("%s's stream did not end", p.name)
		}
		var pendingEvents int
		for _, ev := range sw.events {
			if eventCase(ev) == "hidden_hit_pending" {
				pendingEvents++
			}
			if p == m {
				continue
			}
			if found := namesAny(string(mustJSON(ev)), leak.needles()); len(found) > 0 {
				t.Errorf("%s's stream carries %v: %s", p.name, found, mustJSON(ev))
			}
		}
		switch {
		case p == m && pendingEvents != 1:
			t.Errorf("the master heard %d questions, want 1 (the positive control)", pendingEvents)
		case p != m && pendingEvents != 0:
			t.Errorf("%s heard of a question", p.name)
		}
	}
}
