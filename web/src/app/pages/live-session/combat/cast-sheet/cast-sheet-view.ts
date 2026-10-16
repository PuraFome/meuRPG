import {
  AttackOutcome,
  type CombatantState,
  type Combatant,
  type DiceRoll,
  type Encounter,
  type PendingDamage,
  type PreviewSpellAreaResponse,
  type SpellCast,
  type SpellTargetResult,
  type SpellTargets,
} from '../../../../../gen/meurpg/play/v1/combat_pb';
import {
  ActionEconomy,
  type MetamagicOption,
  type SpellDetails,
  SpellDamageChoice,
  SpellRangeKind,
} from '../../../../../gen/meurpg/rules/v1/rules_pb';
import {
  type CastTargetRow,
  type SlotRow,
  type SpellKind,
  type TargetRule,
  castTargetRows,
  dartsPlaced,
  dartsStatus,
  freeText,
  saveAbility,
  spentLine,
} from '../../../../core/combat/cast-flow';
import { article } from '../../../../core/combat/combat-log';
import { diceName, sumRange } from '../../../../core/combat/combat-dice';
import { circleLabel } from '../../../../core/combat/combat-grid';
import { isHit, outcomeWord } from '../../../../core/combat/attack-flow';
import { criticalHint, criticalTypedHint, fixedParts } from '../../../../core/combat/critical';
import { groupFeminine, groupName, isCreature } from '../../../../core/combat/creature-names';
import { abilityMissing } from '../../../../core/effects/ability-choice';
import {
  type MetamagicPicks,
  metamagicMissing,
  metamagicSpentLine,
} from '../../../../core/resources/metamagic';
import type { Pool } from '../../../../core/resources/pools';
import type { LuckyRoll } from '../../../../core/resources/resources-client';
import { luckyReroll } from '../../../../core/combat/lucky-question';
import {
  type RollShow,
  showOfDice,
  showOfDie,
} from '../../../../shared/roll-overlay/roll-animator';
import type { SlotAfter } from './cast-result';
import type { SculptRow } from './sculpt-picker';

/** What the cast sheet shows and asks, as pure functions of the spell's details, the combat and the choices made. */

/** The damage types a spell lets the caster pick among; empty when it offers no choice. */
export function damageChoicesOf(d: SpellDetails | null): { key: string; name: string }[] {
  return d && d.damageChoice !== SpellDamageChoice.UNSPECIFIED && d.damage.length > 1
    ? d.damage.map((x) => ({ key: x.damageTypeKey, name: x.damageTypePt }))
    : [];
}

/** The picked damage type; the first until the caster picks another. */
export function damageTypeOf(choices: readonly { key: string }[], picked: string): string {
  return choices.find((c) => c.key === picked)?.key ?? choices[0]?.key ?? '';
}

/** How far a ranged spell reaches, `null` for any other. */
export function reachFtOf(details: SpellDetails | null): number | null {
  const r = details?.range;
  return r?.kind === SpellRangeKind.RANGED ? r.distanceFt : null;
}

/** The caster is a target only of a heal or a spell the app has no effect for (a blessing): a dart, a ray or a
 * fireball at oneself is never the choice. */
export function castRowsOf(
  targets: SpellTargets | undefined,
  kind: SpellKind,
  casterId: string,
  reachFt: number | null,
): CastTargetRow[] {
  const self = kind === 'heal' || kind === 'plain';
  const list = (targets?.targets ?? []).filter((t) => self || t.combatantId !== casterId);
  return castTargetRows(list, casterId, reachFt);
}

/** The ids of the combatants that pass `test`. */
export function idsWhere(
  combatants: readonly Combatant[],
  test: (c: Combatant) => boolean,
): Set<string> {
  return new Set(combatants.filter(test).map((c) => c.id));
}

/** "Constrição encerra a concentração em Conjurar Animais.", and "Os 2 Lobos atrozes somem." when that concentration
 * holds the caster's creatures: only when this spell needs concentration and the caster holds another one, from the
 * combat's own data. */
