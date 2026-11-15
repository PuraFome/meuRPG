package leaktest

import (
	"testing"

	"google.golang.org/protobuf/proto"

	mapsv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/maps/v1"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	"github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1/charactersv1connect"
	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1/playv1connect"
)

// The use of a consumable outside a combat is the master's and the owner's: what a potion was, that it was used and the
// resistance it left are in the answers of nobody else. (Its line in a combat's log is the user's and the receiver's owner's
// and the master's, as the charges' lines are.)
var consumableWords = []string{"item:potion-of-resistance-fire", "Resistência a fogo", "consumable_used", "COMBAT_LOG_KIND_CONSUMABLE_USED", `"consumableUse"`}

// anaHolds puts the potion on Pensantus's sheet as the master and returns the entry's id.
func (w *world) anaHolds(key string) string { return w.holds(w.pens, key) }

// holds puts the potion on the character's sheet as the master and returns the entry's id.
func (w *world) holds(c *charactersv1.Character, key string) string {
	w.t.Helper()
	ctx := w.t.Context()
	cur := must(w.master.characters.GetCharacter(ctx, rq(&charactersv1.GetCharacterRequest{CampaignId: w.campaign, CharacterId: c.GetId()}))).GetCharacter()
	sheet := proto.CloneOf(cur.GetSheet())
	sheet.GetFull().MagicItems = append(sheet.GetFull().MagicItems, &charactersv1.SheetMagicItem{Key: key})
	saved := must(w.master.characters.UpdateCharacter(ctx, rq(&charactersv1.UpdateCharacterRequest{
		CampaignId: w.campaign, CharacterId: c.GetId(), Revision: cur.GetRevision(), Name: cur.GetName(), Sheet: sheet,
	}))).GetCharacter()
	for _, it := range saved.GetSheet().GetFull().GetMagicItems() {
		if it.GetKey() == key {
			return it.GetId()
		}
	}
	w.t.Fatalf("%s is not on the sheet", key)
	return ""
}

// consumableReads are the answers that could hold the use: the live session, Pensantus's sheet and the effects on characters.
// sessionReads are the reads of the session that could hold the use: the live session, the messages and the free rolls.
func (w *world) sessionReads(p *person) []reply {
	return []reply{
		p.call(playv1connect.PlayServiceGetLiveSessionProcedure, &playv1.GetLiveSessionRequest{CampaignId: w.campaign}),
		p.call(playv1connect.PlayServiceListSessionMessagesProcedure, &playv1.ListSessionMessagesRequest{CampaignId: w.campaign}),
		p.call(playv1connect.PlayServiceListFreeRollsProcedure, &playv1.ListFreeRollsRequest{CampaignId: w.campaign}),
	}
}

func (w *world) consumableReads(p *person) []reply {
	w.t.Helper()
	return append(w.sessionReads(p),
		p.call(charactersv1connect.CharacterServiceGetCharacterProcedure, &charactersv1.GetCharacterRequest{CampaignId: w.campaign, CharacterId: w.pens.GetId()}),
		p.call(playv1connect.LastingEffectServiceListCharacterEffectsProcedure, &playv1.ListCharacterEffectsRequest{CampaignId: w.campaign}),
	)
}

func TestAConsumableUsedOutsideACombatIsTheOwnersAndTheMastersAlone(t *testing.T) {
	w := newWorld(t)
	must(w.master.combat.EndEncounter(t.Context(), rq(&playv1.EndEncounterRequest{CampaignId: w.campaign, EncounterId: w.encounter.GetId(), IdempotencyKey: newKey()})))
	entry := w.anaHolds("item:potion-of-resistance-fire")
	// Pensantus fell in the world's combat: an unconscious creature does not act, so she is up again to drink.
	must(w.master.play.AdjustCharacterVitals(t.Context(), rq(&playv1.AdjustCharacterVitalsRequest{
		CampaignId: w.campaign, CharacterId: w.pens.GetId(), IdempotencyKey: newKey(), ExpectedRevision: w.master.revisionOf(w, w.pens.GetId()), HitPointsCurrent: proto.Int32(3),
	})))
	for _, n := range consumableWords {
		w.secrets.add(&canary{needle: n, kind: "consumable", readers: map[string]bool{w.ana.name: true}})
	}
	if other := w.caio.call(playv1connect.ChargesServiceUseConsumableProcedure, &playv1.UseConsumableRequest{
		CampaignId: w.campaign, CharacterId: w.pens.GetId(), EntryId: entry, IdempotencyKey: newKey(), RollInApp: true,
	}); other.ok() {
		t.Error("another player used Ana's potion")
	}
	res := w.ana.call(playv1connect.ChargesServiceUseConsumableProcedure, &playv1.UseConsumableRequest{
		CampaignId: w.campaign, CharacterId: w.pens.GetId(), EntryId: entry, IdempotencyKey: newKey(), RollInApp: true,
	})
	if !res.ok() {
		t.Fatalf("Ana's use: status %d %s", res.status, res.body)
	}
	if !contains(res.body, "damage-type:fire") {
		t.Errorf("the owner's answer holds no resistance: %s", res.body)
	}
	for _, p := range []*person{w.caio, w.pending, w.stranger} {
		for _, r := range w.consumableReads(p) {
			for _, f := range w.inspect(p, r, nil) {
				t.Errorf("%s: %s", p.name, f)
			}
		}
	}
	found := false
	for _, r := range w.consumableReads(w.master) {
		for _, n := range consumableWords {
			found = found || contains(r.body, n)
		}
	}
	if !found {
		t.Error("the master's reads hold no needle: the canary is unreachable")
	}
}

