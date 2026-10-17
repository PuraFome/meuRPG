package progression

import (
	"slices"
	"testing"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/proto"

	campaignsv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/campaigns/v1"
	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	progressionv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/progression/v1"
)

// Characterization of viewsOf and milestoneViews (MR-016, MR-041, RN-10,
// RN-20): what each kind of viewer reads of the history and of the
// milestones, over a campaign that held every kind of award (gold with its
// treasures, enemies with its combat, by hand, undone, given by an account
// that is gone) and then every state of a milestone (planned, reached, given
// to more, reached and undone, reached again, off the list, off the list and
// undone). Each viewer's answers are a golden file
// (testdata/history_*.golden.json, IDs replaced by labels, times by their
// rank); run with -update-history-golden to write them again after a change
// of behavior that was meant. Then each failure of a read, with its log line.

// historyScene builds the campaign of the goldens and labels its IDs.
func historyScene(t *testing.T) *historyRig {
	t.Helper()
	rig := newHistoryRig(t, gold, 3)
	ctx := t.Context()
	pens, toren, brisa := rig.pcs[0].GetId(), rig.pcs[1].GetId(), rig.pcs[2].GetId()
	award := func(name string, a *progressionv1.XPAward) {
		t.Helper()
		if a.GetId() == "" {
			t.Fatalf("%s: no award", name)
		}
		rig.label(a.GetId(), name)
	}

	// Gold: two treasures, one found in the session; then 30 XP by hand to Brisa.
	mapID := rig.newMap()
	rig.label(mapID, "map")
	session := rig.startSession(t)
	rig.label(session, "session")
	if _, err := rig.master.play.SetCurrentMap(ctx, connect.NewRequest(&playv1.SetCurrentMapRequest{CampaignId: rig.campaign, MapId: mapID})); err != nil {
		t.Fatalf("SetCurrentMap() error = %v", err)
	}
	// The treasures are listed by point ID, a random order: they are worth the
	// same and labeled by that order.
	bolsa := rig.treasure(t, mapID, "Bolsa do capitão", 90, &session, toren)
	idolo := rig.treasure(t, mapID, "Ídolo de prata", 90, nil, pens)
	rig.label(min(bolsa, idolo), "treasure-first")
	rig.label(max(bolsa, idolo), "treasure-second")
	res, err := rig.backToTown(newKey(), []string{pens, toren}, bolsa, idolo)
	if err != nil {
		t.Fatalf("AwardXP(gold) error = %v", err)
	}
	award("award-gold", res.GetAward())
	byHand := rig.master.manual(t, rig.campaign, 30, brisa).GetAward()
	award("award-manual-gone-master", byHand)

	// Enemies: a combat with two goblins, one defeated; then 10 XP by hand, undone.
	if err := rig.master.setXPMode(rig.campaign, campaignsv1.XpMode_XP_MODE_ENEMIES, true); err != nil {
		t.Fatalf("SetCampaignXpMode(enemies) error = %v", err)
	}
	goblin := rig.master.minion(t, rig.campaign, "Goblin", 50)
	rig.label(goblin.GetId(), "goblin")
	enc := rig.fight(t, goblin, 2, 1, true)
	rig.label(enc.GetId(), "encounter")
	for i, c := range enc.GetCombatants() {
		rig.label(c.GetId(), "combatant"+string(rune('a'+i)))
	}
	res, err = rig.enemies(enc.GetId(), pens, toren, brisa)
	if err != nil {
		t.Fatalf("AwardXP(enemies) error = %v", err)
	}
	award("award-enemies", res.GetAward())
	award("award-manual-undone", rig.master.manual(t, rig.campaign, 10, pens).GetAward())
	if _, err := rig.master.undo(rig.campaign, newKey()); err != nil {
		t.Fatalf("UndoLastXPAward() error = %v", err)
	}
	// The master's account of the 30 XP is gone (the row keeps the award, without who gave it).
	if _, err := rig.h.pool.Exec(ctx, `UPDATE xp_awards SET given_by = NULL WHERE id = $1`, byHand.GetId()); err != nil {
		t.Fatalf("clear given_by: %v", err)
	}

	// Milestones: A reached and given to more, B reached and undone (planned
	// again), C reached, undone and reached again, D never; two off the list,
	// the second undone.
	if err := rig.master.setXPMode(rig.campaign, campaignsv1.XpMode_XP_MODE_MILESTONES, true); err != nil {
		t.Fatalf("SetCampaignXpMode(milestones) error = %v", err)
	}
	a := rig.master.plan(t, rig.campaign, "Chegar ao Vale Seco")
	b := rig.master.plan(t, rig.campaign, "Revelar o traidor da guilda")
	c := rig.master.plan(t, rig.campaign, "Derrotar a Sombra")
	d := rig.master.plan(t, rig.campaign, "Fechar o portal")
	for name, m := range map[string]*progressionv1.Milestone{"milestone-a": a, "milestone-b": b, "milestone-c": c, "milestone-d": d} {
		rig.label(m.GetId(), name)
	}
	award("mark-a", rig.master.reach(t, rig.campaign, a.GetId(), pens).GetAward())
	given, err := rig.master.giveMilestone(a.GetId(), rig.campaign, toren)
	if err != nil {
		t.Fatalf("GiveMilestoneTo() error = %v", err)
	}
	award("mark-a-given", given.GetAward())
	award("mark-b-undone", rig.master.reach(t, rig.campaign, b.GetId(), brisa).GetAward())
	if _, err := rig.master.undo(rig.campaign, newKey()); err != nil {
		t.Fatalf("UndoLastXPAward(b) error = %v", err)
	}
	award("mark-c-undone", rig.master.reach(t, rig.campaign, c.GetId(), pens).GetAward())
	if _, err := rig.master.undo(rig.campaign, newKey()); err != nil {
		t.Fatalf("UndoLastXPAward(c) error = %v", err)
	}
	award("mark-c-again", rig.master.reach(t, rig.campaign, c.GetId(), toren).GetAward())
	award("mark-off-list", rig.master.milestone(t, rig.campaign, brisa))
	off, err := rig.master.xp.MarkMilestone(ctx, connect.NewRequest(&progressionv1.MarkMilestoneRequest{
		CampaignId: rig.campaign, Reason: "Achar o mapa do tesouro", CharacterIds: []string{pens}, IdempotencyKey: newKey(),
	}))
	if err != nil {
		t.Fatalf("MarkMilestone(second) error = %v", err)
	}
	award("mark-off-list-undone", off.Msg.GetAward())
	if _, err := rig.master.undo(rig.campaign, newKey()); err != nil {
		t.Fatalf("UndoLastXPAward(off the list) error = %v", err)
	}
	rig.takeLogs(t)
	return rig
}

