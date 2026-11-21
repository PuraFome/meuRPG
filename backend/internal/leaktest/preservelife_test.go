package leaktest

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"

	"connectrpc.com/connect"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// Preserve Life (RN-10, RN-20): the creatures a player may name and the creatures the points are shared among are the ones
// that player reads, and each reader's result and line are the ones of an action that restored only the creatures that
// reader reads. The scene is the one of Turn Undead (turnWorld); each row runs it twice, with the creatures the cleric's
// player may read among others and with those alone, and the whole answer and the whole line are the same in both.

const (
	preserveLifeKey = "feature:channel-divinity-preserve-life"
	// preserveAsk is what each named creature is asked to regain: within the five times the cleric's level.
	preserveAsk = 5
	// bandit is a creature of no undead or construct type, with 11 hit points: half is 5.
	bandit = "monster:bandit"
)

// preserve makes p take Preserve Life, naming the ids (none: the server shares the points).
func (tw *turnWorld) preserve(p *person, ids ...string) (*playv1.TakeActionResponse, error) {
	return tw.preserveWith(p, newKey(), ids...)
}

func (tw *turnWorld) preserveWith(p *person, key string, ids ...string) (*playv1.TakeActionResponse, error) {
	tw.t.Helper()
	req := &playv1.TakeActionRequest{CampaignId: tw.campaign, EncounterId: tw.encounter.GetId(), CombatantId: tw.cleric, ActionKey: preserveLifeKey, IdempotencyKey: key}
	for _, id := range ids {
		req.PreserveLife = append(req.PreserveLife, &playv1.PreserveLifeHeal{CombatantId: id, HitPoints: preserveAsk})
	}
	res, err := p.combat.TakeAction(tw.t.Context(), rq(req))
	if err != nil {
		return nil, err
	}
	return res.Msg, nil
}

func (tw *turnWorld) idsOf(labels []string) []string {
	var out []string
	for _, l := range labels {
		out = append(out, tw.ids[l])
	}
	return out
}

// refusalText is the whole refusal: its code, its message and each of its details.
func refusalText(err error) string {
	var ce *connect.Error
	if !errors.As(err, &ce) {
		return fmt.Sprintf("not a refusal: %v", err)
	}
	out := fmt.Sprintf("%v|%s", ce.Code(), ce.Message())
	for _, d := range ce.Details() {
		if m, err := d.Value(); err == nil {
			out += "|" + jsonOf(m)
		}
	}
	return out
}

func healLabels(res *playv1.ChannelDivinityResult) []string {
	var out []string
	for _, h := range res.GetHeals() {
		out = append(out, h.GetLabel())
	}
	slices.Sort(out)
	return out
}

func (tw *turnWorld) checkHeals(what string, got *playv1.ChannelDivinityResult, want []string) {
	tw.t.Helper()
	if got == nil {
		tw.t.Errorf("%s: no result of Channel Divinity", what)
		return
	}
	if l := healLabels(got); !slices.Equal(l, want) {
		tw.t.Errorf("%s lists %v, want %v", what, l, want)
	}
}

// restoredBy are the labels of the creatures an action restores: the named ones, or the hurt ones of the cleric's side.
func restoredBy(zs []turnZombie, named []string, keep func(turnZombie) bool) []string {
	return labelsOfZombies(zs, func(z turnZombie) bool {
		if !keep(z) {
			return false
		}
		if named == nil {
			return z.ally && z.hp > 0
		}
		return slices.Contains(named, z.label)
	})
}

type preserveRow struct {
	name    string
	mode    turnMap
	zombies []turnZombie
	named   []string // nil: no one is named
	// byMaster says the master acts for the cleric.
	byMaster bool
}

