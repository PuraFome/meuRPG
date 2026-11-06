package link

// EquipKind is the kind of gear an EquipChange moves.
type EquipKind string

// The kinds of gear a character equips or stows.
const (
	EquipWeapon EquipKind = "weapon"
	EquipShield EquipKind = "shield"
	EquipArmor  EquipKind = "armor"
)

// EquipChange is one piece of gear the character equips or stows (characters.ReviewHost.EquipInCombat):
// its content key, its kind and whether it is equipped after the change.
type EquipChange struct {
	Key   string
	Kind  EquipKind
	Equip bool
}
