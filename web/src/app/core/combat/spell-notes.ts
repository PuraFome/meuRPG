/**
 * What a spell leaves to the table after the cast, one note for each thing the app does not track: the master applies it
 * (SRD 5.1). Static Portuguese by spell key, worded as a condition: the note never says what happened to a creature, so
 * it tells a player nothing the roll did not.
 */
export const SPELL_NOTES: Readonly<Record<string, readonly string[]>> = {
  'spell:disintegrate': [
    'Se uma criatura chegar a 0 PV por essa magia, ela vira pó: o mestre aplica isso.',
  ],
  'spell:flame-blade': [
    'Enquanto a magia durar, dá para atacar com a lâmina como uma ação, em cada turno seguinte.',
  ],
  'spell:eyebite': [
    'O efeito e a escolha de um novo alvo são uma ação em cada turno seguinte, enquanto a magia durar. O mestre aplica o efeito.',
  ],
  'spell:faithful-hound': [
    'A mordida do cão acontece em cada turno dele, enquanto a magia durar. O mestre aplica a mordida.',
  ],
  'spell:vampiric-touch': [
    'Enquanto a magia durar, dá para repetir o ataque como uma ação, em cada turno seguinte.',
  ],
};

/** The notes of a spell, empty for one that has none. */
export function spellNotes(key: string): readonly string[] {
  return SPELL_NOTES[key] ?? [];
}