func TestPreserveLifeResultFollowsTheCreaturesItsReaderMayRead(t *testing.T) {
	a := turnZombie{label: "Quimbul4321", key: bandit, hp: 1}
	hid := turnZombie{label: "Quimbul4322", key: bandit, hp: 1, hidden: true}
	far := turnZombie{label: "Quimbul4322", key: bandit, hp: 1, behind: true}
	half := turnZombie{label: "Quimbul4321", key: bandit, hp: 5}
	friend := turnZombie{label: "Xandruk4321", key: bandit, ally: true, hp: 1}
	hidFriend := turnZombie{label: "Xandruk4322", key: bandit, ally: true, hp: 1, hidden: true}
	pens := turnZombie{label: "Pensantus", party: true, hp: 1}
	rows := []preserveRow{
		{"named, one hidden by the master", turnClearMap, []turnZombie{a, hid}, []string{"Quimbul4321"}, false},
		{"named, one out of sight on a fog map", turnFogMap, []turnZombie{a, far}, []string{"Quimbul4321"}, false},
		{"named, one hidden by the master, on a fog map", turnFogMap, []turnZombie{a, hid}, []string{"Quimbul4321"}, false},
		{"named, no map, one hidden by the master", turnNoMap, []turnZombie{a, hid}, []string{"Quimbul4321"}, false},
		{"named, the others are all readable", turnClearMap, []turnZombie{a, friend}, []string{"Quimbul4321", "Xandruk4321"}, false},
		{"named, at half the maximum", turnClearMap, []turnZombie{half, hid}, []string{"Quimbul4321"}, false},
		{"shared, one hidden by the master", turnClearMap, []turnZombie{friend, hidFriend}, nil, false},
		{"shared, one hidden by the master, on a fog map", turnFogMap, []turnZombie{friend, hidFriend}, nil, false},
		{"shared, no map, one hidden by the master", turnNoMap, []turnZombie{friend, hidFriend}, nil, false},
		{"shared, with a creature of the other side among them", turnClearMap, []turnZombie{friend, a, hidFriend}, nil, false},
		{"named, another player's character", turnClearMap, []turnZombie{pens}, []string{"Pensantus"}, false},
		{"shared, no creature of the cleric's side that is read is hurt", turnClearMap, []turnZombie{a, hidFriend}, nil, false},
		{"shared by the master, one creature hidden by the master", turnClearMap, []turnZombie{friend, hidFriend}, nil, true},
	}
	for _, row := range rows {
		t.Run(row.name, func(t *testing.T) {
			var alone []turnZombie
			for _, z := range row.zombies {
				if z.readable(row.mode) {
					alone = append(alone, z)
				}
			}
			full, only := newTurnWorld(t, row.mode, row.zombies), newTurnWorld(t, row.mode, alone)
			readable := restoredBy(row.zombies, row.named, func(z turnZombie) bool { return z.readable(row.mode) })
			named := func(tw *turnWorld) []string {
				return tw.idsOf(restoredBy(alone, row.named, func(turnZombie) bool { return true }))
			}
			if row.named == nil {
				named = func(*turnWorld) []string { return nil }
			}
			actor := func(tw *turnWorld) *person {
				if row.byMaster {
					return tw.master
				}
				return tw.caio
			}
			gotFull, err := full.preserve(actor(full), named(full)...)
			if err != nil {
				t.Fatalf("Preserve Life with the others: %v", err)
			}
			gotOnly, err := only.preserve(actor(only), named(only)...)
			if err != nil {
				t.Fatalf("Preserve Life without the others: %v", err)
			}
			restored := readable
			if row.byMaster { // the master's reader reads every creature
				restored = restoredBy(row.zombies, row.named, func(turnZombie) bool { return true })
				full.checkHeals("the master's answer", gotFull.GetChannelDivinity(), restored)
			} else {
				if a, b := same(gotFull), same(gotOnly); a != b {
					t.Errorf("the cleric's answer with the others differs from the one without them:\n%s\n%s", a, b)
				}
				full.checkHeals("the cleric's answer", gotFull.GetChannelDivinity(), readable)
				for _, h := range gotFull.GetChannelDivinity().GetHeals() {
					if h.HitPointsAfterKnown != nil || h.ProtoReflect().Has(h.ProtoReflect().Descriptor().Fields().ByNumber(5)) {
						t.Errorf("the cleric's answer gives the hit points after of %s", h.GetLabel())
					}
				}
			}
			for _, p := range []*person{full.caio, full.ana} {
				if a, b := same(full.line(p)), same(only.line(only.counterpart(p))); a != b {
					t.Errorf("%s's line with the others differs from the one without them:\n%s\n%s", p.name, a, b)
				}
			}
			full.checkHeals("the cleric's line", full.line(full.caio).GetChannelDivinity(), readable)
			if full.line(full.ana).GetChannelDivinity() != nil {
				t.Errorf("another player reads the numbers: %v", full.line(full.ana))
			}
			full.checkHeals("the master's line", full.line(full.master).GetChannelDivinity(), restored)
			for _, h := range full.line(full.master).GetChannelDivinity().GetHeals() {
				if h.HitPointsAfterKnown == nil {
					t.Errorf("the master's line gives no hit points after of %s", h.GetLabel())
				}
			}
			unread := labelsOfZombies(row.zombies, func(z turnZombie) bool { return !z.readable(row.mode) })
			reads := map[string]string{}
			if !row.byMaster {
				reads["the cleric's answer"] = jsonOf(gotFull)
			}
			for k, v := range full.reads(full.caio) {
				reads[k] = v
			}
			full.scan(unread, reads)
		})
	}
}

