import { DeathSaveOutcome } from '../../../gen/meurpg/play/v1/combat_pb';
import { conditionName, conditionTags, CONDITIONS, listNames, sameKeys } from './conditions';
import { countsSentence, countText, marks, outcomeText, saveAnnouncement } from './death-saves';

const roll = (face: number) => ({ diceCount: 1, diceSides: 20, faces: [face], modifier: 0, total: face, physical: false }) as never;

describe('the death saves (E6-13, RN-03)', () => {
  it('draws three marks of one kind', () => {
    expect(marks(1)).toEqual([true, false, false]);
    expect(countText(0)).toBe('0 de 3');
    expect(countsSentence(1, 1)).toBe('1 sucesso, 1 falha');
    expect(countsSentence(2, 0)).toBe('2 sucessos, 0 falhas');
  });

  it('announces each result', () => {
    expect(saveAnnouncement({ roll: roll(14), outcome: DeathSaveOutcome.SUCCESS, successes: 1, failures: 1 } as never, 'Brisa')).toBe(
      'Teste contra a morte: 1d20 (14) = 14. Sucesso.',
    );
    expect(saveAnnouncement({ roll: roll(20), outcome: DeathSaveOutcome.REVIVED } as never, 'Brisa')).toBe(
      'Teste contra a morte: 1d20 (20) = 20. Brisa volta com 1 PV.',
    );
    expect(saveAnnouncement({ roll: roll(1), outcome: DeathSaveOutcome.CRITICAL_FAILURE, failures: 3 } as never, 'Brisa')).toBe(
      'Teste contra a morte: 1d20 (1) = 1. Falha: um 1 conta duas falhas. Três falhas.',
    );
    expect(saveAnnouncement({ roll: roll(12), outcome: DeathSaveOutcome.SUCCESS, successes: 3, stable: true } as never, 'Brisa')).toContain(
      'Estável: não rola mais.',
    );
    expect(outcomeText(DeathSaveOutcome.FAILURE)).toBe('Falha');
  });
});

describe('the conditions (E6-29)', () => {
  it('has the 15 of the SRD in alphabetical order, prone as "Derrubado" (question 43)', () => {
    expect(CONDITIONS).toHaveLength(15);
    expect(CONDITIONS.map((c) => c.name)).toEqual([...CONDITIONS.map((c) => c.name)].sort((a, b) => a.localeCompare(b, 'pt')));
    expect(conditionName('condition:prone')).toBe('Derrubado');
  });

  it('takes the tags from the names the server sends, then from the keys', () => {
    expect(conditionTags({ conditions: ['condition:prone'], conditionNamesPt: ['Derrubado'] })).toEqual(['Derrubado']);
    expect(conditionTags({ conditions: ['condition:poisoned'], conditionNamesPt: [] })).toEqual(['Envenenado']);
  });

  it('compares sets without order, and reads names aloud', () => {
    expect(sameKeys(['a', 'b'], ['b', 'a'])).toBe(true);
    expect(sameKeys(['a'], ['a', 'b'])).toBe(false);
    expect(listNames(['Cego', 'Surdo', 'Impedido'])).toBe('Cego, Surdo e Impedido');
    expect(listNames(['Cego'])).toBe('Cego');
  });
});
