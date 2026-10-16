package characters

import (
	"fmt"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	"github.com/PuraFome/meuRPG/backend/internal/platform/db"
	"github.com/PuraFome/meuRPG/backend/internal/play/link"
)

// What play reads and writes of the creatures through play.CombatRoster
// (MR-037): SummonCreatures records a casting, SyncCreatures keeps the creatures
// in step with a combat. These tests freeze what each call returns and what it
// leaves in the table.

// rosterCreature is a link.Creature as the tests compare it: the IDs as labels.
func rosterCreature(c link.Creature, owner string) string {
	return fmt.Sprintf("%s (%s) source %s attack %s concentration %v hp %d/%d owner %v init %d speed %d fly %d jump %d/%d size %s",
		c.Name, c.MonsterKey, c.Source, c.Attack, c.DependsOnConcentration, c.HitPointsCurrent, c.HitPointsMax, c.OwnerUserID == owner,
		c.InitiativeBonus, c.SpeedFt, c.SpeedFlyFt, c.JumpLongDFt, c.JumpHighDFt, c.Size)
}

func rosterCreatures(list []link.Creature, owner string) string {
	out := make([]string, 0, len(list))
	for _, c := range list {
		out = append(out, rosterCreature(c, owner))
	}
	return strings.Join(out, "\n")
}

// summonInTx runs SummonCreatures in a transaction of its own.
func (tb *creatureTable) summonInTx(t *testing.T, sm link.Summon) (link.SummonResult, error) {
	t.Helper()
	sm.CampaignID, sm.CharacterID = tb.campaign, tb.pc.GetId()
	if sm.GroupID == "" {
		sm.GroupID = uuid.New().String()
	}
	var res link.SummonResult
	err := db.InTx(t.Context(), tb.h.pool, func(tx pgx.Tx) error {
		var err error
		res, err = tb.h.svc.SummonCreatures(t.Context(), tx, sm)
		return err
	})
	return res, err
}

