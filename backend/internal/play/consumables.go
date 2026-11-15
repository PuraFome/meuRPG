package play

import (
	"context"
	"errors"

	"connectrpc.com/connect"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// The use of a consumable (ChargesService.UseConsumable): a potion, a scroll or another single-use item of the sheet is
// spent, and what it does is carried out or, for the consumables the app has no effect for, left to the table's text.

// eventConsumableUsed: a consumable was used up. Like the charge events it is not an undoable kind, so it closes the
// undo chain; its payload (consumableUsedEvent) holds ids, numbers and the catalog's names, never free text.
const eventConsumableUsed = "consumable_used"

// consumableUsedEvent is the payload of consumable_used. The keys round, combatant_id and secret are the ones the
// combat log reads from every event; the rest only the line of the use reads. It is for the master, the user's owner
// and the receiver's owner alone (Secret).
type consumableUsedEvent struct {
	Round  int32  `json:"round"`
	Actor  string `json:"combatant_id,omitempty"`
	Secret bool   `json:"secret"`
	// Entry, Item and Name are the entry's id on the sheet, the catalog key and the name the line shows.
	Entry string `json:"use_entry,omitempty"`
	Item  string `json:"use_item"`
	Name  string `json:"use_name_pt,omitempty"`
	// Outcome is one of the consumableOutcome* values.
	Outcome string `json:"use_outcome"`
	// Target is the character that received the effect and TargetCombatant its combatant in a running combat; both are
	// empty when the user received it.
	Target          string `json:"use_target,omitempty"`
	TargetCombatant string `json:"use_target_combatant,omitempty"`
	// Dice, Faces and Total are the healing dice as the sheet writes them, the faces rolled or typed and their total
	// with the potion's bonus; Healed the hit points regained, cut to the maximum. Physical says the faces were typed.
	Dice     string  `json:"use_dice,omitempty"`
	Faces    []int32 `json:"use_faces,omitempty"`
	Total    int32   `json:"use_total,omitempty"`
	Healed   int32   `json:"use_healed,omitempty"`
	Physical bool    `json:"use_physical,omitempty"`
	// DamageType is the key of the damage a resistance potion gave resistance to.
	DamageType string `json:"use_damage_type,omitempty"`
}

// What a use left behind, as the payload says it (ConsumableUseOutcome on the line).
const (
	consumableOutcomeHealed     = "healed"
	consumableOutcomeResistance = "resistance"
	consumableOutcomeTextOnly   = "text_only"
)

// UseConsumable is not carried out yet.
func (s *Service) UseConsumable(context.Context, *connect.Request[playv1.UseConsumableRequest]) (*connect.Response[playv1.UseConsumableResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("UseConsumable is not carried out yet"))
}
