import type { EncounterBlocked, ResourceTarget } from '../../../gen/meurpg/play/v1/combat_pb';
import {
  type ActionOption,
  FeatureTargetWho,
  UseScope,
} from '../../../gen/meurpg/rules/v1/rules_pb';
import { joinDots, tight, upFirst } from '../format/text';
import { article } from '../format/article';
import type { ResourceUsageVm } from '../play/resource-usage.types';
import { resourceRows, type ResourceRow } from '../resources/resource-targets';
import { metersText } from '../units';
import { LIMITED_ACTION_BLOCKED_TEXT } from './combat-errors-limits';
import { isUnlimited } from './unlimited-uses';

/**
 * The words of "Usar característica" (#743): a feature whose action declares a `use` is taken with
 * `FeatureService.UseFeature`. Everything here is for DISPLAY: the limits, the targets and the choices are the server's
 * (`ActionOption`), which checks the call again in its own transaction and refuses what is no longer so.
 */

/** The feature is taken with `UseFeature`, not `TakeAction`. */
export function viaUseFeature(o: ActionOption): boolean {
  return o.viaUseFeature;
}

/** "por turno", "por rodada", "por combate"; `''` for a feature with no limit in a scope. */
export function scopeWord(scope: UseScope): string {
  switch (scope) {
    case UseScope.TURN:
      return 'por turno';
    case UseScope.ROUND:
      return 'por rodada';
    case UseScope.COMBAT:
      return 'por combate';
    default:
      return '';
  }
}

/** The use asks for creatures (the user alone, or none, is not a question). */
export function asksTargets(o: ActionOption): boolean {
  return (
    o.targetMax > 0 &&
    o.targetWho !== FeatureTargetWho.SELF &&
    o.targetWho !== FeatureTargetWho.UNSPECIFIED
  );
}

/** The use asks which of its options ("Qual tipo"). */
export function asksChoice(o: ActionOption): boolean {
  return o.choices.length > 0;
}

/** The use asks something before the call: who, or which. */
export function asksSomething(o: ActionOption): boolean {
  return asksTargets(o) || asksChoice(o);
}

/**
 * The use opens its sheet: when it asks who or which, and always for the master, whose use is made in the name of a player's
 * character and is confirmed. The player's own use with nothing to ask goes straight through.
 */
export function needsSheet(o: ActionOption, master: boolean): boolean {
  return master || asksSomething(o);
}

/** "inimigo", "aliados", "criaturas": who the use may be aimed at, in the number the limit says. */
function whoWord(who: FeatureTargetWho, plural: boolean): string {
  switch (who) {
    case FeatureTargetWho.ALLY:
      return plural ? 'aliados' : 'aliado';
    case FeatureTargetWho.ENEMY:
      return plural ? 'inimigos' : 'inimigo';
    default:
      return plural ? 'criaturas' : 'criatura';
  }
}

/**
 * How many it takes, of whom and how far: "1 inimigo a até 3 m" on the row, "de 1 a 3 aliados até 9 m" in the head of the
 * list. `''` for a use that takes no creature.
 */
export function targetLimitText(o: ActionOption, where: 'row' | 'head' = 'row'): string {
  if (!asksTargets(o)) {
    return '';
  }
  const count =
    o.targetMin === o.targetMax ? `${o.targetMax}` : `de ${o.targetMin} a ${o.targetMax}`;
  const who = whoWord(o.targetWho, o.targetMax !== 1);
  const range =
    o.targetRangeFt > 0 ? `${where === 'row' ? ' a' : ''} até ${metersText(o.targetRangeFt)}` : '';
  return tight(`${count} ${who}${range}`);
}

/** The pool a use spends from, by the action's `resource_key`, in the player's vitals; `undefined` when it has none or is not read. */
export function resourceOf(
  o: ActionOption,
  resources: readonly ResourceUsageVm[] | undefined,
): ResourceUsageVm | undefined {
  const key = o.action?.resourceKey;
  return key ? resources?.find((r) => r.key === key) : undefined;
}

/** "gasta 1 uso de Forma Selvagem (restam 2 de 2)": what a use spends of its rest resource, and what is left before it. */
function resourceText(o: ActionOption, resource: ResourceUsageVm | undefined): string {
  if (!o.action?.resourceKey) {
    return '';
  }
  const cost = Math.max(o.action.cost, 1);
  const name = resource?.namePt ? ` de ${resource.namePt}` : '';
  const total = o.resourceTotal || resource?.total || 0;
  const left = total > 0 && !isUnlimited(total) ? ` (restam ${o.usesLeft} de ${total})` : '';
  return `gasta ${cost} ${cost === 1 ? 'uso' : 'usos'}${name}${left}`;
}

/**
 * The line under the name of a feature taken with `UseFeature`: the limit in its scope with what is left ("1 por combate
 * · restam 1 de 1"), whom it takes ("1 inimigo a até 3 m"), what it spends of its rest resource and the temporary hit
 * points it gives. The server's own reason for a row that is off is a line of its own, not part of this one.
 */
