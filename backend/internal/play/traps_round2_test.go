package play

import (
	"context"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	mapsv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/maps/v1"
	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
	maplink "github.com/PuraFome/meuRPG/backend/internal/maps/link"
)

// The fix round of slice 9.8 (review of the first build): what the undo skips, a save for
// each hit, an extended firing, the activity read outside a combat, the maps the players
// see, the reads inside the move's transaction, the rules of the SRD's search, and the
// damage that must never vanish.

// secondMap makes a map of the campaign that the players do not see yet (hidden, not
// current), with the cave's size.
func (r *trapRig) secondMap(t *testing.T) string {
	t.Helper()
	return r.h.newMapOf(r.campaignID, 24, 1200, 800)
}

func (r *trapRig) trapOn(t *testing.T, mapID, name string, col, row int, edit ...func(*mapsv1.TrapSpec)) *mapsv1.MapPoint {
	t.Helper()
	spec := aTrap()
	for _, e := range edit {
		e(spec)
	}
	x, y := sq(col, row)
	res, err := r.mc(r.master).CreateMapPoint(t.Context(), connect.NewRequest(&mapsv1.CreateMapPointRequest{
		CampaignId: r.campaignID, MapId: mapID, Kind: mapsv1.MapPointKind_MAP_POINT_KIND_TRAP, Name: name, XBp: x, YBp: y, Trap: spec,
	}))
	if err != nil {
		t.Fatalf("CreateMapPoint(%s) error = %v", name, err)
	}
	return res.Msg.GetPoint()
}

func (r *trapRig) placeOn(t *testing.T, mapID, characterID string, col, row int) {
	t.Helper()
	x, y := sq(col, row)
	if _, err := r.mc(r.master).PlaceMapToken(t.Context(), connect.NewRequest(&mapsv1.PlaceMapTokenRequest{
		CampaignId: r.campaignID, MapId: mapID, CharacterId: characterID, XBp: x, YBp: y,
	})); err != nil {
		t.Fatalf("PlaceMapToken() error = %v", err)
	}
}

func (r *trapRig) activity(t *testing.T, u *user) []*playv1.TrapActivity {
	t.Helper()
	res, err := u.play.ListTrapActivity(t.Context(), connect.NewRequest(&playv1.ListTrapActivityRequest{CampaignId: r.campaignID}))
	if err != nil {
		t.Fatalf("ListTrapActivity() error = %v", err)
	}
	return res.Msg.GetActivity()
}

func (r *trapRig) fog(t *testing.T, light mapsv1.LightLevel) {
	t.Helper()
	if _, err := r.mc(r.master).SetMapFog(t.Context(), connect.NewRequest(&mapsv1.SetMapFogRequest{
		CampaignId: r.campaignID, MapId: r.mapID, FogEnabled: new(true), BaseLight: &light,
	})); err != nil {
		t.Fatalf("SetMapFog() error = %v", err)
	}
}

// TestMR035_ANoticeAndOtherEventsDoNotBlockTheUndo: a move that fires a trap and notices
// another leaves the firing undoable (the knowledge stays), and so does a firing outside
// a combat made by a token on another map.
func TestMR035_ANoticeAndOtherEventsDoNotBlockTheUndo(t *testing.T) {
	t.Parallel()
	r := newTrapRig(t)
	hole := r.trap(t, "Fosso Dourado", 9, 7, pit("1d6"))
	other := r.trap(t, "Fosso Vizinho", 10, 8, func(s *mapsv1.TrapSpec) { s.NoticeDc = 10 })
	e := r.fight(t)
	// A firing outside the combat, on another map the players see.
	m2 := r.secondMap(t)
	if _, err := r.mc(r.master).SetMapRevealed(t.Context(), connect.NewRequest(&mapsv1.SetMapRevealedRequest{CampaignId: r.campaignID, MapId: m2, Revealed: true})); err != nil {
		t.Fatalf("SetMapRevealed() error = %v", err)
	}
	r.trapOn(t, m2, "Lá", 5, 5, pit("1d4"))
	r.placeOn(t, m2, r.bri.GetId(), 5, 5) // writes a trap_triggered with no combat
	if n := r.eventCount(t, "trap_triggered"); n != 1 {
		t.Fatalf("trap_triggered events after the token = %d, want 1", n)
	}

	r.mustMove(t, r.caio, "Toren", 12, 7) // stops on (9, 7): fires it, and notices the neighbor
	r.wantKnows(t, "Toren's player (the neighbor)", r.caio, other.GetId(), true)
	if n := r.eventCount(t, "trap_noticed"); n != 1 {
		t.Fatalf("trap_noticed events = %d, want 1", n)
	}
	r.undoLast(t) // the firing, past the notice and the other map's firing
	if got := r.point(t, r.master, hole.GetId()).GetTrap().GetState(); got != mapsv1.TrapState_TRAP_STATE_ARMED {
		t.Errorf("the pit after the undo = %v, want armed", got)
	}
	r.wantKnows(t, "Toren's player (the neighbor) after the undo", r.caio, other.GetId(), true) // what was noticed stays
	_ = e
}

