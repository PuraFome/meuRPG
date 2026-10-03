import { TestBed } from '@angular/core/testing';
import { FormBuilder } from '@angular/forms';

import { BasicCharacterFormValue } from '../character-editor.types';
import {
  basicFormToValue,
  createAttackGroup,
  createBasicForm,
  invalidBasicFields,
  parseDice,
  parseSigned,
  patchBasicForm,
} from './basic-form';

describe('parseSigned', () => {
  it.each([
    ['2', 2],
    ['+2', 2],
    ['-1', -1],
    ['−1', -1], // a real minus sign
    [' +4 ', 4],
    ['0', 0],
  ])('reads %j as %d', (text, n) => {
    expect(parseSigned(text)).toBe(n);
  });

  it.each(['', '+', '1,5', '2.5', 'dois', '1d6', '--1'])('refuses %j', (text) => {
    expect(parseSigned(text)).toBeNull();
  });
});

describe('parseDice', () => {
  it('reads NdS with the dice the rules allow', () => {
    expect(parseDice('1d6')).toEqual({ count: 1, sides: 6 });
    expect(parseDice(' 2D8 ')).toEqual({ count: 2, sides: 8 });
    expect(parseDice('20d12')).toEqual({ count: 20, sides: 12 });
  });

  it.each(['', 'd6', '0d6', '21d6', '1d7', '1d20', '1d6+2', '6'])('refuses %j', (text) => {
    expect(parseDice(text)).toBeNull();
  });
});

describe('the NPC short form', () => {
  const fb = () => TestBed.inject(FormBuilder);
  const goblin: BasicCharacterFormValue = {
    name: 'Goblin',
    hitPointsMax: 7,
    armorClass: 15,
    speedFt: 30,
    initiativeBonus: 2,
    attacks: [
      {
        name: 'Cimitarra',
        attackBonus: 4,
        damageDiceCount: 1,
        damageDiceSides: 6,
        damageBonus: 2,
        damageType: 'slashing',
        rangeFt: 0,
      },
      {
        name: 'Arco curto',
        attackBonus: 4,
        damageDiceCount: 2,
        damageDiceSides: 4,
        damageBonus: -1,
        damageType: 'piercing',
        rangeFt: 80,
      },
    ],
    legacyDamage: '',
    legacyAttackBonus: 0,
    description: 'Pequeno e esperto.',
    challengeRating: '1/4',
    xpValue: 50,
    portraitImageId: '',
  };

  it('loads a sheet and gives the same value back (metres on screen, feet stored)', () => {
    const form = createBasicForm(fb());
    patchBasicForm(fb(), form, goblin);

    expect(form.controls.speedWalkM.value).toBe(9);
    expect(form.controls.initiativeBonus.value).toBe('+2');
    expect(form.controls.attacks.length).toBe(2);
    expect(form.valid).toBe(true);
    expect(basicFormToValue(form)).toEqual(goblin);
  });

  it('keeps the reach of an attack although the form has no field for it', () => {
    const form = createBasicForm(fb());
    patchBasicForm(fb(), form, goblin);
    expect(basicFormToValue(form).attacks[1].rangeFt).toBe(80);
  });

  it('keeps the old damage text for the note, and loading twice does not stack cards', () => {
    const old = { ...goblin, attacks: [], legacyDamage: 'mordida venenosa', legacyAttackBonus: 3 };
    const form = createBasicForm(fb());
    patchBasicForm(fb(), form, goblin);
    patchBasicForm(fb(), form, old);

    expect(form.controls.attacks.length).toBe(0);
    expect(basicFormToValue(form).legacyDamage).toBe('mordida venenosa');
    expect(basicFormToValue(form).legacyAttackBonus).toBe(3);
  });

  it('accepts speeds in steps of 1,5 m and refuses the rest', () => {
    const speed = createBasicForm(fb()).controls.speedWalkM;
    for (const ok of [0, 1.5, 4.5, 9, 90]) {
      speed.setValue(ok);
      expect(speed.valid).toBe(true);
    }
    for (const bad of [1, 2.2, -1.5, 91.5]) {
      speed.setValue(bad);
      expect(speed.valid).toBe(false);
    }
  });

  it('validates the initiative and the attack numbers within the API limits', () => {
    const form = createBasicForm(fb());
    form.controls.initiativeBonus.setValue('21');
    expect(form.controls.initiativeBonus.valid).toBe(false);
    form.controls.initiativeBonus.setValue('−10');
    expect(form.controls.initiativeBonus.valid).toBe(true);

    const attack = createAttackGroup(fb(), { name: 'Faca', damageType: 'piercing' });
    expect(attack.valid).toBe(true);
    attack.controls.attackBonus.setValue('+21');
    attack.controls.damageBonus.setValue('41');
    attack.controls.dice.setValue('1d20');
    expect(attack.controls.attackBonus.valid).toBe(false);
    expect(attack.controls.damageBonus.valid).toBe(false);
    expect(attack.controls.dice.valid).toBe(false);
  });

  it('allows at most three attacks', () => {
    const form = createBasicForm(fb());
    for (let i = 0; i < 4; i++) {
      form.controls.attacks.push(createAttackGroup(fb(), { name: 'A', damageType: 'fire' }));
    }
    expect(form.controls.attacks.valid).toBe(false);
    form.controls.attacks.removeAt(0);
    expect(form.controls.attacks.valid).toBe(true);
  });

  it('names the invalid fields, with the attack they belong to', () => {
    const form = createBasicForm(fb());
    form.controls.attacks.push(createAttackGroup(fb(), { name: 'A', damageType: 'fire' }));
    form.controls.attacks.push(createAttackGroup(fb())); // no name, no type
    form.controls.name.setValue('');

    expect(invalidBasicFields(form).map((f) => f.label)).toEqual([
      'Nome do personagem',
      'Ataque 2: Nome do ataque',
      'Ataque 2: Tipo de dano',
    ]);
  });
});
