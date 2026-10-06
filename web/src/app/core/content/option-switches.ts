import { signal } from '@angular/core';

import { type OptionSwitchEntry, TableContentKind } from '../../../gen/meurpg/rules/v1/table_content_pb';
import { type ContentNavKind } from './content-kinds';

/**
 * "Opções para os jogadores" (MR-025, RN-23, E10-01 state 3): what the master's list says about each option and the page's
 * state, as plain functions and one small class so the counts, the search, "Ligar todas" and the save line are tested without
 * a screen. Nothing here decides who sees what: the server's `hidden` and `off` are written as they come; the only thing
 * computed here is the guess a switch shows for a child until the server's answer arrives.
 */

/** The words a group uses for "ligadas": the nouns agree ("Raças: 9 de 10 ligadas", "Antecedentes: 2 de 2 ligados"). */
const ON_WORDS: Readonly<Record<ContentNavKind['slug'], { one: string; many: string }>> = {
  classes: { one: 'ligada', many: 'ligadas' },
  subclasses: { one: 'ligada', many: 'ligadas' },
  racas: { one: 'ligada', many: 'ligadas' },
  antecedentes: { one: 'ligado', many: 'ligados' },
  magias: { one: 'ligada', many: 'ligadas' },
};

export interface OptionCount {
  readonly on: number;
  readonly total: number;
}

export function isKindOf(o: Pick<OptionSwitchEntry, 'kind'>, kinds: readonly TableContentKind[]): boolean {
  return kinds.includes(o.kind);
}

/** The group's rows, in the server's order (by kind, then Portuguese name); a race's sub-races follow the races. */
export function groupRows(options: readonly OptionSwitchEntry[], nav: ContentNavKind): OptionSwitchEntry[] {
  return options.filter((o) => isKindOf(o, nav.kinds));
}

/** How many have their own switch on. In "Raças" the sub-races are counted apart (`subraces`). */
export function countOf(options: readonly OptionSwitchEntry[], kinds: readonly TableContentKind[]): OptionCount {
  const rows = options.filter((o) => isKindOf(o, kinds));
  return { on: rows.filter((o) => !o.off).length, total: rows.length };
}

export function raceCount(options: readonly OptionSwitchEntry[]): OptionCount {
  return countOf(options, [TableContentKind.RACE]);
}

export function subraceCount(options: readonly OptionSwitchEntry[]): OptionCount {
  return countOf(options, [TableContentKind.SUBRACE]);
}

/** The group's own kind for the counter: the races, not the sub-races. */
export function mainCount(options: readonly OptionSwitchEntry[], nav: ContentNavKind): OptionCount {
  return nav.slug === 'racas' ? raceCount(options) : countOf(options, nav.kinds);
}

/** "Raças: 9 de 10 ligadas" */
export function counterText(nav: ContentNavKind, c: OptionCount): string {
  const w = ON_WORDS[nav.slug];
  return `${nav.plural}: ${c.on} de ${c.total} ${c.total === 1 ? w.one : w.many}`;
}

/** "Sub-raças: 4 de 4 ligadas" */
export function subraceCounterText(c: OptionCount): string {
  return `Sub-raças: ${c.on} de ${c.total} ${c.total === 1 ? 'ligada' : 'ligadas'}`;
}

/** The menu's short form: "9 de 10". */
export function menuCount(options: readonly OptionSwitchEntry[], nav: ContentNavKind): string {
  const c = mainCount(options, nav);
  return `${c.on} de ${c.total}`;
}

/** Lower-case, no accents: "Raça" is found by "raca". */
export function fold(text: string): string {
  return text
    .normalize('NFD')
    .replace(/[̀-ͯ]/g, '')
    .toLowerCase()
    .trim();
}

export function searchRows(rows: readonly OptionSwitchEntry[], query: string): OptionSwitchEntry[] {
  const q = fold(query);
  return q ? rows.filter((o) => fold(o.namePt).includes(q)) : [...rows];
}

/** "Nenhuma ficha usa", "1 ficha usa", "3 fichas usam" (a non-breaking space keeps the number with its noun). */
export function usingText(n: number): string {
  if (n <= 0) {
    return 'Nenhuma ficha usa';
  }
  return `${n} ${n === 1 ? 'ficha usa' : 'fichas usam'}`;
}

/** What the row says under the name when the players do not get it for a reason that is not its own switch. */
export function hiddenNote(o: OptionSwitchEntry, byKey: ReadonlyMap<string, OptionSwitchEntry>): string {
  if (!o.hidden || o.off || o.archived) {
    return '';
  }
  const parent = o.parentKey ? byKey.get(o.parentKey) : undefined;
  if (!parent) {
    return 'Os jogadores não a recebem.';
  }
  return `Some para os jogadores: ${parent.namePt} ${parent.off ? 'está desligada' : 'está arquivada'}.`;
}

/** The sentence a switch leaves for a screen reader and the eye: what happened and what it did not touch. */
export function changeSentence(o: OptionSwitchEntry, off: boolean): string {
  const base = off ? `${o.namePt}: desligada para os jogadores.` : `${o.namePt}: ligada para os jogadores.`;
  if (!off) {
    return base;
  }
  const n = o.charactersUsing;
  if (n > 0) {
    return `${base} ${n === 1 ? '1 ficha usa e continua funcionando' : `${n} fichas usam e continuam funcionando`}.`;
  }
  return base;
}

