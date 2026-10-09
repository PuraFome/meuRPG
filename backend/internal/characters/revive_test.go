package characters

import (
	"context"
	"testing"

	"connectrpc.com/connect"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// Reviver (RN-03, SRD 5.1 "Dropping to 0 Hit Points"): the master brings a dead character back with
// 1 hit point, as it was before it died, while its player has no other living character.

// revive calls ReviveCharacter as u.
func (u *user) revive(c *charactersv1.Character, key string) (*charactersv1.Character, error) {
	res, err := u.api.ReviveCharacter(context.Background(), connect.NewRequest(&charactersv1.ReviveCharacterRequest{
		CampaignId: c.GetCampaignId(), CharacterId: c.GetId(), IdempotencyKey: key,
	}))
	if err != nil {
		return nil, err
	}
	return res.Msg.GetCharacter(), nil
}

// adjust changes a character's vitals the way package play does, inside a transaction.
func (h *harness) adjust(campaignID, characterID string, req *playv1.AdjustCharacterVitalsRequest) {
	h.t.Helper()
	tx, err := h.pool.Begin(h.t.Context())
	if err != nil {
		h.t.Fatalf("begin: %v", err)
	}
	defer func() { _ = tx.Rollback(h.t.Context()) }()
	if _, _, err := h.svc.AdjustVitals(h.t.Context(), tx, campaignID, characterID, req); err != nil {
		h.t.Fatalf("AdjustVitals() error = %v", err)
	}
	if err := tx.Commit(h.t.Context()); err != nil {
		h.t.Fatalf("commit: %v", err)
	}
}

// reviveTable is a campaign where Lia's approved, locked character fell in a fight and the master
// marked it dead, after it spent a 1st level slot.
type reviveTable struct {
	*reviewTable
	dead *charactersv1.Character
}

func newReviveTable(t *testing.T) *reviveTable {
	t.Helper()
	tb, _ := newReviewTable(t)
	tb.master.approve(t, tb.pc)
	tb.h.lockSheets(tb.campaign)
	hp := int32(0)
	tb.h.adjust(tb.campaign, tb.pc.GetId(), &playv1.AdjustCharacterVitalsRequest{
		SpellSlotsUsed: []*playv1.SpellSlotsUsed{{Level: 1, Used: 1}}, HitPointsCurrent: &hp,
	})
	return &reviveTable{reviewTable: tb, dead: tb.master.markDead(t, tb.pc)}
}

// RN-03: Reviver gives 1 hit point, keeps the slots, goes back to the state before the death and
// leaves the history.
func TestRN03_ReviveBringsTheCharacterBackWithOneHitPoint(t *testing.T) {
	t.Parallel()
	tb := newReviveTable(t)
	if !tb.dead.GetCanRevive() || tb.dead.GetCanMarkDead() || tb.dead.GetDiedAt() == nil {
		t.Fatalf("the dead character = can_revive %v, can_mark_dead %v, died_at %v; want revivable", tb.dead.GetCanRevive(), tb.dead.GetCanMarkDead(), tb.dead.GetDiedAt())
	}
	revived, err := tb.master.revive(tb.dead, key())
	if err != nil {
		t.Fatalf("ReviveCharacter() error = %v", err)
	}
	if revived.GetState() != charactersv1.CharacterState_CHARACTER_STATE_LOCKED || revived.GetDiedAt() != nil || revived.GetRevivedAt() == nil ||
		revived.GetCanRevive() || !revived.GetCanMarkDead() || revived.GetRevision() != tb.dead.GetRevision() {
		t.Fatalf("ReviveCharacter() = state %v died_at %v revived_at %v can_revive %v; want LOCKED as before, alive, same revision", revived.GetState(), revived.GetDiedAt(), revived.GetRevivedAt(), revived.GetCanRevive())
	}
	v, err := tb.h.svc.GetVitals(t.Context(), tb.campaign, tb.pc.GetId())
	if err != nil || v.GetHitPointsCurrent() != 1 {
		t.Fatalf("vitals after the revival = %v, %v; want 1 hit point", v, err)
	}
	if got := slotUsed(v, 1); got != 1 {
		t.Errorf("1st level slots used after the revival = %d, want 1: Reviver gives nothing back", got)
	}
	// Her player reads the revival, and the master's history keeps when she died.
	if hers := tb.lia.get(t, tb.campaign, tb.pc.GetId()); hers.GetRevivedAt() == nil || hers.GetDiedAt() != nil {
		t.Errorf("owner's GetCharacter() revived_at %v died_at %v; want revived", hers.GetRevivedAt(), hers.GetDiedAt())
	}
	var diedAt *string
	if err := tb.h.pool.QueryRow(t.Context(), "SELECT died_at::TEXT FROM characters WHERE id = $1", tb.pc.GetId()).Scan(&diedAt); err != nil || diedAt == nil {
		t.Errorf("died_at in the table = %v, %v; want it kept as history", diedAt, err)
	}
	if got := tb.host.told(); len(got) == 0 || got[len(got)-1] != "revived:"+tb.pc.GetId() {
		t.Errorf("the stream was told %v, want the revival last", got)
	}
}

func slotUsed(v *playv1.CharacterVitals, level int32) int32 {
	for _, s := range v.GetSpellSlots() {
		if s.GetLevel() == level {
			return s.GetUsed()
		}
	}
	return 0
}

// RN-03: a retry with the same key is the first answer; another key finds a living character.
func TestRN03_ReviveIsIdempotentAndRefusesTheLiving(t *testing.T) {
	t.Parallel()
	tb := newReviveTable(t)
	k := key()
	if _, err := tb.master.revive(tb.dead, k); err != nil {
		t.Fatalf("ReviveCharacter() error = %v", err)
	}
	if again, err := tb.master.revive(tb.dead, k); err != nil || again.GetState() != charactersv1.CharacterState_CHARACTER_STATE_LOCKED {
		t.Errorf("retry of ReviveCharacter = %v, %v; want the character as it is", again.GetState(), err)
	}
	if got := len(tb.host.told()); got == 0 {
		t.Fatalf("the stream heard nothing")
	}
	told := len(tb.host.told())
	if _, err := tb.master.revive(tb.dead, k); err != nil || len(tb.host.told()) != told {
		t.Errorf("a retry told the stream again (%d -> %d), err %v", told, len(tb.host.told()), err)
	}
	_, err := tb.master.revive(tb.dead, key())
	if d := blocked(t, "ReviveCharacter(living)", err); d.GetReason() != charactersv1.CharacterBlockedReason_CHARACTER_BLOCKED_REASON_NOT_DEAD {
		t.Errorf("CharacterBlocked = %v, want NOT_DEAD", d)
	}
	npc := tb.master.create(t, tb.campaign, charactersv1.CharacterKind_CHARACTER_KIND_BOSS, "Strahd", enemySheet())
	_, err = tb.master.revive(npc, key())
	wantCode(t, "ReviveCharacter(an NPC)", err, connect.CodeInvalidArgument)
	_, err = tb.master.revive(tb.dead, "")
	wantCode(t, "ReviveCharacter(no key)", err, connect.CodeInvalidArgument)
}

// RN-03: while the player has another living character, Reviver is refused and names it; once the
// master marks that one dead, it works.
func TestRN03_ReviveIsRefusedWhileThePlayerHasAnotherLivingCharacter(t *testing.T) {
	t.Parallel()
	tb := newReviveTable(t)
	nuvem := tb.lia.create(t, tb.campaign, charactersv1.CharacterKind_CHARACTER_KIND_PLAYER, "Nuvem", pensantusSheet()) // she may: the other died
	tb.master.approve(t, nuvem)

	_, err := tb.master.revive(tb.dead, key())
	d := blocked(t, "ReviveCharacter(another living)", err)
	if d.GetReason() != charactersv1.CharacterBlockedReason_CHARACTER_BLOCKED_REASON_LIVING_CHARACTER_EXISTS || d.GetCharacterId() != nuvem.GetId() || d.GetLivingCharacterName() != "Nuvem" {
		t.Fatalf("CharacterBlocked = %v, want LIVING_CHARACTER_EXISTS naming Nuvem", d)
	}
	if got := tb.master.get(t, tb.campaign, tb.pc.GetId()); got.GetState() != charactersv1.CharacterState_CHARACTER_STATE_DEAD {
		t.Errorf("the dead character after the refusal = %v, want it still dead", got.GetState())
	}
	if v, err := tb.h.svc.GetVitals(t.Context(), tb.campaign, nuvem.GetId()); err != nil || v == nil {
		t.Errorf("the other character's vitals = %v, %v", v, err)
	}
	// The master retires the other one first; the app does nothing by itself.
	tb.master.markDead(t, nuvem)
	if _, err := tb.master.revive(tb.dead, key()); err != nil {
		t.Errorf("ReviveCharacter() after the other died error = %v", err)
	}
	// A pending character counts too (RN-03): it is living.
	tb2 := newReviveTable(t)
	tb2.h.joinPending(tb2.master, tb2.campaign, tb2.h.newUser("Zé"))
	pend := tb2.lia.create(t, tb2.campaign, charactersv1.CharacterKind_CHARACTER_KIND_PLAYER, "Pendente", pensantusSheet())
	if _, err := tb2.master.revive(tb2.dead, key()); connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Errorf("ReviveCharacter() with %q alive error = %v, want failed_precondition", pend.GetName(), err)
	}
}

