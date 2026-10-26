import { ownHitOf } from './own-hit';

describe('ownHitOf', () => {
  it("says the attack's damage, with who did it", () => {
    expect(
      ownHitOf({ id: 'a', actorLabel: 'Espada voadora 2', amount: 9, damageTypePt: 'cortante' }),
    ).toEqual({ id: 'a', text: 'A Espada voadora 2 acertou você: 9 de dano cortante.' });
  });

  it('writes the other types with "de"', () => {
    expect(
      ownHitOf({ id: 'a', actorLabel: 'Armadura animada', amount: 9, damageTypePt: 'fogo' }).text,
    ).toBe('A Armadura animada acertou você: 9 de dano de fogo.');
  });

  it('says "Alguém" when the attacker is hidden, and "dano" when the hit has no type', () => {
    expect(ownHitOf({ id: 'a', actorLabel: '', amount: 5, damageTypePt: '' }).text).toBe(
      'Alguém acertou você: 5 de dano.',
    );
  });
});
