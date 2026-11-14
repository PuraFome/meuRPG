import {
  CombatLogEffectChange,
  type CombatLogEntry,
  CombatLogKind,
} from '../../../gen/meurpg/play/v1/combat_pb';
import { tight } from '../format/text';
import { ofSubject, the } from './combat-log-words';

/**
 * The notice above the log when a concentration ended before the caster's next weapon hit spent the effect it held ("A sua
 * concentração acabou. O Clarão na Lâmina acabou sem golpe."). The caster's player and the master read it; another player
 * reads the log's line alone. What it says is the server's (the effect's `unused` and `reason`, and the caster the line
 * names); nothing here works out a rule.
 */

/** The effect that lapsed: the spell and the caster it was kept by (a label). */
export interface Lapse {
  readonly spell: string;
  readonly caster: string;
}

/** Who is reading: the master, or the player whose character is `ownLabel`. */
export interface LapseViewer {
  readonly master: boolean;
  readonly ownLabel: string;
  /** The labels of the player characters in the combat, for how the master's text names the caster. */
  readonly players?: ReadonlySet<string>;
}

/** "A sua concentração acabou. O Clarão na Lâmina acabou sem golpe." for the caster's player; the master's names the caster. */
export function lapseNoticeText(lapse: Lapse, viewer: LapseViewer): string {
  const spell = lapse.spell ? the(lapse.spell) : 'o efeito';
  const cap = spell.charAt(0).toUpperCase() + spell.slice(1);
  if (!viewer.master) {
    return tight(`A sua concentração acabou. ${cap} acabou sem golpe.`);
  }
  const of = ofSubject(lapse.caster, { master: true, players: viewer.players ?? new Set() });
  return tight(`A concentração ${of} acabou. ${cap} ${of} acabou sem golpe.`);
}

/**
 * Reads the combat log for the end of a concentration that left an on-hit effect without its hit. The lines the log held when
 * the combat was first read are not news (a reconnect or a re-read never repeats a notice), and another combat starts clean.
 */
export class LapseNotices {
  private seen: Set<string> | null = null;
  private combat = '';

  /** `reset` says the combat changed (clear what was on screen); `notice` is the new lapse to tell, if any. */
  read(
    encounterId: string,
    loaded: boolean,
    entries: readonly CombatLogEntry[],
    viewer: LapseViewer,
  ): { reset: boolean; notice: Lapse | null } {
    const reset = encounterId !== this.combat;
    if (reset) {
      this.combat = encounterId;
      this.seen = null;
    }
    if (!loaded) {
      return { reset, notice: null };
    }
    const lines = entries.filter((e) => isLapse(e));
    if (this.seen === null) {
      this.seen = new Set(lines.map((l) => l.id));
      return { reset, notice: null };
    }
    let notice: Lapse | null = null;
    for (const line of lines) {
      if (this.seen.has(line.id)) {
        continue;
      }
      this.seen.add(line.id);
      // An on-hit effect's caster is its only target (the server writes the caster as the label).
      const caster = line.effect?.targetLabels[0] ?? '';
      if (caster && (viewer.master || (viewer.ownLabel !== '' && caster === viewer.ownLabel))) {
        notice = { spell: line.effect?.sourceNamePt ?? '', caster };
      }
    }
    return { reset, notice };
  }
}

function isLapse(e: CombatLogEntry): boolean {
  const fx = e.effect;
  return (
    e.kind === CombatLogKind.EFFECT &&
    !!fx &&
    fx.change === CombatLogEffectChange.ENDED &&
    fx.unused &&
    fx.reason === 'concentration'
  );
}
