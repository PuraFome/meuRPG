import { create } from '@bufbuild/protobuf';
import { TestBed } from '@angular/core/testing';

import {
  CreatureBonusSchema,
  CreatureDamageModifierSchema,
  CreatureNamedKeySchema,
  CreatureTraitSchema,
  SenseSchema,
} from '../../../gen/meurpg/rules/v1/rules_pb';

import { ogre, raven } from '../../core/creatures/creatures-testing';
import { StatBlock } from './stat-block';

/** What the template reads, taken straight from the component. */
interface Peek {
  tiles(): readonly { label: string; value: string; note?: string; of?: string }[];
  lines(): readonly { term: string; parts: { text: string; en?: boolean }[] }[];
  groups(): readonly { title: string; entries: readonly unknown[] }[];
}

const key = (k: string, namePt: string) => create(CreatureNamedKeySchema, { key: k, namePt });
const mod = (types: ReturnType<typeof key>[], note: string) =>
  create(CreatureDamageModifierSchema, { types, note });
const entry = create(CreatureTraitSchema, { name: 'x', text: 'y', usage: '' });

function build(creature: ReturnType<typeof raven>, inputs: { full?: boolean; hp?: boolean } = {}) {
  const fixture = TestBed.createComponent(StatBlock);
  fixture.componentRef.setInput('creature', creature);
  if (inputs.hp) {
    fixture.componentRef.setInput('hp', { current: 3, max: 9 });
  }
  fixture.componentRef.setInput('full', inputs.full ?? false);
  fixture.detectChanges();
  return fixture.componentInstance as unknown as Peek;
}

describe('StatBlock: the numbers it builds', () => {
  it('draws the creature own hit points when it has them', () => {
    expect(build(ogre(), { hp: true }).tiles()[1]).toEqual({
      label: 'PV',
      value: '3',
      of: 'de 9',
    });
  });

  it('the bestiary page says where each number comes from', () => {
    const tiles = build(ogre(), { full: true }).tiles();
    expect(tiles.map((t) => t.label)).toEqual(['CA', 'PV', 'Deslocamento', 'Nível de desafio']);
    expect(tiles[0].note).toBe('gibão de peles');
    expect(tiles[1].note).toBe('7d10 + 21');
    expect(tiles[2].note).toBe('40 pés');
    expect(tiles[3]).toMatchObject({ value: '2', note: '450 XP' });
  });

  it('falls back to the armor label, and to a bare hit-points tile without dice', () => {
    const c = ogre({ armorClassNote: '', hitPointsRoll: '' });
    expect(build(c, { full: true }).tiles()[0].note).toBe('Armadura');
    expect(build(c).tiles()[1]).toEqual({ label: 'PV', value: '59', of: '' });
  });

  it('shows a walking tile of 0 when the creature has no speed at all', () => {
    const tiles = build(ogre({ speedWalkFt: 0 }), { full: true }).tiles();
    expect(tiles.map((t) => t.label)).toEqual(['CA', 'PV', 'Deslocamento', 'Nível de desafio']);
    expect(tiles[2].note).toBeUndefined();
  });

  it('lists every kind of speed the creature has', () => {
    const c = ogre({ speedFlyFt: 30, speedSwimFt: 20, speedClimbFt: 10, speedBurrowFt: 5 });
    expect(
      build(c)
        .tiles()
        .map((t) => t.label),
    ).toEqual(['CA', 'PV', 'Deslocamento', 'Voo', 'Natação', 'Escalada', 'Escavação']);
  });

  it('writes the saves, skills, damage modifiers and condition immunities', () => {
    const c = ogre({
      savingThrows: [create(CreatureBonusSchema, { key: 'a', namePt: 'Força', bonus: 5 })],
      skills: [create(CreatureBonusSchema, { key: 'b', namePt: 'Furtividade', bonus: -1 })],
      vulnerabilities: [mod([key('f', 'Fogo')], '')],
      resistances: [
        mod([key('g', 'Gelo')], 'from nonmagical weapons'),
        mod([], 'damage from spells'),
        mod([key('h', 'Raio')], ''),
      ],
      immunities: [mod([], 'poison')],
      conditionImmunities: [key('c', 'Medo')],
      senses: [create(SenseSchema, { namePt: 'Visão no escuro', rangeFt: 60 })],
      languages: 'Common',
    });
    const lines = build(c, { full: true }).lines();
    expect(lines.map((l) => l.term)).toEqual([
      'Testes de resistência',
      'Perícias',
      'Vulnerável a',
      'Resistente a',
      'Imune a',
      'Condições',
      'Sentidos',
      'Idiomas',
      'Bônus de proficiência',
    ]);
    expect(lines[0].parts[0].text).toBe('Força +5');
    expect(lines[1].parts[0].text).toBe('Furtividade −1');
    expect(lines[2].parts).toEqual([{ text: 'Fogo' }]);
    expect(lines[3].parts).toEqual([
      { text: 'Gelo ' },
      { text: 'from nonmagical weapons', en: true },
      { text: '; damage from spells', en: true },
      { text: '; Raio' },
    ]);
    expect(lines[4].parts).toEqual([{ text: 'poison', en: true }]);
    expect(lines[5].parts[0].text).toBe('imune a Medo');
    expect(lines[6].parts[0].text).toContain('Percepção passiva');
    expect(lines[7].parts).toEqual([{ text: 'Common', en: true }]);
  });

  it('puts the note of a later modifier after "; " when it names no type', () => {
    const c = ogre({
      resistances: [mod([key('g', 'Gelo')], ''), mod([], 'damage from spells')],
    });
    const line = build(c)
      .lines()
      .find((l) => l.term === 'Resistente a');
    expect(line?.parts).toEqual([{ text: 'Gelo' }, { text: '; damage from spells', en: true }]);
  });

  it('groups traits, actions, reactions and legendary actions that exist', () => {
    const c = ogre({
      traits: [entry],
      reactions: [entry],
      legendaryActions: [entry],
    });
    expect(
      build(c)
        .groups()
        .map((g) => g.title),
    ).toEqual(['Características', 'Ações', 'Reações', 'Ações lendárias']);
    expect(
      build(raven())
        .groups()
        .map((g) => g.title),
    ).toEqual(['Características', 'Ações']);
  });
});
