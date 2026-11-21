package leaktest

import (
	"cmp"
	"regexp"
	"slices"
	"testing"

	"google.golang.org/protobuf/proto"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	mapsv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/maps/v1"
	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
)

// Turn Undead (RN-10, RN-20): the saves of the undead reach a reader only for the undead that reader may read. The
// answer lists the ones the cleric's player sees when it is made, the log line the ones the player saw at the action
// that the master has not hidden since; the master reads all.
// Each row runs the scene twice, with the undead the cleric's player may read among others and with those alone, and the
// whole answer and the whole line of the action are the same in both.

const (
	turnUndeadKey = "feature:channel-divinity-turn-undead"
	// turnFace is the d20 every undead saves with: below the cleric's DC, so each one in range is turned.
	turnFace = 5
)

type turnMap int

const (
	turnNoMap turnMap = iota
	turnClearMap
	turnFogMap
)

// turnZombie is one zombie of a scene: behind the wall (out of the player's sight on a fog map), or hidden by the master.
type turnZombie struct {
	label  string
	behind bool
	hidden bool
	// key is the creature of a monster ("monster:zombie" when empty); ally says the master put it on the cleric's side; party
	// says it is a character of the scene by its label ("Brisa", "Pensantus") that the row only hurts.
	key   string
	ally  bool
	party bool
	// hp is the hit points the master leaves the creature with before the action; 0 leaves it whole.
	hp int32
}

// readable says the cleric's player may read the zombie when the action is made.
func (z turnZombie) readable(m turnMap) bool {
	return !z.hidden && (m != turnFogMap || !z.behind)
}

// turnWorld is a combat with a Life cleric of Caio's, a wizard of Ana's and zombies, the cleric's turn first.
type turnWorld struct {
	*world
	mode   turnMap
	cleric string // the cleric's combatant
	ids    map[string]string
	count  int
	rolled bool
}

const turnWallCol = 5

