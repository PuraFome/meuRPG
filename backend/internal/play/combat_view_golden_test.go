package play

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
)

// The golden of the combat view (RN-10, RN-20): what the master, each player, a player of
// a table that hides the death saves, a player who does not see an NPC under the fog and an
// outsider (a player with no combatant in it) read of the same encounter, and the turn each
// of them is told. testdata/combat_view.golden holds it; UPDATE_GOLDEN=1 writes it again,
// and the diff is the review. No database: the view is a function of the rows.

const cviewGoldenPath = "testdata/combat_view.golden"

func cviewNames(key string) string {
	if key == "" {
		return ""
	}
	return "pt:" + key
}

func cviewViewers() []logViewer {
	return []logViewer{
		{"master", combatViewer{master: true, userID: "u-master"}},
		{"player Aria (u1)", combatViewer{userID: "u1"}},
		{"player Bruno (u2)", combatViewer{userID: "u2"}},
		{"player Bruno, death saves hidden", combatViewer{userID: "u2", hideDeath: true}},
		{"player Bruno, fog hides npc-fog", combatViewer{userID: "u2", unseen: map[string]bool{"npc-fog": true}}},
		{"outsider (u9)", combatViewer{userID: "u9"}},
		{"nobody (no user)", combatViewer{}},
	}
}

func cviewInt(n int32) *int32   { return &n }
func cviewStr(s string) *string { return &s }

// cviewCombatant is a combatant with the numbers every case shares.
func cviewCombatant(id, label, kind, turn string, init int32) playdb.Combatant {
	return playdb.Combatant{
		ID: id, Label: label, Kind: kind, TurnState: turn, Initiative: cviewInt(init), Side: sideEnemyKey, Size: "medium",
		SpeedFt: 30, MovementUsedDft: 55, HpCurrent: cviewInt(10), HpMax: cviewInt(12), HpTemp: cviewInt(0), XpValue: 50,
		InitiativeBonus: 2,
	}
}

// cviewParty is a table with a character in every state the view tells apart.
func cviewParty() *encounterData {
	start := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	aria := cviewCombatant("pc-aria", "Aria", kindPlayer, turnActing, 18)
	aria.UserID, aria.CharacterID, aria.Side = cviewStr("u1"), "char-aria", sidePartyKey
	aria.InitiativeFace, aria.GridCol, aria.GridRow = cviewInt(16), cviewInt(2), cviewInt(3)
	aria.Conditions, aria.ConcentrationSpell, aria.CoverMark = []string{"condition:prone"}, cviewStr("spell:bless"), "half"
	aria.DeathFailures, aria.AttackedHostile, aria.TookDamage, aria.RageEndPending = 3, true, true, true
	aria.ConditionSources = []byte(`[{"condition":"condition:prone","source_id":"npc-gob1","ends_combatant_id":"pc-bruno","ends_phase":"end_of_turn"}]`)
	bruno := cviewCombatant("pc-bruno", "Bruno", kindPlayer, turnActing, 18)
	bruno.UserID, bruno.CharacterID, bruno.Side = cviewStr("u2"), "char-bruno", sidePartyKey
	bruno.DeathSuccesses, bruno.DeathFailures, bruno.AcBonus, bruno.EffectAcBonus = 2, 1, 2, 1
	bruno.Size, bruno.SpeedFlyFt, bruno.ActionUsed = "large", 20, true
	cleo := cviewCombatant("pc-cleo", "Cleo", kindPlayer, turnEnded, 18)
	cleo.UserID, cleo.CharacterID, cleo.Side, cleo.Defeated = cviewStr("u3"), "char-cleo", sidePartyKey, true
	dara := cviewCombatant("pc-dara", "Dara", kindPlayer, turnIdle, 9)
	dara.UserID, dara.CharacterID, dara.Side, dara.DeathSuccesses, dara.DeathSaveRolled = cviewStr("u4"), "char-dara", sidePartyKey, 3, true
	eli := cviewCombatant("pc-eli", "Eli", kindPlayer, turnIdle, 8)
	eli.UserID, eli.CharacterID, eli.Side, eli.DeathSaveRolled = cviewStr("u5"), "char-eli", sidePartyKey, false
	wolf := cviewCombatant("cr-wolf", "Lobo de Aria", kindCreature, turnActing, 18)
	wolf.UserID, wolf.CharacterID, wolf.Side = cviewStr("u1"), "char-aria", sidePartyKey
	wolf.CreatureID, wolf.MonsterKey, wolf.SummonAttack, wolf.SummonGroupID = cviewStr("creature-1"), cviewStr("wolf"), cviewStr("reaction"), cviewStr("grp-1")
	wolf.HpMaxBonus, wolf.GridCol, wolf.GridRow = 2, cviewInt(0), cviewInt(0)
	gob1 := cviewCombatant("npc-gob1", "Goblin 1", kindNPC, turnIdle, 12)
	gob1.GridCol, gob1.GridRow, gob1.CharacterID = cviewInt(5), cviewInt(5), "char-gob1"
	gob2 := cviewCombatant("npc-gob2", "Goblin 2", kindNPC, turnIdle, 12)
	hid := cviewCombatant("npc-hid", "Escondido", kindNPC, turnIdle, 12)
	hid.Hidden = true
	fog := cviewCombatant("npc-fog", "Na Neblina", kindNPC, turnIdle, 5)
	tie := cviewCombatant("npc-tie", "Empate", kindNPC, turnIdle, 5)
	tie.Initiative = cviewInt(5)
	noInit := cviewCombatant("npc-late", "Sem Iniciativa", kindNPC, turnIdle, 0)
	noInit.Initiative = nil
	return &encounterData{
		enc: playdb.Encounter{
			ID: "enc-1", Name: "Emboscada", Status: statusActive, Round: 2, MapID: cviewStr("map-1"), MapPointID: cviewStr("pt-1"),
			GridColumns: 10, GridRows: 8, Revision: 7, Mode: "theatre", StartedAt: &start,
		},
		cs: []playdb.Combatant{aria, bruno, cleo, wolf, gob1, gob2, hid, dara, eli, fog, tie, noInit},
		states: []playdb.CombatantState{
			{ID: "st-1", CombatantID: "pc-aria", Kind: "rage", StartedRound: 1},
		},
	}
}

