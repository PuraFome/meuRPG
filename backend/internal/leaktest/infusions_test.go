package leaktest

import (
	"bytes"
	"os"
	"strings"
	"testing"
	"time"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	"github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1/charactersv1connect"
	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1/playv1connect"
	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
)

// The infusions a character keeps live (the name of the infusion, the object it is on, the piece it is on, the bonus it
// gives) are the owner's and the master's, as the sheet is. A player of the table reads none of it: not in the party
// views, the combat, its log after an attack with the infused weapon (the attack's result is the only thing in it), the
// session's reads or the stream. The owner's and the master's reads are the positive controls.
//
// The table is the one of the attack rolls: Toren (Caio) is the artificer, with an infusion on his dagger and another on
// an object he names; Ana plays the barbarian.

// The fixture pack's class and what the test needs of it.
const (
	artificerPack  = "../characters/testdata/pack_engenhista.json"
	artificerClass = "class:engenhista-de-ligas@mesa"

	infusionBlade = "feature:class-engenhista-de-li-6d59b9--gume-vivo@mesa"
	infusionPlate = "feature:class-engenhista-de-li-6d59b9--couraca-de-petalas@mesa"
	infusionAsh   = "feature:class-engenhista-de-li-6d59b9--manto-de-cinza@mesa"
	infusionFalse = "feature:class-engenhista-de-li-6d59b9--fundo-falso@mesa"
	infusionSeal  = "feature:class-engenhista-de-li-6d59b9--selo-de-casa@mesa"

	// The names the pack gives the blade and the seal, which the test swaps for markers.
	packBladeName, packSealName = "Gume Vivo", "Selo de Casa"

	kindInfusionName   = "infusion-name"
	kindInfusionSeal   = "infusion-seal-name"
	kindInfusionObject = "infusion-object-name"
)

// artificerPackWith is the fixture pack with the names of two of its infusions replaced.
func artificerPackWith(bladeName, sealName string) *rulesv1.TableContentPack {
	data, err := os.ReadFile(artificerPack)
	if err != nil {
		panic(err)
	}
	data = bytes.ReplaceAll(data, []byte(packBladeName), []byte(bladeName))
	data = bytes.ReplaceAll(data, []byte(packSealName), []byte(sealName))
	pack := &rulesv1.TableContentPack{}
	if err := (protojson.UnmarshalOptions{DiscardUnknown: true}).Unmarshal(data, pack); err != nil {
		panic(err)
	}
	return pack
}

// importArtificer gives the table the artificer class, with the names of the blade and the seal as the markers given.
func (w *world) importArtificer(bladeName, sealName string) {
	must(w.master.table.ImportTableContent(w.t.Context(), rq(&rulesv1.ImportTableContentRequest{
		CampaignId: w.campaign, Pack: artificerPackWith(bladeName, sealName), Mode: rulesv1.TableImportMode_TABLE_IMPORT_MODE_APPLY, IdempotencyKey: newKey(),
	})))
}

// artificerSheet is the sheet of an artificer of level 6 who knows five infusions and carries the weapons.
func (w *world) artificerSheet(weapons ...string) *charactersv1.CharacterSheet {
	sheet := w.fullSheet(artificerClass, "race:human", 6)
	full := sheet.GetFull()
	full.WeaponKeys = weapons
	full.SkillProficiencyKeys = []string{"skill:arcana", "skill:history"}
	full.FeatureChoiceKeys = []string{infusionBlade, infusionPlate, infusionAsh, infusionFalse, infusionSeal}
	return sheet
}

// infuse puts an infusion on the piece of the character's gear (or on an object, when itemKey is empty), as p.
func (w *world) infuse(p *person, characterID, infusionKey, itemKey, object string) *charactersv1.SheetInfusion {
	res := must(p.infusions.InfuseItem(w.t.Context(), rq(&charactersv1.InfuseItemRequest{
		CampaignId: w.campaign, CharacterId: characterID, InfusionKey: infusionKey, HolderCharacterId: characterID,
		ItemKey: itemKey, ObjectNamePt: object, IdempotencyKey: newKey(),
	})))
	return res.GetInfusion()
}

// infusedTable is the table of the attack rolls with Toren as the artificer.
type infusedTable struct {
	*rollsTable
	bladeName, sealName, objectName string
	blade, seal                     *charactersv1.SheetInfusion
}

