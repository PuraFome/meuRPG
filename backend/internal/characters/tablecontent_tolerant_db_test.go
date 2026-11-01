package characters

import (
	"slices"
	"strings"
	"testing"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/proto"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
	"github.com/PuraFome/meuRPG/backend/internal/authz"
	"github.com/PuraFome/meuRPG/backend/internal/rules"
)

// refuseStored makes the engine refuse the stored entries (a class with a die it does not
// have, a spell above level 9) and moves the revision, as an engine change would leave them.
func (h *harness) refuseStored(campaign string, keys ...string) {
	h.t.Helper()
	for _, k := range keys {
		set := `jsonb_set(data, '{level}', '12')`
		if strings.HasPrefix(k, "class:") {
			set = `jsonb_set(data, '{hit_die}', '7')`
		}
		if _, err := h.pool.Exec(h.t.Context(), `UPDATE campaign_content SET data = `+set+` WHERE campaign_id = $1 AND content_key = $2`, campaign, k); err != nil {
			h.t.Fatalf("refuse %s: %v", k, err)
		}
	}
	if _, err := h.pool.Exec(h.t.Context(), `UPDATE campaign_content_state SET revision = revision + 1 WHERE campaign_id = $1`, campaign); err != nil {
		h.t.Fatalf("bump the revision: %v", err)
	}
}

// tolerantTable is a table with a class, its subclass, a race and a spell, and a sheet
// that knows the spell.
type tolerantTable struct {
	h                    *harness
	master, owner        *user
	campaign             string
	class, sub, race, sp *rulesv1.TableEntry
	pc                   *charactersv1.Character
}

func newTolerantTable(t *testing.T) *tolerantTable {
	t.Helper()
	h := newHarness(t)
	master, owner := h.newUser("Samuel"), h.newUser("Dona")
	campaign := h.newCampaign(master, "Mirathel", owner)
	f := &tolerantTable{h: h, master: master, owner: owner, campaign: campaign}
	f.class = master.addEntry(t, campaign, testClass("Guardião"))
	f.sub = master.addEntry(t, campaign, testSubclass("Caminho", f.class.GetKey()))
	f.race = master.addEntry(t, campaign, testRace("Gnomo de teste"))
	f.sp = master.addEntry(t, campaign, testSpell("Raio de teste", "class:wizard"))
	f.pc = owner.create(t, campaign, charactersv1.CharacterKind_CHARACTER_KIND_PLAYER, "Pensantus", pensantusWith(f.sp.GetKey()))
	return f
}

// TestTableSourceReadsWithoutTheEntriesTheRulesRefuse: a stored entry the engine refuses
// is left out with what depends on it; the rest of the campaign reads and plays.
func TestTableSourceReadsWithoutTheEntriesTheRulesRefuse(t *testing.T) {
	t.Parallel()
	f := newTolerantTable(t)
	f.h.refuseStored(f.campaign, f.class.GetKey(), f.sp.GetKey())
	src := f.h.svc.content.(*TableSource)

	c, err := src.ContentFor(t.Context(), nil, f.campaign)
	if err != nil {
		t.Fatalf("ContentFor() error = %v, want the content without the refused entries", err)
	}
	if c.NamePT(f.race.GetKey()) != "Gnomo de teste" {
		t.Errorf("the healthy race is not in the content")
	}
	for _, e := range []*rulesv1.TableEntry{f.class, f.sub, f.sp} {
		if c.NamePT(e.GetKey()) != "" {
			t.Errorf("%s is in the content, want it left out", e.GetKey())
		}
	}
	got := map[string]string{}
	for _, r := range c.Refused() {
		got[r.Key] = r.Reason
	}
	if len(got) != 3 || got[f.class.GetKey()] != rules.ReasonValue || got[f.sub.GetKey()] != rules.ReasonReference || got[f.sp.GetKey()] != rules.ReasonValue {
		t.Errorf("Refused() = %v, want the class and the spell (bad_value) and the subclass (dangling_reference)", c.Refused())
	}
	if again, err := src.ContentFor(t.Context(), nil, f.campaign); err != nil || again != c {
		t.Errorf("a second read = %p, %v; want the cached content %p", again, err, c)
	}
	if listed := f.master.entries(t, f.campaign); listed[f.class.GetKey()] == nil || listed[f.sp.GetKey()] == nil {
		t.Errorf("ListTableEntries lost a refused row: %d entries", len(listed))
	}
	sheet := f.owner.get(t, f.campaign, f.pc.GetId())
	if !slices.ContainsFunc(sheet.GetDerived().GetIssues(), func(is *rulesv1.Issue) bool { return is.GetCode() == rules.IssueUnknownKey }) {
		t.Errorf("the sheet that knew the spell has issues %v, want %s", sheet.GetDerived().GetIssues(), rules.IssueUnknownKey)
	}
}

