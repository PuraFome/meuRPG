import { type Combatant, type Encounter } from '../../../gen/meurpg/play/v1/combat_pb';
import { openWindows } from './reactions';

/**
 * The master's card of a legendary creature's actions (#557): what is left of the round, the options of its stat block
 * with their cost, and why one is off. The server decides what a spend costs and when it is allowed
 * (`LegendaryState.usable_now`, `LegendaryOption.affordable`); this only phrases it. An option's name is always the
 * Portuguese one (`name_pt`): `name` and `text` are the SRD's English and never reach the screen. Folding the card is the
 * screen's own business and never calls the server. Pure functions, tested without a DOM.
 */

/** What the row's main button does: spends the cost on a text option, opens the attack sheet, or opens the move. */
export type LegendaryAction = 'spend' | 'attack' | 'move';

/** One option of the stat block. */
export interface LegendaryRow {
  readonly key: string;
  /** The Portuguese name ("Ataque de cauda"). */
  readonly name: string;
  /** "custa 1". */
  readonly costText: string;
  readonly cost: number;
  /** "Usar" (text and move) or "Atacar". */
  readonly buttonLabel: string;
  readonly action: LegendaryAction;
  /** The stat block action an attack option makes (`RollAttack.attack_key`); `''` for the others. */
  readonly attackKey: string;
  /** What the row says under the name when it has something to say: "Você resolve: o app só conta o gasto.". */
  readonly detail: string;
  readonly disabled: boolean;
  /** Why it is off ("Custa 2; restam 1."), or `''`. */
  readonly reason: string;
}

/** The card of one legendary creature. */
export interface LegendaryCard {
  readonly combatantId: string;
  /** The creature's label. */
  readonly title: string;
  /** One mark per action of the round: `true` for one left, `false` for one spent. */
  readonly pips: readonly boolean[];
  readonly spent: number;
  readonly left: number;
  readonly total: number;
  /** "3 de 3". */
  readonly counts: string;
  /** "voltam no começo do turno dele". */
  readonly recovery: string;
  /** "Depois" while a window holds the table or the creature cannot act, else "Opcional". */
  readonly badge: 'Depois' | 'Opcional';
  /** "Use no fim do turno de Tor, uma opção de cada vez.". */
  readonly hint: string;
  /** Whether a reaction window is open: the table waits and the rows are not offered yet. */
  readonly blocked: boolean;
  /** "Depois da janela acima: enquanto uma reação espera, nada anda." while blocked, else `''`. */
  readonly waiting: string;
  /** "Incapacitado: a criatura não pode usar ações lendárias agora." when the server says it cannot spend, else `''`. */
  readonly unusable: string;
  readonly usableNow: boolean;
  readonly rows: readonly LegendaryRow[];
}

/** What the card of the dragon's own turn says instead: "Ações lendárias: 3 de 3, renovadas agora". */
export interface LegendaryRecovered {
  readonly combatantId: string;
  readonly title: string;
  readonly actions: string;
  /** "Resistência Lendária: 2 de 3 hoje", or `''` for a stat block without the trait. */
  readonly resistance: string;
  /** "Ações lendárias se usam no turno de outra criatura, nunca no dele.". */
  readonly note: string;
}

export const LEGENDARY_UNUSABLE = 'Incapacitado: a criatura não pode usar ações lendárias agora.';

const WORDS: Readonly<Record<number, string>> = { 2: 'duas', 3: 'três', 4: 'quatro', 5: 'cinco' };

function legendaryOf(c: Combatant) {
  const l = c.legendary;
  return l && l.options.length > 0 ? l : null;
}

/** "Depois da janela acima: ..." / "Depois das duas janelas acima: ...", for the n windows that hold the table. */
export function waitingBehind(n: number): string {
  if (n <= 0) {
    return '';
  }
  const behind = n === 1 ? 'da janela acima' : `das ${WORDS[n] ?? n} janelas acima`;
  return `Depois ${behind}: enquanto uma reação espera, nada anda.`;
}

