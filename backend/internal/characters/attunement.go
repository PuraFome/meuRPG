package characters

import (
	"context"
	"errors"
	"log/slog"
	"slices"
	"strconv"
	"strings"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/proto"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	"github.com/PuraFome/meuRPG/backend/internal/authz"
	"github.com/PuraFome/meuRPG/backend/internal/characters/charactersdb"
	"github.com/PuraFome/meuRPG/backend/internal/platform/db"
	"github.com/PuraFome/meuRPG/backend/internal/platform/logging"
	"github.com/PuraFome/meuRPG/backend/internal/platform/uid"
	"github.com/PuraFome/meuRPG/backend/internal/rules"
)

// Attunement (#561, SRD 5.1): a magic item that asks for it works only while the character is attuned, at most
// three at once. The sheet's save never changes `attuned`; SetAttunement does, inside the window a rest opens.

// attunementFactsOf is what a restriction reads from the sheet besides the build.
func attunementFactsOf(f *charactersv1.FullSheet) rules.AttunementFacts {
	if f.GetAlignment() == charactersv1.Alignment_ALIGNMENT_UNSPECIFIED {
		return rules.AttunementFacts{}
	}
	name := strings.TrimPrefix(f.GetAlignment().String(), "ALIGNMENT_")
	return rules.AttunementFacts{Alignment: strings.ToLower(name)}
}

// bonusID is the id of a +N piece's entry: its key is unique on the sheet.
func bonusID(key string) string { return "bonus:" + key }

// keepAttunement carries over what the server keeps of the items in a saved sheet : the `attuned` of an entry already on the stored sheet,
// false for a new one, the entry ids, `requires_attunement` of a piece from its catalog item, and none of the
// fields the server fills on a read. stored is nil for a new character.
func keepAttunement(content *rules.Content, stored, saved *charactersv1.FullSheet) {
	was := map[string]*charactersv1.ItemBonus{}
	for _, ib := range stored.GetItemBonuses() {
		was[ib.GetKey()] = ib
	}
	for _, ib := range saved.GetItemBonuses() {
		old := was[ib.GetKey()]
		ib.Id = bonusID(ib.GetKey())
		ib.Attuned = old != nil && old.GetItemKey() == ib.GetItemKey() && old.GetAttuned()
		// The catalog decides only when the piece names an item; without one the flag is the stored one (or the
		// save's, for a new piece), as it was before pieces could name an item.
		if ib.GetItemKey() != "" {
			ib.RequiresAttunement = content.AttunementAsked(ib.GetItemKey())
		}
		ib.RestrictionPt, ib.Restriction, ib.SameItemAttuned, ib.ItemNamePt = "", charactersv1.AttunementRestriction_ATTUNEMENT_RESTRICTION_UNSPECIFIED, false, ""
	}
	keepMagicItems(stored, saved)
}

// keepMagicItems keeps the ids and the `attuned` of the entries a saved sheet shares with the stored one, and gives an
// id to every other entry: a client that repeats an id or sends one the sheet never had gets a new entry.
func keepMagicItems(stored, saved *charactersv1.FullSheet) {
	was := map[string]*charactersv1.SheetMagicItem{}
	used := map[string]bool{}
	next := 0
	for _, it := range stored.GetMagicItems() {
		was[it.GetId()] = it
		used[it.GetId()] = true
		next = max(next, entryNumber(it.GetId()))
	}
	taken := map[string]bool{}
	for _, it := range saved.GetMagicItems() {
		old := was[it.GetId()]
		if old == nil || old.GetKey() != it.GetKey() || taken[it.GetId()] {
			next++
			for used[entryID(next)] {
				next++
			}
			used[entryID(next)] = true
			it.Id, it.Attuned = entryID(next), false
		} else {
			it.Attuned = old.GetAttuned()
		}
		taken[it.GetId()] = true
		it.RequiresAttunement, it.RestrictionPt, it.Restriction, it.SameItemAttuned, it.NamePt = false, "", charactersv1.AttunementRestriction_ATTUNEMENT_RESTRICTION_UNSPECIFIED, false, ""
		it.ChargesMax, it.ChargesLeft, it.Destroyed = 0, 0, false
	}
}

func entryID(n int) string { return "m" + strconv.Itoa(n) }

// entryNumber is the n of an id "m<n>", or 0.
func entryNumber(id string) int {
	n, err := strconv.Atoi(strings.TrimPrefix(id, "m"))
	if err != nil || !strings.HasPrefix(id, "m") {
		return 0
	}
	return n
}

// entry is one attunable thing of the sheet: a +N piece of gear or a catalog magic item. Both carry the same
// fields, so one view reads and sets them.
type entry struct {
	bonus *charactersv1.ItemBonus
	magic *charactersv1.SheetMagicItem
}

func (e entry) id() string {
	if e.bonus != nil {
		return e.bonus.GetId()
	}
	return e.magic.GetId()
}

// itemKey is the catalog item the entry is, or "" for a +N piece that names none.
func (e entry) itemKey() string {
	if e.bonus != nil {
		return e.bonus.GetItemKey()
	}
	return e.magic.GetKey()
}

