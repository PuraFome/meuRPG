package maps

import (
	"slices"
	"strconv"
	"strings"
	"testing"

	"connectrpc.com/connect"

	mapsv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/maps/v1"
	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

func (s *scenes) summaryOf(u *user, sessionID string) *playv1.SessionSummary {
	s.h.t.Helper()
	res, err := u.play.GetSessionSummary(s.h.t.Context(), connect.NewRequest(&playv1.GetSessionSummaryRequest{CampaignId: s.campaign, GameSessionId: sessionID}))
	if err != nil {
		s.h.t.Fatalf("GetSessionSummary() error = %v", err)
	}
	return res.Msg.GetSummary()
}

func (s *scenes) markFound(p *mapsv1.MapPoint, ids ...string) {
	s.h.t.Helper()
	if _, err := s.master.maps.MarkTreasureFound(s.h.t.Context(), connect.NewRequest(&mapsv1.MarkTreasureFoundRequest{
		CampaignId: s.campaign, MapId: s.mapID, PointId: p.GetId(), CharacterIds: ids,
	})); err != nil {
		s.h.t.Fatalf("MarkTreasureFound(%s) error = %v", p.GetName(), err)
	}
}

// MR-032, MR-041, D8: "Mais tesouro encontrado" is the PO each character found in
// that session, as the treasures stand now. A treasure found by two splits its
// value (rounded down); one found outside a session, one unmarked (through
// UnmarkTreasureFound) and one found in another session count for nothing; one
// converted into XP later still counts in the session it was found in. Every
// member gets the category, and a player their own number.
func TestMR032_SummaryCountsTreasureFoundInTheSession(t *testing.T) {
	t.Parallel()
	s := newScenes(t, true) // session A is open
	pens, toren := s.pens.GetId(), s.other.GetId()
	a := s.currentSession()

	bolsa := s.newTreasure("Bolsa do capitão", "", 120, 1000, 1000)
	bau := s.newTreasure("Baú de moedas", "", 251, 1100, 1000)
	calice := s.newTreasure("Cálice desmarcado", "", 400, 1200, 1000)
	colar := s.newTreasure("Colar convertido depois", "", 300, 1300, 1000)
	fora := s.newTreasure("Anel achado fora da sessão", "", 90, 1400, 1000)
	idolo := s.newTreasure("Ídolo da outra sessão", "", 77, 1500, 1000)

	s.markFound(bolsa, toren)
	s.markFound(bau, pens, toren) // 125 each: 1 PO is dropped
	s.markFound(calice, pens)
	if _, err := s.master.maps.UnmarkTreasureFound(t.Context(), connect.NewRequest(&mapsv1.UnmarkTreasureFoundRequest{CampaignId: s.campaign, MapId: s.mapID, PointId: calice.GetId()})); err != nil {
		t.Fatalf("UnmarkTreasureFound() error = %v", err)
	}
	s.markFound(colar, toren)
	s.master.end(s.campaign, a)

	// Between the sessions: found with none open, and the necklace turned into XP.
	s.markFound(fora, pens)
	var award string
	if err := s.h.pool.QueryRow(t.Context(), `
		INSERT INTO xp_awards (campaign_id, created_at, mode, reason, gold, total_xp, idempotency_key)
		VALUES ($1, now(), 'gold', 'Voltar à cidade', 300, 300, gen_random_uuid()) RETURNING id::TEXT`, s.campaign).Scan(&award); err != nil {
		t.Fatalf("insert an XP award: %v", err)
	}
	if _, err := s.h.pool.Exec(t.Context(), `UPDATE map_points SET treasure_converted_award_id = $1 WHERE id = $2`, award, colar.GetId()); err != nil {
		t.Fatalf("convert the treasure: %v", err)
	}
	b := s.master.start(s.campaign)
	s.markFound(idolo, pens)
	s.master.end(s.campaign, b)

	cats := func(sum *playv1.SessionSummary) []string {
		var out []string
		for _, c := range sum.GetCategories() {
			var who []string
			for _, w := range c.GetWinners() {
				who = append(who, w.GetName())
			}
			out = append(out, strings.TrimPrefix(c.GetKind().String(), "HIGHLIGHT_KIND_")+" "+strconv.Itoa(int(c.GetValue()))+": "+strings.Join(who, ", "))
		}
		return out
	}
	// Session A: Toren 120 + 125 + 300 = 545, Pensantus 125.
	sum := s.summaryOf(s.master, a.GetId())
	if got, want := cats(sum), []string{"TREASURE_FOUND 545: Toren"}; !slices.Equal(got, want) {
		t.Errorf("session A categories = %q, want %q", got, want)
	}
	byName := map[string]int32{}
	for _, p := range sum.GetPlayers() {
		byName[p.GetHighlights().GetName()] = p.GetTreasureFoundPo()
	}
	if len(byName) != 2 || byName["Toren"] != 545 || byName["Pensantus"] != 125 {
		t.Errorf("session A table = %v, want Toren 545 and Pensantus 125", byName)
	}
	for who, c := range map[string]struct {
		u  *user
		po int32
	}{"Toren's player": {s.caio, 545}, "Pensantus's player": {s.ana, 125}} {
		got := s.summaryOf(c.u, a.GetId())
		if cs := cats(got); !slices.Equal(cs, []string{"TREASURE_FOUND 545: Toren"}) || got.GetMine().GetTreasureFoundPo() != c.po || len(got.GetPlayers()) != 0 {
			t.Errorf("%s: categories %q, own %d PO, table %d rows; want the category, %d PO and no table", who, cs, got.GetMine().GetTreasureFoundPo(), len(got.GetPlayers()), c.po)
		}
	}
	// Session B: only the idol, and nothing of A or of the ring found outside a session.
	sumB := s.summaryOf(s.master, b.GetId())
	if got, want := cats(sumB), []string{"TREASURE_FOUND 77: Pensantus"}; !slices.Equal(got, want) {
		t.Errorf("session B categories = %q, want %q", got, want)
	}
}
