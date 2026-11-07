package characters

import (
	"context"
	"errors"
	"log/slog"
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

// keepAttunement carries over what the server keeps of the items in a saved sheet: the `attuned` of a key already on
// the stored sheet, false for a new entry, and none of the fields the server fills on a read.
func keepAttunement(stored, saved *charactersv1.FullSheet) {
	was := map[string]bool{}
	for _, ib := range stored.GetItemBonuses() {
		was[ib.GetKey()] = ib.GetAttuned()
	}
	for _, it := range stored.GetMagicItems() {
		was[it.GetKey()] = it.GetAttuned()
	}
	for _, ib := range saved.GetItemBonuses() {
		ib.Attuned = was[ib.GetKey()]
	}
	for _, it := range saved.GetMagicItems() {
		it.Attuned = was[it.GetKey()]
		it.RequiresAttunement, it.AttunementByPt, it.RestrictionMet = false, "", false
	}
}

// fillMagicItems fills the fields of the magic items that the server says on every read: whether the item asks for
// attunement, its restriction in Portuguese and whether the character meets it.
func fillMagicItems(content *rules.Content, f *charactersv1.FullSheet, build rules.Build, d rules.Derived) {
	facts := attunementFactsOf(f)
	for _, it := range f.GetMagicItems() {
		requires, by, met, ok := content.AttunementOf(it.GetKey(), build, d, facts)
		if !ok {
			continue
		}
		it.RequiresAttunement, it.AttunementByPt, it.RestrictionMet = requires, by, met == rules.RestrictionMet
	}
}

// attunementRequest is a SetAttunement request whose shape is checked.
type attunementRequest struct {
	id, key  string
	attuned  bool
	revision int32
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
	if req.Msg.GetKey() == "" {
		return nil, invalidArgument(fieldErr("key", "is required"))
	}
	in := attunementRequest{id: id, key: req.Msg.GetKey(), attuned: req.Msg.GetAttuned(), revision: req.Msg.GetRevision()}
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
		logging.Event(ctx, s.logger, "character.attuned", slog.String("character_id", id), slog.String("key", in.key),
			slog.Bool("attuned", in.attuned), slog.Bool("by_master", isMaster(m)), slog.String("by_user_id", m.UserID))
	}
	c, err := s.character(ctx, content, row, m)
	if err != nil {
		return nil, s.dbError(ctx, "read an attuned character", err)
	}
	return connect.NewResponse(&charactersv1.SetAttunementResponse{Character: c}), nil
}

// attuneIn is the transaction of SetAttunement: the character locked, the item found on its sheet, the window read and
// spent with the write, and the refusals in the order the caller is answered.
func (s *Service) attuneIn(ctx context.Context, tx pgx.Tx, content *rules.Content, m authz.Membership, in attunementRequest) (charactersdb.Character, bool, error) {
	q := s.queries.WithTx(tx)
	current, err := visibleForUpdate(ctx, q, m, in.id)
	if err != nil {
		return current, false, err
	}
	master := isMaster(m)
	if current.Status == statusDead && !master {
		return current, false, errBlocked(charactersv1.CharacterBlockedReason_CHARACTER_BLOCKED_REASON_CHARACTER_DEAD, current.ID)
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
	target, ok := attunableOf(next, in.key)
	build := buildOf(next)
	var met rules.Restriction
	if target.catalog != nil {
		target.requires, _, met, _ = content.AttunementOf(in.key, build, rules.Derive(build, content), attunementFactsOf(next))
	}
	if !ok || !target.requires {
		return current, false, invalidArgument(fieldErr("key", "is not an item of the sheet that asks for attunement"))
	}
	if target.attuned() == in.attuned {
		return current, false, nil // already as asked: nothing changes, and no window is spent
	}
	if err := s.checkAttunement(ctx, q, m, current, in, met, rules.AttunedCount(build)); err != nil {
		return current, false, err
	}
	target.set(in.attuned)
	row, err := s.writeAttunement(ctx, q, m, current, next)
	return row, err == nil, err
}

// attunable is the entry of the sheet that a key names: a +N piece of gear, a catalog magic item, or both.
type attunable struct {
	bonus    *charactersv1.ItemBonus
	catalog  *charactersv1.SheetMagicItem
	requires bool // for a +N piece; a catalog item's comes from the catalog
}

func attunableOf(full *charactersv1.FullSheet, key string) (attunable, bool) {
	var a attunable
	found := false
	for _, ib := range full.GetItemBonuses() {
		if ib.GetKey() == key {
			a.bonus, a.requires, found = ib, ib.GetRequiresAttunement(), true
		}
	}
	for _, it := range full.GetMagicItems() {
		if it.GetKey() == key {
			a.catalog, found = it, true
		}
	}
	return a, found
}

// attuned says whether the entry is attuned already (a +N piece decides when the key is both).
func (a attunable) attuned() bool {
	if a.bonus != nil {
		return a.bonus.GetAttuned()
	}
	return a.catalog.GetAttuned()
}

func (a attunable) set(attuned bool) {
	if a.bonus != nil {
		a.bonus.Attuned = attuned
	} else {
		a.catalog.Attuned = attuned
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

// checkAttunement refuses what the owner and the master may not do now: the owner needs the window, a restriction the
// character does not meet (or the sheet cannot say) is the master's alone to override, and a fourth item is refused to
// both. Ending an attunement is never refused for the limit or the restriction.
func (s *Service) checkAttunement(ctx context.Context, q *charactersdb.Queries, m authz.Membership, current charactersdb.Character, in attunementRequest, met rules.Restriction, attuned int) error {
	if !isMaster(m) {
		open, err := q.GetAttunementOpen(ctx, current.ID)
		if err != nil {
			return wrap("read the window to attune", err)
		}
		if !open {
			return errBlocked(charactersv1.CharacterBlockedReason_CHARACTER_BLOCKED_REASON_ATTUNEMENT_NEEDS_REST, current.ID)
		}
	}
	if !in.attuned {
		return nil
	}
	if met != rules.RestrictionMet && !isMaster(m) {
		return errBlocked(charactersv1.CharacterBlockedReason_CHARACTER_BLOCKED_REASON_ATTUNEMENT_RESTRICTED, current.ID)
	}
	if attuned >= rules.MaxAttunements {
		return errBlocked(charactersv1.CharacterBlockedReason_CHARACTER_BLOCKED_REASON_ATTUNEMENT_LIMIT, current.ID)
	}
	return nil
}
