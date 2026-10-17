package play

import (
	"errors"
	"strings"
	"testing"

	"connectrpc.com/connect"

	mapsv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/maps/v1"
	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
)

// What the master writes in a puzzle, frozen refusal by refusal: the code, the reason,
// the field and the message, and the order the checks run in (the first refusal wins).

// puzzleRefusal reads the field of the PuzzleInvalid detail of err.
func puzzleRefusalField(err error) string {
	var ce *connect.Error
	if !errors.As(err, &ce) {
		return ""
	}
	for _, d := range ce.Details() {
		if v, err := d.Value(); err == nil {
			if b, ok := v.(*playv1.PuzzleInvalid); ok {
				return b.GetField()
			}
		}
	}
	return ""
}

// wantRulesRefusal checks the whole refusal: INVALID_ARGUMENT, the reason, the field
// and the exact message.
func wantRulesRefusal(t *testing.T, call string, err error, reason playv1.PuzzleInvalidReason, field, msg string) {
	t.Helper()
	wantPuzzleInvalid(t, call, err, reason)
	var ce *connect.Error
	if !errors.As(err, &ce) {
		t.Fatalf("%s error = %v, want a connect error", call, err)
	}
	if ce.Message() != msg {
		t.Errorf("%s message = %q, want %q", call, ce.Message(), msg)
	}
	if got := puzzleRefusalField(err); got != field {
		t.Errorf("%s field = %q, want %q", call, got, field)
	}
}

type rulesCase struct {
	name   string
	edit   func(*playv1.CreatePuzzleRequest)
	reason playv1.PuzzleInvalidReason
	field  string
	msg    string
}

func (p *puzzleTable) rulesRiddle() *playv1.CreatePuzzleRequest {
	return &playv1.CreatePuzzleRequest{CampaignId: p.campaignID, Name: "Enigma", Config: riddleConfig(), Solution: riddleSolution("sombra")}
}

func (p *puzzleTable) wantRulesCases(t *testing.T, base func() *playv1.CreatePuzzleRequest, cases []rulesCase) {
	t.Helper()
	for _, tc := range cases {
		req := base()
		tc.edit(req)
		_, err := p.pc(p.master).CreatePuzzle(t.Context(), connect.NewRequest(req))
		wantRulesRefusal(t, tc.name, err, tc.reason, tc.field, tc.msg)
	}
}

