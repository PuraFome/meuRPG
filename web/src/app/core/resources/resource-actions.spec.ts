import { resourceFlow } from './resource-actions';
import { poolOf, pointsText } from './pools';

describe('resourceFlow', () => {
  it('routes the four feature actions that TakeAction refuses to their dialogs', () => {
    expect(resourceFlow('feature:lay-on-hands')).toBe('lay-on-hands');
    expect(resourceFlow('feature:flexible-casting-creating-spell-slots')).toBe('flexible-create');
    expect(resourceFlow('feature:flexible-casting-converting-spell-slot')).toBe('flexible-convert');
    expect(resourceFlow('feature:bardic-inspiration-d6')).toBe('bardic-give');
  });

  it('leaves any other feature to TakeAction', () => {
    expect(resourceFlow('feature:second-wind')).toBeNull();
    expect(resourceFlow('feature:wild-shape')).toBeNull();
    expect(resourceFlow('standard:dash')).toBeNull();
  });
});

describe('pools', () => {
  const resources = [
    { key: 'lay_on_hands', total: 25, used: 8, recharge: 'long_rest' as const },
    { key: 'sorcery_points', total: 5, used: 9, recharge: 'long_rest' as const },
  ];

  it('reads what is left of a resource, never below nothing', () => {
    expect(poolOf(resources, 'lay_on_hands')).toEqual({ left: 17, total: 25 });
    expect(poolOf(resources, 'sorcery_points')).toEqual({ left: 0, total: 5 });
  });

  it('is null for a character without the resource, or without a list', () => {
    expect(poolOf(resources, 'bardic_inspiration')).toBeNull();
    expect(poolOf(undefined, 'lay_on_hands')).toBeNull();
  });

  it('says points in the singular and the plural', () => {
    expect(pointsText(1)).toBe('1 ponto');
    expect(pointsText(8)).toBe('8 pontos');
  });
});
