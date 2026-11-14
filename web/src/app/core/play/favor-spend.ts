import { computed, signal } from '@angular/core';

import {
  type AdvantageSource,
  AdvantageSourceKind,
  RollMode,
} from '../../../gen/meurpg/play/v1/combat_rolls_pb';
import { RollModeKind, type RollNote } from '../../../gen/meurpg/play/v1/contest_types_pb';
import { FavorBlockedReason } from '../../../gen/meurpg/play/v1/favor_pb';
import type { Combatant } from '../../../gen/meurpg/play/v1/combat_pb';
import { isPlayer } from '../combat/combat-view';
import { orNormal } from '../combat/roll-mode';
import { favorBlocked, favorBlockedMessage } from './favor-errors';
import type { FavorState } from './favor-state';

/** The sentence under the box when the Inspiração is the master's to hold, not the master's to spend (board 581-07, item 7). */
export const FAVOR_ONLY_PLAYER = 'Só a jogadora pode usar a Inspiração dela.';
/** The read of who has it failed: the box is there, dashed, and says so (581-07, item 13). */
export const FAVOR_READ_FAILED = 'Não deu para ler a Inspiração agora. Tente de novo em instantes.';
/** A mode asked of the master is on its way: the Inspiração stays out of this roll (the app's own words, not on a board). */
export const FAVOR_MODE_ASKED = 'Há um pedido de modo ao mestre nesta rolagem.';
/** The words of the lock the roll-mode segment wears while the box is ticked (581-03). */
export const FAVOR_LOCK_WORD = 'pela Inspiração';

/** "O mestre agindo por Brisa". */
export function favorByMasterText(name: string): string {
  return `O mestre agindo por ${name}`;
}

/** What a roll's circumstances say, as the roller reads them: the mode the roll would be made with, which counts the sources the
 * roller may not read, and the name of each source they do read (a hidden source, "Outra fonte", is neither: the spend is
 * judged on what the roller sees, as the server does, so a refusal never tells a player that a hidden source exists). */
export interface FavorCircumstances {
  readonly mode: RollMode;
  readonly advantages: readonly string[];
  readonly disadvantages: readonly string[];
}

/** "Ajuda de Tor" from "Ajuda de Tor: vantagem": the part of a sentence before its colon names the source. */
function shortName(text: string): string {
  return text.split(':')[0].trim();
}

/** The circumstances of an attack or a spell attack: the server's sources (`TargetInReach.sources`) and its suggested mode. */
export function circumstancesOfSources(
  mode: RollMode,
  sources: readonly AdvantageSource[],
): FavorCircumstances {
  const seen = sources.filter(
    (s) => s.kind !== AdvantageSourceKind.OTHER_SOURCE && s.kind !== AdvantageSourceKind.FAVOR,
  );
  const named = (effect: RollMode) =>
    seen.filter((s) => s.effect === effect).map((s) => shortName(s.textPt));
  return {
    mode,
    advantages: named(RollMode.ADVANTAGE),
    disadvantages: named(RollMode.DISADVANTAGE),
  };
}

/** The circumstances of a check: the mode of the check's option and its notes (`RollNote`). A bonus an effect adds says nothing of
 * the mode, a hidden source reads "other" and the Inspiração itself is not one of the circumstances. */
export function circumstancesOfNotes(
  mode: RollModeKind,
  notes: readonly RollNote[],
): FavorCircumstances {
  const seen = notes.filter((n) => !['other', 'favor', 'effect_bonus'].includes(n.kind));
  return {
    mode:
      mode === RollModeKind.ADVANTAGE
        ? RollMode.ADVANTAGE
        : mode === RollModeKind.DISADVANTAGE
          ? RollMode.DISADVANTAGE
          : RollMode.NORMAL,
    advantages: seen.filter((n) => n.advantage).map((n) => n.labelPt),
    disadvantages: seen.filter((n) => !n.advantage).map((n) => n.labelPt),
  };
}

/** The circumstances of a saving throw a window asks (the concentration save, an effect's save): the prompt's `mode` ("normal",
 * "advantage" or "disadvantage") and its sources in words. A source the master hides reads "Outra fonte" and is not counted; the
 * sentences do not say which side they are on, so with a mode that is not normal they all name that side, and a normal mode names
 * none (the server judges it). */
