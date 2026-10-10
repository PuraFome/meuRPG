import { type MessageInitShape, create } from '@bufbuild/protobuf';

import {
  type CatalogEffect,
  CatalogEffectSchema,
  type CharacterEffect,
  CharacterEffectSchema,
  EffectAudience,
  EffectDurationKind,
  EffectPhase,
  EffectSourceKind,
  type LastingEffect,
  LastingEffectSchema,
  type TurnClockEntry,
  TurnClockEntrySchema,
  ConcentrationEntrySchema,
} from '../../../gen/meurpg/play/v1/lasting_effects_pb';
import {
  type ListLastingEffectsResponse,
  ListLastingEffectsResponseSchema,
} from '../../../gen/meurpg/play/v1/lasting_effects_service_pb';

/** A lasting effect for specs: a spell by the master on one target, visible to everyone, with no end text unless given. */
export function lastingEffect(
  over: MessageInitShape<typeof LastingEffectSchema> & { id: string },
): LastingEffect {
  return create(LastingEffectSchema, {
    groupId: `group-${over.id}`,
    encounterId: 'enc',
    targetIds: ['t1'],
    targetLabels: ['Toren'],
    sourceKind: EffectSourceKind.SPELL,
    sourceKey: 'spell:bless',
    sourceNamePt: 'Bênção',
    originPt: 'De Tavo',
    durationKind: EffectDurationKind.ROUNDS,
    clockTextPt: 'Restam 8 rodadas: acaba no turno de Tavo, na rodada 11.',
    endTextPt: 'Restam 8 rodadas: acaba no turno de Tavo, rodada 11.',
    playerVisible: true,
    audience: EffectAudience.ALL,
    ...over,
  });
}

export function catalogEffect(
  over: MessageInitShape<typeof CatalogEffectSchema> & { key: string },
): CatalogEffect {
  return create(CatalogEffectSchema, {
    namePt: over.key,
    sourceKind: EffectSourceKind.SPELL,
    defaultDurationKind: EffectDurationKind.ROUNDS,
    defaultRounds: 10,
    ...over,
  });
}

export function clockEntry(
  over: MessageInitShape<typeof TurnClockEntrySchema> & { effectId: string },
): TurnClockEntry {
  return create(TurnClockEntrySchema, {
    round: 3,
    combatantId: 'c1',
    combatantLabel: 'Goblin 2',
    phase: EffectPhase.END,
    isSave: true,
    textPt: 'Teste de Sabedoria (CD 14) contra Imobilizar Pessoa, de Orla.',
    ...over,
  });
}

export function characterEffect(
  over: MessageInitShape<typeof CharacterEffectSchema> & { id: string },
): CharacterEffect {
  return create(CharacterEffectSchema, {
    characterId: 'ch1',
    characterName: 'Toren',
    sourceKey: 'spell:longstrider',
    sourceNamePt: 'Passos Largos',
    durationTextPt: 'dura 1 hora',
    playerVisible: true,
    audience: EffectAudience.ALL,
    ...over,
  });
}

const DC_BRISA = 11;
const DC_GOBLIN = 14;

