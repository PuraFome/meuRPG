import type { MessageInitShape } from '@bufbuild/protobuf';

import type { AbilityScores } from '../../../gen/meurpg/rules/v1/rules_pb';
import {
  type TableFeat,
  type TableFeatSchema,
} from '../../../gen/meurpg/rules/v1/table_content_pb';
import { type EffectMenuVm, draftToEffect, effectToDraft } from './effect-draft';
import {
  type FeatureDraft,
  ABILITY_FIELDS,
  type AbilityField,
  emptyFeature,
} from './feature-draft';
import { paragraphs } from './spell-draft';

/**
 * The feat form (MR-025): a name, a text, what the feat asks of a character and its effects, which come from the same
 * menu as a feature's (plus `ability_increase`, only a feat's). The feature editor holds the name, the text and the
 * effects (`feature`, with no key: a feat has none); the prerequisite fields are kept as text so a half-typed number
 * is never rewritten. Whether a character meets the prerequisite is the server's.
 */

export type FeatInit = MessageInitShape<typeof TableFeatSchema>;

/** The score asked of each ability, as typed ("13"); empty asks nothing. */
export type Scores = Record<AbilityField, string>;

export function noScores(): Scores {
  return {
    strength: '',
    dexterity: '',
    constitution: '',
    intelligence: '',
    wisdom: '',
    charisma: '',
  };
}

export interface FeatDraft {
  feature: FeatureDraft;
  minimums: Scores;
  anyOf: Scores;
  proficiencyKey: string;
  spellcasting: boolean;
  raceKey: string;
  /** As typed; empty asks nothing. */
  level: string;
}

export function emptyFeat(): FeatDraft {
  return {
    feature: emptyFeature(),
    minimums: noScores(),
    anyOf: noScores(),
    proficiencyKey: '',
    spellcasting: false,
    raceKey: '',
    level: '',
  };
}

function scoresOf(s: AbilityScores | undefined): Scores {
  const out = noScores();
  for (const a of ABILITY_FIELDS) {
    const v = s?.[a] ?? 0;
    out[a] = v > 0 ? String(v) : '';
  }
  return out;
}

export function featToDraft(f: TableFeat): FeatDraft {
  const p = f.prerequisite;
  return {
    feature: {
      ...emptyFeature(),
      name: f.namePt,
      text: f.descPt.join('\n\n'),
      effects: f.effects.map(effectToDraft),
    },
    minimums: scoresOf(p?.minimums),
    anyOf: scoresOf(p?.anyOf),
    proficiencyKey: p?.proficiencyKey ?? '',
    spellcasting: p?.spellcasting ?? false,
    raceKey: p?.raceKey ?? '',
    level: p && p.level > 0 ? String(p.level) : '',
  };
}

/** The abilities with a score typed; an unreadable text is NaN, which the server refuses at that field. */
function scoresInit(s: Scores): Partial<Record<AbilityField, number>> {
  const out: Partial<Record<AbilityField, number>> = {};
  for (const a of ABILITY_FIELDS) {
    const t = s[a].trim();
    if (t !== '') {
      out[a] = Number(t);
    }
  }
  return out;
}

export function draftToFeat(d: FeatDraft, menu: EffectMenuVm): FeatInit {
  const minimums = scoresInit(d.minimums);
  const anyOf = scoresInit(d.anyOf);
  const level = d.level.trim();
  return {
    namePt: d.feature.name.trim(),
    descPt: paragraphs(d.feature.text),
    prerequisite: {
      ...(Object.keys(minimums).length > 0 ? { minimums } : {}),
      ...(Object.keys(anyOf).length > 0 ? { anyOf } : {}),
      proficiencyKey: d.proficiencyKey,
      spellcasting: d.spellcasting,
      raceKey: d.raceKey,
      level: level === '' ? 0 : Number(level),
    },
    effects: d.feature.effects.map((e) => draftToEffect(e, menu)),
  };
}

/** The paths of the inputs the feat editor draws (a violation at another path lands on the nearest one above). */
export function featPaths(d: FeatDraft, menu: EffectMenuVm): string[] {
  const p = 'table_feat';
  const out = [
    `${p}.name_pt`,
    `${p}.desc_pt`,
    `${p}.prerequisite`,
    `${p}.prerequisite.minimums`,
    `${p}.prerequisite.any_of`,
    `${p}.prerequisite.proficiency_key`,
    `${p}.prerequisite.race_key`,
    `${p}.prerequisite.level`,
    `${p}.prerequisite.spellcasting`,
    `${p}.effects`,
  ];
  for (const a of ABILITY_FIELDS) {
    out.push(`${p}.prerequisite.minimums.${a}`, `${p}.prerequisite.any_of.${a}`);
  }
  d.feature.effects.forEach((e, k) => {
    const at = `${p}.effects[${k}]`;
    out.push(at, `${at}.type`);
    for (const field of menu.fieldsOf(e.type)) {
      out.push(`${at}.${field.name}`);
    }
  });
  return out;
}