// TestMR035_ADartIsItsOwnSave: a trap that asks the save of the creatures it hit asks it
// once for each hit.
func TestMR035_ADartIsItsOwnSave(t *testing.T) {
	t.Parallel()
	r := newTrapRig(t)
	darts := r.trap(t, "Dardos", 15, 12, func(s *mapsv1.TrapSpec) {
		s.Trigger = rulesv1.TrapTrigger_TRAP_TRIGGER_MANUAL
		s.Effect = &rulesv1.TrapEffect{
			Attack: &rulesv1.TrapAttack{Bonus: 20, Count: 3, Damage: &rulesv1.TrapDamage{Dice: "1d4", DamageTypeKey: "damage-type:piercing"}},
			Save: &rulesv1.TrapSaveEffect{
				Ability: rulesv1.Ability_ABILITY_DEXTERITY, Dc: 20, AppliesTo: rulesv1.TrapSaveApplies_TRAP_SAVE_APPLIES_HIT,
				OnFail: &rulesv1.TrapOnFail{Damage: []*rulesv1.TrapDamage{{Dice: "1d4", DamageTypeKey: "damage-type:poison"}}},
				OnPass: rulesv1.TrapPassOutcome_TRAP_PASS_OUTCOME_NONE,
			},
			Targets: rulesv1.TrapTargets_TRAP_TARGETS_MANUAL,
		}
	})
	r.fight(t)
	r.h.roller.queue(10, 1, 10, 1, 10, 1, 5, 1, 5, 1, 5, 1) // three darts that hit, then three failed saves
	res, err := r.fireByHand(t, darts, r.id(t, "Toren"))
	if err != nil {
		t.Fatalf("FireTrap() error = %v", err)
	}
	c := res.GetFiring().GetCaught()[0]
	if len(c.GetAttacks()) != 3 || len(c.GetSaves()) != 3 || len(c.GetDamages()) != 6 {
		t.Fatalf("Toren's part = %d darts, %d saves, %d damages; want 3, 3 and 6", len(c.GetAttacks()), len(c.GetSaves()), len(c.GetDamages()))
	}
	if c.GetSave().GetRoll().GetTotal() != c.GetSaves()[0].GetRoll().GetTotal() {
		t.Errorf("save = %v, want the first of saves", c.GetSave())
	}
}

// TestMR035_TheMasterAddsCreaturesToAFiring: the mover is caught at once; the master adds
// others to the same firing, which rolls for them alone and joins the log's entry, and an
// undo takes back only them.
func TestMR035_TheMasterAddsCreaturesToAFiring(t *testing.T) {
	t.Parallel()
	r := newTrapRig(t)
	net := r.trap(t, "Rede", 9, 7, func(s *mapsv1.TrapSpec) {
		s.Effect = &rulesv1.TrapEffect{
			Save: &rulesv1.TrapSaveEffect{
				Ability: rulesv1.Ability_ABILITY_DEXTERITY, Dc: 12, AppliesTo: rulesv1.TrapSaveApplies_TRAP_SAVE_APPLIES_CAUGHT,
				OnFail: &rulesv1.TrapOnFail{Damage: []*rulesv1.TrapDamage{{Dice: "1d6", DamageTypeKey: "damage-type:bludgeoning"}}},
				OnPass: rulesv1.TrapPassOutcome_TRAP_PASS_OUTCOME_NONE,
			},
			Targets: rulesv1.TrapTargets_TRAP_TARGETS_MANUAL,
		}
	})
	e := r.fight(t)
	r.h.roller.queue(5, 3)
	r.mustMove(t, r.caio, "Toren", 12, 7) // the mover only: the effect's targets are the master's
	var entry *playv1.CombatLogEntry
	find := func() {
		entry = nil
		for _, round := range r.log(t, r.master, r.get(t, r.master)).GetRounds() {
			for _, en := range round.GetEntries() {
				if en.GetKind() == playv1.CombatLogKind_COMBAT_LOG_KIND_TRAP_TRIGGERED {
					entry = en
				}
			}
		}
	}
	find()
	if entry == nil || len(entry.GetTrap().GetCaught()) != 1 || entry.GetTrap().GetId() != entry.GetId() {
		t.Fatalf("the log entry = %v, want a firing that caught the mover, with its id", entry)
	}
	r.h.roller.queue(5, 4)
	res, err := r.master.play.FireTrap(t.Context(), connect.NewRequest(&playv1.FireTrapRequest{
		CampaignId: r.campaignID, MapId: r.mapID, PointId: net.GetId(), TargetIds: []string{r.id(t, "Pensantus")}, IdempotencyKey: newKey(), ExtendFiringId: entry.GetId(),
	}))
	if err != nil {
		t.Fatalf("FireTrap(extend) error = %v", err)
	}
	if got := res.Msg.GetFiring(); got.GetId() != entry.GetId() || len(got.GetCaught()) != 1 || got.GetCaught()[0].GetTargetLabel() != "Pensantus" || got.GetCaught()[0].GetDamages()[0].GetAmount() != 4 {
		t.Errorf("the extension = %v, want Pensantus alone, 4 damage, the same firing", got)
	}
	find()
	if len(entry.GetTrap().GetCaught()) != 2 {
		t.Errorf("the log entry after the extension caught %d, want 2", len(entry.GetTrap().GetCaught()))
	}
	if d := r.trapDamages(t); len(d) != 2 {
		t.Errorf("trap damages = %d, want 2 (Toren's and Pensantus's)", len(d))
	}
	// A firing that is not this trap's, or a made-up one, is not found; a player may not.
	_, err = r.master.play.FireTrap(t.Context(), connect.NewRequest(&playv1.FireTrapRequest{
		CampaignId: r.campaignID, MapId: r.mapID, PointId: net.GetId(), TargetIds: []string{r.id(t, "Brisa")}, IdempotencyKey: newKey(), ExtendFiringId: newKey(),
	}))
	wantCode(t, "extending a made-up firing", err, connect.CodeNotFound)
	r.undoLast(t) // only the added creature
	if d := r.trapDamages(t); len(d) != 1 {
		t.Errorf("trap damages after the undo = %d, want only Toren's", len(d))
	}
	if got := r.point(t, r.master, net.GetId()).GetTrap().GetState(); got != mapsv1.TrapState_TRAP_STATE_TRIGGERED {
		t.Errorf("the net after undoing the extension = %v, want it still triggered", got)
	}
	_ = e
}