func TestPuzzleRules_OnWrongRefusals(t *testing.T) {
	t.Parallel()
	p := newPuzzleTable(t)
	trap := p.trapPoint(t, "Dardos", 12, 4)
	scene, err := p.mc(p.master).CreateMapPoint(t.Context(), connect.NewRequest(&mapsv1.CreateMapPointRequest{
		CampaignId: p.campaignID, MapId: p.mapID, Kind: mapsv1.MapPointKind_MAP_POINT_KIND_SCENE, Name: "Cena", XBp: 100, YBp: 100,
	}))
	if err != nil {
		t.Fatal(err)
	}
	reason := playv1.PuzzleInvalidReason_PUZZLE_INVALID_REASON_ON_WRONG
	notTrap := "on_wrong.trap is not a trap point of this campaign"
	target := func(mapID, pointID string) *playv1.PuzzleTrapTarget {
		return &playv1.PuzzleTrapTarget{MapId: mapID, PointId: pointID}
	}
	with := func(o *playv1.PuzzleOnWrong) func(*playv1.CreatePuzzleRequest) {
		return func(r *playv1.CreatePuzzleRequest) { r.OnWrong = o }
	}
	lockKind := func(r *playv1.CreatePuzzleRequest) {
		r.Config, r.Solution, r.Start = lockConfig(2, playv1.PuzzleAlphabet_PUZZLE_ALPHABET_DIGITS), lockSolution(1, 1), lockStart(0, 0)
	}
	lightsKind := func(r *playv1.CreatePuzzleRequest) { r.Config, r.Solution, r.Start = lightsConfig(3), nil, nil }
	both := func(a, b func(*playv1.CreatePuzzleRequest)) func(*playv1.CreatePuzzleRequest) {
		return func(r *playv1.CreatePuzzleRequest) { a(r); b(r) }
	}
	p.wantRulesCases(t, p.rulesRiddle, []rulesCase{
		{"attempts 11", with(&playv1.PuzzleOnWrong{AttemptsPerPlayer: 11}), reason, "on_wrong.attempts_per_player", "attempts_per_player must be 1 to 10"},
		{"attempts -1", with(&playv1.PuzzleOnWrong{AttemptsPerPlayer: -1}), reason, "on_wrong.attempts_per_player", "attempts_per_player must be 1 to 10"},
		{"attempts out of range wins over a kind that does not judge", both(lightsKind, with(&playv1.PuzzleOnWrong{AttemptsPerPlayer: 11})), reason, "on_wrong.attempts_per_player", "attempts_per_player must be 1 to 10"},
		{"attempts on the lights", both(lightsKind, with(&playv1.PuzzleOnWrong{AttemptsPerPlayer: 3})), reason, "on_wrong.attempts_per_player", "this kind has no wrong move to spend an attempt"},
		{"moves 201", with(&playv1.PuzzleOnWrong{MaxMoves: 201}), reason, "on_wrong.max_moves", "max_moves must be 1 to 200"},
		{"moves -3", with(&playv1.PuzzleOnWrong{MaxMoves: -3}), reason, "on_wrong.max_moves", "max_moves must be 1 to 200"},
		{"time 9", with(&playv1.PuzzleOnWrong{TimeLimitSeconds: 9}), reason, "on_wrong.time_limit_seconds", "time_limit_seconds must be 10 to 14400"},
		{"time 14401", with(&playv1.PuzzleOnWrong{TimeLimitSeconds: 14401}), reason, "on_wrong.time_limit_seconds", "time_limit_seconds must be 10 to 14400"},
		{"time -1", with(&playv1.PuzzleOnWrong{TimeLimitSeconds: -1}), reason, "on_wrong.time_limit_seconds", "time_limit_seconds must be 10 to 14400"},
		// The order: attempts, moves, time, then the trap.
		{"attempts before moves", with(&playv1.PuzzleOnWrong{AttemptsPerPlayer: 11, MaxMoves: 201, TimeLimitSeconds: 9}), reason, "on_wrong.attempts_per_player", "attempts_per_player must be 1 to 10"},
		{"moves before time", with(&playv1.PuzzleOnWrong{MaxMoves: 201, TimeLimitSeconds: 9}), reason, "on_wrong.max_moves", "max_moves must be 1 to 200"},
		{"time before the trap", with(&playv1.PuzzleOnWrong{TimeLimitSeconds: 9, Trap: target("x", "y")}), reason, "on_wrong.time_limit_seconds", "time_limit_seconds must be 10 to 14400"},
		{"a trap on a lock", both(lockKind, with(&playv1.PuzzleOnWrong{Trap: target(p.mapID, trap.GetId())})), reason, "on_wrong.trap", "this kind has no wrong move to fire a trap"},
		{"the kind before the ids", both(lockKind, with(&playv1.PuzzleOnWrong{Trap: &playv1.PuzzleTrapTarget{}})), reason, "on_wrong.trap", "this kind has no wrong move to fire a trap"},
		{"a trap without ids", with(&playv1.PuzzleOnWrong{Trap: &playv1.PuzzleTrapTarget{}}), reason, "on_wrong.trap", "on_wrong.trap must name a map and a trap point"},
		{"a trap with a map only", with(&playv1.PuzzleOnWrong{Trap: target(p.mapID, "")}), reason, "on_wrong.trap", "on_wrong.trap must name a map and a trap point"},
		{"a trap with a point only", with(&playv1.PuzzleOnWrong{Trap: target("", trap.GetId())}), reason, "on_wrong.trap", "on_wrong.trap must name a map and a trap point"},
		{"a trap that is not a UUID", with(&playv1.PuzzleOnWrong{Trap: target("map", "point")}), reason, "on_wrong.trap", "on_wrong.trap must name a map and a trap point"},
		{"a map that is not there", with(&playv1.PuzzleOnWrong{Trap: target(newKey(), trap.GetId())}), reason, "on_wrong.trap", notTrap},
		{"a point that is not there", with(&playv1.PuzzleOnWrong{Trap: target(p.mapID, newKey())}), reason, "on_wrong.trap", notTrap},
		{"a scene point", with(&playv1.PuzzleOnWrong{Trap: target(p.mapID, scene.Msg.GetPoint().GetId())}), reason, "on_wrong.trap", notTrap},
	})
	// The good cases: the bounds, the ids written in capitals, a trap on the lights is
	// refused but the numbers alone are fine.
	got := p.riddle(t, "Limites", onWrong(func(o *playv1.PuzzleOnWrong) {
		o.AttemptsPerPlayer, o.MaxMoves, o.TimeLimitSeconds = 1, 200, 14400
	}))
	if o := got.GetOnWrong(); o.GetAttemptsPerPlayer() != 1 || o.GetMaxMoves() != 200 || o.GetTimeLimitSeconds() != 14400 || o.GetTrap() != nil {
		t.Errorf("the bounds read %v", o)
	}
	upper := p.riddle(t, "Maiusculas", onWrong(onWrongTrap(target(strings.ToUpper(p.mapID), strings.ToUpper(trap.GetId())))))
	if tr := upper.GetOnWrong().GetTrap(); tr.GetMapId() != p.mapID || tr.GetPointId() != trap.GetId() {
		t.Errorf("a trap written in capitals reads %v, want the lower case ids", tr)
	}
	// An edit goes through the same checks.
	_, err = p.pc(p.master).UpdatePuzzle(t.Context(), connect.NewRequest(&playv1.UpdatePuzzleRequest{
		CampaignId: p.campaignID, PuzzleId: got.GetId(), Name: "Limites", Config: riddleConfig(), Solution: riddleSolution("sombra"),
		OnWrong: &playv1.PuzzleOnWrong{MaxMoves: 201},
	}))
	wantRulesRefusal(t, "an edit", err, reason, "on_wrong.max_moves", "max_moves must be 1 to 200")
}

