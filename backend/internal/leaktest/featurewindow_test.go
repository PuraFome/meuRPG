package leaktest

import (
	"bytes"
	"strings"
	"testing"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	mapsv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/maps/v1"
	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1/playv1connect"
	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
)

// The window of a reaction feature (#743, RN-10): Toren, Caio's fighter, has Halo, a reaction that waits for an enemy to start its
// turn or to move within 10 ft of him. When the creature is hidden, the window is the master's alone: Caio, whose character it is,
// and Ana read nothing of it in the combat, the log, the turn options or the stream, and neither can answer it. The master reads
// it (the positive control: the needles are reachable). One test for each trigger.

const (
	haloRangeFt = 10
	hiddenNear  = 5 // squares: 25 ft off Toren's, a step of the move
)

// haloFight builds a combat of Toren (Halo, waiting for the kinds), Pensantus and a hidden goblin at the square, with the
// goblin's turn the next to come. It returns the goblin's combatant.
func (w *world) haloFight(kinds []rulesv1.FeatureTriggerKind, goblinAt [2]int32) *playv1.Combatant {
	w.t.Helper()
	return w.haloFightOf(kinds, goblinAt, true)
}

// haloFightOf is haloFight with the goblin hidden or in view.
func (w *world) haloFightOf(kinds []rulesv1.FeatureTriggerKind, goblinAt [2]int32, hidden bool) *playv1.Combatant {
	w.t.Helper()
	halo := &rulesv1.FeatureUse{
		LimitScope: rulesv1.UseScope_USE_SCOPE_COMBAT, LimitUses: "2",
		Targets: &rulesv1.FeatureTargets{Min: 1, Max: 1, Who: rulesv1.FeatureTargetWho_FEATURE_TARGET_WHO_ENEMY, RangeFt: haloRangeFt},
		Trigger: &rulesv1.FeatureTrigger{Kinds: kinds, RangeFt: haloRangeFt},
	}
	return w.haloFightWith(halo, goblinAt, hidden)
}

