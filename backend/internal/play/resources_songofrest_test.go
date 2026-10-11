package play

import (
	"testing"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/proto"

	campaignsv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/campaigns/v1"
	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
)

// Song of Rest (SRD 5.1, Bard 2): during a short rest, a creature that hears a bard's
// performance and regains hit points by spending hit dice regains an extra die, once for
// the rest. The table's rule for "can hear" is the party that rests with a bard who has at
// least 1 hit point. These tests need the database (MEURPG_TEST_DATABASE_URL).

// songTable is a party of a fighter (Ana) and a bard (Bia), a session open.
type songTable struct {
	*restTable
	bard *charactersv1.Character
}

func newSongTable(t *testing.T, bardLevel int32) *songTable {
	t.Helper()
	h := newHarness(t)
	r := &restTable{h: h, master: h.newUser("Mestre"), ana: h.newUser("Ana"), bia: h.newUser("Bia")}
	r.campaign = h.newCampaign(r.master, "Mirathel", r.ana, r.bia)
	r.fighter = r.ana.hero(t, r.campaign, "Toren", "class:fighter", "race:human", 4, abilities(16, 13, 14, 10, 10), []string{battleaxe}, nil)
	bard := r.bia.hero(t, r.campaign, "Orla", "class:bard", "race:human", bardLevel,
		&rulesv1.AbilityScores{Strength: 8, Dexterity: 14, Constitution: 14, Intelligence: 10, Wisdom: 10, Charisma: 16}, nil, nil)
	r.session = r.master.start(t, r.campaign).GetGameSession()
	return &songTable{restTable: r, bard: bard}
}

func typedFaces(face, song int32) func(*playv1.SpendHitDiceRequest) {
	return func(q *playv1.SpendHitDiceRequest) {
		q.Roll = &playv1.SpendHitDiceRequest_TypedFace{TypedFace: face}
		q.TypedSongFace = song
	}
}

func (s *songTable) hurt(t *testing.T, c *charactersv1.Character, hp int32) {
	t.Helper()
	s.use(t, c, func(q *playv1.AdjustCharacterVitalsRequest) { q.HitPointsCurrent = proto.Int32(hp) })
}

func (s *songTable) shortRest(t *testing.T) {
	t.Helper()
	if _, err := s.rest(t, playv1.RestKind_REST_KIND_SHORT); err != nil {
		t.Fatalf("TakeRest(short) error = %v", err)
	}
}

// The first die a creature spends in a short rest, with a bard of the party, brings the
// song's die; the next ones do not, and a new short rest brings it again.
func TestSongOfRestAddsADieToTheFirstHitDieOfTheShortRest(t *testing.T) {
	t.Parallel()
	s := newSongTable(t, 2)
	s.hurt(t, s.fighter, 3)
	s.shortRest(t)
	s.h.roller.queue(7, 4) // the d10, then the d6 of the song
	res, err := s.spend(t, s.ana, s.fighter, 10)
	if err != nil {
		t.Fatalf("SpendHitDice() error = %v", err)
	}
	// Constitution 14 is +2: 7 + 2 + 4 = 13.
	if res.GetFace() != 7 || res.GetSongOfRestSides() != 6 || res.GetSongOfRestFace() != 4 || res.GetHealed() != 13 || res.GetVitals().GetHitPointsCurrent() != 16 {
		t.Errorf("face %d, song d%d = %d, healed %d, hit points %d; want 7, d6 = 4, 13 and 16",
			res.GetFace(), res.GetSongOfRestSides(), res.GetSongOfRestFace(), res.GetHealed(), res.GetVitals().GetHitPointsCurrent())
	}
	// Once for the rest: the second die is the plain 5 + 2.
	s.h.roller.queue(5)
	res, err = s.spend(t, s.ana, s.fighter, 10)
	if err != nil || res.GetSongOfRestSides() != 0 || res.GetHealed() != 7 {
		t.Errorf("second die = %v, %v; want no song and 7", res, err)
	}
	// Another short rest, another song.
	s.shortRest(t)
	s.h.roller.queue(1, 6)
	res, err = s.spend(t, s.ana, s.fighter, 10)
	if err != nil || res.GetSongOfRestFace() != 6 || res.GetHealed() != 9 {
		t.Errorf("next rest = %v, %v; want the song again and 1 + 2 + 6", res, err)
	}
	// The event keeps the song, for the history.
	var withSong int
	if err := s.h.pool.QueryRow(t.Context(), `SELECT count(*) FROM session_events WHERE game_session_id = $1 AND kind = $2 AND character_id = $3 AND payload ->> 'song_sides' IS NOT NULL`,
		s.session.GetId(), eventHitDiceSpent, s.fighter.GetId()).Scan(&withSong); err != nil || withSong != 2 {
		t.Errorf("hit_dice_spent events with a song = %d (%v), want 2", withSong, err)
	}
}

