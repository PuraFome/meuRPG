import { type MessageInitShape, create } from '@bufbuild/protobuf';

import {
  type OptionSwitchEntry,
  OptionSwitchEntrySchema,
  TableContentKind,
} from '../../../gen/meurpg/rules/v1/table_content_pb';
import { CONTENT_NAV, navBySlug } from './content-kinds';
import {
  OptionSwitchesState,
  bulkChanges,
  changeSentence,
  counterText,
  groupRows,
  hiddenByParent,
  hiddenCounterText,
  hiddenNote,
  mainCount,
  menuCount,
  nestRows,
  onOffWords,
  searchRows,
  subraceCount,
  subraceCounterText,
  usingText,
  withHidden,
} from './option-switches';

function opt(
  kind: TableContentKind,
  namePt: string,
  over: MessageInitShape<typeof OptionSwitchEntrySchema> = {},
): OptionSwitchEntry {
  return create(OptionSwitchEntrySchema, {
    key: `${TableContentKind[kind].toLowerCase()}:${namePt.toLowerCase()}`,
    kind,
    namePt,
    ...over,
  });
}

const races = (): OptionSwitchEntry[] => [
  opt(TableContentKind.RACE, 'Anão'),
  opt(TableContentKind.RACE, 'Corujeiro', {
    table: true,
    key: 'race:corujeiro@mesa',
    charactersUsing: 2,
  }),
  opt(TableContentKind.RACE, 'Tiefling', { off: true, hidden: true }),
  opt(TableContentKind.SUBRACE, 'Anão da Colina', {
    parentKey: 'race:anão',
    key: 'subrace:colina',
  }),
];

