import type { HookResult } from '../../../gen/meurpg/play/v1/combat_pb';

/**
 * What an on_event hook of a table feature did (MR-025), in words. The server sends the numbers only to the master and the
 * character's own player (RN-10), so these functions print whatever arrived and invent nothing: a number that is 0 is not said.
 */

/** "reserva_de_teste" → "reserva de teste": the resource the hook names, as its key reads. */
function resourceWords(key: string): string {
  return key.replace(/_/g, ' ');
}

function usesWord(n: number): string {
  return n === 1 ? 'uso' : 'usos';
}

/** The effects of a hook: "recupera 5 PV", "ganha 3 PV temporários", "devolve 1 uso de …", "gasta 2 usos de …". With `average`
 * the heal is marked "(em média)": a rest preview shows the average of the hook's dice, and the rest itself rolls them. */
export function hookEffects(h: HookResult, average = false): string[] {
  const out: string[] = [];
  if (h.healed > 0) {
    out.push(`recupera ${h.healed} PV${average ? ' (em média)' : ''}`);
  }
  if (h.tempHpGained > 0) {
    out.push(`ganha ${h.tempHpGained} PV temporários`);
  }
  if (h.restored > 0) {
    out.push(
      `devolve ${h.restored} ${usesWord(h.restored)} de ${resourceWords(h.restoredResource)}`,
    );
  }
  if (h.spent > 0) {
    out.push(`gasta ${h.spent} ${usesWord(h.spent)} de ${resourceWords(h.spentResource)}`);
  }
  return out;
}

/** The feature that has the hook, as the server names it. */
function sourceOf(h: HookResult): string {
  return h.sourceNamePt || 'Uma característica';
}

/** "Fôlego da Mesa entrou em ação: ganha 3 PV temporários". */
export function hookSentence(h: HookResult, average = false): string {
  const effects = hookEffects(h, average);
  return `${sourceOf(h)} entrou em ação${effects.length > 0 ? `: ${effects.join(', ')}` : ''}`;
}

/** "Fôlego da Mesa levantou Toren com 6 PV": the hook of a failed death save that healed. */
export function hookStandUpSentence(h: HookResult, name: string): string {
  return h.healed > 0 ? `${sourceOf(h)} levantou ${name} com ${h.healed} PV` : hookSentence(h);
}