function rowOf(
  o: NonNullable<Combatant['legendary']>['options'][number],
  left: number,
  usableNow: boolean,
): LegendaryRow {
  const action: LegendaryAction =
    o.kind === 'attack' ? 'attack' : o.kind === 'move' ? 'move' : 'spend';
  const affordable = o.affordable && o.cost <= left;
  return {
    key: o.key,
    // Never `o.name` (English): the server sends the Portuguese one, even for homebrew that has no other.
    name: o.namePt || 'Ação lendária',
    costText: `custa ${o.cost}`,
    cost: o.cost,
    buttonLabel: action === 'attack' ? 'Atacar' : 'Usar',
    action,
    attackKey: action === 'attack' ? o.attackKey : '',
    detail: action === 'spend' ? 'Você resolve: o app só conta o gasto.' : '',
    disabled: !usableNow || !affordable,
    reason: !usableNow ? LEGENDARY_UNUSABLE : !affordable ? `Custa ${o.cost}; restam ${left}.` : '',
  };
}

/** The card of one legendary creature that does not have the turn, or `null` for any other combatant. */
export function legendaryCard(e: Encounter, c: Combatant): LegendaryCard | null {
  const l = legendaryOf(c);
  if (!l || c.id === e.currentCombatantId) {
    return null;
  }
  const blocking = openWindows(e).length;
  const turn = e.combatants.find((x) => x.id === e.currentCombatantId);
  const left = Math.max(0, Math.min(l.actionsLeft, l.actionsPerRound));
  return {
    combatantId: c.id,
    title: c.label,
    pips: Array.from({ length: l.actionsPerRound }, (_, i) => i < left),
    spent: l.actionsPerRound - left,
    left,
    total: l.actionsPerRound,
    counts: `${left} de ${l.actionsPerRound}`,
    recovery: 'voltam no começo do turno dele',
    badge: blocking > 0 || !l.usableNow ? 'Depois' : 'Opcional',
    hint: `Use no fim do turno de ${turn?.label || 'outra criatura'}, uma opção de cada vez.`,
    blocked: blocking > 0,
    waiting: waitingBehind(blocking),
    unusable: l.usableNow ? '' : LEGENDARY_UNUSABLE,
    usableNow: l.usableNow,
    rows: l.options.map((o) => rowOf(o, left, l.usableNow)),
  };
}

/** The cards of every legendary creature of the combat that may be offered now (none while the combat is not running). */
export function legendaryCards(e: Encounter): readonly LegendaryCard[] {
  return e.combatants.flatMap((c) => {
    const card = legendaryCard(e, c);
    return card ? [card] : [];
  });
}

/** The folded card: "Ações lendárias · 3 de 3". Folding never calls the server. */
export function legendaryFolded(card: LegendaryCard): string {
  return `Ações lendárias · ${card.counts}`;
}

/** The note of the dragon's own turn (the actions came back, the card is gone), or `null` when the creature is not on turn. */
export function legendaryRecovered(e: Encounter, c: Combatant): LegendaryRecovered | null {
  const l = legendaryOf(c);
  if (!l || c.id !== e.currentCombatantId) {
    return null;
  }
  return {
    combatantId: c.id,
    title: c.label,
    actions: `Ações lendárias: ${l.actionsPerRound} de ${l.actionsPerRound}, renovadas agora`,
    resistance:
      l.resistancePerDay > 0
        ? `Resistência Lendária: ${l.resistanceLeft} de ${l.resistancePerDay} hoje`
        : '',
    note: 'Ações lendárias se usam no turno de outra criatura, nunca no dele.',
  };
}

/** The English option name of the stat block to its Portuguese one, for the log's lines (`CombatLogLegendarySpent.option_name`). */
export function legendaryNames(e: Encounter): ReadonlyMap<string, string> {
  const names = new Map<string, string>();
  for (const c of e.combatants) {
    for (const o of c.legendary?.options ?? []) {
      if (o.name && o.namePt) {
        names.set(o.name, o.namePt);
      }
    }
  }
  return names;
}