func newTurnWorld(t *testing.T, mode turnMap, zombies []turnZombie) *turnWorld {
	t.Helper()
	w := &world{stack: newStackOf(t, false), secrets: newSecrets(), pts: map[string]*mapsv1.MapPoint{}, puzzles: map[string]*playv1.Puzzle{}, notes: map[string]string{}}
	tw := &turnWorld{world: w, mode: mode, ids: map[string]string{}}
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("building the fixture: %v", r)
		}
	}()
	ctx := t.Context()
	w.buildPeople()
	m := w.master
	if mode != turnNoMap {
		img := m.upload(w.campaign, "img.png", pngOf(240, 160, 3))
		w.fogMap = must(m.maps.CreateMap(ctx, rq(&mapsv1.CreateMapRequest{CampaignId: w.campaign, Name: "Estrada", ImageId: img}))).GetMap().GetId()
		must(m.maps.SetMapRevealed(ctx, rq(&mapsv1.SetMapRevealedRequest{CampaignId: w.campaign, MapId: w.fogMap, Revealed: true})))
		must(m.maps.SetMapGrid(ctx, rq(&mapsv1.SetMapGridRequest{CampaignId: w.campaign, MapId: w.fogMap, Columns: caveColumns})))
		var wall [][2]int32
		for row := range caveGrid.Rows {
			wall = append(wall, [2]int32{turnWallCol, int32(row)})
		}
		w.paint(w.fogMap, mapsv1.MapLayer_MAP_LAYER_WALL, 1, wall)
		tw.setFog(mode == turnFogMap)
	}
	w.session = must(m.play.StartGameSession(ctx, rq(&playv1.StartGameSessionRequest{CampaignId: w.campaign}))).GetGameSession().GetId()
	if mode != turnNoMap {
		must(m.play.SetCurrentMap(ctx, rq(&playv1.SetCurrentMapRequest{CampaignId: w.campaign, MapId: w.fogMap})))
	}

	cleric := must(w.caio.characters.CreateCharacter(ctx, rq(&charactersv1.CreateCharacterRequest{
		CampaignId: w.campaign, Kind: charactersv1.CharacterKind_CHARACTER_KIND_PLAYER, Name: "Brisa",
		Sheet: &charactersv1.CharacterSheet{Content: &charactersv1.CharacterSheet_Full{Full: &charactersv1.FullSheet{
			BaseScores: &rulesv1.AbilityScores{Strength: 10, Dexterity: 14, Constitution: 14, Intelligence: 10, Wisdom: 16, Charisma: 8}, RaceKey: "race:human",
			Classes:    []*charactersv1.ClassLevel{{ClassKey: "class:cleric", Level: 5, Subclass: &charactersv1.ClassLevel_SubclassKey{SubclassKey: "subclass:life"}}},
			WeaponKeys: []string{"equipment:mace"},
		}}},
	}))).GetCharacter()
	friend := w.pc(w.ana, "Pensantus", "race:human")
	if mode != turnNoMap {
		w.place(w.fogMap, cleric.GetId(), 3, 5)
		w.place(w.fogMap, friend.GetId(), 2, 6)
	}
	start := &playv1.StartEncounterRequest{
		CampaignId: w.campaign, IdempotencyKey: newKey(), Name: "Cripta", Mode: mode.encounterMode(),
		Participants: []*playv1.Participant{{CharacterId: cleric.GetId()}, {CharacterId: friend.GetId()}},
	}

	e := must(m.combat.StartEncounter(ctx, rq(start))).GetEncounter()
	initiative := func(id string, face int32) {
		must(m.combat.SubmitInitiative(ctx, rq(&playv1.SubmitInitiativeRequest{
			CampaignId: w.campaign, EncounterId: e.GetId(), CombatantId: id, IdempotencyKey: newKey(), Roll: &playv1.SubmitInitiativeRequest_D20Face{D20Face: face},
		})))
	}
	for _, c := range e.GetCombatants() {
		switch c.GetCharacterId() {
		case cleric.GetId():
			tw.cleric = c.GetId()
			initiative(c.GetId(), 20)
		case friend.GetId():
			initiative(c.GetId(), 10)
		}
		if c.GetCharacterId() == friend.GetId() {
			tw.ids["Pensantus"] = c.GetId()
		}
	}
	tw.ids["Brisa"] = tw.cleric
	for i, z := range zombies {
		hidden := z.hidden
		var id string
		if !z.party {
			key := cmp.Or(z.key, "monster:zombie")
			id = must(m.combat.AddMonsters(ctx, rq(&playv1.AddMonstersRequest{
				CampaignId: w.campaign, EncounterId: e.GetId(), IdempotencyKey: newKey(), CreatureKey: key, Count: 1, Name: z.label, Hidden: &hidden,
			}))).GetCombatantIds()[0]
			tw.ids[z.label] = id
			initiative(id, int32(2+i))
			if z.ally {
				must(m.combat.SetCombatantSide(ctx, rq(&playv1.SetCombatantSideRequest{CampaignId: w.campaign, EncounterId: e.GetId(), CombatantId: id, IdempotencyKey: newKey(), Side: playv1.CombatantSide_COMBATANT_SIDE_PARTY})))
			}
		}
		if mode != turnNoMap && !z.party {
			col := int32(4)
			if z.behind {
				col = turnWallCol + 1
			}
			must(m.combat.MoveCombatant(ctx, rq(&playv1.MoveCombatantRequest{CampaignId: w.campaign, EncounterId: e.GetId(), CombatantId: id, IdempotencyKey: newKey(), Col: col, Row: int32(5 + i), Forced: true})))
		}
	}
	tw.count = 0
	for _, z := range zombies {
		if !z.party && z.key == "" {
			tw.count++
		}
	}
	w.encounter = must(m.combat.BeginCombat(ctx, rq(&playv1.BeginCombatRequest{CampaignId: w.campaign, EncounterId: e.GetId(), IdempotencyKey: newKey()}))).GetEncounter()
	for _, z := range zombies {
		if z.hp > 0 && z.party {
			charID := map[string]string{"Brisa": cleric.GetId(), "Pensantus": friend.GetId()}[z.label]
			must(m.play.AdjustCharacterVitals(ctx, rq(&playv1.AdjustCharacterVitalsRequest{
				CampaignId: w.campaign, CharacterId: charID, IdempotencyKey: newKey(), ExpectedRevision: m.revisionOf(w, charID), HitPointsCurrent: new(z.hp),
			})))
		} else if z.hp > 0 {
			must(m.combat.AdjustCombatantHitPoints(ctx, rq(&playv1.AdjustCombatantHitPointsRequest{
				CampaignId: w.campaign, EncounterId: e.GetId(), CombatantId: tw.ids[z.label], IdempotencyKey: newKey(),
				Change: &playv1.AdjustCombatantHitPointsRequest_HitPoints{HitPoints: z.hp},
			})))
		}
	}
	return tw
}

