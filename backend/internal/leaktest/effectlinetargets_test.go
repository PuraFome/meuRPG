package leaktest

import (
	"slices"
	"strings"
	"testing"

	"google.golang.org/protobuf/proto"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	mapsv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/maps/v1"
	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// The log line of a lasting effect on several creatures (RN-10): a player reads the label of a target
// only if they saw that creature when the line was written, whoever else the line names. The table is a
// cave in three parts: Ana's character in the north-west, Caio's in the east, and a closed room in the
// south-west that no character sees. Ana casts; Caio is the reader whose sight decides what the line
// lists; the master reads everything.

// effTable is a cave in three parts (see above) with Ana's and Caio's characters and three NPCs: A in the
// north-west (Ana sees it), B in the east (Caio sees it) and C in the closed room (nobody sees it).
type effTable struct {
	*world
	enc   *playv1.Encounter
	ids   map[string]string // symbol -> combatant id
	label map[string]string // symbol -> label
	seen  map[*person]map[string]bool
}

var effSquares = map[string][2]int32{"A": {3, 7}, "B": {18, 5}, "C": {3, 12}, "ana": {3, 5}, "caio": {20, 12}}

func newEffTable(t *testing.T, hideB bool) *effTable {
	t.Helper()
	w := &world{stack: newStack(t), secrets: newSecrets(), pts: map[string]*mapsv1.MapPoint{}, puzzles: map[string]*playv1.Puzzle{}, notes: map[string]string{}}
	z := &effTable{world: w, ids: map[string]string{}, label: map[string]string{}, seen: map[*person]map[string]bool{}}
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("building the fixture: %v", r)
		}
	}()
	ctx := t.Context()
	w.buildPeople()
	m := w.master
	img := w.image("effect-map-image", 240, 160)
	w.fogMap = must(m.maps.CreateMap(ctx, rq(&mapsv1.CreateMapRequest{CampaignId: w.campaign, Name: "Caverna", ImageId: img}))).GetMap().GetId()
	must(m.maps.SetMapRevealed(ctx, rq(&mapsv1.SetMapRevealedRequest{CampaignId: w.campaign, MapId: w.fogMap, Revealed: true})))
	must(m.maps.SetMapGrid(ctx, rq(&mapsv1.SetMapGridRequest{CampaignId: w.campaign, MapId: w.fogMap, Columns: caveColumns})))
	var wall [][2]int32
	for row := range caveGrid.Rows {
		wall = append(wall, [2]int32{12, int32(row)})
	}
	for col := range 12 {
		wall = append(wall, [2]int32{int32(col), 9})
	}
	w.paint(w.fogMap, mapsv1.MapLayer_MAP_LAYER_WALL, 1, wall)
	must(m.maps.SetMapFog(ctx, rq(&mapsv1.SetMapFogRequest{CampaignId: w.campaign, MapId: w.fogMap, FogEnabled: new(true), GroupVision: new(false), BaseLight: mapsv1.LightLevel_LIGHT_LEVEL_DIM.Enum()})))
	w.session = must(m.play.StartGameSession(ctx, rq(&playv1.StartGameSessionRequest{CampaignId: w.campaign}))).GetGameSession().GetId()
	must(m.play.SetCurrentMap(ctx, rq(&playv1.SetCurrentMapRequest{CampaignId: w.campaign, MapId: w.fogMap})))

	anaPC := w.pc(w.ana, "Pensantus", "race:gnome")
	caioPC := w.pc(w.caio, "Vael", "race:gnome")
	chars := map[string]*charactersv1.Character{"ana": anaPC, "caio": caioPC}
	participants := []*playv1.Participant{{CharacterId: anaPC.GetId()}, {CharacterId: caioPC.GetId()}}
	for i, sym := range []string{"A", "B", "C"} {
		npc := must(m.characters.CreateCharacter(ctx, rq(&charactersv1.CreateCharacterRequest{
			CampaignId: w.campaign, Kind: charactersv1.CharacterKind_CHARACTER_KIND_MINION, Name: []string{"Zorvak", "Quimbul", "Xandruk"}[i],
			Sheet: &charactersv1.CharacterSheet{Content: &charactersv1.CharacterSheet_Basic{Basic: &charactersv1.BasicSheet{HitPointsMax: 40, ArmorClass: 12, SpeedFt: 30}}},
		}))).GetCharacter()
		chars[sym] = npc
		participants = append(participants, &playv1.Participant{CharacterId: npc.GetId(), Hidden: new(false)})
	}
	e := must(m.combat.StartEncounter(ctx, rq(&playv1.StartEncounterRequest{CampaignId: w.campaign, IdempotencyKey: newKey(), Name: "Caverna", Participants: participants}))).GetEncounter()
	for _, cb := range e.GetCombatants() {
		for sym, ch := range chars {
			if cb.GetCharacterId() == ch.GetId() {
				z.ids[sym], z.label[sym] = cb.GetId(), cb.GetLabel()
			}
		}
		if cb.GetKind() == playv1.CombatantKind_COMBATANT_KIND_PLAYER {
			must(m.combat.SubmitInitiative(ctx, rq(&playv1.SubmitInitiativeRequest{
				CampaignId: w.campaign, EncounterId: e.GetId(), CombatantId: cb.GetId(), IdempotencyKey: newKey(),
				Roll: &playv1.SubmitInitiativeRequest_D20Face{D20Face: 12},
			})))
		}
	}
	z.enc = e
	for sym, sq := range effSquares {
		z.put(sym, sq[0], sq[1])
	}
	w.place(w.fogMap, anaPC.GetId(), 3, 5)
	w.place(w.fogMap, caioPC.GetId(), 20, 12)
	for _, sym := range []string{"A", "B", "C"} {
		w.place(w.fogMap, chars[sym].GetId(), 0, 0)
		w.showToken(w.fogMap, chars[sym].GetId())
	}
	if hideB {
		must(m.combat.SetCombatantHidden(ctx, rq(&playv1.SetCombatantHiddenRequest{CampaignId: w.campaign, EncounterId: e.GetId(), CombatantId: z.ids["B"], IdempotencyKey: newKey(), Hidden: true})))
	}
	z.enc = must(m.combat.BeginCombat(ctx, rq(&playv1.BeginCombatRequest{CampaignId: w.campaign, EncounterId: e.GetId(), IdempotencyKey: newKey()}))).GetEncounter()
	return z
}

