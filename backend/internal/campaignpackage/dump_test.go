package campaignpackage_test

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"testing"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	campaignsv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/campaigns/v1"
	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	mapsv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/maps/v1"
	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
)

var uuidText = regexp.MustCompile(`[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`)

// dumper reads everything a package carries of a campaign through the API and
// writes it as text with the ids replaced by their order of appearance, so the
// dump of a campaign and the dump of its import are equal exactly when every
// field and every reference came across (ids aside).
type dumper struct {
	t        *testing.T
	campaign string
	labels   map[string]string
	out      strings.Builder
}

// ignored are the fields a new campaign has of its own: moments and counters.
var ignored = map[string]bool{
	"created_at": true, "updated_at": true, "revision": true, "layers_revision": true, "light_revision": true, "vision_epoch": true,
	"characters_using": true, "content_revision": true, "table_revision": true, "content_version": true, "expires_at": true, "archived_at": true,
	"revealed_at": true, "updated_by_display_name": true, "uploaded_by_display_name": true, "uploaded_by": true,
}

func (d *dumper) alias(id string) string {
	if id == d.campaign {
		return "CAMPAIGN"
	}
	if l, ok := d.labels[id]; ok {
		return l
	}
	l := fmt.Sprintf("#%d", len(d.labels)+1)
	d.labels[id] = l
	return l
}

func (d *dumper) normalize(v any) any {
	switch x := v.(type) {
	case map[string]any:
		keys := make([]string, 0, len(x))
		for k := range x {
			if !ignored[k] {
				keys = append(keys, k)
			}
		}
		sort.Strings(keys)
		out := map[string]any{}
		for _, k := range keys {
			out[k] = d.normalize(x[k])
		}
		return out
	case []any:
		out := make([]any, len(x))
		for i, e := range x {
			out[i] = d.normalize(e)
		}
		return out
	case string:
		return uuidText.ReplaceAllStringFunc(x, d.alias)
	}
	return v
}

// add writes a section: the message as normalized JSON.
func (d *dumper) add(section string, m proto.Message) {
	d.t.Helper()
	raw, err := protojson.MarshalOptions{UseProtoNames: true}.Marshal(m)
	if err != nil {
		d.t.Fatal(err)
	}
	var generic any
	if err := json.Unmarshal(raw, &generic); err != nil {
		d.t.Fatal(err)
	}
	b, err := json.MarshalIndent(d.normalize(generic), "", " ")
	if err != nil {
		d.t.Fatal(err)
	}
	fmt.Fprintf(&d.out, "## %s\n%s\n", section, b)
}

func (d *dumper) addText(section, text string) {
	fmt.Fprintf(&d.out, "## %s\n%s\n", section, uuidText.ReplaceAllStringFunc(text, d.alias))
}

// dump reads a campaign as its master sees it.
func (h *harness) dump(master *user, campaignID string) string {
	t := h.t
	ctx := t.Context()
	d := &dumper{t: t, campaign: campaignID, labels: map[string]string{}}

	mine := must(master.campaigns.ListMyCampaigns(ctx, connect.NewRequest(&campaignsv1.ListMyCampaignsRequest{})))
	for _, c := range mine.GetCampaigns() {
		if c.GetId() == campaignID {
			d.add("campaign", &campaignsv1.Campaign{Name: c.GetName(), XpMode: c.GetXpMode(), DiceMode: c.GetDiceMode(), MyRole: c.GetMyRole()})
		}
	}
	rulesRes := must(master.campaigns.GetTableRules(ctx, connect.NewRequest(&campaignsv1.GetTableRulesRequest{CampaignId: campaignID})))
	d.add("table rules", rulesRes.GetRules())
	doc := must(master.document.GetCampaignDocument(ctx, connect.NewRequest(&campaignsv1.GetCampaignDocumentRequest{CampaignId: campaignID})))
	d.addText("document", doc.GetDocument().GetBody())

	gallery := must(master.gallery.ListGalleryImages(ctx, connect.NewRequest(&mapsv1.ListGalleryImagesRequest{CampaignId: campaignID})))
	for _, img := range gallery.GetImages() {
		img.ByteSize = 0 // the re-encoded file may differ by a few bytes
		d.add("image "+img.GetName(), img)
	}

	maps := must(master.maps.ListMaps(ctx, connect.NewRequest(&mapsv1.ListMapsRequest{CampaignId: campaignID})))
	for _, m := range maps.GetMaps() {
		full := must(master.maps.GetMap(ctx, connect.NewRequest(&mapsv1.GetMapRequest{CampaignId: campaignID, MapId: m.GetId()})))
		d.add("map "+m.GetName(), full)
		layers := must(master.maps.GetMapLayers(ctx, connect.NewRequest(&mapsv1.GetMapLayersRequest{CampaignId: campaignID, MapId: m.GetId()})))
		d.add("layers of "+m.GetName(), layers)
		for _, p := range full.GetPoints() {
			if p.GetKind() != mapsv1.MapPointKind_MAP_POINT_KIND_BATTLE {
				continue
			}
			enc, err := master.encounters.GetBattleEncounter(ctx, connect.NewRequest(&playv1.GetBattleEncounterRequest{CampaignId: campaignID, MapPointId: p.GetId()}))
			if err != nil {
				t.Fatalf("GetBattleEncounter() error = %v", err)
			}
			enc.Msg.Evaluation = nil // measured against today's party
			d.add("encounter of "+p.GetName(), enc.Msg)
		}
	}

	chars := must(master.characters.ListCharacters(ctx, connect.NewRequest(&charactersv1.ListCharactersRequest{CampaignId: campaignID})))
	for _, c := range chars.GetCharacters() {
		full := must(master.characters.GetCharacter(ctx, connect.NewRequest(&charactersv1.GetCharacterRequest{CampaignId: campaignID, CharacterId: c.GetId()})))
		d.add("character "+c.GetName(), full.GetCharacter().GetSheet())
		d.add("story of "+c.GetName(), full.GetCharacter().GetStory())
		d.addText("kind of "+c.GetName(), c.GetKind().String())
		notes := must(master.characters.GetMasterNotes(ctx, connect.NewRequest(&charactersv1.GetMasterNotesRequest{CampaignId: campaignID, CharacterId: c.GetId()})))
		d.addText("notes of "+c.GetName(), notes.GetNotes())
	}

	puzzles := must(master.puzzles.ListPuzzles(ctx, connect.NewRequest(&playv1.ListPuzzlesRequest{CampaignId: campaignID, IncludeArchived: true})))
	for _, p := range puzzles.GetPuzzles() {
		d.add("puzzle "+p.GetName(), p)
	}

	entries := must(master.table.ListTableEntries(ctx, connect.NewRequest(&rulesv1.ListTableEntriesRequest{CampaignId: campaignID})))
	for _, e := range entries.GetEntries() {
		d.add("table entry "+e.GetKey(), e)
	}
	switches := must(master.table.ListOptionSwitches(ctx, connect.NewRequest(&rulesv1.ListOptionSwitchesRequest{CampaignId: campaignID})))
	for _, o := range switches.GetOptions() {
		if o.GetOff() {
			d.addText("switched off", o.GetKey())
		}
	}
	return d.out.String()
}