export function concentrationEnd(
  e: Encounter | null,
  spell: {
    readonly casterId: string;
    readonly concentration: boolean;
    readonly key: string;
    readonly name: string;
  },
): { ends: string; goes: string } | null {
  const caster = e?.combatants.find((c) => c.id === spell.casterId);
  if (
    !e ||
    !caster ||
    !spell.concentration ||
    !caster.concentrationSpell ||
    caster.concentrationSpell === spell.key
  ) {
    return null;
  }
  const held = e.combatants.filter(
    (c) =>
      isCreature(c) &&
      c.ownerCharacterId === caster.characterId &&
      !!c.summonGroupId &&
      !c.defeated,
  );
  let goes = '';
  if (held.length === 1) {
    goes = `${article(held[0].label) === 'a' ? 'A' : 'O'} ${held[0].label} some.`;
  } else if (held.length > 1) {
    goes = `${groupFeminine(held) ? 'As' : 'Os'} ${held.length} ${groupName(held)} somem.`;
  }
  return {
    ends: `${spell.name} encerra a concentração em ${caster.concentrationSpellNamePt || 'a magia'}.`,
    goes,
  };
}

/** What the target step still needs, in words; `''` when it is complete. */
export function targetsMissingText(
  rule: TargetRule,
  kind: SpellKind,
  have: {
    readonly chosen: number;
    readonly dealt: ReadonlyMap<string, number>;
    readonly darts: number;
  },
  canApp: boolean,
): string {
  if (rule.kind === 'darts') {
    return dartsPlaced(have.dealt) === have.darts ? '' : dartsStatus(have.darts, have.dealt);
  }
  if ((rule.kind === 'single' || rule.kind === 'multi') && have.chosen < rule.min) {
    return rule.kind === 'single' ? 'Escolha o alvo.' : 'Escolha pelo menos um alvo.';
  }
  if (kind === 'attack' && have.chosen > 1 && !canApp) {
    return 'Com dados físicos, escolha um alvo por vez.';
  }
  return '';
}

/** Why "Conjurar" is not ready yet, in words; empty when it is. */
export function castMissing(i: {
  readonly detailsRead: boolean;
  readonly detailsFailed: boolean;
  readonly level: number;
  readonly slotChosen: boolean;
  readonly spellKey: string;
  readonly ability: string;
  /** A placed area takes no target: the server works out who is inside from the point. */
  readonly placedArea: boolean;
  readonly targets: () => string;
  readonly metaOptions: readonly MetamagicOption[];
  readonly chosenMeta: readonly string[];
  readonly picks: MetamagicPicks;
  readonly sorceryPoints: Pool | null;
}): string {
  if (!i.detailsRead && !i.detailsFailed) {
    return 'Lendo a magia…';
  }
  if (i.detailsFailed) {
    return 'Não deu para ler a magia. Feche e tente de novo.';
  }
  if (i.level > 0 && !i.slotChosen) {
    return 'Escolha o espaço de magia.';
  }
  const ability = abilityMissing(i.spellKey, i.ability);
  if (ability) {
    return ability;
  }
  const target = i.placedArea ? '' : i.targets();
  return target || metamagicMissing(i.metaOptions, i.chosenMeta, i.picks, i.sorceryPoints);
}

/** The creatures the caster can spare with Sculpt Spells: the others in the placed area, or the ones ticked in the
 * list; allies first. */
export function sculptRowsOf(
  offered: boolean,
  area: PreviewSpellAreaResponse | null | undefined,
  placed: boolean,
  listed: readonly CastTargetRow[],
  chosen: readonly string[],
): SculptRow[] {
  if (!offered) {
    return [];
  }
  const rows: SculptRow[] = placed
    ? (area?.targets ?? [])
        .filter((t) => !t.self)
        .map((t) => ({ id: t.combatantId, label: t.label, ally: t.ally }))
    : listed
        .filter((t) => !t.blocked && chosen.includes(t.id))
        .map((t) => ({ id: t.id, label: t.label, ally: false }));
  return [...rows.filter((r) => r.ally), ...rows.filter((r) => !r.ally)];
}

