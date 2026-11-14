import { FeatureEconomy } from '../../../gen/meurpg/play/v1/combat_pb';

/** What the server writes of a use's cost in `CombatLogFeatureUsed` (fields 7-9, read by the master and the user's player only). */
export interface FeatureCostFields {
  readonly economy: FeatureEconomy;
  readonly resourceNamePt: string;
  readonly resourceLeft: number;
  /** The most uses of the resource, or of the scope for a use with no resource of its own; 0 for a use with no limit. */
  readonly resourceMax: number;
}

/** "gastou a ação", "gastou a ação bônus", "gastou a reação"; `''` for a use that cost no part of the turn. */
function economyText(economy: FeatureEconomy): string {
  switch (economy) {
    case FeatureEconomy.ACTION:
      return 'a ação';
    case FeatureEconomy.BONUS_ACTION:
      return 'a ação bônus';
    case FeatureEconomy.REACTION:
      return 'a reação';
    default:
      return '';
  }
}

/** "(restam 2)", and "(restam 2 de 3)" once the entry says the most: the one place that writes what is left. */
function leftText(f: FeatureCostFields): string {
  const of = f.resourceMax > 0 ? ` de ${f.resourceMax}` : '';
  return `(restam ${f.resourceLeft}${of})`;
}

/**
 * "1 uso de Forma Selvagem (restam 2 de 3)": the resource spent. A use limited only by its scope has no name ("1 uso (restam 1 de
 * 2)"); with no name and no limit the server sent no resource at all.
 */
function resourceText(f: FeatureCostFields): string {
  if (!f.resourceNamePt && f.resourceMax <= 0) {
    return '';
  }
  const name = f.resourceNamePt ? ` de ${f.resourceNamePt}` : '';
  return `1 uso${name} ${leftText(f)}`;
}

/**
 * What a use cost, said by the typed fields: "gastou a reação", "gastou a ação e 1 uso de Forma Selvagem (restam 2 de 3)". `''` when
 * the entry says nothing (UNSPECIFIED with no resource: another player's line, or an older entry), so the line stays the plain one.
 */
export function featureCostText(f: FeatureCostFields): string {
  const parts = [economyText(f.economy), resourceText(f)].filter(Boolean);
  return parts.length > 0 ? `gastou ${parts.join(' e ')}` : '';
}