// TestMR035_TheActivityOutsideACombat: a net dropped on Pensantus outside a combat does no
// damage, only a condition; the master reads the firing, and Pensantus's player reads
// their own line, never another's. A search is read the same way, and the master's
// stream gets a hint with no content.
func TestMR035_TheActivityOutsideACombat(t *testing.T) {
	t.Parallel()
	r := newTrapRig(t)
	net := r.trap(t, "Rede que cai", 12, 7, func(s *mapsv1.TrapSpec) {
		s.Effect = &rulesv1.TrapEffect{Conditions: []*rulesv1.TrapCondition{{ConditionKey: "condition:restrained"}}}
	})
	hidden := r.trap(t, "Fosso Oculto", 4, 4, func(s *mapsv1.TrapSpec) { s.FindDc = 10 })
	masterStream := r.watch(t, r.master, r.campaignID)
	r.place(t, r.pens.GetId(), 12, 7)
	r.place(t, r.toren.GetId(), 3, 3) // next to the hidden pit, Toren's player searches
	if n := len(r.trapDamages(t)); n != 0 {
		t.Fatalf("trap damages = %d, want none: a net has no damage", n)
	}
	got := r.activity(t, r.master)
	if len(got) != 1 || got[0].GetFiring().GetName() != "Rede que cai" || len(got[0].GetFiring().GetCaught()) != 1 ||
		got[0].GetFiring().GetCaught()[0].GetTargetLabel() != "Pensantus" || len(got[0].GetFiring().GetCaught()[0].GetConditionKeys()) != 1 {
		t.Fatalf("the master's activity = %v, want the net that caught Pensantus (restrained)", got)
	}
	mine := r.activity(t, r.ana)
	if len(mine) != 1 || mine[0].GetFiring().GetCaught()[0].GetTargetLabel() != "Pensantus" {
		t.Errorf("Pensantus's player's activity = %v, want their own line", mine)
	}
	if other := r.activity(t, r.bia); len(other) != 0 {
		t.Errorf("Brisa's player's activity = %v, want nothing (Brisa was not caught)", other)
	}

	// A search outside a combat: the master reads the roll and what it found, the searcher
	// only their own, nobody else; the master's stream gets a hint with no content.
	drain := func() {
		time.Sleep(200 * time.Millisecond)
		r.drain(masterStream)
	}
	drain()
	res, err := r.search(t, r.caio, investigation, 12) // Toren: 12 + 0 against DC 10
	if err != nil || len(res.GetFoundPointIds()) != 1 || res.GetFoundPointIds()[0] != hidden.GetId() {
		t.Fatalf("Toren's search = %v, %v; want the hidden pit", res, err)
	}
	time.Sleep(300 * time.Millisecond)
	hinted := false
	for _, ev := range r.drain(masterStream) {
		if ev.GetMapChanged() != nil {
			hinted = true
		}
	}
	if !hinted {
		t.Errorf("the master's stream got no hint after a player's search")
	}
	var line *playv1.TrapSearchResult
	for _, a := range r.activity(t, r.master) {
		if a.GetSearch() != nil {
			line = a.GetSearch()
		}
	}
	if line == nil || line.GetCharacterName() != "Toren" || line.GetRoll().GetTotal() != 12 || len(line.GetFoundPointIds()) != 1 || line.GetFoundNames()[0] != "Fosso Oculto" {
		t.Errorf("the master's search line = %v, want Toren's 12 that found the Fosso Oculto", line)
	}
	for who, u := range map[string]*user{"Pensantus's player": r.ana, "Brisa's player": r.bia} {
		for _, a := range r.activity(t, u) {
			if a.GetSearch() != nil {
				t.Errorf("%s reads another character's search: %v", who, a)
			}
		}
		js, _ := protojson.Marshal(&playv1.ListTrapActivityResponse{Activity: r.activity(t, u)})
		if strings.Contains(string(js), "Fosso Oculto") || strings.Contains(string(js), hidden.GetId()) {
			t.Errorf("%s's activity names a trap they do not know: %s", who, js)
		}
	}
	if own := r.activity(t, r.caio); len(own) != 1 || own[0].GetSearch().GetRoll().GetTotal() != 12 {
		t.Errorf("Toren's player's activity = %v, want their own search", own)
	}
	_ = net
}

