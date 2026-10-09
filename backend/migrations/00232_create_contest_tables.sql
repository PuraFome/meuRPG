-- +goose Up
-- The tables of the contests and the special actions of a combat (SRD 5.1,
-- "Contests", "Grappling", "Shoving a Creature", "Hide", "Help", "Group Checks",
-- "Surprise"). They hold IDs, keys, numbers and the short reasons the master writes;
-- none of it goes to session_events (docs/privacy.md).

-- combat_contests is a contest of two combatants: a grapple or a shove, an escape from
-- a grapple, and the grapple that came from a creature's attack. kind is 'contest' (both
-- roll an ability check and the totals are compared) or 'escape_dc' (the check is
-- against a fixed escape DC and nobody else rolls: the master's number, escape_dc, which
-- no player reads). purpose is 'grapple', 'shove' or 'escape'. initiator_id starts it
-- (the one that grapples or shoves, the one that tries to escape) and defender_id is the
-- one that answers (the target, the one that holds). status is 'awaiting_defender' (the
-- defender has not rolled), 'awaiting_outcome' (a won shove waits for the choice
-- between knocking prone and pushing), 'resolved' or 'closed' (the master ended it with
-- no answer). initiator_roll and defender_roll are the rolls (the skill, the d20 or the
-- pair, the modifier, the total, whether the dice were physical); winner is 'initiator',
-- 'defender' or 'tie'; shove_outcome is 'prone', 'push' or 'stays'. Deleting the
-- encounter or a combatant deletes it.
CREATE TABLE IF NOT EXISTS combat_contests (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    encounter_id UUID NOT NULL REFERENCES encounters (id) ON DELETE CASCADE,
    kind TEXT NOT NULL,
    purpose TEXT NOT NULL,
    initiator_id UUID NOT NULL REFERENCES combatants (id) ON DELETE CASCADE,
    defender_id UUID NOT NULL REFERENCES combatants (id) ON DELETE CASCADE,
    status TEXT NOT NULL,
    escape_dc INT4 NULL,
    initiator_roll JSONB NULL,
    defender_roll JSONB NULL,
    winner TEXT NULL,
    shove_outcome TEXT NULL,
    round INT4 NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    resolved_at TIMESTAMPTZ NULL,
    CONSTRAINT combat_contests_kind_valid CHECK (kind IN ('contest', 'escape_dc')),
    CONSTRAINT combat_contests_purpose_valid CHECK (purpose IN ('grapple', 'shove', 'escape')),
    CONSTRAINT combat_contests_status_valid CHECK (status IN ('awaiting_defender', 'awaiting_outcome', 'resolved', 'closed')),
    CONSTRAINT combat_contests_winner_valid CHECK (winner IS NULL OR winner IN ('initiator', 'defender', 'tie')),
    CONSTRAINT combat_contests_outcome_valid CHECK (shove_outcome IS NULL OR shove_outcome IN ('prone', 'push', 'stays')),
    CONSTRAINT combat_contests_escape_dc_valid CHECK (escape_dc IS NULL OR escape_dc BETWEEN 1 AND 40)
);

CREATE INDEX IF NOT EXISTS combat_contests_encounter_id_idx ON combat_contests (encounter_id);
CREATE INDEX IF NOT EXISTS combat_contests_initiator_id_idx ON combat_contests (initiator_id);
CREATE INDEX IF NOT EXISTS combat_contests_defender_id_idx ON combat_contests (defender_id);

-- combat_holds says who holds a grappled combatant (the Grappled condition is on
-- the combatant; this is who put it there and, for a grapple from an attack, the
-- escape DC, which no player reads). One row per grappled combatant. A hold that no
-- longer holds (the grappler is incapacitated or defeated, the two are not within reach,
-- the condition was taken off) is deleted by the change that made it so.
CREATE TABLE IF NOT EXISTS combat_holds (
    grappled_id UUID PRIMARY KEY REFERENCES combatants (id) ON DELETE CASCADE,
    encounter_id UUID NOT NULL REFERENCES encounters (id) ON DELETE CASCADE,
    grappler_id UUID NOT NULL REFERENCES combatants (id) ON DELETE CASCADE,
    escape_dc INT4 NULL,
    created_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT combat_holds_escape_dc_valid CHECK (escape_dc IS NULL OR escape_dc BETWEEN 1 AND 40)
);

