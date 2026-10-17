package play

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"

	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/proto"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
)

// A roll can wait for a question before it resolves: the halfling's Lucky about a 1, a Bardic
// Inspiration die about the roll. Two tables keep such a roll, roll_holds for a weapon attack
// (combat_inspiration.go) and inspiration_holds for the d20 rolls that are not (inspiration_held.go);
// the stores of this file give both the one shape the gate (held_gate.go) works on, heldRoll, and
// write an answer back where the roll lives.

// heldKind is the kind of roll that waits; held_registry.go says what each one does.
type heldKind string

// The kinds of roll that wait, which are the values of inspiration_holds.kind plus the weapon
// attack of roll_holds. The first four are rolled out of a combat (the trap search and the hint try
// also by a character in one, when the die is its combatant's); spell_attack, effect_save and
// concentration_save only in a combat, by a player's character.
const (
	heldWeaponAttack      heldKind = "weapon_attack"
	heldSpellAttack       heldKind = "spell_attack"
	heldEffectSave        heldKind = "effect_save"
	heldConcentrationSave heldKind = "concentration_save"
	heldSceneCheck        heldKind = "scene_check"
	heldGroupCheck        heldKind = "group_check"
	heldTrapSearch        heldKind = "trap_search"
	heldHintTry           heldKind = "hint_try"
)

// heldTable is the table a held roll lives in. The zero value is "either", for a lookup that does not
// know yet.
type heldTable int

const (
	rollHolds heldTable = iota + 1
	inspirationHolds
)

// checksTheAnswer says the table refuses an answer to a roll whose question went stale (its combat
// round is over) or whose die is gone. Today's behavior: roll_holds leaves it to the attack's own checks
// of the turn, and E4-6, which moves the attack to the other table, decides it.
func (t heldTable) checksTheAnswer() bool { return t == inspirationHolds }

// checksTheAnswer is heldTable.checksTheAnswer for a roll as found: a weapon attack kept in inspiration_holds
// (a later release writes it there) answers as it always did in roll_holds, by the attack's own checks.
func (h heldRoll) checksTheAnswer() bool {
	return h.table.checksTheAnswer() && h.Kind != heldWeaponAttack
}

// heldRoll is one held roll from either table in one shape; table says where its answer is written
// back.
type heldRoll struct {
	ID, CampaignID, CharacterID, CombatantID, EncounterID, UserID string
	// Key is the key of the request that made the roll wait, without the "#window" a reaction
	// window's replay adds (Window says it did).
	Key                       string
	Kind                      heldKind
	Window                    bool
	Physical                  bool
	Request                   []byte
	Faces                     []int
	Counted                   int
	Modifier                  int
	Total                     int
	Round                     *int32
	LuckyIndex, LuckyTo       *int32
	LuckyAnswerKey, AnswerKey *string
	table                     heldTable
}

// heldRoller is who rolls: the character, and its combatant in a combat. Combatant is nil out of
// one.
type heldRoller struct {
	CampaignID, CharacterID, UserID string
	Combatant                       *playdb.Combatant
	Round                           int32
}

// in is the roller as the state found it: its combatant and the round of its combat, for the character
// of a roll that does not know where it is.
func (r heldRoller) in(st rollerState) heldRoller {
	r.Combatant, r.Round = st.Die.Combatant, st.Die.Round
	return r
}

// holdClass is the kind of roll an RPC answers: the questions of an attack roll and of every other roll have
// their own RPCs, and a hold of the other class is not found there, whichever table it is kept in.
type holdClass int

const (
	anyHold holdClass = iota
	attackHold
	outsideHold
)

// admits says the RPC of the class answers a roll of the kind, in the combat the reference names (none
// when it names none).
func (c holdClass) admits(h heldRoll, encounter string) bool {
	switch c {
	case attackHold:
		return h.Kind == heldWeaponAttack && (encounter == "" || h.EncounterID == encounter)
	case outsideHold:
		return h.Kind != heldWeaponAttack
	}
	return true
}

// holdRef says which hold to look for. A zero Table looks in both tables; Encounter is only for
// roll_holds, whose rows have no campaign, and for the weapon attacks kept in inspiration_holds, which
// must belong to it.
type holdRef struct {
	Table                   heldTable
	Class                   holdClass
	Campaign, Encounter, ID string
}

