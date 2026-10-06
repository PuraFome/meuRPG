import { spellRows } from './spell-details-format';
import { SpellDetailsVm } from './spell-details.types';

const base: SpellDetailsVm = {
  key: 'spell:shield',
  namePt: 'Escudo Arcano',
  nameEn: 'Shield',
  level: 1,
  schoolNamePt: 'Abjuração',
  ritual: false,
  concentration: false,
  castingTime: { amount: 1, unit: 'reaction', trigger: '', raw: '1 reaction' },
  range: { kind: 'self', distanceFt: 0, raw: 'Self' },
  components: { verbal: true, somatic: true, material: false, materialText: '' },
  duration: { kind: 'timed', amount: 1, unit: 'round', upTo: false, concentration: false, raw: '1 round' },
  description: ['x'],
  higherLevel: [],
};

describe('spellRows (E10-11)', () => {
  const labels = (d: SpellDetailsVm) => spellRows(d).map((r) => r.label);

  it('has the four facts and no "Alvo" when the server names no target', () => {
    expect(labels(base)).toEqual(['Tempo de conjuração', 'Alcance', 'Componentes', 'Duração']);
  });

  it('puts "Alvo" after the range, from target.label_pt', () => {
    expect(labels({ ...base, targetLabel: 'Só quem conjura' })).toEqual(['Tempo de conjuração', 'Alcance', 'Alvo', 'Componentes', 'Duração']);
    const cone = spellRows({ ...base, targetLabel: 'Cone de 4,5 m' }).find((r) => r.label === 'Alvo')!;
    // The number and its unit (and the "de" before it) are tied with no-break spaces.
    expect(cone.value.text.replace(/\u00a0/g, ' ')).toBe('Cone de 4,5 m');
    expect(cone.value.text).toContain('4,5\u00a0m');
  });

  it('adds "Ataque" and "Dano" only for a table spell (an SRD spell\'s text says them)', () => {
    const attack = { attack: 'ranged' as const, damage: [{ dice: '2d8', typePt: 'necrótico' }] };
    expect(labels({ ...base, ...attack })).not.toContain('Ataque');
    const rows = spellRows({ ...base, ...attack, table: true });
    expect(rows.find((r) => r.label === 'Ataque')?.value.text).toBe('Ataque de magia à distância');
    expect(rows.find((r) => r.label === 'Dano')?.value.text).toBe('2d8 necrótico');
    expect(spellRows({ ...base, table: true, attack: 'melee' }).find((r) => r.label === 'Ataque')?.value.text).toBe('Ataque de magia corpo a corpo');
    expect(labels({ ...base, table: true, attack: 'none' })).not.toContain('Ataque');
  });
});
