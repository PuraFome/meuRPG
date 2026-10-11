import { CREATURE_TYPES } from './creature-types';

describe('CREATURE_TYPES', () => {
  // The cards say these words (rules/creatures.go) and so do the Favored Enemy's options (names_pt.json):
  // the filter must say the same, not "Limo" or "Constructo".
  it('uses the official Portuguese words the creature cards use', () => {
    const label = (value: string) => CREATURE_TYPES.find((t) => t.value === value)?.label;
    expect(label('ooze')).toBe('Gosma');
    expect(label('construct')).toBe('Construto');
    expect(CREATURE_TYPES.map((t) => t.label)).not.toContain('Limo');
    expect(CREATURE_TYPES.map((t) => t.label)).not.toContain('Constructo');
  });
});