func (z *effTable) put(sym string, col, row int32) {
	must(z.master.combat.MoveCombatant(z.t.Context(), rq(&playv1.MoveCombatantRequest{
		CampaignId: z.campaign, EncounterId: z.enc.GetId(), CombatantId: z.ids[sym], IdempotencyKey: newKey(), Col: col, Row: row, Forced: true,
	})))
}

func (z *effTable) log(p *person) *playv1.ListCombatLogResponse {
	return must(p.combat.ListCombatLog(z.t.Context(), rq(&playv1.ListCombatLogRequest{CampaignId: z.campaign, EncounterId: z.enc.GetId()})))
}

func (z *effTable) readers() map[string]*person {
	return map[string]*person{"ana": z.ana, "caio": z.caio, "master": z.master}
}

// mark remembers the entries every reader holds now, so that fresh lists the ones a step added.
func (z *effTable) mark() {
	for _, p := range z.readers() {
		z.seen[p] = map[string]bool{}
		for _, r := range z.log(p).GetRounds() {
			for _, en := range r.GetEntries() {
				z.seen[p][en.GetId()] = true
			}
		}
	}
}

// effectEntries are the effect lines of the person's log that are not marked, and the whole reply as text.
func (z *effTable) effectEntries(p *person) (entries []*playv1.CombatLogEntry, reply string) {
	res := z.log(p)
	for _, r := range res.GetRounds() {
		for _, en := range r.GetEntries() {
			if en.GetKind() == playv1.CombatLogKind_COMBAT_LOG_KIND_EFFECT && !z.seen[p][en.GetId()] {
				entries = append(entries, en)
			}
		}
	}
	return entries, jsonOf(res)
}

// entriesOf are the effect lines of the change that the person's log holds since the mark, as the person reads them:
// the line's own id, time and round set aside.
func (z *effTable) entriesOf(p *person, change playv1.CombatLogEffectChange) []*playv1.CombatLogEntry {
	entries, _ := z.effectEntries(p)
	var out []*playv1.CombatLogEntry
	for _, en := range entries {
		if en.GetEffect().GetChange() == change {
			c := proto.Clone(en).(*playv1.CombatLogEntry)
			c.Id, c.At, c.Round = "", nil, 0
			// The order of an effect's targets is the order of its rows, which the combat does not fix: the labels
			// are compared as a set, and the first target is one of them.
			if !slices.Contains(c.GetEffect().GetTargetLabels(), c.GetTargetLabel()) && len(c.GetEffect().GetTargetLabels()) > 0 {
				z.t.Errorf("the line's target %q is not among its labels %v", c.GetTargetLabel(), c.GetEffect().GetTargetLabels())
			}
			slices.Sort(c.GetEffect().TargetLabels)
			if len(c.GetEffect().GetTargetLabels()) > 1 {
				c.TargetId, c.TargetLabel = "", ""
			}
			out = append(out, c)
		}
	}
	return out
}