func TestPreserveLifeNamesOnlyWhatItsReaderMayRead(t *testing.T) {
	bandits := func(z turnZombie) turnZombie { z.key, z.hp = bandit, 1; return z }
	a := bandits(turnZombie{label: "Quimbul4321"})
	// named are the ids the cleric's player names: z (a creature in the scene), another combat's creature, or a readable one and z.
	type naming func(tw *turnWorld, z turnZombie) []string
	onlyZ := func(tw *turnWorld, z turnZombie) []string { return []string{tw.ids[z.label]} }
	unread := map[string]struct {
		mode   turnMap
		z      turnZombie
		scene  []turnZombie
		naming naming
	}{
		"hidden by the master":                                 {turnClearMap, bandits(turnZombie{label: "Quimbul4322", hidden: true}), nil, onlyZ},
		"hidden by the master, no map":                         {turnNoMap, bandits(turnZombie{label: "Quimbul4322", hidden: true}), nil, onlyZ},
		"out of sight on a fog map":                            {turnFogMap, bandits(turnZombie{label: "Quimbul4322", behind: true}), nil, onlyZ},
		"an undead hidden by the master":                       {turnClearMap, turnZombie{label: "Quimbul4322", hidden: true}, nil, onlyZ},
		"an undead out of sight on a fog map":                  {turnFogMap, turnZombie{label: "Quimbul4322", behind: true}, nil, onlyZ},
		"a creature of the cleric's side hidden by the master": {turnClearMap, bandits(turnZombie{label: "Xandruk4322", ally: true, hidden: true}), nil, onlyZ},
		"a creature of another combat": {turnClearMap, turnZombie{}, []turnZombie{a}, func(tw *turnWorld, _ turnZombie) []string {
			other := newTurnWorld(tw.t, turnClearMap, []turnZombie{bandits(turnZombie{label: "Quimbul4323"})})
			return []string{other.ids["Quimbul4323"]}
		}},
		"two named, one readable and one hidden by the master": {turnClearMap, bandits(turnZombie{label: "Quimbul4322", hidden: true}), nil, func(tw *turnWorld, z turnZombie) []string {
			return []string{tw.ids[a.label], tw.ids[z.label]}
		}},
		"two named, one hidden by the master and one readable, no map": {turnNoMap, bandits(turnZombie{label: "Quimbul4322", hidden: true}), nil, func(tw *turnWorld, z turnZombie) []string {
			return []string{tw.ids[z.label], tw.ids[a.label]}
		}},
	}
	for name, row := range unread {
		t.Run(name, func(t *testing.T) {
			scene := row.scene
			if scene == nil {
				scene = []turnZombie{a, row.z}
			}
			tw := newTurnWorld(t, row.mode, scene)
			before := tw.reads(tw.caio)
			_, got := tw.preserve(tw.caio, row.naming(tw, row.z)...)
			_, madeUp := tw.preserve(tw.caio, newKey())
			if got == nil || madeUp == nil {
				t.Fatalf("naming a creature that is not read: %v, naming a made-up id: %v, want two refusals", got, madeUp)
			}
			if g, m := refusalText(got), refusalText(madeUp); g != m {
				t.Errorf("the refusal for the creature differs from the one for a made-up id:\n%s\n%s", g, m)
			}
			if g := refusalText(got); !strings.Contains(g, "that creature is not in the combat") {
				t.Errorf("the refusal = %q, want the one of a creature that is not in the combat", g)
			}
			if row.z.label != "" {
				tw.scan([]string{row.z.label}, map[string]string{"the refusal": refusalText(got)})
			}
			// Nothing was spent or written: what the cleric's player reads is what it was, and the action still works.
			for k, v := range tw.reads(tw.caio) {
				if v != before[k] {
					t.Errorf("%s changed after the refusals", k)
				}
			}
			res, err := tw.preserve(tw.caio, tw.ids[a.label])
			if err != nil {
				t.Fatalf("Preserve Life after the refusals: %v", err)
			}
			tw.checkHeals("the answer after the refusals", res.GetChannelDivinity(), []string{a.label})
		})
	}
}

