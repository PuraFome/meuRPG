import { LevelUpMulticlassException } from '../../../gen/meurpg/characters/v1/characters_pb';
import type {
  Attack,
  DerivedSheet,
  DerivedSkill,
  SavingThrow,
} from '../../../gen/meurpg/rules/v1/rules_pb';
import { Ability as GenAbility } from '../../../gen/meurpg/rules/v1/rules_pb';
import { abilityLabel, formatModifier } from '../characters/character-labels';
import type { AbilityKey } from '../characters/characters.types';
import { joinDots } from '../format/text';
import { hitDiceSum } from '../resources/hit-dice-text';
import {
  type ChangeRow,
  LIST,
  type MulticlassContext,
  type SummaryContext,
  article,
  row,
} from './levelup-summary-rows';
import { spellRows } from './levelup-summary-spells';

export type { ChangeRow, MulticlassContext, SummaryContext };

const EXCEPTION_TEXT: Record<number, string> = {
  [LevelUpMulticlassException.EXTRA_ATTACK]:
    'Ataque Extra não se soma ao que o personagem já tinha: o número de ataques é o maior.',
  [LevelUpMulticlassException.CHANNEL_DIVINITY]:
    'Canalizar Divindade: os efeitos das duas classes ficam à escolha, mas os usos não se somam.',
  [LevelUpMulticlassException.UNARMORED_DEFENSE]:
    'Defesa sem Armadura não se soma à que o personagem já tinha: vale a primeira.',
};

const ABILITY_KEY: Record<number, AbilityKey> = {
  [GenAbility.STRENGTH]: 'str',
  [GenAbility.DEXTERITY]: 'dex',
  [GenAbility.CONSTITUTION]: 'con',
  [GenAbility.INTELLIGENCE]: 'int',
  [GenAbility.WISDOM]: 'wis',
  [GenAbility.CHARISMA]: 'cha',
};

/** "3d6", or "3d6 + 1d8" for a multiclass (kept apart by size). */
function hitDice(sheet: DerivedSheet): string {
  return hitDiceSum(sheet.hitDice);
}

/** The rows only a class new to the sheet has: what it gives as proficiencies and features, and the equipment it does not. */
function multiclassRows(mc: MulticlassContext): ChangeRow[] {
  const gained = mc.summary?.proficienciesGained.map((p) => p.namePt) ?? [];
  const out: ChangeRow[] = [
    {
      key: 'proficiencies',
      label: 'Proficiências novas',
      before: '—',
      after: gained.length === 0 ? 'nenhuma' : LIST.format(gained),
      sub:
        gained.length === 0
          ? `A tabela de multiclasse d${article(mc.newClassName)} ${mc.newClassName} não dá nenhuma.`
          : 'Só as da tabela de multiclasse.',
    },
  ];
  const exceptions = (mc.summary?.exceptions ?? []).map((e) => EXCEPTION_TEXT[e]).filter(Boolean);
  if (mc.newFeatures.length > 0 || exceptions.length > 0) {
    out.push({
      key: 'new-class-features',
      label: 'Características novas',
      before: '—',
      after: mc.newFeatures.length > 0 ? joinDots([...mc.newFeatures]) : 'nenhuma',
      sub: exceptions.join(' '),
    });
  }
  out.push({
    key: 'equipment',
    label: 'Equipamento novo',
    before: '—',
    after: 'nenhum',
    sub: 'Só a primeira classe dá equipamento.',
  });
  return out;
}

function savesChanged(before: DerivedSheet, after: DerivedSheet): ChangeRow[] {
  const was = new Map<number, SavingThrow>(before.savingThrows.map((s) => [s.ability, s]));
  return after.savingThrows.flatMap((s) => {
    const b = was.get(s.ability);
    const key = ABILITY_KEY[s.ability];
    const r =
      b && key
        ? row(
            `save-${key}`,
            `Teste de resistência de ${abilityLabel(key)}`,
            formatModifier(b.bonus),
            formatModifier(s.bonus),
          )
        : null;
    return r ? [r] : [];
  });
}

/** Skills that move by the same amount are one line: "Arcanismo, História, Investigação +6 → +7". */
function skillsChanged(before: DerivedSheet, after: DerivedSheet): ChangeRow[] {
  const was = new Map<string, DerivedSkill>(before.skills.map((s) => [s.key, s]));
  const groups = new Map<string, { names: string[]; before: string; after: string }>();
  for (const s of after.skills) {
    const b = was.get(s.key);
    if (!b || b.bonus === s.bonus) continue;
    const id = `${b.bonus}>${s.bonus}`;
    const g = groups.get(id) ?? {
      names: [],
      before: formatModifier(b.bonus),
      after: formatModifier(s.bonus),
    };
    g.names.push(s.namePt);
    groups.set(id, g);
  }
  return [...groups].map(([id, g]) => ({
    key: `skills-${id}`,
    label: LIST.format(g.names),
    before: g.before,
    after: g.after,
    sub: '',
  }));
}

/**
 * The attacks whose bonus or damage moved (a Strength increase raises the weapon's to-hit and damage; a cantrip
 * rolls more dice), one line each: "+5 · 1d8+3" → "+7 · 1d8+5", with the damage type under it. A spell that asks
 * for a saving throw has no to-hit, so it shows the damage alone.
 */
