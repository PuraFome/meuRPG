import { create } from '@bufbuild/protobuf';

import {
  type GetEffectMenuResponse,
  GetEffectMenuResponseSchema,
  type TableEntry,
  TableContentKind,
  TableEntrySchema,
  TableClassSchema,
  TableRaceSchema,
  TableSpellSchema,
  TableSubclassSchema,
  TableBackgroundSchema,
  TableFeatureSchema,
  TableEffectSchema,
} from '../../../gen/meurpg/rules/v1/table_content_pb';
import { Ability, ContentSchema } from '../../../gen/meurpg/rules/v1/rules_pb';
import { type CatalogAbility, type CatalogVm, catalogVm } from './catalog';
import { EffectMenuVm } from './effect-draft';

/** Test builders: an entry as the server sends it, and a small effect menu with the shape of the real one. */

type Body = TableEntry['body'];

export function entry(kind: TableContentKind, namePt: string, over: { archived?: boolean; charactersUsing?: number; key?: string; body?: Body } = {}): TableEntry {
  const slug = namePt.toLowerCase().replace(/[^a-z]+/g, '-');
  const prefix = { [TableContentKind.CLASS]: 'class', [TableContentKind.SUBCLASS]: 'subclass', [TableContentKind.RACE]: 'race', [TableContentKind.SUBRACE]: 'subrace', [TableContentKind.BACKGROUND]: 'background', [TableContentKind.SPELL]: 'spell' }[kind as 1];
  const body: Body =
    over.body ??
    (kind === TableContentKind.CLASS
      ? { case: 'tableClass', value: create(TableClassSchema, { namePt, hitDie: 10, subclassLevel: 3 }) }
      : kind === TableContentKind.SUBCLASS
        ? { case: 'tableSubclass', value: create(TableSubclassSchema, { namePt, classKey: 'class:wizard', level: 2 }) }
        : kind === TableContentKind.RACE
          ? { case: 'tableRace', value: create(TableRaceSchema, { namePt, size: 'Medium', speedFt: 30 }) }
          : kind === TableContentKind.BACKGROUND
            ? { case: 'tableBackground', value: create(TableBackgroundSchema, { namePt, skills: ['skill:arcana', 'skill:history'] }) }
            : { case: 'tableSpell', value: create(TableSpellSchema, { namePt, level: 1 }) });
  return create(TableEntrySchema, {
    key: over.key ?? `${prefix}:${slug}@mesa`,
    kind,
    namePt,
    archived: over.archived ?? false,
    charactersUsing: over.charactersUsing ?? 0,
    revision: 3,
    body,
  });
}

/** The seven entries of the artboard (E10-01): two classes, one archived, two subclasses, a race, a background, a spell. */
export function mirathel(): TableEntry[] {
  return [
    entry(TableContentKind.CLASS, 'Guardião do Vale', { charactersUsing: 2 }),
    entry(TableContentKind.CLASS, 'Bardo das Cinzas', { archived: true, charactersUsing: 1 }),
    entry(TableContentKind.SUBCLASS, 'Tradição da Tinta'),
    entry(TableContentKind.SUBCLASS, 'Domínio do Caminho'),
    entry(TableContentKind.RACE, 'Corujeiro', { charactersUsing: 2 }),
    entry(TableContentKind.BACKGROUND, 'Cartógrafo do Vale'),
    entry(TableContentKind.SPELL, 'Lâmina de Nanquim', { charactersUsing: 1 }),
  ];
}

export function feature(name: string, effects: Parameters<typeof create<typeof TableEffectSchema>>[1][] = []) {
  return create(TableFeatureSchema, { key: `feature:${name}`, namePt: name, descPt: ['Texto.'], effects: effects.map((e) => create(TableEffectSchema, e)) });
}

