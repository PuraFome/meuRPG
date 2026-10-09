package characters

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"time"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/proto"

	pkgv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/campaignpackage/v1"
	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
	"github.com/PuraFome/meuRPG/backend/internal/authz"
	"github.com/PuraFome/meuRPG/backend/internal/campaignpackage"
	"github.com/PuraFome/meuRPG/backend/internal/characters/charactersdb"
	"github.com/PuraFome/meuRPG/backend/internal/platform/names"
	"github.com/PuraFome/meuRPG/backend/internal/rules"
)

// The characters module's share of the campaign package (MR-050): the table
// content (the entries and the options switched off), the NPCs and creatures
// with the master's notes about them, and the players' characters. Everything
// goes through the checks the calls that make these things by hand use
// (prepare and the overlay for the content, checkNewSheet, checkStory and
// names.Clean for the characters), against the table content the package
// itself brings, so a sheet that needs a class of the table is checked with
// that class.
//
// A character comes without what happened to it in play: no vitals, no
// level-up history, no creatures, no lock, no story release, no player.

// PackagePart returns the characters' part of the package.
func (s *Service) PackagePart() campaignpackage.Part { return &packagePart{s: s} }

type packagePart struct{ s *Service }

const (
	stashContent    = "characters.content"
	stashCharacters = "characters.characters"
	optionsEntry    = "content/options.json"
	nsCharacter     = "character"
)

// orderStep is the time between two things made from one package, so the
// lists keep the package's order.
const orderStep = time.Millisecond

// --- export -----------------------------------------------------------------

// Export implements campaignpackage.Part.
func (p *packagePart) Export(ctx context.Context, tx pgx.Tx, campaignID string, snap *campaignpackage.Snapshot) error {
	q := p.s.queries.WithTx(tx)
	rows, err := q.ListCampaignContent(ctx, campaignID)
	if err != nil {
		return fmt.Errorf("list the table content: %w", err)
	}
	for _, r := range rows {
		e, err := decodeRow(r)
		if err != nil {
			return err
		}
		name := campaignpackage.EntryName("content", snap.Next("content"), ".json")
		if err := snap.AddMessage(pkgv1.PackageEntryKind_PACKAGE_ENTRY_KIND_CONTENT, name, &pkgv1.PackageContent{Entry: entryToProto(e, 0)}); err != nil {
			return err
		}
	}
	off, err := q.ListContentOff(ctx, campaignID)
	if err != nil {
		return fmt.Errorf("list the options switched off: %w", err)
	}
	if len(off) > 0 {
		if err := snap.AddMessage(pkgv1.PackageEntryKind_PACKAGE_ENTRY_KIND_CONTENT_OPTIONS, optionsEntry, &pkgv1.PackageContentOptions{Disabled: off}); err != nil {
			return err
		}
	}
	return p.exportCharacters(ctx, q, campaignID, snap)
}

func (p *packagePart) exportCharacters(ctx context.Context, q *charactersdb.Queries, campaignID string, snap *campaignpackage.Snapshot) error {
	chars, err := q.ListCharactersForPackage(ctx, campaignID)
	if err != nil {
		return fmt.Errorf("list the characters: %w", err)
	}
	noteRows, err := q.ListMasterNotesOfCampaign(ctx, campaignID)
	if err != nil {
		return fmt.Errorf("list the master's notes: %w", err)
	}
	notes := map[string]string{}
	for _, n := range noteRows {
		notes[n.CharacterID] = n.Notes
	}
	for _, c := range chars {
		sheet, err := loadSheet(c.ID, c.Sheet)
		if err != nil {
			return err
		}
		story, err := loadStory(c.ID, c.Story)
		if err != nil {
			return err
		}
		switch {
		case c.Kind == kindPlayer && c.Status != statusActive:
			continue // a pending request or a dead character is not part of the table to come
		case c.Kind != kindPlayer && sheet.GetBasic().GetCombatOnly():
			continue // the app's own copy of a creature put in combat: it is made again on first use
		}
		if c.Kind == kindPlayer {
			name := campaignpackage.EntryName("characters", snap.Next("characters"), ".json")
			out := &pkgv1.PackageCharacter{Id: c.ID, Name: c.Name, Sheet: sheet, Story: story, MasterNotes: notes[c.ID]}
			if err := snap.AddMessage(pkgv1.PackageEntryKind_PACKAGE_ENTRY_KIND_CHARACTER, name, out); err != nil {
				return err
			}
			continue
		}
		name := campaignpackage.EntryName("npcs", snap.Next("npcs"), ".json")
		out := &pkgv1.PackageNpc{Id: c.ID, Name: c.Name, Sheet: sheet, Story: story, MasterNotes: notes[c.ID]}
		if err := snap.AddMessage(pkgv1.PackageEntryKind_PACKAGE_ENTRY_KIND_NPC, name, out); err != nil {
			return err
		}
	}
	return nil
}

