package characters

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
)

// TestStaleRevisionIsAborted: two people editing the same character cannot
// overwrite each other; the second save gets aborted (AIP-154) and changes
// nothing. A lock wins over a stale revision, because retrying would not
// help.
func TestStaleRevisionIsAborted(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	master, player := h.newUser("Mestre"), h.newUser("Jogadora")
	campaign := h.newCampaign(master, "Mirathel", player)
	read := player.createPensantus(t, campaign) // revision 1, as the player's tab has it

	if _, err := master.update(t, read, "Pensantus do mestre", pensantusSheet()); err != nil {
		t.Fatalf("master's UpdateCharacter() error = %v", err)
	}
	_, err := player.update(t, read, "Pensantus da jogadora", pensantusSheet())
	wantCode(t, "UpdateCharacter() with a stale revision", err, connect.CodeAborted)
	_, err = player.updateStory(t, read, &charactersv1.CharacterStory{Backstory: "Outra história"})
	wantCode(t, "UpdateCharacterStory() with a stale revision", err, connect.CodeAborted)
	if got := master.get(t, campaign, read.GetId()); got.GetName() != "Pensantus do mestre" || got.GetRevision() != 2 || got.GetStory().GetBackstory() != "" {
		t.Errorf("after the stale saves the character = %v, want the master's, revision 2", got)
	}

	_, err = player.api.UpdateCharacter(t.Context(), connect.NewRequest(&charactersv1.UpdateCharacterRequest{
		CampaignId: campaign, CharacterId: read.GetId(), Revision: 0, Name: "X", Sheet: pensantusSheet(),
	}))
	wantCode(t, "UpdateCharacter(revision 0)", err, connect.CodeInvalidArgument)

	h.lockSheets(campaign)
	_, err = player.update(t, read, "Pensantus", pensantusSheet())
	blocked(t, "UpdateCharacter() locked and stale", err)
	_, err = player.updateStory(t, read, &charactersv1.CharacterStory{})
	blocked(t, "UpdateCharacterStory() locked and stale", err)
}

// TestUpdateCharacterValidation: a request that breaks a rule gets
// invalid_argument naming the field, never repeating what was typed.
func TestUpdateCharacterValidation(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	master, player := h.newUser("Mestre"), h.newUser("Jogadora")
	campaign := h.newCampaign(master, "Mirathel", player)
	c := player.createPensantus(t, campaign)
	const typed = "Segredo"

	tests := []struct {
		name  string
		edit  func(req *charactersv1.UpdateCharacterRequest)
		field string
	}{
		{"ability 31", func(r *charactersv1.UpdateCharacterRequest) { r.Sheet.GetFull().BaseScores.Intelligence = 31 }, "sheet.full.base_scores.intelligence"},
		{"empty name", func(r *charactersv1.UpdateCharacterRequest) { r.Name = "   " }, "name"},
		{"long name", func(r *charactersv1.UpdateCharacterRequest) { r.Name = typed + strings.Repeat("x", MaxNameLength) }, "name"},
		{"name with a line break", func(r *charactersv1.UpdateCharacterRequest) { r.Name = typed + "\nlinha" }, "name"},
		{"no sheet", func(r *charactersv1.UpdateCharacterRequest) { r.Sheet = nil }, "sheet"},
		{"a basic sheet for a player character", func(r *charactersv1.UpdateCharacterRequest) { r.Sheet = basicSheet() }, "sheet"},
		{"unknown subrace", func(r *charactersv1.UpdateCharacterRequest) { r.Sheet.GetFull().SubraceKey = "subrace:high-elf" }, "sheet.full.subrace_key"},
		{"item typed with a tab", func(r *charactersv1.UpdateCharacterRequest) {
			r.Sheet.GetFull().Equipment[0].Name = typed + "\tx"
		}, "sheet.full.equipment[0].name"},
	}
	for _, tt := range tests {
		req := &charactersv1.UpdateCharacterRequest{
			CampaignId: campaign, CharacterId: c.GetId(), Revision: c.GetRevision(), Name: "Pensantus", Sheet: pensantusSheet(),
		}
		tt.edit(req)
		_, err := player.api.UpdateCharacter(t.Context(), connect.NewRequest(req))
		if connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Errorf("%s: error = %v, want invalid_argument", tt.name, err)
			continue
		}
		if msg := err.Error(); !strings.Contains(msg, tt.field+" ") || strings.Contains(msg, typed) {
			t.Errorf("%s: message %q should name %s and never repeat what was typed", tt.name, msg, tt.field)
		}
	}
	_, err := master.api.CreateCharacter(t.Context(), connect.NewRequest(&charactersv1.CreateCharacterRequest{CampaignId: campaign, Name: "Sem tipo", Sheet: basicSheet()}))
	wantCode(t, "CreateCharacter() without a kind", err, connect.CodeInvalidArgument)
	_, err = player.api.UpdateCharacterStory(t.Context(), connect.NewRequest(&charactersv1.UpdateCharacterStoryRequest{
		CampaignId: campaign, CharacterId: c.GetId(), Revision: c.GetRevision(),
		Story: &charactersv1.CharacterStory{Backstory: strings.Repeat("é", maxBackstoryLength+1)},
	}))
	wantCode(t, "UpdateCharacterStory() with a long backstory", err, connect.CodeInvalidArgument)
	_, err = master.api.UpdateMasterNotes(t.Context(), connect.NewRequest(&charactersv1.UpdateMasterNotesRequest{
		CampaignId: campaign, CharacterId: c.GetId(), Notes: strings.Repeat("x", MaxMasterNotesLength+1),
	}))
	wantCode(t, "UpdateMasterNotes() too long", err, connect.CodeInvalidArgument)

	if got := player.get(t, campaign, c.GetId()); got.GetRevision() != 1 {
		t.Errorf("revision after refused updates = %d, want 1", got.GetRevision())
	}
}