/** The menu: the real one's types and field order (backend/internal/rules/tablemenu.go), with a few of its lists. */
export function menuResponse(): GetEffectMenuResponse {
  const field = (name: string, kind: string, required = false, list = '', min = 0, max = 0) => ({ name, kind, required, list, min, max });
  return create(GetEffectMenuResponseSchema, {
    types: [
      { type: 'modifier', namePt: 'Modificador', hintPt: 'Soma, define ou limita um número da ficha.', fields: [field('target', 'choice', true, 'modifier_targets'), field('mode', 'choice', true, 'modifier_modes'), field('value', 'formula', true), field('when', 'condition'), field('tags', 'tags', false, 'tag_prefixes'), field('text_pt', 'text')] },
      { type: 'proficiency', namePt: 'Proficiência', hintPt: 'Dá proficiência.', fields: [field('proficiency', 'choice', true, 'proficiency_targets'), field('level', 'choice', false, 'proficiency_levels'), field('when', 'condition'), field('text_pt', 'text')] },
      { type: 'sense', namePt: 'Sentido', hintPt: 'Visão no escuro.', fields: [field('sense', 'choice', true, 'senses'), field('range_ft', 'number', true, '', 1), field('when', 'condition'), field('text_pt', 'text')] },
      { type: 'choice', namePt: 'Escolha', hintPt: 'Algo que o jogador escolhe.', fields: [field('choice', 'choice', true, 'choice_kinds'), field('count', 'number', true, '', 1), field('from', 'choices'), field('when', 'condition'), field('text_pt', 'text')] },
      { type: 'note', namePt: 'Nota e magia concedida', hintPt: 'Um lembrete.', fields: [field('text_pt', 'text'), field('value', 'formula'), field('spells', 'choices'), field('tags', 'tags', false, 'tag_prefixes'), field('when', 'condition')] },
    ],
    lists: [
      { name: 'modifier_targets', values: [{ key: 'speed.walk', namePt: 'Deslocamento' }, { key: 'ac', namePt: 'Classe de Armadura' }] },
      { name: 'modifier_modes', values: [{ key: 'add', namePt: 'Somar' }] },
      { name: 'proficiency_targets', values: [{ key: 'skill:perception', namePt: 'Percepção' }] },
      { name: 'proficiency_levels', values: [{ key: 'half', namePt: 'Metade' }, { key: 'full', namePt: 'Completa' }] },
      { name: 'senses', values: [{ key: 'darkvision', namePt: 'Visão no escuro' }] },
      { name: 'choice_kinds', values: [{ key: 'skill', namePt: 'Perícias' }, { key: 'feature', namePt: 'Uma opção de uma lista do SRD' }] },
      { name: 'skills', values: [{ key: 'skill:arcana', namePt: 'Arcanismo' }, { key: 'skill:perception', namePt: 'Percepção' }] },
      { name: 'languages', values: [{ key: 'language:common', namePt: 'Comum' }, { key: 'language:primordial', namePt: 'Primordial' }] },
      { name: 'tools', values: [{ key: 'proficiency:cartographers-tools', namePt: 'Ferramentas de cartógrafo' }] },
      { name: 'tag_prefixes', values: [{ key: 'against:', namePt: 'Contra…' }, { key: 'about:', namePt: 'Sobre…' }] },
    ],
    helpers: [{ call: 'mod("<atributo>")', returns: 'number', hintPt: 'O modificador.' }, { call: 'prof()', returns: 'number', hintPt: 'O bônus de proficiência.' }],
    optionSets: [{ key: 'feature:fighting-style', namePt: 'Estilo de luta', choose: 1, options: [{ key: 'option:archery', namePt: 'Arquearia' }] }],
    classIndexes: ['wizard', 'cleric'],
    maxFeaturesPerClass: 60,
    maxEffectsPerFeature: 4,
    maxTagsPerEffect: 4,
    extraAttackMin: 2,
    extraAttackMax: 4,
  });
}

export function menu(): EffectMenuVm {
  return new EffectMenuVm(menuResponse());
}

/** The six abilities as the catalog names them, for the specs. */
export const abilities: readonly CatalogAbility[] = (['strength', 'dexterity', 'constitution', 'intelligence', 'wisdom', 'charisma'] as const).map((field, i) => ({
  field,
  ability: (i + 1) as Ability,
  name: ['Força', 'Destreza', 'Constituição', 'Inteligência', 'Sabedoria', 'Carisma'][i],
}));

/** A catalog like the server's, with what a table entry points at named (languages, proficiencies, damage types, skills). */
export function catalog(extra: Parameters<typeof catalogVm>[1] = []): CatalogVm {
  return catalogVm(
    create(ContentSchema, {
      abilities: abilities.map((a) => ({ ability: a.ability, namePt: a.name })),
      races: [{ key: 'race:human', namePt: 'Humano' }],
      classes: [{ key: 'class:wizard', namePt: 'Mago', subclassLevel: 2, spellcasting: { ability: 4 } }],
      skills: [{ key: 'skill:arcana', namePt: 'Arcanismo' }, { key: 'skill:perception', namePt: 'Percepção' }, { key: 'skill:investigation', namePt: 'Investigação' }],
      spells: [{ key: 'spell:light', namePt: 'Luz', schoolKey: 'school:evocation', schoolNamePt: 'Evocação' }],
      languages: [{ key: 'language:common', namePt: 'Comum' }, { key: 'language:primordial', namePt: 'Primordial' }],
      proficiencies: [
        { key: 'proficiency:cartographers-tools', namePt: 'Ferramentas de cartógrafo' },
        { key: 'proficiency:light-armor', namePt: 'Armadura leve' },
        { key: 'proficiency:medium-armor', namePt: 'Armadura média' },
        { key: 'proficiency:shields', namePt: 'Escudos' },
        { key: 'proficiency:simple-weapons', namePt: 'Armas simples' },
        { key: 'proficiency:martial-weapons', namePt: 'Armas marciais' },
      ],
      damageTypes: [{ key: 'damage-type:necrotic', namePt: 'necrótico' }, { key: 'damage-type:fire', namePt: 'fogo' }],
    }),
    extra,
  );
}
