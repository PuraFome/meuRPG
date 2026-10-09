package campaignpackage_test

import (
	"archive/zip"
	"bytes"
	"encoding/base64"
	"io"
	"strings"
	"testing"

	"connectrpc.com/connect"

	campaignsv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/campaigns/v1"
	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	mapsv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/maps/v1"
	notesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/notes/v1"
	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
)

// everything is the text of every entry of a zip, and the names of the entries.
func everything(t *testing.T, data []byte) string {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	var sb strings.Builder
	for _, f := range zr.File {
		sb.WriteString(f.Name + "\n")
		if strings.HasPrefix(f.Name, "images/") {
			continue // the image files are binary
		}
		rc, _ := f.Open()
		b, _ := io.ReadAll(rc)
		_ = rc.Close()
		sb.Write(b)
		sb.WriteString("\n")
	}
	return sb.String()
}

// What belongs to the players, the sessions and the accounts never goes in a
// package: their private notes, the session history and logs, their names and
// e-mails, who owns what, the invites, the claim links and what each remembers
// of the fog.
func TestAPackageNeverCarriesWhatBelongsToPlayersSessionsOrAccounts(t *testing.T) {
	h := newHarness(t)
	master := h.newUser("Mestre LEAKCANARY-account-1")
	player := h.newUser("Jogadora LEAKCANARY-account-2")
	f := h.buildFixture(master)
	ctx := t.Context()
	h.invite(master, f.campaign, player)

	// The player's own character, and a private note.
	mine := must(player.characters.CreateCharacter(ctx, connect.NewRequest(&charactersv1.CreateCharacterRequest{
		CampaignId: f.campaign, Kind: charactersv1.CharacterKind_CHARACTER_KIND_PLAYER, Name: "Personagem LEAKCANARY-char-1",
		AbilityMethod: charactersv1.AbilityMethod_ABILITY_METHOD_TYPED,
		Sheet: &charactersv1.CharacterSheet{Content: &charactersv1.CharacterSheet_Full{Full: &charactersv1.FullSheet{
			BaseScores: &rulesv1.AbilityScores{Strength: 12, Dexterity: 14, Constitution: 13, Intelligence: 10, Wisdom: 10, Charisma: 10},
			RaceKey:    "race:human", Classes: []*charactersv1.ClassLevel{{ClassKey: "class:fighter", Level: 1}},
		}}},
	})))
	_ = mine
	_ = must(player.notes.CreateNote(ctx, connect.NewRequest(&notesv1.CreateNoteRequest{CampaignId: f.campaign, Text: "LEAKCANARY-note-1 o mestre esconde algo"})))

	// A pending invite token, and a session.
	inv := must(master.campaigns.CreateInvite(ctx, connect.NewRequest(&campaignsv1.CreateInviteRequest{CampaignId: f.campaign, MaxUses: 3})))
	_ = must(master.campaigns.ListInvites(ctx, connect.NewRequest(&campaignsv1.ListInvitesRequest{CampaignId: f.campaign})))
	_ = must(master.play.StartGameSession(ctx, connect.NewRequest(&playv1.StartGameSessionRequest{CampaignId: f.campaign})))
	// The session's history carries free text (a dice label). The event is written
	// into the session that was just started, and exactly one row must take it.
	logged, err := h.pool.Exec(ctx, `
		INSERT INTO session_events (game_session_id, seq, kind, actor_user_id, payload, created_at)
		SELECT gs.id, COALESCE((SELECT MAX(e.seq) FROM session_events AS e WHERE e.game_session_id = gs.id), 0) + 1,
		       'character_vitals_adjusted', $2, '{"label":"LEAKCANARY-log-1"}'::JSONB, now()
		FROM game_sessions AS gs WHERE gs.campaign_id = $1 AND gs.ended_at IS NULL`, f.campaign, player.id)
	if err != nil {
		t.Fatalf("plant the log line: %v", err)
	}
	if logged.RowsAffected() != 1 {
		t.Fatalf("the log line was planted in %d session events, want exactly 1", logged.RowsAffected())
	}

	// What the player remembers of the fog of a map: bytes, which a package would carry as base64.
	fog := []byte("LEAKCANARY-fog-1")
	planted, err := h.pool.Exec(ctx, `INSERT INTO map_vision_memory (map_id, user_id, seen, epoch, updated_at) VALUES ($1, $2, $3, 0, now())`, f.village, player.id, fog)
	if err != nil {
		t.Fatalf("plant the fog memory: %v", err)
	}
	if planted.RowsAffected() != 1 {
		t.Fatalf("the fog memory was planted in %d rows, want 1", planted.RowsAffected())
	}

	// Positive controls: all of it exists in the source campaign before the export, so
	// that "the package lacks it" means the package left it out.
	if n := h.count(`SELECT count(*)::INT FROM player_notes WHERE campaign_id = $1 AND text LIKE '%LEAKCANARY-note-1%'`, f.campaign); n != 1 {
		t.Fatalf("the note exists %d times in the source campaign, want 1", n)
	}
	if n := h.count(`SELECT count(*)::INT FROM session_events AS e JOIN game_sessions AS gs ON gs.id = e.game_session_id WHERE gs.campaign_id = $1 AND e.payload::TEXT LIKE '%LEAKCANARY-log-1%'`, f.campaign); n != 1 {
		t.Fatalf("the log line exists %d times in the source campaign, want 1", n)
	}
	if n := h.count(`SELECT count(*)::INT FROM map_vision_memory WHERE map_id = $1 AND seen = $2`, f.village, fog); n != 1 {
		t.Fatalf("the fog memory exists %d times in the source campaign, want 1", n)
	}

	zipped := master.export(f.campaign)
	text := everything(t, zipped)
	for _, canary := range []string{
		"LEAKCANARY-account-1", "LEAKCANARY-account-2", "LEAKCANARY-note-1", "LEAKCANARY-log-1", "LEAKCANARY-fog-1",
		base64.StdEncoding.EncodeToString(fog),
		inv.GetToken(), player.id, master.id, "example.com", "@",
	} {
		if canary != "@" && strings.Contains(text, canary) {
			t.Errorf("the package carries %q", canary)
		}
	}
	for _, field := range []string{"player_user_id", "master_user_id", "email", "user_id", "token", "claim", "owner", "seen", "vision", "epoch"} {
		if strings.Contains(text, `"`+field) {
			t.Errorf("the package has a field named like %q", field)
		}
	}
	// The positive control: what is the master's is there.
	for _, want := range []string{"Taverna do Corvo", "Esconde um segredo.", "Trabalha para o conde."} {
		if !strings.Contains(text, want) {
			t.Errorf("the package lacks %q", want)
		}
	}
	// A player's character goes too, as the table's character to hand out again: its name and sheet, never its owner.
	if !strings.Contains(text, "Personagem LEAKCANARY-char-1") {
		t.Error("the player's character is missing")
	}
}