/** What the cast sends: the ones marked that are still listed, no more than the limit. */
export function sculptedIdsOf(
  marked: readonly string[],
  rows: readonly SculptRow[],
  limit: number,
): string[] {
  const listed = new Set(rows.map((r) => r.id));
  return marked.filter((id) => listed.has(id)).slice(0, limit);
}

/** The marked creatures after a click: out when it was in, in while there is room under the limit. */
export function toggledSculpt(now: readonly string[], id: string, limit: number): string[] {
  if (now.includes(id)) {
    return now.filter((x) => x !== id);
  }
  return now.length < limit ? [...now, id] : [...now];
}

/** The targets still chosen when the list or the slot changed, or `null` when all of them still fit. */
export function keptTargets(
  chosen: readonly string[],
  fits: ReadonlySet<string>,
  max: number,
): string[] | null {
  const kept = chosen.filter((id) => fits.has(id)).slice(0, Math.max(max, 0));
  return kept.length !== chosen.length ? kept : null;
}

/** The darts after the slot or the targets changed, or `null` when they stand: the only reachable target takes them
 * all when none is placed yet, and darts past what the slot makes are taken back. */
export function dartsAfterChange(
  dealt: ReadonlyMap<string, number>,
  reachable: readonly string[],
  total: number,
  cast: boolean,
): Map<string, number> | null {
  let next: ReadonlyMap<string, number> = dealt;
  let changed = false;
  if (reachable.length === 1 && dartsPlaced(next) === 0 && total > 0 && !cast) {
    next = new Map([[reachable[0], total]]);
    changed = true;
  }
  if (dartsPlaced(next) > total) {
    next = new Map();
    changed = true;
  }
  return changed ? new Map(next) : null;
}

/** The Metamagic picks after the targets changed: a creature no longer a target cannot stay the second target or the
 * one with disadvantage. */
export function picksAfterTargets(p: MetamagicPicks, chosen: readonly string[]): MetamagicPicks {
  return {
    twinned: p.twinned === chosen[0] ? '' : p.twinned,
    careful: p.careful,
    heightened: chosen.includes(p.heightened) ? p.heightened : '',
  };
}

/** "A cobertura é medida do ponto de origem...": only when the area's preview counts cover. */
export function coverNoteOf(p: PreviewSpellAreaResponse | null | undefined, name: string): string {
  if (!p || p.targets.length === 0) {
    return '';
  }
  return p.coverCounts
    ? `A cobertura é medida do ponto de origem, não de você, e só conta quando o teste é de Destreza (como o ${article(name) === 'a' ? 'da' : 'do'} ${name}).`
    : '';
}

function creaturesText(n: number): string {
  return `${n} ${n === 1 ? 'criatura' : 'criaturas'}`;
}

/** "3º nível · 4 criaturas · teste de resistência de Destreza (CD 16)": what an area hit, counted from the cast's own
 * targets (a player's never holds a hidden creature, RN-10). */
export function areaResultLine(
  circle: string,
  cast: SpellCast | null,
  details: SpellDetails | null,
): string {
  const parts = [circle, creaturesText(cast?.targets.length ?? 0)];
  const ability = saveAbility(details);
  if (ability) {
    const dc = cast?.targets.find((t) => (t.save?.dc ?? 0) > 0)?.save?.dc ?? 0;
    parts.push(`teste de resistência de ${ability}${dc > 0 ? ` (CD ${dc})` : ''}`);
  }
  return parts.filter((p) => p !== '').join(' · ');
}

