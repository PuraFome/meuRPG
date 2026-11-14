import type { Combatant } from '../../../gen/meurpg/play/v1/combat_pb';
import type {
  LastingEffect,
  WeaponHitDamage,
} from '../../../gen/meurpg/play/v1/lasting_effects_pb';
import { conditionPhrases, damageChip, hitUntil } from '../content/hit-effect-read';
import { emptyHit, type HitDraft, type HitTriggerKey } from '../content/hit-effect-draft';
import { hoursText, upFirst } from '../format/text';
import { abilityName } from './ability-choice';
import { SECONDS_PER_ROUND } from './effects-text';

/**
 * The pending damage a spell adds to the caster's weapon hits (#577), as the caster's player reads it: the card at the top of
 * "Ataques" and the line of "Seus efeitos". The server sends the numbers worked out for the slot (`LastingEffect.modifiers[].hit`,
 * only on the caster's own card); the damage words are the content editor's (`damageChip`), so the editor's preview and the card
 * are one text. Pure Portuguese, tested without a DOM. Another player never gets a `hit`, so nothing here is for them.
 */

/** One card of the caster's pending hit effects. */
export interface HitCardView {
  readonly key: string;
  /** "Sinal da Presa no Wight": the spell, and the marked creature when it is a marked target and the app can tell who. */
  readonly title: string;
  /** The chip "Concentração" is drawn only for a concentration effect. */
  readonly concentration: boolean;
  /** "Cada golpe com arma no Wight: +1d6 do tipo da arma. 58 min restantes." */
  readonly line: string;
}

const TRIGGER_WORDS: Readonly<Record<string, string>> = {
  next_hit: 'no próximo golpe com arma',
  next_melee_hit: 'no próximo golpe corpo a corpo',
  each_hit: 'cada golpe com arma',
  marked_target: 'cada golpe com arma',
};

const SECONDS_PER_MINUTE = 60;

/** What the marked creature is called when the app cannot tell which one it is. */
const MARKED_FALLBACK = 'alvo marcado';

/** The weapon-hit modifier of an effect, as the caster's own card carries it; `undefined` for any other effect. */
export function hitOf(effect: LastingEffect): WeaponHitDamage | undefined {
  return effect.modifiers.find((m) => m.hit)?.hit;
}

/**
 * "58 min restantes", "1 h 52 min restantes", and under a minute "Restam 4 rodadas": the words of the server's Wild Shape
 * clock (`formTimeText`), so every effect timer reads alike.
 */
export function timeLeftText(seconds: number): string {
  if (seconds >= SECONDS_PER_MINUTE) {
    return `${hoursText(seconds)} restantes`;
  }
  const rounds = Math.floor(seconds / SECONDS_PER_ROUND);
  return rounds === 1 ? 'Resta 1 rodada' : `Restam ${rounds} rodadas`;
}

/** The hit as the content editor's draft holds it, so its damage and condition words are reused instead of written twice. */
function draftOf(hit: WeaponHitDamage): HitDraft {
  return {
    ...emptyHit(),
    enabled: true,
    trigger: hit.trigger as HitTriggerKey,
    dice: String(hit.dice),
    die: hit.die,
    perSlot: String(hit.perSlotDice),
    cap: String(hit.maxDice),
    damageType: hit.damageTypeKey,
    saveAbility: hit.save,
    condition: hit.conditionKeys[0] ?? '',
    extraConditions: hit.conditionKeys.slice(1),
    conditionLasts: hit.conditionDuration === 'spell_end' ? 'spell_end' : 'next_turn',
  };
}

/** "teste de resistência de Constituição ou fica Cego até o fim do próximo turno dele"; empty with no test. */
function saveText(hit: WeaponHitDamage, d: HitDraft): string {
  if (hit.save === '') {
    return '';
  }
  const test = `teste de resistência de ${abilityName(hit.save, '')}`;
  const known = conditionPhrases(hit.conditionKeys);
  const names = known.length > 0 ? known : [...hit.conditionNamesPt];
  if (names.length === 0) {
    return test;
  }
  const list =
    names.length === 1 ? names[0] : `${names.slice(0, -1).join(', ')} e ${names[names.length - 1]}`;
  return `${test} ou fica ${list} ${hitUntil(d.conditionLasts)}`;
}

/** The time the effect has left, or empty when its clock says no end (a concentration with no count). */
function timeText(effect: LastingEffect): string {
  return effect.roundsLeft === undefined ? '' : timeLeftText(effect.roundsLeft * SECONDS_PER_ROUND);
}

/**
 * The creature a marked-target effect is on, when the app can tell: the one other combatant that carries a label of the spell's
 * name (the mark the target shows, which the caster's player reads as a label and never as a card). Two creatures with that
 * label, or none, give `''`: the line then says "no alvo marcado" instead of naming the wrong one.
 */
export function markedNameOf(
  effect: LastingEffect,
  own: Combatant,
  combatants: readonly Combatant[],
): string {
  const marked = combatants.filter(
    (c) => c.id !== own.id && c.effectLabels.some((l) => l.textPt === effect.sourceNamePt),
  );
  return marked.length === 1 ? marked[0].label : '';
}

/** The one sentence of the effect: "Cada golpe com arma no Wight: +1d6 do tipo da arma. 58 min restantes." */
export function hitCardLine(effect: LastingEffect, markName: string): string {
  const hit = hitOf(effect);
  if (!hit) {
    return '';
  }
  const d = draftOf(hit);
  const nameOf = (key: string): string => (key === hit.damageTypeKey ? hit.damageTypePt : '');
  const trigger =
    hit.trigger === 'marked_target'
      ? `${TRIGGER_WORDS[hit.trigger]} no ${markName || MARKED_FALLBACK}`
      : (TRIGGER_WORDS[hit.trigger] ?? '');
  const save = saveText(hit, d);
  const time = timeText(effect);
  const damage = `${damageChip(d, nameOf)}${save ? `; ${save}` : ''}.`;
  return `${upFirst(trigger)}: ${damage}${time ? ` ${time}.` : ''}`;
}

/** The caster's pending hit effects: their own rows with a hit that no hit has spent yet, in the server's order. */
export function hitCards(own: Combatant, combatants: readonly Combatant[]): HitCardView[] {
  return own.effects
    .filter((e) => e.casterId === own.id && !e.spent && hitOf(e))
    .map((e) => {
      const marked = hitOf(e)?.trigger === 'marked_target' ? markedNameOf(e, own, combatants) : '';
      return {
        key: e.id,
        title: marked ? `${e.sourceNamePt} no ${marked}` : e.sourceNamePt,
        concentration: e.concentration,
        line: hitCardLine(e, marked),
      };
    });
}