// TestSetStoryEditing: only for a player character, setting the same value
// changes nothing, and the revision never moves.
func TestSetStoryEditing(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	master, player := h.newUser("Mestre"), h.newUser("Jogadora")
	campaign := h.newCampaign(master, "Mirathel", player)
	c := player.createPensantus(t, campaign)
	npc := master.create(t, campaign, charactersv1.CharacterKind_CHARACTER_KIND_STORY, "Taverneiro", basicSheet())

	_, err := master.api.SetStoryEditing(t.Context(), connect.NewRequest(&charactersv1.SetStoryEditingRequest{CampaignId: campaign, CharacterId: npc.GetId(), Allowed: true}))
	wantCode(t, "SetStoryEditing(NPC)", err, connect.CodeInvalidArgument)

	once := master.setStoryEditing(t, c, true)
	twice := master.setStoryEditing(t, c, true)
	if !twice.GetStoryEditingAllowed() || twice.GetRevision() != c.GetRevision() || !twice.GetUpdatedAt().AsTime().Equal(c.GetUpdatedAt().AsTime()) {
		t.Errorf("SetStoryEditing(true) twice = %v, want allowed, with the revision and updated_at unchanged", twice)
	}
	if !once.GetCanSetStoryEditing() {
		t.Error("can_set_story_editing is false for the master")
	}
	if off := master.setStoryEditing(t, c, false); off.GetStoryEditingAllowed() {
		t.Errorf("SetStoryEditing(false) = %v, want not allowed", off)
	}
}

// TestMasterNotesBelongToTheCampaign: notes are per campaign and character,
// and a character of another campaign is "not found".
func TestMasterNotesBelongToTheCampaign(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	master, player := h.newUser("Mestre"), h.newUser("Jogadora")
	campaign := h.newCampaign(master, "Mirathel", player)
	other := h.newCampaign(master, "Outra")
	npc := master.create(t, campaign, charactersv1.CharacterKind_CHARACTER_KIND_BOSS, "Strahd", enemySheet())

	res, err := master.api.GetMasterNotes(t.Context(), connect.NewRequest(&charactersv1.GetMasterNotesRequest{CampaignId: campaign, CharacterId: npc.GetId()}))
	if err != nil || res.Msg.GetNotes() != "" || res.Msg.GetUpdatedAt() != nil {
		t.Errorf("GetMasterNotes() with none written = %v, %v; want empty", res, err)
	}
	_, err = master.api.GetMasterNotes(t.Context(), connect.NewRequest(&charactersv1.GetMasterNotesRequest{CampaignId: other, CharacterId: npc.GetId()}))
	wantCode(t, "GetMasterNotes(character of another campaign)", err, connect.CodeNotFound)
	_, err = master.api.UpdateMasterNotes(t.Context(), connect.NewRequest(&charactersv1.UpdateMasterNotesRequest{CampaignId: other, CharacterId: npc.GetId(), Notes: "x"}))
	wantCode(t, "UpdateMasterNotes(character of another campaign)", err, connect.CodeNotFound)
	_, err = master.api.UpdateMasterNotes(t.Context(), connect.NewRequest(&charactersv1.UpdateMasterNotesRequest{CampaignId: campaign, CharacterId: "pensantus", Notes: "x"}))
	wantCode(t, "UpdateMasterNotes(not a UUID)", err, connect.CodeNotFound)
}