func (tw *turnWorld) setFog(on bool) {
	must(tw.master.maps.SetMapFog(tw.t.Context(), rq(&mapsv1.SetMapFogRequest{CampaignId: tw.campaign, MapId: tw.fogMap, FogEnabled: new(on), BaseLight: mapsv1.LightLevel_LIGHT_LEVEL_BRIGHT.Enum()})))
}

func (tw *turnWorld) setHidden(label string, hidden bool) {
	must(tw.master.combat.SetCombatantHidden(tw.t.Context(), rq(&playv1.SetCombatantHiddenRequest{
		CampaignId: tw.campaign, EncounterId: tw.encounter.GetId(), CombatantId: tw.ids[label], IdempotencyKey: newKey(), Hidden: hidden,
	})))
}

// turn makes the cleric take Turn Undead as p, with every undead saving on turnFace.
func (tw *turnWorld) turn(p *person) *playv1.TakeActionResponse {
	tw.t.Helper()
	return tw.turnWith(p, newKey())
}

// turnWith is turn under an idempotency key: a second call with the key is a retry, which rolls nothing.
func (tw *turnWorld) turnWith(p *person, key string) *playv1.TakeActionResponse {
	tw.t.Helper()
	if tw.rolled {
		return must(p.combat.TakeAction(tw.t.Context(), rq(tw.request(key))))
	}
	tw.rolled = true
	faces := make([]int, tw.count)
	for i := range faces {
		faces[i] = turnFace
	}
	tw.dice.ScriptFor(tw.t, 20, faces...)
	return must(p.combat.TakeAction(tw.t.Context(), rq(tw.request(key))))
}

func (tw *turnWorld) request(key string) *playv1.TakeActionRequest {
	return &playv1.TakeActionRequest{CampaignId: tw.campaign, EncounterId: tw.encounter.GetId(), CombatantId: tw.cleric, ActionKey: turnUndeadKey, IdempotencyKey: key}
}

// line is the entry of the action in the log p reads.
func (tw *turnWorld) line(p *person) *playv1.CombatLogEntry {
	tw.t.Helper()
	res := must(p.combat.ListCombatLog(tw.t.Context(), rq(&playv1.ListCombatLogRequest{CampaignId: tw.campaign, EncounterId: tw.encounter.GetId()})))
	for _, r := range res.GetRounds() {
		for _, e := range r.GetEntries() {
			if e.GetKey() == turnUndeadKey || e.GetKey() == preserveLifeKey {
				return e
			}
		}
	}
	tw.t.Fatalf("%s reads no line of Channel Divinity", p.name)
	return nil
}

// turnRevisionRE is the revision of the encounter: it counts the events of the combat, which differ between the two scenes
// by the undead that only one of them has.
var turnRevisionRE = regexp.MustCompile(`"revision":\d+`)

// same is the text of a read with the ids and the times of the fixture, and the revision of the encounter, made alike.
func same(m proto.Message) string {
	return normalized(turnRevisionRE.ReplaceAllString(jsonOf(m), `"revision":0`))
}

func labelsOf(turns []*playv1.ChannelDivinityTurn) []string {
	var out []string
	for _, t := range turns {
		out = append(out, t.GetLabel())
	}
	slices.Sort(out)
	return out
}

func labelsOfZombies(zs []turnZombie, keep func(turnZombie) bool) []string {
	var out []string
	for _, z := range zs {
		if keep(z) {
			out = append(out, z.label)
		}
	}
	slices.Sort(out)
	return out
}

func (tw *turnWorld) checkLabels(what string, got *playv1.ChannelDivinityResult, want []string) {
	tw.t.Helper()
	if got == nil {
		tw.t.Errorf("%s: no result of Channel Divinity", what)
		return
	}
	if l := labelsOf(got.GetTurns()); !slices.Equal(l, want) {
		tw.t.Errorf("%s lists %v, want %v", what, l, want)
	}
}

