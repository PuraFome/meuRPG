package reqlimit

import (
	"net/http"
	"strings"
	"sync"

	"github.com/PuraFome/meuRPG/backend/gen/meurpg/campaigns/v1/campaignsv1connect"
	"github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1/charactersv1connect"
	"github.com/PuraFome/meuRPG/backend/gen/meurpg/maps/v1/mapsv1connect"
	"github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1/rulesv1connect"
	"github.com/PuraFome/meuRPG/backend/internal/platform/slowclient"
)

const (
	// DefaultReadLimit is the most bytes the body of a unary RPC may have, for a
	// procedure that raisedReadLimits does not name.
	DefaultReadLimit = 64 << 10
	// Backstop is the most any one RPC message may have, whatever the
	// procedure: the server's connect.WithReadMaxBytes. No read limit is above it.
	Backstop = 4 << 20
)

// raisedReadLimits are the procedures whose unary body may be larger than
// DefaultReadLimit, by procedure ("/meurpg.x.v1.Service/Method"), in bytes.
// Each entry has a comment that says why. TestRegistry requires an entry for
// every procedure that reaches a field raised over its defaults, and none
// above Backstop. Tests change it only through the lock (withRaised).
var (
	raisedMu         sync.RWMutex
	raisedReadLimits = map[string]int64{
		// pack_json is up to 3 MiB (characters.MaxPackBytes), and proto JSON writes a
		// bytes field in base64: the message may reach the backstop.
		rulesv1connect.TableContentServiceImportTableContentProcedure: Backstop,
		// body is up to 400 KiB, and the JSON of a text of line breaks is twice its size.
		campaignsv1connect.CampaignDocumentServiceUpdateCampaignDocumentProcedure: 512 << 10,
		// An entry's data is up to 64 KiB of JSON (characters.MaxTableEntryBytes) and the
		// text fields (desc_pt, higher_level_pt, text_pt) go up to 16 KiB each.
		rulesv1connect.TableContentServiceCreateTableEntryProcedure: 128 << 10,
		rulesv1connect.TableContentServiceUpdateTableEntryProcedure: 128 << 10,
		// switches is up to 1400 entries of a content key and a flag.
		rulesv1connect.TableContentServiceSetOptionSwitchesProcedure: 256 << 10,
		// A full sheet carries lists of up to 200 known spells, 1000 known_issues and
		// 1000 content_baselines, and texts of up to 40 KiB (custom_features_text, backstory).
		charactersv1connect.CharacterServiceCreateCharacterProcedure:  128 << 10,
		charactersv1connect.CharacterServiceUpdateCharacterProcedure:  128 << 10,
		charactersv1connect.CharacterServicePreviewCharacterProcedure: 128 << 10,
		charactersv1connect.CharacterServicePreviewChoicesProcedure:   128 << 10,
		// backstory is up to 40 KiB, with the other texts of the story.
		charactersv1connect.CharacterServiceUpdateCharacterStoryProcedure: 128 << 10,
		// notes is up to 80 KiB.
		charactersv1connect.CharacterServiceUpdateMasterNotesProcedure: 128 << 10,
		// The choices list up to 200 known_spell_keys, with the other lists of keys of
		// up to 128 bytes each.
		charactersv1connect.CharacterServiceLevelUpCharacterProcedure: 128 << 10,
		charactersv1connect.CharacterServicePreviewLevelUpProcedure:   128 << 10,
		// hooks is up to 16 KiB, with the point's description and scene actions.
		mapsv1connect.MapServiceCreateMapPointProcedure: 128 << 10,
		mapsv1connect.MapServiceUpdateMapPointProcedure: 128 << 10,
		// squares is up to 400 entries of a column and a row.
		mapsv1connect.MapServicePaintMapCellsProcedure: 128 << 10,
	}
)

// ReadLimit is the most bytes the body of a unary request to the procedure may have.
func ReadLimit(procedure string) int64 {
	raisedMu.RLock()
	defer raisedMu.RUnlock()
	if n, ok := raisedReadLimits[procedure]; ok {
		return n
	}
	return DefaultReadLimit
}

// Middleware reads the body of a unary RPC through http.MaxBytesReader at
// ReadLimit(procedure). Connect reports a body over the limit as
// `resource_exhausted`. Put it outside slowclient.Middleware, so that the
// deadline's body stays the outermost one.
func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/meurpg.") && slowclient.IsUnaryRPC(r) {
			r.Body = http.MaxBytesReader(w, r.Body, ReadLimit(r.URL.Path))
		}
		next.ServeHTTP(w, r)
	})
}
