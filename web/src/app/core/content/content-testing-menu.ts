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

/**
 * The menu with the real one's cost, healing and hook fields (backend/internal/rules/tablemenu.go): the action with its
 * cost, cost_max, heal, ability, temp_hp and value_per_point, the on_event hook, the resource with the recharge list and the
 * event list. Invented names only; the hints are the server's.
 */
export function costsMenu(): EffectMenuVm {
  const res = menuResponse();
  const action = res.types.find((t) => t.type === 'grant_action')!;
  action.fields = [
    { name: 'economy', kind: 'choice', required: true, list: 'economies' },
    { name: 'resource', kind: 'resource_ref' },
    { name: 'cost', kind: 'formula' },
    { name: 'cost_max', kind: 'formula' },
    { name: 'heal', kind: 'dice' },
    { name: 'ability', kind: 'choice', list: 'abilities' },
    { name: 'temp_hp', kind: 'formula' },
    { name: 'value_per_point', kind: 'formula' },
    { name: 'when', kind: 'condition' },
    { name: 'text_pt', kind: 'text' },
  ] as never;
  res.types.push(
    create(EffectMenuTypeSchema, {
      type: 'resource',
      namePt: 'Recurso',
      hintPt: 'Uma reserva de usos.',
      fields: [
        { name: 'resource', kind: 'text', required: true },
        { name: 'max', kind: 'formula', required: true },
        { name: 'recharge', kind: 'choice', required: true, list: 'recharges' },
      ],
    }),
    create(EffectMenuTypeSchema, {
      type: 'on_event',
      namePt: 'Gatilho',
      hintPt: 'Algo que acontece com o dono quando um evento ocorre.',
      fields: [
        { name: 'event', kind: 'choice', required: true, list: 'events' },
        { name: 'resource', kind: 'resource_ref' },
        { name: 'cost', kind: 'formula' },
        { name: 'heal', kind: 'dice' },
        { name: 'ability', kind: 'choice', list: 'abilities' },
        { name: 'heal_percent_max', kind: 'number', min: 1, max: 100 },
        { name: 'temp_hp', kind: 'formula' },
        { name: 'restore', kind: 'resource_ref' },
        { name: 'value', kind: 'formula' },
        { name: 'when', kind: 'condition' },
        { name: 'text_pt', kind: 'text' },
      ],
    }),
  );
  res.lists.push(
    create(EffectMenuListSchema, {
      name: 'recharges',
      values: [
        { key: 'short_rest', namePt: 'Descanso curto' },
        { key: 'long_rest', namePt: 'Descanso longo' },
        {
          key: 'initiative',
          namePt: 'Ao rolar iniciativa',
          hintPt: 'Volta um uso quando o personagem rola iniciativa, mas só se não restar nenhum.',
        },
        {
          key: 'restore_one_on_short_rest',
          namePt: 'Um uso por descanso curto',
          hintPt: 'Um descanso curto devolve um uso; um descanso longo devolve todos.',
        },
        {
          key: 'restore_one_on_long_rest',
          namePt: 'Um uso por descanso longo',
          hintPt: 'Um descanso longo devolve um uso; um descanso curto não devolve nenhum.',
        },
      ],
    }),
    create(EffectMenuListSchema, {
      name: 'events',
      values: [
        {
          key: 'short_rest',
          namePt: 'Ao terminar um descanso curto',
          hintPt: 'Quando o dono termina um descanso curto.',
        },
        {
          key: 'long_rest',
          namePt: 'Ao terminar um descanso longo',
          hintPt: 'Quando o dono termina um descanso longo.',
        },
        {
          key: 'death_save_failed',
          namePt: 'Ao falhar num teste contra a morte',
          hintPt: 'Quando o dono falha num teste contra a morte.',
        },
        {
          key: 'drop_to_zero',
          namePt: 'Ao cair a 0 pontos de vida',
          hintPt: 'Quando o dono cai a 0 pontos de vida.',
        },
      ],
    }),
  );
  return new EffectMenuVm(res);
}