// historyAnswer is a response of the rig as a golden holds it.
func historyAnswer[T any, PT interface {
	*T
	proto.Message
}](t *testing.T, rig *historyRig, res *connect.Response[T], err error) any {
	t.Helper()
	if err != nil {
		return rig.historyRead(t, nil, err)
	}
	return rig.historyRead(t, PT(res.Msg), nil)
}

// historyReads is what u reads of the history and of the milestones.
func (rig *historyRig) historyReads(t *testing.T, u *user) map[string]any {
	t.Helper()
	client := rig.client(u)
	hist, err := client.ListXPAwards(t.Context(), connect.NewRequest(&progressionv1.ListXPAwardsRequest{CampaignId: rig.campaign, PageSize: maxPageSize}))
	reads := map[string]any{"ListXPAwards": historyAnswer(t, rig, hist, err)}
	list, err := client.ListMilestones(t.Context(), connect.NewRequest(&progressionv1.ListMilestonesRequest{CampaignId: rig.campaign}))
	reads["ListMilestones"] = historyAnswer(t, rig, list, err)
	return reads
}

// TestHistoryAndMilestonesWhatEachViewerReads: the master reads every award,
// with its treasures and the one "Desfazer", and every milestone; a player
// reads every award with the count of treasures only, and the reason of an
// undone mark only while its milestone is reached again, and only the reached
// milestones; an outsider and a pending member read nothing.
func TestHistoryAndMilestonesWhatEachViewerReads(t *testing.T) {
	t.Parallel()
	rig := historyScene(t)
	outsider := rig.h.newUser("Intruso")
	pending := rig.h.newUser("Pendente")
	rig.h.joinPending(rig.master, rig.campaign, pending)
	for name, u := range map[string]*user{
		"master": rig.master, "pensantus": rig.players[0], "brisa": rig.players[2],
		"outsider": outsider, "pending": pending, "anonymous": rig.h.anonymous(),
	} {
		rig.wantHistoryGolden(t, "history_"+name, rig.historyReads(t, u))
	}
	wantHistoryLines(t, "the logs of the reads", rig.takeLogs(t), nil)

	// Each failure of a read: the code the app gets and the one log line.
	listAwards := func(u *user) error {
		_, err := rig.client(u).ListXPAwards(t.Context(), connect.NewRequest(&progressionv1.ListXPAwardsRequest{CampaignId: rig.campaign}))
		return err
	}
	listMilestones := func(u *user) error {
		_, err := rig.client(u).ListMilestones(t.Context(), connect.NewRequest(&progressionv1.ListMilestonesRequest{CampaignId: rig.campaign}))
		return err
	}
	player := rig.players[1]
	for _, tc := range []struct {
		name  string
		arm   func()
		call  func(*user) error
		u     *user
		want  string
		lines []string
	}{
		{"history: the characters' names", func() { rig.faults.arm("names") }, listAwards, rig.master, "internal: internal error", []string{"ERROR progression: cannot read XP awards code=internal error=read the characters' names: injected failure"}},
		{"history: the display names", func() { rig.faults.arm("display_names") }, listAwards, rig.master, "internal: internal error", []string{"ERROR progression: cannot read XP awards code=internal error=read display names: injected failure"}},
		{"history: the combats' names", func() { rig.faults.arm("encounter_names") }, listAwards, rig.master, "internal: internal error", []string{"ERROR progression: cannot read XP awards code=internal error=read the combats' names: injected failure"}},
		{"history: the player's marks", func() { rig.faults.armQuery("ListLiveMilestoneAwards", 0) }, listAwards, player, "canceled: the request was canceled", []string{"DEBUG progression: cannot read XP awards code=canceled error=list the marks: timeout: context already done: context canceled"}},
		{"history: the page", func() { rig.faults.armQuery("ListXPAwardsPage", 0) }, listAwards, player, "canceled: the request was canceled", []string{"DEBUG progression: cannot read XP awards code=canceled error=list XP awards: timeout: context already done: context canceled"}},
		{"milestones: the planned ones", func() { rig.faults.armQuery("ListPlannedMilestones", 0) }, listMilestones, rig.master, "canceled: the request was canceled", []string{"DEBUG progression: cannot list milestones code=canceled error=list the milestones: timeout: context already done: context canceled"}},
		{"milestones: the marks", func() { rig.faults.armQuery("ListLiveMilestoneAwards", 0) }, listMilestones, rig.master, "canceled: the request was canceled", []string{"DEBUG progression: cannot list milestones code=canceled error=list the marks: timeout: context already done: context canceled"}},
		{"milestones: the ones with history", func() { rig.faults.armQuery("ListMilestoneIDsWithAwards", 0) }, listMilestones, rig.master, "canceled: the request was canceled", []string{"DEBUG progression: cannot list milestones code=canceled error=list the milestones with history: timeout: context already done: context canceled"}},
		{"milestones: the marks' shares", func() { rig.faults.armQuery("ListXPSharesOfAwards", 0) }, listMilestones, player, "canceled: the request was canceled", []string{"DEBUG progression: cannot list milestones code=canceled error=list the shares: timeout: context already done: context canceled"}},
		{"milestones: the characters' names", func() { rig.faults.arm("names") }, listMilestones, player, "internal: internal error", []string{"ERROR progression: cannot list milestones code=internal error=read the characters' names: injected failure"}},
		{"milestones: the display names", func() { rig.faults.arm("display_names") }, listMilestones, rig.master, "internal: internal error", []string{"ERROR progression: cannot list milestones code=internal error=read display names: injected failure"}},
	} {
		tc.arm()
		if got := rig.redact(historyRefusal(tc.call(tc.u))); got != tc.want {
			t.Errorf("%s: error = %s, want %s", tc.name, got, tc.want)
		}
		wantHistoryLines(t, tc.name+": logs", rig.takeLogs(t), tc.lines)
		rig.wantNoPendingHistoryFault(t, tc.name)
	}

	// The milestones' marks have no combat, so the combats are not asked: a failing
	// read of their names does not matter, and the fault is still armed afterwards.
	rig.faults.arm("encounter_names")
	if got := historyRefusal(listMilestones(rig.master)); got != "ok" {
		t.Errorf("milestones with no combat award, the combats failing: error = %s, want ok", got)
	}
	wantHistoryLines(t, "milestones with no combat award: logs", rig.takeLogs(t), nil)
	if p := rig.faults.pending(); !slices.Equal(p, []string{"encounter_names"}) {
		t.Errorf("milestones with no combat award: pending faults = %v, want the combats' names untouched", p)
	}
	if err := rig.faults.take("encounter_names"); err == nil {
		t.Error("the combats' names fault was not armed")
	}
}