// heldStore is one table of held rolls.
type heldStore interface {
	// byKey finds the roll the request with this key made.
	byKey(ctx context.Context, roller heldRoller, key string) (heldRoll, bool, error)
	// byID finds a roll, pgx.ErrNoRows when there is none.
	byID(ctx context.Context, ref holdRef) (heldRoll, error)
	// openOf lists the rolls of the roller that wait for an answer.
	openOf(ctx context.Context, roller heldRoller) ([]heldRoll, error)
	// insert keeps a roll that waits and gives it back as stored.
	insert(ctx context.Context, env gateEnv, h heldRoll) (heldRoll, error)
	// answerLucky writes the Lucky answer: after is the roll with the new d20; staysHeld says the die's
	// question comes next, so the roll keeps waiting.
	answerLucky(ctx context.Context, before, after heldRoll, staysHeld bool) error
	// answer closes the roll under the key of the answer that settled it.
	answer(ctx context.Context, h heldRoll, key string) error
	// forget deletes a roll nobody will answer.
	forget(ctx context.Context, h heldRoll) error
	// asked is called when a question starts to wait, for the stores whose question is part of the
	// combat the players read.
	asked(ctx context.Context, env gateEnv) error
}

// heldStores are both tables: they are read together, and written where a kind's registry entry says.
type heldStores struct{ rolls, inspirations heldStore }

func newHeldStores(q *playdb.Queries) heldStores {
	return heldStores{rolls: rollHoldStore{q}, inspirations: inspirationHoldStore{q}}
}

// of is the store of a table.
func (hs heldStores) of(t heldTable) heldStore {
	if t == rollHolds {
		return hs.rolls
	}
	return hs.inspirations
}

// byID finds a roll in the table the reference names, or in either, of the class it names.
func (hs heldStores) byID(ctx context.Context, ref holdRef) (heldRoll, error) {
	h, err := hs.findByID(ctx, ref)
	if err == nil && !ref.Class.admits(h, ref.Encounter) {
		return heldRoll{}, pgx.ErrNoRows
	}
	return h, err
}

func (hs heldStores) findByID(ctx context.Context, ref holdRef) (heldRoll, error) {
	if ref.Table != 0 {
		return hs.of(ref.Table).byID(ctx, ref)
	}
	h, err := hs.inspirations.byID(ctx, ref)
	if errors.Is(err, pgx.ErrNoRows) && ref.Encounter != "" {
		return hs.rolls.byID(ctx, ref)
	}
	return h, err
}

// byIDOf finds the roll of a kind: in the table the kind is written to, and, while a release writes it in
// one table and reads it in the other too, there.
func (hs heldStores) byIDOf(ctx context.Context, kind heldKind, ref holdRef) (heldRoll, error) {
	def := heldRegistry[kind]
	ref.Table = def.write
	h, err := hs.of(def.write).byID(ctx, ref)
	if errors.Is(err, pgx.ErrNoRows) && def.alsoRead != 0 {
		ref.Table = def.alsoRead
		if h, err = hs.of(def.alsoRead).byID(ctx, ref); err == nil && h.Kind != kind {
			return heldRoll{}, pgx.ErrNoRows
		}
	}
	return h, err
}

// byKeyOf is byIDOf for the roll the request with a key made.
func (hs heldStores) byKeyOf(ctx context.Context, kind heldKind, roller heldRoller, key string) (heldRoll, bool, error) {
	def := heldRegistry[kind]
	h, found, err := hs.of(def.write).byKey(ctx, roller, key)
	if err != nil || found || def.alsoRead == 0 {
		return h, found, err
	}
	if h, found, err = hs.of(def.alsoRead).byKey(ctx, roller, key); err != nil || !found || h.Kind != kind {
		return heldRoll{}, false, err
	}
	return h, true, nil
}

// openOf lists the rolls of the roller that wait in the tables, in the order given.
func (hs heldStores) openOf(ctx context.Context, roller heldRoller, tables ...heldTable) ([]heldRoll, error) {
	var out []heldRoll
	for _, t := range tables {
		open, err := hs.of(t).openOf(ctx, roller)
		if err != nil {
			return nil, err
		}
		out = append(out, open...)
	}
	return out, nil
}

