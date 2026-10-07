import { FormControl, FormGroup, Validators } from '@angular/forms';

import {
  FULL_SHEET_FIELDS,
  abilityAbbreviation,
  countLabel,
  describeBonusesInUse,
  describeInvalidFields,
  invalidFields,
} from './editor-labels';

describe('editor labels', () => {
  it('abbreviates each ability the way the official sheet does', () => {
    expect(abilityAbbreviation('str')).toBe('For');
    expect(abilityAbbreviation('dex')).toBe('Des');
    expect(abilityAbbreviation('con')).toBe('Con');
    expect(abilityAbbreviation('int')).toBe('Int');
    expect(abilityAbbreviation('wis')).toBe('Sab');
    expect(abilityAbbreviation('cha')).toBe('Car');
  });

  it('names only the manual bonuses in use, in sheet order, with their sign', () => {
    expect(describeBonusesInUse({ str: 0, dex: 0, con: 0, int: 0, wis: 0, cha: 0 })).toBe('');
    expect(describeBonusesInUse({ str: 0, dex: 0, con: 1, int: 2, wis: 0, cha: -1 })).toBe(
      'Constituição +1, Inteligência +2, Carisma -1',
    );
    // A field being typed in (not a number yet) is not "in use".
    expect(describeBonusesInUse({ str: null as unknown as number, dex: 0 })).toBe('');
  });

  it('counts with the right plural', () => {
    const count = (n: number) =>
      countLabel(n, 'perícia marcada', 'perícias marcadas', 'Nenhuma perícia marcada');
    expect(count(0)).toBe('Nenhuma perícia marcada');
    expect(count(1)).toBe('1 perícia marcada');
    expect(count(3)).toBe('3 perícias marcadas');
  });

  it('lists the invalid fields of a full sheet grouped by step, in step order', () => {
    const form = new FormGroup({
      name: new FormControl('', Validators.required),
      race: new FormControl('', Validators.required),
      level: new FormControl(1),
      abilities: new FormGroup({
        str: new FormControl(31, Validators.max(30)),
        dex: new FormControl(10, Validators.max(30)),
      }),
      extraAbilityBonuses: new FormGroup({
        con: new FormControl(11, Validators.max(10)),
      }),
    });

    const invalid = invalidFields(form, FULL_SHEET_FIELDS);

    expect(invalid.map((f) => f.path)).toEqual([
      'name',
      'race',
      'abilities.str',
      'extraAbilityBonuses.con',
    ]);
    expect(describeInvalidFields(invalid)).toBe(
      'Básico: Nome do personagem, Raça. Habilidades: Força, bônus manual de Constituição.',
    );
  });

  it('describes nothing when every field is valid', () => {
    expect(describeInvalidFields([])).toBe('');
  });
});
