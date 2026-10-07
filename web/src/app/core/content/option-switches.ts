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

/**
 * Who uses an option: "1 ficha usa", "3 fichas usam" (a non-breaking space keeps the number with its noun); nothing for an
 * option no sheet uses, except on an off row, where "Nenhuma ficha usa" says that turning it off touches no sheet.
 */
export function usingText(n: number, off = false): string {
  if (n > 0) {
    return `${n}\u00a0${n === 1 ? 'ficha usa' : 'fichas usam'}`;
  }
  return off ? 'Nenhuma\u00a0ficha\u00a0usa' : '';
}

/** Whether the option's noun is masculine ("o antecedente"): the words that agree with it follow. */
export function isMasculine(kind: TableContentKind): boolean {
  return kind === TableContentKind.BACKGROUND;
}

/** "ligada"/"ligado", "desligada"/"desligado", agreeing with the kind's noun. */
export function onOffWords(kind: TableContentKind): { on: string; off: string } {
  return isMasculine(kind) ? { on: 'ligado', off: 'desligado' } : { on: 'ligada', off: 'desligada' };
}

/** How many children (subclasses, sub-races) the players do not receive because their parent is off or archived, while their own switch is on. */
export function hiddenByParent(rows: readonly OptionSwitchEntry[]): number {
  return rows.filter((o) => o.hidden && !o.off && !o.archived).length;
}

/** "· 3 escondidas pela classe": the counter's second half for the groups that have children. */
export function hiddenCounterText(nav: ContentNavKind, n: number): string {
  if (n <= 0) {
    return '';
  }
  const by = nav.slug === 'racas' ? 'pela raça' : 'pela classe';
  return ` · ${n} ${n === 1 ? 'escondida' : 'escondidas'} ${by}`;
}

/** The races with their sub-races under them (a sub-race whose race is not in the rows follows at the end). */
export function nestRows(rows: readonly OptionSwitchEntry[]): { row: OptionSwitchEntry; nested: boolean }[] {
  const keys = new Set(rows.map((o) => o.key));
  const children = new Map<string, OptionSwitchEntry[]>();
  for (const o of rows) {
    if (o.kind === TableContentKind.SUBRACE && o.parentKey && keys.has(o.parentKey)) {
      children.set(o.parentKey, [...(children.get(o.parentKey) ?? []), o]);
    }
  }
  const out: { row: OptionSwitchEntry; nested: boolean }[] = [];
  for (const o of rows) {
    if (o.kind === TableContentKind.SUBRACE && o.parentKey && keys.has(o.parentKey)) {
      continue;
    }
    out.push({ row: o, nested: false });
    for (const c of children.get(o.key) ?? []) {
      out.push({ row: c, nested: true });
    }
  }
  return out;
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
  const noun = parent.kind === TableContentKind.RACE ? 'a raça' : 'a classe';
  return `Some para os jogadores: ${noun} ${parent.namePt} ${parent.off ? 'está desligada' : 'está arquivada'}.`;
}

/** The sentence a switch leaves for a screen reader and the eye: what happened and what it did not touch. */
export function changeSentence(o: OptionSwitchEntry, off: boolean): string {
  const w = onOffWords(o.kind);
  const base = `${o.namePt}: ${off ? w.off : w.on} para os jogadores.`;
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
  /** "Tudo salvo" at rest: nothing is waiting, nothing failed. */
  readonly save = signal<SaveLine>({ kind: 'saved' });
  /** The last thing a switch did, said once (a status region reads it). */
  readonly announce = signal('');

  private queue: Promise<void> = Promise.resolve();
  private pending = 0;
  private seq = 0;
  /** Counts the changes started: a read that began before one is older than it and is dropped. */
  private changes = 0;
  /** The keys whose last save failed: the error stays until each of them saves or the person dismisses it. */
  private readonly failed = new Set<string>();
  private failure = '';

  constructor(
    private readonly source: SwitchSource,
    private readonly campaignId: string,
    private readonly errorText: (err: unknown) => string,
  ) {}

  async load(silent = false): Promise<void> {
    const seq = ++this.seq;
    const startedAt = this.changes;
    if (!silent) {
      this.status.set('loading');
    }
    try {
      const res = await this.source.switches(this.campaignId);
      if (seq !== this.seq || this.pending > 0 || this.changes !== startedAt) {
        // A newer read, or a change started (or is on its way) after this one began: its answer is the newer truth.
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

  /** The error under the counter is closed (the switches are already back where the server has them). */
  dismiss(): void {
    this.failed.clear();
    this.failure = '';
    if (this.pending === 0) {
      this.save.set({ kind: 'saved' });
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
    const masculine = rows.length > 0 && rows.every((o) => isMasculine(o.kind));
    const w = masculine ? { on: 'ligado', off: 'desligado' } : { on: 'ligada', off: 'desligada' };
    const word = (off ? w.off : w.on) + (changes.length === 1 ? '' : 's');
    const tail = off && using > 0 ? ` ${using === 1 ? 'A ficha que a usa continua funcionando' : 'As fichas que as usam continuam funcionando'}.` : '';
    return this.change(changes, `${what}: ${changes.length} ${word}.${tail}`);
  }

  private change(changes: { key: string; off: boolean }[], sentence: string): Promise<void> {
    const before = new Map(this.options().map((o) => [o.key, o.off]));
    const asked = new Map(changes.map((c) => [c.key, c.off]));
    this.changes++;
    // The switch moves at once; what is hidden for the players follows the server's answer (this is only the guess).
    this.options.update((list) => withHidden(list.map((o) => (asked.has(o.key) ? ({ ...o, off: asked.get(o.key)! } as OptionSwitchEntry) : o))));
    this.pending++;
    this.save.set({ kind: 'saving' });
    this.queue = this.queue.then(async () => {
      try {
        for (let i = 0; i < changes.length; i += SWITCH_BATCH) {
          const res = await this.source.setSwitches(this.campaignId, changes.slice(i, i + SWITCH_BATCH));
          this.options.update((list) => mergeChanged(list, res.options));
        }
        this.pending--;
        for (const c of changes) {
          this.failed.delete(c.key);
        }
        this.announce.set(sentence);
        this.settleLine();
      } catch (err) {
        this.pending--;
        // Back to what it was: the switch never says what the server did not accept.
        this.options.update((list) => withHidden(list.map((o) => (before.has(o.key) && asked.has(o.key) ? ({ ...o, off: before.get(o.key)! } as OptionSwitchEntry) : o))));
        for (const c of changes) {
          this.failed.add(c.key);
        }
        this.failure = `${this.errorText(err)} O que você mudou voltou ao que era.`;
        this.announce.set('');
        this.settleLine();
      }
    });
    return this.queue;
  }

  /** The line at the end of a call: an error stays while a key that failed has not saved; otherwise "Tudo salvo". */
  private settleLine(): void {
    if (this.pending > 0) {
      this.save.set({ kind: 'saving' });
    } else if (this.failed.size > 0) {
      this.save.set({ kind: 'error', message: this.failure });
    } else {
      this.save.set({ kind: 'saved' });
    }
  }
}
