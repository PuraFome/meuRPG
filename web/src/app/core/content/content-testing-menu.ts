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
          field('only_proficient', 'yes_no'),
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
      {
        name: 'proficiency_targets',
        values: [
          { key: 'skill:perception', namePt: 'Percepção' },
          { key: 'save.wis', namePt: 'Teste de Sabedoria' },
        ],
      },
      {
        name: 'proficiency_levels',
        values: [
          { key: 'half', namePt: 'Metade' },
          { key: 'full', namePt: 'Completa' },
          { key: 'expertise', namePt: 'Especialização' },
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
        name: 'option_set_names',
        values: [
          {
            key: 'invocation',
            namePt: 'Invocação mística',
            hintPt: 'Uma opção da Invocação mística do bruxo.',
          },
          { key: 'fighting_style', namePt: 'Estilo de luta' },
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
      {
        key: 'feature:pact-boon',
        namePt: 'Dádiva do pacto',
        choose: 1,
        options: [{ key: 'feature:pact-of-the-tome', namePt: 'Dádiva do pacto: Pacto do Tomo' }],
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

/** The menu with the real one's resistance effect (damage types or every type but one, its kind as cards), for its own specs. */
export function resistanceMenu(): EffectMenuVm {
  const res = menuResponse();
  res.types.push(
    create(EffectMenuTypeSchema, {
      type: 'resistance',
      namePt: 'Resistência, imunidade ou vulnerabilidade a dano',
      hintPt: 'Resistência, imunidade ou vulnerabilidade aos tipos de dano escolhidos.',
      fields: [
        { name: 'kind', kind: 'cards', list: 'resistance_kinds', alwaysShown: true },
        {
          name: 'damage_types',
          kind: 'choices',
          list: 'resistance_damage_types',
          alwaysShown: true,
        },
        { name: 'except', kind: 'choice', list: 'damage_types' },
        { name: 'while', kind: 'choice', list: 'resistance_conditions' },
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
      name: 'resistance_damage_types',
      values: [
        { key: 'all', namePt: 'Todos os tipos de dano' },
        {
          key: 'nonmagical_bps',
          namePt: 'Contundente, perfurante e cortante não mágicos',
          hintPt: 'Pancada, perfuração e corte de ataques que não são mágicos.',
        },
        { key: 'damage-type:cold', namePt: 'Frio' },
        { key: 'damage-type:poison', namePt: 'Veneno' },
      ],
    }),
    create(EffectMenuListSchema, {
      name: 'resistance_kinds',
      values: [
        { key: 'resist', namePt: 'Resistência', hintPt: 'Sofre metade do dano.' },
        { key: 'immune', namePt: 'Imunidade', hintPt: 'Não sofre dano.' },
        { key: 'vulnerable', namePt: 'Vulnerabilidade', hintPt: 'Sofre o dobro do dano.' },
      ],
    }),
    create(EffectMenuListSchema, {
      name: 'resistance_conditions',
      values: [{ key: 'rage', namePt: 'Em fúria' }],
    }),
  );
  return new EffectMenuVm(res);
}

/** The menu with the real one's condition_immunity and the roll mode against a condition, for their own specs. */
export function conditionMenu(): EffectMenuVm {
  const res = menuResponse();
  res.types.push(
    create(EffectMenuTypeSchema, {
      type: 'condition_immunity',
      namePt: 'Imunidade a condição',
      hintPt: 'O personagem não fica com as condições escolhidas.',
      fields: [
        { name: 'conditions', kind: 'choices', required: true, list: 'conditions' },
        { name: 'text_pt', kind: 'text' },
      ],
    }),
    create(EffectMenuTypeSchema, {
      type: 'roll_mode',
      namePt: 'Vantagem ou desvantagem',
      hintPt: 'Vantagem ou desvantagem em certas rolagens.',
      fields: [
        { name: 'roll', kind: 'choice', required: true, list: 'roll_modes' },
        { name: 'targets', kind: 'choices', required: true, list: 'roll_targets' },
        { name: 'against_condition', kind: 'choice', list: 'conditions' },
        { name: 'tags', kind: 'tags', list: 'tag_prefixes' },
        { name: 'when', kind: 'condition' },
        { name: 'text_pt', kind: 'text' },
      ],
    }),
  );
  res.lists.push(
    create(EffectMenuListSchema, {
      name: 'conditions',
      values: [
        { key: 'condition:frightened', namePt: 'Amedrontado' },
        { key: 'condition:poisoned', namePt: 'Envenenado' },
      ],
    }),
    create(EffectMenuListSchema, {
      name: 'roll_modes',
      values: [
        { key: 'advantage', namePt: 'Vantagem' },
        { key: 'disadvantage', namePt: 'Desvantagem' },
      ],
    }),
    create(EffectMenuListSchema, {
      name: 'roll_targets',
      values: [
        { key: 'save.wis', namePt: 'Teste de resistência de Sabedoria' },
        { key: 'save.all', namePt: 'Todos os testes de resistência' },
      ],
    }),
  );
  return new EffectMenuVm(res);
}

const damageTypeList = () =>
  create(EffectMenuListSchema, {
    name: 'damage_types',
    values: [
      { key: 'damage-type:cold', namePt: 'Frio' },
      { key: 'damage-type:fire', namePt: 'Fogo' },
    ],
  });

/** The menu with the real one's extra_damage (a fixed part and dice, the type, the weapon kind, how often), for its own specs. */
export function extraDamageMenu(): EffectMenuVm {
  const res = menuResponse();
  res.types.push(
    create(EffectMenuTypeSchema, {
      type: 'extra_damage',
      namePt: 'Dano extra',
      hintPt: 'Dano somado a um ataque com arma que acertou.',
      fields: [
        { name: 'applies', kind: 'choice', required: true, list: 'extra_damage_applies' },
        { name: 'value', kind: 'formula', alwaysShown: true },
        { name: 'dice', kind: 'formula', alwaysShown: true },
        { name: 'dice_sides', kind: 'number', min: 4, max: 12, alwaysShown: true },
        { name: 'damage_type', kind: 'choice', list: 'damage_types', alwaysShown: true },
        { name: 'weapon', kind: 'choice', list: 'extra_damage_weapons', alwaysShown: true },
        { name: 'frequency', kind: 'choice', list: 'extra_damage_frequencies', alwaysShown: true },
        { name: 'no_crit_doubling', kind: 'yes_no' },
        { name: 'resource', kind: 'resource_ref' },
        { name: 'when', kind: 'condition' },
        { name: 'text_pt', kind: 'text' },
      ],
    }),
  );
  res.lists.push(
    create(EffectMenuListSchema, {
      name: 'extra_damage_applies',
      values: [{ key: 'any_target', namePt: 'Qualquer alvo' }],
    }),
    create(EffectMenuListSchema, {
      name: 'extra_damage_weapons',
      values: [
        { key: 'any', namePt: 'Qualquer ataque com arma' },
        { key: 'ranged', namePt: 'Ataque com arma à distância' },
      ],
    }),
    create(EffectMenuListSchema, {
      name: 'extra_damage_frequencies',
      values: [
        { key: 'once_per_turn', namePt: 'Uma vez por turno' },
        { key: 'each_hit', namePt: 'Em todo acerto' },
      ],
    }),
    damageTypeList(),
  );
  return new EffectMenuVm(res);
}

/** The menu with a spell_modifier ("Opção de magia"): the change decides which of the other fields are asked. */
export function spellModifierMenu(): EffectMenuVm {
  const res = menuResponse();
  res.types.push(
    create(EffectMenuTypeSchema, {
      type: 'spell_modifier',
      namePt: 'Opção de magia',
      hintPt: 'Uma opção que a ficha oferece ao conjurar.',
      fields: [
        { name: 'applies_to', kind: 'choice', required: true, list: 'spell_modifier_applies' },
        { name: 'spell_change', kind: 'choice', required: true, list: 'spell_changes' },
        { name: 'from', kind: 'choices', list: 'damage_types', alwaysShown: true },
        { name: 'value', kind: 'formula', alwaysShown: true },
        { name: 'damage_type', kind: 'choice', list: 'damage_types', alwaysShown: true },
        { name: 'count', kind: 'number', min: 1, max: 4, alwaysShown: true },
        { name: 'resource', kind: 'resource_ref' },
        { name: 'when', kind: 'condition' },
        { name: 'text_pt', kind: 'text' },
      ],
    }),
  );
  res.lists.push(
    create(EffectMenuListSchema, {
      name: 'spell_modifier_applies',
      values: [{ key: 'damage_spell', namePt: 'Magia que causa dano' }],
    }),
    create(EffectMenuListSchema, {
      name: 'spell_changes',
      values: [
        { key: 'damage_type_swap', namePt: 'Trocar o tipo de dano' },
        { key: 'bonus_damage', namePt: 'Dano extra' },
        { key: 'extra_dice_per_slot_over', namePt: 'Dados a mais por nível' },
      ],
    }),
    damageTypeList(),
  );
  return new EffectMenuVm(res);
}