func TestPuzzleRules_OnWrongWithNoTrapBook(t *testing.T) {
	t.Parallel()
	p := newPuzzleTable(t)
	trap := p.trapPoint(t, "Dardos", 12, 4)
	p.h.svc.SetTraps(nil)
	req := p.rulesRiddle()
	req.OnWrong = &playv1.PuzzleOnWrong{Trap: &playv1.PuzzleTrapTarget{MapId: p.mapID, PointId: trap.GetId()}}
	_, err := p.pc(p.master).CreatePuzzle(t.Context(), connect.NewRequest(req))
	wantCode(t, "a trap with no trap book", err, connect.CodeUnavailable)
	if got := err.Error(); !strings.Contains(got, "traps are not available on this server") {
		t.Errorf("message = %q", got)
	}
	// With no trap book the order still holds: a kind that does not judge is refused first,
	// and the numbers alone need no book.
	req = p.rulesRiddle()
	req.OnWrong = &playv1.PuzzleOnWrong{Trap: &playv1.PuzzleTrapTarget{}}
	_, err = p.pc(p.master).CreatePuzzle(t.Context(), connect.NewRequest(req))
	wantCode(t, "a trap with no ids and no book", err, connect.CodeUnavailable)
	p.riddle(t, "Sem armadilha", onWrong(func(o *playv1.PuzzleOnWrong) { o.AttemptsPerPlayer = 2 }))
}

func TestPuzzleRules_PartsRefusals(t *testing.T) {
	t.Parallel()
	p := newPuzzleTable(t)
	late := p.h.newUser("Lia")
	p.h.joinPending(p.master, p.campaignID, late)
	pending := late.hero(t, p.campaignID, "Lia", "class:wizard", "race:human", 1, &rulesv1.AbilityScores{Strength: 8, Dexterity: 14, Constitution: 12, Intelligence: 16, Wisdom: 10, Charisma: 10}, nil, []string{fireBolt})
	reason := playv1.PuzzleInvalidReason_PUZZLE_INVALID_REASON_PARTS
	part := func(id, text string) *playv1.PuzzlePart { return &playv1.PuzzlePart{CharacterId: id, Text: text} }
	with := func(parts ...*playv1.PuzzlePart) func(*playv1.CreatePuzzleRequest) {
		return func(r *playv1.CreatePuzzleRequest) { r.Parts = parts }
	}
	owner := "a part is for a living player character of the campaign"
	text := "a part must have 1 to 300 characters"
	p.wantRulesCases(t, p.rulesRiddle, []rulesCase{
		{"nine parts", with(part("", "1"), part("", "2"), part("", "3"), part("", "4"), part("", "5"), part("", "6"), part("", "7"), part("", "8"), part("", "9")), reason, "parts", "a puzzle has at most 8 parts"},
		{"nine bad parts: the count first", with(part("", ""), part("", ""), part("", ""), part("", ""), part("", ""), part("", ""), part("", ""), part("", ""), part("", "")), reason, "parts", "a puzzle has at most 8 parts"},
		{"an empty text", with(part("", "  ")), reason, "parts[0].text", text},
		{"the second text is empty", with(part("", "ok"), part("", "")), reason, "parts[1].text", text},
		{"301 characters", with(part("", strings.Repeat("a", 301))), reason, "parts[0].text", text},
		{"301 characters of two bytes", with(part("", strings.Repeat("ã", 301))), reason, "parts[0].text", text},
		{"not a UUID", with(part("Toren", "parte")), reason, "parts[0].character_id", owner},
		{"a character that is not there", with(part("", "ok"), part(newKey(), "parte")), reason, "parts[1].character_id", owner},
		{"an NPC", with(part(p.goblin.GetId(), "parte")), reason, "parts[0].character_id", owner},
		{"a pending member's character", with(part(pending.GetId(), "parte")), reason, "parts[0].character_id", owner},
		{"a character twice", with(part(p.toren.GetId(), "um"), part(p.toren.GetId(), "dois")), reason, "parts[1].character_id", "a character has one part"},
		{"the text before the owner", with(part("Toren", "")), reason, "parts[0].text", text},
		{"the owner before the twice", with(part(p.toren.GetId(), "um"), part("Toren", "dois"), part(p.toren.GetId(), "tres")), reason, "parts[1].character_id", owner},
	})
	// Fine: ids with spaces and capitals are normalized, a blank id is nobody's, the same
	// text twice, three parts of nobody, and the text is trimmed.
	got := p.riddle(t, "Partes", with(
		part("  "+strings.ToUpper(p.toren.GetId())+" ", "  do Toren  "),
		part("   ", "de ninguem"),
		part("", "de ninguem tambem"),
		part("", "de ninguem tambem"),
	))
	if len(got.GetParts()) != 4 || got.GetParts()[0].GetCharacterId() != p.toren.GetId() || got.GetParts()[0].GetText() != "do Toren" ||
		got.GetParts()[1].GetCharacterId() != "" || got.GetParts()[1].GetText() != "de ninguem" {
		t.Errorf("the parts read %v", got.GetParts())
	}
	if none := p.riddle(t, "Nenhuma", with()); len(none.GetParts()) != 0 {
		t.Errorf("no parts read %v", none.GetParts())
	}
}

