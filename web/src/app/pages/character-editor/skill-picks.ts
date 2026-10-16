import { toggledIn } from './spell-picks';

/** The most skills the "Outro" background's feature gives (`CustomBackground.skill_keys`: "at most 2"). */
export const CUSTOM_BACKGROUND_SKILLS = 2;

/** The most tools or languages the "Outro" background gives. */
export const CUSTOM_BACKGROUND_PROFICIENCIES = 2;

/** The skills after a click on one: `null` when the click does nothing (a skill the race or the background gives is
 * locked). `expertise` is the new expertise set, only when untick drops it: expertise is always a subset of the
 * proficient skills, so it never breaks silently. */
export function toggledSkill(
  selected: ReadonlySet<string>,
  expertise: ReadonlySet<string>,
  granted: ReadonlyMap<string, string>,
  key: string,
): { skills: Set<string>; expertise: Set<string> | null } | null {
  if (granted.has(key)) {
    return null;
  }
  const skills = new Set(selected);
  if (!skills.has(key)) {
    skills.add(key);
    return { skills, expertise: null };
  }
  skills.delete(key);
  return { skills, expertise: expertise.has(key) ? toggledIn(expertise, key) : null };
}

/** The expertise after a click: `null` on a skill that is not proficient yet (the checkbox is disabled for those). */
export function toggledExpertise(
  selected: ReadonlySet<string>,
  granted: ReadonlyMap<string, string>,
  expertise: ReadonlySet<string>,
  key: string,
): Set<string> | null {
  if (!selected.has(key) && !granted.has(key)) {
    return null;
  }
  return toggledIn(expertise, key);
}

/** The set after a click when it takes at most `cap`: a third click while two are in does nothing, so the player
 * unticks one first. */
export function toggledCapped(current: ReadonlySet<string>, key: string, cap: number): Set<string> {
  const next = new Set(current);
  if (next.has(key)) {
    next.delete(key);
  } else if (next.size < cap) {
    next.add(key);
  }
  return next;
}