// heldOfRollHold is a roll_holds row in the shape of the gate: the faces come from the request that
// was pinned with them (or are the one face), the die that counts is the pinned mode's pick, the total
// is the face and the bonus, and the d20 is physical when the request typed it.
func heldOfRollHold(row playdb.RollHold) heldRoll {
	var orig playv1.RollAttackRequest
	_ = proto.Unmarshal(row.Request, &orig) // a request this module wrote
	faces := []int{int(row.Face)}
	if pair := orig.GetD20Faces(); len(pair) == pairDice {
		faces = intsOf(pair)
	}
	mode, _ := modeFromProto(orig.GetRollMode())
	round := row.Round
	return heldRoll{
		ID: row.ID, CampaignID: orig.GetCampaignId(), CombatantID: row.CombatantID, EncounterID: row.EncounterID,
		Key: strings.TrimSuffix(row.IdempotencyKey, windowHoldSuffix), Kind: heldWeaponAttack,
		Window: strings.HasSuffix(row.IdempotencyKey, windowHoldSuffix), Physical: typedRoll(&orig), Request: row.Request,
		Faces: faces, Counted: mode.Pick(faces), Modifier: int(row.Modifier), Total: int(row.Face) + int(row.Modifier),
		Round: &round, LuckyIndex: row.LuckyIndex, LuckyTo: row.LuckyTo, LuckyAnswerKey: row.LuckyAnswerKey,
		AnswerKey: row.AnswerKey, table: rollHolds,
	}
}

// heldOfInspirationHold is an inspiration_holds row in the shape of the gate.
func heldOfInspirationHold(row playdb.InspirationHold) heldRoll {
	return heldRoll{
		ID: row.ID, CampaignID: row.CampaignID, CharacterID: row.CharacterID, UserID: row.UserID,
		CombatantID: deref(row.CombatantID), EncounterID: deref(row.EncounterID), Window: row.WindowHold,
		Key: row.IdempotencyKey, Kind: heldKind(row.Kind), Physical: row.Physical, Request: row.Request,
		Faces: intsOf(row.Faces), Counted: int(row.Counted), Modifier: int(row.Modifier), Total: int(row.Total),
		Round: row.Round, LuckyIndex: row.LuckyIndex, LuckyTo: row.LuckyTo, LuckyAnswerKey: row.LuckyAnswerKey,
		AnswerKey: row.AnswerKey, table: inspirationHolds,
	}
}

// rollHoldStore keeps the weapon attacks, in roll_holds: scoped by the encounter, owned by the combatant.
type rollHoldStore struct{ q *playdb.Queries }

func (st rollHoldStore) byKey(ctx context.Context, roller heldRoller, key string) (heldRoll, bool, error) {
	if roller.Combatant == nil {
		return heldRoll{}, false, nil
	}
	row, err := st.q.GetRollHoldByKey(ctx, playdb.GetRollHoldByKeyParams{EncounterID: roller.Combatant.EncounterID, IdempotencyKey: key})
	if errors.Is(err, pgx.ErrNoRows) {
		return heldRoll{}, false, nil
	}
	if err != nil {
		return heldRoll{}, false, fmt.Errorf("find the held roll: %w", err)
	}
	return heldOfRollHold(row), true, nil
}

func (st rollHoldStore) byID(ctx context.Context, ref holdRef) (heldRoll, error) {
	row, err := st.q.GetRollHold(ctx, playdb.GetRollHoldParams{EncounterID: ref.Encounter, ID: ref.ID})
	if err != nil {
		return heldRoll{}, err //nolint:wrapcheck // the caller says what it was looking for
	}
	h := heldOfRollHold(row)
	if ref.Campaign != "" && h.CampaignID != ref.Campaign { // the row has no campaign: the request it kept names it
		return heldRoll{}, pgx.ErrNoRows
	}
	return h, nil
}

func (st rollHoldStore) openOf(ctx context.Context, roller heldRoller) ([]heldRoll, error) {
	if roller.Combatant == nil {
		return nil, nil
	}
	row, err := st.q.GetOpenRollHoldOf(ctx, playdb.GetOpenRollHoldOfParams{EncounterID: roller.Combatant.EncounterID, CombatantID: roller.Combatant.ID})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("find the open held roll: %w", err)
	}
	return []heldRoll{heldOfRollHold(row)}, nil
}