// clear ends the effects the combat holds, so that the next casting starts from none.
func (z *effTable) clear() {
	z.t.Helper()
	ctx := z.t.Context()
	for _, ef := range must(z.master.lasting.ListLastingEffects(ctx, rq(&playv1.ListLastingEffectsRequest{CampaignId: z.campaign, EncounterId: z.enc.GetId()}))).GetEffects() {
		must(z.master.lasting.EndLastingEffect(ctx, rq(&playv1.EndLastingEffectRequest{
			CampaignId: z.campaign, EncounterId: z.enc.GetId(), IdempotencyKey: newKey(), EffectId: ef.GetId(), Scope: playv1.EffectEndScope_EFFECT_END_SCOPE_THIS,
		})))
	}
}

func (z *effTable) cast(caster string, targets []string, audience playv1.EffectAudience, rounds int32) string {
	z.t.Helper()
	z.clear()
	var ids []string
	for _, s := range targets {
		ids = append(ids, z.ids[s])
	}
	req := &playv1.AddLastingEffectRequest{
		CampaignId: z.campaign, EncounterId: z.enc.GetId(), IdempotencyKey: newKey(), CatalogKey: "spell:bless", TargetIds: ids, CasterId: z.ids[caster],
		Duration: &playv1.EffectDurationChoice{Kind: playv1.EffectDurationKind_EFFECT_DURATION_KIND_ROUNDS, Rounds: rounds, AnchorCombatantId: z.ids[caster]},
		Audience: audience,
	}
	must(z.master.lasting.AddLastingEffect(z.t.Context(), rq(req)))
	held := must(z.master.lasting.ListLastingEffects(z.t.Context(), rq(&playv1.ListLastingEffectsRequest{CampaignId: z.campaign, EncounterId: z.enc.GetId()}))).GetEffects()
	if len(held) == 0 {
		z.t.Fatalf("cast %v on %v made no effect", caster, targets)
	}
	return held[0].GetId()
}

// effKind is a kind of line an effect writes: act casts the effect on the targets and brings the line about; the lines are
// read from the mark act sets.
type effKind struct {
	name   string
	change playv1.CombatLogEffectChange
	only   bool // the kind writes no line but the effect's
	act    func(z *effTable, targets []string)
}

var (
	effAdded   = playv1.CombatLogEffectChange_COMBAT_LOG_EFFECT_CHANGE_ADDED
	effEnded   = playv1.CombatLogEffectChange_COMBAT_LOG_EFFECT_CHANGE_ENDED
	effChanged = playv1.CombatLogEffectChange_COMBAT_LOG_EFFECT_CHANGE_DURATION_CHANGED
)

var effKinds = []effKind{
	{"added outside a cast", effAdded, true, func(z *effTable, ts []string) {
		z.mark()
		z.cast("ana", ts, playv1.EffectAudience_EFFECT_AUDIENCE_ALL, 10)
	}},
	{"duration changed", effChanged, true, func(z *effTable, ts []string) {
		id := z.cast("ana", ts, playv1.EffectAudience_EFFECT_AUDIENCE_ALL, 10)
		z.mark()
		must(z.master.lasting.ChangeLastingEffectDuration(z.t.Context(), rq(&playv1.ChangeLastingEffectDurationRequest{
			CampaignId: z.campaign, EncounterId: z.enc.GetId(), IdempotencyKey: newKey(), EffectId: id,
			Duration: &playv1.EffectDurationChoice{Kind: playv1.EffectDurationKind_EFFECT_DURATION_KIND_ROUNDS, Rounds: 5, AnchorCombatantId: z.ids["ana"]},
		})))
	}},
	{"ended by the master", effEnded, true, func(z *effTable, ts []string) {
		id := z.cast("ana", ts, playv1.EffectAudience_EFFECT_AUDIENCE_ALL, 10)
		z.mark()
		must(z.master.lasting.EndLastingEffect(z.t.Context(), rq(&playv1.EndLastingEffectRequest{
			CampaignId: z.campaign, EncounterId: z.enc.GetId(), IdempotencyKey: newKey(), EffectId: id, Scope: playv1.EffectEndScope_EFFECT_END_SCOPE_THIS,
		})))
	}},
	{"ended with the concentration", effEnded, false, func(z *effTable, ts []string) {
		z.cast("ana", ts, playv1.EffectAudience_EFFECT_AUDIENCE_ALL, 10)
		z.mark()
		must(z.master.combat.EndConcentration(z.t.Context(), rq(&playv1.EndConcentrationRequest{
			CampaignId: z.campaign, EncounterId: z.enc.GetId(), CombatantId: z.ids["ana"], IdempotencyKey: newKey(),
		})))
	}},
	{"ended when its time runs out", effEnded, false, func(z *effTable, ts []string) {
		z.cast("ana", ts, playv1.EffectAudience_EFFECT_AUDIENCE_ALL, 1)
		z.mark()
		z.untilEnded()
	}},
}