// The bard hears itself, and the die grows with the bard's level: d8 at 9, d12 at 17.
func TestTheBardHearsItselfAndTheDieGrowsWithItsLevel(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		level int32
		sides int32
	}{{2, 6}, {9, 8}, {13, 10}, {17, 12}} {
		s := newSongTable(t, tc.level)
		s.hurt(t, s.bard, 1)
		s.shortRest(t)
		s.h.roller.queue(3, 1)
		res, err := s.spend(t, s.bia, s.bard, 8)
		if err != nil || res.GetSongOfRestSides() != tc.sides || res.GetSongOfRestFace() != 1 {
			t.Errorf("bard %d: %v, %v; want the song's d%d", tc.level, res, err, tc.sides)
		}
	}
}

// There is no song without a short rest in the session, after a long rest, with no bard in
// the party, or when the bard is unconscious (0 hit points).
func TestThereIsNoSongOutsideAShortRestOrWithoutASinger(t *testing.T) {
	t.Parallel()
	s := newSongTable(t, 2)
	s.hurt(t, s.fighter, 3)
	// No rest taken yet.
	s.h.roller.queue(5)
	if res, err := s.spend(t, s.ana, s.fighter, 10); err != nil || res.GetSongOfRestSides() != 0 || res.GetHealed() != 7 {
		t.Errorf("before any rest: %v, %v; want no song", res, err)
	}
	// The latest rest is a long one.
	if _, err := s.rest(t, playv1.RestKind_REST_KIND_LONG); err != nil {
		t.Fatalf("TakeRest(long) error = %v", err)
	}
	s.hurt(t, s.fighter, 3)
	s.h.roller.queue(5)
	if res, err := s.spend(t, s.ana, s.fighter, 10); err != nil || res.GetSongOfRestSides() != 0 {
		t.Errorf("after a long rest: %v, %v; want no song", res, err)
	}
	// A short rest, but the bard is at 0 hit points.
	s.shortRest(t)
	s.hurt(t, s.bard, 0)
	s.h.roller.queue(5)
	if res, err := s.spend(t, s.ana, s.fighter, 10); err != nil || res.GetSongOfRestSides() != 0 {
		t.Errorf("an unconscious bard: %v, %v; want no song", res, err)
	}
	// The bard wakes up: the song is heard (the fighter has not had it in this rest).
	s.hurt(t, s.bard, 4)
	s.h.roller.queue(5, 2)
	if res, err := s.spend(t, s.ana, s.fighter, 10); err != nil || res.GetSongOfRestSides() != 6 || res.GetSongOfRestFace() != 2 {
		t.Errorf("a bard back on its feet: %v, %v; want the song", res, err)
	}
}