// --- stage ------------------------------------------------------------------

// contentKeyShape is the key of a table entry: the kind, a colon, a slug and
// "@mesa". The slug is what the server makes from the name.
var contentKeyShape = regexp.MustCompile(`^(class|subclass|race|subrace|background|spell):([a-z0-9]+(?:-[a-z0-9]+)*)@mesa$`)

type stagedContent struct {
	entries []entryRow
	off     []string
	content *rules.Content
	// kinds the entries are of, by key, for the problems.
	names map[string]string
}

// contentReason is the problem a violation of the table content becomes.
func contentReason(reason string) pkgv1.PackageProblemReason {
	switch reason {
	case rules.ReasonReference, rules.ReasonKey:
		return pkgv1.PackageProblemReason_PACKAGE_PROBLEM_REASON_UNKNOWN_CONTENT
	case rules.ReasonLimit, reasonSizeLimit:
		return pkgv1.PackageProblemReason_PACKAGE_PROBLEM_REASON_LIMIT
	case reasonDuplicateName, rules.ReasonDuplicateKey:
		return pkgv1.PackageProblemReason_PACKAGE_PROBLEM_REASON_DUPLICATE
	default:
		return pkgv1.PackageProblemReason_PACKAGE_PROBLEM_REASON_INVALID
	}
}

// Stage implements campaignpackage.Part.
func (p *packagePart) Stage(_ context.Context, in *campaignpackage.Import) error {
	sc := p.stageContent(in)
	if sc == nil {
		// The content has problems: the characters cannot be judged against it.
		return nil
	}
	in.Set(stashContent, sc)
	return p.stageCharacters(in, sc)
}

// stageContent reads, checks and compiles the table content. A nil answer
// means a problem was recorded.
func (p *packagePart) stageContent(in *campaignpackage.Import) *stagedContent {
	s := p.s
	contentKind := pkgv1.PackageProblemKind_PACKAGE_PROBLEM_KIND_CONTENT
	files := in.Pkg.Entries(pkgv1.PackageEntryKind_PACKAGE_ENTRY_KIND_CONTENT)
	sc := &stagedContent{names: map[string]string{}}
	if len(files) > rules.MaxOverlayEntries {
		in.Problem(contentKind, "", pkgv1.PackageProblemReason_PACKAGE_PROBLEM_REASON_LIMIT, rules.MaxOverlayEntries)
		return nil
	}
	now := s.now()
	good := true
	seen := map[string]bool{}
	for i, f := range files {
		var pc pkgv1.PackageContent
		if !in.Read(f.GetPath(), &pc) {
			good = false
			continue
		}
		e, ok := p.stageEntry(in, pc.GetEntry(), now.Add(time.Duration(i)*orderStep), seen)
		if !ok {
			good = false
			continue
		}
		sc.entries = append(sc.entries, e)
		sc.names[e.row.ContentKey] = e.row.NamePt
	}
	opts := in.Pkg.Entries(pkgv1.PackageEntryKind_PACKAGE_ENTRY_KIND_CONTENT_OPTIONS)
	var options pkgv1.PackageContentOptions
	if len(opts) > 1 || (len(opts) == 1 && opts[0].GetPath() != optionsEntry) {
		in.Problem(contentKind, "", pkgv1.PackageProblemReason_PACKAGE_PROBLEM_REASON_UNEXPECTED_ENTRY, 0)
		good = false
	} else if len(opts) == 1 && !in.Read(optionsEntry, &options) {
		good = false
	}
	if !good {
		return nil
	}
	// The whole content, compiled at once: an entry can depend on another (a
	// subclass on its class, a spell list a class reuses).
	overlay, idx := overlayFromEntries(sc.entries, 1)
	for _, e := range sc.entries {
		overlay.Strict = append(overlay.Strict, e.row.ContentKey)
	}
	content, err := s.srd.With(overlay)
	if err != nil {
		for _, v := range violationsOf(err, idx, "") {
			name := sc.names[v.GetKey()]
			in.Problem(contentKind, name, contentReason(v.GetReason()), 0)
		}
		return nil
	}
	sc.content = content
	for _, key := range options.GetDisabled() {
		if !content.Switchable(key) || slices.Contains(sc.off, key) {
			in.Problem(contentKind, key, pkgv1.PackageProblemReason_PACKAGE_PROBLEM_REASON_UNKNOWN_CONTENT, 0)
			good = false
			continue
		}
		sc.off = append(sc.off, key)
	}
	if !good {
		return nil
	}
	in.Counts.ContentEntries += int32(len(sc.entries)) //nolint:gosec // G115: at most 300
	return sc
}