func TestPreserveLifeAnswersTheMasterWithWhatTheMastersReaderReads(t *testing.T) {
	for _, mode := range []turnMap{turnNoMap, turnClearMap, turnFogMap} {
		t.Run(fmt.Sprint("mode ", int(mode)), func(t *testing.T) {
			zs := []turnZombie{{label: "Quimbul4321", key: bandit, hp: 1}, {label: "Quimbul4322", key: bandit, hp: 1, hidden: true}, {label: "Quimbul4323", key: bandit, hp: 1, behind: true}}
			tw := newTurnWorld(t, mode, zs)
			res, err := tw.preserve(tw.master, tw.idsOf([]string{"Quimbul4321", "Quimbul4322", "Quimbul4323"})...)
			if err != nil {
				t.Fatalf("Preserve Life as the master: %v", err)
			}
			tw.checkHeals("the master's answer", res.GetChannelDivinity(), []string{"Quimbul4321", "Quimbul4322", "Quimbul4323"})
			for _, h := range res.GetChannelDivinity().GetHeals() {
				if h.HitPointsAfterKnown == nil || h.GetHitPointsAfterKnown() != 5 {
					t.Errorf("the master reads %s at %v after the action, want 5", h.GetLabel(), h.HitPointsAfterKnown)
				}
			}
			tw.checkHeals("the master's line", tw.line(tw.master).GetChannelDivinity(), []string{"Quimbul4321", "Quimbul4322", "Quimbul4323"})
		})
	}
}

func TestPreserveLifeGivesTheHitPointsAfterToTheOnesWhoReadThemAsNumbers(t *testing.T) {
	friend := turnZombie{label: "Xandruk4321", key: bandit, ally: true, hp: 1}
	tw := newTurnWorld(t, turnClearMap, []turnZombie{friend, {label: "Quimbul4321", key: bandit, hp: 1}})
	res, err := tw.preserve(tw.caio, tw.ids["Brisa"], tw.ids["Xandruk4321"], tw.ids["Quimbul4321"])
	if err != nil {
		t.Fatalf("Preserve Life: %v", err)
	}
	for _, h := range res.GetChannelDivinity().GetHeals() {
		if got, want := h.HitPointsAfterKnown != nil, h.GetLabel() == "Brisa"; got != want {
			t.Errorf("the cleric's player reads the hit points after of %s: %v, want %v", h.GetLabel(), got, want)
		}
	}
	for _, h := range tw.line(tw.caio).GetChannelDivinity().GetHeals() {
		if got, want := h.HitPointsAfterKnown != nil, h.GetLabel() == "Brisa"; got != want {
			t.Errorf("the cleric's line gives the hit points after of %s: %v, want %v", h.GetLabel(), got, want)
		}
	}
}

