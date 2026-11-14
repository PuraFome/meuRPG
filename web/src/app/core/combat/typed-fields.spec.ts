import type { DamagePart } from '../../../gen/meurpg/play/v1/combat_rolls_pb';
import { typedBonusLine, typedEntries, typedFields } from './typed-fields';

const part = (over: Partial<DamagePart>): DamagePart =>
  ({
    key: 'weapon',
    labelPt: 'Rapieira',
    diceCount: 1,
    diceSides: 8,
    flat: 0,
    damageTypeKey: 'damage-type:piercing',
    damageTypePt: 'perfurante',
    ...over,
  }) as DamagePart;

const rapier = part({ flat: 3 });
const flare = part({
  key: 'hit:flare',
  labelPt: 'Clarão na Lâmina',
  diceCount: 2,
  diceSides: 6,
  damageTypeKey: 'damage-type:radiant',
  damageTypePt: 'radiante',
  auto: true,
});

describe('typedFields: one field for each damage type', () => {
  it('gives each part its own field, named by part and type, when the types differ', () => {
    expect(typedFields([rapier, flare], [])).toEqual([
      {
        key: 'damage-type:piercing',
        kind: 'type',
        label: 'Rapieira, perfurante: 1d8',
        hint: 'De 1 a 8.',
        min: 1,
        max: 8,
      },
      {
        key: 'damage-type:radiant',
        kind: 'type',
        label: 'Clarão na Lâmina, radiante: 2d6',
        hint: 'De 2 a 12.',
        min: 2,
        max: 12,
      },
    ]);
  });

  it('shares one field "Soma dos dados" among the parts of the same type, with the group range', () => {
    const fire = part({
      key: 'hit:fire',
      labelPt: 'Chama',
      diceCount: 1,
      diceSides: 6,
      damageTypeKey: 'damage-type:piercing',
    });
    expect(typedFields([rapier, fire], [])).toEqual([
      {
        key: 'damage-type:piercing',
        kind: 'type',
        label: 'Soma dos dados: 1d8 + 1d6',
        hint: 'De 2 a 14.',
        min: 2,
        max: 14,
      },
    ]);
  });

  it('names the type on the shared field when another field is there, so two sums are never alike', () => {
    const fire = part({ key: 'hit:fire', labelPt: 'Chama', diceCount: 1, diceSides: 6 });
    const cold = part({ key: 'hit:cold', damageTypeKey: 'damage-type:cold', damageTypePt: 'frio' });
    const cold2 = part({
      key: 'hit:cold2',
      damageTypeKey: 'damage-type:cold',
      damageTypePt: 'frio',
    });
    const labels = typedFields([rapier, fire, cold, cold2], []).map((f) => f.label);
    expect(labels).toEqual([
      'Soma dos dados, perfurante: 1d8 + 1d6',
      'Soma dos dados, frio: 1d8 + 1d8',
    ]);
  });

  it('keeps a part that rolls no dice out, and the fixed bonus with it', () => {
    const rage = part({ key: 'rage', diceCount: 0, diceSides: 0, flat: 2, auto: true });
    expect(typedFields([rapier, rage], []).map((f) => f.key)).toEqual(['damage-type:piercing']);
  });

  it('reads the dice the server sends, doubled on a critical hit, so the ranges double too', () => {
    const crit = (p: DamagePart) => ({ ...p, diceCount: p.diceCount * 2, doubled: true });
    const fields = typedFields([crit(rapier), crit(flare)], []);
    expect(fields.map((f) => [f.label, f.hint])).toEqual([
      ['Rapieira, perfurante: 2d8', 'De 2 a 16.'],
      ['Clarão na Lâmina, radiante: 4d6', 'De 4 a 24.'],
    ]);
    const same = typedFields([crit(rapier), crit({ ...rapier, key: 'x' })], []);
    expect([same[0].min, same[0].max]).toEqual([4, 32]);
  });

  it('takes an extra in only when it is marked, with the slot dice of Golpe Divino', () => {
    const smite = part({
      key: 'divine-smite',
      labelPt: 'Golpe Divino',
      diceCount: 2,
      diceSides: 8,
      damageTypeKey: 'damage-type:radiant',
      damageTypePt: 'radiante',
      choosable: true,
      needsSlot: true,
      slotOptions: [{ level: 3, pact: false, free: 1, max: 2, diceCount: 4 }] as never,
    });
    expect(typedFields([rapier, smite], []).map((f) => f.key)).toEqual(['damage-type:piercing']);
    const picks = [{ key: 'divine-smite', slotLevel: 3, pact: false }];
    const fields = typedFields([rapier, smite], picks);
    expect(fields.map((f) => [f.label, f.hint])).toEqual([
      ['Rapieira, perfurante: 1d8', 'De 1 a 8.'],
      ['Golpe Divino, radiante: 4d8', 'De 4 a 32.'],
    ]);
  });

  it('keeps the extra die of Golpe Divino on its own part key, out of the radiant group', () => {
    const smiteExtra = part({
      key: 'divine-smite-extra',
      labelPt: 'Dado extra do Golpe Divino',
      diceCount: 1,
      diceSides: 8,
      damageTypeKey: 'damage-type:radiant',
      damageTypePt: 'radiante',
    });
    const fields = typedFields([flare, smiteExtra], []);
    expect(fields.map((f) => [f.kind, f.key, f.label])).toEqual([
      ['type', 'damage-type:radiant', 'Clarão na Lâmina, radiante: 2d6'],
      ['part', 'divine-smite-extra', 'Dado extra do Golpe Divino, radiante: 1d8'],
    ]);
  });

  it('falls back to the part key for a part with no damage type key', () => {
    const odd = part({ damageTypeKey: '', damageTypePt: '' });
    expect(typedFields([odd], [])).toEqual([
      { key: 'weapon', kind: 'part', label: 'Rapieira: 1d8', hint: 'De 1 a 8.', min: 1, max: 8 },
    ]);
  });

  it('leaves out the spell effect this weapon does not carry', () => {
    const off = part({
      key: 'frostfist',
      labelPt: 'Geada no Punho',
      damageTypeKey: 'damage-type:cold',
      available: false,
      reasonPt: 'só num golpe corpo a corpo.',
    });
    expect(typedFields([rapier, off], []).map((f) => f.key)).toEqual(['damage-type:piercing']);
  });
});

describe('typedEntries: what goes on the wire', () => {
  it('sends a damage_type entry for a type field and a part_key entry for a part field', () => {
    const fields = typedFields(
      [
        flare,
        part({
          key: 'divine-smite-extra',
          labelPt: 'Dado extra',
          damageTypeKey: 'damage-type:radiant',
        }),
        rapier,
      ],
      [],
    );
    expect(typedEntries(fields, [6, 3, 5])).toEqual([
      { damageType: 'damage-type:radiant', sum: 6 },
      { partKey: 'divine-smite-extra', sum: 3 },
      { damageType: 'damage-type:piercing', sum: 5 },
    ]);
  });

  it('names exactly one of the two on each entry', () => {
    for (const e of typedEntries(typedFields([rapier], []), [4])) {
      expect(Object.keys(e).filter((k) => k !== 'sum')).toHaveLength(1);
    }
  });
});

describe('typedBonusLine', () => {
  it('says the app adds the fixed number, with its sign', () => {
    expect(typedBonusLine(3)).toBe('Sem o + 3: o app soma o bônus.');
    expect(typedBonusLine(-1)).toBe('Sem o − 1: o app soma o bônus.');
  });

  it('says nothing when there is no fixed number', () => {
    expect(typedBonusLine(0)).toBe('');
  });
});