// stageEntry checks one entry of the table content the way a write of it does
// (shape, feature count and keys, size), and returns it as it will be stored.
func (p *packagePart) stageEntry(in *campaignpackage.Import, te *rulesv1.TableEntry, at time.Time, seen map[string]bool) (entryRow, bool) {
	contentKind := pkgv1.PackageProblemKind_PACKAGE_PROBLEM_KIND_CONTENT
	name := te.GetNamePt()
	bad := func(reason pkgv1.PackageProblemReason) (entryRow, bool) {
		in.Problem(contentKind, name, reason, 0)
		return entryRow{}, false
	}
	body, kind := bodyOf(te)
	m := contentKeyShape.FindStringSubmatch(te.GetKey())
	if body == nil || kind != te.GetKind() || m == nil || m[1] != kindPrefix(kind) || len(m[2]) > rules.MaxSlugLength || body.GetNamePt() != te.GetNamePt() {
		return bad(pkgv1.PackageProblemReason_PACKAGE_PROBLEM_REASON_INVALID)
	}
	if seen[te.GetKey()] {
		return bad(pkgv1.PackageProblemReason_PACKAGE_PROBLEM_REASON_DUPLICATE)
	}
	seen[te.GetKey()] = true
	body = proto.Clone(body).(tableBody) //nolint:forcetypeassert // Clone returns the same type
	if v := firstViolation(checkShape(body), checkFeatureCount(kind, body)); v != nil {
		return bad(contentReason(v.GetReason()))
	}
	// Every feature key in the package was made by a server: they are the entry's own.
	if v := featureKeys(te.GetKey(), body, body); len(v) > 0 {
		return bad(contentReason(v[0].GetReason()))
	}
	data, err := storedData(body)
	if err != nil {
		return bad(pkgv1.PackageProblemReason_PACKAGE_PROBLEM_REASON_INVALID)
	}
	if len(data) > MaxTableEntryBytes {
		return bad(pkgv1.PackageProblemReason_PACKAGE_PROBLEM_REASON_LIMIT)
	}
	row := charactersdb.CampaignContent{
		CampaignID: in.CampaignID, ContentKey: te.GetKey(), Kind: kindPrefix(kind), NamePt: te.GetNamePt(), Data: data,
		Revision: 1, CreatedAt: at, UpdatedAt: at,
	}
	if te.GetArchived() {
		row.ArchivedAt = &at
	}
	return entryRow{row: row, kind: kind, body: body}, true
}

// firstViolation is the first violation of the lists, or nil.
func firstViolation(lists ...[]*rulesv1.TableContentViolation) *rulesv1.TableContentViolation {
	for _, l := range lists {
		if len(l) > 0 {
			return l[0]
		}
	}
	return nil
}

type stagedCharacter struct {
	id, name  string
	kind      string
	sheet     []byte
	story     []byte
	notes     string
	createdAt time.Time
}

type stagedCharacters struct{ npcs, players []*stagedCharacter }