/** The panel of W7-E board 4: five effects, three clock entries and the concentrations. */
export function boardEffects(): ListLastingEffectsResponse {
  const hold = (id: string, caster: string, target: string, dc: number, label: string) =>
    lastingEffect({
      id,
      groupId: `g-${id}`,
      targetIds: [`id-${target}`],
      targetLabels: [target],
      sourceKey: 'spell:hold-person',
      sourceNamePt: 'Imobilizar Pessoa',
      originPt: `De ${caster}`,
      concentration: true,
      endTextPt: 'Resta 9 rodadas: acaba no turno de Orla, rodada 12.',
      endSave: {
        ability: 'wis',
        abilityNamePt: 'Sabedoria',
        phase: EffectPhase.END,
        dc,
      },
      playerVisible: true,
      playersSeePt: label,
    });
  return create(ListLastingEffectsResponseSchema, {
    round: 3,
    currentCombatantId: 'nael',
    effects: [
      hold('hold-1', 'Fanático do culto', 'Brisa', DC_BRISA, 'Paralisada'),
      hold('hold-2', 'Orla', 'Goblin 2', DC_GOBLIN, 'Paralisado'),
      lastingEffect({
        id: 'bless',
        groupId: 'g-bless',
        targetIds: ['toren', 'brisa', 'ragna'],
        targetLabels: ['Toren', 'Brisa', 'Ragna'],
        concentration: true,
        tagsPt: ['+1d4 em ataques e resistências'],
        playersSeePt: '',
      }),
      lastingEffect({
        id: 'dodge',
        groupId: 'g-dodge',
        targetIds: ['toren'],
        targetLabels: ['Toren'],
        sourceKind: EffectSourceKind.FEATURE,
        sourceKey: 'effect:dodging',
        sourceNamePt: 'Esquivando',
        originPt: 'Toren (Esquivar)',
        durationKind: EffectDurationKind.UNTIL_START_OF_TURN_OF,
        endTextPt:
          'Até o começo do turno de Toren, rodada 4, ou se ele ficar incapacitado ou com deslocamento 0.',
      }),
      lastingEffect({
        id: 'prone',
        groupId: 'g-prone',
        targetIds: ['hob'],
        targetLabels: ['Hobgoblin'],
        sourceKind: EffectSourceKind.CONDITION,
        sourceKey: 'condition:prone',
        sourceNamePt: 'Derrubado',
        originPt: 'Do mestre',
        durationKind: EffectDurationKind.UNTIL_DISMISSED,
        endTextPt: 'Até ele se levantar (gasta metade do deslocamento).',
      }),
    ],
    turnClock: [
      clockEntry({ effectId: 'hold-2', round: 3, combatantId: 'g2', combatantLabel: 'Goblin 2' }),
      clockEntry({
        effectId: 'hold-1',
        round: 4,
        combatantId: 'brisa',
        combatantLabel: 'Brisa',
        textPt: 'Teste de Sabedoria (CD 11) contra Imobilizar Pessoa, do Fanático.',
      }),
      clockEntry({
        effectId: 'dodge',
        round: 4,
        combatantId: 'toren',
        combatantLabel: 'Toren',
        phase: EffectPhase.START,
        isSave: false,
        textPt: 'Esquivando acaba.',
      }),
      clockEntry({
        effectId: 'bless',
        round: 11,
        combatantId: 'tavo',
        combatantLabel: 'Tavo',
        phase: EffectPhase.START,
        isSave: false,
        textPt: 'Bênção acaba (10 rodadas desde a rodada 1).',
      }),
    ],
    concentrations: [
      create(ConcentrationEntrySchema, {
        casterId: 'tavo',
        casterLabel: 'Tavo',
        spellKey: 'spell:bless',
        spellNamePt: 'Bênção',
        effectIds: ['bless'],
      }),
      create(ConcentrationEntrySchema, {
        casterId: 'orla',
        casterLabel: 'Orla',
        spellKey: 'spell:hold-person',
        spellNamePt: 'Imobilizar Pessoa',
        effectIds: ['hold-2'],
      }),
    ],
    catalog: [
      catalogEffect({ key: 'spell:bless', namePt: 'Bênção', concentration: true, hasCaster: true }),
      catalogEffect({
        key: 'spell:haste',
        namePt: 'Velocidade',
        concentration: true,
        hasCaster: true,
      }),
      catalogEffect({
        key: 'condition:prone',
        namePt: 'Derrubado',
        sourceKind: EffectSourceKind.CONDITION,
        defaultDurationKind: EffectDurationKind.UNTIL_DISMISSED,
        defaultRounds: 0,
      }),
    ],
  });
}