func TestPuzzleRules_HintCheckRefusals(t *testing.T) {
	t.Parallel()
	p := newPuzzleTable(t)
	reason := playv1.PuzzleInvalidReason_PUZZLE_INVALID_REASON_HINT_CHECK
	skill := "hint_check.skill_key must be one of the skills, such as skill:investigation"
	dc := "hint_check.dc must be 1 to 30"
	hints := func(r *playv1.CreatePuzzleRequest) { r.Hints = []string{"dica"} }
	check := func(key string, d int32, extra ...func(*playv1.CreatePuzzleRequest)) func(*playv1.CreatePuzzleRequest) {
		return func(r *playv1.CreatePuzzleRequest) {
			for _, e := range extra {
				e(r)
			}
			r.HintCheck = &playv1.PuzzleHintCheck{SkillKey: key, Dc: d}
		}
	}
	p.wantRulesCases(t, p.rulesRiddle, []rulesCase{
		{"an unknown skill", check("skill:cooking", 12, hints), reason, "hint_check.skill_key", skill},
		{"an ability", check("ability:int", 12, hints), reason, "hint_check.skill_key", skill},
		{"no skill", check("", 12, hints), reason, "hint_check.skill_key", skill},
		{"the prefix only", check("skill:", 12, hints), reason, "hint_check.skill_key", skill},
		{"the skill before the DC and the hints", check("skill:cooking", 0), reason, "hint_check.skill_key", skill},
		{"DC 0", check("skill:arcana", 0, hints), reason, "hint_check.dc", dc},
		{"DC 31", check("skill:arcana", 31, hints), reason, "hint_check.dc", dc},
		{"the DC before the hints", check("skill:arcana", 31), reason, "hint_check.dc", dc},
		{"no hints to win", check("skill:arcana", 12), reason, "hint_check", "a skill check needs hints to win"},
	})
	// The key is trimmed and stored trimmed.
	got := p.riddle(t, "Chave", func(r *playv1.CreatePuzzleRequest) {
		hints(r)
		r.HintCheck = &playv1.PuzzleHintCheck{SkillKey: "  skill:arcana  ", Dc: 12}
	})
	if hc := got.GetHintCheck(); hc.GetSkillKey() != "skill:arcana" || hc.GetDc() != 12 {
		t.Errorf("the hint check reads %v", hc)
	}
}