// TestMilestonesOfACampaignWithNoMark: with no mark, the master reads the
// planned milestones (none with history) and a player an empty list; no
// award is read at all.
func TestMilestonesOfACampaignWithNoMark(t *testing.T) {
	t.Parallel()
	rig := newHistoryRig(t, milestones, 1)
	rig.label(rig.master.plan(t, rig.campaign, "Chegar ao Vale Seco").GetId(), "milestone-a")
	rig.label(rig.master.plan(t, rig.campaign, "Derrotar a Sombra").GetId(), "milestone-b")
	// A failing read of the shares would show if the marks were read with none.
	rig.faults.armQuery("ListXPSharesOfAwards", 0)
	rig.faults.arm("names")
	reads := map[string]any{}
	for name, u := range map[string]*user{"master": rig.master, "player": rig.players[0]} {
		res, err := rig.client(u).ListMilestones(t.Context(), connect.NewRequest(&progressionv1.ListMilestonesRequest{CampaignId: rig.campaign}))
		reads[name] = historyAnswer(t, rig, res, err)
	}
	rig.wantHistoryGolden(t, "milestones_no_mark", reads)
	if p := rig.faults.pending(); len(p) != 2 {
		t.Errorf("faults fired with no mark: pending %v, want both", p)
	}
}
