package play

// The session events of an Artificer's infusions (the characters module's character_infusions). Each payload holds ids and
// keys, never a name or free text. No writer uses them yet.
const (
	// eventInfusionInfused: an item was infused.
	eventInfusionInfused = "infusion_infused"
	// eventInfusionMoved: an infusion was put on another item.
	eventInfusionMoved = "infusion_moved"
	// eventInfusionEnded: an infusion ended.
	eventInfusionEnded = "infusion_ended"
)

// infusionEvent is the payload of the three infusion events: the record, the artificer, the holder and the infusion's
// option, with the item it is on. Ended is why a record ended (infusion_ended only).
type infusionEvent struct {
	RecordID    string `json:"record_id"`
	ArtificerID string `json:"artificer_character_id"`
	HolderID    string `json:"holder_character_id"`
	Infusion    string `json:"infusion_key"`
	ItemKind    string `json:"item_kind"`
	ItemKey     string `json:"item_key"`
	Ended       string `json:"ended_reason,omitempty"`
}
