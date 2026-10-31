package play

import (
	"slices"
	"strings"
	"testing"

	"connectrpc.com/connect"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/internal/play/live"
)

// What the master hides from the players reaches them through no channel (RN-10): the zone of a hidden
// caster is the master's alone, in the view, the log and the hints; a zone that hits a hidden creature
// does not name it to a player.

// npcCaster creates an NPC druid of level 5 who prepared Moonbeam (the master plays it).
func (u *user) npcCaster(t *testing.T, campaignID, name string, more ...string) *charactersv1.Character {
	t.Helper()
	sheet := &charactersv1.CharacterSheet{Content: &charactersv1.CharacterSheet_Full{Full: &charactersv1.FullSheet{
		BaseScores: abilities(10, 14, 12, 16, 10), RaceKey: "race:human", Classes: []*charactersv1.ClassLevel{{ClassKey: "class:druid", Level: 5}},
		PreparedSpellKeys: append([]string{moonbeamKey}, more...),
	}}}
	res, err := u.characters.CreateCharacter(t.Context(), connect.NewRequest(&charactersv1.CreateCharacterRequest{
		CampaignId: campaignID, Kind: charactersv1.CharacterKind_CHARACTER_KIND_BOSS, Name: name, Sheet: sheet,
	}))
	if err != nil {
		t.Fatalf("CreateCharacter(%s) error = %v", name, err)
	}
	return res.Msg.GetCharacter()
}

// druidFight starts the combat with the NPC druid first (hidden unless revealed), then the ogre
// (revealed) and the goblin (hidden unless revealed).
func (c *cave) druidFight(t *testing.T, reveal ...string) *playv1.Encounter {
	t.Helper()
	return c.druidFightWith(t, nil, reveal...)
}

// druidFightWith is druidFight with more spells prepared by the druid.
func (c *cave) druidFightWith(t *testing.T, more []string, reveal ...string) *playv1.Encounter {
	t.Helper()
	druid := c.master.npcCaster(t, c.campaignID, "Druida", more...)
	return c.start(t, plan{
		npcs:     []*playv1.Participant{{CharacterId: druid.GetId()}, {CharacterId: c.ogre.GetId()}, {CharacterId: c.goblins.GetId()}},
		npcRolls: []int{20, 2, 1},
		players:  map[string]int32{"Toren": 1, "Pensantus": 1, "Brisa": 1},
		reveal:   append([]string{"Ogro"}, reveal...),
		at:       map[string][2]int32{"Druida": {5, 7}, "Toren": {3, 7}, "Brisa": {3, 8}, "Pensantus": {12, 7}, "Ogro": {7, 7}, "Goblin": {7, 8}},
	})
}

// zoneLeaks lists the words of a hidden caster's zone that a player's copy must not have.
func zoneLeaks(t *testing.T, c *cave, u *user, extra ...string) []string {
	t.Helper()
	e := c.get(t, u)
	text := asJSONAny(t, e) + asJSONAny(t, c.log(t, u, e))
	var found []string
	for _, w := range append([]string{moonbeamKey, "Druida", "Moonbeam", "Raio de Lua"}, extra...) {
		if strings.Contains(text, w) {
			found = append(found, w)
		}
	}
	return found
}

// The Moonbeam of a hidden NPC caster is on the master's map and nobody else's.
func TestAHiddenCastersZoneIsTheMastersAlone(t *testing.T) {
	t.Parallel()
	c := newCaveWith(t, 5, []string{thunderwave})
	c.druidFight(t)
	c.mustCastArea(t, c.master, "Druida", moonbeamKey, slotOfLevel(2), at(7, 7), nil)
	z := c.oneZone(t, c.master, moonbeamKey)
	if z.GetVisibility() != playv1.SpellZoneVisibility_SPELL_ZONE_VISIBILITY_MASTER_ONLY {
		t.Errorf("visibility = %v, want MASTER_ONLY: the caster is hidden", z.GetVisibility())
	}
	for who, u := range map[string]*user{"Toren's player": c.caio, "Pensantus's player": c.ana, "Brisa's player": c.bia} {
		if zs := c.get(t, u).GetZones(); len(zs) != 0 {
			t.Errorf("%s reads zones %v, want none: the caster is hidden", who, zs)
		}
		if leaks := zoneLeaks(t, c, u, z.GetId(), "ZONE"); len(leaks) != 0 {
			t.Errorf("%s's copy has %v of the hidden caster's zone", who, leaks)
		}
	}
	// The master reads the cast and, later, the hit.
	c.h.roller.queue(5, 6, 6)
	c.passTo(t, c.get(t, c.master), "Ogro")
	if cur, _, _ := c.hp(t, "Ogro"); cur != 59-12 {
		t.Fatalf("the ogre's hit points = %d, want 47: it started its turn in the beam", cur)
	}
	if len(c.zoneLines(t, c.master, moonbeamKey)) == 0 {
		t.Error("the master's log has no line for the beam")
	}
	for who, u := range map[string]*user{"Toren's player": c.caio, "Pensantus's player": c.ana} {
		if lines := c.zoneLines(t, u, moonbeamKey); len(lines) != 0 {
			t.Errorf("%s reads the lines %v of a hidden caster's zone", who, lines)
		}
		if leaks := zoneLeaks(t, c, u); len(leaks) != 0 {
			t.Errorf("%s's copy has %v after the hit", who, leaks)
		}
	}
}

