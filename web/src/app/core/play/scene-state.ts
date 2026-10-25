import { computed, signal } from '@angular/core';

import type { SceneClue } from '../../../gen/meurpg/maps/v1/maps_pb';
import type {
  OpenSceneInfo,
  SceneAudienceView,
  StageNpc,
} from '../../../gen/meurpg/play/v1/scene_pb';
import { formatClock } from '../../shared/session-time/session-time';
import { attemptsAnnouncement, rollAnnouncement } from './scene-view';
import { stageAnnouncement } from './stage-view';

/**
 * The scene open in the session, as this person sees it (MR-015): a signal
 * the master's blocks and the player's block read, and the words a live
 * region says when it changes. Pure TypeScript, so the rules are tested
 * without a DOM:
 *
 * - the first read says nothing (the page was just opened);
 * - a player hears "O mestre abriu uma cena: …" when one opens, and "O mestre
 *   fechou a cena." when it closes;
 * - the master hears each new roll ("Toren: Seguir os rastros dos goblins, 7,
 *   não passou");
 * - a player hears when the master gives another attempt ("O mestre deu mais
 *   uma tentativa em Resistir ao cheiro de fumaça.", MR-015);
 * - a player also hears the stage move ("Mira entrou na cena.", "Aldo fala.",
 *   MR-031);
 * - a stale answer never overwrites a newer one.
 */
export class SceneState {
  readonly scene = signal<OpenSceneInfo | null>(null);
  /** The NPCs on the scene's stage, in the order they came in (empty with no
   * scene). Part of the scene: the server clears it when the scene closes. */
  readonly stage = computed<readonly StageNpc[]>(() => this.scene()?.stage ?? []);
  /** Who is in and out of the open scene, as the master sees it (MR-015, RN-10); `null` for a player or with no scene. */
  readonly audience = computed<SceneAudienceView | null>(() => this.scene()?.audience ?? null);
  /** "Os jogadores podem entrar": a scene open to some may be joined by the others. */
  readonly playersMayJoin = computed(() => this.scene()?.playersMayJoin ?? false);
  /** "Toren entrou na cena às 20:41.": the master's line after a player joined by themselves, until the next news. */
  readonly joined = signal('');
  /** What the live region says now; changes with each news. */
  readonly notice = signal('');
  /**
   * Where focus goes next, because the master's own action replaced what had it
   * (E7-02): "title" after a scene opens or is swapped (the open scene's title),
   * "open" after it closes (the "Abrir cena" button of the panel that comes
   * back). Whoever is drawn for it takes the focus and sets this back to
   * `null`; a scene that was already open when the page loaded never sets it.
   */
  readonly focusNext = signal<'title' | 'open' | null>(null);
  /** The player came into this scene from the card "Há uma cena aberta" (JoinScene, C7-06): the scene says "Você entrou na cena". */
  readonly enteredHere = signal(false);

  private generation = 0;
  /** Moves with each local answer: a read that began before it may lack what the answer carries (or carry less
   * than the answer), so it is dropped and the scene is read once more, not lost. */
  private edits = 0;
  private loaded = false;

  constructor(
    private readonly load: () => Promise<OpenSceneInfo | null>,
    private readonly isMaster: () => boolean,
  ) {}

  /** Reads the scene again (`ready`, `scene_changed`, `scene_check_rolled`).
   * A failed read keeps the copy on screen: the next event reads it again. */
  async refresh(): Promise<void> {
    const generation = ++this.generation;
    const edits = this.edits;
    try {
      const next = await this.load();
      if (generation !== this.generation) {
        return;
      }
      if (edits !== this.edits) {
        return await this.refresh();
      }
      this.show(next);
    } catch {
      // The stream's next event, or reconnection, reads it again.
    }
  }

  /** The scene as an answer of the master's own call (open, swap, close). */
  apply(next: OpenSceneInfo | null): void {
    this.edits++;
    this.show(next);
  }

  private show(next: OpenSceneInfo | null): void {
    const prev = this.scene();
    this.scene.set(next);
    if (prev?.joinable && next && !next.joinable) {
      this.enteredHere.set(true);
    } else if (!next || next.joinable || (prev && !sameScene(prev, next))) {
      this.enteredHere.set(false);
    }
    if (!next || !prev || !sameScene(prev, next)) {
      this.joined.set('');
    }
    if (this.loaded) {
      this.say(prev, next);
    }
    this.loaded = true;
  }

  /** A clue the master just revealed here: the open scene carries it with the
   * players who have it now (the stream's `scene_changed` reads the same). */
  clueRevealed(clue: SceneClue): void {
    const scene = this.scene();
    if (!scene) {
      return;
    }
    this.edits++;
    this.scene.set({ ...scene, clues: scene.clues.map((c) => (c.id === clue.id ? clue : c)) });
  }

