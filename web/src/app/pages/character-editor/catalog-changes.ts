import type { RulesCatalogVm } from './character-editor.types';

/** The keys a catalog offers the pickers that the master's switches move: races and sub-races, classes and subclasses, backgrounds, spells. */
function signature(c: RulesCatalogVm): string {
  return JSON.stringify([
    c.races.map((r) => [r.key, r.namePt, r.subraces.map((s) => s.key)]),
    c.classes.map((x) => [x.key, x.namePt, x.subclasses.map((s) => s.key)]),
    c.backgrounds.map((b) => b.key),
    c.spells.map((s) => [s.key, s.classKeys]),
  ]);
}

/** Whether the lists the person picks from are not the ones on screen (the master turned something on or off, or wrote an entry). */
export function catalogChanged(before: RulesCatalogVm, after: RulesCatalogVm): boolean {
  return signature(before) !== signature(after);
}

/** The form fields that can hold a content key the master switched off. */
export type OffField = 'race' | 'subrace' | 'class' | 'subclass' | 'background';

/** Which field of the form holds `key` (null when none does: a spell, or a choice the person already changed). */
export function offFieldOf(
  key: string,
  form: { race: string; subrace: string; className: string; subclassName: string; background: string },
): OffField | null {
  if (key === form.race) {
    return 'race';
  }
  if (key === form.subrace) {
    return 'subrace';
  }
  if (key === form.className) {
    return 'class';
  }
  if (key === form.subclassName) {
    return 'subclass';
  }
  if (key === form.background) {
    return 'background';
  }
  return null;
}