// untilEnded passes turns until the master's log holds an ended line of an effect since the mark.
func (z *effTable) untilEnded() {
	ctx := z.t.Context()
	for range 30 {
		if len(z.entriesOf(z.master, effEnded)) > 0 {
			return
		}
		cur := must(z.master.combat.GetEncounter(ctx, rq(&playv1.GetEncounterRequest{CampaignId: z.campaign}))).GetEncounter()
		must(z.master.combat.EndTurn(ctx, rq(&playv1.EndTurnRequest{CampaignId: z.campaign, EncounterId: z.enc.GetId(), IdempotencyKey: newKey(), ExpectedCombatantId: cur.GetCurrentCombatantId(), ExpectedRound: cur.GetRound()})))
	}
	z.t.Fatal("the effect never ended")
}

// effPair says a reader's line for the targets is the line for the ones the reader may read alone.
type effPair struct {
	name   string
	reader string
	all    []string
	own    []string
}

var effPairs = []effPair{
	{"an NPC seen and one not, the seen one first", "ana", []string{"A", "B"}, []string{"A"}},
	{"an NPC seen and one not, the seen one second", "ana", []string{"B", "A"}, []string{"A"}},
	{"an NPC seen and two not, the seen one in the middle", "caio", []string{"A", "B", "C"}, []string{"B"}},
	{"an NPC seen and one not, for the other reader", "caio", []string{"A", "B"}, []string{"B"}},
	{"a character and an NPC not seen", "caio", []string{"caio", "A"}, []string{"caio"}},
	{"an NPC not seen and a character", "ana", []string{"B", "caio"}, []string{"caio"}},
	{"a character, an NPC seen and one not", "caio", []string{"caio", "B", "A"}, []string{"caio", "B"}},
}

func TestTheLineOfAnEffectForAReaderIsTheLineOfTheTargetsTheReaderMayRead(t *testing.T) {
	for _, k := range effKinds {
		t.Run(k.name, func(t *testing.T) {
			z := newEffTable(t, false)
			for _, pr := range effPairs {
				p := z.readers()[pr.reader]
				k.act(z, pr.all)
				all := z.entriesOf(p, k.change)
				k.act(z, pr.own)
				own := z.entriesOf(p, k.change)
				if len(own) == 0 {
					t.Fatalf("%s: %s reads no line for %v: the comparison proves nothing", pr.name, pr.reader, pr.own)
				}
				if len(all) != len(own) {
					t.Errorf("%s: %s reads %d lines for %v, want %d", pr.name, pr.reader, len(all), pr.all, len(own))
					continue
				}
				for i := range all {
					if !proto.Equal(all[i], own[i]) {
						t.Errorf("%s: %s reads %v for %v, want %v", pr.name, pr.reader, all[i], pr.all, own[i])
					}
				}
				_, reply := z.effectEntries(p)
				for _, s := range []string{"A", "B", "C"} {
					if !slices.Contains(pr.all, s) || slices.Contains(pr.own, s) {
						continue
					}
					if strings.Contains(reply, z.label[s]) {
						t.Errorf("%s: %s's reply holds the label of %s, which they did not see", pr.name, pr.reader, s)
					}
				}
			}
			// The master reads every label.
			k.act(z, []string{"A", "B", "C"})
			for _, en := range z.entriesOf(z.master, k.change) {
				if got := len(en.GetEffect().GetTargetLabels()); got != 0 && got != 3 {
					t.Errorf("the master reads %d labels, want 3", got)
				}
			}
		})
	}
}