// In a combat the table reads that Toren gave a potion to Pensantus, with its name; the dice and the hit points it gave back are
// the master's and the two owners' (RN-10): a third player reads the line and none of its numbers.
func TestAPotionGivenInACombatShowsAThirdPlayerTheLineAndNoNumbers(t *testing.T) {
	w := &world{stack: newStack(t), secrets: newSecrets(), pts: map[string]*mapsv1.MapPoint{}, puzzles: map[string]*playv1.Puzzle{}, notes: map[string]string{}}
	ctx := t.Context()
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("building the fixture: %v", r)
		}
	}()
	w.buildPeople()
	m := w.master
	w.session = must(m.play.StartGameSession(ctx, rq(&playv1.StartGameSessionRequest{CampaignId: w.campaign}))).GetGameSession().GetId()
	w.pens, w.toren = w.pc(w.ana, "Pensantus", "race:gnome"), w.pc(w.caio, "Toren", "race:human")
	dani := w.newPerson("Dani")
	w.join(false, dani)
	entry := w.holds(w.toren, "item:potion-of-healing-common")
	must(m.play.AdjustCharacterVitals(ctx, rq(&playv1.AdjustCharacterVitalsRequest{
		CampaignId: w.campaign, CharacterId: w.pens.GetId(), IdempotencyKey: newKey(), ExpectedRevision: m.revisionOf(w, w.pens.GetId()), HitPointsCurrent: proto.Int32(0),
	})))
	goblin := must(m.characters.CreateNpcFromCreature(ctx, rq(&charactersv1.CreateNpcFromCreatureRequest{
		CampaignId: w.campaign, CreatureKey: "monster:goblin", Name: "Goblin", Kind: charactersv1.CharacterKind_CHARACTER_KIND_MINION, IdempotencyKey: newKey(),
	}))).GetCharacter()
	e := must(m.combat.StartEncounter(ctx, rq(&playv1.StartEncounterRequest{
		CampaignId: w.campaign, IdempotencyKey: newKey(), Name: "Emboscada", Mode: playv1.EncounterMode_ENCOUNTER_MODE_THEATRE,
		Participants: []*playv1.Participant{{CharacterId: goblin.GetId()}},
	}))).GetEncounter()
	for _, c := range e.GetCombatants() {
		face := int32(10)
		if c.GetCharacterId() == w.toren.GetId() {
			face = 18
		}
		must(m.combat.SubmitInitiative(ctx, rq(&playv1.SubmitInitiativeRequest{
			CampaignId: w.campaign, EncounterId: e.GetId(), CombatantId: c.GetId(), IdempotencyKey: newKey(), Roll: &playv1.SubmitInitiativeRequest_D20Face{D20Face: face},
		})))
	}
	must(m.combat.BeginCombat(ctx, rq(&playv1.BeginCombatRequest{CampaignId: w.campaign, EncounterId: e.GetId(), IdempotencyKey: newKey()})))
	res := must(w.caio.charges.UseConsumable(ctx, rq(&playv1.UseConsumableRequest{
		CampaignId: w.campaign, CharacterId: w.toren.GetId(), EntryId: entry, TargetCharacterId: w.pens.GetId(), IdempotencyKey: newKey(), RollInApp: true,
	})))
	if res.GetHealed() == 0 || len(res.GetFaces()) != 2 {
		t.Fatalf("the gift = %v, want a heal rolled on two dice", res)
	}
	owners := names([]*person{w.ana, w.caio})
	for _, c := range []struct {
		kind, field string
		n           int32
	}{{"consumable-healed", "healed", res.GetHealed()}, {"consumable-roll-total", "roll_total", res.GetRollTotal()}} {
		w.secrets.add(&canary{kind: c.kind, numeric: true, value: int64(c.n), fields: []string{c.field}, readers: owners})
	}
	w.secrets.add(&canary{needle: "2d4+2", kind: "consumable-dice", readers: owners})
	logOf := func(p *person) reply {
		return p.call(playv1connect.CombatServiceListCombatLogProcedure, &playv1.ListCombatLogRequest{CampaignId: w.campaign, EncounterId: e.GetId()})
	}
	for _, p := range []*person{w.master, w.ana, w.caio} {
		if r := logOf(p); !contains(r.body, "2d4+2") || !contains(r.body, "consumableUse") {
			t.Errorf("%s reads no numbers in the line: %s", p.name, r.body)
		}
	}
	r := logOf(dani)
	if !contains(r.body, "consumableUse") {
		t.Fatalf("a third player reads no line: %s", r.body)
	}
	for _, f := range w.inspect(dani, r, nil) {
		t.Errorf("Dani: %s", f)
	}
	for _, p := range []*person{w.pending, w.stranger} {
		if got := logOf(p); got.ok() || contains(got.body, "consumableUse") {
			t.Errorf("%s read the log: %d %s", p.name, got.status, got.body)
		}
	}
}