// TestMR035_ATokenFiresOnlyOnAMapThePlayersSee: a token put on a map the players do not
// see (hidden, not current) fires and notices nothing; once the map is revealed it does.
func TestMR035_ATokenFiresOnlyOnAMapThePlayersSee(t *testing.T) {
	t.Parallel()
	r := newTrapRig(t)
	m2 := r.secondMap(t)
	hole := r.trapOn(t, m2, "Fosso Dourado", 5, 5, pit("1d4"), func(s *mapsv1.TrapSpec) { s.NoticeDc = 5 })
	r.placeOn(t, m2, r.pens.GetId(), 5, 5)
	pointState := func() mapsv1.TrapState {
		res, err := r.mc(r.master).GetMap(t.Context(), connect.NewRequest(&mapsv1.GetMapRequest{CampaignId: r.campaignID, MapId: m2}))
		if err != nil {
			t.Fatalf("GetMap() error = %v", err)
		}
		for _, p := range res.Msg.GetPoints() {
			if p.GetId() == hole.GetId() {
				return p.GetTrap().GetState()
			}
		}
		return 0
	}
	if got := pointState(); got != mapsv1.TrapState_TRAP_STATE_ARMED {
		t.Fatalf("the trap after a token on a hidden map = %v, want armed", got)
	}
	if n := r.eventCount(t, "trap_noticed"); n != 0 {
		t.Errorf("trap_noticed events on a hidden map = %d, want 0", n)
	}
	if _, err := r.mc(r.master).SetMapRevealed(t.Context(), connect.NewRequest(&mapsv1.SetMapRevealedRequest{CampaignId: r.campaignID, MapId: m2, Revealed: true})); err != nil {
		t.Fatalf("SetMapRevealed() error = %v", err)
	}
	r.placeOn(t, m2, r.pens.GetId(), 5, 5)
	if got := pointState(); got != mapsv1.TrapState_TRAP_STATE_TRIGGERED {
		t.Errorf("the trap after a token on a revealed map = %v, want triggered", got)
	}
}

// staleBook answers every read of the map's traps with a list taken before: what the move
// read before the review's fix, when it read outside its transaction.
type staleBook struct {
	TrapBook
	stale []maplink.Trap
}

func (b *staleBook) Traps(_ context.Context, _ pgx.Tx, _, _ string) ([]maplink.Trap, error) {
	return b.stale, nil
}

// TestMR035_ATrapDisarmedOrDeletedMeanwhileIsJustNotThere: a move whose read of the traps
// is stale (the trap was disarmed or deleted after it) never fails: it stops where the
// stale read said, nothing fires and nothing errors ("point not found" failed the move
// before). The move itself reads the traps inside its transaction, so a disarm or a delete
// that commits meanwhile makes it read again (serializable isolation; a disarm also takes
// the session's row, so it waits for the move).
func TestMR035_ATrapDisarmedOrDeletedMeanwhileIsJustNotThere(t *testing.T) {
	t.Parallel()
	for name, change := range map[string]func(r *trapRig, p *mapsv1.MapPoint) error{
		"disarmed": func(r *trapRig, p *mapsv1.MapPoint) error {
			_, err := r.mc(r.master).DisarmTrap(context.Background(), connect.NewRequest(&mapsv1.DisarmTrapRequest{CampaignId: r.campaignID, MapId: r.mapID, PointId: p.GetId()}))
			return err
		},
		"deleted": func(r *trapRig, p *mapsv1.MapPoint) error {
			_, err := r.mc(r.master).DeleteMapPoint(context.Background(), connect.NewRequest(&mapsv1.DeleteMapPointRequest{CampaignId: r.campaignID, MapId: r.mapID, PointId: p.GetId()}))
			return err
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			r := newTrapRig(t)
			hole := r.trap(t, "Fosso Dourado", 9, 7, pit("1d6"))
			r.fight(t)
			stale, err := r.msvc.Traps(t.Context(), nil, r.campaignID, r.mapID)
			if err != nil || len(stale) != 1 {
				t.Fatalf("Traps() = %v, %v; want the trap", stale, err)
			}
			if err := change(r, hole); err != nil {
				t.Fatalf("the change error = %v", err)
			}
			r.h.svc.SetTraps(&staleBook{TrapBook: r.msvc, stale: stale})
			res, err := r.moveResult(t, r.caio, "Toren", 12, 7)
			if err != nil {
				t.Fatalf("MoveCombatant() error = %v, want the move to stand", err)
			}
			if got := byLabel(t, res.GetEncounter(), "Toren"); got.GetCol() != 9 {
				t.Errorf("Toren after the move = (%d, %d), want where the stale read stopped him (9, 7)", got.GetCol(), got.GetRow())
			}
			if n := r.eventCount(t, "trap_triggered"); n != 0 {
				t.Errorf("trap_triggered events = %d, want 0", n)
			}
		})
	}
}

