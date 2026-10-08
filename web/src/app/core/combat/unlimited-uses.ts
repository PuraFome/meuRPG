/**
 * The server stores "unlimited uses" (a barbarian's Rage at level 20, a druid's
 * Wild Shape at level 20) as a resource of 99 uses. Screens say "ilimitado"
 * instead of a count that would never run out.
 */
export const UNLIMITED_USES = 99;

/** Whether a resource's maximum stands for unlimited uses. */
export function isUnlimited(total: number): boolean {
  return total >= UNLIMITED_USES;
}