func TestPuzzleRules_OnSolveRefusals(t *testing.T) {
	t.Parallel()
	p := newPuzzleTable(t)
	reason := playv1.PuzzleInvalidReason_PUZZLE_INVALID_REASON_TARGET
	text := playv1.PuzzleInvalidReason_PUZZLE_INVALID_REASON_TEXT
	act := playv1.PuzzleSolveAction_PUZZLE_SOLVE_ACTION_UNSPECIFIED
	with := func(o *playv1.PuzzleOnSolve) func(*playv1.CreatePuzzleRequest) {
		return func(r *playv1.CreatePuzzleRequest) { r.OnSolve = o }
	}
	door := func(mapID string, c, r int32) *playv1.PuzzleOnSolve_Door {
		return &playv1.PuzzleOnSolve_Door{Door: &playv1.PuzzleDoorTarget{MapId: mapID, Col: c, Row: r}}
	}
	point := func(mapID, id string) *playv1.PuzzleOnSolve_Point {
		return &playv1.PuzzleOnSolve_Point{Point: &playv1.PuzzlePointTarget{MapId: mapID, PointId: id}}
	}
	clue := func(id string) *playv1.PuzzleOnSolve_Clue {
		return &playv1.PuzzleOnSolve_Clue{Clue: &playv1.PuzzleClueTarget{ClueId: id}}
	}
	notValid := func(f string) string { return f + " is not valid for this action" }
	const (
		openDoor    = playv1.PuzzleSolveAction_PUZZLE_SOLVE_ACTION_OPEN_DOOR
		revealPoint = playv1.PuzzleSolveAction_PUZZLE_SOLVE_ACTION_REVEAL_POINT
		revealClue  = playv1.PuzzleSolveAction_PUZZLE_SOLVE_ACTION_REVEAL_CLUE
		notify      = playv1.PuzzleSolveAction_PUZZLE_SOLVE_ACTION_NOTIFY
	)
	p.wantRulesCases(t, p.rulesRiddle, []rulesCase{
		{"a message of 201 characters", with(&playv1.PuzzleOnSolve{Message: strings.Repeat("a", 201)}), text, "on_solve.message", "on_solve.message must have 0 to 200 characters"},
		{"the message before the target", with(&playv1.PuzzleOnSolve{Action: openDoor, Message: strings.Repeat("a", 201)}), text, "on_solve.message", "on_solve.message must have 0 to 200 characters"},
		{"a target on an unspecified action", with(&playv1.PuzzleOnSolve{Action: act, Target: clue(newKey())}), reason, "on_solve.target", notValid("on_solve.target")},
		{"a target on a notify", with(&playv1.PuzzleOnSolve{Action: notify, Target: door(p.mapID, 1, 1)}), reason, "on_solve.target", notValid("on_solve.target")},
		{"a door action with no door", with(&playv1.PuzzleOnSolve{Action: openDoor}), reason, "on_solve.door", notValid("on_solve.door")},
		{"a door action with a clue", with(&playv1.PuzzleOnSolve{Action: openDoor, Target: clue(newKey())}), reason, "on_solve.door", notValid("on_solve.door")},
		{"a point action with no point", with(&playv1.PuzzleOnSolve{Action: revealPoint}), reason, "on_solve.point", notValid("on_solve.point")},
		{"a clue action with no clue", with(&playv1.PuzzleOnSolve{Action: revealClue}), reason, "on_solve.clue", notValid("on_solve.clue")},
		{"an action this server does not know", with(&playv1.PuzzleOnSolve{Action: playv1.PuzzleSolveAction(99)}), reason, "on_solve.action", notValid("on_solve.action")},
		{"a door of no map", with(&playv1.PuzzleOnSolve{Action: openDoor, Target: door(newKey(), 1, 1)}), reason, "on_solve.target", notValid("on_solve.target")},
		{"a door on a square without one", with(&playv1.PuzzleOnSolve{Action: openDoor, Target: door(p.mapID, 1, 1)}), reason, "on_solve.target", notValid("on_solve.target")},
		{"a point that is not there", with(&playv1.PuzzleOnSolve{Action: revealPoint, Target: point(p.mapID, newKey())}), reason, "on_solve.target", notValid("on_solve.target")},
		{"a point of no map", with(&playv1.PuzzleOnSolve{Action: revealPoint, Target: point(newKey(), newKey())}), reason, "on_solve.target", notValid("on_solve.target")},
		{"a clue that is not there", with(&playv1.PuzzleOnSolve{Action: revealClue, Target: clue(newKey())}), reason, "on_solve.target", notValid("on_solve.target")},
		{"the on solve before the on wrong", func(r *playv1.CreatePuzzleRequest) {
			r.OnSolve = &playv1.PuzzleOnSolve{Action: revealClue}
			r.OnWrong = &playv1.PuzzleOnWrong{MaxMoves: 201}
		}, reason, "on_solve.clue", notValid("on_solve.clue")},
	})
	// What is kept: a notify with a message keeps the text (trimmed), a notify with none
	// keeps nothing, an unspecified action is a notify.
	kept := p.riddle(t, "Aviso", with(&playv1.PuzzleOnSolve{Message: "  A porta rangeu.  "}))
	if s := kept.GetOnSolve(); s.GetAction() != notify || s.GetMessage() != "A porta rangeu." || s.GetTarget() != nil {
		t.Errorf("the notify reads %v", s)
	}
	none := p.riddle(t, "Sem aviso", with(&playv1.PuzzleOnSolve{Action: notify, Message: "   "}))
	if s := none.GetOnSolve(); s.GetAction() != notify || s.GetMessage() != "" {
		t.Errorf("the empty notify reads %v", s)
	}
	if bare := p.riddle(t, "Nada", with(nil)); bare.GetOnSolve().GetAction() != notify {
		t.Errorf("no on solve reads %v", bare.GetOnSolve())
	}
	// A message of 200 characters is fine, and travels with a door.
	p.paintDoor(t, mapsv1.DoorState_DOOR_STATE_CLOSED, 3, 3)
	withDoor := p.riddle(t, "Porta", with(&playv1.PuzzleOnSolve{Action: openDoor, Message: strings.Repeat("a", 200), Target: door(p.mapID, 3, 3)}))
	if s := withDoor.GetOnSolve(); s.GetAction() != openDoor || len(s.GetMessage()) != 200 || s.GetDoor().GetCol() != 3 {
		t.Errorf("the door reads %v", s)
	}
}

