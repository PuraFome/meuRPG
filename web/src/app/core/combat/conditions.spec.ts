import { cannotAct, cannotActWhy } from './conditions';

describe('cannotActWhy', () => {
  it('names the condition that takes the actions', () => {
    expect(cannotActWhy({ conditions: ['condition:stunned'] })).toBe('Atordoado: não pode agir.');
    expect(cannotActWhy({ conditions: ['condition:poisoned', 'condition:paralyzed'] })).toBe(
      'Paralisado: não pode agir.',
    );
  });

  it('is empty for who can act', () => {
    expect(cannotActWhy({ conditions: ['condition:poisoned'] })).toBe('');
    expect(cannotActWhy({})).toBe('');
    expect(cannotAct({ conditions: ['condition:poisoned'] })).toBe(false);
  });
});