CREATE INDEX IF NOT EXISTS combat_holds_encounter_id_idx ON combat_holds (encounter_id);
CREATE INDEX IF NOT EXISTS combat_holds_grappler_id_idx ON combat_holds (grappler_id);

-- combat_hide_attempts are the Hide actions of a combat: the Dexterity (Stealth) roll
-- and what the master decided. status is 'pending' (waits for the master), 'applied'
-- (the hider is hidden from the creatures that do not notice it: combat_hiding) or
-- 'refused' (the master saw nowhere to hide; refusal is his one sentence, 1 to 120
-- characters, or NULL for the usual one). The roll is JSON as in combat_contests.
CREATE TABLE IF NOT EXISTS combat_hide_attempts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    encounter_id UUID NOT NULL REFERENCES encounters (id) ON DELETE CASCADE,
    hider_id UUID NOT NULL REFERENCES combatants (id) ON DELETE CASCADE,
    status TEXT NOT NULL,
    roll JSONB NOT NULL,
    refusal TEXT NULL,
    round INT4 NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    resolved_at TIMESTAMPTZ NULL,
    CONSTRAINT combat_hide_attempts_status_valid CHECK (status IN ('pending', 'applied', 'refused')),
    CONSTRAINT combat_hide_attempts_refusal_length CHECK (refusal IS NULL OR char_length(refusal) BETWEEN 1 AND 120)
);

CREATE INDEX IF NOT EXISTS combat_hide_attempts_encounter_id_idx ON combat_hide_attempts (encounter_id);
CREATE INDEX IF NOT EXISTS combat_hide_attempts_hider_id_idx ON combat_hide_attempts (hider_id);

-- combat_hiding is one state per creature that could see the hider: whether it noticed
-- the hider (the stealth total did not beat its passive Perception, or the master says it
-- sees the hider clearly). total and passive are the two numbers it was decided with,
-- only the master reads them. The hider is hidden while one row is not noticed; the
-- rows go when the hider attacks, casts, is seen or the master ends it.
CREATE TABLE IF NOT EXISTS combat_hiding (
    hider_id UUID NOT NULL REFERENCES combatants (id) ON DELETE CASCADE,
    observer_id UUID NOT NULL REFERENCES combatants (id) ON DELETE CASCADE,
    encounter_id UUID NOT NULL REFERENCES encounters (id) ON DELETE CASCADE,
    noticed BOOLEAN NOT NULL,
    total INT4 NOT NULL,
    passive INT4 NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (hider_id, observer_id)
);

CREATE INDEX IF NOT EXISTS combat_hiding_observer_id_idx ON combat_hiding (observer_id);
CREATE INDEX IF NOT EXISTS combat_hiding_encounter_id_idx ON combat_hiding (encounter_id);

