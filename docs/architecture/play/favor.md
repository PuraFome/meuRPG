# The master's Inspiration

The master grants Inspiration to a player character; the rule is [RN-43](../../product/rules.md#rn-43-the-masters-inspiration).
The code calls it favor because "inspiration" already means the Bardic Inspiration die ([Held rolls](held-rolls.md)); the app says "Inspiração".
The contract is `FavorService` (`proto/meurpg/play/v1/favor.proto`).
The code is `favor_grant.go`, `favor_read.go`, `favor_events.go`, `favor_rpc.go` and `favor_spend.go`.
`GrantFavor` and `GetFavor` are built; `PassFavor` answers `unimplemented` to a member of the campaign.
The spend is the helper `useFavor`, which the roll paths call.

## The table

`character_favor` has one row for each character that has it, keyed by the character, so it never stacks ([Data](../../data/play.md#character_favor)).
The row carries the campaign and the master that granted it.
Only a player character that is not in the reserve gets a row.
An NPC, a combatant's id, a reserved character, a character of another campaign and an id nobody has all get the same refusal.
A player character gets a grant whatever its status: a dead one is granted too.

## The grant

`GrantFavor` is the master's call, in an open session, with or without a combat.
It does not touch the combat: no combatant is read or written.

1.
   It checks the caller is the master, the key and the character id, and finds the open session.
2.
   `Service.tx` locks the session's row, so grants take turns ([Live session](live-session.md#the-hint-number)).
3.
   It looks for the idempotency key in `session_events`: the same request is answered as the first call was, another request under the key is `invalid_argument`.
4.
   It asks the characters module (`CombatRoster.SessionCharacters`) whether the id is a player character of the campaign that is not reserved.
5.
   It inserts the row with `ON CONFLICT DO NOTHING`: no row back means the character has it.
6.
   It writes the session event `favor_granted`, with the character's id and no text, under the key.
7.
   It collects the hint, which `Service.tx` sends after the commit.

A refusal writes nothing and moves no hint number.
A grant moves it once.
A retry tells the hint again, from the stored event, and moves nothing.
Of concurrent grants to the same character exactly one succeeds and the rest get `ALREADY_HAS`.

## The read

`GetFavor` is a read (`IDEMPOTENT`) for any member of the campaign, in or out of a session.
The master reads every holder of the campaign.
A player reads only the holders among their own characters, and a player with none reads an empty list.
Someone outside the campaign, and a pending member, get `not_found`.
Each holder is the character's id and name; the name comes from the characters module, since the query cannot join its table.

## The spend

The owner of a player character that has the Inspiration spends it for advantage on one of the character's own d20 rolls.
The player chooses it before the d20 is rolled, with `spend_favor` on the roll's request.
The roll is an attack, a saving throw or an ability check; the request of each roll path has the field.
`useFavor` (`favor_spend.go`) is the one place that decides it, inside the transaction of the roll.

| The roll before the spend | The roll with it |
| --- | --- |
| Normal, with no source | Advantage |
| Disadvantage only | Normal |
| Advantage | Refused with `ADDS_NOTHING` |
| Advantages and disadvantages that cancel to normal | Refused with `ADDS_NOTHING` |

The helper answers in this order:

1. A request that does not ask for it keeps its mode.
2. The answer to a held roll reads `favor_spent` from the hold, and never spends or refuses.
3. A retry of a held roll finds the hold under its key: made with the Inspiration, it counts and spends nothing more; made without it, the request is refused with `AFTER_THE_ROLL`.
4. A roller that is not the owner (the master rolling for an absent player included) gets `NOT_THE_HOLDER`.
5. A call that rolls no d20 gets `NOTHING_TO_ROLL`.
6. A roll the Inspiration would not change gets `ADDS_NOTHING`: it already has advantage, or an advantage and a disadvantage cancel.
7. The helper deletes the character's row, and no row back is `HAS_NONE`.

Of concurrent spends of one Inspiration exactly one wins and the rest get `HAS_NONE`.
A refusal spends nothing and moves no hint number.
The row is deleted in the transaction of the roll, so a roll that is not written leaves it.
The spend is part of the roll's event (`Favor`), never an event of its own, so it never closes the undo chain.
`favorGiveBack` puts the row back, for the undo of such a roll; it is idempotent.
The Inspiration is a source of the roll, `SourceFavor`, shown as "Inspiração" to the master and the roller and never hidden.
A hold keeps `favor_spent`, so a retry and the answer count the Inspiration again without spending it.

## Who hears and who reads

| What | The master | The holder's player | Another player |
| --- | --- | --- | --- |
| The row, in `GetFavor` | Every holder | Their own characters | None |
| The hint of a grant | Yes | Yes | No |
| The session's hint number | Steps by one | Steps by one | Steps by one, the same for whoever was picked |
| The combat log and the session summary | No line | No line | No line |

The hint is `vitals_changed` with the character's vitals, which the master and the player already read, so it carries nothing about the Inspiration; the screens read the character and `GetFavor` again.
A character whose vitals cannot be read (a dead one) tells nothing, and the number moves all the same.
`favor_granted` is no line of the combat log: it belongs to no combat, so the log never reads it.

## Reasons

`FavorBlocked` carries the reason of a `failed_precondition`.

| Reason | When |
| --- | --- |
| `ALREADY_HAS` | The character already has the Inspiration |
| `NOT_A_PLAYER_CHARACTER` | The id is not a player character of this campaign that is out of the reserve |

The spend sends `NOT_THE_HOLDER`, `HAS_NONE`, `ADDS_NOTHING`, `AFTER_THE_ROLL` and `NOTHING_TO_ROLL`.
`CANNOT_RECEIVE` and `SELF` are for the pass, which is not built.
A grant with no open session is `NO_OPEN_SESSION` (`GameSessionBlocked`).
A player calling it is `permission_denied`.

## Tests

`favor_grant_test.go` covers the grant, the read, the refusals (the combatants of a combat among them), a dead holder, the retry and the races.
`favor_spend_test.go` covers the decision table of the helper, the held retry and answer, the race of twenty spends and the adapters.
`TestAuthorizationMatrix` has the rows of the three calls.
`internal/leaktest/favor_test.go` proves that a third player reads nothing of who has it, in the read, the log and the stream.