// stageCharacters checks the NPCs and the players' characters against the
// table content of the package.
func (p *packagePart) stageCharacters(in *campaignpackage.Import, sc *stagedContent) error {
	s := p.s
	content := sc.content
	if content == nil {
		content = s.srd // a package without table content: the SRD alone
	}
	npcFiles := in.Pkg.Entries(pkgv1.PackageEntryKind_PACKAGE_ENTRY_KIND_NPC)
	charFiles := in.Pkg.Entries(pkgv1.PackageEntryKind_PACKAGE_ENTRY_KIND_CHARACTER)
	out := &stagedCharacters{}
	in.Set(stashCharacters, out)
	if len(npcFiles)+len(charFiles) > s.maxCharacters {
		in.Problem(pkgv1.PackageProblemKind_PACKAGE_PROBLEM_KIND_NPC, "", pkgv1.PackageProblemReason_PACKAGE_PROBLEM_REASON_LIMIT, int64(s.maxCharacters))
		return nil
	}
	master := authz.Membership{CampaignID: in.CampaignID, UserID: in.UserID, Role: authz.RoleMaster}
	base := s.now()
	for i, f := range npcFiles {
		var n pkgv1.PackageNpc
		if !in.Read(f.GetPath(), &n) {
			continue
		}
		c := p.stageOne(in, content, master, stageInput{
			kind: pkgv1.PackageProblemKind_PACKAGE_PROBLEM_KIND_NPC, id: n.GetId(), name: n.GetName(), sheet: n.GetSheet(), story: n.GetStory(), notes: n.GetMasterNotes(), player: false, npcKind: n.GetKind(),
		}, base.Add(time.Duration(i)*orderStep))
		if c != nil {
			out.npcs = append(out.npcs, c)
			in.Counts.Npcs++
		}
	}
	for i, f := range charFiles {
		var ch pkgv1.PackageCharacter
		if !in.Read(f.GetPath(), &ch) {
			continue
		}
		c := p.stageOne(in, content, master, stageInput{
			kind: pkgv1.PackageProblemKind_PACKAGE_PROBLEM_KIND_CHARACTER, id: ch.GetId(), name: ch.GetName(), sheet: ch.GetSheet(), story: ch.GetStory(), notes: ch.GetMasterNotes(), player: true,
		}, base.Add(time.Duration(len(npcFiles)+i)*orderStep))
		if c != nil {
			out.players = append(out.players, c)
			in.Facts.Players[ch.GetId()] = true
			in.Counts.Characters++
		}
	}
	return nil
}

type stageInput struct {
	kind            pkgv1.PackageProblemKind
	id, name, notes string
	sheet           *charactersv1.CharacterSheet
	story           *charactersv1.CharacterStory
	player          bool
	npcKind         charactersv1.CharacterKind
}

// stageOne checks one character or NPC and returns it ready to insert.
func (p *packagePart) stageOne(in *campaignpackage.Import, content *rules.Content, master authz.Membership, c stageInput, at time.Time) *stagedCharacter {
	s := p.s
	bad := func(name string, reason pkgv1.PackageProblemReason) *stagedCharacter {
		in.Problem(c.kind, name, reason, 0)
		return nil
	}
	name, err := names.Clean(c.name, MaxNameLength)
	if err != nil {
		return bad("", pkgv1.PackageProblemReason_PACKAGE_PROBLEM_REASON_INVALID)
	}
	id, ok := in.IDs.Define(nsCharacter, c.id)
	if !ok || c.sheet == nil {
		return bad(name, pkgv1.PackageProblemReason_PACKAGE_PROBLEM_REASON_INVALID)
	}
	notes, err := names.CleanText(c.notes, MaxMasterNotesLength)
	if err != nil {
		return bad(name, pkgv1.PackageProblemReason_PACKAGE_PROBLEM_REASON_INVALID)
	}
	kind := kindPlayer
	sheet := proto.Clone(c.sheet).(*charactersv1.CharacterSheet) //nolint:forcetypeassert // Clone returns the same type
	var creature *charactersv1.BasicSheet
	if !c.player {
		var known bool
		if kind, known = kindToDB[c.npcKind]; !known || kind == kindPlayer {
			return bad(name, pkgv1.PackageProblemReason_PACKAGE_PROBLEM_REASON_INVALID)
		}
		if sheet.GetBasic() != nil {
			creature = &charactersv1.BasicSheet{MonsterKey: sheet.GetBasic().GetMonsterKey(), AbilityScores: sheet.GetBasic().GetAbilityScores()}
			if sheet.GetBasic().GetCombatOnly() {
				return bad(name, pkgv1.PackageProblemReason_PACKAGE_PROBLEM_REASON_INVALID)
			}
		}
		// The portrait is a gallery image of the package: it gets the new id.
		if old := portraitOf(sheet); old != "" {
			setPortrait(sheet, in.IDs.Ref("image", old, c.kind, name))
		}
	}
	checked, err := checkNewSheet(content, master, kind, sheet)
	if err != nil {
		return bad(name, sheetReason(err))
	}
	if creature != nil && creature.GetMonsterKey() != "" {
		// The link to the creature and its scores are the server's: only the NPC made
		// from a creature has them, and the package keeps them as they were.
		if _, known := s.srd.CreatureByKey(creature.GetMonsterKey()); !known || !validAbilityScores(creature.GetAbilityScores()) {
			return bad(name, pkgv1.PackageProblemReason_PACKAGE_PROBLEM_REASON_UNKNOWN_CONTENT)
		}
		checked.GetBasic().MonsterKey, checked.GetBasic().AbilityScores = creature.GetMonsterKey(), creature.GetAbilityScores()
	}
	story, err := checkStory(c.story)
	if err != nil {
		return bad(name, pkgv1.PackageProblemReason_PACKAGE_PROBLEM_REASON_INVALID)
	}
	out := &stagedCharacter{id: id, name: name, kind: kind, notes: notes, createdAt: at}
	if out.sheet, err = storeJSON.Marshal(checked); err != nil {
		return bad(name, pkgv1.PackageProblemReason_PACKAGE_PROBLEM_REASON_INVALID)
	}
	if out.story, err = storeJSON.Marshal(story); err != nil {
		return bad(name, pkgv1.PackageProblemReason_PACKAGE_PROBLEM_REASON_INVALID)
	}
	return out
}

