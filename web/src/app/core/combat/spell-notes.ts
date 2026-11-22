/**
 * What a spell leaves to the table after the cast: the master applies what the app does not track.
 * Static Portuguese by spell key, worded as a condition.
 * A note never says what happened to a creature.
 */
export const SPELL_NOTES: Readonly<Record<string, readonly string[]>> = {
  'spell:disintegrate': [
    'Se uma criatura chegar a 0 PV por essa magia, ela vira pó: o mestre aplica isso.',
  ],
  'spell:flame-blade': [
    'Enquanto a magia durar, dá para atacar com a lâmina como uma ação, em cada turno seguinte.',
  ],
  'spell:eyebite': [
    'Quem falhar no teste de resistência dorme, entra em pânico ou passa mal, à escolha de quem conjura. Em cada turno seguinte, enquanto a magia durar, quem conjura pode usar uma ação para mirar outra criatura. O mestre aplica o efeito.',
  ],
  'spell:faithful-hound': [
    'No início de cada turno de quem conjura, enquanto a magia durar, o cão tenta morder uma criatura hostil a até 1,5 m dele (4d8 perfurante, se acertar). O mestre aplica a mordida.',
  ],
  'spell:vampiric-touch': [
    'Enquanto a magia durar, dá para repetir o ataque como uma ação, em cada turno seguinte.',
  ],
};

/** The notes of a spell, empty for one that has none. */
export function spellNotes(key: string): readonly string[] {
  return SPELL_NOTES[key] ?? [];
}