// A party without a bard has no song.
func TestAPartyWithoutABardHasNoSong(t *testing.T) {
	t.Parallel()
	r := newRestTable(t)
	if _, err := r.rest(t, playv1.RestKind_REST_KIND_SHORT); err != nil {
		t.Fatalf("TakeRest(short) error = %v", err)
	}
	r.h.roller.queue(5)
	if res, err := r.spend(t, r.ana, r.fighter, 10); err != nil || res.GetSongOfRestSides() != 0 {
		t.Errorf("no bard: %v, %v; want no song", res, err)
	}
}

// With real dice the face of the song's die is typed too: the call is refused, saying the
// die's size, until it comes, and the face must be on the die.
func TestTheSongsDieRolledWithARealDieIsTyped(t *testing.T) {
	t.Parallel()
	s := newSongTable(t, 9)
	s.hurt(t, s.fighter, 3)
	s.shortRest(t)
	if _, err := s.master.campaigns.SetCampaignDiceMode(t.Context(), connect.NewRequest(&campaignsv1.SetCampaignDiceModeRequest{CampaignId: s.campaign, Mode: campaignsv1.DiceMode_DICE_MODE_PHYSICAL})); err != nil {
		t.Fatalf("SetCampaignDiceMode() error = %v", err)
	}
	_, err := s.spend(t, s.ana, s.fighter, 10, typedFaces(6, 0))
	if b := wantResourceBlocked(t, err, playv1.ResourceBlockedReason_RESOURCE_BLOCKED_REASON_SONG_OF_REST_FACE_NEEDED); b.GetNeeded() != 8 {
		t.Errorf("needed = %d, want the d8", b.GetNeeded())
	}
	if _, err := s.spend(t, s.ana, s.fighter, 10, typedFaces(6, 9)); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Errorf("a face of 9 on a d8 = %v, want invalid_argument", err)
	}
	// Nothing was spent by the refusals.
	if v := s.vitals(t, s.master, s.fighter); v.GetHitDiceUsed() != 0 || v.GetHitPointsCurrent() != 3 {
		t.Errorf("after the refusals: %d hit dice spent, %d hit points; want 0 and 3", v.GetHitDiceUsed(), v.GetHitPointsCurrent())
	}
	res, err := s.spend(t, s.ana, s.fighter, 10, typedFaces(6, 8))
	if err != nil || res.GetSongOfRestSides() != 8 || res.GetSongOfRestFace() != 8 || res.GetHealed() != 6+2+8 {
		t.Errorf("typed = %v, %v; want the song's 8 and 16 hit points", res, err)
	}
	// The song was had: the next die needs no song face.
	if res, err := s.spend(t, s.ana, s.fighter, 10, typedFaces(4, 0)); err != nil || res.GetSongOfRestSides() != 0 {
		t.Errorf("second typed die = %v, %v; want no song and no face asked", res, err)
	}
}

// The extra die never takes the character past its maximum, and a retry of the same call
// (the same key) neither rolls nor spends twice.
func TestTheSongStopsAtTheMaximumAndARetryIsTheSameDie(t *testing.T) {
	t.Parallel()
	s := newSongTable(t, 2)
	hpMax := s.vitals(t, s.master, s.fighter).GetHitPointsMax()
	s.hurt(t, s.fighter, hpMax-1)
	s.shortRest(t)
	s.h.roller.queue(9, 6)
	key := newKey()
	withKey := func(q *playv1.SpendHitDiceRequest) { q.IdempotencyKey = key }
	res, err := s.spend(t, s.ana, s.fighter, 10, withKey)
	if err != nil || res.GetHealed() != 1 || res.GetVitals().GetHitPointsCurrent() != hpMax || res.GetSongOfRestFace() != 6 {
		t.Fatalf("SpendHitDice() = %v, %v; want 1 hit point healed, the song's 6 rolled", res, err)
	}
	again, err := s.spend(t, s.ana, s.fighter, 10, withKey)
	if err != nil || again.GetSongOfRestFace() != 6 || again.GetHealed() != 1 || again.GetVitals().GetHitDiceUsed() != 1 {
		t.Errorf("the retry = %v, %v; want the same answer and one die spent", again, err)
	}
}