function attacksChanged(before: DerivedSheet, after: DerivedSheet): ChangeRow[] {
  const was = new Map<string, Attack>(before.attacks.map((a) => [a.key, a]));
  const text = (a: Attack) =>
    [a.saveDc > 0 ? '' : formatModifier(a.attackBonus), a.damage]
      .filter((p) => p !== '')
      .join(' · ');
  return after.attacks.flatMap((a) => {
    const b = was.get(a.key);
    const r = b ? row(`attack-${a.key}`, a.namePt, text(b), text(a), a.damageTypePt) : null;
    return r ? [r] : [];
  });
}

/** The total level (when a class is new) and the classes, "Mago 3" → "Mago 4". "Nível" always stays. */
function levelRows(
  before: DerivedSheet,
  after: DerivedSheet,
  mc: MulticlassContext | undefined,
): ChangeRow[] {
  const level = (s: DerivedSheet) =>
    s.classes.map((c) => `${c.namePt} ${c.level}`).join(mc ? ' · ' : ' / ');
  const classes: ChangeRow = {
    key: 'level',
    label: mc ? 'Classes' : 'Nível',
    before: level(before),
    after: level(after),
    sub: '',
  };
  if (!mc) {
    return [classes];
  }
  const total: ChangeRow = {
    key: 'total-level',
    label: 'Nível total',
    before: String(before.totalLevel),
    after: String(after.totalLevel),
    sub: `O XP para este nível é o do nível total${before.nextLevelXp > 0 ? `: ${before.nextLevelXp.toLocaleString('pt-BR')}` : ''}.`,
  };
  return [total, classes];
}

/** The ability scores that moved, with the modifier under them. */
function abilityRows(before: DerivedSheet, after: DerivedSheet): (ChangeRow | null)[] {
  const was = new Map(before.abilities.map((a) => [a.ability, a]));
  return after.abilities.flatMap((a) => {
    const b = was.get(a.ability);
    const key = ABILITY_KEY[a.ability];
    if (!b || !key || b.score === a.score) {
      return [];
    }
    return [
      row(
        `ability-${key}`,
        abilityLabel(key),
        String(b.score),
        String(a.score),
        `Modificador ${formatModifier(b.modifier)} → ${formatModifier(a.modifier)}`,
      ),
    ];
  });
}

/** The proficiency bonus follows the total level: on a new class the row stays even when the number does not move. */
function proficiencyRow(
  before: DerivedSheet,
  after: DerivedSheet,
  mc: MulticlassContext | undefined,
): ChangeRow | null {
  const was = formatModifier(before.proficiencyBonus);
  const now = formatModifier(after.proficiencyBonus);
  if (!mc) {
    return row('proficiency', 'Bônus de proficiência', was, now);
  }
  return {
    key: 'proficiency',
    label: 'Bônus de proficiência',
    before: was,
    after: now,
    sub:
      before.proficiencyBonus === after.proficiencyBonus
        ? `Pelo nível total (${after.totalLevel}): não muda.`
        : `Pelo nível total (${after.totalLevel}).`,
  };
}

/** The features the level gives by themselves, counted, with their names under the count. */
function featuresRow(ctx: SummaryContext): ChangeRow[] {
  if (!ctx.newFeatures || ctx.newFeatures.length === 0) {
    return [];
  }
  return [
    {
      key: 'features',
      label: 'Novas características',
      before: '',
      after: String(ctx.newFeatures.length),
      sub: joinDots([...ctx.newFeatures]),
    },
  ];
}

/** The passive scores. */
function passiveRows(before: DerivedSheet, after: DerivedSheet): (ChangeRow | null)[] {
  return [
    row(
      'passive-perception',
      'Percepção passiva',
      String(before.passivePerception),
      String(after.passivePerception),
    ),
    row(
      'passive-investigation',
      'Investigação passiva',
      String(before.passiveInvestigation),
      String(after.passiveInvestigation),
    ),
    row(
      'passive-insight',
      'Intuição passiva',
      String(before.passiveInsight),
      String(after.passiveInsight),
    ),
  ];
}

/**
 * Everything the level changes, before → after, as the server derived both sheets
 * (`Character.derived` and `PreviewLevelUp.after`): the browser only lines the
 * numbers up and words them. A line whose two sides are the same is left out;
 * "Nível" always stays.
 */
export function changeRows(
  before: DerivedSheet,
  after: DerivedSheet,
  ctx: SummaryContext,
): ChangeRow[] {
  const mc = ctx.multiclass;
  const rows: (ChangeRow | null)[] = [
    ...levelRows(before, after, mc),
    ...abilityRows(before, after),
    row('hp', 'Pontos de vida', String(before.hitPointsMax), String(after.hitPointsMax), ctx.hpSub),
    row(
      'hit-dice',
      'Dados de vida',
      hitDice(before),
      hitDice(after),
      mc && after.hitDice.length > 1 ? 'Ficam separados por tipo.' : '',
    ),
    proficiencyRow(before, after, mc),
    row('armor', 'Classe de Armadura', String(before.armorClass), String(after.armorClass)),
    row(
      'initiative',
      'Iniciativa',
      formatModifier(before.initiative),
      formatModifier(after.initiative),
    ),
    ...spellRows(before, after, ctx),
    ...featuresRow(ctx),
    ...(mc ? multiclassRows(mc) : []),
    ...savesChanged(before, after),
    ...attacksChanged(before, after),
    ...skillsChanged(before, after),
    ...passiveRows(before, after),
  ];
  return rows.filter((r): r is ChangeRow => r !== null);
}