// newInfusedTable builds the table: the class is the table's content, Toren is the artificer and his dagger and an object
// carry the blade and the seal. The names and the records are canaries the owner may read.
func newInfusedTable(t *testing.T) *infusedTable {
	t.Helper()
	it := &infusedTable{}
	it.rollsTable = newRollsTableWith(t, func(rt *rollsTable) {
		it.rollsTable = rt
		w := rt.world
		it.bladeName = w.secrets.marker(kindInfusionName, w.caio)
		it.sealName = w.secrets.marker(kindInfusionSeal, w.caio)
		it.objectName = w.secrets.marker(kindInfusionObject, w.caio)
		w.importArtificer(it.bladeName, it.sealName)
		w.toren = must(w.caio.characters.UpdateCharacter(w.t.Context(), rq(&charactersv1.UpdateCharacterRequest{
			CampaignId: w.campaign, CharacterId: w.toren.GetId(), Revision: w.toren.GetRevision(), Name: w.toren.GetName(), Sheet: w.artificerSheet(dagger, shortbow),
		}))).GetCharacter()
		it.blade = w.infuse(w.caio, w.toren.GetId(), infusionBlade, dagger, "")
		it.seal = w.infuse(w.caio, w.toren.GetId(), infusionSeal, "", it.objectName)
		w.secrets.id("infusion-record", it.blade.GetId(), w.caio)
		w.secrets.id("infusion-seal-record", it.seal.GetId(), w.caio)
	})
	return it
}

// buildInfusionRows adds to the table's reads the ones that show the sheet and the session.
func (it *infusedTable) buildInfusionRows() {
	it.buildRows()
	sheet := func(procedure string, req func(w *world) proto.Message) read {
		return read{procedure: procedure, label: "Toren", allow: onlyCaio, why: "the sheet is the owner's and the master's", req: req}
	}
	it.rows = append(it.rows,
		sheet(charactersv1connect.CharacterServiceGetCharacterProcedure, func(w *world) proto.Message {
			return &charactersv1.GetCharacterRequest{CampaignId: w.campaign, CharacterId: w.toren.GetId()}
		}),
		sheet(charactersv1connect.InfusionServiceListCharacterInfusionsProcedure, func(w *world) proto.Message {
			return &charactersv1.ListCharacterInfusionsRequest{CampaignId: w.campaign, CharacterId: w.toren.GetId()}
		}),
		read{
			procedure: charactersv1connect.CharacterServiceListCharactersProcedure, allow: membersAndPending, why: "the party's list: it names the characters, not their infusions",
			req: func(w *world) proto.Message { return &charactersv1.ListCharactersRequest{CampaignId: w.campaign} },
		},
		read{
			procedure: playv1connect.PlayServiceGetLiveSessionProcedure, allow: members,
			req: func(w *world) proto.Message { return &playv1.GetLiveSessionRequest{CampaignId: w.campaign} },
		},
		read{
			procedure: playv1connect.PlayServiceListGameSessionsProcedure, allow: members,
			req: func(w *world) proto.Message { return &playv1.ListGameSessionsRequest{CampaignId: w.campaign} },
		},
	)
}

// torenAttack is the line of the dagger in the options of Toren's turn, as p reads them.
func (it *infusedTable) torenAttack(p *person) *rulesv1.Attack {
	for _, a := range it.optionsOf(p, labelToren).GetOptions().GetAttacks() {
		if a.GetAttack().GetKey() == dagger {
			return a.GetAttack()
		}
	}
	panic("no dagger among the attacks of Toren as " + p.name + " reads them")
}

// sessionEvents counts the session events of the infusions the table has written.
func (it *infusedTable) sessionEvents(kind string) int {
	var n int
	if err := it.pool.QueryRow(it.t.Context(), `SELECT count(*) FROM session_events WHERE kind = $1`, kind).Scan(&n); err != nil {
		it.t.Fatalf("count the events of kind %s: %v", kind, err)
	}
	return n
}

