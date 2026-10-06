package play

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"
	"unicode/utf8"
	"uuid"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/types/known/timestamppb"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	maplink "github.com/PuraFome/meuRPG/backend/internal/maps/link"
	"github.com/PuraFome/meuRPG/backend/internal/play/link"
	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
)

// What the master sets in a puzzle whatever its kind (slice 10.7b), and the limits
// worked out from a run: "Ao errar" (a trap, the attempts, the limits), the skill check
// that wins a hint and the split information. The checks run when the master makes
// or edits the puzzle, inside its transaction.

// The limits of what the master writes here.
const (
	maxParts        = 8
	maxPartText     = 300
	maxAttempts     = 10
	maxMovesLimit   = 200
	minTimeLimit    = 10    // seconds
	maxTimeLimit    = 14400 // seconds: four hours
	minHintDC       = 1
	maxHintDC       = 30
	stoppedLine     = "O quebra-cabeça parou. O mestre decide o que acontece agora."
	skillKeyPrefix  = "skill:"
	hintTriesOnView = 50
)

func onWrongInvalid(field, msg string) error {
	return puzzleInvalid(playv1.PuzzleInvalidReason_PUZZLE_INVALID_REASON_ON_WRONG, field, msg)
}

// checkOnWrong normalizes "Ao errar" and checks it inside tx: the numbers are in range,
// the trap is a trap point of the campaign, and a trap or attempts are asked only of a
// kind that judges its moves. It returns nil for nothing to do.
func (s *Service) checkOnWrong(ctx context.Context, tx pgx.Tx, campaignID string, kind puzzleKind, on *playv1.PuzzleOnWrong) (*playv1.PuzzleOnWrong, error) {
	if on == nil {
		return nil, nil //nolint:nilnil // nil means "nothing happens": the stored column is NULL
	}
	out := &playv1.PuzzleOnWrong{}
	if n := on.GetAttemptsPerPlayer(); n != 0 {
		if n < 1 || n > maxAttempts {
			return nil, onWrongInvalid("on_wrong.attempts_per_player", fmt.Sprintf("attempts_per_player must be 1 to %d", maxAttempts))
		}
		if !kind.judges() {
			return nil, onWrongInvalid("on_wrong.attempts_per_player", "this kind has no wrong move to spend an attempt")
		}
		out.AttemptsPerPlayer = n
	}
	if n := on.GetMaxMoves(); n != 0 {
		if n < 1 || n > maxMovesLimit {
			return nil, onWrongInvalid("on_wrong.max_moves", fmt.Sprintf("max_moves must be 1 to %d", maxMovesLimit))
		}
		out.MaxMoves = n
	}
	if n := on.GetTimeLimitSeconds(); n != 0 {
		if n < minTimeLimit || n > maxTimeLimit {
			return nil, onWrongInvalid("on_wrong.time_limit_seconds", fmt.Sprintf("time_limit_seconds must be %d to %d", minTimeLimit, maxTimeLimit))
		}
		out.TimeLimitSeconds = n
	}
	if t := on.GetTrap(); t != nil {
		if !kind.judges() {
			return nil, onWrongInvalid("on_wrong.trap", "this kind has no wrong move to fire a trap")
		}
		if s.traps == nil {
			return nil, errNoTraps()
		}
		mapID, err1 := uuid.Parse(t.GetMapId())
		pointID, err2 := uuid.Parse(t.GetPointId())
		if err1 != nil || err2 != nil {
			return nil, onWrongInvalid("on_wrong.trap", "on_wrong.trap must name a map and a trap point")
		}
		traps, err := s.traps.Traps(ctx, tx, campaignID, mapID.String())
		if connect.CodeOf(err) == connect.CodeNotFound {
			return nil, onWrongInvalid("on_wrong.trap", "on_wrong.trap is not a trap point of this campaign")
		}
		if err != nil {
			return nil, err
		}
		if !slices.ContainsFunc(traps, func(tr maplink.Trap) bool { return tr.PointID == pointID.String() }) {
			return nil, onWrongInvalid("on_wrong.trap", "on_wrong.trap is not a trap point of this campaign")
		}
		out.Trap = &playv1.PuzzleTrapTarget{MapId: mapID.String(), PointId: pointID.String()}
	}
	if out.GetTrap() == nil && out.GetAttemptsPerPlayer() == 0 && out.GetMaxMoves() == 0 && out.GetTimeLimitSeconds() == 0 {
		return nil, nil //nolint:nilnil // all zero is "nothing happens"
	}
	return out, nil
}