// A line no reader may read a target of is not theirs: no entry and no move of their log's number.
func TestAnEffectLineOfTargetsAReaderMayNotReadIsNotInTheirLog(t *testing.T) {
	for _, k := range effKinds {
		t.Run(k.name, func(t *testing.T) {
			z := newEffTable(t, false)
			for _, c := range []struct {
				reader  string
				targets []string
			}{{"caio", []string{"A", "C"}}, {"ana", []string{"B", "C"}}, {"caio", []string{"C", "A"}}} {
				p := z.readers()[c.reader]
				z.clear()
				before := z.log(p).GetLastSeq()
				k.act(z, c.targets)
				if got := z.entriesOf(p, k.change); len(got) != 0 {
					t.Errorf("%s reads %d lines for %v, want none", c.reader, len(got), c.targets)
				}
				if after := z.log(p).GetLastSeq(); k.only && after != before {
					t.Errorf("%s's log number moved from %d to %d", c.reader, before, after)
				}
			}
		})
	}
}

// What an effect line is for each reader when the line is not one the players read alone.
func TestAnEffectLineKeepsItsAudienceWhenTheTargetsAreNotAllReadable(t *testing.T) {
	z := newEffTable(t, false)
	both := []string{"ana", "caio"}
	// An NPC that nobody sees casts: neither player reads the line, the master does.
	z.mark()
	z.cast("C", both, playv1.EffectAudience_EFFECT_AUDIENCE_ALL, 10)
	for name, p := range z.readers() {
		got := z.entriesOf(p, effAdded)
		if want := name == "master"; (len(got) > 0) != want {
			t.Errorf("an NPC nobody sees casts: %s reads %d lines", name, len(got))
		}
	}
	// An effect for its targets' players alone, on NPCs: only the master reads it.
	z.mark()
	z.cast("ana", []string{"A", "B"}, playv1.EffectAudience_EFFECT_AUDIENCE_OWNER, 10)
	for name, p := range z.readers() {
		if got := z.entriesOf(p, effAdded); (len(got) > 0) != (name == "master") {
			t.Errorf("an effect for its owner on NPCs: %s reads %d lines", name, len(got))
		}
	}
	// One target: the line is the readers' who see it.
	z.mark()
	z.cast("ana", []string{"A"}, playv1.EffectAudience_EFFECT_AUDIENCE_ALL, 10)
	for name, want := range map[string]bool{"ana": true, "caio": false, "master": true} {
		if got := z.entriesOf(z.readers()[name], effAdded); (len(got) > 0) != want {
			t.Errorf("one target: %s reads %d lines", name, len(got))
		}
	}
}

// A target the master hid is left out of every player's line, and the master reads it.
func TestAnEffectLineLeavesOutATargetTheMasterHid(t *testing.T) {
	z := newEffTable(t, true)
	z.mark()
	z.cast("ana", []string{"A", "B"}, playv1.EffectAudience_EFFECT_AUDIENCE_ALL, 10)
	want := map[string][]string{"ana": {z.label["A"]}, "caio": nil, "master": {z.label["A"], z.label["B"]}}
	for name, p := range z.readers() {
		var got []string
		for _, en := range z.entriesOf(p, effAdded) {
			got = append(got, en.GetEffect().GetTargetLabels()...)
		}
		slices.Sort(got)
		slices.Sort(want[name])
		if !slices.Equal(got, want[name]) {
			t.Errorf("%s reads the labels %v, want %v", name, got, want[name])
		}
	}
}

// A line keeps the sight of the moment it was written: a creature that leaves a reader's sight afterwards stays in it.
func TestTheLineOfAnEffectKeepsWhoSawEachTargetWhenItWasWritten(t *testing.T) {
	z := newEffTable(t, false)
	z.mark()
	z.cast("ana", []string{"A", "B"}, playv1.EffectAudience_EFFECT_AUDIENCE_ALL, 10)
	before := z.entriesOf(z.ana, effAdded)
	z.put("A", 3, 12)
	after := z.entriesOf(z.ana, effAdded)
	if len(before) != 1 || len(after) != 1 || !proto.Equal(before[0], after[0]) {
		t.Errorf("Ana's line changed when A left her sight: %v then %v", before, after)
	}
}
