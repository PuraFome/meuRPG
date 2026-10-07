-- +goose Up
-- character_creatures are the creatures that belong to a player's character
-- (MR-037, Etapa 9): its familiar, the animals and undead it summoned, the
-- creatures the master gave it. They last from one session to the next until
-- the player or the master dismisses them; the app does not count a spell's
-- duration. A creature is a kind of SRD creature (monster_key, "monster:wolf":
-- the stat block is rules content, never stored) with a name the table chose.
--
-- character_id is the owner. source says where it came from ('familiar' is
-- Encontrar Familiar, one at a time; 'animate_dead'; 'conjure_animals', which
-- lasts while the caster concentrates; 'master', a gift). attack is what it
-- may do on its own in a combat, from the spell that brought it (rules.SummonAttack*:
-- 'none' for a familiar, 'reaction' for the Pact of the Chain's, 'full').
-- summon_group_id is shared by the creatures of one casting, which share one
-- initiative roll in a combat. concentration_cast_id is set for the creatures
-- that depend on the caster's concentration: when it ends, they are dismissed.
--
-- hp_current and hp_max are the creature's own hit points (a creature is not a
-- sheet: its maximum is its stat block's, copied when it is created). A combat
-- copies them to the combatant and writes them back when it ends. The master
-- corrects them outside a combat (RN-02).
--
-- A dismissed creature is kept, with dismissed_at and dismissed_reason ('owner',
-- 'master', 'defeated' at 0 hit points, 'concentration', 'replaced' by a new
-- familiar, 'undone'), so an undo of the master can bring it back. name is free
-- text written by a player: it is in docs/privacy.md's inventory.
--
-- Deleting the campaign or the character deletes its creatures.
CREATE TABLE IF NOT EXISTS character_creatures (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    campaign_id UUID NOT NULL REFERENCES campaigns (id) ON DELETE CASCADE,
    character_id UUID NOT NULL REFERENCES characters (id) ON DELETE CASCADE,
    monster_key TEXT NOT NULL,
    name TEXT NOT NULL,
    source TEXT NOT NULL,
    attack TEXT NOT NULL,
    summon_group_id UUID NOT NULL,
    concentration_cast_id UUID NULL,
    hp_current INT4 NOT NULL,
    hp_max INT4 NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    dismissed_at TIMESTAMPTZ NULL,
    dismissed_reason TEXT NULL,
    CONSTRAINT character_creatures_name_length CHECK (char_length(name) BETWEEN 1 AND 40),
    CONSTRAINT character_creatures_monster_key_length CHECK (char_length(monster_key) BETWEEN 1 AND 100),
    CONSTRAINT character_creatures_source_valid CHECK (source IN ('familiar', 'animate_dead', 'conjure_animals', 'master')),
    CONSTRAINT character_creatures_attack_valid CHECK (attack IN ('none', 'reaction', 'full')),
    CONSTRAINT character_creatures_hit_points_valid CHECK (hp_max >= 1 AND hp_current BETWEEN 0 AND hp_max),
    CONSTRAINT character_creatures_dismissed_valid CHECK (
        (dismissed_at IS NULL) = (dismissed_reason IS NULL)
        AND (dismissed_reason IS NULL OR dismissed_reason IN ('owner', 'master', 'defeated', 'concentration', 'replaced', 'undone'))
    )
);

-- +goose Down
DROP TABLE IF EXISTS character_creatures;