// haloFightWith is haloFightOf with the use Toren's reaction declares.
func (w *world) haloFightWith(halo *rulesv1.FeatureUse, goblinAt [2]int32, hidden bool) *playv1.Combatant {
	w.t.Helper()
	ctx := w.t.Context()
	m := w.master
	w.imgMap = w.image("halo-map-image", 240, 160)
	w.fogMap = must(m.maps.CreateMap(ctx, rq(&mapsv1.CreateMapRequest{CampaignId: w.campaign, Name: w.secrets.public("halo-map-name"), ImageId: w.imgMap}))).GetMap().GetId()
	must(m.maps.SetMapRevealed(ctx, rq(&mapsv1.SetMapRevealedRequest{CampaignId: w.campaign, MapId: w.fogMap, Revealed: true})))
	must(m.maps.SetMapGrid(ctx, rq(&mapsv1.SetMapGridRequest{CampaignId: w.campaign, MapId: w.fogMap, Columns: 24})))
	w.map2 = must(m.maps.CreateMap(ctx, rq(&mapsv1.CreateMapRequest{CampaignId: w.campaign, Name: w.secrets.marker("halo-map2-name"), ImageId: w.image("halo-map2-image", 60, 40)}))).GetMap().GetId()
	w.session = must(m.play.StartGameSession(ctx, rq(&playv1.StartGameSessionRequest{CampaignId: w.campaign}))).GetGameSession().GetId()
	must(m.play.SetCurrentMap(ctx, rq(&playv1.SetCurrentMapRequest{CampaignId: w.campaign, MapId: w.fogMap})))

	sub := must(m.table.CreateTableEntry(ctx, rq(&rulesv1.CreateTableEntryRequest{
		CampaignId: w.campaign, Body: &rulesv1.CreateTableEntryRequest_TableSubclass{TableSubclass: &rulesv1.TableSubclass{
			NamePt: "Guardião do Halo", ClassKey: "class:fighter", DescPt: []string{"Um guardião."},
			Levels: []*rulesv1.TableSubclassLevel{{Level: 3, Features: []*rulesv1.TableFeature{{NamePt: "Halo de Guarda", DescPt: []string{"Uma luz."}, Effects: []*rulesv1.TableEffect{{
				Type: "grant_action", Economy: "reaction", TextPt: "Uma luz.", Use: halo,
			}}}}}},
		}},
	}))).GetEntry()
	must(w.master.characters.UpdateCharacter(ctx, rq(&charactersv1.UpdateCharacterRequest{
		CampaignId: w.campaign, CharacterId: w.toren.GetId(), Revision: w.toren.GetRevision(), Name: w.toren.GetName(),
		Sheet: &charactersv1.CharacterSheet{Content: &charactersv1.CharacterSheet_Full{Full: &charactersv1.FullSheet{
			BaseScores: &rulesv1.AbilityScores{Strength: 16, Dexterity: 13, Constitution: 14, Intelligence: 10, Wisdom: 10, Charisma: 16},
			RaceKey:    "race:human", Classes: []*charactersv1.ClassLevel{{ClassKey: "class:fighter", Level: 5, Subclass: &charactersv1.ClassLevel_SubclassKey{SubclassKey: sub.GetKey()}}},
			WeaponKeys: []string{"equipment:longsword"},
		}}},
	})))
	goblin, _ := w.npc("halo-goblin", &charactersv1.BasicSheet{HitPointsMax: 20, ArmorClass: 12, SpeedFt: 30})
	e := must(m.combat.StartEncounter(ctx, rq(&playv1.StartEncounterRequest{
		CampaignId: w.campaign, IdempotencyKey: newKey(), Name: w.secrets.public("halo-encounter"),
		Participants: []*playv1.Participant{{CharacterId: goblin.GetId(), Hidden: &hidden}},
	}))).GetEncounter()
	for _, c := range e.GetCombatants() {
		if c.GetKind() == playv1.CombatantKind_COMBATANT_KIND_PLAYER {
			must(m.combat.SubmitInitiative(ctx, rq(&playv1.SubmitInitiativeRequest{
				CampaignId: w.campaign, EncounterId: e.GetId(), CombatantId: c.GetId(), IdempotencyKey: newKey(),
				Roll: &playv1.SubmitInitiativeRequest_D20Face{D20Face: 12},
			})))
		}
	}
	gob := w.combatantOf(e, goblin.GetId())
	must(m.combat.SubmitInitiative(ctx, rq(&playv1.SubmitInitiativeRequest{
		CampaignId: w.campaign, EncounterId: e.GetId(), CombatantId: gob.GetId(), IdempotencyKey: newKey(),
		Roll: &playv1.SubmitInitiativeRequest_D20Face{D20Face: 1},
	})))
	w.placeForced(m, e.GetId(),
		placement{w.combatantOf(e, w.toren.GetId()).GetId(), 3, 3}, placement{w.combatantOf(e, w.pens.GetId()).GetId(), 3, 6}, placement{gob.GetId(), goblinAt[0], goblinAt[1]})
	w.encounter = must(m.combat.BeginCombat(ctx, rq(&playv1.BeginCombatRequest{CampaignId: w.campaign, EncounterId: e.GetId(), IdempotencyKey: newKey()}))).GetEncounter()
	return w.combatantOf(w.encounter, goblin.GetId())
}

// passToGoblin ends the turns until the hidden goblin's turn starts.
func (w *world) passToGoblin(goblin *playv1.Combatant) {
	w.t.Helper()
	for range 12 {
		cur := w.master.currentEncounter(w)
		if cur.GetCurrentCombatantId() == goblin.GetId() {
			w.encounter = cur
			return
		}
		must(w.master.combat.EndTurn(w.t.Context(), rq(&playv1.EndTurnRequest{
			CampaignId: w.campaign, EncounterId: w.encounter.GetId(), IdempotencyKey: newKey(), ExpectedCombatantId: cur.GetCurrentCombatantId(), ExpectedRound: cur.GetRound(),
		})))
	}
	w.t.Fatal("the goblin's turn never came")
}

// haloNeedles are what no player reads of the hidden creature's window: the window and its kind, and the creature's name.
func (w *world) haloNeedles(windowID string) {
	w.secrets.add(&canary{needle: windowID, kind: "feature-window-id"})
	w.secrets.add(&canary{needle: "REACTION_KIND_FEATURE", kind: "feature-window-kind"})
	w.secrets.add(&canary{needle: "FEATURE_TRIGGER_KIND_", kind: "feature-window-trigger"})
	w.secrets.add(&canary{needle: "feature_trigger", kind: "feature-window-trigger-json"})
}

