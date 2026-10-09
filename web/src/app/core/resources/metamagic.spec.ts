import { create } from '@bufbuild/protobuf';

import { MetamagicOptionSchema } from '../../../gen/meurpg/rules/v1/rules_pb';
import {
  CAREFUL_KEY,
  EMPOWERED_KEY,
  HEIGHTENED_KEY,
  NO_PICKS,
  TWINNED_KEY,
  castLabel,
  chosenLine,
  metamagicChoices,
  metamagicCost,
  metamagicMissing,
  metamagicName,
  metamagicRows,
  metamagicSpentLine,
  toggledOption,
  validChoice,
} from './metamagic';

const option = (key: string, namePt: string, cost: number, allowed = true, why = '') =>
  create(MetamagicOptionSchema, {
    key,
    namePt,
    cost,
    summaryPt: `${namePt}: efeito`,
    allowed,
    disabledReasonPt: why,
  });

const twinned = option(TWINNED_KEY, 'Magia Duplicada', 1);
const careful = option(
  CAREFUL_KEY,
  'Magia Cuidadosa',
  1,
  false,
  'O Raio de Gelo não pede teste de resistência.',
);
const empowered = option(EMPOWERED_KEY, 'Magia Potencializada', 1);
const heightened = option(HEIGHTENED_KEY, 'Magia Aumentada', 3);
const all = [twinned, careful, empowered, heightened];

describe('the choice of Metamagic (rules.CheckMetamagicChoice)', () => {
  it('takes none, one, or Empowered Spell with one other', () => {
    expect(validChoice([])).toBe(true);
    expect(validChoice([TWINNED_KEY])).toBe(true);
    expect(validChoice([TWINNED_KEY, EMPOWERED_KEY])).toBe(true);
    expect(validChoice([TWINNED_KEY, HEIGHTENED_KEY])).toBe(false);
    expect(validChoice([TWINNED_KEY, EMPOWERED_KEY, HEIGHTENED_KEY])).toBe(false);
  });

  it('marks one option, refuses a second one that cannot join it, and unmarks', () => {
    expect(toggledOption([], TWINNED_KEY)).toEqual([TWINNED_KEY]);
    expect(toggledOption([TWINNED_KEY], HEIGHTENED_KEY)).toEqual([TWINNED_KEY]);
    expect(toggledOption([TWINNED_KEY], EMPOWERED_KEY)).toEqual([TWINNED_KEY, EMPOWERED_KEY]);
    expect(toggledOption([TWINNED_KEY], TWINNED_KEY)).toEqual([]);
  });
});

describe('metamagicRows', () => {
  it('shows only the options the server listed, each with its cost, and greys the one the spell does not take with the server reason', () => {
    const rows = metamagicRows(all, []);
    expect(rows.map((r) => [r.name, r.cost, r.blocked])).toEqual([
      ['Magia Duplicada', '1 ponto', ''],
      ['Magia Cuidadosa', '1 ponto', 'O Raio de Gelo não pede teste de resistência.'],
      ['Magia Potencializada', '1 ponto', ''],
      ['Magia Aumentada', '3 pontos', ''],
    ]);
  });

  it('greys the others with the reason once one is marked, except Empowered Spell, which adds to it', () => {
    const rows = metamagicRows(all, [TWINNED_KEY]);
    expect(rows.find((r) => r.key === HEIGHTENED_KEY)?.blocked).toContain('Só uma opção por magia');
    expect(rows.find((r) => r.key === EMPOWERED_KEY)?.blocked).toBe('');
    expect(rows.find((r) => r.key === TWINNED_KEY)?.checked).toBe(true);
  });
});

describe('the cost and the words', () => {
  it('adds the cost of the options chosen', () => {
    expect(metamagicCost(all, [HEIGHTENED_KEY, EMPOWERED_KEY])).toBe(4);
    expect(metamagicCost(all, [])).toBe(0);
  });

  it('says what was chosen and what the cast button costs', () => {
    expect(chosenLine(all, [TWINNED_KEY])).toBe('Você escolheu: Magia Duplicada (1 ponto).');
    expect(chosenLine(all, [])).toBe('');
    expect(castLabel('Raio de Gelo', 1)).toBe('Conjurar e gastar 1 ponto');
    expect(castLabel('Raio de Gelo', 4)).toBe('Conjurar e gastar 4 pontos');
    expect(castLabel('Raio de Gelo', 0)).toBe('Conjurar Raio de Gelo');
  });

  it('names the options for the log, and a key it does not know "Metamagia"', () => {
    expect(metamagicName(TWINNED_KEY)).toBe('Magia Duplicada');
    expect(metamagicName('feature:metamagic-new')).toBe('Metamagia');
  });

  it('says what was spent and what is left', () => {
    expect(metamagicSpentLine(['Magia Duplicada'], 1, { left: 5, total: 5 })).toBe(
      'Magia Duplicada · gastou 1 ponto de feitiçaria (restam 4 de 5)',
    );
    expect(metamagicSpentLine(['Magia Duplicada', 'Magia Potencializada'], 2, null)).toBe(
      'Magia Duplicada e Magia Potencializada · gastou 2 pontos de feitiçaria',
    );
  });
});

describe('what a cast with Metamagic needs', () => {
  const points = { left: 2, total: 5 };

  it('needs nothing without a choice', () => {
    expect(metamagicMissing(all, [], NO_PICKS, points)).toBe('');
  });

  it('needs the points, then the creatures each option asks for', () => {
    expect(metamagicMissing(all, [HEIGHTENED_KEY], NO_PICKS, points)).toBe(
      'Faltam pontos de feitiçaria: custa 3 pontos e você tem 2.',
    );
    expect(metamagicMissing(all, [TWINNED_KEY], NO_PICKS, points)).toBe(
      'Escolha o segundo alvo da Magia Duplicada.',
    );
    expect(metamagicMissing(all, [TWINNED_KEY], { ...NO_PICKS, twinned: 'b' }, points)).toBe('');
    expect(metamagicMissing(all, [CAREFUL_KEY], NO_PICKS, points)).toBe(
      'Escolha as criaturas da Magia Cuidadosa.',
    );
    expect(metamagicMissing(all, [HEIGHTENED_KEY], NO_PICKS, { left: 5, total: 5 })).toBe(
      'Escolha o alvo da Magia Aumentada.',
    );
  });

  it('builds the request entries with the creatures of each option', () => {
    expect(
      metamagicChoices([TWINNED_KEY, EMPOWERED_KEY], {
        twinned: 'b',
        careful: ['x'],
        heightened: '',
      }),
    ).toEqual([
      { key: TWINNED_KEY, targetIds: ['b'], carefulIds: [], heightenedId: '' },
      { key: EMPOWERED_KEY, targetIds: [], carefulIds: [], heightenedId: '' },
    ]);
    expect(
      metamagicChoices([CAREFUL_KEY], { twinned: '', careful: ['x', 'y'], heightened: '' }),
    ).toEqual([{ key: CAREFUL_KEY, targetIds: [], carefulIds: ['x', 'y'], heightenedId: '' }]);
  });
});
