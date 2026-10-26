import { DeathSaveOutcome, type DeathSave } from '../../../gen/meurpg/play/v1/combat_pb';
import { rollText } from './combat-dice';
import { hookEffects } from './hook-text';

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

/** A hook that stood the character up (a table feature, MR-025): the save's word, who acted, and that no failure counts. */
function standUpText(save: DeathSave): string {
  const hook = save.hook;
  if (!hook || !(hook.healed > 0) || save.outcome === DeathSaveOutcome.REVIVED) {
    return '';
  }
  const word = save.outcome === DeathSaveOutcome.SUCCESS ? 'Sucesso' : 'Falha';
  return `${word}, mas o traço ${hook.sourceNamePt || 'uma característica'} entrou em ação: você volta com ${hook.healed} PV. Nenhuma falha entra na conta.`;
}

/** What a hook that did not stand the character up gave: " O traço X entrou em ação: ganha 3 PV temporários." */
function hookTail(hook: DeathSave['hook']): string {
  const effects = hook ? hookEffects(hook) : [];
  return hook && effects.length > 0
    ? ` O traço ${hook.sourceNamePt || 'uma característica'} entrou em ação: ${effects.join(', ')}.`
    : '';
}

/** The result a death save announces, in a live region: "Teste contra a
 * morte: 1d20 (14) = 14. Sucesso." A natural 20 brings the character back
 * ("Brisa volta com 1 PV"); three successes make it stable. */
export function saveAnnouncement(save: DeathSave, name: string): string {
  const roll = save.roll
    ? `Teste contra a morte: ${rollText(save.roll)}.`
    : 'Teste contra a morte.';
  const stoodUp = standUpText(save);
  if (stoodUp) {
    return `${roll} ${stoodUp}`;
  }
  const tail = hookTail(save.hook);
  switch (save.outcome) {
    case DeathSaveOutcome.REVIVED:
      return `${roll} ${name} volta com 1 PV.${tail}`;
    case DeathSaveOutcome.CRITICAL_FAILURE:
      return `${roll} Falha: um 1 conta duas falhas.${save.failures >= 3 ? ' Três falhas.' : ''}${tail}`;
    case DeathSaveOutcome.FAILURE:
      return `${roll} Falha.${save.failures >= 3 ? ' Três falhas.' : ''}${tail}`;
    default:
      return `${roll} Sucesso.${save.stable ? ' Estável: não rola mais.' : ''}${tail}`;
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