export function circumstancesOfPrompt(
  mode: string,
  sourcesPt: readonly string[],
): FavorCircumstances {
  const names = sourcesPt.filter((s) => !s.startsWith('Outra fonte')).map(shortName);
  return {
    mode:
      mode === 'advantage'
        ? RollMode.ADVANTAGE
        : mode === 'disadvantage'
          ? RollMode.DISADVANTAGE
          : RollMode.NORMAL,
    advantages: mode === 'advantage' ? names : [],
    disadvantages: mode === 'disadvantage' ? names : [],
  };
}

/** What the Inspiração would do to a roll: nothing (and why), or the mode the roll then has and the sentence that says it. */
export type FavorJudgement =
  | { readonly ok: false; readonly reason: string }
  | { readonly ok: true; readonly mode: RollMode; readonly line: string };

/** The one place that works out the mode a roll has with the Inspiração (the server's table, "The spend" in the favor note):
 * normal becomes advantage, a disadvantage turns normal, an advantage (or an advantage and a disadvantage that cancel) is
 * `ADDS_NOTHING`, judged on the sources the roller reads. */
export function judgeFavor(c: FavorCircumstances): FavorJudgement {
  const nothing = 'A Inspiração não mudaria esta rolagem:';
  if (c.advantages.length > 0 && c.disadvantages.length > 0) {
    return {
      ok: false,
      reason: `${nothing} vantagem (${c.advantages.join(', ')}) e desvantagem (${c.disadvantages.join(', ')}) se anulam.`,
    };
  }
  if (c.advantages.length > 0) {
    return { ok: false, reason: `${nothing} ela já tem vantagem (${c.advantages.join(', ')}).` };
  }
  if (orNormal(c.mode) === RollMode.DISADVANTAGE || c.disadvantages.length > 0) {
    const names = c.disadvantages.length > 0 ? ` (${c.disadvantages.join(', ')})` : '';
    return {
      ok: true,
      mode: RollMode.NORMAL,
      line: `Desvantagem${names} e Inspiração se anulam: um d20. Desmarque a caixa para voltar a desvantagem.`,
    };
  }
  return {
    ok: true,
    mode: RollMode.ADVANTAGE,
    line: 'Vantagem pela Inspiração: rola dois d20 e fica com o maior. Desmarque a caixa para escolher o modo.',
  };
}

/** What the box shows. `open`: it can be ticked (`mode` is what the roll then reads, `null` when the sheet does not know the
 * circumstances beforehand). `off`: dashed, with `sub` under the label and `reason` in its own line. */
export type FavorView =
  | { readonly kind: 'open'; readonly mode: RollMode | null; readonly line: string }
  | { readonly kind: 'off'; readonly sub: string; readonly reason: string };

/** The character a roll is for. An empty `characterId` is the player's own, for the sheets only a player opens (a scene check, a
 * group check, a trap search, a hint): the read has none but their characters. */
export interface FavorWho {
  readonly characterId: string;
  readonly name: string;
}

/** The character of a combatant when it is a player's (an NPC or a creature has no Inspiração: `null`). */
export function favorWho(c: Combatant | undefined | null): FavorWho | null {
  return c && isPlayer(c) ? { characterId: c.characterId, name: c.label } : null;
}

/** What a roll sheet tells the box. */
export interface FavorSpendHost {
  readonly favor: FavorState;
  /** Who the roll is for, or `null` for one that has no Inspiração (an NPC, a creature): then nothing is drawn. */
  readonly who: () => FavorWho | null;
  /** The master opened the sheet: he acts for the player and cannot spend what is the player's. */
  readonly byMaster?: () => boolean;
  /** The circumstances the sheet knows before the roll; without them the server judges alone. */
  readonly circumstances?: () => FavorCircumstances | null;
  /** The text of a pass that waits for the master ("Você pediu para passar a Inspiração a Tor. Cancele o pedido para usar."),
   * or `''`. SLICE 3 EXTENSION POINT: the pass is not built (`PassFavor` is unimplemented), so no sheet sets it; when it is, the
   * box turns dashed with this reason and nothing else changes. */
  readonly passPending?: () => string;
  /** A reason that comes from the sheet itself: a mode asked of the master. */
  readonly sheetReason?: () => string;
  /** The tick changed: the request is another one, so a sheet that keeps one key for a retry makes a new one. */
  readonly changed?: () => void;
}

