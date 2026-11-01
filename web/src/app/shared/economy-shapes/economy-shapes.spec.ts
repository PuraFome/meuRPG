import { TestBed } from '@angular/core/testing';

import { ReactionKind } from '../../../gen/meurpg/play/v1/combat_pb';
import { EconomyShapes, economyShapes } from './economy-shapes';

const free = { actionUsed: false, bonusActionUsed: false, reactionUsed: false };

describe('economyShapes', () => {
  it('names the three shapes and leaves out the walker without a movement', () => {
    const shapes = economyShapes(free);
    expect(shapes.map((s) => s.kind)).toEqual(['action', 'bonus', 'reaction']);
    expect(shapes.map((s) => s.name)).toEqual([
      'Ação disponível',
      'Ação bônus disponível',
      'Reação disponível',
    ]);
    expect(shapes.some((s) => s.spent)).toBe(false);
  });

  it('draws the walker with the metres left', () => {
    const shapes = economyShapes(free, { moveText: '9 m' });
    expect(shapes.at(-1)).toEqual({
      kind: 'move',
      text: '9 m',
      name: 'Movimento: 9 m',
      spent: false,
    });
  });

  it('marks what was spent', () => {
    const shapes = economyShapes({ ...free, bonusActionUsed: true, reactionUsed: true });
    expect(shapes.map((s) => s.spent)).toEqual([false, true, true]);
    expect(shapes[1].name).toBe('Ação bônus usada');
  });

  it('names the reaction that was spent and the round', () => {
    const name = (over: object) => economyShapes({ ...free, reactionUsed: true, ...over })[2].name;
    expect(name({ reactionKind: ReactionKind.SHIELD, reactionRound: 2 })).toBe(
      'Reação usada (Escudo Arcano, rodada 2)',
    );
    expect(name({ reactionRound: 2 })).toBe('Reação usada (rodada 2)');
    expect(name({})).toBe('Reação usada');
    expect(
      economyShapes({ ...free, reactionKind: ReactionKind.SHIELD, reactionRound: 2 })[2].name,
    ).toBe('Reação disponível');
  });

  it('keeps the Action filled while Extra Attack has attacks left', () => {
    const shapes = economyShapes(
      { ...free, actionUsed: true },
      { attacksLeft: 1, attacksPerAction: 2 },
    );
    expect(shapes[0].spent).toBe(false);
    expect(shapes[0].name).toBe('Ação, 1 ataque restante');
  });

  it('spends all three for one who cannot act', () => {
    const shapes = economyShapes({ ...free, conditions: ['condition:paralyzed'] });
    expect(shapes.map((s) => s.spent)).toEqual([true, true, true]);
    expect(shapes[0].name).toBe('Ação indisponível');
  });
});

describe('EconomyShapes', () => {
  function render(inputs: { onlyLeft?: boolean; spent?: boolean }) {
    const fixture = TestBed.createComponent(EconomyShapes);
    fixture.componentRef.setInput(
      'shapes',
      economyShapes({ ...free, bonusActionUsed: inputs.spent ?? true }, { moveText: '9 m' }),
    );
    fixture.componentRef.setInput('onlyLeft', inputs.onlyLeft ?? false);
    fixture.detectChanges();
    return fixture.nativeElement as HTMLElement;
  }

  it('draws a circle, a triangle, a square and a walker, the spent one dashed with a slash', () => {
    const el = render({});
    const items = [...el.querySelectorAll('li')];
    expect(items.map((li) => li.getAttribute('aria-label'))).toEqual([
      'Ação disponível',
      'Ação bônus usada',
      'Reação disponível',
      'Movimento: 9 m',
    ]);
    expect(items[0].querySelector('circle.mark')).not.toBeNull();
    expect(items[1].querySelector('path.mark')).not.toBeNull();
    expect(items[1].classList.contains('shape--spent')).toBe(true);
    expect(items[1].querySelector('.slash')).not.toBeNull();
    expect(items[2].querySelector('rect.mark')).not.toBeNull();
    expect(items[3].querySelector('.walker')).not.toBeNull();
  });

  it('with onlyLeft, shows only what is still there', () => {
    const el = render({ onlyLeft: true });
    expect([...el.querySelectorAll('li')].map((li) => li.getAttribute('aria-label'))).toEqual([
      'Ação disponível',
      'Reação disponível',
      'Movimento: 9 m',
    ]);
  });
});
