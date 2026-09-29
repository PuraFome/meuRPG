import {
  abilityLabel,
  characterKindLabel,
  characterStateLabel,
  formatModifier,
  formatSpeedFt,
  skillProficiencyLabel,
} from './character-labels';

describe('abilityLabel', () => {
  it('names the six abilities in sheet order, in Portuguese', () => {
    expect(abilityLabel('str')).toBe('Força');
    expect(abilityLabel('dex')).toBe('Destreza');
    expect(abilityLabel('con')).toBe('Constituição');
    expect(abilityLabel('int')).toBe('Inteligência');
    expect(abilityLabel('wis')).toBe('Sabedoria');
    expect(abilityLabel('cha')).toBe('Carisma');
  });
});

describe('characterKindLabel', () => {
  it('names every kind, with no "D&D" trademark anywhere', () => {
    const labels = [
      characterKindLabel('player'),
      characterKindLabel('enemy'),
      characterKindLabel('boss'),
      characterKindLabel('minion'),
      characterKindLabel('story'),
    ];
    for (const label of labels) {
      expect(label).not.toContain('D&D');
      expect(label.length).toBeGreaterThan(0);
    }
    expect(characterKindLabel('enemy')).toBe('Inimigo');
    expect(characterKindLabel('boss')).toBe('Boss');
    expect(characterKindLabel('minion')).toBe('Minion');
    expect(characterKindLabel('story')).toBe('NPC de história');
  });
});

describe('characterStateLabel', () => {
  it('matches docs/produto/regras.md\'s lifecycle names', () => {
    expect(characterStateLabel('draft')).toBe('Rascunho');
    expect(characterStateLabel('locked')).toBe('Travada');
    expect(characterStateLabel('dead')).toBe('Morto');
  });
});

describe('formatModifier', () => {
  it('always shows a sign, even for zero, and never recomputes it', () => {
    expect(formatModifier(3)).toBe('+3');
    expect(formatModifier(0)).toBe('+0');
    expect(formatModifier(-1)).toBe('-1');
    // The plan's own example: a score of 18 paired with an inconsistent
    // +9 modifier must still render "+9" — the browser trusts the server.
    expect(formatModifier(9)).toBe('+9');
  });
});

describe('formatSpeedFt', () => {
  it('shows meters and feet, matching the plan\'s worked example exactly', () => {
    expect(formatSpeedFt(25)).toBe('7,5 m (25 pés)');
  });

  it('drops a trailing ",0" when the meters value is a whole number', () => {
    expect(formatSpeedFt(30)).toBe('9 m (30 pés)');
    expect(formatSpeedFt(20)).toBe('6 m (20 pés)');
  });
});

describe('skillProficiencyLabel', () => {
  it('names every proficiency level', () => {
    expect(skillProficiencyLabel('none')).toBe('sem proficiência');
    expect(skillProficiencyLabel('half')).toBe('meia proficiência');
    expect(skillProficiencyLabel('proficient')).toBe('proficiente');
    expect(skillProficiencyLabel('expertise')).toBe('expertise');
  });
});
