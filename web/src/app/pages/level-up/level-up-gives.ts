import {
  LevelUpSpellsKind,
  type LevelUpOptions,
} from '../../../gen/meurpg/characters/v1/characters_pb';
import type { DerivedSheet, PactMagic } from '../../../gen/meurpg/rules/v1/rules_pb';
import {
  formatModifier,
  pactSlotsText,
  spellLevelLabel,
} from '../../core/characters/character-labels';
import type { LevelUpDraft } from '../../core/levelup/levelup-draft';
import { hitDiceSum } from '../../core/resources/hit-dice-text';

/** "1 magia conhecida", "2 magias conhecidas"; "1 magia para o livro", "2 magias para o livro"; empty for none. */
export function newSpellsTitle(count: number, spellbook: boolean): string {
  if (count < 1) {
    return '';
  }
  const noun = count === 1 ? '1 magia' : `${count} magias`;
  if (spellbook) {
    return `${noun} para o livro`;
  }
  return `${noun} ${count === 1 ? 'conhecida' : 'conhecidas'}`;
}

/** One line of "O que o nível N dá": what it is, and whether it is chosen, still to choose, or automatic. */
export interface GiveRow {
  readonly title: string;
  readonly sub: string;
  readonly tag: 'chosen' | 'choose' | 'auto';
}

/** What "O que o nível N dá" is worked out from: the player's picks, the level's options and the sheet before and after. */
export interface GiveInput {
  readonly draft: LevelUpDraft;
  readonly options: LevelUpOptions;
  readonly before: DerivedSheet;
  readonly after: DerivedSheet;
}

type ChoiceStep = 'abilities' | 'picks' | 'spells';

/** The feature keys the flow covers with a step of its own, so they are not listed as "Característica do nível". */
const OWN_STEP_FEATURE = /ability-score-improvement|subclass/;

/** A choice of the level: which step makes it, and whether it is made. */
function choiceRow(d: LevelUpDraft, title: string, step: ChoiceStep, done: boolean): GiveRow {
  const n = d.steps().indexOf(step) + 1;
  return {
    title,
    sub: done ? `Feito no passo ${n}` : `Passo ${n}`,
    tag: done ? 'chosen' : 'choose',
  };
}

/** Whether a choice of the step, named by one of the ids (or an id that starts with it), is still missing. */
function pending(d: LevelUpDraft, step: 'picks' | 'spells', ids: readonly string[]): boolean {
  return d
    .missing()
    .some((m) => m.step === step && ids.some((id) => m.id === id || m.id.startsWith(id)));
}

/** The title of the ability step's line: a feat taken, a feat or an increase, or the increase alone. */
function abilityTitle(d: LevelUpDraft): string {
  const feat = d.featSummary();
  if (feat) {
    return `Talento: ${feat.name}`;
  }
  return d.hasFeats() ? 'Incremento ou talento' : 'Incremento no Valor de Habilidade';
}

/** The choices of the "Escolhas" step, in the order the step shows them. */
function pickRows(i: GiveInput): GiveRow[] {
  const { draft: d, options: o } = i;
  const t = d.totals();
  const rows: GiveRow[] = [];
  const picks = (title: string, ids: readonly string[]) =>
    rows.push(choiceRow(d, title, 'picks', !pending(d, 'picks', ids)));
  if (o.lateChoices.length > 0) {
    picks('Escolhas que ficaram para trás', ['late']);
  }
  if (o.newChoices.length > 0) {
    picks('Escolhas do nível', ['new']);
  }
  if (o.subclassDue) {
    rows.push(choiceRow(d, 'Subclasse', 'picks', d.subclassKey() !== ''));
  }
  t.featureChoices.forEach((f, idx) =>
    picks(f.feature?.namePt ?? 'Característica', [`feature-${idx}`]),
  );
  if (t.skills > 0) {
    picks(t.skills === 1 ? 'Perícia nova' : `${t.skills} perícias novas`, ['skills']);
  }
  if (t.expertise > 0) {
    picks('Especialização', ['expertise']);
  }
  return rows;
}