// haloReads are what a player reads of the combat: the combat, its log and the options of the character.
func (w *world) haloReads(p *person) []reply {
	w.t.Helper()
	var out []reply
	out = append(out, w.reads(p)...)
	for _, c := range []*charactersv1.Character{w.toren, w.pens} {
		id := w.combatant(c).GetId()
		out = append(out, p.call(playv1connect.CombatServiceGetTurnOptionsProcedure, &playv1.GetTurnOptionsRequest{CampaignId: w.campaign, EncounterId: w.encounter.GetId(), CombatantId: id}))
	}
	return out
}

// requireNothingOfTheWindow checks what Caio and Ana read, and that neither can answer the window as if it were there.
func (w *world) requireNothingOfTheWindow(t *testing.T, window *playv1.ReactionWindow) {
	t.Helper()
	for _, p := range []*person{w.ana, w.caio} {
		for _, r := range w.haloReads(p) {
			if r.status != 200 && !strings.Contains(string(r.body), "not_found") {
				continue // a refusal of a read that is not the player's (Ana's options of Toren) holds nothing of it either
			}
			for _, f := range w.inspect(p, r, nil) {
				t.Errorf("%s: %s", p.name, f)
			}
		}
		if enc := p.currentEncounter(w); len(enc.GetReactionWindows()) != 0 {
			t.Errorf("%s reads the windows %v, want none: the window is the master's", p.name, enc.GetReactionWindows())
		}
	}
	asked := w.caio.call(playv1connect.CombatServiceAnswerReactionProcedure, &playv1.AnswerReactionRequest{
		CampaignId: w.campaign, EncounterId: w.encounter.GetId(), WindowId: window.GetId(), Answer: playv1.ReactionChoice_REACTION_CHOICE_PASS, IdempotencyKey: newKey(),
	})
	fake := w.caio.call(playv1connect.CombatServiceAnswerReactionProcedure, &playv1.AnswerReactionRequest{
		CampaignId: w.campaign, EncounterId: w.encounter.GetId(), WindowId: newKey(), Answer: playv1.ReactionChoice_REACTION_CHOICE_PASS, IdempotencyKey: newKey(),
	})
	if asked.ok() || fake.ok() || asked.status != fake.status || !bytes.Equal(asked.body, fake.body) {
		t.Errorf("Caio's answer to the real window: %d %s; to an invented one: %d %s; they must read the same", asked.status, asked.body, fake.status, fake.body)
	}
}

// windowCounts counts the openings and the closings of windows a stream heard.
func windowCounts(lines []string) (opened, closed int) {
	for _, ev := range lines {
		switch {
		case strings.HasPrefix(ev, "reaction_window_opened"):
			opened++
		case strings.HasPrefix(ev, "reaction_window_closed"):
			closed++
		}
	}
	return opened, closed
}

// requireWindowHeardByTheMasterAlone checks the streams of the hidden part: the master heard the window open, the players did not,
// and no line a player heard holds a needle.
func (w *world) requireWindowHeardByTheMasterAlone(t *testing.T, got map[string][]string, window *playv1.ReactionWindow) {
	t.Helper()
	if o, c := windowCounts(got[w.master.name]); o != 1 || c != 1 {
		t.Errorf("the master heard %d openings and %d closings of a window, want 1 and 1 (%v)", o, c, got[w.master.name])
	}
	for _, p := range []*person{w.ana, w.caio} {
		if o, c := windowCounts(got[p.name]); o != 0 || c != 0 {
			t.Errorf("%s heard %d openings and %d closings of a hidden creature's window", p.name, o, c)
		}
		for _, line := range got[p.name] {
			if strings.Contains(line, window.GetId()) || strings.Contains(line, "REACTION_KIND_FEATURE") {
				t.Errorf("%s heard the window: %s", p.name, line)
			}
		}
	}
}

// haloWorld is the mini-world of the two tests: the people, Toren and Pensantus.
func haloWorld(t *testing.T) *world {
	t.Helper()
	w := &world{stack: newStack(t), secrets: newSecrets(), pts: map[string]*mapsv1.MapPoint{}, puzzles: map[string]*playv1.Puzzle{}, notes: map[string]string{}}
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("building the fixture: %v", r)
		}
	}()
	w.buildPeople()
	w.pens = w.pc(w.ana, "Pensantus", "race:gnome")
	w.toren = w.pc(w.caio, "Toren", "race:human")
	return w
}

