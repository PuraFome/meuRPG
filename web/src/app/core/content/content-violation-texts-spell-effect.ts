/** What the server's reason at a field of a table spell's effect says in Portuguese (#579): the paths of
 * backend/internal/rules/overlay_spell_effect.go and combateffects.go, under `table_spell.effect`. The index of a modifier is
 * `[]` here. The first row that matches is the one said. */
export const SPELL_EFFECT_TEXTS: readonly { shape: RegExp; reason?: string; text: string }[] = [
  {
    shape: /^table_spell\.effect\.player_label_pt$/,
    text: 'O que os outros jogadores leem vai até 30 caracteres.',
  },
  { shape: /^table_spell\.effect\.visibility$/, text: 'Escolha quem vê o efeito.' },
  {
    shape: /^table_spell\.effect\.applies$/,
    text: '“Quem falhar no teste” pede um teste de resistência na magia e “Quem o ataque acertar” pede um ataque; uma magia só em quem conjura vale para todos.',
  },
  { shape: /^table_spell\.effect\.conditions$/, text: 'No máximo 8 condições.' },
  {
    shape: /^table_spell\.effect\.conditions\[\]$/,
    text: 'Esta condição não é do SRD. Tire-a da lista.',
  },
  { shape: /^table_spell\.effect\.modifiers$/, text: 'No máximo 8 mudanças no efeito.' },
  {
    shape: /^table_spell\.effect\.modifiers\[\]\.kind$/,
    text: 'Este tipo não vale aqui. Numa aura só valem os que a jogada lê; para impedir de agir, use uma condição (Incapacitado, Paralisado); ou escolha outro tipo.',
  },
  {
    shape: /^table_spell\.effect\.modifiers\[\]\.value$/,
    text: 'Este número está fora do que o tipo aceita: CA de 1 a 20, PV temporários de 1 a 1.000, bônus de -20 a 20 (sem o 0), deslocamento de 0 a 400 % ou até 36 m a mais.',
  },
  { shape: /^table_spell\.effect\.modifiers\[\]\.die$/, text: 'O dado vai de d2 a d100.' },
  { shape: /^table_spell\.effect\.modifiers\[\]\.sign$/, text: 'Escolha se o dado soma ou tira.' },
  {
    shape: /^table_spell\.effect\.modifiers\[\]\.applies_to$/,
    text: 'Marque ao menos uma jogada.',
  },
  {
    shape: /^table_spell\.effect\.modifiers\[\]\.abilities$/,
    text: 'Marque ao menos uma habilidade.',
  },
  { shape: /^table_spell\.effect\.modifiers\[\]\.skill$/, text: 'Escolha a perícia.' },
  {
    shape: /^table_spell\.effect\.modifiers\[\]\.resistance_kind$/,
    text: 'Escolha resistência, imunidade ou vulnerabilidade.',
  },
  {
    shape: /^table_spell\.effect\.modifiers\[\]\.damage_types(\[\])?$/,
    text: 'Escolha ao menos um tipo de dano; “todo dano” e “armas não mágicas” valem sozinhos.',
  },
  {
    shape: /^table_spell\.effect\.modifiers\[\]$/,
    text: 'Confira esta mudança.',
  },
  {
    shape: /^table_spell\.effect\.aura\.radius_ft$/,
    text: 'O alcance da aura vai de 1,5 m a 36 m, em passos de 1,5 m.',
  },
  { shape: /^table_spell\.effect\.aura\.affects$/, text: 'Escolha quem a aura alcança.' },
  {
    shape: /^table_spell\.effect\.aura$/,
    text: 'Uma aura dura enquanto a magia dura, sai de quem conjura (alvo “Só quem conjura”), não põe condição e só dá o que a jogada lê: não vale CA, deslocamento nem PV temporários.',
  },
  {
    shape: /^table_spell\.effect$/,
    text: 'Ponha ao menos uma condição ou uma mudança. Um efeito vazio não é salvo.',
  },
];