// TestTableContentWritesWorkWithARefusedEntryStored: another entry is written, and the
// refused one is fixed; a write that breaks a stored dependent is still refused.
func TestTableContentWritesWorkWithARefusedEntryStored(t *testing.T) {
	t.Parallel()
	f := newTolerantTable(t)
	f.h.refuseStored(f.campaign, f.sp.GetKey())
	other := f.master.addEntry(t, f.campaign, testBackground("Guarda de farol"))
	if other.GetKey() == "" {
		t.Fatal("no key")
	}
	// Archiving and bringing back work, and so does switching an option off.
	if _, _, err := f.h.svc.setArchived(t.Context(), f.membership(), other.GetKey(), true); err != nil {
		t.Fatalf("archive with a refused entry stored: %v", err)
	}
	if _, _, err := f.h.svc.setArchived(t.Context(), f.membership(), other.GetKey(), false); err != nil {
		t.Fatalf("unarchive with a refused entry stored: %v", err)
	}
	for k := range f.master.options(t, f.campaign) {
		f.master.setOff(t, f.campaign, true, k)
		break
	}
	// Fixing the refused entry (its name back) works.
	spell := f.master.entries(t, f.campaign)[f.sp.GetKey()]
	fixed := proto.Clone(bodyMessage(spell)).(*rulesv1.TableSpell)
	fixed.Level = 1
	if _, err := f.master.table.UpdateTableEntry(t.Context(), connect.NewRequest(updateReq(f.campaign, f.sp.GetKey(), spell.GetRevision(), fixed))); err != nil {
		t.Fatalf("fixing the refused spell: %v", err)
	}
	c, err := f.h.svc.content.ContentFor(t.Context(), nil, f.campaign)
	if err != nil || len(c.Refused()) != 0 || c.NamePT(f.sp.GetKey()) != "Raio de teste" {
		t.Fatalf("after the fix: refused %v, %v; want the spell back", c.Refused(), err)
	}
	// A write that breaks a healthy stored dependent: the subclass needs the class
	// to offer it at its first level, so a later subclass level is refused.
	class := f.master.entries(t, f.campaign)[f.class.GetKey()]
	broken := proto.Clone(bodyMessage(class)).(*rulesv1.TableClass)
	broken.SubclassLevel = 9
	_, err = f.master.table.UpdateTableEntry(t.Context(), connect.NewRequest(updateReq(f.campaign, f.class.GetKey(), class.GetRevision(), broken)))
	if err == nil {
		t.Fatal("a write that breaks the stored subclass was accepted")
	}
	// With the spell refused again, the same write is still refused.
	f.h.refuseStored(f.campaign, f.sp.GetKey())
	class = f.master.entries(t, f.campaign)[f.class.GetKey()]
	_, err = f.master.table.UpdateTableEntry(t.Context(), connect.NewRequest(updateReq(f.campaign, f.class.GetKey(), class.GetRevision(), broken)))
	if err == nil {
		t.Fatal("with a refused entry stored, the write that breaks the subclass was accepted")
	}
	if v := violationsOfErr(t, err); !slices.ContainsFunc(v, func(x *rulesv1.TableContentViolation) bool { return x.GetKey() == f.sub.GetKey() }) {
		t.Errorf("violations %v, want one that names the subclass", err)
	}
}

func (f *tolerantTable) membership() authz.Membership {
	return authz.Membership{CampaignID: f.campaign, Role: authz.RoleMaster}
}

// TestTableImportIgnoresAStoredEntryTheRulesRefuse: a stored entry outside the pack that the
// rules refuse does not make the import refuse; the pack's entries land.
func TestTableImportIgnoresAStoredEntryTheRulesRefuse(t *testing.T) {
	t.Parallel()
	pt := newPackTable(t)
	pt.master.addEntry(t, pt.from, testRace("Anão das Brumas"))
	pack := pt.export(t, pt.from)
	spell := pt.master.addEntry(t, pt.to, testSpell("Raio de teste", "class:wizard"))
	pt.h.refuseStored(pt.to, spell.GetKey())

	pv := pt.preview(t, pt.to, pack)
	if pv.GetTotals().GetRefused() != 0 || pv.GetTotals().GetNew() != 1 {
		t.Fatalf("preview = %v, want one new entry and no refusal", pv)
	}
	pt.apply(t, pt.to, pack, "tolerant-1")
	c, err := pt.h.svc.content.ContentFor(t.Context(), nil, pt.to)
	if err != nil || c.NamePT(pack.GetEntries()[0].GetKey()) != "Anão das Brumas" || len(c.Refused()) != 1 {
		t.Errorf("after the import: refused %v, %v; want the race in and the spell still left out", c.Refused(), err)
	}
}