func (st rollHoldStore) insert(ctx context.Context, env gateEnv, h heldRoll) (heldRoll, error) {
	key := h.Key
	if h.Window {
		key += windowHoldSuffix
	}
	row, err := st.q.InsertRollHold(ctx, playdb.InsertRollHoldParams{
		EncounterID: h.EncounterID, CombatantID: h.CombatantID, IdempotencyKey: key, Request: h.Request,
		Face: clamp32(h.Faces[h.Counted], 1, d20Sides), Modifier: clamp32(h.Modifier, math.MinInt32, math.MaxInt32),
		Round: derefInt32(h.Round), CreatedAt: env.now, LuckyIndex: h.LuckyIndex,
	})
	if err != nil {
		return heldRoll{}, fmt.Errorf("hold the roll: %w", err)
	}
	return heldOfRollHold(row), nil
}

// answerLucky writes the Lucky answer's key and the new face. A roll that stays held for the die also
// keeps the d20 after it (the face that counts and the request's pair of faces), which the die's answer
// settles; a roll resolved now keeps its request as it was, the one its event is written for.
func (st rollHoldStore) answerLucky(ctx context.Context, before, after heldRoll, staysHeld bool) error {
	face, body := before.Faces[before.Counted], before.Request
	if staysHeld {
		var err error
		if body, err = withHeldFaces(after.Request, after.Faces); err != nil {
			return err
		}
		face = after.Faces[after.Counted]
	}
	_, err := st.q.AnswerRollHoldLucky(ctx, playdb.AnswerRollHoldLuckyParams{
		ID: after.ID, Face: clamp32(face, 1, d20Sides), Request: body, LuckyAnswerKey: after.LuckyAnswerKey, LuckyTo: after.LuckyTo,
	})
	return err //nolint:wrapcheck // the question wraps it
}

func (st rollHoldStore) answer(ctx context.Context, h heldRoll, key string) error {
	return st.q.AnswerRollHold(ctx, playdb.AnswerRollHoldParams{ID: h.ID, AnswerKey: &key}) //nolint:wrapcheck // the gate wraps it
}

func (st rollHoldStore) forget(ctx context.Context, h heldRoll) error {
	return st.q.DeleteRollHold(ctx, h.ID) //nolint:wrapcheck // the gate wraps it
}

// asked moves the combat's revision: the question is part of the combat the master and the player read.
func (st rollHoldStore) asked(ctx context.Context, env gateEnv) error {
	if env.c == nil {
		return nil
	}
	enc, err := env.c.q.TouchEncounter(ctx, env.c.enc.ID)
	if err != nil {
		return fmt.Errorf("touch the encounter: %w", err)
	}
	env.c.enc = enc
	return nil
}

// withHeldFaces is the request of a held attack with the pair of d20 it was rolled with.
func withHeldFaces(request []byte, faces []int) ([]byte, error) {
	var req playv1.RollAttackRequest
	if err := proto.Unmarshal(request, &req); err != nil {
		return nil, fmt.Errorf("read the held request: %w", err)
	}
	if len(faces) == pairDice {
		req.D20Faces = faces32(faces)
	}
	body, err := proto.Marshal(&req)
	if err != nil {
		return nil, fmt.Errorf("keep the request: %w", err)
	}
	return body, nil
}

// inspirationHoldStore keeps the other d20 rolls, in inspiration_holds: scoped by the campaign, owned by
// the character.
type inspirationHoldStore struct{ q *playdb.Queries }

func (st inspirationHoldStore) byKey(ctx context.Context, roller heldRoller, key string) (heldRoll, bool, error) {
	row, err := st.q.GetInspirationHoldByKey(ctx, playdb.GetInspirationHoldByKeyParams{CampaignID: roller.CampaignID, IdempotencyKey: key})
	if errors.Is(err, pgx.ErrNoRows) {
		return heldRoll{}, false, nil
	}
	if err != nil {
		return heldRoll{}, false, fmt.Errorf("find the held roll: %w", err)
	}
	return heldOfInspirationHold(row), true, nil
}

func (st inspirationHoldStore) byID(ctx context.Context, ref holdRef) (heldRoll, error) {
	row, err := st.q.GetInspirationHold(ctx, playdb.GetInspirationHoldParams{CampaignID: ref.Campaign, ID: ref.ID})
	if err != nil {
		return heldRoll{}, err //nolint:wrapcheck // the caller says what it was looking for
	}
	return heldOfInspirationHold(row), nil
}