// checkHintCheck normalizes the skill check that wins a hint: a skill the scene checks
// know, a DC from 1 to 30, and at least one hint to win.
func (s *Service) checkHintCheck(hc *playv1.PuzzleHintCheck, hints []string) (*playv1.PuzzleHintCheck, error) {
	if hc == nil {
		return nil, nil //nolint:nilnil // nil means no check
	}
	bad := func(field, msg string) error {
		return puzzleInvalid(playv1.PuzzleInvalidReason_PUZZLE_INVALID_REASON_HINT_CHECK, field, msg)
	}
	key := strings.TrimSpace(hc.GetSkillKey())
	if !strings.HasPrefix(key, skillKeyPrefix) || s.roster.SceneCheckName(key) == "" {
		return nil, bad("hint_check.skill_key", "hint_check.skill_key must be one of the skills, such as skill:investigation")
	}
	if hc.GetDc() < minHintDC || hc.GetDc() > maxHintDC {
		return nil, bad("hint_check.dc", fmt.Sprintf("hint_check.dc must be %d to %d", minHintDC, maxHintDC))
	}
	if len(hints) == 0 {
		return nil, bad("hint_check", "a skill check needs hints to win")
	}
	return &playv1.PuzzleHintCheck{SkillKey: key, Dc: hc.GetDc()}, nil
}

// checkParts normalizes the split information inside tx: up to 8 parts of 1 to 300
// characters, each for a living player character of the campaign (the party: a member
// that is still pending has none) or for nobody yet, and no character twice.
func (s *Service) checkParts(ctx context.Context, tx pgx.Tx, campaignID string, parts []*playv1.PuzzlePart) ([]*playv1.PuzzlePart, error) {
	bad := func(field, msg string) error {
		return puzzleInvalid(playv1.PuzzleInvalidReason_PUZZLE_INVALID_REASON_PARTS, field, msg)
	}
	if len(parts) > maxParts {
		return nil, bad("parts", fmt.Sprintf("a puzzle has at most %d parts", maxParts))
	}
	if len(parts) == 0 {
		return nil, nil
	}
	party, err := s.roster.CombatParty(ctx, tx, campaignID)
	if err != nil {
		return nil, err
	}
	out := make([]*playv1.PuzzlePart, 0, len(parts))
	owners := map[string]bool{}
	for i, p := range parts {
		text := strings.TrimSpace(p.GetText())
		if n := utf8.RuneCountInString(text); n < 1 || n > maxPartText {
			return nil, bad(fmt.Sprintf("parts[%d].text", i), fmt.Sprintf("a part must have 1 to %d characters", maxPartText))
		}
		id := strings.TrimSpace(p.GetCharacterId())
		if id != "" {
			parsed, err := uuid.Parse(id)
			if err != nil || !slices.ContainsFunc(party, func(c link.Character) bool { return c.ID == parsed.String() }) {
				return nil, bad(fmt.Sprintf("parts[%d].character_id", i), "a part is for a living player character of the campaign")
			}
			id = parsed.String()
			if owners[id] {
				return nil, bad(fmt.Sprintf("parts[%d].character_id", i), "a character has one part")
			}
			owners[id] = true
		}
		out = append(out, &playv1.PuzzlePart{CharacterId: id, Text: text})
	}
	return out, nil
}