describe('option switches: counts and words (E10-01 state 3)', () => {
  const racas = navBySlug('races')!;

  it('counts the races apart from the sub-races: "Raças: 2 de 3 ligadas"', () => {
    expect(counterText(racas, mainCount(races(), racas))).toBe('Raças: 2 de 3 ligadas');
    expect(subraceCounterText(subraceCount(races()))).toBe('Sub-raças: 1 de 1 ligada');
    expect(menuCount(races(), racas)).toBe('2 de 3');
  });

  it('agrees the noun: backgrounds are "ligados"', () => {
    const bg = navBySlug('backgrounds')!;
    const list = [
      opt(TableContentKind.BACKGROUND, 'Acólito'),
      opt(TableContentKind.BACKGROUND, 'Nobre', { off: true }),
    ];
    expect(counterText(bg, mainCount(list, bg))).toBe('Antecedentes: 1 de 2 ligados');
  });

  it('keeps each kind in its own group', () => {
    const mixed = [
      ...races(),
      opt(TableContentKind.CLASS, 'Mago'),
      opt(TableContentKind.SPELL, 'Luz'),
    ];
    expect(groupRows(mixed, navBySlug('races')!).map((o) => o.namePt)).toEqual([
      'Anão',
      'Corujeiro',
      'Tiefling',
      'Anão da Colina',
    ]);
    expect(CONTENT_NAV.map((n) => groupRows(mixed, n).length)).toEqual([1, 0, 4, 0, 1, 0]);
  });

  it('says how many sheets use an option only above zero, and "Nenhuma ficha usa" only on an off row, with no-break spaces', () => {
    expect(usingText(0)).toBe('');
    expect(usingText(0, true)).toBe('Nenhuma\u00a0ficha\u00a0usa');
    expect(usingText(1)).toBe('1\u00a0ficha usa');
    expect(usingText(3, true)).toBe('3\u00a0fichas usam');
  });

  it("counts the children the players do not get for their parent's sake, apart from the group's own count", () => {
    const list = withHidden([
      opt(TableContentKind.CLASS, 'Mago', { key: 'class:wizard', off: true }),
      opt(TableContentKind.SUBCLASS, 'Evocação', {
        key: 'subclass:evocation',
        parentKey: 'class:wizard',
      }),
      opt(TableContentKind.SUBCLASS, 'Ilusão', {
        key: 'subclass:illusion',
        parentKey: 'class:wizard',
      }),
      opt(TableContentKind.SUBCLASS, 'Necromancia', {
        key: 'subclass:necro',
        parentKey: 'class:wizard',
        off: true,
      }),
    ]);
    const subs = list.filter((o) => o.kind === TableContentKind.SUBCLASS);
    expect(hiddenByParent(subs)).toBe(2);
    const nav = navBySlug('subclasses')!;
    expect(
      counterText(nav, mainCount(list, nav)) + hiddenCounterText(nav, hiddenByParent(subs)),
    ).toBe('Subclasses: 2 de 3 ligadas · 2 escondidas pela classe');
    expect(hiddenCounterText(nav, 0)).toBe('');
    expect(hiddenCounterText(navBySlug('races')!, 1)).toBe(' · 1 escondida pela raça');
  });

  it('puts each sub-race under its race, and a lone sub-race (its race filtered out) at the end', () => {
    const rows = [
      opt(TableContentKind.RACE, 'Anão', { key: 'race:dwarf' }),
      opt(TableContentKind.RACE, 'Elfo', { key: 'race:elf' }),
      opt(TableContentKind.SUBRACE, 'Alto Elfo', { key: 'subrace:high', parentKey: 'race:elf' }),
      opt(TableContentKind.SUBRACE, 'Anão da Colina', {
        key: 'subrace:hill',
        parentKey: 'race:dwarf',
      }),
      opt(TableContentKind.SUBRACE, 'Gnomo da Rocha', {
        key: 'subrace:rock',
        parentKey: 'race:gnome',
      }),
    ];
    expect(nestRows(rows).map((r) => `${r.nested ? '  ' : ''}${r.row.namePt}`)).toEqual([
      'Anão',
      '  Anão da Colina',
      'Elfo',
      '  Alto Elfo',
      'Gnomo da Rocha',
    ]);
  });

  it('agrees the words with the noun: backgrounds are masculine', () => {
    expect(onOffWords(TableContentKind.BACKGROUND)).toEqual({ on: 'ligado', off: 'desligado' });
    expect(onOffWords(TableContentKind.RACE)).toEqual({ on: 'ligada', off: 'desligada' });
    expect(changeSentence(opt(TableContentKind.BACKGROUND, 'Acólito'), true)).toBe(
      'Acólito: desligado para os jogadores.',
    );
  });

  it('searches by name without accents or case', () => {
    expect(searchRows(races(), 'ANAO').map((o) => o.namePt)).toEqual(['Anão', 'Anão da Colina']);
    expect(searchRows(races(), '  ')).toHaveLength(4);
    expect(searchRows(races(), 'xyz')).toEqual([]);
  });

  it('says why a child is hidden when its own switch is on', () => {
    const list = withHidden([
      opt(TableContentKind.CLASS, 'Mago', { key: 'class:wizard', off: true }),
      opt(TableContentKind.SUBCLASS, 'Evocação', {
        key: 'subclass:evocation',
        parentKey: 'class:wizard',
      }),
      opt(TableContentKind.SUBCLASS, 'Ilusão', {
        key: 'subclass:illusion',
        parentKey: 'class:wizard',
        off: true,
      }),
    ]);
    const byKey = new Map(list.map((o) => [o.key, o]));
    expect(list[1].hidden).toBe(true);
    expect(list[1].off).toBe(false);
    expect(hiddenNote(list[1], byKey)).toBe(
      'Some para os jogadores: a classe Mago está desligada.',
    );
    // Its own switch is the reason for the third: no extra line.
    expect(hiddenNote(list[2], byKey)).toBe('');
  });

  it('writes what a switch did and what it did not touch', () => {
    const using = opt(TableContentKind.RACE, 'Gnomo', { charactersUsing: 1 });
    expect(changeSentence(using, true)).toBe(
      'Gnomo: desligada para os jogadores. 1 ficha usa e continua funcionando.',
    );
    expect(
      changeSentence(opt(TableContentKind.RACE, 'Elfo', { charactersUsing: 3 }), true),
    ).toContain('3 fichas usam e continuam funcionando');
    expect(changeSentence(using, false)).toBe('Gnomo: ligada para os jogadores.');
  });

  it('"Ligar todas" changes only what is not already on', () => {
    expect(bulkChanges(races(), false)).toEqual([{ key: 'race:tiefling', off: false }]);
    expect(bulkChanges(races(), true).map((c) => c.key)).toEqual([
      'race:anão',
      'race:corujeiro@mesa',
      'subrace:colina',
    ]);
  });
});

