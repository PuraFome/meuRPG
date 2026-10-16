package play

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"

	maplink "github.com/PuraFome/meuRPG/backend/internal/maps/link"
	"github.com/PuraFome/meuRPG/backend/internal/play/link"
	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
	"github.com/PuraFome/meuRPG/backend/internal/rules/grid"
	"github.com/PuraFome/meuRPG/backend/internal/rules/vision"
)

// These tests freeze reactorSees (combat_reaction_counterspell.go): whether a reactor sees the
// combatant that triggered a window, and how far. They need no database: the sight and the roster
// are the small fakes below.

// sightFake is the sight of a map: the squares the players see and what a pair sees.
type sightFake struct {
	maplink.CombatSight
	seen     map[grid.Square]bool
	canSee   bool
	asked    []vision.Senses
	askedAt  []grid.Square
	askedFor []grid.Square
}

func (f *sightFake) Sees(_ string, sq grid.Square) bool { return f.seen[sq] }

func (f *sightFake) CanSee(from grid.Square, senses vision.Senses, to grid.Square) bool {
	f.asked, f.askedAt, f.askedFor = append(f.asked, senses), append(f.askedAt, from), append(f.askedFor, to)
	return f.canSee
}

// sheetRosterFake gives every character the same senses, or an error.
type sheetRosterFake struct {
	CombatRoster
	darkvision int
	err        error
}

func (r sheetRosterFake) CombatSheet(context.Context, pgx.Tx, string, string) (link.Sheet, error) {
	return link.Sheet{Senses: link.Senses{DarkvisionFt: r.darkvision}}, r.err
}

func cellPtrs(col, row int32) (*int32, *int32) { return &col, &row }

func combatantAt(id, kind string, col, row int32) playdb.Combatant {
	c, r := cellPtrs(col, row)
	return playdb.Combatant{ID: id, Kind: kind, GridCol: c, GridRow: r, CharacterID: "char-" + id}
}