/** The circle line of a cast sheet that is done: "3º nível", or "Truque"; a pool spell counts the creatures too. */
export function doneSubtitle(i: {
  readonly cast: SpellCast | null;
  readonly level: number;
  readonly kind: SpellKind;
  readonly placedArea: boolean;
  readonly details: SpellDetails | null;
}): string {
  const s = i.cast?.slot;
  const circle = s ? circleLabel(s.level) : i.level === 0 ? 'Truque' : '';
  const n = i.cast?.targets.length ?? 0;
  if (i.placedArea) {
    return areaResultLine(circle, i.cast, i.details);
  }
  return i.kind === 'pool' && n > 0 ? `${circle} · ${creaturesText(n)} na área` : circle;
}

/** The title of the sheet in each stage. */
export function castTitle(i: {
  readonly name: string;
  readonly resume: boolean;
  readonly done: boolean;
  readonly hpDone: boolean;
  readonly placedArea: boolean;
  readonly typing: boolean;
}): string {
  if (i.resume) {
    return `Role o dano de ${i.name}`;
  }
  if (i.hpDone || (i.placedArea && i.done)) {
    return `${i.name} conjurad${article(i.name) === 'a' ? 'a' : 'o'}`;
  }
  if (i.done) {
    return `Você conjurou ${i.name}`;
  }
  return i.typing ? 'Digite o resultado' : `Conjurar ${i.name}`;
}

/** The slot count after the cast, for "Espaços de 1º nível: 1 livre de 4". */
export function slotAfter(cast: SpellCast | null, rows: readonly SlotRow[]): SlotAfter | null {
  const s = cast?.slot;
  const row = s ? rows.find((r) => r.level === s.level && r.pact === s.pact) : undefined;
  if (!s || !row) {
    return null;
  }
  const free = Math.max(0, row.free - 1);
  return {
    level: s.level,
    total: row.total,
    used: row.total === null ? 0 : row.total - free,
    text: `Espaços de ${circleLabel(s.level)}: ${freeText(free, row.total)}`,
  };
}

/** "Escudo Arcano indisponível: sem espaço de 1º nível." when the cast took Escudo's last slot. */
export function shieldLostText(
  cast: SpellCast | null,
  shieldFree: number | null,
  shieldName: string,
): string {
  const s = cast?.slot;
  return s && shieldFree === 1
    ? `${shieldName} indisponível: sem espaço de ${circleLabel(s.level)}.`
    : '';
}

/** What the cast leaves to say: the concentration, the metamagic spent and the action. */
export function castNotes(
  c: SpellCast | null,
  spell: {
    readonly name: string;
    readonly economy: ActionEconomy;
    readonly metaOptions: readonly MetamagicOption[];
    readonly sorceryPoints: Pool | null;
  },
): string[] {
  if (!c) {
    return [];
  }
  const out: string[] = [];
  if (c.concentrating) {
    out.push(`Você está concentrado em ${spell.name}.`);
  }
  if (c.concentrationEndedSpellKey) {
    out.push('A sua concentração anterior acabou.');
  }
  if (c.metamagicKeys.length > 0) {
    const names = c.metamagicKeys.map(
      (k) => spell.metaOptions.find((o) => o.key === k)?.namePt ?? 'Metamagia',
    );
    out.push(`${metamagicSpentLine(names, c.sorceryPointsSpent, spell.sorceryPoints)}.`);
  }
  out.push(spentLine(spell.economy));
  return out;
}

/** The spell is plain: nothing to roll, hit or save. */
export function isPlainCast(c: SpellCast | null, kind: SpellKind): boolean {
  return (
    !!c &&
    kind === 'plain' &&
    c.targets.every((t) => !t.pendingDamageId && t.outcome === AttackOutcome.UNSPECIFIED && !t.save)
  );
}