func TestPuzzleRules_OnSolveWithNoMapsModule(t *testing.T) {
	t.Parallel()
	p := newPuzzleTable(t)
	p.h.svc.SetPuzzleMaps(nil)
	open := playv1.PuzzleSolveAction_PUZZLE_SOLVE_ACTION_OPEN_DOOR
	// The shape of the target is checked before the module is looked for.
	req := p.rulesRiddle()
	req.OnSolve = &playv1.PuzzleOnSolve{Action: open}
	_, err := p.pc(p.master).CreatePuzzle(t.Context(), connect.NewRequest(req))
	wantRulesRefusal(t, "a door action with no door", err, playv1.PuzzleInvalidReason_PUZZLE_INVALID_REASON_TARGET, "on_solve.door", "on_solve.door is not valid for this action")
	req = p.rulesRiddle()
	req.OnSolve = &playv1.PuzzleOnSolve{Action: open, Target: &playv1.PuzzleOnSolve_Door{Door: &playv1.PuzzleDoorTarget{MapId: p.mapID, Col: 1, Row: 1}}}
	_, err = p.pc(p.master).CreatePuzzle(t.Context(), connect.NewRequest(req))
	wantCode(t, "a door with no maps module", err, connect.CodeInternal)
	// A notify and a cipher with no key clue need no module.
	p.riddle(t, "Aviso", func(r *playv1.CreatePuzzleRequest) { r.OnSolve = &playv1.PuzzleOnSolve{Message: "ok"} })
	p.cipher(t, "Carta")
	req = &playv1.CreatePuzzleRequest{CampaignId: p.campaignID, Name: "Carta", Config: cipherConfig(newKey()), Solution: cipherSolution(letter, 3)}
	_, err = p.pc(p.master).CreatePuzzle(t.Context(), connect.NewRequest(req))
	wantCode(t, "a key clue with no maps module", err, connect.CodeInternal)
}

func TestPuzzleRules_CipherKeyClueRefusals(t *testing.T) {
	t.Parallel()
	p := newPuzzleTable(t)
	msg := "config.cipher.key_clue_id is not a clue of this campaign"
	reason := playv1.PuzzleInvalidReason_PUZZLE_INVALID_REASON_CIPHER
	for name, id := range map[string]string{"not a UUID": "chave", "a clue that is not there": newKey()} {
		_, err := p.pc(p.master).CreatePuzzle(t.Context(), connect.NewRequest(&playv1.CreatePuzzleRequest{
			CampaignId: p.campaignID, Name: "Carta", Config: cipherConfig(id), Solution: cipherSolution(letter, 3),
		}))
		wantRulesRefusal(t, name, err, reason, "config.cipher.key_clue_id", msg)
	}
}