/**
 * The spend of the master's Inspiração on one roll, as a roll sheet holds it (581-03, 581-07): whether the box shows and how, the
 * tick, and the mode the roll reads while it is ticked. The sheet sends `spendFavor: favor.spend()` and no `rollMode` while it is
 * ticked (the server refuses a worse mode beside the spend), and words a refusal with `refused`.
 */
export class FavorSpend {
  /** The box is ticked. Only counts while it can be (`spend`). */
  readonly ticked = signal(false);

  readonly view = computed<FavorView | null>(() => {
    const { favor } = this.host;
    const who = this.host.who();
    if (favor.campaignId() === '' || !who) {
      return null;
    }
    if (favor.failed()) {
      return { kind: 'off', sub: '', reason: FAVOR_READ_FAILED };
    }
    const holds = who.characterId === '' ? favor.count() > 0 : favor.has(who.characterId);
    if (!holds) {
      return null;
    }
    const off = this.sheetOff(who);
    if (off) {
      return off;
    }
    const c = this.host.circumstances?.();
    if (!c) {
      return { kind: 'open', mode: null, line: '' };
    }
    const j = judgeFavor(c);
    return j.ok
      ? { kind: 'open', mode: j.mode, line: j.line }
      : { kind: 'off', sub: '', reason: j.reason };
  });

  /** The reasons the sheet itself knows to dash the box: the master acts for the player, a pass waits (slice 3), a mode was asked. */
  private sheetOff(who: FavorWho): FavorView | null {
    if (this.host.byMaster?.()) {
      const name = who.name || this.host.favor.holders().get(who.characterId) || '';
      return { kind: 'off', sub: favorByMasterText(name), reason: FAVOR_ONLY_PLAYER };
    }
    const reason = (this.host.passPending?.() ?? '') || (this.host.sheetReason?.() ?? '');
    return reason === '' ? null : { kind: 'off', sub: '', reason };
  }

  /** The box is ticked and can be: the request carries `spend_favor`. */
  readonly spend = computed(() => this.ticked() && this.view()?.kind === 'open');

  /** The mode the roll reads while the box is ticked, `null` when it is not (or the sheet does not know). */
  readonly mode = computed<RollMode | null>(() => {
    const v = this.view();
    return this.spend() && v?.kind === 'open' ? v.mode : null;
  });

  /** What the roll-mode segment wears while the box is ticked and the mode is known: it only reads the mode (`app-roll-mode-picker`). */
  readonly lock = computed(() => {
    const v = this.view();
    const mode = this.mode();
    return v?.kind === 'open' && mode !== null
      ? { mode, word: FAVOR_LOCK_WORD, line: v.line }
      : null;
  });

  constructor(private readonly host: FavorSpendHost) {}

  set(ticked: boolean): void {
    this.ticked.set(ticked);
    this.host.changed?.();
  }

  /** The roll was made with the Inspiração (or a question holds it): it is spent, so the box is unticked and who has it is read
   * again, which takes the box away at once instead of on the next hint. A roll that did not use it changes nothing. */
  spent(): void {
    if (this.ticked()) {
      this.ticked.set(false);
      void this.host.favor.refresh();
    }
  }

  /** What a refused roll says when the server refused the Inspiração (any `FavorBlocked`), or `null` for another error. The box
   * is unticked, so the next try goes without it; with no Inspiração left the read is made again, which takes the box away. */
  refused(err: unknown): string | null {
    const blocked = favorBlocked(err);
    if (!blocked) {
      return null;
    }
    this.ticked.set(false);
    if (blocked.reason === FavorBlockedReason.HAS_NONE) {
      void this.host.favor.refresh();
    }
    return favorBlockedMessage(blocked.reason);
  }
}