// TestMR035_ARetriedTrapMoveKeepsTheOffers: a move that provokes and fires a trap, sent
// again with the same key, answers the same (provoked).
func TestMR035_ARetriedTrapMoveKeepsTheOffers(t *testing.T) {
	t.Parallel()
	r := newTrapRig(t)
	r.trap(t, "Fosso Dourado", 9, 7, pit("1d6"))
	r.fight(t)
	r.mustMove(t, r.master, "Goblin 1", 6, 6) // next to Toren
	req := &playv1.MoveCombatantRequest{CampaignId: r.campaignID, EncounterId: r.get(t, r.master).GetId(), CombatantId: r.id(t, "Toren"), IdempotencyKey: newKey(), Col: 12, Row: 7}
	var first, again *playv1.MoveCombatantResponse
	for i, dst := range []**playv1.MoveCombatantResponse{&first, &again} {
		res, err := r.caio.combat.MoveCombatant(t.Context(), connect.NewRequest(req))
		if err != nil {
			t.Fatalf("MoveCombatant() #%d error = %v", i+1, err)
		}
		*dst = res.Msg
	}
	if !first.GetProvoked() || !again.GetProvoked() || !again.GetStoppedEarly() {
		t.Errorf("provoked = %v then %v, stopped early on the retry = %v; want provoked both times", first.GetProvoked(), again.GetProvoked(), again.GetStoppedEarly())
	}
}

// TestMR035_APerceptionSearchInDimLightHasDisadvantage: the SRD's lightly obscured area
// gives disadvantage on a Perception check that relies on sight. In the app the server rolls
// two d20 and the lower counts there; with a real die the second one is required.
// Investigation is not affected.
func TestMR035_APerceptionSearchInDimLightHasDisadvantage(t *testing.T) {
	t.Parallel()
	r := newTrapRig(t)
	pit := r.trap(t, "Fosso Dourado", 9, 7, func(s *mapsv1.TrapSpec) { s.NoticeDc, s.FindDc = 12, 12 })
	r.fog(t, mapsv1.LightLevel_LIGHT_LEVEL_DIM)
	r.place(t, r.toren.GetId(), 8, 7) // a human: dim light is lightly obscured to him (Pensantus's darkvision makes it bright)
	_, err := r.search(t, r.caio, perception, 15)
	wantEncounterBlocked(t, err, playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_SEARCH_NEEDS_TWO_DICE)
	two := func(a, b int32) (*playv1.SearchForTrapsResponse, error) {
		res, err := r.caio.play.SearchForTraps(t.Context(), connect.NewRequest(&playv1.SearchForTrapsRequest{
			CampaignId: r.campaignID, IdempotencyKey: newKey(), Skill: perception, Roll: &playv1.SearchForTrapsRequest_D20Face{D20Face: a}, D20Face_2: new(b),
		}))
		if err != nil {
			return nil, err
		}
		return res.Msg, nil
	}
	if res, err := two(15, 8); err != nil || len(res.GetFoundPointIds()) != 0 || res.GetSecondRoll().GetTotal() != 8 {
		t.Fatalf("15 and 8 = %v, %v; want the lower (8) to count: nothing found", res, err)
	}
	if res, err := two(8, 15); err != nil || len(res.GetFoundPointIds()) != 0 {
		t.Fatalf("8 and 15 = %v, %v; want the lower (8) to count: nothing found", res, err)
	}
	if res, err := two(14, 13); err != nil || len(res.GetFoundPointIds()) != 1 || res.GetFoundPointIds()[0] != pit.GetId() {
		t.Fatalf("14 and 13 = %v, %v; want the pit found", res, err)
	}
	// In the app: two d20 are rolled (the first 15, the second 3): the lower counts.
	r.place(t, r.toren.GetId(), 8, 8)
	r.h.roller.queue(15, 3)
	res, err := r.caio.play.SearchForTraps(t.Context(), connect.NewRequest(&playv1.SearchForTrapsRequest{
		CampaignId: r.campaignID, IdempotencyKey: newKey(), Skill: perception, Roll: &playv1.SearchForTrapsRequest_RollInApp{RollInApp: true},
	}))
	if err != nil || res.Msg.GetRoll().GetTotal() != 15 || res.Msg.GetSecondRoll().GetTotal() != 3 {
		t.Fatalf("the app's roll = %v, %v; want 15 and 3", res, err)
	}
}

