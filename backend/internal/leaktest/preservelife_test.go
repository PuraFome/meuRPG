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

// restored are the labels of the creatures an action restores: the named ones, or the hurt ones of the cleric's side.
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
}

func TestPreserveLifeResultFollowsTheCreaturesItsReaderMayRead(t *testing.T) {
	a := turnZombie{label: "Quimbul4321", key: bandit, hp: 1}
	hid := turnZombie{label: "Quimbul4322", key: bandit, hp: 1, hidden: true}
	far := turnZombie{label: "Quimbul4322", key: bandit, hp: 1, behind: true}
	half := turnZombie{label: "Quimbul4321", key: bandit, hp: 5}
	friend := turnZombie{label: "Xandruk4321", key: bandit, ally: true, hp: 1}
	hidFriend := turnZombie{label: "Xandruk4322", key: bandit, ally: true, hp: 1, hidden: true}
	rows := []preserveRow{
		{"named, one hidden by the master", turnClearMap, []turnZombie{a, hid}, []string{"Quimbul4321"}},
		{"named, one out of sight on a fog map", turnFogMap, []turnZombie{a, far}, []string{"Quimbul4321"}},
		{"named, one hidden by the master, on a fog map", turnFogMap, []turnZombie{a, hid}, []string{"Quimbul4321"}},
		{"named, no map, one hidden by the master", turnNoMap, []turnZombie{a, hid}, []string{"Quimbul4321"}},
		{"named, the others are all readable", turnClearMap, []turnZombie{a, friend}, []string{"Quimbul4321", "Xandruk4321"}},
		{"named, at half the maximum", turnClearMap, []turnZombie{half, hid}, []string{"Quimbul4321"}},
		{"shared, one hidden by the master", turnClearMap, []turnZombie{friend, hidFriend}, nil},
		{"shared, one hidden by the master, on a fog map", turnFogMap, []turnZombie{friend, hidFriend}, nil},
		{"shared, no map, one hidden by the master", turnNoMap, []turnZombie{friend, hidFriend}, nil},
		{"shared, with a creature of the other side among them", turnClearMap, []turnZombie{friend, a, hidFriend}, nil},
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
			gotFull, err := full.preserve(full.caio, named(full)...)
			if err != nil {
				t.Fatalf("Preserve Life with the others: %v", err)
			}
			gotOnly, err := only.preserve(only.caio, named(only)...)
			if err != nil {
				t.Fatalf("Preserve Life without the others: %v", err)
			}
			if a, b := same(gotFull), same(gotOnly); a != b {
				t.Errorf("the cleric's answer with the others differs from the one without them:\n%s\n%s", a, b)
			}
			full.checkHeals("the cleric's answer", gotFull.GetChannelDivinity(), readable)
			for _, h := range gotFull.GetChannelDivinity().GetHeals() {
				if h.HitPointsAfterKnown != nil || h.GetHitPointsAfter() != 0 {
					t.Errorf("the cleric's answer gives the hit points after of %s", h.GetLabel())
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
			unread := labelsOfZombies(row.zombies, func(z turnZombie) bool { return !z.readable(row.mode) })
			reads := map[string]string{"the cleric's answer": jsonOf(gotFull)}
			for k, v := range full.reads(full.caio) {
				reads[k] = v
			}
			full.scan(unread, reads)
		})
	}
}

func TestPreserveLifeNamesOnlyWhatItsReaderMayRead(t *testing.T) {
	bandits := func(z turnZombie) turnZombie { z.key, z.hp = bandit, 1; return z }
	unread := map[string]struct {
		mode turnMap
		z    turnZombie
	}{
		"hidden by the master":                                 {turnClearMap, bandits(turnZombie{label: "Quimbul4322", hidden: true})},
		"hidden by the master, no map":                         {turnNoMap, bandits(turnZombie{label: "Quimbul4322", hidden: true})},
		"out of sight on a fog map":                            {turnFogMap, bandits(turnZombie{label: "Quimbul4322", behind: true})},
		"an undead hidden by the master":                       {turnClearMap, turnZombie{label: "Quimbul4322", hidden: true}},
		"an undead out of sight on a fog map":                  {turnFogMap, turnZombie{label: "Quimbul4322", behind: true}},
		"a creature of the cleric's side hidden by the master": {turnClearMap, bandits(turnZombie{label: "Xandruk4322", ally: true, hidden: true})},
	}
	for name, row := range unread {
		t.Run(name, func(t *testing.T) {
			a := bandits(turnZombie{label: "Quimbul4321"})
			tw := newTurnWorld(t, row.mode, []turnZombie{a, row.z})
			before := tw.reads(tw.caio)
			_, got := tw.preserve(tw.caio, tw.ids[row.z.label])
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
			tw.scan([]string{row.z.label}, map[string]string{"the refusal": refusalText(got)})
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

func TestPreserveLifeAnswersTheMasterWithEveryCreatureAndItsHitPoints(t *testing.T) {
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

func TestPreserveLifeLineFollowsTheMastersSwitchAfterTheAction(t *testing.T) {
	a, b := turnZombie{label: "Quimbul4321", key: bandit, hp: 1}, turnZombie{label: "Quimbul4322", key: bandit, hp: 1}
	both := []string{"Quimbul4321", "Quimbul4322"}
	t.Run("the master hides a listed creature", func(t *testing.T) {
		tw := newTurnWorld(t, turnClearMap, []turnZombie{a, b})
		key := newKey()
		if _, err := tw.preserveWith(tw.caio, key, tw.idsOf(both)...); err != nil {
			t.Fatalf("Preserve Life: %v", err)
		}
		tw.checkHeals("the line", tw.line(tw.caio).GetChannelDivinity(), both)
		tw.setHidden("Quimbul4322", true)
		tw.checkHeals("the line after the switch", tw.line(tw.caio).GetChannelDivinity(), []string{"Quimbul4321"})
		retried, err := tw.preserveWith(tw.caio, key, tw.idsOf(both)...)
		if err != nil {
			t.Fatalf("retry: %v", err)
		}
		tw.checkHeals("the retried answer", retried.GetChannelDivinity(), []string{"Quimbul4321"})
		tw.scan([]string{"Quimbul4322"}, tw.reads(tw.caio))
	})
	t.Run("the fog changes on a fog map", func(t *testing.T) {
		tw := newTurnWorld(t, turnFogMap, []turnZombie{a})
		key := newKey()
		if _, err := tw.preserveWith(tw.caio, key, tw.ids["Quimbul4321"]); err != nil {
			t.Fatalf("Preserve Life: %v", err)
		}
		tw.setFog(false)
		tw.checkHeals("the line after the fog is off", tw.line(tw.caio).GetChannelDivinity(), []string{"Quimbul4321"})
		retried, err := tw.preserveWith(tw.caio, key, tw.ids["Quimbul4321"])
		if err != nil {
			t.Fatalf("retry: %v", err)
		}
		tw.checkHeals("the retried answer", retried.GetChannelDivinity(), []string{"Quimbul4321"})
	})
}

func TestPreserveLifeStoredBeforeItsRecordsListsWhatItsMarkSays(t *testing.T) {
	zs := []turnZombie{{label: "Quimbul4321", key: bandit, hp: 1}, {label: "Brisa", party: true, hp: 1}}
	both := []string{"Brisa", "Quimbul4321"}
	for _, row := range []struct {
		name     string
		mode     turnMap
		underFog *bool
		want     []string
	}{
		{"on a fog map, no mark: the party's characters", turnFogMap, nil, []string{"Brisa"}},
		{"on a map, no mark: the party's characters", turnClearMap, nil, []string{"Brisa"}},
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
