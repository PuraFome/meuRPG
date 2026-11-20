-- +goose Up
-- Cutting Words (SRD 5.1, College of Lore) on an ability check a creature of the master's side makes outside a combat: an NPC's
-- Deception against the party in a scene with no map, say. The master's roll is a held roll of the kind `npc_check` (the d20 and
-- its bonus are the creature's, character_id is the NPC and user_id the master), and the bards of the table that may cut it are
-- asked one at a time, in the order of the party: one row of npc_check_asks for each bard asked. The question belongs to the bard
-- (the hold is read by the master and by the bard that is asked now, and by nobody else).
-- The widened CHECK only adds a value, so every row stays valid.
ALTER TABLE inspiration_holds
    DROP CONSTRAINT IF EXISTS inspiration_holds_kind_valid,
    ADD CONSTRAINT inspiration_holds_kind_valid CHECK (kind IN (
        'scene_check', 'group_check', 'trap_search', 'hint_try', 'spell_attack', 'effect_save', 'concentration_save',
        'weapon_attack', 'death_save', 'initiative', 'contest_check', 'hide_check', 'counterspell_check', 'mount_save', 'npc_check'
    ));

-- position is the order the bards are asked in (0 first); state is where the question stands: 'asked' (open, or waiting for the
-- bards before it), 'cut' (the die was taken off: die_face is its face), 'passed' (the bard let it go) or 'closed' (no answer: the
-- master went on without the bard, or it could not answer any more). creature_seen says the bard's player sees the creature, so
-- its question names it ('uma criatura' otherwise). answer_key is the key of the answer that closed the question and answer_hash the
-- hash of the request that gave it, so a retry with the same key and another body is told from a true retry.
-- map_id is the session's current map when the check was rolled (NULL with no map, or once the map is deleted): the creature's
-- token that the master hides on that map closes the check, whatever map the session is on later.
-- bard_character_id is NULL once the bard's character is deleted: the question stays (with its place in the order, and the
-- master's read of it) and is let pass when its turn comes or the next read finds it.
CREATE TABLE IF NOT EXISTS npc_check_asks (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    hold_id UUID NOT NULL REFERENCES inspiration_holds (id) ON DELETE CASCADE,
    map_id UUID NULL REFERENCES maps (id) ON DELETE SET NULL,
    bard_character_id UUID NULL REFERENCES characters (id) ON DELETE SET NULL,
    position INT4 NOT NULL CHECK (position >= 0),
    state TEXT NOT NULL DEFAULT 'asked' CHECK (state IN ('asked', 'cut', 'passed', 'closed')),
    sides INT4 NOT NULL CHECK (sides IN (6, 8, 10, 12)),
    creature_seen BOOL NOT NULL,
    die_face INT4 NULL CHECK (die_face >= 1),
    physical BOOL NOT NULL DEFAULT false,
    answer_key UUID NULL,
    answer_hash TEXT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    answered_at TIMESTAMPTZ NULL,
    UNIQUE (hold_id, bard_character_id),
    UNIQUE (hold_id, position)
);

-- The master's roll is the session's event `npc_check_held` while a bard is asked, and `npc_check_resolved` when the check settles
-- (the master's own history: ids and numbers only).
INSERT INTO session_event_kinds (kind) VALUES
    ('npc_check_held'),
    ('npc_check_resolved')
ON CONFLICT (kind) DO NOTHING;

-- +goose Down
DELETE FROM session_events WHERE kind IN ('npc_check_held', 'npc_check_resolved');
DELETE FROM session_event_kinds WHERE kind IN ('npc_check_held', 'npc_check_resolved');
DROP TABLE IF EXISTS npc_check_asks;
DELETE FROM inspiration_holds WHERE kind = 'npc_check';
ALTER TABLE inspiration_holds
    DROP CONSTRAINT IF EXISTS inspiration_holds_kind_valid,
    ADD CONSTRAINT inspiration_holds_kind_valid CHECK (kind IN (
        'scene_check', 'group_check', 'trap_search', 'hint_try', 'spell_attack', 'effect_save', 'concentration_save',
        'weapon_attack', 'death_save', 'initiative', 'contest_check', 'hide_check', 'counterspell_check', 'mount_save'
    ));
