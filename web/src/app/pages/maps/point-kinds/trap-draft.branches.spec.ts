import { create } from '@bufbuild/protobuf';
import { describe, expect, it } from 'vitest';

import { MapPointSchema, TrapState } from '../../../../gen/meurpg/maps/v1/maps_pb';
import {
  Ability,
  TrapPassOutcome,
  TrapPresetSchema,
  TrapSaveApplies,
  TrapTargets,
  TrapTrigger,
} from '../../../../gen/meurpg/rules/v1/rules_pb';
import {
  blankTrapDraft,
  hasTrapErrors,
  newCondition,
  newSave,
  presetSummary,
  trapChangesOf,
  trapDraftFromPreset,
  trapDraftOf,
  trapErrors,
} from './trap-draft';

/** The branches the other trap-draft specs leave: each error text, each fallback of a half-filled preset or point. */
describe('trapErrors, field by field', () => {
  const ok = { ...blankTrapDraft('A'), findDc: '15' };

  it('says what is wrong with a name: too long, or not on one line', () => {
    expect(trapErrors({ ...ok, name: 'a'.repeat(81) }).name).toBe('Use até 80 caracteres.');
    expect(trapErrors({ ...ok, name: 'a\nb' }).name).toBe('Use um nome numa linha só.');
    expect(trapErrors({ ...ok, name: ' ' }).name).toBe('Dê um nome ao ponto.');
    expect(hasTrapErrors(trapErrors(ok))).toBe(false);
  });

  it('counts characters, not code units, and limits the description', () => {
    expect(trapErrors({ ...ok, name: '🎲'.repeat(80) }).name).toBeUndefined();
    expect(trapErrors({ ...ok, description: 'x'.repeat(2001) }).description).toBe(
      'Use até 2.000 caracteres.',
    );
    expect(trapErrors({ ...ok, description: 'x'.repeat(2000) }).description).toBeUndefined();
  });

  it('limits what a condition lasts to 60 characters, on the part and on the failure', () => {
    const long = 'h'.repeat(61);
    const e = trapErrors({
      ...ok,
      conditions: [{ ...newCondition(), duration: long }, newCondition()],
      save: {
        ...newSave(),
        dc: '12',
        failDamage: [{ dice: 'x', typeKey: 'damage-type:fire' }],
        failCondition: { key: 'condition:prone', duration: long },
      },
    });
    expect(e.parts).toEqual({
      'condition:0': 'Use até 60 caracteres.',
      'fail:0': 'Use dados como 2d6 (de d4 a d12) ou um número de 1 a 100.',
      failCondition: 'Use até 60 caracteres.',
    });
  });

  it('checks the damage dice of the attack', () => {
    const e = trapErrors({
      ...ok,
      attack: { bonus: '5', count: '1', damage: { dice: 'oops', typeKey: 'damage-type:fire' } },
    });
    expect(e.attackDice).toContain('2d6');
    expect(e.attackBonus).toBeUndefined();
  });

  it('a save of "quem foi atingido" with an attack, and a clean failure, has no error', () => {
    const e = trapErrors({
      ...ok,
      attack: { bonus: '5', count: '1', damage: { dice: '1d6', typeKey: 'damage-type:fire' } },
      save: {
        ...newSave(),
        dc: '12',
        appliesTo: TrapSaveApplies.HIT,
        failDamage: [{ dice: '1d6', typeKey: 'damage-type:fire' }],
        onPass: TrapPassOutcome.HALF,
      },
    });
    expect(hasTrapErrors(e)).toBe(false);
  });
});

describe('the form from a preset or a point with parts missing', () => {
  it('fills an attack without damage with a blank bludgeoning part, and a save with every part', () => {
    const d = trapDraftFromPreset(
      create(TrapPresetSchema, {
        key: 'trap:x',
        namePt: 'X',
        noticeDc: 12,
        findDc: 14,
        areaSize: 4,
        trigger: TrapTrigger.MANUAL,
        effect: {
          targets: TrapTargets.MANUAL,
          attack: { bonus: 7, count: 2 },
          save: {
            ability: Ability.WISDOM,
            dc: 13,
            appliesTo: TrapSaveApplies.HIT,
            onPass: TrapPassOutcome.HALF,
            onFail: {
              damage: [{ dice: '1d4', damageTypeKey: 'damage-type:fire' }],
              condition: { conditionKey: 'condition:blinded', durationPt: '1 min' },
            },
          },
        },
      }),
    );
    expect(d).toMatchObject({
      noticeDc: '12',
      areaSize: 4,
      trigger: TrapTrigger.MANUAL,
      targets: TrapTargets.MANUAL,
      attack: { bonus: '7', count: '2', damage: { dice: '', typeKey: 'damage-type:bludgeoning' } },
      save: {
        ability: Ability.WISDOM,
        dc: '13',
        appliesTo: TrapSaveApplies.HIT,
        onPass: TrapPassOutcome.HALF,
        failDamage: [{ dice: '1d4', typeKey: 'damage-type:fire' }],
        failCondition: { key: 'condition:blinded', duration: '1 min' },
      },
    });
  });

  it('reads an area of 3, and falls back to a square, "ao entrar" and armed for what it does not know', () => {
    const three = trapDraftFromPreset(create(TrapPresetSchema, { areaSize: 3 }));
    expect(three.areaSize).toBe(3);
    expect(three.trigger).toBe(TrapTrigger.ENTER);
    const bare = trapDraftOf({ name: 'N', description: 'D', trap: undefined });
    expect(bare).toMatchObject({
      presetKey: '',
      findDc: '',
      noticeDc: '',
      areaSize: 1,
      trigger: TrapTrigger.ENTER,
      state: TrapState.ARMED,
      targets: TrapTargets.AREA,
      attack: null,
      save: null,
    });
    const fired = trapDraftOf(
      create(MapPointSchema, {
        name: 'N',
        trap: { findDc: 12, areaSize: 4, trigger: TrapTrigger.MANUAL, state: TrapState.TRIGGERED },
      }),
    );
    expect(fired).toMatchObject({
      areaSize: 4,
      trigger: TrapTrigger.MANUAL,
      state: TrapState.TRIGGERED,
    });
  });

  it('sends a changed description along with the spec', () => {
    const point = create(MapPointSchema, { name: 'N', description: 'old', trap: { findDc: 12 } });
    const changes = trapChangesOf({ ...trapDraftOf(point), description: 'new' }, point);
    expect(changes?.description).toBe('new');
    expect(changes?.name).toBeUndefined();
  });
});

describe('presetSummary, the other shapes', () => {
  it('names an attack with and without damage, the damage, and the ability of the save', () => {
    expect(
      presetSummary(
        create(TrapPresetSchema, {
          effect: {
            attack: { bonus: 5, damage: { dice: '1d6', damageTypePt: 'fogo' } },
            damage: [{ dice: '2d4', damageTypePt: 'veneno' }],
            save: { ability: Ability.CONSTITUTION },
          },
        }),
      ),
    ).toBe('Ataque +5, 1d6 fogo · 2d4 veneno · resistência de Constituição');
    expect(presetSummary(create(TrapPresetSchema, { effect: { attack: { bonus: 3 } } }))).toBe(
      'Ataque +3',
    );
    expect(
      presetSummary(
        create(TrapPresetSchema, { effect: { save: { ability: Ability.UNSPECIFIED } } }),
      ),
    ).toBe('Resistência de habilidade');
    expect(presetSummary(create(TrapPresetSchema, {}))).toBe('Só descrição');
  });
});
