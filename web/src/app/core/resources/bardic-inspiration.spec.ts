import { create } from '@bufbuild/protobuf';

import {
  CombatantState,
  InspirationDieSchema,
  ResourceTargetSchema,
  TargetInReachSchema,
} from '../../../gen/meurpg/play/v1/combat_pb';
import {
  dieCard,
  expiresText,
  inspirationDieName,
  promptTitle,
  useCost,
  useLabel,
} from './bardic-inspiration';
import { resourceRows } from './resource-targets';
import { plainText } from './plain-text';

const die = create(InspirationDieSchema, {
  sides: 8,
  fromLabel: 'Orla',
  fromCombatantId: 'o',
  expiresAtRound: 10,
});

describe('the die of Bardic Inspiration', () => {
  it('names the die and asks the question of the board', () => {
    expect(inspirationDieName(8)).toBe('d8');
    expect(promptTitle(die)).toBe('Usar a Inspiração de Bardo (d8)?');
    expect(useLabel(die)).toBe('Somar o d8 (Inspiração de Orla)');
  });

  it('counts the minutes left from the rounds (6 s each), rounded up', () => {
    expect(expiresText(100, 20)).toBe('até 8 min');
    expect(expiresText(23, 20)).toBe('até 1 min');
    expect(expiresText(20, 20)).toBe('acaba agora');
    expect(expiresText(10, 20)).toBe('acaba agora');
  });

  it('writes the die card with the bard and the time left', () => {
    expect(dieCard({ ...die, expiresAtRound: 100 }, 20)).toEqual({
      title: 'Inspiração de Bardo: d8',
      sub: 'de Orla · até 8 min',
    });
  });

  it('says what a use costs', () => {
    expect(useCost({ left: 3, total: 3 })).toBe('1 uso: restam 2 de 3');
    expect(useCost({ left: 1, total: 3 })).toBe('1 uso: restam 0 de 3');
  });
});

describe('resourceRows for the bard (60 ft)', () => {
  const t = (
    id: string,
    label: string,
    over: { why?: string; dist?: number; far?: boolean } = {},
  ) =>
    create(ResourceTargetSchema, {
      target: create(TargetInReachSchema, {
        combatantId: id,
        label,
        state: CombatantState.UNHURT,
        distanceFt: over.dist,
        tooFar: over.far ?? false,
      }),
      disabledReasonPt: over.why ?? '',
    });

  it('prints the reason the server gave as it came, and never a type', () => {
    const rows = plainText(
      resourceRows(
        [
          t('t', 'Toren', { dist: 15 }),
          t('b', 'Brisa', { dist: 25, why: 'Já tem um dado' }),
          t('s', 'Sálvia', { dist: 30, why: 'Não ouve você' }),
        ],
        60,
      ),
    );
    expect(rows.map((r) => [r.label, r.sub, r.blocked])).toEqual([
      ['Toren', 'Ileso · a 4,5 m', ''],
      ['Brisa', 'Ileso · a 7,5 m', 'Já tem um dado'],
      ['Sálvia', 'Ileso · a 9,0 m', 'Não ouve você'],
    ]);
  });

  it('says "Longe demais" for a creature past the voice with no reason of its own', () => {
    const [row] = resourceRows([t('x', 'Goblin', { dist: 90, far: true })], 60);
    expect(plainText(row.blocked)).toBe('Longe demais: alcance de 18 m');
  });
});