func TestReactorSeesFreezesWhoSeesWhomAndHowFar(t *testing.T) {
	t.Parallel()
	user := "user-1"
	player := func(id string, col, row int32) playdb.Combatant {
		c := combatantAt(id, kindPlayer, col, row)
		c.UserID = &user
		return c
	}
	hidden := func(c playdb.Combatant) playdb.Combatant { c.Hidden = true; return c }
	with := func(c playdb.Combatant, conditions ...string) playdb.Combatant { c.Conditions = conditions; return c }
	unplaced := func(c playdb.Combatant) playdb.Combatant { c.GridCol, c.GridRow = nil, nil; return c }

	tests := []struct {
		name         string
		reactor      playdb.Combatant
		actor        playdb.Combatant
		theatre      bool
		sight        *sightFake // nil: no fog
		darkvision   int
		sheetErr     error
		wantDist     int32
		wantSees     bool
		wantErr      bool
		wantCanSeeAt int // how many pairs the sight was asked about
	}{
		{name: "a blind reactor sees nothing", reactor: with(combatantAt("r", kindNPC, 0, 0), "condition:blinded"), actor: combatantAt("a", kindPlayer, 1, 0)},
		{name: "a hidden NPC is seen by no player", reactor: player("r", 0, 0), actor: hidden(combatantAt("a", kindNPC, 1, 0))},
		{name: "a hidden NPC is seen by another NPC", reactor: combatantAt("r", kindNPC, 0, 0), actor: hidden(combatantAt("a", kindNPC, 1, 0)), wantDist: 5, wantSees: true},
		{name: "a hidden player is seen", reactor: combatantAt("r", kindNPC, 0, 0), actor: hidden(combatantAt("a", kindPlayer, 1, 0)), wantDist: 5, wantSees: true},
		{name: "an invisible actor is seen by no one", reactor: combatantAt("r", kindNPC, 0, 0), actor: with(combatantAt("a", kindPlayer, 1, 0), "condition:invisible")},
		{name: "60 feet are in range", reactor: combatantAt("r", kindNPC, 0, 0), actor: combatantAt("a", kindPlayer, 12, 0), wantDist: 60, wantSees: true},
		{name: "65 feet are not", reactor: combatantAt("r", kindNPC, 0, 0), actor: combatantAt("a", kindPlayer, 13, 0)},
		{name: "a combatant with no square is in range at no distance", reactor: combatantAt("r", kindNPC, 0, 0), actor: unplaced(combatantAt("a", kindPlayer, 40, 0)), wantSees: true},
		{name: "a theatre has no distance", reactor: combatantAt("r", kindNPC, 0, 0), actor: combatantAt("a", kindPlayer, 40, 0), theatre: true, wantSees: true},
		{
			name: "a player reactor sees an NPC the fog shows", reactor: player("r", 0, 0), actor: combatantAt("a", kindNPC, 3, 0),
			sight: &sightFake{seen: map[grid.Square]bool{{Col: 3, Row: 0}: true}}, wantDist: 15, wantSees: true,
		},
		{
			name: "a player reactor does not see an NPC the fog hides", reactor: player("r", 0, 0), actor: combatantAt("a", kindNPC, 3, 0),
			sight: &sightFake{seen: map[grid.Square]bool{}}, wantDist: 15,
		},
		{
			name: "a player reactor sees a player in the fog", reactor: player("r", 0, 0), actor: combatantAt("a", kindPlayer, 3, 0),
			sight: &sightFake{seen: map[grid.Square]bool{}}, wantDist: 15, wantSees: true,
		},
		{
			name: "a player reactor with no user falls to the plain answer", reactor: combatantAt("r", kindPlayer, 0, 0), actor: combatantAt("a", kindNPC, 3, 0),
			sight: &sightFake{seen: map[grid.Square]bool{}}, wantDist: 15, wantSees: true,
		},
		{
			name: "an NPC reactor sees with its senses", reactor: combatantAt("r", kindNPC, 0, 0), actor: combatantAt("a", kindPlayer, 3, 0),
			sight: &sightFake{canSee: true}, darkvision: 60, wantDist: 15, wantSees: true, wantCanSeeAt: 1,
		},
		{
			name: "an NPC reactor that the light does not reach", reactor: combatantAt("r", kindNPC, 0, 0), actor: combatantAt("a", kindPlayer, 3, 0),
			sight: &sightFake{canSee: false}, wantDist: 15, wantCanSeeAt: 1,
		},
		{
			name: "an NPC reactor whose sheet cannot be read", reactor: combatantAt("r", kindNPC, 0, 0), actor: combatantAt("a", kindPlayer, 3, 0),
			sight: &sightFake{canSee: true}, sheetErr: errors.New("no sheet"), wantErr: true,
		},
		{
			name: "an NPC reactor with no square is not asked", reactor: unplaced(combatantAt("r", kindNPC, 0, 0)), actor: combatantAt("a", kindPlayer, 3, 0),
			sight: &sightFake{canSee: false}, wantSees: true,
		},
		{
			name: "an NPC reactor and an actor with no square are not asked", reactor: combatantAt("r", kindNPC, 0, 0), actor: unplaced(combatantAt("a", kindPlayer, 3, 0)),
			sight: &sightFake{canSee: false}, wantSees: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			s := &Service{roster: sheetRosterFake{darkvision: tc.darkvision, err: tc.sheetErr}}
			c := &combatTx{session: playdb.GameSession{CampaignID: "campaign"}}
			if tc.theatre {
				c.enc.Mode = modeTheatre
			}
			if tc.sight != nil {
				c.sight = &fogSight{sight: tc.sight}
			}
			cs := []playdb.Combatant{tc.reactor, tc.actor}
			dist, sees, err := s.reactorSees(t.Context(), c, cs, tc.reactor, tc.actor)
			if (err != nil) != tc.wantErr {
				t.Fatalf("reactorSees() error = %v, want an error: %v", err, tc.wantErr)
			}
			if dist != tc.wantDist || sees != tc.wantSees {
				t.Errorf("reactorSees() = %d ft, %v; want %d ft, %v", dist, sees, tc.wantDist, tc.wantSees)
			}
			if tc.sight != nil && len(tc.sight.asked) != tc.wantCanSeeAt {
				t.Fatalf("the sight was asked about %d pairs, want %d", len(tc.sight.asked), tc.wantCanSeeAt)
			}
			if tc.wantCanSeeAt == 1 && !tc.wantErr {
				got := tc.sight.asked[0]
				if got.DarkvisionFt != tc.darkvision || tc.sight.askedAt[0] != squareOfCombatant(tc.reactor) || tc.sight.askedFor[0] != squareOfCombatant(tc.actor) {
					t.Errorf("the sight was asked %v from %v to %v", got, tc.sight.askedAt[0], tc.sight.askedFor[0])
				}
			}
		})
	}
}