// openOf lists the rolls the roller owns that wait: its combatant's, in a combat, and the character's that
// name no combatant (made out of a combat, or before the column was written). A combatant that is not
// backed by a character (a creature, an NPC's copy) owns only its own, whatever character_id it carries.
func (st inspirationHoldStore) openOf(ctx context.Context, roller heldRoller) ([]heldRoll, error) {
	owner := holdOwnerOf(roller)
	if owner.CombatantID == nil && owner.CharacterID == nil {
		return nil, nil
	}
	rows, err := st.q.ListOpenInspirationHolds(ctx, playdb.ListOpenInspirationHoldsParams(owner))
	if err != nil {
		return nil, fmt.Errorf("list the held rolls: %w", err)
	}
	out := make([]heldRoll, len(rows))
	for i, row := range rows {
		out[i] = heldOfInspirationHold(row)
	}
	return out, nil
}

// holdOwner is whom the holds of inspiration_holds belong to: the combatant, in its combat, and the character
// that backs it (or that rolls out of a combat). A nil field matches nothing.
type holdOwner struct{ EncounterID, CombatantID, CharacterID *string }

// holdOwnerOfCombatant is the owner of the holds of a combatant that a character backs.
func holdOwnerOfCombatant(c playdb.Combatant) holdOwner {
	r := heldRoller{Combatant: &c}
	if backedByCharacter(c) {
		r.CharacterID = c.CharacterID
	}
	return holdOwnerOf(r)
}

// holdOwnerOf is the owner of the roller's holds.
func holdOwnerOf(r heldRoller) holdOwner {
	var o holdOwner
	if r.Combatant != nil {
		o.EncounterID, o.CombatantID = &r.Combatant.EncounterID, &r.Combatant.ID
	}
	if r.CharacterID != "" {
		o.CharacterID = &r.CharacterID
	}
	return o
}

func (st inspirationHoldStore) insert(ctx context.Context, env gateEnv, h heldRoll) (heldRoll, error) {
	row, err := st.q.InsertInspirationHold(ctx, playdb.InsertInspirationHoldParams{
		CampaignID: h.CampaignID, CharacterID: h.CharacterID, UserID: h.UserID, Kind: string(h.Kind), IdempotencyKey: h.Key,
		Request: h.Request, Faces: faces32(h.Faces), Modifier: clamp32(h.Modifier, -heldBonusLimit, heldBonusLimit),
		Total: clamp32(h.Total, -heldBonusLimit, heldBonusLimit), Counted: clamp32(h.Counted, 0, 1), Physical: h.Physical,
		CreatedAt: env.now, Round: h.Round, LuckyIndex: h.LuckyIndex,
		CombatantID: nilIfEmpty(h.CombatantID), EncounterID: nilIfEmpty(h.EncounterID), WindowHold: h.Window && h.Kind == heldWeaponAttack,
	})
	if err != nil {
		return heldRoll{}, fmt.Errorf("hold the roll: %w", err)
	}
	return heldOfInspirationHold(row), nil
}

// heldBonusLimit bounds the modifier and the total a hold keeps: no sheet gets near it.
const heldBonusLimit = 1000

// answerLucky writes the d20 after the Lucky answer (the faces, the one that counts and the total), the
// answer's key and the new face. The roll is read from here by the die's question and by the settled
// roll alike, so it is written whether the die comes next or not.
func (st inspirationHoldStore) answerLucky(ctx context.Context, _, after heldRoll, _ bool) error {
	_, err := st.q.AnswerInspirationHoldLucky(ctx, playdb.AnswerInspirationHoldLuckyParams{
		ID: after.ID, Faces: faces32(after.Faces), Counted: clamp32(after.Counted, 0, 1),
		Total: clamp32(after.Total, -heldBonusLimit, heldBonusLimit), LuckyAnswerKey: after.LuckyAnswerKey, LuckyTo: after.LuckyTo,
	})
	return err //nolint:wrapcheck // the question wraps it
}

func (st inspirationHoldStore) answer(ctx context.Context, h heldRoll, key string) error {
	return st.q.AnswerInspirationHold(ctx, playdb.AnswerInspirationHoldParams{ID: h.ID, AnswerKey: &key}) //nolint:wrapcheck // the gate wraps it
}

func (st inspirationHoldStore) forget(ctx context.Context, h heldRoll) error {
	return st.q.DeleteInspirationHold(ctx, h.ID) //nolint:wrapcheck // the gate wraps it
}

// asked does nothing: a question in inspiration_holds does not move the combat's revision today
// (TestAnOutsideHoldInACombatDoesNotTouchTheEncounterToday); E4-7 changes it for every question.
func (inspirationHoldStore) asked(context.Context, gateEnv) error { return nil }