/** The choices of the "Magias" step: new cantrips and spells together, and the prepared maximum. */
function spellRows(i: GiveInput): GiveRow[] {
  const { draft: d } = i;
  const t = d.totals();
  const rows: GiveRow[] = [];
  const cantrips = t.cantrips === 1 ? 'Truque novo' : `${t.cantrips} truques novos`;
  const bits = [
    t.cantrips > 0 ? cantrips : '',
    newSpellsTitle(t.spells, d.effective().spellsKind === LevelUpSpellsKind.SPELLBOOK),
  ].filter((s) => s !== '');
  if (bits.length > 0) {
    rows.push(
      choiceRow(d, bits.join(' e '), 'spells', !pending(d, 'spells', ['cantrips', 'spells'])),
    );
  }
  if (d.effective().prepares && d.preparedMaxAfter() > 0) {
    rows.push(
      choiceRow(
        d,
        `Magias preparadas: até ${d.preparedMaxAfter()}`,
        'spells',
        !pending(d, 'spells', ['prepared']),
      ),
    );
  }
  return rows;
}

/** What the player chooses, in the order of the steps: abilities, picks, spells. */
function choiceRows(i: GiveInput): GiveRow[] {
  const { draft: d, options: o } = i;
  const rows: GiveRow[] = [];
  if (o.abilityScoreImprovement) {
    rows.push(choiceRow(d, abilityTitle(d), 'abilities', d.missingIn('abilities').length === 0));
  }
  return [...rows, ...pickRows(i), ...spellRows(i)];
}

const auto = (title: string, change: string): GiveRow => ({
  title,
  sub: `Entra sozinho · ${change}`,
  tag: 'auto',
});

/** The spell slots of each circle that change with the level. */
function slotRows(o: LevelUpOptions): GiveRow[] {
  const rows: GiveRow[] = [];
  const at = (arr: readonly number[], idx: number) => arr[idx] ?? 0;
  for (let idx = 0; idx < Math.max(o.spellSlotsBefore.length, o.spellSlotsAfter.length); idx++) {
    const was = at(o.spellSlotsBefore, idx);
    const now = at(o.spellSlotsAfter, idx);
    if (was !== now) rows.push(auto(`Espaços de ${spellLevelLabel(idx + 1)}`, `${was} → ${now}`));
  }
  return rows;
}

/** What comes with the level with no choice: slots, the pact slots, the proficiency bonus and the hit dice. */
function autoRows(i: GiveInput): GiveRow[] {
  const { options: o } = i;
  const rows = slotRows(o);
  const pact = (m: PactMagic | undefined) => pactSlotsText(m);
  if (pact(o.pactMagicBefore) !== pact(o.pactMagicAfter)) {
    rows.push(auto('Espaços do pacto', `${pact(o.pactMagicBefore)} → ${pact(o.pactMagicAfter)}`));
  }
  if (o.proficiencyBonusBefore !== o.proficiencyBonusAfter) {
    rows.push(
      auto(
        'Bônus de proficiência',
        `${formatModifier(o.proficiencyBonusBefore)} → ${formatModifier(o.proficiencyBonusAfter)}`,
      ),
    );
  }
  rows.push(
    auto('Dados de vida', `${hitDiceSum(i.before.hitDice)} → ${hitDiceSum(i.after.hitDice)}`),
  );
  return rows;
}

/** The features the level gives that the player does not choose, nor the master adds, nor a step of its own covers. */
function featureRows(i: GiveInput): GiveRow[] {
  const { draft: d, options: o } = i;
  const chosen = new Set(d.totals().featureChoices.map((f) => f.feature?.key));
  const skip = new Set(o.masterAdds.map((m) => m.key));
  return o.newFeatures
    .filter((f) => !chosen.has(f.key) && !skip.has(f.key) && !OWN_STEP_FEATURE.test(f.key))
    .map((f) => ({ title: f.namePt, sub: 'Característica do nível', tag: 'auto' as const }));
}

/** "O que o nível N dá", in the order of the steps: chosen, still to choose, and automatic. */
export function givesOf(i: GiveInput): GiveRow[] {
  return [...choiceRows(i), ...autoRows(i), ...featureRows(i)];
}
