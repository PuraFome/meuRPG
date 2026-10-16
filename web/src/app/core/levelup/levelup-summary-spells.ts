import type { DerivedSheet, Spellcasting } from '../../../gen/meurpg/rules/v1/rules_pb';
import { formatModifier, pactSlotsText, spellLevelLabel } from '../characters/character-labels';
import {
  type ChangeRow,
  type SummaryContext,
  article,
  cast,
  newNames,
  row,
} from './levelup-summary-rows';

/** Before a class casts there is nothing to compare: a dash, never a 0 or a +0 that looks like a number. */
const NONE = '—';

/** Why the slots changed on a sheet with several classes: the caster level of the multiclass table, or the one class's own table. */
function slotsReason(ctx: SummaryContext, after: DerivedSheet): string {
  const s = ctx.multiclass?.summary;
  if (!s) {
    return '';
  }
  if (s.slotsByTable) {
    return `Nível de conjurador ${s.casterLevelAfter}, pela tabela de multiclasse.`;
  }
  const casters = after.spellcasting;
  if (casters.length === 1) {
    const level = after.classes.find((c) => c.classKey === casters[0].classKey)?.level ?? 0;
    return level === s.casterLevelAfter
      ? `Nível de conjurador ${s.casterLevelAfter}: só ${article(casters[0].classNamePt)} ${casters[0].classNamePt} conta.`
      : `Espaços da tabela d${article(casters[0].classNamePt)} ${casters[0].classNamePt}.`;
  }
  return '';
}

/** A class with no cantrips has nothing to gain: no row for "— → 0". */
function cantripsRow(
  bc: Spellcasting | undefined,
  ac: Spellcasting,
  ctx: SummaryContext,
): ChangeRow | null {
  if (!bc && ac.cantripsKnown <= 0) {
    return null;
  }
  return row(
    'cantrips',
    'Truques',
    bc ? String(bc.cantripsKnown) : NONE,
    String(ac.cantripsKnown),
    newNames(ctx.cantrips, 'Novo', 'Novos'),
  );
}

/** The spells known (or in the book) before and after, with how many are still missing from the picks. */
function knownSpellsRow(
  before: DerivedSheet,
  after: DerivedSheet,
  bc: Spellcasting | undefined,
  ac: Spellcasting,
  ctx: SummaryContext,
): ChangeRow | null {
  const list = ctx.spellListClassKey || ctx.classKey;
  // A class that learns a fixed number of spells has it from the server, whichever list they came from (a Bard's Magical
  // Secrets); the others (a spellbook, a table class) are counted among the spells of the sheet that are on the class list.
  const onList = (s: DerivedSheet) =>
    s.spells.filter((x) => (x.spell?.level ?? 0) > 0 && x.spell?.classKeys.includes(list)).length;
  const knownBefore = bc && bc.spellsKnown > 0 ? bc.spellsKnown : onList(before);
  // The picks of the draft count at once: the preview of the sheet that comes after them can still be the one before
  // the last pick, and a count read from it alone would flip between runs. The picks are always new spells.
  const knownAfter =
    ac.spellsKnown > 0
      ? ac.spellsKnown
      : Math.max(onList(after), onList(before) + ctx.spells.length);
  if (ctx.learnsSpells === false || (knownAfter === knownBefore && ctx.spells.length === 0)) {
    return null;
  }
  const missing = ctx.spellsMissing > 0 ? ` (falta ${ctx.spellsMissing})` : '';
  return {
    key: 'spells',
    label: ctx.spellbook ? 'Livro de magias' : 'Magias conhecidas',
    before: bc ? String(knownBefore) : NONE,
    after: `${knownAfter}${missing}`,
    sub: newNames(ctx.spells, 'Nova', 'Novas'),
  };
}

/** A row of what the class table gives is tagged, so the screen says "Da mesa". */
function tagged(r: ChangeRow | null, ctx: SummaryContext): ChangeRow | null {
  return r && ctx.table ? { ...r, table: true } : r;
}

/** One row for each circle of slots that either sheet has. */
function slotRows(
  before: DerivedSheet,
  after: DerivedSheet,
  ctx: SummaryContext,
): (ChangeRow | null)[] {
  const slots = (s: DerivedSheet, level: number) =>
    s.spellSlots.find((x) => x.level === level)?.count ?? 0;
  const circles = new Set([...before.spellSlots, ...after.spellSlots].map((x) => x.level));
  return [...circles]
    .sort((a, b) => a - b)
    .map((level) =>
      tagged(
        row(
          `slots-${level}`,
          `Espaços de ${spellLevelLabel(level)}`,
          String(slots(before, level)),
          String(slots(after, level)),
          ctx.table ? 'Da tabela da classe' : slotsReason(ctx, after),
        ),
        ctx,
      ),
    );
}

function pactRow(before: DerivedSheet, after: DerivedSheet, ctx: SummaryContext): ChangeRow | null {
  if (!after.pactMagic) {
    return null;
  }
  return tagged(
    row(
      'pact',
      'Espaços do pacto',
      pactSlotsText(before.pactMagic),
      pactSlotsText(after.pactMagic),
      ctx.table ? 'Da tabela da classe' : '',
    ),
    ctx,
  );
}

function preparedRow(
  bc: Spellcasting | undefined,
  ac: Spellcasting,
  ctx: SummaryContext,
): ChangeRow | null {
  if (ac.preparedMax <= 0) {
    return null;
  }
  return row(
    'prepared',
    'Magias preparadas',
    bc ? String(bc.preparedMax) : NONE,
    String(ac.preparedMax),
    newNames(ctx.prepared, 'Nova', 'Novas'),
  );
}

/** The rows about the spells of the class gaining the level, in the order the screen lists them; none for a class that does not cast. */
export function spellRows(
  before: DerivedSheet,
  after: DerivedSheet,
  ctx: SummaryContext,
): (ChangeRow | null)[] {
  const bc = cast(before, ctx.classKey);
  const ac = cast(after, ctx.classKey);
  if (!ac) {
    return [];
  }
  return [
    row('dc', 'CD das magias', bc ? String(bc.saveDc) : NONE, String(ac.saveDc)),
    row(
      'attack',
      'Ataque com magia',
      bc ? formatModifier(bc.attackBonus) : NONE,
      formatModifier(ac.attackBonus),
    ),
    cantripsRow(bc, ac, ctx),
    knownSpellsRow(before, after, bc, ac, ctx),
    ...slotRows(before, after, ctx),
    pactRow(before, after, ctx),
    preparedRow(bc, ac, ctx),
  ];
}