// Once the master reveals the caster, the same zone is the table's.
func TestARevealedCastersZoneIsTheTables(t *testing.T) {
	t.Parallel()
	c := newCaveWith(t, 5, []string{thunderwave})
	c.druidFight(t, "Druida")
	c.mustCastArea(t, c.master, "Druida", moonbeamKey, slotOfLevel(2), at(7, 7), nil)
	for who, u := range map[string]*user{"master": c.master, "Toren's player": c.caio, "Pensantus's player": c.ana} {
		zs := zonesOf(c.get(t, u), moonbeamKey)
		if len(zs) != 1 || zs[0].GetVisibility() != playv1.SpellZoneVisibility_SPELL_ZONE_VISIBILITY_EVERYONE {
			t.Errorf("%s reads zones %v, want the beam, for everyone", who, zs)
		}
	}
}

// A zone that hits a creature the players do not see does not name it to them, and the damage is
// nobody's line but the master's.
func TestAZoneThatHitsAHiddenCreatureDoesNotNameIt(t *testing.T) {
	t.Parallel()
	c := newCaveWith(t, 5, []string{thunderwave})
	c.druidFight(t, "Druida")
	c.mustCastArea(t, c.master, "Druida", moonbeamKey, slotOfLevel(2), at(7, 8), nil) // over the hidden goblin
	c.h.roller.queue(5, 6, 6)
	c.passTo(t, c.get(t, c.master), "Goblin")
	if cur, _, down := c.hp(t, "Goblin"); cur != 0 || !down {
		t.Fatalf("the goblin's hit points = %d, down = %v, want it down: 12 of the beam on 7 hit points", cur, down)
	}
	for who, u := range map[string]*user{"Toren's player": c.caio, "Pensantus's player": c.ana} {
		e := c.get(t, u)
		text := asJSONAny(t, e) + asJSONAny(t, c.log(t, u, e))
		for _, w := range []string{"Goblin", c.id(t, "Goblin")} {
			if strings.Contains(text, w) {
				t.Errorf("%s's copy names the hidden goblin (%q)", who, w)
			}
		}
		for _, line := range c.zoneLines(t, u, moonbeamKey) { // the ogre next to the goblin is caught too, and is theirs to read
			if slices.Contains(line.GetEffect().GetTargetLabels(), "Goblin") || line.GetTargetLabel() == "Goblin" {
				t.Errorf("%s reads the line %v about the hidden goblin", who, line)
			}
		}
	}
}

// The hint of a zone that moves goes to the master alone when its caster is hidden, and a player
// cannot ask for the zone: it is not found for them, as one that never existed.
func TestAHiddenCastersZoneMovesWithoutAHintToThePlayers(t *testing.T) {
	t.Parallel()
	c := newCaveWith(t, 5, []string{thunderwave})
	c.druidFight(t)
	c.mustCastArea(t, c.master, "Druida", moonbeamKey, slotOfLevel(2), at(7, 7), nil)
	z := c.oneZone(t, c.master, moonbeamKey)
	master, err := c.h.svc.hub.Subscribe(c.campaignID, live.Subscriber{UserID: "m", Master: true})
	if err != nil {
		t.Fatal(err)
	}
	defer master.Close()
	player, err := c.h.svc.hub.Subscribe(c.campaignID, live.Subscriber{UserID: c.ana.id})
	if err != nil {
		t.Fatal(err)
	}
	defer player.Close()
	if _, err := c.moveZone(t, c.ana, z.GetId(), 8, 8); connect.CodeOf(err) != connect.CodeNotFound {
		t.Errorf("a player asked to move a hidden caster's zone: error = %v, want not_found", err)
	}
	if _, err := c.moveZone(t, c.master, z.GetId(), 8, 8); err != nil {
		t.Fatalf("MoveSpellZone() as the master error = %v", err)
	}
	if got := heard(master); got == 0 {
		t.Error("the master heard nothing of the zone that moved")
	}
	if got := heard(player); got != 0 {
		t.Errorf("a player heard %d events of a hidden caster's zone that moved, want none", got)
	}
}

// heard drains a subscription and counts the events it had.
func heard(sub *live.Subscription) int {
	n := 0
	for {
		select {
		case ev := <-sub.Events():
			sub.Taken(ev)
			n++
		default:
			return n
		}
	}
}