// TestMR035_BlindsightIsNeverLightlyObscured: a creature with blindsight (a bat's 60 ft)
// notices a trap in the dark without the -5 that darkvision or dim light would cost it.
func TestMR035_BlindsightIsNeverLightlyObscured(t *testing.T) {
	t.Parallel()
	r := newTrapRig(t)
	hole := r.trap(t, "Fosso Dourado", 9, 8, func(s *mapsv1.TrapSpec) { s.NoticeDc = 9 }) // the bat's passive Perception is 11
	r.fog(t, mapsv1.LightLevel_LIGHT_LEVEL_DARK)
	r.give(t, r.toren, "monster:bat", "Morcego")
	e := r.start(t, plan{
		npcs:     []*playv1.Participant{{CharacterId: r.goblins.GetId(), Count: 1}},
		npcRolls: []int{2},
		players:  map[string]int32{"Toren": 18, "Pensantus": 10, "Brisa": 1, "Morcego": 15},
		reveal:   []string{"Goblin"},
		at:       map[string][2]int32{"Toren": {6, 7}, "Pensantus": {5, 8}, "Brisa": {4, 7}, "Morcego": {7, 7}, "Goblin": {20, 7}},
	})
	r.mustEndTurn(t, r.caio, e) // the bat's turn
	if rev := r.revealedTo(t, hole.GetId()); len(rev) != 0 {
		t.Fatalf("revealed to %v before the move, want nobody", rev)
	}
	r.mustMove(t, r.caio, "Morcego", 8, 7) // 7,1 ft from the area, which it feels with blindsight: a passive 11, no penalty
	// The map is dark and Toren sees nothing, so the master's card is where to read it.
	if rev := r.revealedTo(t, hole.GetId()); len(rev) != 1 || rev[0].GetCharacterId() != r.toren.GetId() || rev[0].GetHow() != mapsv1.TrapRevealHow_TRAP_REVEAL_HOW_NOTICED {
		t.Errorf("revealed to %v after the bat's move, want Toren, noticed", rev)
	}
}

// TestMR035_AnEndedCombatKeepsTheTrapDamageForTheMaster: the damage a trap did to a player's
// character always waits for the master (question 73): ending the combat turns it into
// one outside a combat, and ending the session does not lose it, until he applies it.
func TestMR035_AnEndedCombatKeepsTheTrapDamageForTheMaster(t *testing.T) {
	t.Parallel()
	r := newTrapRig(t)
	r.trap(t, "Fosso Dourado", 9, 7, pit("1d6"))
	e := r.fight(t)
	r.h.roller.queue(4)
	r.mustMove(t, r.caio, "Toren", 12, 7)
	if d := r.trapDamages(t); len(d) != 1 || d[0].GetEncounterId() != e.GetId() {
		t.Fatalf("trap damages in the combat = %v, want one", d)
	}
	if _, err := r.master.combat.EndEncounter(t.Context(), connect.NewRequest(&playv1.EndEncounterRequest{CampaignId: r.campaignID, EncounterId: e.GetId(), IdempotencyKey: newKey()})); err != nil {
		t.Fatalf("EndEncounter() error = %v", err)
	}
	d := r.trapDamages(t)
	if len(d) != 1 || d[0].GetEncounterId() != "" || d[0].GetAmount() != 4 || d[0].GetCharacterName() != "Toren" || d[0].GetSessionNumber() != 1 {
		t.Fatalf("trap damages after the combat ended = %v, want Toren's 4, outside a combat, of session 1", d)
	}
	// The session ends: the damage is still there, with its session's date.
	before := r.vitals(t, r.toren).GetHitPointsCurrent()
	r.master.end(t, r.master.liveSession(t, r.campaignID).GetGameSession())
	d = r.trapDamages(t)
	if len(d) != 1 || d[0].GetSessionStartedAt() == nil {
		t.Fatalf("trap damages after the session ended = %v, want the same one, with its session's date", d)
	}
	if _, err := r.master.play.ApplyTrapDamage(t.Context(), connect.NewRequest(&playv1.ApplyTrapDamageRequest{
		CampaignId: r.campaignID, TrapDamageId: d[0].GetId(), IdempotencyKey: newKey(), Amount: proto.Int32(3),
	})); err != nil {
		t.Fatalf("ApplyTrapDamage() with no session error = %v", err)
	}
	if got := r.hpOf(t, r.toren.GetId()); got != before-3 {
		t.Errorf("Toren's hit points = %d, want %d", got, before-3)
	}
	if d := r.trapDamages(t); len(d) != 0 {
		t.Errorf("trap damages after the apply = %v, want none", d)
	}
}