// checkCipherKeyClue checks that the clue a cipher's key lives in is a clue of the
// campaign. A cipher with no key clue is fine: the master may tell the key at the table.
func (s *Service) checkCipherKeyClue(ctx context.Context, tx pgx.Tx, campaignID string, cfg *playv1.PuzzleConfig) error {
	id := cfg.GetCipher().GetKeyClueId()
	if id == "" {
		return nil
	}
	if s.puzzles.maps == nil {
		return errors.New("play: the maps module is not connected to the puzzles")
	}
	notAClue := puzzleInvalid(playv1.PuzzleInvalidReason_PUZZLE_INVALID_REASON_CIPHER, "config.cipher.key_clue_id", "config.cipher.key_clue_id is not a clue of this campaign")
	parsed, err := uuid.Parse(id)
	if err != nil {
		return notAClue
	}
	if err := s.puzzles.maps.PuzzleCheckClue(ctx, tx, campaignID, parsed.String()); err != nil {
		if connect.CodeOf(err) == connect.CodeNotFound {
			return notAClue
		}
		return err
	}
	return nil
}

// --- the limits, worked out from the run ---

// roundMoves is how many moves the run's current round has: the ones after the move
// the round began at.
func roundMoves(run playdb.PuzzleRun) int { return int(run.MovesMade - run.RoundStartSeq) }

// deadlineOf is when the time limit runs out, nil without one or while the run is not
// shown.
func deadlineOf(d puzzleDef, run playdb.PuzzleRun) *time.Time {
	secs := d.onWrong.GetTimeLimitSeconds()
	if secs <= 0 || run.RoundStartedAt == nil {
		return nil
	}
	t := run.RoundStartedAt.Add(time.Duration(secs) * time.Second)
	return &t
}

// stopOf says whether a limit stopped the puzzle at now, and which: the limit of moves
// when the round has made them all, the time limit when it ran out. A solved run is
// never stopped. It is worked out from the clock and the counters, never stored: the
// restart that begins a new round is what ends it.
func stopOf(d puzzleDef, run playdb.PuzzleRun, now time.Time) playv1.PuzzleStopReason {
	if run.SolvedAt != nil || d.onWrong == nil {
		return playv1.PuzzleStopReason_PUZZLE_STOP_REASON_UNSPECIFIED
	}
	if limit := int(d.onWrong.GetMaxMoves()); limit > 0 && roundMoves(run) >= limit {
		return playv1.PuzzleStopReason_PUZZLE_STOP_REASON_MOVES
	}
	if dl := deadlineOf(d, run); dl != nil && !now.Before(*dl) {
		return playv1.PuzzleStopReason_PUZZLE_STOP_REASON_TIME
	}
	return playv1.PuzzleStopReason_PUZZLE_STOP_REASON_UNSPECIFIED
}

// limitsProto is the counters of "Ao errar" as a player reads them: nil when the
// puzzle has no limit at all (a trap alone is not one).
func limitsProto(d puzzleDef, run playdb.PuzzleRun, now time.Time, wrongMine int) *playv1.PuzzleLimits {
	on := d.onWrong
	if on.GetAttemptsPerPlayer() == 0 && on.GetMaxMoves() == 0 && on.GetTimeLimitSeconds() == 0 {
		return nil
	}
	out := &playv1.PuzzleLimits{
		AttemptsPerPlayer: on.GetAttemptsPerPlayer(), MaxMoves: on.GetMaxMoves(), TimeLimitSeconds: on.GetTimeLimitSeconds(),
		MovesMade: clamp32(roundMoves(run), 0, 1<<20),
	}
	if on.GetAttemptsPerPlayer() > 0 {
		out.AttemptsLeft = max(on.GetAttemptsPerPlayer()-clamp32(wrongMine, 0, 1<<20), 0)
	}
	if dl := deadlineOf(d, run); dl != nil {
		out.Deadline = timestamppb.New(*dl)
		out.SecondsLeft = clamp32(int(max(dl.Sub(now), 0)/time.Second), 0, maxTimeLimit)
	}
	return out
}