func cviewVitals() map[string]*playv1.CharacterVitals {
	return map[string]*playv1.CharacterVitals{
		"char-aria": {
			CharacterId: "char-aria", HitPointsCurrent: 0, HitPointsMax: 20, HitPointsMaxBonus: 3,
			WildShape:     &playv1.WildShapeState{BeastKey: "wolf", BeastNamePt: "Lobo", HitPointsCurrent: 7, HitPointsMax: 11},
			FamiliarSight: &playv1.FamiliarSightState{CreatureId: "creature-2"},
		},
		"char-bruno": {CharacterId: "char-bruno", HitPointsCurrent: 14, HitPointsMax: 20, HitPointsTemporary: 4},
		"char-dara":  {CharacterId: "char-dara", HitPointsCurrent: 0, HitPointsMax: 18},
		"char-eli":   {CharacterId: "char-eli", HitPointsCurrent: 0, HitPointsMax: 18},
	}
}

// cviewMessage is the message as JSON with sorted keys: protojson's whitespace is not stable.
func cviewMessage(m proto.Message) string {
	raw, err := protojson.MarshalOptions{UseProtoNames: true}.Marshal(m)
	if err != nil {
		return "ERR " + err.Error()
	}
	var tree map[string]any
	if err := json.Unmarshal(raw, &tree); err != nil {
		return "ERR " + err.Error()
	}
	out, err := json.Marshal(tree)
	if err != nil {
		return "ERR " + err.Error()
	}
	return string(out)
}

// cviewRenderView writes the encounter as every viewer reads it: the turn, then each
// field of the encounter outside the combatants, then one line per combatant.
func cviewRenderView(b *strings.Builder, title string, d *encounterData) {
	fmt.Fprintf(b, "######## %s\n", title)
	armor := map[string]int32{"char-aria": 15, "char-bruno": 17, "creature:cr-wolf": 13, "char-gob1": 15}
	npcs := map[string]masterNPC{
		"char-gob1":        {portrait: "/p/gob.png", monsterKey: "goblin", challengeRating: "1/4"},
		"creature:cr-wolf": {monsterKey: "wolf", challengeRating: "1/4"},
	}
	for _, vw := range cviewViewers() {
		fmt.Fprintf(b, "-- %s\n", vw.name)
		fmt.Fprintf(b, "  turn: %+v\n", d.turnFor(vw.v))
		out := d.view(vw.v, cviewVitals(), armor, npcs, cviewNames)
		cs := out.Combatants
		out.Combatants = nil
		fmt.Fprintf(b, "  encounter: %s\n", cviewMessage(out))
		for _, c := range cs {
			fmt.Fprintf(b, "  %s: %s\n", c.GetId(), cviewMessage(c))
		}
	}
}

// cviewTurns is the turn of one running combat, every viewer.
func cviewRenderTurns(b *strings.Builder, title string, d *encounterData) {
	fmt.Fprintf(b, "######## turn: %s\n", title)
	for _, vw := range cviewViewers() {
		fmt.Fprintf(b, "  %s: %+v\n", vw.name, d.turnFor(vw.v))
	}
}