// TestMR035_AKeyReusedOnAnotherTrapDamageIsRefused: a retry of an apply repeats it; the
// same key on another damage is invalid_argument.
func TestMR035_AKeyReusedOnAnotherTrapDamageIsRefused(t *testing.T) {
	t.Parallel()
	r := newTrapRig(t)
	one := r.trap(t, "Um", 12, 7, pit("1d6"), func(s *mapsv1.TrapSpec) { s.Trigger = rulesv1.TrapTrigger_TRAP_TRIGGER_MANUAL })
	two := r.trap(t, "Dois", 13, 7, pit("1d6"), func(s *mapsv1.TrapSpec) { s.Trigger = rulesv1.TrapTrigger_TRAP_TRIGGER_MANUAL })
	r.place(t, r.pens.GetId(), 12, 7)
	if _, err := r.fireByHand(t, one); err != nil {
		t.Fatalf("FireTrap() error = %v", err)
	}
	r.place(t, r.pens.GetId(), 13, 7)
	if _, err := r.fireByHand(t, two); err != nil {
		t.Fatalf("FireTrap() error = %v", err)
	}
	d := r.trapDamages(t)
	if len(d) != 2 {
		t.Fatalf("trap damages = %v, want 2", d)
	}
	key := newKey()
	apply := func(id string) error {
		_, err := r.master.play.ApplyTrapDamage(t.Context(), connect.NewRequest(&playv1.ApplyTrapDamageRequest{CampaignId: r.campaignID, TrapDamageId: id, IdempotencyKey: key}))
		return err
	}
	if err := apply(d[0].GetId()); err != nil {
		t.Fatalf("ApplyTrapDamage() error = %v", err)
	}
	if err := apply(d[0].GetId()); err != nil {
		t.Errorf("the retry error = %v, want the first answer", err)
	}
	wantCode(t, "the key on another damage", apply(d[1].GetId()), connect.CodeInvalidArgument)
}

// TestMR035_APassiveNoticeTellsNobodyElseAndTheMasterOnlyWithNoContent: another player's
// stream gets no map change when a character notices a trap; the master's gets a hint
// with no content; the event's actor is the noticer.
func TestMR035_APassiveNoticeTellsNobodyElseAndTheMasterOnlyWithNoContent(t *testing.T) {
	t.Parallel()
	r := newTrapRig(t)
	pit := r.trap(t, "Fosso Dourado", 9, 7, func(s *mapsv1.TrapSpec) { s.NoticeDc = 10 })
	r.fight(t)
	anaStream := r.watch(t, r.ana, r.campaignID)
	masterStream := r.watch(t, r.master, r.campaignID)
	r.mustMove(t, r.caio, "Toren", 7, 7)
	r.wantKnows(t, "Toren's player", r.caio, pit.GetId(), true)
	time.Sleep(400 * time.Millisecond)
	for _, ev := range r.drain(anaStream) {
		if ev.GetMapChanged() != nil {
			t.Errorf("Pensantus's stream got %v after Toren noticed a trap; want no map change", ev)
		}
		if js, _ := protojson.Marshal(ev); strings.Contains(string(js), pit.GetId()) {
			t.Errorf("Pensantus's stream names the trap: %s", js)
		}
	}
	hinted := false
	for _, ev := range r.drain(masterStream) {
		hinted = hinted || ev.GetMapChanged() != nil
	}
	if !hinted {
		t.Errorf("the master's stream got no hint")
	}
	var actor string
	if err := r.h.pool.QueryRow(t.Context(), `SELECT actor_user_id FROM session_events WHERE kind = 'trap_noticed'`).Scan(&actor); err != nil || actor != r.caio.id {
		t.Errorf("trap_noticed actor = %q, %v; want Toren's player %q", actor, err, r.caio.id)
	}
}

// hpOf reads a character's stored hit points, with or without a session.
func (r *trapRig) hpOf(t *testing.T, characterID string) int32 {
	t.Helper()
	var hp int32
	if err := r.h.pool.QueryRow(t.Context(), `SELECT hit_points_current FROM character_vitals WHERE character_id = $1`, characterID).Scan(&hp); err != nil {
		t.Fatalf("read the hit points: %v", err)
	}
	return hp
}

// A trap's line in the combat log carries the character id of a player's character, never
// of an NPC: an NPC's character is the master's secret, and GetEncounter withholds it from
// the same player.
func TestTrapFiringLogWithholdsAnNPCCharacterID(t *testing.T) {
	t.Parallel()
	r := newTrapRig(t)
	statue := r.trap(t, "Estátua de Fogo", 15, 12, func(s *mapsv1.TrapSpec) {
		s.Trigger = rulesv1.TrapTrigger_TRAP_TRIGGER_MANUAL
		s.Effect = &rulesv1.TrapEffect{
			Damage:  []*rulesv1.TrapDamage{{Dice: "1d4", DamageTypeKey: "damage-type:fire"}},
			Targets: rulesv1.TrapTargets_TRAP_TARGETS_MANUAL,
		}
	})
	r.fight(t)
	r.h.roller.queue(2)
	if _, err := r.fireByHand(t, statue, r.id(t, "Goblin 1")); err != nil {
		t.Fatalf("FireTrap: %v", err)
	}
	enc := r.get(t, r.ana)
	if id := byLabel(t, enc, "Goblin 1").GetCharacterId(); id != "" {
		t.Fatalf("precondition: the player's GetEncounter already shows the goblin's character_id %q", id)
	}
	found := false
	for _, rd := range r.log(t, r.ana, enc).GetRounds() {
		for _, en := range rd.GetEntries() {
			for _, c := range en.GetTrap().GetCaught() {
				found = true
				if c.GetCharacterId() != "" {
					t.Errorf("player's combat log shows NPC %q character_id %q (GetEncounter withholds it)", c.GetTargetLabel(), c.GetCharacterId())
				}
			}
		}
	}
	if !found {
		t.Fatal("no trap entry in the player's log")
	}
}