/** Who gets the "o que cada jogador vê" line of a group: the children of an off parent are listed with their own switch. */
export function withHidden(options: readonly OptionSwitchEntry[]): OptionSwitchEntry[] {
  const byKey = new Map(options.map((o) => [o.key, o]));
  return options.map((o) => {
    const parent = o.parentKey ? byKey.get(o.parentKey) : undefined;
    const hidden = o.off || o.archived || (parent !== undefined && (parent.off || parent.archived));
    return hidden === o.hidden ? o : ({ ...o, hidden } as OptionSwitchEntry);
  });
}

/** Merges the options the server says changed into the list (by key). */
export function mergeChanged(options: readonly OptionSwitchEntry[], changed: readonly OptionSwitchEntry[]): OptionSwitchEntry[] {
  const byKey = new Map(changed.map((o) => [o.key, o]));
  return options.map((o) => byKey.get(o.key) ?? o);
}

/** The keys that `Ligar todas` or `Desligar todas` changes: only those that are not in the state asked. */
export function bulkChanges(rows: readonly OptionSwitchEntry[], off: boolean): { key: string; off: boolean }[] {
  return rows.filter((o) => o.off !== off).map((o) => ({ key: o.key, off }));
}

export interface SwitchSource {
  switches(campaignId: string): Promise<{ options: readonly OptionSwitchEntry[]; tableRevision: number }>;
  setSwitches(
    campaignId: string,
    switches: readonly { key: string; off: boolean }[],
  ): Promise<{ options: readonly OptionSwitchEntry[]; changed: number; tableRevision: number }>;
}

export type SaveLine =
  | { readonly kind: 'idle' }
  | { readonly kind: 'saving' }
  | { readonly kind: 'saved' }
  | { readonly kind: 'error'; readonly message: string };

/** The server accepts up to 700 switches in a call; the biggest group (the spells) has 320, but the call stays in the limit. */
export const SWITCH_BATCH = 700;

/**
 * The page's state. Every change is sent at once, one call after another (the server orders them, and so does the queue here),
 * and shown at once: the switch moves, and if the call fails it moves back with the reason. The list is read again after the
 * hint that the table changed (`reload`), without a spinner.
 */
export class OptionSwitchesState {
  readonly options = signal<readonly OptionSwitchEntry[]>([]);
  readonly status = signal<'loading' | 'ready' | 'error'>('loading');
  readonly loadError = signal('');
  readonly save = signal<SaveLine>({ kind: 'idle' });
  /** The last thing a switch did, said once (a status region reads it). */
  readonly announce = signal('');

  private queue: Promise<void> = Promise.resolve();
  private pending = 0;
  private seq = 0;

  constructor(
    private readonly source: SwitchSource,
    private readonly campaignId: string,
    private readonly errorText: (err: unknown) => string,
  ) {}

  async load(silent = false): Promise<void> {
    const seq = ++this.seq;
    if (!silent) {
      this.status.set('loading');
    }
    try {
      const res = await this.source.switches(this.campaignId);
      if (seq !== this.seq || this.pending > 0) {
        // A newer read, or a change on its way (its answer is the newer truth): this one is stale.
        return;
      }
      this.options.set(res.options);
      this.status.set('ready');
    } catch (err) {
      if (seq === this.seq && !silent) {
        this.loadError.set(this.errorText(err));
        this.status.set('error');
      }
    }
  }

  /** One option, on or off. */
  toggle(key: string, off: boolean): Promise<void> {
    const o = this.options().find((x) => x.key === key);
    if (!o || o.off === off) {
      return Promise.resolve();
    }
    return this.change([{ key, off }], changeSentence(o, off));
  }

  /** "Ligar todas" and "Desligar todas": the rows in view, the ones not in the state asked. */
  setAll(rows: readonly OptionSwitchEntry[], off: boolean, what: string): Promise<void> {
    const changes = bulkChanges(rows, off);
    if (changes.length === 0) {
      return Promise.resolve();
    }
    const using = rows.filter((o) => o.off !== off).reduce((n, o) => n + o.charactersUsing, 0);
    const word = off ? 'desligadas' : 'ligadas';
    const tail = off && using > 0 ? ' As fichas que as usam continuam funcionando.' : '';
    return this.change(changes, `${what}: ${changes.length} ${changes.length === 1 ? (off ? 'desligada' : 'ligada') : word}.${tail}`);
  }

  private change(changes: { key: string; off: boolean }[], sentence: string): Promise<void> {
    const before = new Map(this.options().map((o) => [o.key, o.off]));
    const asked = new Map(changes.map((c) => [c.key, c.off]));
    this.options.update((list) => withHidden(list.map((o) => (asked.has(o.key) ? ({ ...o, off: asked.get(o.key)! } as OptionSwitchEntry) : o))));
    this.pending++;
    this.save.set({ kind: 'saving' });
    this.queue = this.queue.then(async () => {
      try {
        for (let i = 0; i < changes.length; i += SWITCH_BATCH) {
          const res = await this.source.setSwitches(this.campaignId, changes.slice(i, i + SWITCH_BATCH));
          this.options.update((list) => withHidden(mergeChanged(list, res.options)));
        }
        this.pending--;
        if (this.pending === 0) {
          this.save.set({ kind: 'saved' });
        }
        this.announce.set(sentence);
      } catch (err) {
        this.pending--;
        // Back to what it was: the switch never says what the server did not accept.
        this.options.update((list) => withHidden(list.map((o) => (before.has(o.key) && asked.has(o.key) ? ({ ...o, off: before.get(o.key)! } as OptionSwitchEntry) : o))));
        this.save.set({ kind: 'error', message: `${this.errorText(err)} O que você mudou voltou ao que era.` });
        this.announce.set('');
      }
    });
    return this.queue;
  }
}