func (e entry) attuned() bool {
	if e.bonus != nil {
		return e.bonus.GetAttuned()
	}
	return e.magic.GetAttuned()
}

func (e entry) set(attuned bool) {
	if e.bonus != nil {
		e.bonus.Attuned = attuned
	} else {
		e.magic.Attuned = attuned
	}
}

// entriesOf lists the entries of a sheet: the +N pieces, then the magic items.
func entriesOf(f *charactersv1.FullSheet) []entry {
	var out []entry
	for _, ib := range f.GetItemBonuses() {
		out = append(out, entry{bonus: ib})
	}
	for _, it := range f.GetMagicItems() {
		out = append(out, entry{magic: it})
	}
	return out
}

// copyAttuned says another entry of the same catalog item than e is attuned.
func copyAttuned(entries []entry, e entry) bool {
	if e.itemKey() == "" {
		return false
	}
	return slices.ContainsFunc(entries, func(o entry) bool {
		return o.id() != e.id() && o.itemKey() == e.itemKey() && o.attuned()
	})
}

// fillMagicItems fills the fields of the entries that the server says on every read: whether the item asks for
// attunement, what it asks in Portuguese, whether the character meets it, whether another copy is attuned, and the
// catalog name.
func fillMagicItems(content *rules.Content, f *charactersv1.FullSheet, build rules.Build, d rules.Derived) {
	facts := attunementFactsOf(f)
	setBonusIDs(f)
	entries := entriesOf(f)
	for _, e := range entries {
		key := e.itemKey()
		if key == "" {
			continue
		}
		requires, by, met, ok := content.AttunementOf(key, build, d, facts)
		if !ok {
			continue
		}
		state := restrictionOf(by, met)
		same := !e.attuned() && copyAttuned(entries, e)
		name := content.MagicItemNamePT(key)
		if e.bonus != nil {
			e.bonus.RestrictionPt, e.bonus.Restriction, e.bonus.SameItemAttuned, e.bonus.ItemNamePt = by, state, same, name
			continue
		}
		e.magic.RequiresAttunement, e.magic.RestrictionPt, e.magic.Restriction, e.magic.SameItemAttuned, e.magic.NamePt = requires, by, state, same, name
	}
}

func restrictionOf(by string, met rules.Restriction) charactersv1.AttunementRestriction {
	switch {
	case by == "":
		return charactersv1.AttunementRestriction_ATTUNEMENT_RESTRICTION_UNSPECIFIED
	case met == rules.RestrictionMet:
		return charactersv1.AttunementRestriction_ATTUNEMENT_RESTRICTION_MET
	case met == rules.RestrictionNotMet:
		return charactersv1.AttunementRestriction_ATTUNEMENT_RESTRICTION_NOT_MET
	}
	return charactersv1.AttunementRestriction_ATTUNEMENT_RESTRICTION_UNKNOWN
}

// attunementRequest is a SetAttunement request whose shape is checked.
type attunementRequest struct {
	id, entryID string
	attuned     bool
	revision    int32
}

// SetAttunement implements charactersv1connect.CharacterServiceHandler.
func (s *Service) SetAttunement(
	ctx context.Context,
	req *connect.Request[charactersv1.SetAttunementRequest],
) (*connect.Response[charactersv1.SetAttunementResponse], error) {
	m, err := authz.RequireCampaignMember(ctx, req.Msg.GetCampaignId())
	if err != nil {
		return nil, err
	}
	id, ok := uid.Canonical(req.Msg.GetCharacterId())
	if !ok {
		return nil, errCharacterNotFound()
	}
	if req.Msg.GetEntryId() == "" {
		return nil, invalidArgument(fieldErr("entry_id", "is required"))
	}
	in := attunementRequest{id: id, entryID: req.Msg.GetEntryId(), attuned: req.Msg.GetAttuned(), revision: req.Msg.GetRevision()}
	content, err := s.contentFor(ctx, nil, m.CampaignID)
	if err != nil {
		return nil, s.dbError(ctx, "read rules content", err)
	}
	var row charactersdb.Character
	var changed bool
	err = db.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		var err error
		row, changed, err = s.attuneIn(ctx, tx, content, m, in)
		return err
	})
	if err != nil {
		return nil, s.dbError(ctx, "attune a character", err)
	}
	if changed {
		logging.Event(ctx, s.logger, "character.attuned", slog.String("character_id", id), slog.String("entry_id", in.entryID),
			slog.Bool("attuned", in.attuned), slog.Bool("by_master", isMaster(m)), slog.String("by_user_id", m.UserID))
	}
	c, err := s.character(ctx, content, row, m)
	if err != nil {
		return nil, s.dbError(ctx, "read an attuned character", err)
	}
	return connect.NewResponse(&charactersv1.SetAttunementResponse{Character: c}), nil
}