/** The roll of the first damage group: its dice, the sum a typed roll may be. */
export function nextRollOf(
  groups: readonly (readonly PendingDamage[])[],
  labels: ReadonlyMap<string, { label: string; state: CombatantState }>,
) {
  const g = groups[0];
  if (!g) {
    return null;
  }
  const p = g[0];
  const who = g.map((x) => labels.get(x.targetId)?.label ?? 'alvo');
  const name = diceName(p.diceCount, p.diceSides);
  const range = sumRange(p.diceCount, p.diceSides);
  return {
    pending: p,
    name,
    range,
    label: p.diceCount > 1 ? `Role ${name} para o dano: some os dois` : `Role ${name} para o dano`,
    what: p.healing ? `Cura em ${who.join(', ')}` : `Dano em ${who.join(', ')}`,
    count: groups.length,
    // A critical spell attack follows the table's rule: what to roll, said the way it asks (RN-24).
    crit: p.critical
      ? (criticalHint(p.criticalRule, p.diceCount, p.diceSides, p.criticalMax)?.line ??
        'Acerto crítico: os dados dobram.')
      : '',
    typedHint: criticalTypedHint(p.criticalRule, name, range.min, range.max, p.criticalMax),
    // The modifier and the critical's fixed maximum, the server's numbers, added in the total before it is sent.
    modifier: p.bonus + p.criticalMax,
    fixedText: fixedParts(p.criticalMax, p.bonus),
  };
}

/** A cast sheet that opens on a damage the player never rolled has no cast to read: its rows are made from the damages
 * themselves. */
export function resumedCastOf(pendings: Iterable<PendingDamage>): SpellCast {
  const targets = [...pendings].map(
    (p) =>
      ({
        combatantId: p.targetId,
        darts: p.attackKey === 'spell:magic-missile' ? p.diceCount : 0,
        outcome: AttackOutcome.UNSPECIFIED,
        pendingDamageId: p.id,
      }) as unknown as SpellTargetResult,
  );
  return { targets } as unknown as SpellCast;
}

/**
 * The dice the cast just rolled in the app, as the result step shows them: the pool of a spell that reads hit points,
 * or the spell attack d20 (one die per target in the same overlay; with a single target, the total and the word too).
 * The targets' saving throws are not animated: they are not the caster's roll.
 */
export function castShow(name: string, cast: SpellCast): RollShow | null {
  const pool = showOfDice(name, cast.poolRoll, { withTotal: true });
  if (pool) {
    return pool;
  }
  const attacks = cast.targets.filter((t) => t.attackRoll);
  if (attacks.length === 1) {
    return singleAttackShow(name, attacks[0]);
  }
  if (attacks.length > 1) {
    const dice = attacks.flatMap((t) => showOfDice(name, t.attackRoll)?.dice ?? []);
    return dice.length > 0 ? { label: `Ataque com ${name}`, dice } : null;
  }
  return null;
}

function singleAttackShow(name: string, t: SpellTargetResult): RollShow | null {
  const told = t.outcome !== AttackOutcome.UNSPECIFIED;
  const hit = isHit(t.outcome);
  return showOfDice(`Ataque com ${name}`, t.attackRoll, {
    withTotal: true,
    outcome: told ? { word: outcomeWord(t.outcome), good: hit } : undefined,
    critical: t.outcome === AttackOutcome.CRITICAL_HIT,
    fumble: told && !hit,
  });
}

/** The d20 the app rolled again for Lucky, tumbling, when the die's question comes next (a cast that is told animates
 * its own d20): the new face alone, "1 → 14". */
export function luckyShow(roll: DiceRoll | undefined, die: LuckyRoll | null): RollShow | null {
  const again = die && 'inApp' in die ? luckyReroll(roll) : undefined;
  return again
    ? showOfDie('Sortudo', roll?.diceSides ?? 0, again.to, `${again.from} → ${again.to}`)
    : null;
}

/** Who the cast names: nobody for a placed area (the server works it out), the darts, or the targets ticked. */
export function castTargetsOf(
  placedArea: boolean,
  rule: TargetRule,
  dartTargets: () => { combatantId: string; darts: number }[],
  chosen: readonly string[],
): { combatantId: string; darts: number }[] {
  if (placedArea || rule.kind === 'none') {
    return [];
  }
  return rule.kind === 'darts'
    ? dartTargets()
    : chosen.map((combatantId) => ({ combatantId, darts: 0 }));
}