func TestAnInfusionIsTheOwnersAndTheMastersAlone(t *testing.T) {
	it := newInfusedTable(t)
	w, m, caio, ana := it.world, it.master, it.caio, it.ana
	ctx := t.Context()
	it.buildInfusionRows()
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("the table: %v", r)
		}
	}()
	watchers := map[*person]*streamWatcher{m: w.watchStream(m), ana: w.watchStream(ana), caio: w.watchStream(caio)}
	it.toTurn(labelToren)
	it.ask(t, "the infusions are on")

	// The owner and the master read the infusion on the dagger and its bonus; Ana reads the dagger with neither.
	for _, p := range []*person{caio, m} {
		a := it.torenAttack(p)
		if a.GetInfusionNamePt() != it.bladeName || a.GetAttackBonus() == 0 {
			t.Fatalf("%s reads the dagger as %v, want the infusion %q (the positive control)", p.name, a, it.bladeName)
		}
	}
	infused := it.torenAttack(caio).GetAttackBonus()

	// An attack with the infused dagger: the log carries the attack's result and no word of the infusion.
	hit := it.mustAttack(caio, labelToren, dagger, "Esqueleto", []int32{17}, nil)
	if hit.GetPendingDamage() == nil {
		t.Fatalf("the attack with the dagger = %v, want a hit", hit.GetRoll())
	}
	it.damage(caio, hit.GetPendingDamage().GetId(), func(r *playv1.RollDamageRequest) {
		r.ExtrasChosen, r.Roll = true, &playv1.RollDamageRequest_TypedSum{TypedSum: 3}
	})
	it.ask(t, "the infused dagger hit")
	for _, p := range []*person{ana, caio, m} {
		if line := it.attackLine(p, labelToren, "Esqueleto"); line == nil {
			t.Errorf("%s reads no line of the infused attack in the log", p.name)
		}
	}
	it.endTurn(caio)

	// The infusion moves to the bow: the blade goes on another weapon, and the dagger is plain again.
	it.toTurn(labelToren)
	moved := must(caio.infusions.MoveInfusion(ctx, rq(&charactersv1.MoveInfusionRequest{
		CampaignId: w.campaign, CharacterId: w.toren.GetId(), InfusionId: it.blade.GetId(), ItemKey: shortbow, IdempotencyKey: newKey(),
	}))).GetInfusion()
	if moved.GetItemKey() != shortbow {
		t.Fatalf("the infusion moved to %q, want the bow", moved.GetItemKey())
	}
	it.ask(t, "the blade moved")
	for _, p := range []*person{caio, m} {
		if a := it.torenAttack(p); a.GetInfusionNamePt() != "" || a.GetAttackBonus() >= infused {
			t.Errorf("%s reads the dagger as %v after the infusion moved, want it plain", p.name, a)
		}
	}

	// The infusion ends in the middle of the combat: the next options of the turn have no name and no bonus, for anyone.
	must(caio.infusions.EndInfusion(ctx, rq(&charactersv1.EndInfusionRequest{CampaignId: w.campaign, CharacterId: w.toren.GetId(), InfusionId: it.blade.GetId()})))
	it.ask(t, "the blade ended")
	for _, p := range []*person{caio, m} {
		for _, a := range it.optionsOf(p, labelToren).GetOptions().GetAttacks() {
			if a.GetAttack().GetInfusionNamePt() != "" {
				t.Errorf("%s reads the infusion %q after it ended", p.name, a.GetAttack().GetInfusionNamePt())
			}
		}
		if got := it.torenAttack(p).GetAttackBonus(); got >= infused {
			t.Errorf("%s reads the attack bonus %d after the infusion ended, want less than the %d it gave", p.name, got, infused)
		}
	}
	// The master ends the one on the object: the owner's list loses it.
	must(m.infusions.EndInfusion(ctx, rq(&charactersv1.EndInfusionRequest{CampaignId: w.campaign, CharacterId: w.toren.GetId(), InfusionId: it.seal.GetId()})))
	it.ask(t, "the seal ended")

	// The session wrote its events: two infused, one moved and two ended. No read lists them to anyone.
	for kind, want := range map[string]int{"infusion_infused": 2, "infusion_moved": 1, "infusion_ended": 2} {
		if got := it.sessionEvents(kind); got != want {
			t.Errorf("the session holds %d events of kind %s, want %d", got, kind, want)
		}
	}

	// The end: the stream told the master and Caio of the three changes and Ana of none, and carried none of it.
	must(m.combat.EndEncounter(ctx, rq(&playv1.EndEncounterRequest{CampaignId: w.campaign, EncounterId: it.encounter.GetId(), IdempotencyKey: newKey()})))
	must(m.play.EndGameSession(ctx, rq(&playv1.EndGameSessionRequest{CampaignId: w.campaign, GameSessionId: w.session})))
	summary := read{
		procedure: playv1connect.PlayServiceGetSessionSummaryProcedure, allow: members,
		req: func(w *world) proto.Message {
			return &playv1.GetSessionSummaryRequest{CampaignId: w.campaign, GameSessionId: w.session}
		},
	}
	t.Run("the session summary", func(t *testing.T) { w.runRead(t, summary, it.got) })
	for p, sw := range watchers {
		select {
		case <-sw.done:
		case <-time.After(30 * time.Second):
			t.Fatalf("%s's stream did not end with the session", p.name)
		}
		hints := 0
		for _, ev := range sw.events {
			if h := ev.GetCharacterInfusionsChanged(); h != nil {
				hints++
				if h.GetCharacterId() != w.toren.GetId() {
					t.Errorf("%s's stream hears the infusions of %s, want Toren's", p.name, h.GetCharacterId())
				}
			}
			body, err := protojson.Marshal(ev)
			if err != nil {
				t.Fatalf("marshal an event: %v", err)
			}
			it.got.keep(p, reply{status: 200, body: body, msg: ev})
			if p == m {
				continue
			}
			for _, f := range w.inspect(p, reply{status: 200, body: body, msg: ev}, nil) {
				t.Errorf("%s: event %s: %s\n\tevent: %s", p.name, eventCase(ev), f, shorten(body))
			}
			if strings.Contains(string(body), "nfusion") && ev.GetCharacterInfusionsChanged() == nil {
				t.Errorf("%s: event %s speaks of an infusion: %s", p.name, eventCase(ev), shorten(body))
			}
		}
		switch want := p != ana; {
		case want && hints != 3:
			t.Errorf("%s's stream heard %d infusion hints, want 3 (moved, ended, ended)", p.name, hints)
		case !want && hints != 0:
			t.Errorf("%s's stream heard %d infusion hints, want none", p.name, hints)
		}
	}
	t.Run("canaries are reachable", func(t *testing.T) { checkCanariesAreReachable(t, w, it.got) })
}
