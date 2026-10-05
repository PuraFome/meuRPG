import {
  Component,
  ElementRef,
  afterNextRender,
  computed,
  effect,
  input,
  output,
  signal,
  untracked,
  viewChild,
} from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';

import type { Encounter, GetMoveOptionsResponse } from '../../../../../gen/meurpg/play/v1/combat_pb';
import type { JumpLimits } from '../../../../../gen/meurpg/rules/v1/rules_pb';
import { type Square, stepSquare } from '../../../../core/combat/combat-grid';
import {
  HEIGHT_STEP_DFT,
  type JumpMode,
  lineLengthDft,
  limitFor,
  maxHeight,
  stepHeight,
} from '../../../../core/combat/jump-plan';
import {
  afterText,
  costTitle,
  indexOptions,
  leftLine,
  provokeWarning,
  provokedBy,
  refusalText,
  trapQuestion,
  verdictFor,
} from '../../../../core/combat/move-plan';
import { ownCombatant, roundLabel } from '../../../../core/combat/combat-view';
import { type MapLayers, NO_LAYERS } from '../../../../core/maps/layers';
import { metersFixed, reachSquares } from '../../../../core/units';
import { CombatMap, type CombatMapImage, type Reach } from '../../../../shared/combat-map/combat-map';
import { LivePill } from '../../../../shared/live-pill/live-pill';
import { MapLayersLegend } from '../../../../shared/map-layers/map-layers-legend';
import { PHONE_QUERY, mediaQuery } from '../../../../shared/map-view/media-query';
import { JumpPanel } from './jump-panel';
import { MoveAdjust } from './move-adjust';
import { MoveStatus, type MoveSummary } from './move-status';
import { type Segment, Segmented } from './segmented';

/** Sides of a square on this page, in pixels: the map is zoomed in so a thumb hits one (E9-05: 30 px). */
const ZOOMS = [22, 30, 40, 52] as const;
const DEFAULT_ZOOM = 1;

/** What "Saltar para cá" asks for. */
export type JumpRequest =
  | { readonly kind: 'long'; readonly square: Square }
  | { readonly kind: 'high'; readonly heightDft: number };

/**
 * "Mover" and "Saltar" (E6-10, E9-05, E9-06, E9-13; RN-21, MR-034): a full page
 * where the map is the control. The reach comes from the server
 * (`GetMoveOptions`): the squares it can go to are tinted inside the dashed
 * circle of the movement left, each with its cost, and a square it refuses says
 * why (wall, enemy, taken, too costly) without naming what is in the way. The
 * browser only draws and words that: it never works out a cost, a wall or a
 * cover. The player taps a square (or moves the choice with the arrows below
 * the map, or the arrow keys), reads the move in a live region and confirms with
 * "Mover para cá", under the map, where the thumb is. A move that may provoke an
 * opportunity attack warns first, and so does one into a trap the character
 * knows. "Saltar" swaps the walk for a long jump (a square, inside the circle of
 * the server's limit) or a high one (a height, in 0,3 m steps).
 */
@Component({
  selector: 'app-move-page',
  imports: [CombatMap, JumpPanel, LivePill, MapLayersLegend, MatButtonModule, MatIconModule, MoveAdjust, MoveStatus, Segmented],
  templateUrl: './move-page.html',
  styleUrl: './move-page.scss',
})
export class MovePage {
  readonly encounter = input.required<Encounter>();
  readonly image = input.required<CombatMapImage>();
  readonly mapName = input('');
  readonly sessionNumber = input(0);
  /** "Ver mapa": the same page without the reach and the buttons. */
  readonly readOnly = input(false);
  /** The master's "Abrir mapa": hidden combatants are drawn too. */
  readonly isMaster = input(false);
  readonly busy = input(false);
  /** A refusal from the server ("Longe demais…"), shown like a local one. */
  readonly serverError = input('');
  /** Where the combatant can go (`GetMoveOptions`); `null` until it is read, or when it failed. */
  readonly options = input<GetMoveOptionsResponse | null>(null);
  readonly optionsFailed = input(false);
  /** The map's layers, once read. */
  readonly layers = input<MapLayers>(NO_LAYERS);
  /** How far and how high it can jump this turn; unset when the sheet has no Força. */
  readonly jumps = input<JumpLimits | undefined>(undefined);
  /** "Desengajar" is still possible: the action is free and it was not taken. */
  readonly canDisengage = input(false);

  /** "Mover para cá": move the combatant to this square. */
  readonly confirm = output<Square>();
  /** "Saltar para cá" / "Saltar 1,8 m para cima". */
  readonly jump = output<JumpRequest>();
  /** "Desengajar (gasta a ação)": takes the action here, so the warning goes. */
  readonly disengage = output<void>();
  /** "Cancelar", or the back arrow. */
  readonly back = output<void>();

