import { type MessageInitShape, create } from '@bufbuild/protobuf';

import {
  EffectModifierKind,
  EffectPhase,
  EffectRollKind,
  EffectSavePromptSchema,
  ExtraDieSchema,
  LastingEffectSchema,
  CharacterEffectSchema,
} from '../../../gen/meurpg/play/v1/lasting_effects_pb';
import {
  characterCards,
  characterConditions,
  combatCards,
  dieFieldHint,
  dieFields,
  effectSaveView,
  exhaustionLabel,
  exhaustionLines,
  genericDieFields,
  labelTags,
  missingDice,
  noteParts,
  rollDiceOf,
  saveFormula,
  signed,
} from './effects';

const hold = create(LastingEffectSchema, {
  id: 'e1',
  sourceNamePt: 'Imobilizar Pessoa',
  originPt: 'De alguém que você não vê',
  clockTextPt: 'Resta o tempo da magia: acaba no turno de quem a conjurou, na rodada 12.',
  tagsPt: [],
  changesPt: ['Você não age nem se move, e não fala.'],
  endSave: { ability: 'wis', abilityNamePt: 'Sabedoria', phase: EffectPhase.END },
});

const bless = create(LastingEffectSchema, {
  id: 'e2',
  sourceNamePt: 'Bênção',
  originPt: 'De Tavo',
  tagsPt: ['+1d4 em ataques e resistências'],
  modifiers: [
    {
      kind: EffectModifierKind.ROLL_DIE,
      die: 4,
      sign: 1,
      appliesTo: [EffectRollKind.ATTACK, EffectRollKind.SAVE],
    },
  ],
});

describe('the cards of "Seus efeitos"', () => {
  it('carry the name, whose it is, the labels, the clock and what changes, and never a DC', () => {
    const [card] = combatCards([hold]);
    expect(card.name).toBe('Imobilizar Pessoa');
    expect(card.origin).toBe('De alguém que você não vê');
    expect(card.clock).toContain('rodada 12');
    expect(card.save).toBe('No fim de cada turno seu: teste de resistência de Sabedoria.');
    expect(card.changes).toEqual(['Você não age nem se move, e não fala.']);
    expect(JSON.stringify(card)).not.toMatch(/CD|dc/);
  });

  it('say a save at the start of the turn as such, and none without one', () => {
    const start = create(LastingEffectSchema, {
      id: 'e3',
      sourceNamePt: 'Teia',
      startSave: { ability: 'dex', abilityNamePt: 'Destreza', phase: EffectPhase.START },
    });
    expect(combatCards([start])[0].save).toBe(
      'No começo de cada turno seu: teste de resistência de Destreza.',
    );
    expect(combatCards([bless])[0].save).toBe('');
    expect(combatCards(undefined)).toEqual([]);
  });

  it('read the time of an effect outside a combat from the character list, capitalized', () => {
    const outside = create(CharacterEffectSchema, {
      id: 'c1',
      sourceNamePt: 'Bênção',
      durationTextPt: 'dura 1 minuto',
      tagsPt: ['+1d4'],
      conditionNamesPt: ['Paralisado'],
    });
    expect(characterCards([outside])[0]).toMatchObject({
      name: 'Bênção',
      clock: 'Dura 1 minuto',
      origin: '',
      changes: [],
    });
    expect(characterConditions([outside, outside])).toEqual(['Paralisado']);
  });
});

describe('exhaustion', () => {
  it('adds the levels up from the first to the current one', () => {
    expect(exhaustionLabel(4)).toBe('Exaustão 4');
    expect(exhaustionLines(0)).toEqual([]);
    expect(exhaustionLines(4)).toEqual([
      'Desvantagem em testes de habilidade',
      'Deslocamento pela metade',
      'Desvantagem em ataques e testes de resistência',
      'PV máximos pela metade',
    ]);
    expect(exhaustionLines(9)).toHaveLength(6);
  });
});

