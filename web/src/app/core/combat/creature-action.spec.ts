import { create } from '@bufbuild/protobuf';
import { describe, expect, it } from 'vitest';

import {
  AreaPlacement,
  CreatureActionResultSchema,
  SaveOutcome,
} from '../../../gen/meurpg/play/v1/combat_pb';
import {
  Ability,
  CreatureActionClass,
  CreatureActionKind,
  CreatureAreaShape,
  SpellAreaShape,
  SpellSaveSuccess,
} from '../../../gen/meurpg/rules/v1/rules_pb';
import { creatureAction } from './combat-testing';
import {
  actionLines,
  areaPlacementOf,
  areaShapeOf,
  classNote,
  creatureAreaText,
  durationPt,
  resultNotes,
  showsText,
  targetLines,
  usableActions,
  usageText,
} from './creature-action';

/** The text with its no-break spaces as plain ones. */
const plain = (t: string) => t.replace(/\u00a0/g, ' ');

const fireBreath = () =>
  creatureAction({
    key: 'monster:hell-hound#fire-breath',
    name: 'Fire Breath',
    namePt: 'Sopro de Fogo',
    text: 'The hound exhales fire in a 15-foot cone.',
    usage: 'Recharge 5-6',
    area: { shape: CreatureAreaShape.CONE, sizeFt: 15, widthFt: 0 },
    save: {
      ability: Ability.DEXTERITY,
      dc: 13,
      onSuccess: SpellSaveSuccess.HALF,
      damage: [{ dice: '6d6', damageTypeKey: 'damage:fire', damageTypePt: 'fogo' }],
    },
  } as never);

describe('usableActions', () => {
  it('keeps the saves and the no-roll actions, and leaves attacks and the Multiattack out', () => {
    const attack = creatureAction({ key: 'm#bite', kind: CreatureActionKind.ATTACK });
    const multi = creatureAction({ key: 'm#multiattack', kind: CreatureActionKind.MULTIATTACK });
    const other = creatureAction({ key: 'm#etherealness', kind: CreatureActionKind.OTHER });
    expect(usableActions([attack, fireBreath(), multi, other]).map((a) => a.key)).toEqual([
      'monster:hell-hound#fire-breath',
      'm#etherealness',
    ]);
    expect(usableActions(undefined)).toEqual([]);
  });
});

describe('actionLines: what the plan says, in Portuguese', () => {
  it('writes the save, the area, the damage with "metade se passar"', () => {
    expect(actionLines(fireBreath()).map(plain)).toEqual([
      'Teste de resistência de Destreza, CD 13 · cone de 4,5 m',
      'Dano: 6d6 de fogo, metade se passar',
    ]);
  });

  it('writes the condition with its duration and the repeated save', () => {
    const gaze = creatureAction({
      key: 'm#paralyzing-gaze',
      save: {
        ability: Ability.WISDOM,
        dc: 14,
        onSuccess: SpellSaveSuccess.NONE,
        damage: [],
        conditionKey: 'condition:paralyzed',
        conditionPt: 'Paralisado',
        duration: '1 minute',
        repeatSave: true,
      },
    } as never);
    expect(actionLines(gaze)).toEqual([
      'Teste de resistência de Sabedoria, CD 14',
      'Se falhar: Paralisado por 1 minuto; repete o teste no fim de cada turno dele',
    ]);
  });

  it('says an action with no roll spends the action and leaves the text as the reminder', () => {
    const other = creatureAction({ key: 'm#etherealness', kind: CreatureActionKind.OTHER });
    expect(actionLines(other)).toEqual(['Sem teste: gasta a ação e lembra o texto.']);
  });
});

describe('the class of the plan', () => {
  it('shows the SRD text for PARTIAL and TEXT, and for an action with no roll, never for a STRUCTURED save', () => {
    const partial = creatureAction({ key: 'm#a', planClass: CreatureActionClass.PARTIAL });
    const text = creatureAction({ key: 'm#b', planClass: CreatureActionClass.TEXT });
    const other = creatureAction({ key: 'm#c', kind: CreatureActionKind.OTHER });
    expect(showsText(fireBreath())).toBe(false);
    expect(classNote(fireBreath())).toBe('');
    expect(showsText(partial)).toBe(true);
    expect(classNote(partial)).toContain('o app não calcula');
    expect(showsText(text)).toBe(true);
    expect(classNote(text)).toContain('O app não calcula esta ação');
    expect(showsText(other)).toBe(true);
  });
});