  protected readonly chosen = signal<Square | null>(null);
  protected readonly mode = signal<'walk' | 'jump'>('walk');
  protected readonly kind = signal<JumpMode>('long');
  protected readonly height = signal(0);
  protected readonly zoom = signal(DEFAULT_ZOOM);
  /** The question about the known trap is open in the footer. */
  protected readonly trapAsk = signal(false);
  private readonly errorFor = signal<Square | null>(null);
  private readonly scroller = viewChild.required<ElementRef<HTMLElement>>('scroller');
  private readonly safe = viewChild('safe', { read: ElementRef<HTMLButtonElement> });

  protected readonly phone = mediaQuery(PHONE_QUERY);
  /** From 1024px the map fits the left column and the controls sit beside it. */
  protected readonly wide = mediaQuery('(min-width: 1024px)');
  protected readonly cell = computed(() => ZOOMS[this.zoom()]);
  protected readonly round = computed(() => roundLabel(this.encounter().round));
  protected readonly own = computed(() => ownCombatant(this.encounter()));
  protected readonly origin = computed<Square>(() => {
    const own = this.own();
    return own ? { col: own.col, row: own.row } : { col: 0, row: 0 };
  });
  protected readonly leftDft = computed(() => this.options()?.movementLeftDft ?? this.own()?.movementLeftDft ?? 0);
  protected readonly totalDft = computed(() => (this.own()?.speedDft ?? 0) * (this.own()?.dashed ? 2 : 1));
  protected readonly usedDft = computed(() => this.own()?.movementUsedDft ?? 0);
  protected readonly jumping = computed(() => this.mode() === 'jump' && !!this.jumps());
  protected readonly modes: readonly Segment<'walk' | 'jump'>[] = [
    { value: 'walk', label: 'Andar', icon: 'directions_walk' },
    { value: 'jump', label: 'Saltar', icon: 'north_east' },
  ];
  protected readonly kinds: readonly Segment<JumpMode>[] = [
    { value: 'long', label: 'Distância', icon: 'straighten' },
    { value: 'high', label: 'Altura', icon: 'height' },
  ];

  protected readonly index = computed(() => indexOptions(this.options()));
  protected readonly verdict = computed(() => {
    const to = this.chosen();
    return to ? verdictFor(this.index(), this.origin(), to) : null;
  });
  /** The limit of the jump that is picked (the server's, running or standing). */
  protected readonly limit = computed(() => {
    const jumps = this.jumps();
    return jumps ? limitFor(jumps, this.kind()) : 0;
  });
  protected readonly atMin = computed(() => this.height() <= Math.min(HEIGHT_STEP_DFT, maxHeight(this.limit())));
  protected readonly atMax = computed(() => this.height() >= maxHeight(this.limit()));
  /** The line a long jump draws, to say what it costs (display only; the server decides). */
  protected readonly jumpCost = computed(() => {
    const to = this.chosen();
    return to ? lineLengthDft(this.origin(), to) : 0;
  });

  /** The reach the map draws: the walk's squares, or the circle of the jump. */
  protected readonly reach = computed<Reach | null>(() => {
    if (this.readOnly()) {
      return null;
    }
    if (this.jumping()) {
      return this.kind() === 'long' ? { origin: this.origin(), leftDft: this.limit(), squares: [] } : null;
    }
    const options = this.options();
    return {
      origin: this.origin(),
      leftDft: this.leftDft(),
      squares: (options?.reachable ?? []).map((s) => ({ col: s.col, row: s.row })),
    };
  });

  /** The move in words, from the options' cost. */
  protected readonly summary = computed<MoveSummary>(() => {
    const none: MoveSummary = { kind: 'idle', title: '', detail: '', warning: '', trap: '' };
    if (this.jumping()) {
      return this.jumpSummary(none);
    }
    const to = this.chosen();
    const v = this.verdict();
    if (!to || !v) {
      return none;
    }
    if (v.kind === 'ok') {
      const names = provokedBy(v.square, this.encounter().combatants);
      return {
        kind: 'ok',
        title: costTitle(v.square.costDft),
        detail: afterText(this.leftDft(), v.square.costDft),
        warning: names.length > 0 ? provokeWarning(names) : '',
        trap: v.square.knownTrapName,
      };
    }
    const text = refusalText(v, this.leftDft());
    return text ? { kind: 'refused', ...text, warning: '', trap: '' } : none;
  });
  /** The frame on the chosen square: the cost beside it when the move can be made, the
   * refusal's title ("Sem caminho") when it cannot. */
  protected readonly frame = computed(() => {
    const square = this.chosen();
    if (!square || (this.jumping() && this.kind() === 'high')) {
      return null;
    }
    const s = this.summary();
    if (s.kind !== 'ok') {
      return { square, refused: true, label: s.title || undefined };
    }
    const v = this.verdict();
    const dft = this.jumping() ? this.jumpCost() : v?.kind === 'ok' ? v.square.costDft : 0;
    return { square, refused: false, label: metersFixed(dft / 10) };
  });
  protected readonly canMove = computed(() => {
    if (this.busy()) {
      return false;
    }
    if (this.jumping()) {
      return this.kind() === 'high' ? this.height() > 0 : this.chosen() !== null && this.summary().kind === 'ok';
    }
    return this.verdict()?.kind === 'ok';
  });
  protected readonly visibleError = computed(() => {
    const at = this.errorFor();
    const to = this.chosen();
    return this.serverError() !== '' && ((!at && !to) || (!!at && !!to && at.col === to.col && at.row === to.row));
  });
  protected readonly title = computed(() => `${this.jumping() ? 'Saltar' : 'Mover'} ${this.own()?.label ?? ''}`);
  protected readonly lead = computed(() => {
    const ask = this.jumping() ? (this.kind() === 'long' ? 'Toque no quadrado onde quer cair.' : '') : 'Toque num quadrado destacado.';
    const squares = this.jumping() ? null : reachSquares(this.leftDft() / 10);
    return [leftLine(this.leftDft(), this.totalDft(), this.usedDft(), squares), ask].filter(Boolean).join(' ');
  });
  protected readonly leftText = computed(() => metersFixed(this.leftDft() / 10));
  protected readonly limitText = computed(() => metersFixed(this.limit() / 10));
  protected readonly goLabel = computed(() => {
    if (!this.jumping()) {
      return 'Mover para cá';
    }
    return this.kind() === 'long' ? 'Saltar para cá' : `Saltar ${metersFixed(this.height() / 10)} para cima`;
  });
  /** What the footer's button says it is for when it cannot be pressed. */
  protected readonly trapName = computed(() => {
    const v = this.verdict();
    return v?.kind === 'ok' ? v.square.knownTrapName : '';
  });
  protected readonly trapText = computed(() => trapQuestion(this.trapName()));

