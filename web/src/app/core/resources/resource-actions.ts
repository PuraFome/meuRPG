/**
 * The feature actions of the class resource flows. `TakeAction` refuses them: the turn's rows open the dialog of each
 * one instead (`ResourceService`), and `GetTurnOptions.resource_targets` lists who the ones that pick a creature may
 * pick, by these keys.
 */
export const LAY_ON_HANDS_ACTION = 'feature:lay-on-hands';
export const FLEXIBLE_CREATE_ACTION = 'feature:flexible-casting-creating-spell-slots';
export const FLEXIBLE_CONVERT_ACTION = 'feature:flexible-casting-converting-spell-slot';
/** The bard's gift: the key says the die of the first level (`-d6`); the die itself grows with the bard's level. */
export const BARDIC_INSPIRATION_PREFIX = 'feature:bardic-inspiration';

/** Which dialog a feature action opens, or `null` for a feature that is a plain `TakeAction`. */
export type ResourceFlow = 'lay-on-hands' | 'flexible-create' | 'flexible-convert' | 'bardic-give';

export function resourceFlow(actionKey: string): ResourceFlow | null {
  if (actionKey === LAY_ON_HANDS_ACTION) {
    return 'lay-on-hands';
  }
  if (actionKey === FLEXIBLE_CREATE_ACTION) {
    return 'flexible-create';
  }
  if (actionKey === FLEXIBLE_CONVERT_ACTION) {
    return 'flexible-convert';
  }
  return actionKey.startsWith(BARDIC_INSPIRATION_PREFIX) ? 'bardic-give' : null;
}
