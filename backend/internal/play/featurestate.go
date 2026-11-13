package play

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"

	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
)

// The state of a feature on a combatant (combatants.feature_state, docs/architecture/play/feature-state.md): one JSON
// object, a key per feature, each entry saying how often the feature was used for as long as its scope lasts. Nothing
// resets an entry: it is live while its mark matches now, and a stale one reads as empty and goes on the next write.

// The scopes of an entry: how long its mark holds.
const (
	scopeTurn   = "turn"   // the turn: the mark is turnKey
	scopeRound  = "round"  // the round: the mark is the round number
	scopeCombat = "combat" // the whole combat: no mark
)

// The keys of the SRD features that were columns of their own before the state.
const (
	featureSneakAttack    = "sneak_attack"
	featureColossusSlayer = "colossus_slayer"
)

// The limits of a document; the database check holds the size too (00326).
const (
	featureStateMaxKeys  = 64
	featureStateMaxKey   = 80
	featureStateMaxBytes = 8192
)

// featureEntry is the state of one feature: its scope (S), the mark that says when it was written (At), the uses since (N,
// at least 1) and the choice made with it (C, left out when there is none).
type featureEntry struct {
	S  string `json:"s"`
	At string `json:"at"`
	N  int32  `json:"n"`
	C  string `json:"c,omitempty"`
}

// featureState is the document: the entries by feature key.
type featureState map[string]featureEntry

// stateNow is the moment an entry is read against: the turn's key and the round of the encounter.
type stateNow struct {
	Turn  string
	Round int32
}

// stateNowOf is the moment of the encounter.
func stateNowOf(enc playdb.Encounter) stateNow {
	return stateNow{Turn: turnKey(enc), Round: enc.Round}
}

// turnEntry is an entry of scope turn for the moment, with one use.
func (n stateNow) turnEntry() featureEntry {
	return featureEntry{S: scopeTurn, At: n.Turn, N: 1}
}

// live says the entry still holds at the moment: it has a known scope and at least one use, and its mark is the one of now.
func (e featureEntry) live(now stateNow) bool {
	if e.N < 1 {
		return false
	}
	switch e.S {
	case scopeTurn:
		return e.At == now.Turn
	case scopeRound:
		return e.At == strconv.Itoa(int(now.Round))
	case scopeCombat:
		return true
	}
	return false
}

// readFeatureState reads the column; no document, a bad one or one that is not an object is an empty state.
func readFeatureState(raw []byte) featureState {
	out := featureState{}
	if len(raw) == 0 || json.Unmarshal(raw, &out) != nil || out == nil {
		return featureState{}
	}
	return out
}

// live is the entry of the key when it still holds at now; a stale or absent one reads as empty (false).
func (fs featureState) live(key string, now stateNow) (featureEntry, bool) {
	e, ok := fs[key]
	if !ok || !e.live(now) {
		return featureEntry{}, false
	}
	return e, true
}

// used says the feature has an entry that holds at now.
func (fs featureState) used(key string, now stateNow) bool {
	_, ok := fs.live(key, now)
	return ok
}

// set puts the entry of the key; the document is checked when it is written (validate).
func (fs featureState) set(key string, e featureEntry) {
	fs[key] = e
}

// del takes the entry of the key out.
func (fs featureState) del(key string) {
	delete(fs, key)
}

// prune drops the entries that no longer hold at now.
func (fs featureState) prune(now stateNow) {
	for k, e := range fs {
		if !e.live(now) {
			delete(fs, k)
		}
	}
}

// bytes is the document as the column stores it.
func (fs featureState) bytes() []byte {
	if len(fs) == 0 {
		return []byte("{}")
	}
	b, err := json.Marshal(fs)
	if err != nil {
		return []byte("{}")
	}
	return b
}

// validate refuses a document the database would: too many keys, an empty or long key, a document over the size.
func (fs featureState) validate() error {
	if len(fs) > featureStateMaxKeys {
		return fmt.Errorf("the feature state has %d keys, at most %d", len(fs), featureStateMaxKeys)
	}
	for k := range fs {
		if k == "" || len(k) > featureStateMaxKey {
			return fmt.Errorf("the feature state key %q is empty or over %d characters", k, featureStateMaxKey)
		}
	}
	if n := len(fs.bytes()); n > featureStateMaxBytes {
		return errors.New("the feature state is " + strconv.Itoa(n) + " bytes, at most " + strconv.Itoa(featureStateMaxBytes))
	}
	return nil
}

// saveFeatureState writes the document of a combatant inside the combat's transaction: the stale entries go, the limits are
// checked, and the whole document replaces the one stored.
func saveFeatureState(ctx context.Context, c *combatTx, combatantID string, fs featureState, now stateNow) error {
	fs.prune(now)
	if err := fs.validate(); err != nil {
		return err
	}
	if err := c.q.SetCombatantFeatureState(ctx, playdb.SetCombatantFeatureStateParams{ID: combatantID, FeatureState: fs.bytes()}); err != nil {
		return fmt.Errorf("write the feature state: %w", err)
	}
	return nil
}

// featureUsedThisTurn says the combatant's feature has an entry of this turn (the once-per-turn mark).
func featureUsedThisTurn(attacker playdb.Combatant, enc playdb.Encounter, key string) bool {
	e, ok := readFeatureState(attacker.FeatureState).live(key, stateNowOf(enc))
	return ok && e.S == scopeTurn
}
