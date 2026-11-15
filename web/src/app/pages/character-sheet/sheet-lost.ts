import type { FullSheet as GenFullSheet } from '../../../gen/meurpg/characters/v1/characters_pb';
import type { DerivedSheet as GenDerivedSheet } from '../../../gen/meurpg/rules/v1/rules_pb';
import type { FullSheetVm, IssueVm } from './character-sheet.types';
import { lostKeys } from './sheet-format';

/** The sheet's features, each marked when its prerequisite is lost (board 580-05, "Não vale agora"). */
export function featuresVm(full: GenFullSheet, derived: GenDerivedSheet) {
  const lost = lostKeys(full, derived.issues);
  return derived.features.map((f) => ({
    name: f.namePt,
    sourcePt: f.sourcePt,
    description: f.description,
    descriptionPt: f.descriptionPt,
    descriptionPtMissing: f.descriptionPtMissing,
    descriptionPtOnly: f.descriptionPtOnly,
    summaryPt: f.summaryPt,
    lost: lost.has(f.key),
  }));
}

/**
 * The sheet's own issues, for "Pendências de regra": what "A classe mudou" tells above (the issue
 * `table_content_changed` and any sentence a changed entry already lists) is not repeated in the list.
 */
export function ownIssues(full: FullSheetVm): readonly IssueVm[] {
  if (full.changedContent.length === 0) {
    return full.issues;
  }
  const told = new Set(full.changedContent.flatMap((c) => c.messages));
  return full.issues.filter((i) => i.code !== 'table_content_changed' && !told.has(i.message));
}