  /** The master's own eye on an action answered (`UpdateSceneAction`): the open scene carries it at once. */
  setActionHidden(actionId: string, hidden: boolean): void {
    const scene = this.scene();
    if (!scene) {
      return;
    }
    this.edits++;
    this.scene.set({
      ...scene,
      actions: scene.actions.map((a) => (a.id === actionId ? { ...a, hidden } : a)),
    });
  }

  /** The master's own stage call answered: the stage as it is now. Applied in
   * place, so the cards change at once; a read still on its way is dropped and made again. */
  setStage(stage: readonly StageNpc[]): void {
    const scene = this.scene();
    if (!scene) {
      return;
    }
    this.edits++;
    this.scene.set({ ...scene, stage: [...stage] });
  }

  /** The master opened (or swapped) the scene here: the title takes focus. */
  openedHere(next: OpenSceneInfo): void {
    this.apply(next);
    this.focusNext.set('title');
  }

  /** The master closed the scene here: "Abrir cena" takes focus. */
  closedHere(): void {
    this.apply(null);
    this.focusNext.set('open');
  }

  /** A new session or page: nothing is open, and the next read is silent. */
  clear(): void {
    this.generation++;
    this.scene.set(null);
    this.notice.set('');
    this.focusNext.set(null);
    this.enteredHere.set(false);
    this.loaded = false;
    this.joined.set('');
  }

  private say(prev: OpenSceneInfo | null, next: OpenSceneInfo | null): void {
    const news = this.isMaster()
      ? masterNews(prev, next)
      : { notice: playerNotice(prev, next), joined: '' };
    if (news.joined) {
      this.joined.set(news.joined);
    }
    if (news.notice) {
      this.notice.set(news.notice);
    }
  }
}

/** What a read says: the live region's words and, for the master, the line "<nome> entrou na cena às 20:41." (empty when nobody joined). */
interface News {
  readonly notice: string;
  readonly joined: string;
}

/** What the master hears: each new roll of the same scene, and a player who joined by themselves (JoinScene). */
function masterNews(prev: OpenSceneInfo | null, next: OpenSceneInfo | null): News {
  if (!next || !prev || !sameScene(prev, next)) {
    return { notice: '', joined: '' };
  }
  const known = new Set(prev.rolls.map((r) => r.id));
  const fresh = next.rolls.filter((r) => !known.has(r.id)).reverse();
  const came = joinedByThemselves(prev, next).join(' e ');
  return {
    notice: came
      ? `${came} entrou na cena.`
      : fresh.map((r) => rollAnnouncement(next, r)).join('. '),
    joined: came ? `${came} entrou na cena às ${formatClock(new Date())}.` : '',
  };
}

/** The words for a scene a player may join: the card comes, gives way to the scene (entered) or goes (the switch went off). */
function joinNotice(prev: OpenSceneInfo | null, next: OpenSceneInfo | null): string {
  if (next?.joinable && !prev?.joinable) {
    return `Há uma cena aberta: ${next.name}. Você pode entrar.`;
  }
  if (prev?.joinable && next && !next.joinable) {
    return `Você entrou na cena: ${next.name}.`;
  }
  return prev?.joinable && !next ? 'A cena não está mais aberta para entrar.' : '';
}

/** What a player's live region says of a new read: the scene opening, closing, being enterable or entered, the stage and the attempts. */
function playerNotice(prev: OpenSceneInfo | null, next: OpenSceneInfo | null): string {
  const join = joinNotice(prev, next);
  if (join) {
    return join;
  }
  if (next && !next.joinable && (!prev || !sameScene(prev, next))) {
    return `O mestre abriu uma cena: ${next.name}.`;
  }
  if (!next && prev) {
    return 'O mestre fechou a cena.';
  }
  if (next && prev) {
    return attemptsAnnouncement(prev, next) || stageAnnouncement(prev.stage, next.stage);
  }
  return '';
}

/** The names of the players who joined by themselves since `prev` (JoinScene), in the party's order. */
function joinedByThemselves(prev: OpenSceneInfo, next: OpenSceneInfo): string[] {
  const known = new Set(prev.audience?.inside.map((p) => p.userId));
  return (next.audience?.inside ?? [])
    .filter((p) => p.joinedBySelf && !known.has(p.userId))
    .map((p) => p.characterName);
}

/** The same opening of the same scene (closing and opening it again is another). */
function sameScene(a: OpenSceneInfo, b: OpenSceneInfo): boolean {
  return (
    a.pointId === b.pointId &&
    a.openedAt?.seconds === b.openedAt?.seconds &&
    a.openedAt?.nanos === b.openedAt?.nanos
  );
}
