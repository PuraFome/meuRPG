/** A choice in the master's "Pedir um teste": the key the server takes and the Portuguese name. */
export interface AskTask {
  readonly key: string;
  readonly name: string;
}

/** A group of the test select. */
export interface AskTaskGroup {
  readonly label: string;
  readonly tasks: readonly AskTask[];
}

/** The six abilities, by the key the server takes and the official Portuguese name (`names_pt.json`). */
const ABILITIES: readonly AskTask[] = [
  { key: 'str', name: 'Força' },
  { key: 'dex', name: 'Destreza' },
  { key: 'con', name: 'Constituição' },
  { key: 'int', name: 'Inteligência' },
  { key: 'wis', name: 'Sabedoria' },
  { key: 'cha', name: 'Carisma' },
];

/**
 * The groups of the test select in "Pedir um teste": the skills (SRD 5.1, ability checks with a skill), the raw ability checks
 * and the saving throws. The app's term is "teste de resistência", never "salvaguarda".
 */
export function askTaskGroups(skills: readonly AskTask[]): readonly AskTaskGroup[] {
  return [
    { label: 'Perícias', tasks: skills },
    {
      label: 'Habilidades',
      tasks: ABILITIES.map((a) => ({ key: `ability:${a.key}`, name: `Teste de ${a.name}` })),
    },
    {
      label: 'Testes de resistência',
      tasks: ABILITIES.map((a) => ({
        key: `save:${a.key}`,
        name: `Teste de resistência de ${a.name}`,
      })),
    },
  ];
}

/**
 * What the request sends for who rolls and for the group verdict. `picked` is the characters ticked (empty: "Todos"), `party` how
 * many characters there are, `wantGroup` the checkbox. A group check needs two characters or more, so it is false for one.
 */
export function askedWho(
  picked: readonly string[],
  party: number,
  wantGroup: boolean,
): { readonly characterIds: readonly string[]; readonly group: boolean } {
  const asked = picked.length > 0 ? picked.length : party;
  return { characterIds: picked, group: asked >= 2 && wantGroup };
}