// attuneIn is the transaction of SetAttunement: the character locked, the entry found on its sheet, the window read and
// spent with the write, and the refusals in the order the caller is answered.
func (s *Service) attuneIn(ctx context.Context, tx pgx.Tx, content *rules.Content, m authz.Membership, in attunementRequest) (charactersdb.Character, bool, error) {
	q := s.queries.WithTx(tx)
	current, err := visibleForUpdate(ctx, q, m, in.id)
	if err != nil {
		return current, false, err
	}
	if in.revision != 0 && in.revision != current.Revision {
		return current, false, errStaleRevision()
	}
	stored, err := loadSheet(current.ID, current.Sheet)
	if err != nil {
		return current, false, err
	}
	full := stored.GetFull()
	if full == nil {
		return current, false, invalidArgument(fieldErr("character_id", "is a character with a basic sheet, which has no items to attune"))
	}
	next := proto.CloneOf(full)
	setBonusIDs(next)
	entries := entriesOf(next)
	i := slices.IndexFunc(entries, func(e entry) bool { return e.id() == in.entryID })
	if i < 0 {
		return current, false, invalidArgument(fieldErr("entry_id", "is not an entry of the sheet"))
	}
	target := entries[i]
	build := buildOf(next)
	requires, _, met, _ := content.AttunementOf(target.itemKey(), build, rules.Derive(build, content), attunementFactsOf(next))
	if target.bonus != nil {
		requires = target.bonus.GetRequiresAttunement()
	}
	if !requires && (in.attuned || !target.attuned()) {
		return current, false, invalidArgument(fieldErr("entry_id", "is an item that does not ask for attunement"))
	}
	if target.attuned() == in.attuned {
		return current, false, nil // already as asked: nothing changes, and no window is spent
	}
	if err := s.checkAttunement(ctx, q, m, current, in, checkFacts{met: met, attuned: rules.AttunedCount(build), copyAttuned: copyAttuned(entries, target)}); err != nil {
		return current, false, err
	}
	target.set(in.attuned)
	row, err := s.writeAttunement(ctx, q, m, current, next)
	return row, err == nil, err
}

// setBonusIDs gives the +N pieces of a stored sheet their ids (the sheet stores them from the save that made them, but
// a sheet written before the ids has none).
func setBonusIDs(f *charactersv1.FullSheet) {
	for _, ib := range f.GetItemBonuses() {
		ib.Id = bonusID(ib.GetKey())
	}
}

// writeAttunement stores the sheet and, for the owner, spends the window in the same transaction.
func (s *Service) writeAttunement(ctx context.Context, q *charactersdb.Queries, m authz.Membership, current charactersdb.Character, next *charactersv1.FullSheet) (charactersdb.Character, error) {
	doc, err := storeJSON.Marshal(&charactersv1.CharacterSheet{Content: &charactersv1.CharacterSheet_Full{Full: next}})
	if err != nil {
		return current, wrap("encode a sheet", err)
	}
	row, err := q.UpdateCharacterSheet(ctx, charactersdb.UpdateCharacterSheetParams{
		CampaignID: m.CampaignID, ID: current.ID, Revision: current.Revision, Name: current.Name, Sheet: doc, Now: s.now(),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return current, errStaleRevision()
	}
	if err != nil {
		return current, wrap("update sheet", err)
	}
	if !isMaster(m) {
		if err := q.SetAttunementOpen(ctx, charactersdb.SetAttunementOpenParams{CharacterID: current.ID, Open: false, Now: s.now()}); err != nil {
			return current, wrap("spend the window to attune", err)
		}
	}
	return row, nil
}

// checkFacts is what the refusals of SetAttunement read from the sheet.
type checkFacts struct {
	met         rules.Restriction
	attuned     int
	copyAttuned bool
}

// checkAttunement refuses what the owner and the master may not do now, in this order: the owner needs the window (a
// dead character's owner has none), a fourth item is refused to both, a second copy of an item is refused to both,
// and a restriction the character does not meet (or the sheet cannot say) is the master's alone to override. Ending
// an attunement is never refused for the limit, the copy or the restriction.
func (s *Service) checkAttunement(ctx context.Context, q *charactersdb.Queries, m authz.Membership, current charactersdb.Character, in attunementRequest, f checkFacts) error {
	if !isMaster(m) {
		open, err := q.GetAttunementOpen(ctx, current.ID)
		if err != nil {
			return wrap("read the window to attune", err)
		}
		if !open || current.Status == statusDead {
			return errBlocked(charactersv1.CharacterBlockedReason_CHARACTER_BLOCKED_REASON_ATTUNEMENT_NEEDS_REST, current.ID)
		}
	}
	if !in.attuned {
		return nil
	}
	if f.attuned >= rules.MaxAttunements {
		return errBlocked(charactersv1.CharacterBlockedReason_CHARACTER_BLOCKED_REASON_ATTUNEMENT_LIMIT, current.ID)
	}
	if f.copyAttuned {
		return errBlocked(charactersv1.CharacterBlockedReason_CHARACTER_BLOCKED_REASON_ATTUNEMENT_SAME_ITEM, current.ID)
	}
	if f.met != rules.RestrictionMet && !isMaster(m) {
		return errBlocked(charactersv1.CharacterBlockedReason_CHARACTER_BLOCKED_REASON_ATTUNEMENT_RESTRICTED, current.ID)
	}
	return nil
}