-- combat_helps is a Help (SRD 5.1, "Help"): the aid a character gives another one, for
-- the next check of a task (kind 'check', task the check's key) or for the ally's first
-- attack on a creature within 5 feet of the helper (kind 'attack', target_id the
-- target). A help in a combat lasts until it is used or the end of the helper's next
-- turn (expires_round: the round of that turn); outside a combat (encounter_id NULL) it
-- lasts until the master clears it. The characters are the helper's and the ally's.
-- consumed_at is when the ally used it; cleared_at, when the master took it back.
CREATE TABLE IF NOT EXISTS combat_helps (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    game_session_id UUID NOT NULL REFERENCES game_sessions (id) ON DELETE CASCADE,
    encounter_id UUID NULL REFERENCES encounters (id) ON DELETE CASCADE,
    kind TEXT NOT NULL,
    helper_character_id UUID NOT NULL REFERENCES characters (id) ON DELETE CASCADE,
    ally_character_id UUID NOT NULL REFERENCES characters (id) ON DELETE CASCADE,
    task TEXT NULL,
    target_id UUID NULL REFERENCES combatants (id) ON DELETE CASCADE,
    expires_round INT4 NULL,
    created_round INT4 NULL,
    created_at TIMESTAMPTZ NOT NULL,
    consumed_at TIMESTAMPTZ NULL,
    cleared_at TIMESTAMPTZ NULL,
    CONSTRAINT combat_helps_kind_valid CHECK (kind IN ('check', 'attack')),
    CONSTRAINT combat_helps_shape_valid CHECK (
        (kind = 'check' AND task IS NOT NULL AND target_id IS NULL)
        OR (kind = 'attack' AND target_id IS NOT NULL AND task IS NULL)
    ),
    CONSTRAINT combat_helps_task_length CHECK (task IS NULL OR char_length(task) BETWEEN 1 AND 100)
);

CREATE INDEX IF NOT EXISTS combat_helps_game_session_id_idx ON combat_helps (game_session_id);
CREATE INDEX IF NOT EXISTS combat_helps_encounter_id_idx ON combat_helps (encounter_id);
CREATE INDEX IF NOT EXISTS combat_helps_helper_character_id_idx ON combat_helps (helper_character_id);
CREATE INDEX IF NOT EXISTS combat_helps_ally_character_id_idx ON combat_helps (ally_character_id);
CREATE INDEX IF NOT EXISTS combat_helps_target_id_idx ON combat_helps (target_id);

-- combat_surprised are the combatants the master marked as surprised: they do not move,
-- act or react in the first turn of the combat (SRD 5.1, "Surprise"). The mark is read
-- as ended once the combatant's turn of the first round has ended, so the row stays.
CREATE TABLE IF NOT EXISTS combat_surprised (
    combatant_id UUID PRIMARY KEY REFERENCES combatants (id) ON DELETE CASCADE,
    encounter_id UUID NOT NULL REFERENCES encounters (id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS combat_surprised_encounter_id_idx ON combat_surprised (encounter_id);

-- group_checks is a check the master asks of the whole party (SRD 5.1, "Working
-- Together", Group Checks): the skill, the DC the master set (NULL: none) and whether
-- the players read passed and failed (show_dc). status is 'open' or 'closed'; passed is
-- the verdict (at least half of the characters asked passed) once it is closed.
CREATE TABLE IF NOT EXISTS group_checks (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    game_session_id UUID NOT NULL REFERENCES game_sessions (id) ON DELETE CASCADE,
    skill_key TEXT NOT NULL,
    dc INT4 NULL,
    show_dc BOOLEAN NOT NULL DEFAULT false,
    status TEXT NOT NULL,
    passed BOOLEAN NULL,
    created_at TIMESTAMPTZ NOT NULL,
    closed_at TIMESTAMPTZ NULL,
    CONSTRAINT group_checks_status_valid CHECK (status IN ('open', 'closed')),
    CONSTRAINT group_checks_dc_valid CHECK (dc IS NULL OR dc BETWEEN 1 AND 40),
    CONSTRAINT group_checks_skill_length CHECK (char_length(skill_key) BETWEEN 1 AND 100)
);

CREATE INDEX IF NOT EXISTS group_checks_game_session_id_idx ON group_checks (game_session_id);

-- group_check_members are the characters a group check asked, and what each rolled
-- (roll is JSON as in combat_contests; NULL until the character answers). rolled_by_master
-- is true for a roll the master made in the place of a player.
CREATE TABLE IF NOT EXISTS group_check_members (
    group_check_id UUID NOT NULL REFERENCES group_checks (id) ON DELETE CASCADE,
    character_id UUID NOT NULL REFERENCES characters (id) ON DELETE CASCADE,
    roll JSONB NULL,
    rolled_by_master BOOLEAN NOT NULL DEFAULT false,
    rolled_at TIMESTAMPTZ NULL,
    PRIMARY KEY (group_check_id, character_id)
);

CREATE INDEX IF NOT EXISTS group_check_members_character_id_idx ON group_check_members (character_id);

-- +goose Down
DROP TABLE IF EXISTS group_check_members;
DROP TABLE IF EXISTS group_checks;
DROP TABLE IF EXISTS combat_surprised;
DROP TABLE IF EXISTS combat_helps;
DROP TABLE IF EXISTS combat_hiding;
DROP TABLE IF EXISTS combat_hide_attempts;
DROP TABLE IF EXISTS combat_holds;
DROP TABLE IF EXISTS combat_contests;
