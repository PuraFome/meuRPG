import { DeathSaveOutcome, type DeathSave } from '../../../gen/meurpg/play/v1/combat_pb';
import { rollText } from './combat-dice';

/**
 * The death saves as the screens write them (E6-13, E6-30, RN-03): the
 * marks, the counts and the sentence a roll announces. Pure functions.
 */

/** One mark of a row of three: filled (a try that happened) or empty. */
export function marks(count: number): boolean[] {
  return [0, 1, 2].map((i) => i < count);
}

/** "0 de 3", the words under "Sucessos" and "Falhas". */
export function countText(count: number): string {
  return `${count} de 3`;
}

/** "1 sucesso, 1 falha", for the log and the summary. */
export function countsSentence(successes: number, failures: number): string {
  const s = `${successes} ${successes === 1 ? 'sucesso' : 'sucessos'}`;
  const f = `${failures} ${failures === 1 ? 'falha' : 'falhas'}`;
  return `${s}, ${f}`;
}

/** The result a death save announces, in a live region: "Teste contra a
 * morte: 1d20 (14) = 14. Sucesso." A natural 20 brings the character back
 * ("Brisa volta com 1 PV"); three successes make it stable. */
export function saveAnnouncement(save: DeathSave, name: string): string {
  const roll = save.roll ? `Teste contra a morte: ${rollText(save.roll)}.` : 'Teste contra a morte.';
  switch (save.outcome) {
    case DeathSaveOutcome.REVIVED:
      return `${roll} ${name} volta com 1 PV.`;
    case DeathSaveOutcome.CRITICAL_FAILURE:
      return `${roll} Falha: um 1 conta duas falhas.${save.failures >= 3 ? ' Três falhas.' : ''}`;
    case DeathSaveOutcome.FAILURE:
      return `${roll} Falha.${save.failures >= 3 ? ' Três falhas.' : ''}`;
    default:
      return `${roll} Sucesso.${save.stable ? ' Estável: não rola mais.' : ''}`;
  }
}

/** The word of an outcome: "Sucesso", "Falha", "Falha crítica", "Volta com 1 PV". */
export function outcomeText(outcome: DeathSaveOutcome): string {
  switch (outcome) {
    case DeathSaveOutcome.REVIVED:
      return 'Volta com 1 PV';
    case DeathSaveOutcome.CRITICAL_FAILURE:
      return 'Falha crítica';
    case DeathSaveOutcome.FAILURE:
      return 'Falha';
    default:
      return 'Sucesso';
  }
}