// thenTheMasterAnswers checks what the players read of the window, has the master answer it and checks again.
func (w *world) thenTheMasterAnswers(t *testing.T, goblin *playv1.Combatant, answer playv1.ReactionChoice) *playv1.ReactionWindow {
	t.Helper()
	window := windowOfKind(w.master, w, playv1.ReactionKind_REACTION_KIND_FEATURE)
	if window == nil || window.GetTrigger().GetActorId() != goblin.GetId() {
		t.Fatalf("the master reads %v, want the Halo window naming the goblin", window)
	}
	w.haloNeedles(window.GetId())
	// The positive control: the master reads the window and its kind, so the needles are reachable.
	if body := w.master.call(playv1connect.CombatServiceGetEncounterProcedure, &playv1.GetEncounterRequest{CampaignId: w.campaign}).body; !strings.Contains(string(body), window.GetId()) || !strings.Contains(string(body), "REACTION_KIND_FEATURE") {
		t.Error("the master's read does not hold the window: the needles are unreachable")
	}
	w.requireNothingOfTheWindow(t, window)
	must(w.master.combat.AnswerReaction(t.Context(), rq(&playv1.AnswerReactionRequest{
		CampaignId: w.campaign, EncounterId: w.encounter.GetId(), WindowId: window.GetId(), Answer: answer, IdempotencyKey: newKey(),
	})))
	w.requireNothingOfTheWindow(t, window)
	return window
}

// A hidden goblin starts its turn 10 ft from Toren: the window is the master's alone.
func TestAHiddenCreatureStartingItsTurnNearOpensTheFeatureWindowForTheMasterAlone(t *testing.T) {
	w := haloWorld(t)
	goblin := w.haloFight([]rulesv1.FeatureTriggerKind{rulesv1.FeatureTriggerKind_FEATURE_TRIGGER_KIND_ENEMY_STARTS_TURN_WITHIN}, [2]int32{5, 3})
	if windowOfKind(w.master, w, playv1.ReactionKind_REACTION_KIND_FEATURE) != nil {
		t.Fatal("a window is open before the goblin's turn")
	}
	var window *playv1.ReactionWindow
	got := w.hiddenOnlyEvents(func() {
		w.passToGoblin(goblin)
		window = w.thenTheMasterAnswers(t, goblin, playv1.ReactionChoice_REACTION_CHOICE_USE)
	})
	w.requireWindowHeardByTheMasterAlone(t, got, window)
}

// A hidden goblin moves from 35 ft to 10 ft of Toren on its turn: the window is the master's alone.
func TestAHiddenCreatureMovingNearOpensTheFeatureWindowForTheMasterAlone(t *testing.T) {
	w := haloWorld(t)
	goblin := w.haloFight([]rulesv1.FeatureTriggerKind{rulesv1.FeatureTriggerKind_FEATURE_TRIGGER_KIND_ENEMY_MOVES_WITHIN}, [2]int32{10, 3})
	w.passToGoblin(goblin)
	if windowOfKind(w.master, w, playv1.ReactionKind_REACTION_KIND_FEATURE) != nil {
		t.Fatal("a window is open before the goblin moves")
	}
	var window *playv1.ReactionWindow
	got := w.hiddenOnlyEvents(func() {
		must(w.master.combat.MoveCombatant(t.Context(), rq(&playv1.MoveCombatantRequest{
			CampaignId: w.campaign, EncounterId: w.encounter.GetId(), CombatantId: goblin.GetId(), IdempotencyKey: newKey(), Col: 5, Row: 3,
		})))
		window = w.thenTheMasterAnswers(t, goblin, playv1.ReactionChoice_REACTION_CHOICE_PASS)
	})
	w.requireWindowHeardByTheMasterAlone(t, got, window)
}