func TestPuzzleRules_TheChecksRunInOrder(t *testing.T) {
	t.Parallel()
	p := newPuzzleTable(t)
	// Everything wrong at once: the texts, the kind, the target, the key clue, the hint
	// check, the parts and "Ao errar", each refused in turn as the one before is fixed.
	req := &playv1.CreatePuzzleRequest{
		CampaignId: p.campaignID, Name: " ", Config: cipherConfig(newKey()), Solution: cipherSolution(letter, 3),
		OnSolve:   &playv1.PuzzleOnSolve{Action: playv1.PuzzleSolveAction_PUZZLE_SOLVE_ACTION_REVEAL_CLUE},
		HintCheck: &playv1.PuzzleHintCheck{SkillKey: "skill:cooking", Dc: 99},
		Parts:     []*playv1.PuzzlePart{{Text: ""}},
		OnWrong:   &playv1.PuzzleOnWrong{AttemptsPerPlayer: 99},
	}
	steps := []struct {
		fix    func()
		reason playv1.PuzzleInvalidReason
		field  string
	}{
		{func() { req.Name = "Carta" }, playv1.PuzzleInvalidReason_PUZZLE_INVALID_REASON_NAME, "name"},
		{func() { req.OnSolve = nil }, playv1.PuzzleInvalidReason_PUZZLE_INVALID_REASON_TARGET, "on_solve.clue"},
		{func() { req.Config = cipherConfig("") }, playv1.PuzzleInvalidReason_PUZZLE_INVALID_REASON_CIPHER, "config.cipher.key_clue_id"},
		{func() { req.HintCheck = nil }, playv1.PuzzleInvalidReason_PUZZLE_INVALID_REASON_HINT_CHECK, "hint_check.skill_key"},
		{func() { req.Parts = nil }, playv1.PuzzleInvalidReason_PUZZLE_INVALID_REASON_PARTS, "parts[0].text"},
		{func() { req.OnWrong = nil }, playv1.PuzzleInvalidReason_PUZZLE_INVALID_REASON_ON_WRONG, "on_wrong.attempts_per_player"},
	}
	for i, st := range steps {
		_, err := p.pc(p.master).CreatePuzzle(t.Context(), connect.NewRequest(req))
		wantPuzzleInvalid(t, "step "+string(rune('0'+i)), err, st.reason)
		if got := puzzleRefusalField(err); got != st.field {
			t.Errorf("step %d field = %q, want %q", i, got, st.field)
		}
		st.fix()
	}
	if _, err := p.pc(p.master).CreatePuzzle(t.Context(), connect.NewRequest(req)); err != nil {
		t.Fatalf("CreatePuzzle() after the fixes error = %v", err)
	}
}

// storedRiddleRow is a puzzle row as the store keeps it, for decodePuzzle.
func storedRiddleRow() playdb.Puzzle {
	skill, dc := "skill:arcana", int32(12)
	return playdb.Puzzle{
		ID:          "11111111-1111-1111-1111-111111111111",
		Config:      toJSON(riddleConfig()),
		Solution:    toJSON(riddleSolution("sombra")),
		Start:       []byte(`{}`),
		SolveAction: solveRevealClue,
		SolveTarget: toJSON(&playv1.PuzzleOnSolve{Message: "ok", Target: &playv1.PuzzleOnSolve_Clue{Clue: &playv1.PuzzleClueTarget{ClueId: "c1"}}}),
		Hints:       []byte(`["um","dois"]`),
		HintSkill:   &skill,
		HintDc:      &dc,
		Parts:       []byte(`[{"character_id":"a","text":"x"},{"character_id":"","text":"y"}]`),
		OnWrong:     toJSON(&playv1.PuzzleOnWrong{AttemptsPerPlayer: 3, MaxMoves: 5}),
	}
}

func TestPuzzleRules_DecodePuzzle(t *testing.T) {
	t.Parallel()
	d, err := decodePuzzle(storedRiddleRow())
	if err != nil {
		t.Fatalf("decodePuzzle() error = %v", err)
	}
	if d.kind == nil || d.row.ID == "" || d.config.GetRiddle() == nil || d.solution.GetRiddle() == nil {
		t.Errorf("decoded %+v", d)
	}
	if d.onSolve.GetAction() != playv1.PuzzleSolveAction_PUZZLE_SOLVE_ACTION_REVEAL_CLUE || d.onSolve.GetMessage() != "ok" || d.onSolve.GetClue().GetClueId() != "c1" {
		t.Errorf("onSolve = %v", d.onSolve)
	}
	if len(d.hints) != 2 || d.hints[1] != "dois" {
		t.Errorf("hints = %v", d.hints)
	}
	if d.hintCheck.GetSkillKey() != "skill:arcana" || d.hintCheck.GetDc() != 12 {
		t.Errorf("hintCheck = %v", d.hintCheck)
	}
	if len(d.parts) != 2 || d.parts[0].GetCharacterId() != "a" || d.parts[1].GetText() != "y" {
		t.Errorf("parts = %v", d.parts)
	}
	if d.onWrong.GetAttemptsPerPlayer() != 3 || d.onWrong.GetMaxMoves() != 5 {
		t.Errorf("onWrong = %v", d.onWrong)
	}
	// Everything optional left out: a notify, no hints, no check, no parts, no limits.
	row := storedRiddleRow()
	row.SolveAction, row.SolveTarget, row.Hints, row.HintSkill, row.HintDc, row.Parts, row.OnWrong = solveNotify, nil, nil, nil, nil, nil, nil
	d, err = decodePuzzle(row)
	if err != nil {
		t.Fatalf("decodePuzzle(bare) error = %v", err)
	}
	if d.onSolve.GetAction() != playv1.PuzzleSolveAction_PUZZLE_SOLVE_ACTION_NOTIFY || d.hints != nil || d.hintCheck != nil || d.parts != nil || d.onWrong != nil {
		t.Errorf("bare decoded %+v", d)
	}
	// A check needs both its halves.
	row = storedRiddleRow()
	row.HintDc = nil
	if d, err = decodePuzzle(row); err != nil || d.hintCheck != nil {
		t.Errorf("a skill with no DC = %v, %v", d.hintCheck, err)
	}
	row = storedRiddleRow()
	row.HintSkill = nil
	if d, err = decodePuzzle(row); err != nil || d.hintCheck != nil {
		t.Errorf("a DC with no skill = %v, %v", d.hintCheck, err)
	}
	// An unknown stored action falls back to a notify.
	row = storedRiddleRow()
	row.SolveAction = "other"
	if d, err = decodePuzzle(row); err != nil || d.onSolve.GetAction() != playv1.PuzzleSolveAction_PUZZLE_SOLVE_ACTION_NOTIFY {
		t.Errorf("an unknown action = %v, %v", d.onSolve, err)
	}
}

