import { TestBed } from '@angular/core/testing';

import { SkillOptionVm } from '../character-editor.types';
import { SkillPicker } from './skill-picker';

const SKILLS: SkillOptionVm[] = [
  { key: 'skill:acrobatics', namePt: 'Acrobacia', ability: 'dex' },
  { key: 'skill:arcana', namePt: 'Arcanismo', ability: 'int' },
  { key: 'skill:history', namePt: 'História', ability: 'int' },
];

describe('SkillPicker', () => {
  function render(proficient: string[], expertise: string[] = []) {
    TestBed.configureTestingModule({ imports: [SkillPicker] });
    const fixture = TestBed.createComponent(SkillPicker);
    fixture.componentRef.setInput('skills', SKILLS);
    fixture.componentRef.setInput('proficient', new Set(proficient));
    fixture.componentRef.setInput('expertise', new Set(expertise));
    fixture.detectChanges();
    return { fixture, el: fixture.nativeElement as HTMLElement };
  }

  it('shows each skill with its ability abbreviation, in a group named "Perícias"', () => {
    const { el } = render([]);

    const group = el.querySelector('[role="group"]');
    expect(group?.getAttribute('aria-label')).toBe('Perícias');
    expect(el.textContent).toContain('Acrobacia');
    expect(el.textContent).toContain('Des');
    expect(el.textContent).toContain('Nenhuma perícia marcada');
  });

  it('names each expertise box by its skill and only enables it on a proficient skill', () => {
    const { el } = render(['skill:arcana'], ['skill:arcana']);

    const expertise = Array.from(el.querySelectorAll<HTMLInputElement>('.skill__expertise input'));
    expect(expertise.map((i) => i.getAttribute('aria-label'))).toEqual([
      'Especialização em Acrobacia',
      'Especialização em Arcanismo',
      'Especialização em História',
    ]);
    expect(expertise.map((i) => i.disabled)).toEqual([true, false, true]);
    expect(el.textContent).toContain('1 perícia marcada, 1 com especialização');
  });

  it('reports clicks without changing anything itself', () => {
    const { fixture, el } = render([]);
    const toggled: string[] = [];
    fixture.componentInstance.toggleSkill.subscribe((key) => toggled.push(key));

    el.querySelector<HTMLInputElement>('.skill__name input')?.click();

    expect(toggled).toEqual(['skill:acrobatics']);
  });
});
