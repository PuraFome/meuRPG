import {
  type Combatant,
  CombatantKind,
  CombatantSide,
  CombatantState,
  CoverDegree,
  type Encounter,
  EncounterStatus,
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
    turnGroupIds: [],
    npcOnlyGroups: [],
    opportunityOffers: [],
    deaths: [],
    revision: 1,
    ...over,
  } as unknown as Encounter;
}