// validAbilityScores says whether the six scores of a creature are in the
// rules' range.
func validAbilityScores(a *rulesv1.AbilityScores) bool {
	if a == nil {
		return true
	}
	for _, v := range []int32{a.GetStrength(), a.GetDexterity(), a.GetConstitution(), a.GetIntelligence(), a.GetWisdom(), a.GetCharisma()} {
		if v < 1 || v > rules.MaxScore {
			return false
		}
	}
	return true
}

// sheetReason is the problem a refused sheet becomes: a thing the rules do not
// have (a class of the table the package does not bring, an unknown key) is
// told from any other mistake.
func sheetReason(err error) pkgv1.PackageProblemReason {
	msg := err.Error()
	if ce, ok := errors.AsType[*connect.Error](err); ok {
		msg = ce.Message()
	}
	for _, unknown := range []string{"unknown", "not a ", "is not an ", "does not exist", "not in the"} {
		if strings.Contains(msg, unknown) {
			return pkgv1.PackageProblemReason_PACKAGE_PROBLEM_REASON_NOT_IN_PACKAGE
		}
	}
	return pkgv1.PackageProblemReason_PACKAGE_PROBLEM_REASON_INVALID
}

// --- apply ------------------------------------------------------------------

// Apply implements campaignpackage.Part.
func (p *packagePart) Apply(ctx context.Context, tx pgx.Tx, in *campaignpackage.Import) error {
	q := p.s.queries.WithTx(tx)
	if sc, _ := in.Get(stashContent).(*stagedContent); sc != nil && (len(sc.entries) > 0 || len(sc.off) > 0) {
		now := p.s.now()
		if _, err := q.BumpContentRevision(ctx, charactersdb.BumpContentRevisionParams{CampaignID: in.CampaignID, Now: now}); err != nil {
			return fmt.Errorf("start the content revision: %w", err)
		}
		for _, e := range sc.entries {
			r := e.row
			if err := q.InsertImportedCampaignContent(ctx, charactersdb.InsertImportedCampaignContentParams{
				CampaignID: in.CampaignID, ContentKey: r.ContentKey, Kind: r.Kind, NamePt: r.NamePt, Data: r.Data, Revision: r.Revision,
				ArchivedAt: r.ArchivedAt, Now: r.CreatedAt,
			}); err != nil {
				return fmt.Errorf("insert a table entry: %w", err)
			}
		}
		if len(sc.off) > 0 {
			if err := q.InsertContentOff(ctx, charactersdb.InsertContentOffParams{CampaignID: in.CampaignID, ContentKeys: sc.off, Now: now}); err != nil {
				return fmt.Errorf("switch options off: %w", err)
			}
		}
	}
	chars, _ := in.Get(stashCharacters).(*stagedCharacters)
	if chars == nil {
		return nil
	}
	for _, c := range chars.npcs {
		if err := q.InsertImportedNpc(ctx, charactersdb.InsertImportedNpcParams{
			ID: c.id, CampaignID: in.CampaignID, Kind: c.kind, MasterUserID: &in.UserID, Name: c.name, Sheet: c.sheet, Story: c.story, Now: c.createdAt,
		}); err != nil {
			return fmt.Errorf("insert an imported NPC: %w", err)
		}
		if c.notes != "" {
			if _, err := q.UpsertMasterNotes(ctx, charactersdb.UpsertMasterNotesParams{CampaignID: in.CampaignID, CharacterID: c.id, Notes: c.notes, UpdatedAt: c.createdAt}); err != nil {
				return fmt.Errorf("save the master's notes: %w", err)
			}
		}
	}
	return p.applyPlayers(ctx, tx, in, chars.players)
}
