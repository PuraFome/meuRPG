import { TestBed } from '@angular/core/testing';

import { CreatureArt } from './creature-art';

describe('CreatureArt: the picture by key and type (MR-042)', () => {
  function shape(key: string, type = ''): string {
    const fixture = TestBed.createComponent(CreatureArt);
    fixture.componentRef.setInput('monsterKey', key);
    fixture.componentRef.setInput('type', type);
    fixture.detectChanges();
    return (fixture.nativeElement as HTMLElement).querySelector('svg')!.getAttribute('data-shape')!;
  }

  it('a humanoid is a person and a giant a larger figure', () => {
    expect(shape('monster:bandit', 'humanoid')).toBe('person');
    expect(shape('monster:ogre', 'giant')).toBe('giant');
  });

  it('the wolves are a wolf, the Werewolf\'s wolf form too (a humanoid), and the spiders a spider', () => {
    expect(shape('monster:wolf', 'beast')).toBe('dog');
    expect(shape('monster:werewolf-wolf-form', 'humanoid')).toBe('dog');
    expect(shape('monster:giant-wolf-spider', 'beast')).toBe('spider');
  });

  it('a beast is a paw, a bird a bird, and any other type a neutral glyph, never a paw', () => {
    expect(shape('monster:rhinoceros', 'beast')).toBe('paw');
    expect(shape('monster:raven', 'beast')).toBe('bird');
    expect(shape('monster:lich', 'undead')).toBe('other');
    expect(shape('monster:aboleth', 'aberration')).toBe('other');
  });

  it('with no type (a familiar, a Wild Shape form) it is as before', () => {
    expect(shape('monster:rhinoceros')).toBe('paw');
    expect(shape('monster:raven')).toBe('bird');
  });
});