func TestMR037_SummonCreatures(t *testing.T) {
	t.Parallel()
	tb := newCreatureTable(t)
	owner := tb.owner.id

	// A familiar.
	first, err := tb.summonInTx(t, link.Summon{Source: "familiar", Creatures: []link.CreatureSpec{{MonsterKey: "monster:raven", Name: "Nanquim", Attack: "none"}}})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := rosterCreatures(first.Created, owner), "Nanquim (monster:raven) source familiar attack none concentration false hp 1/1 owner true init 2 speed 10 fly 50 jump 20/0 size tiny"; got != want {
		t.Errorf("the familiar =\n%s\nwant\n%s", got, want)
	}
	if len(first.Replaced) != 0 {
		t.Errorf("the first familiar replaced %v", first.Replaced)
	}

	// A new familiar replaces the old one.
	second, err := tb.summonInTx(t, link.Summon{Source: "familiar", Creatures: []link.CreatureSpec{{MonsterKey: "monster:cat", Attack: "none"}}})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := rosterCreatures(second.Replaced, owner), "Nanquim (monster:raven) source familiar attack none concentration false hp 1/1 owner true init 2 speed 10 fly 50 jump 20/0 size tiny"; got != want {
		t.Errorf("replaced =\n%s\nwant\n%s", got, want)
	}
	if got := tb.creatureRow(t, first.Created[0].ID); got != `hp 1/1 dismissed "replaced"` {
		t.Errorf("the old familiar: %s", got)
	}
	if got, want := rosterCreatures(second.Created, owner), "Gato (monster:cat) source familiar attack none concentration false hp 2/2 owner true init 2 speed 40 fly 0 jump 30/0 size tiny"; got != want {
		t.Errorf("the new familiar =\n%s\nwant\n%s", got, want)
	}

	// A casting that is no familiar and no concentration replaces nothing; two of a
	// kind are numbered.
	undead, err := tb.summonInTx(t, link.Summon{Source: "animate_dead", Creatures: []link.CreatureSpec{
		{MonsterKey: "monster:skeleton", Attack: "full"}, {MonsterKey: "monster:skeleton", Attack: "full"}, {MonsterKey: "monster:zombie", Name: "Zeca", Attack: "full"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := rosterCreatures(undead.Created, owner), strings.Join([]string{
		"Esqueleto 1 (monster:skeleton) source animate_dead attack full concentration false hp 13/13 owner true init 2 speed 30 fly 0 jump 100/30 size medium",
		"Esqueleto 2 (monster:skeleton) source animate_dead attack full concentration false hp 13/13 owner true init 2 speed 30 fly 0 jump 100/30 size medium",
		"Zeca (monster:zombie) source animate_dead attack full concentration false hp 22/22 owner true init -2 speed 20 fly 0 jump 130/40 size medium",
	}, "\n"); got != want {
		t.Errorf("the undead =\n%s\nwant\n%s", got, want)
	}
	if len(undead.Replaced) != 0 {
		t.Errorf("the undead replaced %v", undead.Replaced)
	}

	// A concentration casting ends the creatures of the one before.
	wolves, err := tb.summonInTx(t, link.Summon{Source: "conjure_animals", Concentration: true, Creatures: []link.CreatureSpec{
		{MonsterKey: "monster:wolf", Attack: "full"}, {MonsterKey: "monster:wolf", Attack: "full"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if len(wolves.Replaced) != 0 {
		t.Errorf("the first concentration replaced %v", wolves.Replaced)
	}
	boars, err := tb.summonInTx(t, link.Summon{Source: "conjure_animals", Concentration: true, Creatures: []link.CreatureSpec{{MonsterKey: "monster:boar", Attack: "full"}}})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := rosterCreatures(boars.Replaced, owner), strings.Join([]string{
		"Lobo 1 (monster:wolf) source conjure_animals attack full concentration true hp 11/11 owner true init 2 speed 40 fly 0 jump 120/40 size medium",
		"Lobo 2 (monster:wolf) source conjure_animals attack full concentration true hp 11/11 owner true init 2 speed 40 fly 0 jump 120/40 size medium",
	}, "\n"); got != want {
		t.Errorf("replaced =\n%s\nwant\n%s", got, want)
	}
	for _, w := range wolves.Created {
		if got := tb.creatureRow(t, w.ID); got != `hp 11/11 dismissed "concentration"` {
			t.Errorf("a wolf of the old concentration: %s", got)
		}
	}
	if got, want := rosterCreatures(boars.Created, owner), "Javali (monster:boar) source conjure_animals attack full concentration true hp 11/11 owner true init 0 speed 40 fly 0 jump 130/40 size medium"; got != want {
		t.Errorf("the boar =\n%s\nwant\n%s", got, want)
	}
	if boars.Created[0].GroupID == wolves.Created[0].GroupID || wolves.Created[0].GroupID != wolves.Created[1].GroupID {
		t.Errorf("groups: wolves %s and %s, boar %s", wolves.Created[0].GroupID, wolves.Created[1].GroupID, boars.Created[0].GroupID)
	}

	// A key that is no creature fails the casting and writes nothing.
	_, err = tb.summonInTx(t, link.Summon{Source: "conjure_animals", Creatures: []link.CreatureSpec{{MonsterKey: "monster:nope", Attack: "full"}}})
	if err == nil || err.Error() != `"monster:nope" is not a creature` {
		t.Errorf("an unknown creature: %v", err)
	}
}

func TestMR037_SummonCreaturesStopsAtTheLimit(t *testing.T) {
	t.Parallel()
	tb := newCreatureTable(t)
	rats := func(n int) []link.CreatureSpec {
		out := make([]link.CreatureSpec, n)
		for i := range out {
			out[i] = link.CreatureSpec{MonsterKey: "monster:rat", Attack: "full"}
		}
		return out
	}
	if _, err := tb.summonInTx(t, link.Summon{Source: "familiar", Creatures: rats(1)}); err != nil {
		t.Fatal(err)
	}
	if _, err := tb.summonInTx(t, link.Summon{Source: "conjure_animals", Creatures: rats(maxCreaturesPerCharacter - 1)}); err != nil {
		t.Fatalf("up to the limit: %v", err)
	}
	_, err := tb.summonInTx(t, link.Summon{Source: "conjure_animals", Creatures: rats(1)})
	if b := blocked(t, "one past the limit", err); b.GetReason() != charactersv1.CharacterBlockedReason_CHARACTER_BLOCKED_REASON_CREATURE_LIMIT || b.GetCharacterId() != tb.pc.GetId() {
		t.Errorf("blocked = %v", b)
	}
	// A new familiar sends the old one away before the count: the place is free again.
	if _, err := tb.summonInTx(t, link.Summon{Source: "familiar", Creatures: rats(1)}); err != nil {
		t.Errorf("a familiar that replaces one at the limit: %v", err)
	}
}

// syncInTx runs SyncCreatures in a transaction of its own.
func (tb *creatureTable) syncInTx(t *testing.T, states ...link.CreatureState) link.CreatureChanges {
	t.Helper()
	var out link.CreatureChanges
	if err := db.InTx(t.Context(), tb.h.pool, func(tx pgx.Tx) error {
		var err error
		out, err = tb.h.svc.SyncCreatures(t.Context(), tx, tb.campaign, states, time.Now())
		return err
	}); err != nil {
		t.Fatalf("SyncCreatures() error = %v", err)
	}
	return out
}

func TestMR037_SyncCreatures(t *testing.T) {
	t.Parallel()
	tb := newCreatureTable(t)
	owner := tb.owner.id
	presa, garra, dente, sumida := tb.wolf(t, "Presa"), tb.wolf(t, "Garra"), tb.wolf(t, "Dente"), tb.wolf(t, "Sumida")
	if err := tb.dismiss(tb.owner, sumida.GetId()); err != nil {
		t.Fatal(err)
	}

	if got := tb.syncInTx(t); got.Dismissed != nil || got.Revived != nil {
		t.Errorf("no states = %v", got)
	}

	// Defeated: dismissed at 0. Up and live: nothing written (not even the hit
	// points). A creature of no campaign is skipped. One the owner dismissed stays so.
	got := tb.syncInTx(t,
		link.CreatureState{ID: presa.GetId(), HP: 0, Defeated: true},
		link.CreatureState{ID: garra.GetId(), HP: 3},
		link.CreatureState{ID: uuid.New().String(), HP: 3, Defeated: true},
		link.CreatureState{ID: sumida.GetId(), HP: 5},
	)
	if s, want := rosterCreatures(got.Dismissed, owner), "Presa (monster:wolf) source master attack full concentration false hp 0/11 owner true init 2 speed 40 fly 0 jump 120/40 size medium"; s != want {
		t.Errorf("dismissed =\n%s\nwant\n%s", s, want)
	}
	if len(got.Revived) != 0 {
		t.Errorf("revived = %v", got.Revived)
	}
	for id, want := range map[string]string{
		presa.GetId(): `hp 0/11 dismissed "defeated"`, garra.GetId(): `hp 11/11 dismissed ""`, sumida.GetId(): `hp 11/11 dismissed "owner"`,
	} {
		if s := tb.creatureRow(t, id); s != want {
			t.Errorf("after the first sync: %s", s)
		}
	}

	// Defeated again: nothing (it is dismissed already). Up again: revived with the
	// combatant's hit points, clamped to 0 and to the maximum.
	if got := tb.syncInTx(t, link.CreatureState{ID: presa.GetId(), Defeated: true}); got.Dismissed != nil || got.Revived != nil {
		t.Errorf("defeated twice = %v", got)
	}
	if err := tb.dismiss(tb.master, dente.GetId()); err != nil {
		t.Fatal(err)
	}
	got = tb.syncInTx(t, link.CreatureState{ID: presa.GetId(), HP: 50}, link.CreatureState{ID: dente.GetId(), HP: 4})
	if s, want := rosterCreatures(got.Revived, owner), "Presa (monster:wolf) source master attack full concentration false hp 11/11 owner true init 2 speed 40 fly 0 jump 120/40 size medium"; s != want {
		t.Errorf("revived =\n%s\nwant\n%s", s, want)
	}
	if s := tb.creatureRow(t, presa.GetId()); s != `hp 11/11 dismissed ""` {
		t.Errorf("revived: %s", s)
	}
	if s := tb.creatureRow(t, dente.GetId()); s != `hp 11/11 dismissed "master"` {
		t.Errorf("dismissed by the master: %s", s)
	}
	tb.syncInTx(t, link.CreatureState{ID: presa.GetId(), Defeated: true})
	got = tb.syncInTx(t, link.CreatureState{ID: presa.GetId(), HP: -3})
	if len(got.Revived) != 1 || got.Revived[0].HitPointsCurrent != 0 {
		t.Errorf("revived at a negative = %v", got.Revived)
	}
	if s := tb.creatureRow(t, presa.GetId()); s != `hp 0/11 dismissed ""` {
		t.Errorf("revived at a negative: %s", s)
	}
}

// GetSummonOptions beyond TestMR037_SummonOptions: who gets not_found, a dead
// caster's empty answer, a caster with slots of both kinds, and a wizard who
// prepared Convocar Familiar.
func TestMR037_SummonOptionsEdges(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	master, wizard, multi, dying := h.newUser("Mestre"), h.newUser("Maga"), h.newUser("Dupla"), h.newUser("Morta")
	campaign := h.newCampaign(master, "Mirathel", wizard, multi, dying)

	for name, tt := range map[string]struct {
		as   *user
		id   string
		want string
	}{
		"someone of no campaign": {h.newUser("De fora"), uuid.New().String(), "not_found: campaign not found"},
		"an ID that is no UUID":  {master, "x", "not_found: character not found"},
		"no such character":      {master, uuid.New().String(), "not_found: character not found"},
	} {
		_, err := summonOptions(t, tt.as, campaign, tt.id)
		wantCreatureRefusal(t, name, err, tt.want, "")
	}
	npc := master.create(t, campaign, charactersv1.CharacterKind_CHARACTER_KIND_STORY, "Berta", basicSheet())
	_, err := summonOptions(t, master, campaign, npc.GetId())
	wantCreatureRefusal(t, "an NPC", err, "invalid_argument: character_id is an NPC: only a player's character has creatures", "character_id")

	// A dead caster: nothing to cast, no slots.
	dead := master.markDead(t, dying.createPensantus(t, campaign))
	if r, err := summonOptions(t, dying, campaign, dead.GetId()); err != nil || len(r.GetSlots()) != 0 || len(r.GetSpells()) != 0 {
		t.Errorf("a dead caster = %v, %v; want an empty answer", r, err)
	}

	// A wizard who prepared Convocar Familiar casts it with a slot too, at each circle
	// the spell takes.
	sheet := pensantusSheet()
	sheet.GetFull().PreparedSpellKeys = append(sheet.GetFull().PreparedSpellKeys, "spell:find-familiar")
	prepared := wizard.create(t, campaign, charactersv1.CharacterKind_CHARACTER_KIND_PLAYER, "Pensantus", sheet)
	r, err := summonOptions(t, wizard, campaign, prepared.GetId())
	if err != nil {
		t.Fatal(err)
	}
	s := spellOf(t, r, "spell:find-familiar")
	var circles []int32
	for _, c := range s.GetCircles() {
		circles = append(circles, c.GetCircle())
	}
	if !s.GetCanRitual() || !s.GetCanCastWithSlot() || fmt.Sprint(circles) != "[1 2]" {
		t.Errorf("a prepared find familiar: ritual %v, slot %v, circles %v", s.GetCanRitual(), s.GetCanCastWithSlot(), circles)
	}

	// A wizard and warlock: the pact slots sorted in with the others by circle.
	both := multi.create(t, campaign, charactersv1.CharacterKind_CHARACTER_KIND_PLAYER, "Dupla", multiclassCasterSheet())
	r, err = summonOptions(t, multi, campaign, both.GetId())
	if err != nil {
		t.Fatal(err)
	}
	var slots []string
	for _, sl := range r.GetSlots() {
		slots = append(slots, fmt.Sprintf("%d pact %v %d/%d", sl.GetLevel(), sl.GetPact(), sl.GetFree(), sl.GetTotal()))
	}
	if got, want := strings.Join(slots, ", "), "1 pact false 3/3, 1 pact true 1/1"; got != want {
		t.Errorf("slots = %s, want %s", got, want)
	}
}

// multiclassCasterSheet is a human wizard 2 and warlock 1 (the Fiend).
func multiclassCasterSheet() *charactersv1.CharacterSheet {
	sheet := casterSheet("class:wizard", "", 2, 8, 14, 14, 16, 10, 15, nil, []string{"spell:find-familiar"})
	sheet.GetFull().Classes = append(sheet.GetFull().Classes, &charactersv1.ClassLevel{
		ClassKey: "class:warlock", Level: 1, Subclass: &charactersv1.ClassLevel_SubclassKey{SubclassKey: "subclass:fiend"},
	})
	return sheet
}

func TestMR042_MonsterNpcOfNoCreature(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	master := h.newUser("Mestre")
	campaign := h.newCampaign(master, "Mirathel")
	if err := db.InTx(t.Context(), h.pool, func(tx pgx.Tx) error {
		npc, ok, err := h.svc.MonsterNpc(t.Context(), tx, campaign, master.id, "spell:fireball", h.clock.Now())
		if ok || npc.ID != "" {
			t.Errorf("MonsterNpc(spell) = %v, %v; want nothing", npc, ok)
		}
		return err
	}); err != nil {
		t.Fatal(err)
	}
}

// The NPC another combat made between the read and the insert is the one
// answered: the insert finds the key taken and reads the row again.
func TestMR042_MonsterNpcMadeMeanwhile(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	master := h.newUser("Mestre")
	campaign := h.newCampaign(master, "Mirathel")
	var other string
	h.hookAfterQuery("create_key = ", func() {
		if err := db.InTx(t.Context(), h.pool, func(tx pgx.Tx) error {
			npc, _, err := h.svc.MonsterNpc(t.Context(), tx, campaign, master.id, "monster:ogre", h.clock.Now())
			other = npc.ID
			return err
		}); err != nil {
			t.Errorf("the other combat: %v", err)
		}
	})
	var got link.Character
	if err := db.InTx(t.Context(), h.svc.pool, func(tx pgx.Tx) error {
		var err error
		got, _, err = h.svc.MonsterNpc(t.Context(), tx, campaign, master.id, "monster:ogre", h.clock.Now())
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if got.ID == "" || got.ID != other {
		t.Errorf("MonsterNpc() = %s, want the other combat's %s", got.ID, other)
	}
	var n int
	if err := h.pool.QueryRow(t.Context(), "SELECT count(*) FROM characters WHERE campaign_id = $1", campaign).Scan(&n); err != nil || n != 1 {
		t.Errorf("characters = %d, %v; want 1", n, err)
	}
}
