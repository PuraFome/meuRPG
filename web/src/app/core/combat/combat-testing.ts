import {
  type Combatant,
  CombatantKind,
  CombatantSide,
  CombatantState,
  CoverDegree,
  type Encounter,
  EncounterStatus,
  ReactionKind,
  type ReactionWindow,
  ReactionWindowStatus,
} from '../../../gen/meurpg/play/v1/combat_pb';

/** A combatant for specs: an NPC unless `kind` says otherwise. */
export function combatant(over: Partial<Combatant> & { id: string; label: string }): Combatant {
  return {
    kind: CombatantKind.NPC,
    mine: false,
    characterId: `char-${over.id}`,
    hidden: false,
    tieUnresolved: false,
    placed: true,
    col: 0,
    row: 0,
    speedFt: 30,
    movementUsedFt: 0,
    movementLeftFt: 30,
    speedDft: 300,
    movementUsedDft: 0,
    movementLeftDft: 300,
    dashed: false,
    actionUsed: false,
    bonusActionUsed: false,
    reactionUsed: false,
    state: CombatantState.UNHURT,
    defeated: false,
    deathSuccesses: 0,
    deathFailures: 0,
    conditions: [],
    conditionNamesPt: [],
    concentrationSpell: '',
    armorClassBonus: 0,
    deathSaveDue: false,
    turnPartEnded: false,
    side: CombatantSide.ENEMY,
    coverMark: CoverDegree.NONE,
    disengaged: false,
    bestiaryCreatureKey: '',
    challengeRating: '',
    ...over,
  } as unknown as Combatant;
}

/** An encounter for specs: ACTIVE, round 2, on a 20 x 14 grid. */
export function encounter(over: Partial<Encounter> = {}): Encounter {
  return {
    id: 'enc',
    name: 'Emboscada na estrada',
    status: EncounterStatus.ACTIVE,
    round: 2,
    mapId: 'map',
    gridColumns: 20,
    gridRows: 14,
    currentCombatantId: '',
    masterTurn: false,
    combatants: [],
    reactionPrompts: [],
    reactionWindows: [],
    turnGroupIds: [],
    npcOnlyGroups: [],
    opportunityOffers: [],
    revision: 1,
    ...over,
  } as unknown as Encounter;
}

/** A reaction window for specs: open, the first of its group, for the master, with no prompt unless `prompt` says. */
export function reactionWindow(over: Partial<ReactionWindow> & { id: string }): ReactionWindow {
  return {
    kind: ReactionKind.SHIELD,
    status: ReactionWindowStatus.OPEN,
    closedReason: 0,
    groupId: `group-${over.id}`,
    reactorId: '',
    reactorLabel: '',
    reactorIsPlayer: false,
    forYou: true,
    answerNow: true,
    secondStep: false,
    prompt: { case: undefined, value: undefined },
    ...over,
  } as unknown as ReactionWindow;
}
