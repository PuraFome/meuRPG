package play

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"os"
	"path/filepath"
	"testing"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/encoding/protojson"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
)

// Characterization of ListTrapActivity (MR-035, RN-10, RN-20): what each kind of viewer reads of
// each kind of event the open session holds outside a combat. The events are written straight
// into the session, as the services that own them write them, so that every shape (and every
// malformed payload the reader skips) is there at once; the answer of each viewer is a golden
// file (testdata/trap_activity_*.golden.json, ids replaced by names). Run with
// -update-trap-activity to write them again after a change of behavior that was meant.

var updateTrapActivity = flag.Bool("update-trap-activity", false, "write the golden files of ListTrapActivity again")

// activityScript is the events of the session, oldest first, and the ids the golden files name.
type activityScript struct {
	r       *trapRig
	session string
	at      time.Time
	labels  map[string]string // id -> name in the golden files
}

// add writes one event of the session and returns its id.
func (a *activityScript) add(t *testing.T, kind, characterID, label string, payload any) string {
	t.Helper()
	body, ok := payload.([]byte)
	if !ok {
		var err error
		if body, err = json.Marshal(payload); err != nil {
			t.Fatal(err)
		}
	}
	q := a.r.h.svc.queries
	seq, err := q.NextSessionEventSeq(t.Context(), a.session)
	if err != nil {
		t.Fatalf("NextSessionEventSeq() error = %v", err)
	}
	var character *string
	if characterID != "" {
		character = &characterID
	}
	a.at = a.at.Add(time.Second)
	row, err := q.InsertSessionEvent(t.Context(), playdb.InsertSessionEventParams{
		GameSessionID: a.session, Seq: seq, Kind: kind, CharacterID: character, Payload: body, CreatedAt: a.at,
	})
	if err != nil {
		t.Fatalf("InsertSessionEvent(%s) error = %v", label, err)
	}
	a.labels[row.ID] = label
	return row.ID
}

// damage writes a trap damage of the character, as the firing outside a combat leaves it, in the
// status the master has taken it to.
func (a *activityScript) damage(t *testing.T, point, fire, character, status string, applied *int32, label string) string {
	t.Helper()
	row, err := a.r.h.svc.queries.InsertTrapDamage(t.Context(), playdb.InsertTrapDamageParams{
		GameSessionID: a.session, TrapPointID: point, FireID: fire, CharacterID: character,
		DiceCount: 1, DiceSides: 6, DamageType: "damage-type:piercing", Faces: []int32{4}, RollTotal: 4, Amount: 4, CreatedAt: a.at,
	})
	if err != nil {
		t.Fatalf("InsertTrapDamage(%s) error = %v", label, err)
	}
	if status != "rolled" {
		if _, err := a.r.h.pool.Exec(t.Context(), `UPDATE trap_damages SET status = $2, applied_amount = $3, resolved_at = now() WHERE id = $1`, row.ID, status, applied); err != nil {
			t.Fatalf("settle the trap damage %s: %v", label, err)
		}
	}
	a.labels[row.ID] = label
	return row.ID
}

