import { create } from '@bufbuild/protobuf';

import {
  EffectMenuListSchema,
  EffectMenuTypeSchema,
  type GetEffectMenuResponse,
  GetEffectMenuResponseSchema,
} from '../../../gen/meurpg/rules/v1/table_content_pb';
import { EffectMenuVm } from './effect-draft';

/** The menu: the real one's types and field order (backend/internal/rules/tablemenu.go), with a few of its lists. */
export function menuResponse(): GetEffectMenuResponse {
  const field = (name: string, kind: string, required = false, list = '', min = 0, max = 0) => ({
    name,
    kind,
    required,
    list,
    min,
    max,
  });
  return create(GetEffectMenuResponseSchema, {
    types: [
      {
        type: 'modifier',
        namePt: 'Modificador',
        hintPt: 'Soma, define ou limita um número da ficha.',
        fields: [
          field('target', 'choice', true, 'modifier_targets'),
          field('mode', 'choice', true, 'modifier_modes'),
          field('value', 'formula', true),
          field('when', 'condition'),
          field('tags', 'tags', false, 'tag_prefixes'),
          field('text_pt', 'text'),
        ],
      },
      {
        type: 'proficiency',
        namePt: 'Proficiência',
        hintPt: 'Dá proficiência.',
        fields: [
          field('proficiency', 'choice', true, 'proficiency_targets'),
          field('level', 'choice', false, 'proficiency_levels'),
          field('when', 'condition'),
          field('text_pt', 'text'),
        ],
      },
      {
        type: 'sense',
        namePt: 'Sentido',
        hintPt: 'Visão no escuro.',
        fields: [
          field('sense', 'choice', true, 'senses'),
          field('range_ft', 'number', true, '', 1),
          field('when', 'condition'),
          field('text_pt', 'text'),
        ],
      },
      {
        type: 'grant_action',
        namePt: 'Ação',
        hintPt: 'Uma ação que o personagem passa a ter.',
        fields: [
          field('economy', 'choice', true, 'economies'),
          field('resource', 'resource_ref'),
          field('when', 'condition'),
          field('tags', 'tags', false, 'tag_prefixes'),
          field('text_pt', 'text'),
        ],
      },
      {
        type: 'superiority_die',
        namePt: 'Dado de manobra',
        hintPt: 'Um dado que uma opção soma a uma rolagem.',
        fields: [
          field('applies', 'choice', true, 'maneuver_applies'),
          field('resource', 'resource_ref', true),
          field('value', 'formula', true),
          field('ability', 'choice', false, 'abilities'),
          field('economy', 'choice', false, 'economies'),
          field('when', 'condition'),
          field('text_pt', 'text'),
        ],
      },
      {
        type: 'choice',
        namePt: 'Escolha',
        hintPt: 'Algo que o jogador escolhe.',
        fields: [
          field('choice', 'choice', true, 'choice_kinds'),
          field('count', 'number', true, '', 1),
          field('from', 'choices'),
          field('when', 'condition'),
          field('text_pt', 'text'),
        ],
      },
      {
        type: 'ability_increase',
        namePt: 'Aumento de habilidade',
        hintPt: 'O jogador escolhe habilidades de uma lista.',
        featOnly: true,
        fields: [
          field('count', 'number', true, '', 1),
          field('from', 'choices', true, 'abilities'),
          field('value', 'choice', true, 'ability_increase_amounts'),
          field('text_pt', 'text'),
        ],
      },
      {
        type: 'note',
        namePt: 'Nota e magia concedida',
        hintPt: 'Um lembrete.',
        fields: [
          field('text_pt', 'text'),
          field('value', 'formula'),
          field('spells', 'choices'),
          field('tags', 'tags', false, 'tag_prefixes'),
          field('when', 'condition'),
        ],
      },
    ],
    lists: [
      {
        name: 'modifier_targets',
        values: [
          { key: 'speed.walk', namePt: 'Deslocamento' },
          { key: 'ac', namePt: 'Classe de Armadura' },
        ],
      },
      { name: 'modifier_modes', values: [{ key: 'add', namePt: 'Somar' }] },
      { name: 'proficiency_targets', values: [{ key: 'skill:perception', namePt: 'Percepção' }] },
      {
        name: 'proficiency_levels',
        values: [
          { key: 'half', namePt: 'Metade' },
          { key: 'full', namePt: 'Completa' },
        ],
      },
      { name: 'senses', values: [{ key: 'darkvision', namePt: 'Visão no escuro' }] },
      {
        name: 'choice_kinds',
        values: [
          { key: 'skill', namePt: 'Perícias' },
          { key: 'feature', namePt: 'Uma opção de uma lista do SRD' },
          { key: 'feat', namePt: 'Talentos' },
        ],
      },
      {
        name: 'abilities',
        values: [
          { key: 'strength', namePt: 'Força' },
          { key: 'dexterity', namePt: 'Destreza' },
          { key: 'constitution', namePt: 'Constituição' },
        ],
      },
      {
        name: 'ability_increase_amounts',
        values: [
          { key: '1', namePt: '+1' },
          { key: '2', namePt: '+2' },
        ],
      },
      {
        name: 'skills',
        values: [
          { key: 'skill:arcana', namePt: 'Arcanismo' },
          { key: 'skill:perception', namePt: 'Percepção' },
        ],
      },
      {
        name: 'languages',
        values: [
          { key: 'language:common', namePt: 'Comum' },
          { key: 'language:primordial', namePt: 'Primordial' },
        ],
      },
      {
        name: 'tools',
        values: [{ key: 'proficiency:cartographers-tools', namePt: 'Ferramentas de cartógrafo' }],
      },
      {
        name: 'tag_prefixes',
        values: [
          { key: 'against:', namePt: 'Contra…' },
          { key: 'about:', namePt: 'Sobre…' },
        ],
      },
    ],
    helpers: [
      { call: 'mod("<habilidade>")', returns: 'number', hintPt: 'O modificador.' },
      { call: 'prof()', returns: 'number', hintPt: 'O bônus de proficiência.' },
    ],
    optionSets: [
      {
        key: 'feature:fighting-style',
        namePt: 'Estilo de luta',
        choose: 1,
        options: [{ key: 'option:archery', namePt: 'Arquearia' }],
      },
    ],
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

/** The menu with the real one's ranged_attack (a table feat's switches and a spell range multiplier), for its own specs. */
export function rangedAttackMenu(): EffectMenuVm {
  const res = menuResponse();
  res.types.push(
    create(EffectMenuTypeSchema, {
      type: 'ranged_attack',
      namePt: 'Ataque à distância',
      hintPt: 'Muda os ataques de longe do personagem.',
      fields: [
        { name: 'applies', kind: 'choice', required: true, list: 'ranged_attack_applies' },
        { name: 'ignore_cover', kind: 'yes_no', alwaysShown: true },
        { name: 'range_multiplier', kind: 'number', min: 2, max: 3, alwaysShown: true },
        { name: 'no_long_range_disadvantage', kind: 'yes_no', alwaysShown: true },
        { name: 'when', kind: 'condition' },
        { name: 'text_pt', kind: 'text' },
      ],
    }),
  );
  res.lists.push(
    create(EffectMenuListSchema, {
      name: 'ranged_attack_applies',
      values: [
        { key: 'spell', namePt: 'Magias com jogada de ataque' },
        { key: 'weapon', namePt: 'Ataques com arma à distância' },
      ],
    }),
  );
  return new EffectMenuVm(res);
}

/** The menu with the real one's resistance effect (every damage type but one, while raging), for its own specs. */
export function resistanceMenu(): EffectMenuVm {
  const res = menuResponse();
  res.types.push(
    create(EffectMenuTypeSchema, {
      type: 'resistance',
      namePt: 'Resistência a dano',
      hintPt: 'Resistência a todos os tipos de dano menos um.',
      fields: [
        { name: 'except', kind: 'choice', required: true, list: 'damage_types' },
        { name: 'while', kind: 'choice', required: true, list: 'resistance_conditions' },
        { name: 'text_pt', kind: 'text' },
      ],
    }),
  );
  res.lists.push(
    create(EffectMenuListSchema, {
      name: 'damage_types',
      values: [{ key: 'damage-type:psychic', namePt: 'Psíquico' }],
    }),
    create(EffectMenuListSchema, {
      name: 'resistance_conditions',
      values: [{ key: 'rage', namePt: 'Em fúria' }],
    }),
  );
  return new EffectMenuVm(res);
}

/** The menu with the real one's granted_spell and breathes_water effects, for their own specs. */
export function grantedSpellMenu(): EffectMenuVm {
  const res = menuResponse();
  res.types.push(
    create(EffectMenuTypeSchema, {
      type: 'granted_spell',
      namePt: 'Magia concedida',
      hintPt: 'Uma magia que a entrada dá ao personagem fora das listas das classes dele.',
      fields: [
        { name: 'spell', kind: 'choice', required: true },
        { name: 'ability', kind: 'choice', required: true, list: 'granted_spell_abilities' },
        { name: 'from_level', kind: 'formula' },
        { name: 'uses', kind: 'resource_ref' },
        { name: 'also_with_slot', kind: 'yes_no', alwaysShown: true },
        { name: 'ritual_only', kind: 'yes_no', alwaysShown: true },
        { name: 'counts_as_known', kind: 'yes_no', alwaysShown: true },
        { name: 'when', kind: 'condition' },
        { name: 'text_pt', kind: 'text' },
      ],
    }),
    create(EffectMenuTypeSchema, {
      type: 'breathes_water',
      namePt: 'Respira na água',
      hintPt: "O personagem respira debaixo d'água.",
      fields: [
        { name: 'when', kind: 'condition' },
        { name: 'text_pt', kind: 'text' },
      ],
    }),
  );
  res.lists.push(
    create(EffectMenuListSchema, {
      name: 'granted_spell_abilities',
      values: [
        { key: 'int', namePt: 'Inteligência' },
        { key: 'wis', namePt: 'Sabedoria' },
        { key: 'cha', namePt: 'Carisma' },
        { key: 'choice', namePt: 'O jogador escolhe' },
      ],
    }),
  );
  return new EffectMenuVm(res);
}
