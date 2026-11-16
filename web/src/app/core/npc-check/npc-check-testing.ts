import { type MessageInitShape, create } from '@bufbuild/protobuf';
import { timestampFromMs } from '@bufbuild/protobuf/wkt';

import {
  type HeldRollOffer,
  HeldRollKind,
  HeldRollOfferSchema,
  NpcCheckAskState,
} from '../../../gen/meurpg/play/v1/combat_pb';
import {
  type NpcCheckCandidate,
  NpcCheckCandidateSchema,
  type NpcCheckRead,
  NpcCheckReadSchema,
  NpcCheckSource,
} from '../../../gen/meurpg/play/v1/scene_pb';

/**
 * Builders for the specs of the NPC check (never imported by the app, so never bundled): the messages as the server sends them,
 * always made with `create`, so a field added to the proto never leaves a spec with a hand-built object missing it.
 */

/** One bard asked, as the master reads it. */
export function ask(
  name: string,
  state: NpcCheckAskState,
  over: { sides?: number; face?: number } = {},
) {
  return { characterId: name.toLowerCase(), characterName: name, state, sides: over.sides ?? 8, face: over.face ?? 0 };
}

/** A check that waits for Brisa and bilbo: the d20 13 + 4 = 17 of the Capitão bandido's Enganação. */
export function npcCheckRead(over: MessageInitShape<typeof NpcCheckReadSchema> = {}): NpcCheckRead {
  const init = {
    holdId: 'hold-1',
    creatureLabel: 'Capitão bandido',
    checkKey: 'skill:deception',
    checkName: 'Enganação',
    d20: { diceCount: 1, diceSides: 20, faces: [13], modifier: 4, total: 17 },
    modifierKnown: true,
    asks: [
      ask('Brisa', NpcCheckAskState.WAITING),
      ask('bilbo', NpcCheckAskState.WAITING, { sides: 6 }),
    ],
    ...over,
  } as MessageInitShape<typeof NpcCheckReadSchema>;
  return create(NpcCheckReadSchema, init);
}

/** A creature of the picker: the Capitão bandido of the stage, seen by the players, with two checks. */
export function candidate(over: MessageInitShape<typeof NpcCheckCandidateSchema> = {}): NpcCheckCandidate {
  const init = {
    characterId: 'npc-capitao',
    monsterKey: 'monster:bandit-captain',
    name: 'Capitão bandido',
    hasStatBlock: true,
    source: NpcCheckSource.STAGE,
    hidden: false,
    mapMeasures: false,
    checks: [
      { key: 'ability:cha', name: 'Carisma', bonus: 2, known: true },
      { key: 'skill:deception', name: 'Enganação', bonus: 4, known: true },
      { key: 'skill:persuasion', name: 'Persuasão', bonus: 2, known: true },
    ],
    ...over,
  } as MessageInitShape<typeof NpcCheckCandidateSchema>;
  return create(NpcCheckCandidateSchema, init);
}

/** The bard's question: the die and the uses left, and the creature's name only when the bard's player may read it. */
export function cuttingOffer(over: MessageInitShape<typeof HeldRollOfferSchema> = {}): HeldRollOffer {
  const init = {
    holdId: 'hold-1',
    kind: HeldRollKind.NPC_CHECK,
    rollerLabel: 'Capitão bandido',
    createdAt: timestampFromMs(Date.UTC(2026, 10, 16, 21, 14)),
    question: { case: 'cuttingWords', value: { sides: 8, usesLeft: 3 } },
    ...over,
  } as MessageInitShape<typeof HeldRollOfferSchema>;
  return create(HeldRollOfferSchema, init);
}