// The master acts for the cleric and names a creature it has hidden or that the cleric's player never saw; what the cleric's
// player reads afterwards in the line is what it reads when the action restored only the creatures that player may
// read, whether or not the master shows the creature later.
func TestPreserveLifeLineFollowsItsReaderWhenAnotherActs(t *testing.T) {
	a := turnZombie{label: "Quimbul4321", key: bandit, hp: 1}
	hid := turnZombie{label: "Quimbul4322", key: bandit, hp: 1, hidden: true}
	far := turnZombie{label: "Quimbul4322", key: bandit, hp: 1, behind: true}
	for _, row := range []struct {
		name string
		mode turnMap
		z    turnZombie
		show bool // the master shows the creature after the action
	}{
		{"hidden at the action, shown after", turnClearMap, hid, true},
		{"hidden at the action, shown after, on a fog map", turnFogMap, hid, true},
		{"hidden at the action, shown after, no map", turnNoMap, hid, true},
		{"out of the player's sight on a fog map", turnFogMap, far, false},
	} {
		t.Run(row.name, func(t *testing.T) {
			full, only := newTurnWorld(t, row.mode, []turnZombie{a, row.z}), newTurnWorld(t, row.mode, []turnZombie{a})
			key := newKey()
			if _, err := full.preserveWith(full.master, key, full.idsOf([]string{a.label, row.z.label})...); err != nil {
				t.Fatalf("Preserve Life with the creature: %v", err)
			}
			if _, err := only.preserveWith(only.master, key, only.idsOf([]string{a.label})...); err != nil {
				t.Fatalf("Preserve Life without it: %v", err)
			}
			if row.show {
				full.setHidden(row.z.label, false)
			}
			full.checkHeals("the cleric's line", full.line(full.caio).GetChannelDivinity(), []string{a.label})
			if x, y := same(full.line(full.caio)), same(only.line(only.caio)); x != y {
				t.Errorf("the cleric's line differs from the one of the action without the creature:\n%s\n%s", x, y)
			}
			full.scan([]string{row.z.label}, map[string]string{"the cleric's line": jsonOf(full.line(full.caio))})
		})
	}
}