// scan fails when a player reads the label or the id of a zombie they may not read.
func (tw *turnWorld) scan(hidden []string, reads map[string]string) {
	tw.t.Helper()
	var needles []string
	for _, l := range hidden {
		needles = append(needles, l, tw.ids[l])
	}
	for what, text := range reads {
		if found := namesAny(text, needles); len(found) > 0 {
			tw.t.Errorf("%s names %v: %s", what, found, text)
		}
	}
}

func (tw *turnWorld) reads(p *person) map[string]string {
	ctx := tw.t.Context()
	return map[string]string{
		p.name + "'s encounter": jsonOf(must(p.combat.GetEncounter(ctx, rq(&playv1.GetEncounterRequest{CampaignId: tw.campaign})))),
		p.name + "'s log":       jsonOf(must(p.combat.ListCombatLog(ctx, rq(&playv1.ListCombatLogRequest{CampaignId: tw.campaign, EncounterId: tw.encounter.GetId()})))),
	}
}

type turnRow struct {
	name    string
	mode    turnMap
	zombies []turnZombie
}

func TestTurnUndeadListsTheUndeadEachReaderMayRead(t *testing.T) {
	a := turnZombie{label: "Zorvak4321"}
	b := turnZombie{label: "Zorvak4322"}
	hid := turnZombie{label: "Zorvak4322", hidden: true}
	far := turnZombie{label: "Zorvak4322", behind: true}
	rows := []turnRow{
		{"two seen undead", turnClearMap, []turnZombie{a, b}},
		{"one seen, one hidden by the master", turnClearMap, []turnZombie{a, hid}},
		{"one seen, one out of sight on a fog map", turnFogMap, []turnZombie{a, far}},
		{"one seen, one hidden by the master, on a fog map", turnFogMap, []turnZombie{a, hid}},
		{"two seen undead on a fog map", turnFogMap, []turnZombie{a, b}},
		{"one hidden by the master alone", turnClearMap, []turnZombie{hid}},
		{"one out of sight on a fog map alone", turnFogMap, []turnZombie{far}},
		{"no undead", turnClearMap, nil},
		{"no map, one seen, one hidden by the master", turnNoMap, []turnZombie{a, hid}},
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
			gotFull, gotOnly := full.turn(full.caio), only.turn(only.caio)
			if a, b := same(gotFull), same(gotOnly); a != b {
				t.Errorf("the cleric's answer with the others differs from the one without them:\n%s\n%s", a, b)
			}
			wantAnswer := labelsOfZombies(row.zombies, func(z turnZombie) bool { return z.readable(row.mode) })
			full.checkLabels("the cleric's answer", gotFull.GetChannelDivinity(), wantAnswer)
			for _, p := range []*person{full.caio, full.ana} {
				if a, b := same(full.line(p)), same(only.line(only.counterpart(p))); a != b {
					t.Errorf("%s's line with the others differs from the one without them:\n%s\n%s", p.name, a, b)
				}
			}
			wantLine := labelsOfZombies(row.zombies, func(z turnZombie) bool { return z.readable(row.mode) })
			full.checkLabels("the cleric's line", full.line(full.caio).GetChannelDivinity(), wantLine)
			if full.line(full.ana).GetChannelDivinity() != nil {
				t.Errorf("another player reads the numbers: %v", full.line(full.ana))
			}
			all := labelsOfZombies(row.zombies, func(turnZombie) bool { return true })
			full.checkLabels("the master's line", full.line(full.master).GetChannelDivinity(), all)

			unread := labelsOfZombies(row.zombies, func(z turnZombie) bool { return !z.readable(row.mode) })
			reads := map[string]string{"the cleric's answer": jsonOf(gotFull)}
			for _, p := range []*person{full.caio, full.ana} {
				for k, v := range full.reads(p) {
					reads[k] = v
				}
			}
			full.scan(unread, reads)
			for _, l := range unread { // the master reads the save of the undead the players may not read
				if !slices.Contains(labelsOf(full.line(full.master).GetChannelDivinity().GetTurns()), l) {
					t.Errorf("the master does not read the save of %s", l)
				}
			}
			full.checkTurnedEffects(row.zombies, row.mode)
		})
		t.Run(row.name+", the master acts for the cleric", func(t *testing.T) {
			tw := newTurnWorld(t, row.mode, row.zombies)
			got := tw.turn(tw.master)
			tw.checkLabels("the master's answer", got.GetChannelDivinity(), labelsOfZombies(row.zombies, func(turnZombie) bool { return true }))
		})
	}
}

