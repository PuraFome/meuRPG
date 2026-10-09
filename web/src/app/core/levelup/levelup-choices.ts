import {
  type Choice,
  type ChoiceGroup,
  ChoiceKind,
} from '../../../gen/meurpg/characters/v1/characters_pb';
import { choiceTextKey } from '../../shared/choice-groups/choice-picks';

/**
 * The pure half of the choices a level-up carries besides the feature options (PM-05): the ones an earlier level left
 * open (`LevelUpOptions.late_choices`) and the new level's choices that are not options of a feature
 * (`LevelUpOptions.new_choices`: a favored enemy and its language, a terrain, a Mystic Arcanum...). The server says what
 * each choice offers and how many picks it still takes; here the picks are only counted against that, and written the way
 * `LevelUpChoices` wants them (the stored keys, and the free texts of `feature_choice_text`).
 */

/** What the player picked on one choice: the whole selection (the picks already made included) and its free texts. */
export interface ChoiceDraft {
  readonly optionKeys: readonly string[];
  /** The two humanoid races of a favored enemy, 1-based in `choiceTextKey`; empty strings for the ones not written. */
  readonly texts: readonly string[];
}

export type ChoiceDrafts = ReadonlyMap<string, ChoiceDraft>;

/** The choices of the groups, in order. */
export function choicesOf(groups: readonly ChoiceGroup[]): Choice[] {
  return groups.flatMap((g) => g.choices);
}

/** What the player has on `choice`: what they picked here, else what the sheet already has. */
export function draftOf(drafts: ChoiceDrafts, choice: Choice): ChoiceDraft {
  return drafts.get(choice.key) ?? { optionKeys: choice.picked, texts: choice.texts };
}

/** Whether the picked option of `choice` asks for texts (the humanoid favored enemy names two races). */
function needsTexts(choice: Choice, draft: ChoiceDraft): boolean {
  return draft.optionKeys.some((k) => choice.options.find((o) => o.key === k)?.needsText);
}

/** How many selections of `choice` are made: a favored enemy whose races are not both written is one short. */
export function doneCount(choice: Choice, draft: ChoiceDraft): number {
  // A pick the sheet already has never goes away, whatever the card shows.
  const held = new Set([...choice.picked, ...draft.optionKeys]);
  const n = Math.min(held.size, choice.picks);
  const incomplete =
    choice.kind === ChoiceKind.ENEMY &&
    n > 0 &&
    needsTexts(choice, draft) &&
    [0, 1].some((i) => (draft.texts[i] ?? '').trim() === '');
  return incomplete ? n - 1 : n;
}

/** How many selections the groups still lack, and the label of each choice that lacks some. */
export function missingIn(
  groups: readonly ChoiceGroup[],
  drafts: ChoiceDrafts,
): { count: number; labels: string[] } {
  let count = 0;
  const labels: string[] = [];
  for (const choice of choicesOf(groups)) {
    const left = choice.picks - doneCount(choice, draftOf(drafts, choice));
    if (left > 0) {
      count += left;
      labels.push(choice.labelPt);
    }
  }
  return { count, labels };
}

/** The keys to send for the groups: the stored key of every option picked now, in the order of the choices. */
export function storedKeysOf(groups: readonly ChoiceGroup[], drafts: ChoiceDrafts): string[] {
  const out: string[] = [];
  for (const choice of choicesOf(groups)) {
    const stored = new Map(choice.options.map((o) => [o.key, o.storedKey]));
    for (const key of draftOf(drafts, choice).optionKeys) {
      if (!choice.picked.includes(key)) {
        out.push(stored.get(key) ?? key);
      }
    }
  }
  return out;
}

/** The free texts to send for the groups, by `choiceTextKey`, only for a choice whose picked option takes them. */
export function textsOf(
  groups: readonly ChoiceGroup[],
  drafts: ChoiceDrafts,
): Record<string, string> {
  const out: Record<string, string> = {};
  for (const choice of choicesOf(groups)) {
    const draft = draftOf(drafts, choice);
    if (!needsTexts(choice, draft)) {
      continue;
    }
    draft.texts.forEach((text, i) => {
      if (text.trim() !== '') {
        out[choiceTextKey(choice.key, i + 1)] = text;
      }
    });
  }
  return out;
}

/** `drafts` with the whole selection of `choice` set. */
export function withSelection(
  drafts: ChoiceDrafts,
  choice: Choice,
  optionKeys: readonly string[],
): ChoiceDrafts {
  const now = draftOf(drafts, choice);
  return new Map(drafts).set(choice.key, { optionKeys, texts: now.texts });
}

/** `drafts` with the n-th (from 1) text of `choice` set. */
export function withText(
  drafts: ChoiceDrafts,
  choice: Choice,
  n: number,
  text: string,
): ChoiceDrafts {
  const now = draftOf(drafts, choice);
  const texts = [...now.texts];
  while (texts.length < 2) {
    texts.push('');
  }
  texts[n - 1] = text;
  return new Map(drafts).set(choice.key, { optionKeys: now.optionKeys, texts });
}

/** Keeps only the drafts of choices the groups still have (after the sheet was read again). */
export function keepDraftsOf(groups: readonly ChoiceGroup[], drafts: ChoiceDrafts): ChoiceDrafts {
  const wanted = new Map(choicesOf(groups).map((c) => [c.key, c]));
  const out = new Map<string, ChoiceDraft>();
  for (const [key, draft] of drafts) {
    const choice = wanted.get(key);
    if (!choice) {
      continue;
    }
    const offered = new Set(choice.options.map((o) => o.key));
    out.set(key, {
      optionKeys: draft.optionKeys.filter((k) => offered.has(k)).slice(0, choice.picks),
      texts: draft.texts,
    });
  }
  return out;
}
