import { create, type MessageInitShape } from '@bufbuild/protobuf';
import { describe, expect, it } from 'vitest';

import {
  EffectLabelSchema,
  EffectModifierKind,
  EffectModifierSchema,
  LastingEffectSchema,
  WeaponHitDamageSchema,
} from '../../../gen/meurpg/play/v1/lasting_effects_pb';
import { combatant } from '../combat/combat-testing';
import { lastingEffect } from './effects-testing';
import { hitCards, hitCardLine, markedNameOf, timeLeftText } from './hit-effect-text';

type HitOf = MessageInitShape<typeof WeaponHitDamageSchema>;

function hit(over: HitOf = {}) {
  return create(WeaponHitDamageSchema, {
    trigger: 'next_hit',
    die: 6,
    dice: 2,
    damageTypeKey: 'damage-type:radiant',
    damageTypePt: 'radiante',
    ...over,
  });
}

function row(
  id: string,
  over: HitOf = {},
  extra: MessageInitShape<typeof LastingEffectSchema> = {},
) {
  return lastingEffect({
    id,
    sourceKey: 'spell:blade-flare',
    sourceNamePt: 'Clarão na Lâmina',
    casterId: 'me',
    targetIds: ['me'],
    targetLabels: ['Brisa'],
    roundsLeft: 10,
    modifiers: [
      create(EffectModifierSchema, { kind: EffectModifierKind.WEAPON_HIT_DAMAGE, hit: hit(over) }),
    ],
    ...extra,
  });
}

describe('time left', () => {
  it('writes minutes, hours and rounds the way the form chip does', () => {
    expect(timeLeftText(58 * 60)).toBe('58 min restantes');
    expect(timeLeftText(3600)).toBe('1 h restantes');
    expect(timeLeftText(3600 + 52 * 60)).toBe('1 h 52 min restantes');
    expect(timeLeftText(24)).toBe('Restam 4 rodadas');
    expect(timeLeftText(6)).toBe('Resta 1 rodada');
  });
});

describe('the sentence of a hit effect', () => {
  it('reads the next hit with the damage type, no time on a concentration with no end', () => {
    const e = row('a', {}, { roundsLeft: undefined });
    expect(hitCardLine(e, '')).toBe('No próximo golpe com arma: +2d6 radiante.');
  });

  it('adds the time left after the damage', () => {
    expect(hitCardLine(row('a', {}, { roundsLeft: 580 }), '')).toBe(
      'No próximo golpe com arma: +2d6 radiante. 58 min restantes.',
    );
  });

  it('says the weapon type when the effect names none', () => {
    const e = row('a', { trigger: 'each_hit', dice: 1, damageTypeKey: '', damageTypePt: '' });
    expect(hitCardLine(e, '')).toBe('Cada golpe com arma: +1d6 do tipo da arma. 1 min restantes.');
  });

  it('names the melee trigger and the marked creature', () => {
    expect(hitCardLine(row('a', { trigger: 'next_melee_hit' }), '')).toContain(
      'No próximo golpe corpo a corpo: ',
    );
    expect(hitCardLine(row('a', { trigger: 'marked_target' }), 'Wight')).toContain(
      'Cada golpe com arma no Wight: ',
    );
    expect(hitCardLine(row('a', { trigger: 'marked_target' }), '')).toContain(
      'Cada golpe com arma no alvo marcado: ',
    );
  });

  it('says the saving throw with the conditions and how long they last', () => {
    const e = row('a', {
      save: 'con',
      conditionKeys: ['condition:blinded'],
      conditionNamesPt: ['Cego'],
      conditionDuration: 'next_turn',
    });
    expect(hitCardLine(e, '')).toContain(
      '+2d6 radiante; teste de resistência de Constituição ou fica Cego até o fim do próximo turno dele.',
    );
    const spell = row('b', {
      save: 'wis',
      conditionKeys: ['condition:blinded', 'condition:deafened'],
      conditionDuration: 'spell_end',
    });
    expect(hitCardLine(spell, '')).toContain(
      'teste de resistência de Sabedoria ou fica Cego e Surdo até o fim da magia.',
    );
  });

  it('says a saving throw that gives no condition by itself', () => {
    expect(hitCardLine(row('a', { save: 'con' }), '')).toContain(
      '+2d6 radiante; teste de resistência de Constituição.',
    );
  });
});

describe('the cards of the caster', () => {
  const me = combatant({ id: 'me', label: 'Brisa', mine: true });

  it('lists the caster rows with a hit, not the spent ones nor another caster', () => {
    const own = {
      ...me,
      effects: [
        row('a'),
        row('spent', {}, { spent: true }),
        row('other', {}, { casterId: 'x' }),
        lastingEffect({ id: 'bless', casterId: 'me' }),
      ],
    };
    const cards = hitCards(own, [own]);
    expect(cards.map((c) => c.key)).toEqual(['a']);
    expect(cards[0]).toMatchObject({
      title: 'Clarão na Lâmina',
      concentration: false,
      line: 'No próximo golpe com arma: +2d6 radiante. 1 min restantes.',
    });
  });

  it('shows the concentration chip only for a concentration effect', () => {
    const own = { ...me, effects: [row('a', {}, { concentration: true })] };
    expect(hitCards(own, [own])[0].concentration).toBe(true);
  });

  it('names the marked creature in the title and the line from marked_combatant_id', () => {
    const wight = combatant({ id: 'w', label: 'Wight' });
    const zumbi = combatant({ id: 'z', label: 'Zumbi' });
    const own = {
      ...me,
      effects: [
        row(
          'a',
          { trigger: 'marked_target', dice: 1, damageTypeKey: '', damageTypePt: '' },
          {
            sourceNamePt: 'Sinal da Presa',
            concentration: true,
            roundsLeft: 580,
            markedCombatantId: 'w',
          },
        ),
      ],
    };
    const [card] = hitCards(own, [own, wight, zumbi]);
    expect(card.title).toBe('Sinal da Presa no Wight');
    expect(card.line).toBe('Cada golpe com arma no Wight: +1d6 do tipo da arma. 58 min restantes.');
    expect(markedNameOf(own.effects[0], [own, wight, zumbi])).toBe('Wight');
  });

  it('never guesses the mark from a label: with no id it says "no alvo marcado"', () => {
    const label = [create(EffectLabelSchema, { effectId: 'm', textPt: 'Sinal da Presa' })];
    const wight = { ...combatant({ id: 'w', label: 'Wight' }), effectLabels: label };
    const e = row('a', { trigger: 'marked_target' }, { sourceNamePt: 'Sinal da Presa' });
    expect(markedNameOf(e, [me, wight])).toBe('');
    const [card] = hitCards({ ...me, effects: [e] }, [me, wight]);
    expect(card.title).toBe('Sinal da Presa no alvo marcado');
    expect(card.line).toContain('Cada golpe com arma no alvo marcado');
  });

  it('says "no alvo marcado" when the marked creature is not in the combatant list', () => {
    const e = row('a', { trigger: 'marked_target' }, { markedCombatantId: 'gone' });
    const [card] = hitCards({ ...me, effects: [e] }, [me, combatant({ id: 'w', label: 'Wight' })]);
    expect(card.title).toBe('Clarão na Lâmina no alvo marcado');
    expect(card.line).toContain('no alvo marcado');
  });

  it('names no one for an effect that marks nobody', () => {
    const e = row('a', { trigger: 'next_hit' }, { markedCombatantId: 'w' });
    expect(hitCards({ ...me, effects: [e] }, [me])[0].title).toBe('Clarão na Lâmina');
  });
});