describe('the area', () => {
  it('asks for a direction for a cone, a line and a cube, a point for a sphere, nothing for a radius, null for no area', () => {
    const shape = (s: CreatureAreaShape) =>
      areaPlacementOf(
        creatureAction({ key: 'm#x', area: { shape: s, sizeFt: 30, widthFt: 5 } } as never),
      );
    expect(shape(CreatureAreaShape.CONE)).toBe(AreaPlacement.DIRECTION);
    expect(shape(CreatureAreaShape.LINE)).toBe(AreaPlacement.DIRECTION);
    expect(shape(CreatureAreaShape.CUBE)).toBe(AreaPlacement.DIRECTION);
    expect(shape(CreatureAreaShape.SPHERE)).toBe(AreaPlacement.POINT);
    expect(shape(CreatureAreaShape.CYLINDER)).toBe(AreaPlacement.POINT);
    expect(shape(CreatureAreaShape.RADIUS)).toBe(AreaPlacement.CASTER);
    expect(areaPlacementOf(creatureAction({ key: 'm#y' }))).toBeNull();
  });

  it('hands the picker the same shapes the spells have, and words each one', () => {
    expect(areaShapeOf(fireBreath())).toEqual({
      shape: SpellAreaShape.CONE,
      sizeFt: 15,
      widthFt: 0,
    });
    expect(creatureAreaText(fireBreath().area)).toBe('cone de 4,5 m');
    expect(
      creatureAreaText({ shape: CreatureAreaShape.LINE, sizeFt: 60, widthFt: 5 } as never),
    ).toBe('linha de 18 m × 1,5 m');
    expect(creatureAreaText({ shape: CreatureAreaShape.RADIUS, sizeFt: 10 } as never)).toBe(
      'raio de 3 m ao redor dele',
    );
    expect(creatureAreaText(undefined)).toBe('');
  });
});

describe('durationPt', () => {
  it('translates the SRD words of a duration and keeps any other wording', () => {
    expect(durationPt('1 minute')).toBe('1 minuto');
    expect(durationPt('10 minutes')).toBe('10 minutos');
    expect(durationPt('1 hour')).toBe('1 hora');
    expect(durationPt('until the end of its next turn')).toBe('until the end of its next turn');
  });
});

describe('usageText', () => {
  it('translates the SRD limits of an action, with an en dash between the numbers of a recharge', () => {
    expect(usageText('Recharge 5-6')).toBe('Recarga 5\u20136');
    expect(usageText('Recharge 4-6')).toBe('Recarga 4\u20136');
    expect(usageText('Recharge 6')).toBe('Recarga 6');
    expect(usageText('1/Day')).toBe('1/dia');
    expect(usageText('3/day')).toBe('3/dia');
    expect(usageText('4/Day')).toBe('4/dia');
    expect(usageText('Recharges after a Short or Long Rest')).toBe(
      'Recarrega após um descanso curto ou longo',
    );
    expect(usageText('Recharges after a short or long rest')).toBe(
      'Recarrega após um descanso curto ou longo',
    );
    expect(usageText('Recharges after a Long Rest')).toBe('Recarrega após um descanso longo');
  });

  it('keeps any other wording as the SRD has it, and an empty limit empty', () => {
    expect(usageText('')).toBe('');
    expect(usageText('Costs 2 Actions')).toBe('Costs 2 Actions');
  });
});

describe('what a used action did', () => {
  const result = create(CreatureActionResultSchema, {
    actionKey: 'm#paralyzing-gaze',
    conditionKey: 'condition:paralyzed',
    conditionNamePt: 'Paralisado',
    durationText: '1 minute',
    repeatSave: true,
    targets: [
      {
        combatantId: 't1',
        save: {
          outcome: SaveOutcome.FAILED,
          dc: 14,
          roll: { diceCount: 1, diceSides: 20, faces: [7], modifier: 2, total: 9 },
        },
        conditionApplied: true,
      },
      {
        combatantId: 't2',
        save: {
          outcome: SaveOutcome.SAVED,
          dc: 14,
          roll: { diceCount: 1, diceSides: 20, faces: [16], modifier: 2, total: 18 },
        },
      },
    ],
  });
  const labels = new Map([
    ['t1', 'Brisa'],
    ['t2', 'Toren'],
  ]);

  it('writes each save against the DC, the word, and the condition a failure gave', () => {
    const lines = targetLines(result, labels, true);
    expect(lines[0]).toMatchObject({
      label: 'Brisa',
      word: 'Falhou',
      saved: false,
      condition: 'Ficou Paralisado',
    });
    expect(lines[0].roll).toContain('contra CD 14');
    expect(lines[1]).toMatchObject({ label: 'Toren', word: 'Passou: metade do dano', saved: true });
    expect(lines[1].condition).toBe('');
    expect(targetLines(result, labels, false)[1].word).toBe('Passou');
  });

  it('keeps the duration and the repeated save as notes for the master', () => {
    expect(resultNotes(result)).toEqual([
      'Paralisado por 1 minuto.',
      'O alvo repete o teste no fim de cada turno dele; o app não conta o tempo.',
    ]);
  });
});