// cviewTurnCases are the shapes of a turn: a lone actor, a joint turn, NPCs alone, hidden
// members, a defeated member that is the last to act, and nothing to do.
func cviewTurnCases() []struct {
	name string
	d    *encounterData
} {
	mk := func(status string, cs ...playdb.Combatant) *encounterData {
		return &encounterData{enc: playdb.Encounter{ID: "enc-t", Status: status, Round: 1}, cs: cs}
	}
	u1, u2 := cviewStr("u1"), cviewStr("u2")
	pc := func(id string, u *string, turn string) playdb.Combatant {
		c := cviewCombatant(id, id, kindPlayer, turn, 10)
		c.UserID = u
		return c
	}
	npc := func(id, turn string, hidden, defeated bool) playdb.Combatant {
		c := cviewCombatant(id, id, kindNPC, turn, 10)
		c.Hidden, c.Defeated = hidden, defeated
		return c
	}
	return []struct {
		name string
		d    *encounterData
	}{
		{"setup combat has no turn", mk(statusSetup, pc("a", u1, turnActing))},
		{"ended combat has no turn", mk(statusEnded, pc("a", u1, turnActing))},
		{"nobody acts", mk(statusActive, pc("a", u1, turnEnded), npc("g", turnIdle, false, false))},
		{"one player alone", mk(statusActive, pc("a", u1, turnActing), npc("g", turnIdle, false, false))},
		{"joint turn of two players and an npc", mk(statusActive, pc("a", u1, turnEnded), pc("b", u2, turnActing), npc("g", turnActing, false, false))},
		{"one npc alone", mk(statusActive, npc("g", turnActing, false, false))},
		{"two npcs alone", mk(statusActive, npc("g", turnActing, false, false), npc("h", turnActing, false, false))},
		{"two npcs alone, one hidden", mk(statusActive, npc("g", turnActing, false, false), npc("h", turnActing, true, false))},
		{"only a hidden npc acts", mk(statusActive, npc("h", turnActing, true, false), npc("g", turnEnded, false, false))},
		{"only a hidden npc acts beside a player", mk(statusActive, pc("a", u1, turnEnded), npc("h", turnActing, true, false))},
		{"defeated member acts last", mk(statusActive, pc("a", u1, turnEnded), npc("d", turnActing, false, true))},
		{"living member beats a defeated one", mk(statusActive, npc("d", turnActing, false, true), pc("a", u2, turnActing))},
		{"under the fog", mk(statusActive, pc("a", u1, turnEnded), npc("npc-fog", turnActing, false, false))},
	}
}

func cviewGolden() string {
	var b strings.Builder
	cviewRenderView(&b, "party table, round 2", cviewParty())
	setup := cviewParty()
	setup.enc.Status = statusSetup
	setup.enc.MapID, setup.enc.MapPointID, setup.enc.StartedAt = nil, nil, nil
	cviewRenderView(&b, "table in setup", setup)
	for _, c := range cviewTurnCases() {
		cviewRenderTurns(&b, c.name, c.d)
	}
	return b.String()
}

func TestCombatViewGolden(t *testing.T) {
	t.Parallel()
	wantGolden(t, cviewGoldenPath, cviewGolden())
}

// The leaks the rules forbid, said in assertions (RN-10, RN-20), beside the golden.
func TestCombatViewLeakRules(t *testing.T) {
	t.Parallel()
	d := cviewParty()
	read := func(v combatViewer) map[string]*playv1.Combatant {
		out := map[string]*playv1.Combatant{}
		for _, c := range d.view(v, cviewVitals(), nil, nil, cviewNames).Combatants {
			out[c.Id] = c
		}
		return out
	}
	outsider := read(combatViewer{userID: "u9"})
	if _, ok := outsider["npc-hid"]; ok {
		t.Error("an outsider reads a hidden combatant")
	}
	if outsider["npc-gob1"].GetInitiative() != 0 || outsider["npc-gob1"].GetCharacterId() != "" || outsider["npc-gob1"].HitPointsCurrent != nil {
		t.Errorf("an outsider reads an NPC's numbers: %v", outsider["npc-gob1"])
	}
	if outsider["pc-aria"].GetInitiative() != 18 || outsider["pc-aria"].InitiativeBonus != nil {
		t.Errorf("an outsider reads a player's bonus or misses the roll: %v", outsider["pc-aria"])
	}
	if _, ok := read(combatViewer{userID: "u2", unseen: map[string]bool{"npc-fog": true}})["npc-fog"]; ok {
		t.Error("the fog does not hide an unseen NPC")
	}
	master := read(combatViewer{master: true})
	if master["npc-hid"] == nil || !master["npc-hid"].Hidden || master["pc-aria"].HitPointsCurrent == nil {
		t.Errorf("the master lacks what is theirs: %v", master["npc-hid"])
	}
}