// A goblin in view starts its turn 10 ft from Toren and Caio, his player, reads the window naming it. The master then hides the
// goblin while the window waits: from then on Caio and Ana read nothing of the goblin (its name, the distance) or of the window, and
// Caio cannot answer it.
func TestHidingTheCreatureOfAnOpenFeatureWindowTakesItFromThePlayer(t *testing.T) {
	w := haloWorld(t)
	goblin := w.haloFightOf([]rulesv1.FeatureTriggerKind{rulesv1.FeatureTriggerKind_FEATURE_TRIGGER_KIND_ENEMY_STARTS_TURN_WITHIN}, [2]int32{5, 3}, false)
	w.passToGoblin(goblin)
	window := windowOfKind(w.caio, w, playv1.ReactionKind_REACTION_KIND_FEATURE)
	if window == nil || window.GetTrigger().GetActorId() != goblin.GetId() {
		t.Fatalf("Caio reads %v, want the Halo window naming the goblin in view", window)
	}
	if body := w.caio.call(playv1connect.CombatServiceGetEncounterProcedure, &playv1.GetEncounterRequest{CampaignId: w.campaign}).body; !strings.Contains(string(body), "distanceFt") {
		t.Fatal("Caio's read does not hold the distance: the needle is unreachable")
	}
	hidden := true
	must(w.master.combat.SetCombatantHidden(t.Context(), rq(&playv1.SetCombatantHiddenRequest{
		CampaignId: w.campaign, EncounterId: w.encounter.GetId(), CombatantId: goblin.GetId(), IdempotencyKey: newKey(), Hidden: hidden,
	})))
	w.haloNeedles(window.GetId())
	w.secrets.add(&canary{needle: `"distanceFt":10`, kind: "feature-window-distance"})
	for _, p := range []*person{w.ana, w.caio} {
		for _, r := range w.haloReads(p) {
			if r.status != 200 && !strings.Contains(string(r.body), "not_found") {
				continue
			}
			for _, f := range w.inspect(p, r, nil) {
				t.Errorf("%s: %s", p.name, f)
			}
		}
		if enc := p.currentEncounter(w); len(enc.GetReactionWindows()) != 0 {
			t.Errorf("%s reads the windows %v after the hide, want none", p.name, enc.GetReactionWindows())
		}
	}
	// The window is closed: the answer Caio read the id of is refused, and the refusal says nothing of the goblin.
	asked := w.caio.call(playv1connect.CombatServiceAnswerReactionProcedure, &playv1.AnswerReactionRequest{
		CampaignId: w.campaign, EncounterId: w.encounter.GetId(), WindowId: window.GetId(), Answer: playv1.ReactionChoice_REACTION_CHOICE_USE, IdempotencyKey: newKey(),
	})
	if asked.ok() {
		t.Errorf("Caio answered the window of a creature the master hid: %d %s", asked.status, asked.body)
	}
	for _, f := range w.inspect(w.caio, asked, nil) {
		t.Errorf("Caio's refusal: %s", f)
	}
}

// Toren's reaction here is aimed at no creature (it gives him temporary hit points), so no hidden target makes its event secret:
// the master answers the hidden goblin's window with USE and no player reads a line of the use, or the kind of the line. The master reads it (the positive control).
func TestAnUnaimedFeatureUsedOnAHiddenCreaturesWindowLeavesNoLineForThePlayers(t *testing.T) {
	w := haloWorld(t)
	glow := &rulesv1.FeatureUse{
		LimitScope: rulesv1.UseScope_USE_SCOPE_COMBAT, LimitUses: "2", Applies: &rulesv1.FeatureApplies{TempHp: "4"},
		Trigger: &rulesv1.FeatureTrigger{Kinds: []rulesv1.FeatureTriggerKind{rulesv1.FeatureTriggerKind_FEATURE_TRIGGER_KIND_ENEMY_STARTS_TURN_WITHIN}, RangeFt: haloRangeFt},
	}
	goblin := w.haloFightWith(glow, [2]int32{5, 3}, true)
	var window *playv1.ReactionWindow
	got := w.hiddenOnlyEvents(func() {
		w.passToGoblin(goblin)
		window = w.thenTheMasterAnswers(t, goblin, playv1.ReactionChoice_REACTION_CHOICE_USE)
		w.secrets.add(&canary{needle: "COMBAT_LOG_KIND_FEATURE_USED", kind: "feature-use-line-kind"})
		w.secrets.add(&canary{needle: "featureUsed", kind: "feature-use-line-body"})
		if body := w.master.call(playv1connect.CombatServiceListCombatLogProcedure, &playv1.ListCombatLogRequest{CampaignId: w.campaign, EncounterId: w.encounter.GetId()}).body; !strings.Contains(string(body), "COMBAT_LOG_KIND_FEATURE_USED") || !strings.Contains(string(body), "featureUsed") {
			t.Fatalf("the master's log does not hold the line of the use: the needles are unreachable (%s)", body)
		}
		for _, p := range []*person{w.ana, w.caio} {
			for _, r := range w.reads(p) {
				for _, f := range w.inspect(p, r, nil) {
					t.Errorf("%s: %s", p.name, f)
				}
			}
		}
	})
	w.requireWindowHeardByTheMasterAlone(t, got, window)
}
