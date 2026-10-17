import { toggledExpertise, toggledSkill } from './skill-picks';

const granted = new Map([['skill:arcana', 'da raça']]);

describe('toggledSkill', () => {
  it('does nothing on a skill that is given', () => {
    expect(toggledSkill(new Set(), new Set(), granted, 'skill:arcana')).toBeNull();
  });

  it('picks a skill, leaving the expertise alone', () => {
    const next = toggledSkill(new Set(['a']), new Set(), granted, 'b');
    expect([...next!.skills]).toEqual(['a', 'b']);
    expect(next!.expertise).toBeNull();
  });

  it('unpicks a skill and drops its expertise, so expertise stays inside the proficient skills', () => {
    const next = toggledSkill(new Set(['a', 'b']), new Set(['a', 'b']), granted, 'b');
    expect([...next!.skills]).toEqual(['a']);
    expect([...next!.expertise!]).toEqual(['a']);
  });

  it('unpicks a skill with no expertise without touching the expertise', () => {
    const next = toggledSkill(new Set(['a']), new Set(['x']), granted, 'a');
    expect(next!.skills.size).toBe(0);
    expect(next!.expertise).toBeNull();
  });
});

describe('toggledExpertise', () => {
  it('does nothing on a skill that is not proficient', () => {
    expect(toggledExpertise(new Set(['a']), granted, new Set(), 'b')).toBeNull();
  });

  it('toggles it on a picked skill and on a given one', () => {
    expect([...toggledExpertise(new Set(['a']), granted, new Set(), 'a')!]).toEqual(['a']);
    expect([...toggledExpertise(new Set(), granted, new Set(), 'skill:arcana')!]).toEqual([
      'skill:arcana',
    ]);
    expect(toggledExpertise(new Set(['a']), granted, new Set(['a']), 'a')!.size).toBe(0);
  });
});