func TestTurnUndeadLineFollowsTheMastersSwitchAfterTheAction(t *testing.T) {
	a, b := turnZombie{label: "Zorvak4321"}, turnZombie{label: "Zorvak4322"}
	hid := turnZombie{label: "Zorvak4322", hidden: true}
	t.Run("the master hides a listed undead", func(t *testing.T) {
		tw := newTurnWorld(t, turnClearMap, []turnZombie{a, b})
		tw.turn(tw.caio)
		tw.checkLabels("the line", tw.line(tw.caio).GetChannelDivinity(), []string{"Zorvak4321", "Zorvak4322"})
		tw.setHidden("Zorvak4322", true)
		tw.checkLabels("the line after the switch", tw.line(tw.caio).GetChannelDivinity(), []string{"Zorvak4321"})
		tw.scan([]string{"Zorvak4322"}, tw.reads(tw.caio))
	})
	t.Run("the master shows an undead hidden at the action", func(t *testing.T) {
		tw := newTurnWorld(t, turnClearMap, []turnZombie{a, hid})
		tw.turn(tw.caio)
		tw.setHidden("Zorvak4322", false)
		tw.checkLabels("the line after the switch", tw.line(tw.caio).GetChannelDivinity(), []string{"Zorvak4321"})
	})
	t.Run("the fog changes on a fog map", func(t *testing.T) {
		tw := newTurnWorld(t, turnFogMap, []turnZombie{a})
		tw.turn(tw.caio)
		tw.setFog(false)
		tw.checkLabels("the line after the fog is off", tw.line(tw.caio).GetChannelDivinity(), []string{"Zorvak4321"})
	})
}

func TestTurnUndeadRetryListsWhatTheLineLists(t *testing.T) {
	a := turnZombie{label: "Zorvak4321"}
	hid := turnZombie{label: "Zorvak4322", hidden: true}
	t.Run("the master shows an undead hidden at the action", func(t *testing.T) {
		tw := newTurnWorld(t, turnClearMap, []turnZombie{a, hid})
		key := newKey()
		first := tw.turnWith(tw.caio, key)
		tw.checkLabels("the answer", first.GetChannelDivinity(), []string{"Zorvak4321"})
		tw.setHidden("Zorvak4322", false)
		tw.checkLabels("the retried answer", tw.turnWith(tw.caio, key).GetChannelDivinity(), []string{"Zorvak4321"})
	})
	t.Run("on a fog map", func(t *testing.T) {
		tw := newTurnWorld(t, turnFogMap, []turnZombie{a})
		key := newKey()
		tw.checkLabels("the answer", tw.turnWith(tw.caio, key).GetChannelDivinity(), []string{"Zorvak4321"})
		tw.checkLabels("the retried answer", tw.turnWith(tw.caio, key).GetChannelDivinity(), []string{"Zorvak4321"})
	})
}

// counterpart is the person of the other fixture who is p: people are the fixture's, by name.
func (tw *turnWorld) counterpart(p *person) *person {
	for _, q := range []*person{tw.master, tw.ana, tw.caio} {
		if q.name == p.name {
			return q
		}
	}
	panic("no person " + p.name)
}

// encounterMode is the mode of a combat on the map, or with none.
func (m turnMap) encounterMode() playv1.EncounterMode {
	if m == turnNoMap {
		return playv1.EncounterMode_ENCOUNTER_MODE_THEATRE
	}
	return playv1.EncounterMode_ENCOUNTER_MODE_GRID
}

// turnedEffects are the ids of the undead that carry the effect "turned" in the encounter p reads.
func (tw *turnWorld) turnedEffects(p *person) []string {
	tw.t.Helper()
	enc := must(p.combat.GetEncounter(tw.t.Context(), rq(&playv1.GetEncounterRequest{CampaignId: tw.campaign}))).GetEncounter()
	var out []string
	for _, c := range enc.GetCombatants() {
		for _, e := range c.GetEffects() {
			if e.GetSourceKey() == "effect:turned" {
				out = append(out, e.GetTargetIds()...)
			}
		}
	}
	slices.Sort(out)
	return out
}

