import { ComponentFixture, TestBed } from '@angular/core/testing';

import { Ability } from '../../../../gen/meurpg/rules/v1/rules_pb';
import { classChoice } from '../../../core/levelup/levelup-testing';
import { ClassPick, requirementText } from './class-pick';

describe('ClassPick', () => {
  const monk = classChoice({
    classKey: 'class:monk',
    namePt: 'Monge',
    isNew: true,
    available: false,
    prerequisiteMet: false,
    prerequisites: [
      { ability: Ability.DEXTERITY, minimum: 13, have: 14, met: true },
      { ability: Ability.WISDOM, minimum: 13, have: 11, met: false },
    ],
  });
  const fighter = classChoice({
    classKey: 'class:fighter',
    namePt: 'Guerreiro',
    fromLevel: 5,
    toLevel: 6,
    prerequisiteAnyOf: true,
    prerequisites: [
      { ability: Ability.STRENGTH, minimum: 13, have: 16, met: true },
      { ability: Ability.DEXTERITY, minimum: 13, have: 14, met: true },
    ],
  });
  const wizard = classChoice({
    classKey: 'class:wizard',
    namePt: 'Mago',
    isNew: true,
    toLevel: 1,
    prerequisites: [{ ability: Ability.INTELLIGENCE, minimum: 13, have: 13, met: true }],
  });

  function make(newMode: boolean): ComponentFixture<ClassPick> {
    const f = TestBed.createComponent(ClassPick);
    f.componentRef.setInput('choices', [fighter, wizard, monk]);
    f.componentRef.setInput('selected', 'class:fighter');
    f.componentRef.setInput('newMode', newMode);
    f.componentRef.setInput('totalToLevel', 6);
    f.detectChanges();
    return f;
  }
  const text = (f: ComponentFixture<ClassPick>) =>
    (f.nativeElement as HTMLElement).textContent?.replace(/\s+/g, ' ') ?? '';

  it('words what a class asks for: "e" for all of them, "ou" for one', () => {
    expect(requirementText(monk)).toBe('Destreza 13 e Sabedoria 13');
    expect(requirementText(fighter)).toBe('Força 13 ou Destreza 13');
  });

  it('draws the class the character has, and "Uma classe nova" closed until it is asked for', () => {
    const f = make(false);
    expect(text(f)).toContain('Guerreironível 5 → 6');
    expect(text(f)).toContain(
      'Uma classe nova Entra com o nível 1 da classe: o seu nível total vai a 6.',
    );
    expect(text(f)).not.toContain('Qual classe nova?');
  });

  it('says what each new class asks for and what the character has, in words and not only in color', () => {
    const f = make(true);
    expect(text(f)).toContain('Qual classe nova?');
    expect(text(f)).toContain('Exige Inteligência 13. Você tem Inteligência 13.');
    expect(text(f)).toContain('Exige Destreza 13 e Sabedoria 13.');
    expect(text(f)).toContain('Falta: Sabedoria 13 (você tem 11).');
    const radios = (f.nativeElement as HTMLElement).querySelectorAll<HTMLInputElement>(
      'input[name=level-up-new-class]',
    );
    expect(Array.from(radios, (r) => r.getAttribute('aria-disabled'))).toEqual([null, 'true']);
    // A closed card stays in the tab order: it is aria-disabled, not disabled.
    expect(radios[1].disabled).toBe(false);
  });

  it('emits the class of an open card and nothing for a closed one', () => {
    const f = make(true);
    const picked: string[] = [];
    f.componentInstance.pick.subscribe((k) => picked.push(k));
    const radios = (f.nativeElement as HTMLElement).querySelectorAll<HTMLInputElement>(
      'input[name=level-up-new-class]',
    );
    radios[1].click();
    radios[0].click();
    expect(picked).toEqual(['class:wizard']);
  });
});