// TestListContent: any member gets the SRD catalog, with the attribution
// the credits page shows.
func TestListContent(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	master, player := h.newUser("Mestre"), h.newUser("Jogadora")
	campaign := h.newCampaign(master, "Mirathel", player)

	res, err := player.content.ListContent(t.Context(), connect.NewRequest(&rulesv1.ListContentRequest{CampaignId: campaign}))
	if err != nil {
		t.Fatalf("ListContent() error = %v", err)
	}
	c := res.Msg.GetContent()
	if c.GetContentVersion() != loadRules(t).Version() || !strings.Contains(c.GetAttribution(), "Creative Commons") ||
		len(c.GetAbilities()) != 6 || len(c.GetClasses()) != 12 || len(c.GetRaces()) != 9 || len(c.GetSpells()) == 0 {
		t.Errorf("ListContent() = version %q, %d abilities, %d classes, %d races, %d spells; want the SRD 5.1",
			c.GetContentVersion(), len(c.GetAbilities()), len(c.GetClasses()), len(c.GetRaces()), len(c.GetSpells()))
	}
}

// TestUniqueViolationIsRecognized: the 23505 that CockroachDB raises for
// the RN-03 index is the one the handler maps to LIVING_CHARACTER_EXISTS.
func TestUniqueViolationIsRecognized(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	master, player := h.newUser("Mestre"), h.newUser("Jogadora")
	campaign := h.newCampaign(master, "Mirathel", player)
	player.createPensantus(t, campaign)

	_, err := h.pool.Exec(t.Context(), `
		INSERT INTO characters (campaign_id, kind, player_user_id, name, sheet, created_at, updated_at)
		VALUES ($1, 'player', $2, 'Segundo', '{}', now(), now())`, campaign, player.id)
	if !isUniqueViolation(err, "characters_one_living_player_character") {
		t.Errorf("second living character error = %v, want the characters_one_living_player_character violation", err)
	}
	if isUniqueViolation(err, "game_sessions_one_open_per_campaign") {
		t.Error("isUniqueViolation() matched another index")
	}
}

// TestOrphanedPlayerCharactersAreDeletedByTheDatabase: a player character
// with neither a player nor a campaign is expired for CockroachDB's
// row-level TTL job (migration 00020, docs/privacidade.md), and no other
// row is. The job itself runs once a day, so the test reads the table's TTL
// setting and evaluates that same expression over real rows.
func TestOrphanedPlayerCharactersAreDeletedByTheDatabase(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	masterA, masterB := h.newUser("Mestre A"), h.newUser("Mestre B")
	stays, leaves, staysB, leavesB := h.newUser("Fica"), h.newUser("Sai"), h.newUser("Fica B"), h.newUser("Sai B")
	a := h.newCampaign(masterA, "Mirathel", stays, leaves)
	b := h.newCampaign(masterB, "Barovia", staysB, leavesB)
	kept := map[string]string{
		stays.createPensantus(t, a).GetId():  "a player character with its player and campaign",
		leaves.createPensantus(t, a).GetId(): "a player character whose player left (RN-16)",
		staysB.createPensantus(t, b).GetId(): "a player character whose campaign was deleted",
		masterA.create(t, a, charactersv1.CharacterKind_CHARACTER_KIND_MINION, "Goblin", basicSheet()).GetId(): "an NPC",
	}
	orphan := leavesB.createPensantus(t, b).GetId()
	h.deleteUser(leaves.id)
	h.deleteUser(masterB.id) // takes campaign b with it
	h.deleteUser(leavesB.id)

	var options []string
	if err := h.pool.QueryRow(t.Context(), "SELECT reloptions FROM pg_class WHERE relname = 'characters'").Scan(&options); err != nil {
		t.Fatalf("read the table's options: %v", err)
	}
	var ttlOn bool
	var expression string
	for _, o := range options {
		switch {
		case o == "ttl='on'":
			ttlOn = true
		case strings.HasPrefix(o, "ttl_expiration_expression="):
			// Stored as an escaped string literal: e'CASE WHEN kind = \'player\' ...'
			v := strings.TrimPrefix(o, "ttl_expiration_expression=")
			v = strings.TrimSuffix(strings.TrimPrefix(strings.TrimPrefix(v, "e"), "'"), "'")
			expression = strings.ReplaceAll(v, `\'`, `'`)
		}
	}
	if !ttlOn || expression == "" {
		t.Fatalf("characters has no row-level TTL with an expiration expression: %q", options)
	}

	rows, err := h.pool.Query(t.Context(), fmt.Sprintf("SELECT id, (%s) FROM characters", expression))
	if err != nil {
		t.Fatalf("evaluate %q: %v", expression, err)
	}
	defer rows.Close()
	seen := 0
	for rows.Next() {
		var id string
		var expires *time.Time
		if err := rows.Scan(&id, &expires); err != nil {
			t.Fatalf("scan: %v", err)
		}
		seen++
		switch {
		case id == orphan && (expires == nil || !expires.Before(time.Now())):
			t.Errorf("the orphaned character expires at %v, want a time in the past", expires)
		case id != orphan && expires != nil:
			t.Errorf("%s expires at %v, want never", kept[id], expires)
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("read rows: %v", err)
	}
	if seen != len(kept)+1 {
		t.Errorf("evaluated %d rows, want %d", seen, len(kept)+1)
	}
}