// RN-03: a second death starts fresh: the switch of Revivify is off and the date is new.
func TestRN03_ASecondDeathStartsFresh(t *testing.T) {
	t.Parallel()
	tb := newReviveTable(t)
	if _, err := tb.h.pool.Exec(t.Context(), "UPDATE characters SET revivify_blocked = true WHERE id = $1", tb.pc.GetId()); err != nil {
		t.Fatalf("set the switch: %v", err)
	}
	if got := tb.master.get(t, tb.campaign, tb.pc.GetId()); !got.GetRevivifyBlocked() {
		t.Fatalf("master does not read the switch")
	}
	if got := tb.lia.get(t, tb.campaign, tb.pc.GetId()); got.GetRevivifyBlocked() {
		t.Fatalf("owner reads the master's switch: a player never learns why a creature cannot be revived")
	}
	if _, err := tb.master.revive(tb.dead, key()); err != nil {
		t.Fatalf("ReviveCharacter() error = %v", err)
	}
	again := tb.master.markDead(t, tb.pc)
	if again.GetState() != charactersv1.CharacterState_CHARACTER_STATE_DEAD || again.GetRevivifyBlocked() || again.GetDiedAt().AsTime().Before(tb.dead.GetDiedAt().AsTime()) {
		t.Errorf("the second death = state %v blocked %v died_at %v; want dead, switch off, a new date", again.GetState(), again.GetRevivifyBlocked(), again.GetDiedAt())
	}
}

// RN-10, RN-03: a dead character's page is whole for the master and the owner and not found for
// the other players.
func TestRN10_ADeadCharactersPageIsNotFoundToOtherPlayers(t *testing.T) {
	t.Parallel()
	tb := newReviveTable(t)
	for who, u := range map[string]*user{"master": tb.master, "owner": tb.lia} {
		if got := u.get(t, tb.campaign, tb.pc.GetId()); got.GetSheet() == nil || got.GetState() != charactersv1.CharacterState_CHARACTER_STATE_DEAD {
			t.Errorf("%s reads %v, want the whole dead character", who, got)
		}
	}
	_, err := tb.other.api.GetCharacter(t.Context(), connect.NewRequest(&charactersv1.GetCharacterRequest{CampaignId: tb.campaign, CharacterId: tb.pc.GetId()}))
	wantCode(t, "another player's GetCharacter(dead)", err, connect.CodeNotFound)
	_, err = tb.other.revive(tb.dead, key())
	wantCode(t, "another player's ReviveCharacter", err, connect.CodePermissionDenied)
}
