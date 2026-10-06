package characters

import (
	"slices"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
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
// after the sheet was last saved.
const IssueTableContentChanged = "table_content_changed"

// addChangedContent tells the sheet's owner that a table entry it uses changed
// (RN-23, question 80): the numbers already use the new rules, and each entry is
// an issue plus a ChangedContent, for the banner and the sheet of what changed. It
// is a notice, never a Validate error: it blocks nothing, and saving the sheet
// clears it. savedRevision is the content revision the sheet was last saved at.
func addChangedContent(d *rulesv1.DerivedSheet, content *rules.Content, build rules.Build, savedRevision int) {
	for _, ch := range content.ChangedSince(build, savedRevision) {
		name := content.NamePT(ch.Key)
		d.ChangedContent = append(d.ChangedContent, &rulesv1.ChangedContent{
			Key: ch.Key, NamePt: name, Field: ch.Field, Revision: i32(ch.Revision), SavedRevision: i32(savedRevision),
		})
		d.Issues = append(d.Issues, &rulesv1.Issue{
			Code: IssueTableContentChanged, Field: ch.Field,
			Message: changedKinds[keyKind(ch.Key)] + " " + name + " mudou depois da última vez que esta ficha foi salva. Os números já usam as regras novas.",
		})
	}
}