// golden compares the answer to the file, ids replaced by the names of the script.
func (a *activityScript) golden(t *testing.T, viewer string, u *user) {
	t.Helper()
	res, err := u.play.ListTrapActivity(t.Context(), connect.NewRequest(&playv1.ListTrapActivityRequest{CampaignId: a.r.campaignID}))
	if err != nil {
		t.Fatalf("ListTrapActivity(%s) error = %v", viewer, err)
	}
	raw, err := protojson.Marshal(res.Msg)
	if err != nil {
		t.Fatal(err)
	}
	for id, name := range a.labels {
		raw = bytes.ReplaceAll(raw, []byte(id), []byte(name))
	}
	var tree any
	if err := json.Unmarshal(raw, &tree); err != nil {
		t.Fatal(err)
	}
	got, err := json.MarshalIndent(tree, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	got = append(got, '\n')
	path := filepath.Join("testdata", "trap_activity_"+viewer+".golden.json")
	if *updateTrapActivity {
		if err := os.MkdirAll("testdata", 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, got, 0o600); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read the golden file: %v (write it with -update-trap-activity)", err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("ListTrapActivity(%s) differs from %s:\n--- got\n%s\n--- want\n%s", viewer, path, got, want)
	}
}

// shapeOf is what a refusal says: the code, the message and the reason of its detail.
func shapeOf(err error) string {
	if err == nil {
		return "<nil>"
	}
	out := connect.CodeOf(err).String() + "|" + err.Error()
	if ce, ok := errors.AsType[*connect.Error](err); ok {
		for _, d := range ce.Details() {
			if v, err := d.Value(); err == nil {
				if b, ok := v.(*playv1.GameSessionBlocked); ok {
					out += "|" + b.GetReason().String()
				}
			}
		}
	}
	return out
}

// TestListTrapActivity_WhatEachViewerReads: the master reads every line; a player only the lines of
// their own characters (and creatures), with their own dice, never a DC; a player with no
// character reads nothing. Malformed events, extensions of a firing that is not there and
// notices that name nobody are skipped.
func TestListTrapActivity_WhatEachViewerReads(t *testing.T) {
	t.Parallel()
	r := newTrapRig(t)
	pitA := r.trap(t, "Fosso A", 9, 7).GetId()
	pitB := r.trap(t, "Fosso B", 10, 8).GetId()
	wolf := r.give(t, r.pens, "monster:wolf", "Nanquim").GetId()
	visitor := r.h.newUser("Visitante")
	r.h.join(r.master, r.campaignID, visitor)
	toren, pens, bri := r.toren.GetId(), r.pens.GetId(), r.bri.GetId()
	lost := newKey() // a trap that is gone: no name

	a := &activityScript{
		r: r, session: r.master.liveSession(t, r.campaignID).GetGameSession().GetId(),
		at: time.Date(2026, 3, 1, 20, 0, 0, 0, time.UTC),
		labels: map[string]string{
			pitA: "pitA", pitB: "pitB", lost: "lost", wolf: "wolf", toren: "toren", pens: "pens", bri: "bri",
		},
	}

	// The searches: the skill, the second d20 of an advantage, the Lucky reroll of the first or the second
	// d20 (and of a second d20 that is not there), Reliable Talent, and a search with no character.
	a.add(t, eventTrapSearched, toren, "search-toren-perception", actionEvent{Key: searchPerception, D20: 15, Modifier: 3, Found: []string{pitA}})
	a.add(t, eventTrapSearched, pens, "search-pens-investigation", actionEvent{
		Key: searchInvestigation, D20: 17, D20B: 4, Modifier: 2, Lucky: &luckyReroll{Index: 0, From: 1, To: 4},
	})
	a.add(t, eventTrapSearched, bri, "search-bri-other", actionEvent{
		Key: "skill:stealth", D20: 9, D20B: 12, Modifier: 1, Physical: true, ReliableTalent: true,
		Lucky: &luckyReroll{Index: 1, From: 1, To: 12}, Found: []string{pitB, lost},
	})
	a.add(t, eventTrapSearched, toren, "search-toren-lucky-second-missing", actionEvent{
		Key: searchPerception, D20: 20, Lucky: &luckyReroll{Index: 1, From: 1, To: 20},
	})
	a.add(t, eventTrapSearched, "", "search-nobody", actionEvent{Key: searchPerception, D20: 11, Found: []string{pitA}})

	// The notices, and the ones the reader skips.
	a.add(t, eventTrapNoticed, "", "notice-toren-pens", map[string]any{"point_id": pitA, "character_ids": []string{toren, pens}})
	a.add(t, eventTrapNoticed, "", "notice-bri-toren", map[string]any{"point_id": pitB, "character_ids": []string{bri, toren}})
	a.add(t, eventTrapNoticed, "", "notice-lost-trap", map[string]any{"point_id": lost, "character_ids": []string{bri}})
	a.add(t, eventTrapNoticed, "", "skipped-notice-of-nobody", map[string]any{"point_id": pitA, "character_ids": []string{}})
	a.add(t, eventTrapNoticed, "", "skipped-notice-badly-typed", []byte(`{"point_id": 5, "character_ids": ["x"]}`))
	a.add(t, eventTrapNoticed, "", "skipped-notice-badly-typed-event", []byte(`{"round": "x"}`))
	a.add(t, eventTrapTriggered, "", "skipped-firing-badly-typed-event", []byte(`{"round": "x"}`))
	a.add(t, eventTrapTriggered, "", "skipped-firing-without-a-trap", actionEvent{Round: 1})

	// A firing outside a combat: the character, the character with every status of damage, the
	// creature of Pensantus, and a hidden Brisa. Finding fix/trap-hidden-outside-combat: what the master hid
	// never reaches a player, not even the player of the hidden character: Brisa reads the extension's line
	// (the positive control, not hidden) and never the hidden one, the master reads both.
	fire := newKey()
	dRolled := a.damage(t, pitA, fire, toren, "rolled", nil, "damage-rolled")
	three := int32(3)
	dApplied := a.damage(t, pitA, fire, toren, "applied", &three, "damage-applied")
	dDiscarded := a.damage(t, pitA, fire, toren, "discarded", nil, "damage-discarded")
	hit := damageHit{Target: toren, Amount: 4}
	firing := a.add(t, eventTrapTriggered, "", "firing-one", actionEvent{Trap: &trapFireEvent{
		PointID: pitA, MapID: r.mapID, PrevState: "armed",
		Caught: []trapCaughtEvent{
			{
				Target: toren, Character: toren, Player: true, Conditions: []string{"condition:restrained"},
				Attacks: []trapAttackEvent{{D20: 14, Modifier: 5, Total: 19, Outcome: outcomeHit, TargetAC: 15}},
				Saves:   []saveRoll{{D20: 8, Bonus: 2, Total: 10, DC: 13}},
				Damages: []trapDamageEvent{
					{damageHit: damageHit{Pending: dRolled, Target: toren, Amount: 4}, Type: "damage-type:piercing", DiceCount: 1, DiceSides: 6, Faces: []int32{4}, RollTotal: 4},
					{damageHit: damageHit{Pending: dApplied, Target: toren, Amount: 5}, Type: "damage-type:piercing", DiceCount: 1, DiceSides: 6, Faces: []int32{5}, RollTotal: 5},
					{damageHit: damageHit{Pending: dDiscarded, Target: toren, Amount: 2, Half: true}, Type: "damage-type:fire", DiceCount: 1, DiceSides: 4, Faces: []int32{2}, RollTotal: 2, Critical: true},
					{damageHit: hit, Type: "damage-type:acid", DiceCount: 1, DiceSides: 4, Faces: []int32{1}, RollTotal: 1},
				},
			},
			{
				Target: wolf, Character: pens, Player: true,
				Damages: []trapDamageEvent{{
					damageHit: damageHit{Target: wolf, Amount: 4, Applied: true, After: &hpState{Defeated: true}},
					Type:      "damage-type:piercing", DiceCount: 1, DiceSides: 6, Faces: []int32{4}, RollTotal: 4,
				}},
			},
			{Target: bri, Character: bri, Player: true, Hidden: true},
		},
	}})
	// Extensions: the creatures the master added join the firing; one whose firing is not there,
	// and one whose host is a search line, are dropped.
	a.add(t, eventTrapTriggered, "", "extension-one", actionEvent{Trap: &trapFireEvent{
		PointID: pitA, MapID: r.mapID, PrevState: "triggered", ExtendsID: firing,
		Caught: []trapCaughtEvent{{Target: bri, Character: bri, Player: true, Conditions: []string{"condition:prone"}}},
	}})
	a.add(t, eventTrapTriggered, "", "extension-of-nothing", actionEvent{Trap: &trapFireEvent{
		PointID: pitA, MapID: r.mapID, ExtendsID: newKey(), Caught: []trapCaughtEvent{{Target: toren, Character: toren, Player: true}},
	}})
	var searchID string
	for id, name := range a.labels {
		if name == "search-toren-perception" {
			searchID = id
		}
	}
	a.add(t, eventTrapTriggered, "", "extension-of-a-search", actionEvent{Trap: &trapFireEvent{
		PointID: pitA, MapID: r.mapID, ExtendsID: searchID, Caught: []trapCaughtEvent{{Target: toren, Character: toren, Player: true}},
	}})
	// A firing that caught only Toren, one that caught nobody, and one that caught an NPC's character.
	a.add(t, eventTrapTriggered, "", "firing-toren-only", actionEvent{Trap: &trapFireEvent{
		PointID: pitB, MapID: r.mapID, Manual: true, Caught: []trapCaughtEvent{{Target: toren, Character: toren, Player: true, Conditions: []string{"condition:poisoned"}}},
	}})
	a.add(t, eventTrapTriggered, "", "firing-nobody", actionEvent{Trap: &trapFireEvent{PointID: lost, MapID: r.mapID}})
	a.add(t, eventTrapTriggered, "", "firing-an-npc", actionEvent{Trap: &trapFireEvent{
		PointID: pitB, MapID: r.mapID, Caught: []trapCaughtEvent{{Target: r.goblin.GetId(), Character: r.goblin.GetId()}},
	}})
	a.labels[r.goblin.GetId()] = "goblin"
	a.labels[r.capitao.GetId()] = "capitao"

	for viewer, u := range map[string]*user{"master": r.master, "toren": r.caio, "pens": r.ana, "bri": r.bia, "visitor": visitor} {
		a.golden(t, viewer, u)
	}

	// The refusals, with their exact words.
	call := func(u *user, campaign string) error {
		_, err := u.play.ListTrapActivity(t.Context(), connect.NewRequest(&playv1.ListTrapActivityRequest{CampaignId: campaign}))
		return err
	}
	outsider := r.h.newUser("Intruso")
	pending := r.h.newUser("Pendente")
	r.h.joinPending(r.master, r.campaignID, pending)
	const notFound = "not_found|not_found: campaign not found"
	const noSession = "failed_precondition|failed_precondition: the campaign has no open game session|GAME_SESSION_BLOCKED_REASON_NO_OPEN_SESSION"
	for name, tc := range map[string]struct {
		u        *user
		campaign string
		want     string
	}{
		"outsider": {outsider, r.campaignID, notFound}, "pending": {pending, r.campaignID, notFound},
		"anonymous":        {r.h.anonymous(), r.campaignID, "unauthenticated|unauthenticated: sign in to continue"},
		"unknown campaign": {r.master, newKey(), notFound}, "malformed campaign": {r.master, "not-a-uuid", notFound}, "empty campaign": {r.master, "", notFound},
	} {
		if got := shapeOf(call(tc.u, tc.campaign)); got != tc.want {
			t.Errorf("%s: error = %s, want %s", name, got, tc.want)
		}
	}
	r.master.end(t, r.master.liveSession(t, r.campaignID).GetGameSession())
	for name, u := range map[string]*user{"master": r.master, "player": r.caio} {
		if got := shapeOf(call(u, r.campaignID)); got != noSession {
			t.Errorf("no open session (%s): error = %s, want %s", name, got, noSession)
		}
	}
}