describe('OptionSwitchesState: every change is saved at once', () => {
  const answer = (changed: OptionSwitchEntry[]) => ({
    options: changed,
    changed: changed.length,
    tableRevision: 9,
  });

  function make(list: OptionSwitchEntry[]) {
    const setSwitches = vi.fn();
    const switches = vi.fn().mockResolvedValue({ options: list, tableRevision: 8 });
    const state = new OptionSwitchesState(
      { switches, setSwitches },
      'camp-1',
      () => 'Não foi possível salvar.',
    );
    return { state, setSwitches, switches };
  }

  it('turns one off: the switch moves at once, one request, "Tudo salvo", and what the server says about the others', async () => {
    const list = [
      opt(TableContentKind.CLASS, 'Mago', { key: 'class:wizard', charactersUsing: 1 }),
      opt(TableContentKind.SUBCLASS, 'Evocação', {
        key: 'subclass:evocation',
        parentKey: 'class:wizard',
      }),
    ];
    const { state, setSwitches } = make(list);
    await state.load();
    setSwitches.mockResolvedValue(
      answer([
        { ...list[0], off: true, hidden: true } as OptionSwitchEntry,
        { ...list[1], hidden: true } as OptionSwitchEntry,
      ]),
    );
    const done = state.toggle('class:wizard', true);
    // Before the answer: moved, and saving.
    expect(state.options()[0].off).toBe(true);
    expect(state.save()).toEqual({ kind: 'saving' });
    await done;
    expect(setSwitches).toHaveBeenCalledWith('camp-1', [{ key: 'class:wizard', off: true }]);
    expect(state.save()).toEqual({ kind: 'saved' });
    // The subclass keeps its own switch and goes hidden for the players.
    expect(state.options()[1]).toMatchObject({ off: false, hidden: true });
    expect(state.announce()).toBe(
      'Mago: desligada para os jogadores. 1 ficha usa e continua funcionando.',
    );
  });

  it('puts the switch back and says why when the call fails', async () => {
    const { state, setSwitches } = make([opt(TableContentKind.RACE, 'Anão')]);
    await state.load();
    setSwitches.mockRejectedValue(new Error('offline'));
    await state.toggle('race:anão', true);
    expect(state.options()[0].off).toBe(false);
    expect(state.save()).toEqual({
      kind: 'error',
      message: 'Não foi possível salvar. O que você mudou voltou ao que era.',
    });
  });

  it('shows "Tudo salvo" at rest, before anything was changed', async () => {
    const { state } = make([opt(TableContentKind.RACE, 'Anão')]);
    expect(state.save()).toEqual({ kind: 'saved' });
  });

  it('keeps an error until the key that failed saves, or the person dismisses it: another change never hides it behind "Tudo salvo"', async () => {
    const { state, setSwitches } = make([
      opt(TableContentKind.RACE, 'Anão'),
      opt(TableContentKind.RACE, 'Elfo'),
    ]);
    await state.load();
    setSwitches.mockRejectedValueOnce(new Error('offline'));
    await state.toggle('race:anão', true);
    expect(state.save().kind).toBe('error');
    setSwitches.mockResolvedValue(answer([]));
    await state.toggle('race:elfo', true);
    expect(state.save().kind).toBe('error');
    // The key that failed saves now: the error goes.
    await state.toggle('race:anão', true);
    expect(state.save()).toEqual({ kind: 'saved' });
    // Or the person closes it.
    setSwitches.mockRejectedValueOnce(new Error('offline'));
    await state.toggle('race:anão', false);
    expect(state.save().kind).toBe('error');
    state.dismiss();
    expect(state.save()).toEqual({ kind: 'saved' });
  });

  it("merges the server's `hidden` as it comes, and does not let the guess override it", async () => {
    const list = [
      opt(TableContentKind.CLASS, 'Mago', { key: 'class:wizard' }),
      opt(TableContentKind.SUBCLASS, 'Evocação', {
        key: 'subclass:evocation',
        parentKey: 'class:wizard',
      }),
    ];
    const { state, setSwitches } = make(list);
    await state.load();
    // The server says nothing is hidden (say, the subclass is the sheet's own): the screen follows it.
    setSwitches.mockResolvedValue(
      answer([
        { ...list[0], off: true, hidden: true } as OptionSwitchEntry,
        { ...list[1], hidden: false } as OptionSwitchEntry,
      ]),
    );
    await state.toggle('class:wizard', true);
    expect(state.options()[1].hidden).toBe(false);
  });

  it('drops a silent read that began before a change that has already finished (it would flip the switch back)', async () => {
    const { state, setSwitches, switches } = make([opt(TableContentKind.RACE, 'Anão')]);
    await state.load();
    let release!: (v: unknown) => void;
    switches.mockImplementation(() => new Promise((r) => (release = r)));
    const read = state.load(true);
    setSwitches.mockResolvedValue(answer([]));
    await state.toggle('race:anão', true);
    // The slow read answers now, with the old state: it is older than the change.
    release({ options: [opt(TableContentKind.RACE, 'Anão')], tableRevision: 1 });
    await read;
    expect(state.options()[0].off).toBe(true);
  });

  it('"Desligar todas" sends one call with the keys that change', async () => {
    const list = [
      opt(TableContentKind.RACE, 'Anão'),
      opt(TableContentKind.RACE, 'Elfo'),
      opt(TableContentKind.RACE, 'Tiefling', { off: true }),
    ];
    const { state, setSwitches } = make(list);
    await state.load();
    setSwitches.mockResolvedValue(answer([]));
    await state.setAll(state.options(), true, 'Raças');
    expect(setSwitches).toHaveBeenCalledTimes(1);
    expect(setSwitches).toHaveBeenCalledWith('camp-1', [
      { key: 'race:anão', off: true },
      { key: 'race:elfo', off: true },
    ]);
    expect(state.announce()).toBe('Raças: 2 desligadas.');
    // Nothing to change, nothing sent.
    await state.setAll(state.options(), true, 'Raças');
    expect(setSwitches).toHaveBeenCalledTimes(1);
  });

  it('sends the changes one after the other, in the order made', async () => {
    const { state, setSwitches } = make([
      opt(TableContentKind.RACE, 'Anão'),
      opt(TableContentKind.RACE, 'Elfo'),
    ]);
    await state.load();
    const order: string[] = [];
    setSwitches.mockImplementation(async (_c: string, sw: { key: string }[]) => {
      order.push(`start ${sw[0].key}`);
      await new Promise((r) => setTimeout(r, 5));
      order.push(`end ${sw[0].key}`);
      return answer([]);
    });
    await Promise.all([state.toggle('race:anão', true), state.toggle('race:elfo', true)]);
    expect(order).toEqual(['start race:anão', 'end race:anão', 'start race:elfo', 'end race:elfo']);
  });

  it('reads again after the table changed, keeping a change that is still on its way', async () => {
    const { state, setSwitches, switches } = make([opt(TableContentKind.RACE, 'Anão')]);
    await state.load();
    switches.mockResolvedValue({
      options: [opt(TableContentKind.RACE, 'Anão'), opt(TableContentKind.RACE, 'Elfo')],
      tableRevision: 9,
    });
    await state.load(true);
    expect(state.options()).toHaveLength(2);
    // A read that arrives while a change is on its way is the older truth: it is dropped.
    let release!: () => void;
    setSwitches.mockImplementation(() => new Promise((r) => (release = () => r(answer([])))));
    const pending = state.toggle('race:anão', true);
    switches.mockResolvedValue({
      options: [opt(TableContentKind.RACE, 'Anão')],
      tableRevision: 10,
    });
    await state.load(true);
    expect(state.options()).toHaveLength(2);
    expect(state.options()[0].off).toBe(true);
    release();
    await pending;
  });
});
