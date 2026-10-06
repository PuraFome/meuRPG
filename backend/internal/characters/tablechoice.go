package characters

import (
	"context"
	"slices"
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
	"github.com/PuraFome/meuRPG/backend/internal/authz"
	"github.com/PuraFome/meuRPG/backend/internal/rules"
)

// What the table's own content means for a sheet (MR-025, RN-23, ADR-0018).

// newArchivedChoice finds a table entry that the master archived and that after
// picks but before did not: a retired entry is never a new choice, while a sheet
// that already has it keeps it (the key is in before, so it is not new). before is
// nil for a sheet that does not exist yet. It returns the first one, in the
// sheet's order, and the sheet field that holds it.
func newArchivedChoice(content *rules.Content, before, after *charactersv1.FullSheet) (key, field string, found bool) {
	archived := content.ArchivedKeys(buildOf(after))
	if len(archived) == 0 {
		return "", "", false
	}
	already := content.ArchivedKeys(buildOf(before))
	for _, kf := range rules.BuildKeys(buildOf(after)) {
		if slices.Contains(archived, kf.Key) && !slices.Contains(already, kf.Key) {
			return kf.Key, kf.Field, true
		}
	}
	return "", "", false
}

// errArchivedChoice is the refusal of CreateCharacter and UpdateCharacter for a
// newly chosen archived key. characterID is empty for a character that does not
// exist yet.
func errArchivedChoice(characterID, key string) error {
	return errBlockedByContent(charactersv1.CharacterBlockedReason_CHARACTER_BLOCKED_REASON_ARCHIVED_CONTENT, characterID, key)
}

// CheckSheetCanMove refuses a sheet that uses the table's own content ("...@mesa"
// keys) for any copy or move to another campaign: the content belongs to the
// campaign it was written for (ADR-0018, section 10), and the other has no such
// keys. It answers `failed_precondition` with the CharacterBlocked detail
// TABLE_CONTENT_STAYS and the first key, or nil. Nothing calls it yet: the copy of
// a character (MR-021) and the reuse of NPCs (MR-022) must, before they write.
func CheckSheetCanMove(sheet *charactersv1.CharacterSheet) error {
	full := sheet.GetFull()
	if full == nil {
		return nil // a basic sheet has no content keys
	}
	if keys := rules.TableKeys(buildOf(full)); len(keys) > 0 {
		return errBlockedByContent(charactersv1.CharacterBlockedReason_CHARACTER_BLOCKED_REASON_TABLE_CONTENT_STAYS, "", keys[0])
	}
	return nil
}

// changedKinds says what changed, for the issue's text: "A classe mudou".
var changedKinds = map[string]string{
	"class": "A classe", "subclass": "A subclasse", "race": "A raça", "subrace": "A sub-raça",
	"background": "O antecedente", "spell": "A magia",
}

// IssueTableContentChanged is the code of the issue "A classe mudou" (and its
// kin), on the derived sheet of a sheet that uses a table entry which changed
// after the sheet was last saved and that has issues it did not have then.
const IssueTableContentChanged = "table_content_changed"

// issueKeys is a set of issues as the sheet stores it ("code|field", sorted, no
// repeats): what the sheet already had, so a later issue is told apart.
func issueKeys(issues []rules.Issue) []string {
	var out []string
	for _, is := range issues {
		out = append(out, is.Code+"|"+is.Field)
	}
	slices.Sort(out)
	return slices.Compact(out)
}

// newIssues are the issues a sheet has now that it did not have when it was
// last saved (full.KnownIssues): the ones a change of a table entry it uses can
// be blamed for.
func newIssues(content *rules.Content, full *charactersv1.FullSheet) []rules.Issue {
	known := full.GetKnownIssues()
	var out []rules.Issue
	for _, is := range rules.Derive(buildOf(full), content).Issues {
		if !slices.Contains(known, is.Code+"|"+is.Field) {
			out = append(out, is)
		}
	}
	return out
}

// addChangedContent tells the sheet's owner that a table entry it uses changed
// and no longer fits (RN-23, question 80, E10-02 state 8). The numbers already
// use the new rules; this is the notice. It shows only while the sheet has a
// Derive issue that it did not have when it was last saved AND an entry it uses
// changed after that: worked out on every read, so it goes away by itself when no
// such issue remains, and a change that left no new issue (a text edit) shows
// nothing. It is never a Validate error, so it blocks nothing; saving the sheet
// again (which records its issues and revision) clears it as well. d is derived
// from build with content.
func addChangedContent(d *rulesv1.DerivedSheet, content *rules.Content, build rules.Build, full *charactersv1.FullSheet) {
	saved := int(full.GetContentRevision())
	changed := content.ChangedSince(build, saved)
	if len(changed) == 0 {
		return
	}
	known := full.GetKnownIssues()
	var messages []string
	for _, is := range d.Issues {
		if !slices.Contains(known, is.Code+"|"+is.Field) {
			messages = append(messages, is.Message)
		}
	}
	if len(messages) == 0 {
		return
	}
	for _, ch := range changed {
		name := content.NamePT(ch.Key)
		d.ChangedContent = append(d.ChangedContent, &rulesv1.ChangedContent{
			Key: ch.Key, NamePt: name, Field: ch.Field, Revision: i32(ch.Revision), SavedRevision: i32(saved),
			ChangedAt: timestampOf(ch.ChangedAt), Messages: messages,
		})
		d.Issues = append(d.Issues, &rulesv1.Issue{
			Code: IssueTableContentChanged, Field: ch.Field,
			Message: changedKinds[keyKind(ch.Key)] + " " + name + " mudou: a ficha ficou com avisos que não tinha. Os números já usam as regras novas.",
		})
	}
}

func timestampOf(t time.Time) *timestamppb.Timestamp {
	if t.IsZero() {
		return nil
	}
	return timestamppb.New(t)
}

// callerUses says whether one of the caller's own characters in the campaign uses
// the content key: what lets a player keep reading about an entry the master
// archived after their sheet took it.
func (s *Service) callerUses(ctx context.Context, m authz.Membership, key string) (bool, error) {
	sheets, err := s.queries.ListCampaignSheets(ctx, m.CampaignID)
	if err != nil {
		return false, err
	}
	for _, row := range sheets {
		if deref(row.PlayerUserID) != m.UserID {
			continue
		}
		sheet, err := loadSheet(row.ID, row.Sheet)
		if err != nil {
			return false, err
		}
		if full := sheet.GetFull(); full != nil && slices.Contains(rules.TableKeys(buildOf(full)), key) {
			return true, nil
		}
	}
	return false, nil
}