// The characters of an imported campaign are reserved: no player sees them.
func TestImportedPlayerCharactersAreReservedAndInvisibleToPlayers(t *testing.T) {
	h := newHarness(t)
	master := h.newUser("Mestre")
	player := h.newUser("Jogadora")
	f := h.buildFixture(master)
	got := master.importPackage("m.meurpg.zip", master.export(f.campaign))
	id := got.GetCampaign().GetId()
	h.invite(master, id, player)
	list := must(player.characters.ListCharacters(t.Context(), connect.NewRequest(&charactersv1.ListCharactersRequest{CampaignId: id})))
	if len(list.GetCharacters()) != 0 {
		t.Fatalf("a player sees %d characters of the imported campaign", len(list.GetCharacters()))
	}
	if n := h.count(`SELECT count(*)::INT FROM characters WHERE campaign_id = $1 AND reserved AND player_user_id IS NULL AND kind = 'player'`, id); n != 1 {
		t.Fatalf("reserved characters = %d, want 1", n)
	}
	mine := must(master.characters.ListCharacters(t.Context(), connect.NewRequest(&charactersv1.ListCharactersRequest{CampaignId: id})))
	reservedSeen := 0
	for _, c := range mine.GetCharacters() {
		if c.GetName() == "Pensantus" {
			reservedSeen++
		}
	}
	if reservedSeen != 1 {
		t.Fatalf("the master sees Pensantus %d times", reservedSeen)
	}
	_ = mapsv1.MapPointKind_MAP_POINT_KIND_SCENE
}