func TestPreserveLifeLineFollowsTheMastersSwitchAfterTheAction(t *testing.T) {
	a, b := turnZombie{label: "Quimbul4321", key: bandit, hp: 1}, turnZombie{label: "Quimbul4322", key: bandit, hp: 1}
	both := []string{"Quimbul4321", "Quimbul4322"}
	t.Run("the master hides a listed creature", func(t *testing.T) {
		tw, only := newTurnWorld(t, turnClearMap, []turnZombie{a, b}), newTurnWorld(t, turnClearMap, []turnZombie{a})
		key := newKey()
		if _, err := tw.preserveWith(tw.caio, key, tw.idsOf(both)...); err != nil {
			t.Fatalf("Preserve Life: %v", err)
		}
		if _, err := only.preserveWith(only.caio, key, only.idsOf(both[:1])...); err != nil {
			t.Fatalf("Preserve Life without the second: %v", err)
		}
		tw.checkHeals("the line", tw.line(tw.caio).GetChannelDivinity(), both)
		tw.setHidden("Quimbul4322", true)
		if x, y := same(tw.line(tw.caio)), same(only.line(only.caio)); x != y {
			t.Errorf("the line after the switch differs from the one of the action without the second:\n%s\n%s", x, y)
		}
		retried, err := tw.preserveWith(tw.caio, key, tw.idsOf(both)...)
		if err != nil {
			t.Fatalf("retry: %v", err)
		}
		retriedOnly, err := only.preserveWith(only.caio, key, only.idsOf(both[:1])...)
		if err != nil {
			t.Fatalf("retry without the second: %v", err)
		}
		if x, y := same(retried), same(retriedOnly); x != y {
			t.Errorf("the retried answer differs from the one of the action without the second:\n%s\n%s", x, y)
		}
		tw.scan([]string{"Quimbul4322"}, tw.reads(tw.caio))
	})
	t.Run("the fog changes on a fog map", func(t *testing.T) {
		tw, stays := newTurnWorld(t, turnFogMap, []turnZombie{a}), newTurnWorld(t, turnFogMap, []turnZombie{a})
		key := newKey()
		if _, err := tw.preserveWith(tw.caio, key, tw.ids["Quimbul4321"]); err != nil {
			t.Fatalf("Preserve Life: %v", err)
		}
		if _, err := stays.preserveWith(stays.caio, key, stays.ids["Quimbul4321"]); err != nil {
			t.Fatalf("Preserve Life in the other scene: %v", err)
		}
		tw.setFog(false)
		if x, y := same(tw.line(tw.caio)), same(stays.line(stays.caio)); x != y {
			t.Errorf("the line after the fog is off differs from the one of the scene whose fog stays on:\n%s\n%s", x, y)
		}
		retried, err := tw.preserveWith(tw.caio, key, tw.ids["Quimbul4321"])
		if err != nil {
			t.Fatalf("retry: %v", err)
		}
		retriedOther, err := stays.preserveWith(stays.caio, key, stays.ids["Quimbul4321"])
		if err != nil {
			t.Fatalf("retry in the other scene: %v", err)
		}
		if x, y := same(retried), same(retriedOther); x != y {
			t.Errorf("the retried answer differs from the one of the scene whose fog stays on:\n%s\n%s", x, y)
		}
		tw.checkHeals("the retried answer", retried.GetChannelDivinity(), []string{"Quimbul4321"})
	})
}

func TestPreserveLifeStoredWithoutRecordsListsWhatItsMarkSays(t *testing.T) {
	zs := []turnZombie{{label: "Quimbul4321", key: bandit, hp: 1}, {label: "Brisa", party: true, hp: 1}}
	both := []string{"Brisa", "Quimbul4321"}
	for _, row := range []struct {
		name     string
		mode     turnMap
		underFog *bool
		want     []string
	}{
		{"an old line on a fog map: its reader's rule", turnFogMap, nil, []string{"Brisa"}},
		{"an old line on a map: its reader's rule", turnClearMap, nil, []string{"Brisa"}},
		{"on a map, made under the fog", turnFogMap, new(true), []string{"Brisa"}},
		{"on a map, made with no fog", turnClearMap, new(false), both},
		{"with no map", turnNoMap, nil, both},
	} {
		t.Run(row.name, func(t *testing.T) {
			tw := newTurnWorld(t, row.mode, zs)
			key := newKey()
			if _, err := tw.preserveWith(tw.caio, key, tw.idsOf(both)...); err != nil {
				t.Fatalf("Preserve Life: %v", err)
			}
			tw.forgetTheMarks(row.underFog)
			tw.checkHeals("the cleric's line", tw.line(tw.caio).GetChannelDivinity(), row.want)
			retried, err := tw.preserveWith(tw.caio, key, tw.idsOf(both)...)
			if err != nil {
				t.Fatalf("retry: %v", err)
			}
			tw.checkHeals("the retried answer", retried.GetChannelDivinity(), row.want)
			tw.checkHeals("the master's line", tw.line(tw.master).GetChannelDivinity(), both)
		})
	}
}
