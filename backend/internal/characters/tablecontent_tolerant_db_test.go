package characters

import (
	"maps"
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

// refusedKeys are the keys the campaign's content reports as left out.
func (f *tolerantTable) refusedKeys(t *testing.T) []string {
	t.Helper()
	c, err := f.h.svc.content.ContentFor(t.Context(), nil, f.campaign)
	if err != nil {
		t.Fatalf("ContentFor() error = %v", err)
	}
	var keys []string
	for _, r := range c.Refused() {
		keys = append(keys, r.Key)
	}
	return keys
}

// update writes body over the stored entry of key, with the revision the master reads.
func (f *tolerantTable) update(key string, body proto.Message) error {
	rev := tableRevisionOf(f.h.t, f.master, f.campaign, key)
	_, err := f.master.table.UpdateTableEntry(f.h.t.Context(), connect.NewRequest(updateReq(f.campaign, key, rev, body)))
	return err
}

// TestTableContentWritesWorkWithARefusedEntryStored: with two entries refused, other entries
// are written, archived and switched, and repairing one leaves the other out.
func TestTableContentWritesWorkWithARefusedEntryStored(t *testing.T) {
	t.Parallel()
	f := newTolerantTable(t)
	f.h.refuseStored(f.campaign, f.sp.GetKey(), f.class.GetKey())
	other := f.master.addEntry(t, f.campaign, testBackground("Guarda de farol"))
	if _, _, err := f.h.svc.setArchived(t.Context(), f.membership(), other.GetKey(), true); err != nil {
		t.Fatalf("archive with refused entries stored: %v", err)
	}
	if _, _, err := f.h.svc.setArchived(t.Context(), f.membership(), other.GetKey(), false); err != nil {
		t.Fatalf("unarchive with refused entries stored: %v", err)
	}
	options := slices.Sorted(maps.Keys(f.master.options(t, f.campaign)))
	if len(options) == 0 {
		t.Fatal("the campaign lists no option to switch")
	}
	f.master.setOff(t, f.campaign, true, options[0])
	if !f.master.options(t, f.campaign)[options[0]].GetOff() {
		t.Errorf("option %s is not off after the switch", options[0])
	}

	spell := proto.Clone(bodyMessage(f.master.entries(t, f.campaign)[f.sp.GetKey()])).(*rulesv1.TableSpell)
	spell.Level = 1
	if err := f.update(f.sp.GetKey(), spell); err != nil {
		t.Fatalf("repairing the spell while the class is refused: %v", err)
	}
	if got, want := f.refusedKeys(t), []string{f.class.GetKey(), f.sub.GetKey()}; !slices.Equal(got, want) {
		t.Errorf("left out after repairing the spell = %v, want %v", got, want)
	}
	class := proto.Clone(bodyMessage(f.master.entries(t, f.campaign)[f.class.GetKey()])).(*rulesv1.TableClass)
	class.HitDie = 8
	if err := f.update(f.class.GetKey(), class); err != nil {
		t.Fatalf("repairing the class: %v", err)
	}
	if got := f.refusedKeys(t); len(got) != 0 {
		t.Errorf("left out after repairing both = %v, want none", got)
	}
}

// TestTableContentWriteThatBreaksAStoredEntryIsRefused: a write is refused for its own
// violations and for the healthy stored entry it breaks, with or without a refused entry stored.
func TestTableContentWriteThatBreaksAStoredEntryIsRefused(t *testing.T) {
	t.Parallel()
	f := newTolerantTable(t)
	broken := proto.Clone(bodyMessage(f.master.entries(t, f.campaign)[f.class.GetKey()])).(*rulesv1.TableClass)
	broken.SubclassLevel = 9
	wantSub := func(label string, err error) {
		t.Helper()
		if err == nil {
			t.Fatalf("%s: the write that breaks the subclass was accepted", label)
		}
		if v := violationsOfErr(t, err); !slices.ContainsFunc(v, func(x *rulesv1.TableContentViolation) bool {
			return x.GetKey() == f.sub.GetKey() && strings.HasPrefix(x.GetField(), "table_subclass")
		}) {
			t.Errorf("%s: violations %v, want one at the subclass's field with its key", label, v)
		}
	}
	wantSub("healthy table", f.update(f.class.GetKey(), broken))
	f.h.refuseStored(f.campaign, f.sp.GetKey())
	wantSub("a refused spell stored", f.update(f.class.GetKey(), broken))

	// The refused spell written again with the same fault: refused on its own field, no key.
	err := f.update(f.sp.GetKey(), bodyMessage(f.master.entries(t, f.campaign)[f.sp.GetKey()]))
	v := violationsOfErr(t, err)
	if len(v) == 0 || v[0].GetKey() != "" || v[0].GetField() != "table_spell.level" || v[0].GetReason() != rules.ReasonValue {
		t.Errorf("violations of the unrepaired spell = %v, want table_spell.level (bad_value) with no key", v)
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

// TestTableImportThatBreaksAStoredEntryIsRefused: a pack that updates a class so that a stored
// subclass breaks is refused, with a stored refused entry outside the pack present too.
func TestTableImportThatBreaksAStoredEntryIsRefused(t *testing.T) {
	t.Parallel()
	pt := newPackTable(t)
	class := testClass("Guardião")
	class.SubclassLevel = 9
	pt.master.addEntry(t, pt.from, class)
	pack := pt.export(t, pt.from)
	stored := pt.master.addEntry(t, pt.to, testClass("Guardião"))
	sub := pt.master.addEntry(t, pt.to, testSubclass("Caminho", stored.GetKey()))
	spell := pt.master.addEntry(t, pt.to, testSpell("Raio de teste", "class:wizard"))
	pt.h.refuseStored(pt.to, spell.GetKey())

	_, err := pt.importPack(pt.master, pt.to, pack, rulesv1.TableImportMode_TABLE_IMPORT_MODE_PREVIEW, "")
	if err == nil {
		t.Fatal("the import that breaks the stored subclass was accepted")
	}
	if v := violationsOfErr(t, err); !slices.ContainsFunc(v, func(x *rulesv1.TableContentViolation) bool { return x.GetKey() == sub.GetKey() }) {
		t.Errorf("violations %v, want one that names the subclass", v)
	}
}

// TestTableSourceFailsWhenTheContentAsAWholeIsRefused: a violation that names no entry fails the read.
func TestTableSourceFailsWhenTheContentAsAWholeIsRefused(t *testing.T) {
	t.Parallel()
	f := newTolerantTable(t)
	_, err := f.h.pool.Exec(t.Context(), `INSERT INTO campaign_content (campaign_id, content_key, kind, name_pt, data, revision, created_at, updated_at)
		SELECT campaign_id, 'race:copia-' || i || '@mesa', kind, 'Copia ' || i, data, revision, created_at, updated_at
		FROM campaign_content, generate_series(1, 1000) AS i WHERE campaign_id = $1 AND content_key = $2`, f.campaign, f.race.GetKey())
	if err != nil {
		t.Fatalf("store 1000 more entries: %v", err)
	}
	f.h.refuseStored(f.campaign, f.sp.GetKey())
	if c, err := f.h.svc.content.ContentFor(t.Context(), nil, f.campaign); err == nil {
		t.Errorf("ContentFor() = %v, nil; want the read to fail over the limit", c)
	}
}