// checkTurnedEffects says the master reads the effect "turned" on every undead and each player only on the ones they may read.
func (tw *turnWorld) checkTurnedEffects(zs []turnZombie, mode turnMap) {
	tw.t.Helper()
	var all, readable []string
	for _, z := range zs {
		all = append(all, tw.ids[z.label])
		if z.readable(mode) {
			readable = append(readable, tw.ids[z.label])
		}
	}
	slices.Sort(all)
	slices.Sort(readable)
	if got := tw.turnedEffects(tw.master); !slices.Equal(got, all) {
		tw.t.Errorf("the master reads the effect \"turned\" on %v, want %v", got, all)
	}
	for _, p := range []*person{tw.caio, tw.ana} {
		for _, id := range tw.turnedEffects(p) {
			if !slices.Contains(readable, id) {
				tw.t.Errorf("%s reads the effect \"turned\" on %s, which they may not read", p.name, id)
			}
		}
	}
}

// forgetTheMarks strips the records of who saw each entry from the stored event, and sets its mark of the fog to underFog
// (none when nil), as an event stored before the records reads.
func (tw *turnWorld) forgetTheMarks(underFog *bool) {
	tw.t.Helper()
	strip := func(key string) string {
		return `(SELECT COALESCE(jsonb_agg(e - 'fogged' - 'seen_mask'), '[]'::jsonb) FROM jsonb_array_elements(COALESCE(payload->'channel'->'` + key + `', '[]'::jsonb)) e)`
	}
	tag, err := tw.pool.Exec(tw.t.Context(), `UPDATE session_events SET payload = jsonb_set(jsonb_set(payload #- '{channel,under_fog}', '{channel,turns}', `+strip("turns")+`), '{channel,heals}', `+strip("heals")+`)
		WHERE game_session_id = $1 AND payload->'channel' IS NOT NULL`, tw.session)
	if err != nil || tag.RowsAffected() != 1 {
		tw.t.Fatalf("strip the records of the stored event: %v, %d rows", err, tag.RowsAffected())
	}
	if underFog != nil {
		if _, err := tw.pool.Exec(tw.t.Context(), `UPDATE session_events SET payload = jsonb_set(payload, '{channel,under_fog}', to_jsonb($2::boolean)) WHERE game_session_id = $1 AND payload->'channel' IS NOT NULL`, tw.session, *underFog); err != nil {
			tw.t.Fatalf("mark the stored event: %v", err)
		}
	}
}

func TestTurnUndeadStoredBeforeItsRecordsListsWhatItsMarkSays(t *testing.T) {
	a, b := turnZombie{label: "Zorvak4321"}, turnZombie{label: "Zorvak4322"}
	hid := turnZombie{label: "Zorvak4322", hidden: true}
	both := []string{"Zorvak4321", "Zorvak4322"}
	for _, row := range []struct {
		name     string
		mode     turnMap
		zombies  []turnZombie
		underFog *bool
		want     []string
	}{
		{"on a map, no mark", turnClearMap, []turnZombie{a, b}, nil, nil},
		{"on a fog map, no mark", turnFogMap, []turnZombie{a, b}, nil, nil},
		{"on a fog map, made under the fog", turnFogMap, []turnZombie{a, b}, new(true), nil},
		{"on a map, made with no fog", turnClearMap, []turnZombie{a, b}, new(false), both},
		{"with no map", turnNoMap, []turnZombie{a, b}, nil, both},
		{"with no map, one hidden by the master", turnNoMap, []turnZombie{a, hid}, nil, []string{"Zorvak4321"}},
	} {
		t.Run(row.name, func(t *testing.T) {
			tw := newTurnWorld(t, row.mode, row.zombies)
			key := newKey()
			tw.turnWith(tw.caio, key)
			tw.forgetTheMarks(row.underFog)
			tw.checkLabels("the cleric's line", tw.line(tw.caio).GetChannelDivinity(), row.want)
			tw.checkLabels("the retried answer", tw.turnWith(tw.caio, key).GetChannelDivinity(), row.want)
			tw.checkLabels("the master's line", tw.line(tw.master).GetChannelDivinity(), labelsOfZombies(row.zombies, func(turnZombie) bool { return true }))
		})
	}
}
