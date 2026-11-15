package leaktest

import (
	"testing"

	"google.golang.org/protobuf/proto"

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
func (w *world) anaHolds(key string) string {
	w.t.Helper()
	ctx := w.t.Context()
	cur := must(w.master.characters.GetCharacter(ctx, rq(&charactersv1.GetCharacterRequest{CampaignId: w.campaign, CharacterId: w.pens.GetId()}))).GetCharacter()
	sheet := proto.CloneOf(cur.GetSheet())
	sheet.GetFull().MagicItems = append(sheet.GetFull().MagicItems, &charactersv1.SheetMagicItem{Key: key})
	saved := must(w.master.characters.UpdateCharacter(ctx, rq(&charactersv1.UpdateCharacterRequest{
		CampaignId: w.campaign, CharacterId: w.pens.GetId(), Revision: cur.GetRevision(), Name: cur.GetName(), Sheet: sheet,
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