// A trap firing that caught an NPC the master had hidden stays out of the players' log
// after the master reveals it: a revealed combatant does not bring its old entries.
func TestTrapFiringOfAHiddenNPCDoesNotAppearOnReveal(t *testing.T) {
	t.Parallel()
	r := newTrapRig(t)
	statue := r.trap(t, "Estátua de Fogo", 15, 12, func(s *mapsv1.TrapSpec) {
		s.Trigger = rulesv1.TrapTrigger_TRAP_TRIGGER_MANUAL
		s.Effect = &rulesv1.TrapEffect{
			Save: &rulesv1.TrapSaveEffect{
				Ability: rulesv1.Ability_ABILITY_DEXTERITY, Dc: 12, AppliesTo: rulesv1.TrapSaveApplies_TRAP_SAVE_APPLIES_CAUGHT,
				OnFail: &rulesv1.TrapOnFail{
					Damage:    []*rulesv1.TrapDamage{{Dice: "2d6", DamageTypeKey: "damage-type:fire"}},
					Condition: &rulesv1.TrapCondition{ConditionKey: "condition:poisoned"},
				},
				OnPass: rulesv1.TrapPassOutcome_TRAP_PASS_OUTCOME_HALF,
			},
			Targets: rulesv1.TrapTargets_TRAP_TARGETS_MANUAL,
		}
	})
	e := r.fight(t)
	r.hide(t, "Goblin 1")
	r.h.roller.queue(5, 3, 4)
	if _, err := r.fireByHand(t, statue, r.id(t, "Goblin 1")); err != nil {
		t.Fatalf("FireTrap() error = %v", err)
	}

	leaks := func() (n int) {
		for _, round := range r.log(t, r.caio, r.get(t, r.caio)).GetRounds() {
			for _, en := range round.GetEntries() {
				for _, c := range en.GetTrap().GetCaught() {
					if c.GetTargetLabel() == "Goblin 1" {
						n++
						t.Logf("player sees: %v", c)
					}
				}
			}
		}
		return n
	}
	if n := leaks(); n != 0 {
		t.Fatalf("while hidden, the player's log already shows the goblin caught (%d), want nothing", n)
	}
	if _, err := r.master.combat.SetCombatantHidden(t.Context(), connect.NewRequest(&playv1.SetCombatantHiddenRequest{
		CampaignId: r.campaignID, EncounterId: e.GetId(), CombatantId: r.id(t, "Goblin 1"), IdempotencyKey: newKey(), Hidden: false,
	})); err != nil {
		t.Fatalf("SetCombatantHidden(reveal) error = %v", err)
	}
	if n := leaks(); n != 0 {
		t.Errorf("after the reveal the old firing on the formerly hidden goblin reached the player's log (%d caught); a revealed combatant must not bring its old entries", n)
	}
}

// A key used by one player's search is refused to another player: the answer holds the
// first one's roll and the ids of the traps they found (RN-10).
func TestSearchForTrapsKeyIsPerCaller(t *testing.T) {
	t.Parallel()
	r := newTrapRig(t)
	needle := r.trap(t, "Agulha Rubra", 8, 8, func(s *mapsv1.TrapSpec) { s.NoticeDc, s.FindDc = 0, 13 })
	r.place(t, r.pens.GetId(), 8, 7)  // Ana's character, next to the trap
	r.place(t, r.toren.GetId(), 3, 3) // Caio's character, far away

	key := newKey()
	req := func() *playv1.SearchForTrapsRequest {
		return &playv1.SearchForTrapsRequest{CampaignId: r.campaignID, IdempotencyKey: key,
			Skill: investigation, Roll: &playv1.SearchForTrapsRequest_D20Face{D20Face: 20}}
	}
	a, err := r.ana.play.SearchForTraps(t.Context(), connect.NewRequest(req()))
	if err != nil || len(a.Msg.GetFoundPointIds()) != 1 || a.Msg.GetFoundPointIds()[0] != needle.GetId() {
		t.Fatalf("A's search = %v, %v; want the needle", a, err)
	}
	b, err := r.caio.play.SearchForTraps(t.Context(), connect.NewRequest(req()))
	if err == nil {
		t.Errorf("B reusing A's key got no error; response = %v (found %v)", b.Msg, b.Msg.GetFoundPointIds())
		for _, id := range b.Msg.GetFoundPointIds() {
			if id == needle.GetId() {
				t.Errorf("B received trap id %s that B's character never found", id)
			}
		}
	} else {
		wantCode(t, "SearchForTraps(B, A's key)", err, connect.CodeInvalidArgument)
	}
	r.wantKnows(t, "B's player", r.caio, needle.GetId(), false)
	if n := r.eventCount(t, "trap_searched"); n != 1 {
		t.Errorf("trap_searched events = %d, want 1", n)
	}
}