  constructor() {
    // The map is wider than the screen: open with the token in view.
    afterNextRender(() => {
      const el = this.scroller().nativeElement;
      const x = (this.origin().col + 0.5) * this.cell() - el.clientWidth / 2;
      el.scrollLeft = Math.max(0, x);
    });
    // A refusal belongs to the square it came for.
    effect(() => {
      if (this.serverError()) {
        const at = untracked(() => this.chosen());
        this.errorFor.set(at);
      }
    });
    // The high jump starts at the most it can do (E9-06: 1,8 m).
    effect(() => {
      const max = maxHeight(this.limit());
      untracked(() => this.height.set(max));
    });
    // The question about a trap closes when another square is chosen, and
    // opens with the focus on the safe answer.
    effect(() => {
      this.chosen();
      untracked(() => this.trapAsk.set(false));
    });
    effect(() => {
      if (this.trapAsk()) {
        queueMicrotask(() => this.safe()?.nativeElement.focus());
      }
    });
  }

  private jumpSummary(none: MoveSummary): MoveSummary {
    const left = this.leftDft();
    if (this.kind() === 'high') {
      const h = this.height();
      return h > 0
        ? {
            kind: 'ok',
            title: `Saltar ${metersFixed(h / 10)}`,
            detail: `Você sobe sem mudar de quadrado, para agarrar uma borda. ${afterText(left, h)}`,
            warning: '',
            trap: '',
          }
        : { kind: 'refused', title: 'Sem salto em altura', detail: 'Com essa Força, parado, não dá para subir nem um passo de 0,3 m.', warning: '', trap: '' };
    }
    const to = this.chosen();
    if (!to) {
      return none;
    }
    if (to.col === this.origin().col && to.row === this.origin().row) {
      return { kind: 'refused', title: 'Você já está aqui', detail: 'Toque no quadrado onde quer cair.', warning: '', trap: '' };
    }
    const cost = this.jumpCost();
    return {
      kind: 'ok',
      title: `Saltar ${metersFixed(cost / 10)}`,
      detail: `O terreno difícil no caminho não conta. ${afterText(left, cost)}`,
      warning: '',
      trap: '',
    };
  }

  protected choose(square: Square): void {
    if (this.jumping() && this.kind() === 'high') {
      return;
    }
    this.chosen.set(square);
  }

  protected nudge(key: string): void {
    const e = this.encounter();
    this.choose(stepSquare(this.chosen() ?? this.origin(), key, e.gridColumns, e.gridRows));
  }

  protected setMode(mode: 'walk' | 'jump'): void {
    this.mode.set(mode);
    this.chosen.set(null);
  }

  protected setKind(kind: JumpMode): void {
    this.kind.set(kind);
    this.chosen.set(null);
    this.height.set(maxHeight(limitFor(this.jumps()!, kind)));
  }

  protected step(direction: 1 | -1): void {
    this.height.update((h) => stepHeight(h, direction, this.limit()));
  }

  protected zoomBy(delta: 1 | -1): void {
    this.zoom.update((z) => Math.min(ZOOMS.length - 1, Math.max(0, z + delta)));
  }

  protected go(): void {
    if (!this.canMove()) {
      return;
    }
    if (this.jumping()) {
      const to = this.chosen();
      if (this.kind() === 'high') {
        this.jump.emit({ kind: 'high', heightDft: this.height() });
      } else if (to) {
        this.jump.emit({ kind: 'long', square: to });
      }
      return;
    }
    const to = this.chosen();
    if (!to) {
      return;
    }
    if (this.trapName() && !this.trapAsk()) {
      this.trapAsk.set(true);
      return;
    }
    this.confirm.emit(to);
  }
}