export function useLine(o: ActionOption, resource?: ResourceUsageVm): string {
  const parts: string[] = [];
  if (o.useScope !== UseScope.UNSPECIFIED && o.useTotal > 0) {
    parts.push(`${o.useTotal} ${scopeWord(o.useScope)}`, `restam ${o.useLeft} de ${o.useTotal}`);
  }
  const targets = targetLimitText(o);
  if (targets) {
    parts.push(targets);
  }
  const spends = resourceText(o, resource);
  if (spends) {
    parts.push(spends);
  }
  if ((o.action?.tempHp ?? 0) > 0) {
    parts.push(`${o.action?.tempHp} PV temporários`);
  }
  return tight(upFirst(joinDots(parts)));
}

/** The creatures of the list: how hurt and how far, and the server's reason for one that cannot be picked. */
export function useRows(targets: readonly ResourceTarget[], o: ActionOption): ResourceRow[] {
  return resourceRows(targets, o.targetRangeFt);
}

/** What the player has chosen to send. */
export interface UseChoice {
  readonly targetIds: readonly string[];
  readonly choice: string;
}

/** The ids of the creatures that can be picked: the ones of the list with no reason against them. */
export function pickable(rows: readonly ResourceRow[]): string[] {
  return rows.filter((r) => !r.blocked).map((r) => r.id);
}

/** The choice is one the use may be called with: the number of creatures within its limits, and an option when it offers any. */
export function choiceComplete(o: ActionOption, picked: UseChoice): boolean {
  if (asksChoice(o) && !o.choices.some((c) => c.key === picked.choice)) {
    return false;
  }
  if (asksTargets(o)) {
    const n = picked.targetIds.length;
    return n >= o.targetMin && n <= o.targetMax;
  }
  return true;
}

/** "Escolha uma opção." or "Escolha de 1 a 3 criaturas.": what is still missing; `''` when the choice is complete. */
export function missingText(o: ActionOption, picked: UseChoice): string {
  if (choiceComplete(o, picked)) {
    return '';
  }
  if (asksChoice(o) && !o.choices.some((c) => c.key === picked.choice)) {
    return 'Escolha uma opção.';
  }
  return o.targetMax === 1 ? 'Escolha uma criatura.' : `Escolha ${targetLimitText(o, 'head')}.`;
}

/** "Usar", "Usar: Fogo", "Usar no Zumbi 1": the button says what will be sent. */
export function useButtonLabel(
  o: ActionOption,
  picked: UseChoice,
  labelOf: (id: string) => string,
): string {
  const text = o.choices.find((c) => c.key === picked.choice)?.textPt;
  if (text) {
    return `Usar: ${text}`;
  }
  if (picked.targetIds.length === 1) {
    const name = labelOf(picked.targetIds[0]);
    if (name) {
      return `Usar ${article(name) === 'a' ? 'na' : 'no'} ${name}`;
    }
  }
  if (picked.targetIds.length > 1) {
    return `Usar em ${picked.targetIds.length}`;
  }
  return 'Usar';
}

/** "Entidade simbiótica usada: 16 PV temporários. Resta 1 de 2 usos de Forma Selvagem.": the note after a use that worked. */
export function usedNote(
  name: string,
  result: { readonly healed: number; readonly tempHp: number },
  o: ActionOption,
  resource: ResourceUsageVm | undefined,
): string {
  const gave: string[] = [];
  if (result.healed > 0) {
    gave.push(`${result.healed} PV recuperados`);
  }
  if (result.tempHp > 0) {
    gave.push(`${result.tempHp} PV temporários`);
  }
  const head = gave.length > 0 ? `${name} usada: ${gave.join(' e ')}.` : `${name} usada.`;
  if (!o.action?.resourceKey) {
    return head;
  }
  const total = o.resourceTotal || resource?.total || 0;
  if (total <= 0 || isUnlimited(total)) {
    return head;
  }
  const left = Math.max(o.usesLeft - Math.max(o.action.cost, 1), 0);
  const of = resource?.namePt ? ` de ${resource.namePt}` : '';
  const word = left === 1 ? 'Resta' : 'Restam';
  return `${head} ${word} ${left} de ${total} ${total === 1 ? 'uso' : 'usos'}${of}.`;
}

/**
 * The refusal of `UseFeature` for a target or a choice: "O Zumbi 1 saiu do alcance. Escolha outro alvo." names the creatures
 * the server refused (`combatant_ids`; none when the count or the side is wrong), and "Essa opção não vale mais. Escolha
 * outra." is the choice. `''` for any other reason, which `combatErrorMessage` writes.
 */
export function featureRefusalText(
  blocked: EncounterBlocked,
  labelOf: (id: string) => string | undefined,
): string {
  const generic = LIMITED_ACTION_BLOCKED_TEXT.get(blocked.reason) ?? '';
  const names = blocked.combatantIds.map(labelOf).filter((n): n is string => !!n);
  if (names.length === 0 || !/alvo/.test(generic)) {
    return generic;
  }
  const the = names.map((n) => `${article(n) === 'a' ? 'a' : 'o'} ${n}`);
  const list =
    the.length === 1 ? the[0] : `${the.slice(0, -1).join(', ')} e ${the[the.length - 1]}`;
  return `${upFirst(list)} ${names.length === 1 ? 'saiu' : 'saíram'} do alcance. Escolha outro alvo.`;
}

/** The first sentence of a refusal, for the bold lead of the red band; the rest follows it. */
export function leadOf(text: string): { readonly lead: string; readonly rest: string } {
  const at = text.indexOf('. ');
  return at < 0
    ? { lead: text, rest: '' }
    : { lead: text.slice(0, at + 1), rest: text.slice(at + 2) };
}