func TestPuzzleRules_DecodePuzzleRefusals(t *testing.T) {
	t.Parallel()
	const bad = `{"nope":`
	tests := []struct {
		name string
		edit func(*playdb.Puzzle)
		want string
	}{
		{"a bad config", func(r *playdb.Puzzle) { r.Config = []byte(bad) }, "decode a stored *playv1.PuzzleConfig: "},
		{"a bad solution", func(r *playdb.Puzzle) { r.Solution = []byte(bad) }, "decode a stored *playv1.PuzzleSolution: "},
		{"a bad start", func(r *playdb.Puzzle) { r.Start = []byte(bad) }, "decode a stored *playv1.PuzzleState: "},
		{"a bad target", func(r *playdb.Puzzle) { r.SolveTarget = []byte(bad) }, "decode a stored *playv1.PuzzleOnSolve: "},
		{"bad hints", func(r *playdb.Puzzle) { r.Hints = []byte(`{`) }, "decode the stored hints: "},
		{"no kind", func(r *playdb.Puzzle) { r.Config = []byte(`{}`) }, "puzzle 11111111-1111-1111-1111-111111111111 has a kind this server does not know"},
		{"bad parts", func(r *playdb.Puzzle) { r.Parts = []byte(`{`) }, "decode the stored parts: "},
		{"bad limits", func(r *playdb.Puzzle) { r.OnWrong = []byte(bad) }, "decode a stored *playv1.PuzzleOnWrong: "},
		// The order: config, solution, start, target, hints, kind, parts, limits.
		{"the config first", func(r *playdb.Puzzle) { r.Config, r.Solution, r.Start = []byte(bad), []byte(bad), []byte(bad) }, "decode a stored *playv1.PuzzleConfig: "},
		{"the solution before the start", func(r *playdb.Puzzle) { r.Solution, r.Start = []byte(bad), []byte(bad) }, "decode a stored *playv1.PuzzleSolution: "},
		{"the target before the hints", func(r *playdb.Puzzle) { r.SolveTarget, r.Hints = []byte(bad), []byte(`{`) }, "decode a stored *playv1.PuzzleOnSolve: "},
		{"the hints before the kind", func(r *playdb.Puzzle) { r.Hints, r.Config = []byte(`{`), []byte(`{}`) }, "decode the stored hints: "},
		{"the kind before the parts", func(r *playdb.Puzzle) { r.Config, r.Parts = []byte(`{}`), []byte(`{`) }, "puzzle 11111111-1111-1111-1111-111111111111 has a kind"},
		{"the parts before the limits", func(r *playdb.Puzzle) { r.Parts, r.OnWrong = []byte(`{`), []byte(bad) }, "decode the stored parts: "},
	}
	for _, tt := range tests {
		row := storedRiddleRow()
		tt.edit(&row)
		d, err := decodePuzzle(row)
		if err == nil || !strings.HasPrefix(err.Error(), tt.want) {
			t.Errorf("%s error = %v, want prefix %q", tt.name, err, tt.want)
		}
		if d.kind != nil || d.config != nil {
			t.Errorf("%s decoded %+v, want the zero value", tt.name, d)
		}
	}
}