describe('the dice an effect adds', () => {
  it('lists the d4 an attack takes from the effects the caller reads, in their order', () => {
    expect(rollDiceOf([bless, hold], EffectRollKind.ATTACK)).toEqual([
      { name: 'Bênção', faces: 4, sign: 1 },
    ]);
    expect(rollDiceOf(undefined, EffectRollKind.SAVE)).toEqual([]);
  });

  it('words the field, its hint and the generic one a refusal asks for', () => {
    const [field] = dieFields(rollDiceOf([bless], EffectRollKind.ATTACK));
    expect(field.label).toBe('Resultado do d4 (Bênção)');
    expect(dieFieldHint(field)).toBe('Bênção soma 1d4 a esta jogada. Role um d4 além do d20.');
    expect(dieFieldHint({ ...field, sign: -1, name: 'Perdição' })).toBe(
      'Perdição subtrai 1d4 desta jogada. Role um d4 além do d20.',
    );
    expect(genericDieFields(2, 4).map((f) => f.label)).toEqual([
      'Resultado do d4 (Outra fonte)',
      'Resultado do d4 (Outra fonte)',
    ]);
  });

  it('reads how many dice a refusal wants', () => {
    expect(missingDice('the roll takes 2 more die(s): type their faces in extra_die_faces')).toBe(
      2,
    );
    expect(missingDice('something else')).toBe(0);
  });

  it('writes the formula of a save with its modifier and the dice added', () => {
    const die = create(ExtraDieSchema, { sourceNamePt: 'Bênção', faces: 4, sign: 1, face: 3 });
    expect(saveFormula({ d20: 12, modifier: 1, extraDice: [] })).toBe('1d20 (12) + 1');
    expect(saveFormula({ d20: 12, modifier: -1, extraDice: [die] })).toBe(
      '1d20 (12) − 1 + 1d4 (3)',
    );
    expect(signed(-2)).toBe('−2');
    expect(signed(1)).toBe('+1');
  });
});

describe('the saving throw of an effect', () => {
  const prompt = (over: MessageInitShape<typeof EffectSavePromptSchema> = {}) =>
    create(EffectSavePromptSchema, {
      effectId: 'e1',
      sourceNamePt: 'Imobilizar Pessoa',
      ability: 'wis',
      abilityNamePt: 'Sabedoria',
      phase: EffectPhase.END,
      modifier: 1,
      bonusKnown: true,
      mode: 'normal',
      textPt:
        'Teste de resistência de Sabedoria. Se passar, o efeito acaba sobre você. Se falhar, ele continua.',
      ...over,
    });

  it('reads the ability, the modifier and what a pass does, never a DC', () => {
    const v = effectSaveView(prompt(), 3);
    expect(v).toMatchObject({
      title: 'Fim do seu turno',
      subtitle: 'Imobilizar Pessoa',
      ability: 'Sabedoria',
      consequence: 'Se passar, o efeito acaba sobre você. Se falhar, ele continua.',
      modifier: 'Seu modificador: +1.',
      mode: 'normal',
      autoFail: false,
    });
    expect(v.modeLine).toBe('Você não tem vantagem nem desvantagem neste teste.');
  });

  it('says the start of the turn, advantage with its sources and an unknown bonus', () => {
    const v = effectSaveView(
      prompt({
        phase: EffectPhase.START,
        mode: 'advantage',
        modeSourcesPt: ['Esquivando: vantagem'],
        bonusKnown: false,
      }),
      3,
    );
    expect(v.title).toBe('Começo do seu turno');
    expect(v.modeLine).toBe('Você tem vantagem neste teste (Esquivando: vantagem).');
    expect(v.modifier).toBe('');
    expect(effectSaveView(prompt({ mode: 'disadvantage' }), 3).modeLine).toBe(
      'Você tem desvantagem neste teste.',
    );
  });
});

describe('the words around the turn', () => {
  it('splits a note at its first sentence', () => {
    expect(noteParts('Você está Paralisada. Não age nem se move neste turno.')).toEqual({
      lead: 'Você está Paralisada.',
      rest: 'Não age nem se move neste turno.',
    });
    expect(noteParts('Sem pontos')).toEqual({ lead: 'Sem pontos', rest: '' });
  });

  it('lists the labels of the order once, after the conditions', () => {
    expect(
      labelTags(['Paralisado'], [{ textPt: 'Paralisado' }, { textPt: 'Preso numa teia' }]),
    ).toEqual(['Paralisado', 'Preso numa teia']);
    expect(labelTags([], undefined)).toEqual([]);
  });

  it('drops the label that only joins the names of conditions the combatant already has', () => {
    expect(
      labelTags(
        ['Envenenado', 'Derrubado'],
        [{ textPt: 'Envenenado, Derrubado' }, { textPt: 'Bênção' }],
      ),
    ).toEqual(['Envenenado', 'Derrubado', 'Bênção']);
    expect(labelTags(['Envenenado'], [{ textPt: 'Envenenado, Cego' }])).toEqual([
      'Envenenado',
      'Envenenado, Cego',
    ]);
  });
});
