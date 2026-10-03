import { Component, computed, effect, inject, input, output, signal } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatDialog } from '@angular/material/dialog';
import { MatIconModule } from '@angular/material/icon';
import { Code, ConnectError } from '@connectrpc/connect';

import type { DiceMode, DicePreference } from '../../../../gen/meurpg/campaigns/v1/campaigns_pb';
import { CharacterKind } from '../../../../gen/meurpg/characters/v1/characters_pb';
import { type Encounter, EncounterStatus } from '../../../../gen/meurpg/play/v1/combat_pb';
import { CombatClient } from '../../../core/combat/combat-client';
import { combatErrorMessage } from '../../../core/combat/combat-errors';
import { type Square, canReach, reachSquares } from '../../../core/combat/combat-grid';
import type { CombatState } from '../../../core/combat/combat-state';
import { ownCombatant, npcKindLabel } from '../../../core/combat/combat-view';
import type { MapState } from '../../../core/maps/map-state';
import { MoveSaves } from '../../../core/maps/move-saves';
import { RosterClient } from '../../../core/maps/roster-client';
import type { TokenDrop } from '../../../shared/combat-map/combat-map';
import { PHONE_QUERY, mediaQuery } from '../../../shared/map-view/media-query';
import type { PartyMemberInfoVm, VitalsVm } from '../live-session.types';
import type { CombatantInfo } from './combat-info';
import { CombatBar } from './combat-bar/combat-bar';
import { CombatMapCard } from './combat-map-card/combat-map-card';
import { CombatSummary } from './combat-summary/combat-summary';
import { InitiativeSetup } from './initiative-setup/initiative-setup';
import { InitiativeSide } from './initiative-side/initiative-side';
import { MovePage } from './move-page/move-page';
import { OrderList } from './order-list/order-list';
import { PlayerInitiative } from './player-initiative/player-initiative';
import { type StartCombatData, StartCombatDialog } from './start-combat/start-combat-dialog';
import { TurnBar } from './turn-panel/turn-bar';
import { TurnPanel } from './turn-panel/turn-panel';

/**
 * The combat on the session page (MR-013, E6-01 to E6-16): it picks the
 * screen by who is looking and where the combat is, and runs every call.
 *
 * - **Master:** SETUP is the initiative list with its side panel (E6-04);
 *   ACTIVE is the combat bar, the map and the order (E6-11, E6-12); ENDED is
 *   the summary (E6-16).
 * - **Player:** SETUP is "Role a iniciativa" (E6-03); ACTIVE is the turn
 *   banner with the order strip, the map and, projected in, the vitals
 *   (E6-05), or the "Mover" page (E6-10); ENDED is the summary.
 *
 * It never reads the combat by itself: the page does, after
 * `encounter_changed`. A call's answer is the combat as it is now, so it is
 * applied at once; a refusal that means "the screen is stale" reads it again.
 */
@Component({
  selector: 'app-combat-view',
  imports: [
    CombatBar,
    CombatMapCard,
    CombatSummary,
    InitiativeSetup,
    InitiativeSide,
    MatButtonModule,
    MatIconModule,
    MovePage,
    OrderList,
    PlayerInitiative,
    TurnBar,
    TurnPanel,
  ],
  templateUrl: './combat-view.html',
  styleUrl: './combat-view.scss',
})
export class CombatView {
  private readonly api = inject(CombatClient);
  private readonly roster = inject(RosterClient);
  private readonly dialog = inject(MatDialog);
  protected readonly phone = mediaQuery(PHONE_QUERY);
  /** One save in flight per combatant (see `MoveSaves`). */
  private readonly moves = new MoveSaves();

  readonly campaignId = input.required<string>();
  readonly isMaster = input(false);
  readonly state = input.required<CombatState>();
  readonly mapState = input.required<MapState>();
  readonly vitals = input<readonly VitalsVm[]>([]);
  readonly partyInfo = input<ReadonlyMap<string, PartyMemberInfoVm>>(new Map());
  readonly sessionNumber = input(0);
  readonly diceMode = input.required<DiceMode>();
  readonly dicePreference = input.required<DicePreference>();

  /** "Dano/Cura" on a player's row: the page opens its adjust dialog. */
  readonly adjust = output<VitalsVm>();
  /** "Mudar" (how the player rolls). */
  readonly changeDice = output<void>();

  protected readonly Status = EncounterStatus;
  protected readonly busy = signal(false);
  protected readonly error = signal('');
  /** A refusal of the move on the "Mover" page (TOO_FAR, SQUARE_OCCUPIED). */
  protected readonly moveError = signal('');
  protected readonly rosterInfo = signal<ReadonlyMap<string, CombatantInfo>>(new Map());

  protected readonly encounter = computed(() => this.state().shown());
  protected readonly image = computed(() => {
    const e = this.encounter();
    const map = this.mapState().map();
    return e && map && map.id === e.mapId && map.image
      ? { url: map.image.url, width: map.image.width, height: map.image.height }
      : null;
  });
  protected readonly mapName = computed(() => this.mapState().map()?.name ?? '');
  protected readonly info = computed<ReadonlyMap<string, CombatantInfo>>(() => {
    const merged = new Map(this.rosterInfo());
    // A player has no roster: their own combatant says what the table needs.
    for (const [id, p] of this.partyInfo()) {
      if (!merged.has(id)) {
        merged.set(id, { classSummary: p.classSummary, playerName: p.playerName, kindLabel: '', raceName: '' });
      }
    }
    return merged;
  });
  /** The characters whose "Dano/Cura" has a vitals row to adjust. */
  protected readonly adjustable = computed(() => new Set(this.vitals().map((v) => v.characterId)));
  protected readonly own = computed(() => {
    const e = this.encounter();
    return e ? ownCombatant(e) : null;
  });
  protected readonly myTurn = computed(() => {
    const e = this.encounter();
    return !!e && !!this.own() && e.currentCombatantId === this.own()?.id;
  });
  /** The squares a player's own token may be dragged to, on their turn. */
  protected readonly reach = computed(() => {
    const own = this.own();
    return this.myTurn() && own && own.placed
      ? { origin: { col: own.col, row: own.row }, squares: reachSquares(own.movementLeftFt) }
      : null;
  });
  protected readonly unplaced = computed(() =>
    (this.encounter()?.combatants ?? []).filter((c) => !c.placed),
  );
  protected readonly setup = computed(() => this.encounter()?.status === EncounterStatus.SETUP);
  protected readonly active = computed(() => this.encounter()?.status === EncounterStatus.ACTIVE);
  protected readonly ended = computed(() => this.encounter()?.status === EncounterStatus.ENDED);

  constructor() {
    // The master's lists need the roster (the kind of each NPC, the class
    // line, the player's name).
    effect(() => {
      if (this.isMaster() && this.campaignId()) {
        void this.loadRoster(this.campaignId());
      }
    });
    // "Mover" belongs to the player's turn: when it ends, the page closes.
    effect(() => {
      if (!this.myTurn() || !this.active()) {
        this.state().moving.set(false);
      }
      if (!this.active()) {
        this.state().mapOpen.set(false);
      }
    });
  }

  private async loadRoster(campaignId: string): Promise<void> {
    try {
      const entries = await this.roster.list(campaignId);
      this.rosterInfo.set(
        new Map(
          entries.map((e) => [
            e.id,
            {
              classSummary: e.classSummary,
              playerName: e.playerName,
              kindLabel: e.kind === CharacterKind.PLAYER ? '' : npcKindLabel(e.kind),
              raceName: e.raceName,
            },
          ]),
        ),
      );
    } catch {
      // Best effort: the lists fall back to what the combat itself says.
    }
  }

  // ---- calls ----

  /** Runs a call that answers with the combat: applies it, or says why not.
   * A refusal that means the screen is stale reads the combat again. */
  private async run(call: (e: Encounter) => Promise<Encounter>): Promise<boolean> {
    const e = this.encounter();
    if (!e || this.busy()) {
      return false;
    }
    this.busy.set(true);
    this.error.set('');
    try {
      this.state().apply(await call(e));
      return true;
    } catch (err) {
      this.error.set(combatErrorMessage(err));
      await this.refreshAfter(err);
      return false;
    } finally {
      this.busy.set(false);
    }
  }

  private async refreshAfter(err: unknown): Promise<void> {
    const code = ConnectError.from(err).code;
    if (code === Code.Aborted || code === Code.FailedPrecondition || code === Code.NotFound) {
      try {
        this.state().apply(await this.api.get(this.campaignId()));
      } catch {
        // The stream's next `ready` reads it again.
      }
    }
  }

  protected begin(): Promise<boolean> {
    return this.run((e) => this.api.begin(this.campaignId(), e.id));
  }

  protected nextTurn(): Promise<boolean> {
    return this.run((e) => this.api.endTurn(this.campaignId(), e.id, e.currentCombatantId));
  }

  protected endCombat(): Promise<boolean> {
    return this.run((e) => this.api.end(this.campaignId(), e.id));
  }

  protected submitFace(combatantId: string, face: number): Promise<boolean> {
    return this.run((e) => this.api.submitInitiative(this.campaignId(), e.id, combatantId, { face }));
  }

  protected rollInApp(combatantId: string): Promise<boolean> {
    return this.run((e) => this.api.submitInitiative(this.campaignId(), e.id, combatantId, { inApp: true }));
  }

  protected setOrder(ids: string[]): Promise<boolean> {
    return this.run((e) => this.api.setOrder(this.campaignId(), e.id, ids));
  }

  protected reveal(change: { id: string; hidden: boolean }): Promise<boolean> {
    return this.run((e) => this.api.setHidden(this.campaignId(), e.id, change.id, change.hidden));
  }

  protected remove(id: string): Promise<boolean> {
    return this.run((e) => this.api.remove(this.campaignId(), e.id, id));
  }

  protected adjustFor(characterId: string): void {
    const v = this.vitals().find((x) => x.characterId === characterId);
    if (v) {
      this.adjust.emit(v);
    }
  }

  /** "Adicionar combatente": the start dialog, with the NPCs only. */
  protected addCombatants(): void {
    const e = this.encounter();
    if (!e) {
      return;
    }
    const phone = this.phone();
    this.dialog
      .open<StartCombatDialog, StartCombatData, Encounter>(StartCombatDialog, {
        data: {
          campaignId: this.campaignId(),
          mode: 'add',
          map: null,
          encounterId: e.id,
          existing: e.combatants.length,
        },
        width: phone ? '100vw' : '760px',
        maxWidth: phone ? '100vw' : 'calc(100vw - 32px)',
        height: phone ? '100dvh' : undefined,
        maxHeight: phone ? '100dvh' : '92dvh',
        ariaLabelledBy: 'start-title',
        autoFocus: 'first-tabbable',
      })
      .afterClosed()
      .subscribe((added) => added && this.state().apply(added));
  }

  // ---- moving ----

  /** A token dropped on a square (the master's drag, or a player's inside
   * their reach). The token is where it was dropped at once; if the server
   * refuses, it goes back and says why. */
  protected async drop(drop: TokenDrop): Promise<void> {
    const e = this.encounter();
    const c = e?.combatants.find((x) => x.id === drop.id);
    if (!e || !c) {
      return;
    }
    const from: Square = { col: c.col, row: c.row };
    if (c.placed && from.col === drop.col && from.row === drop.row) {
      return;
    }
    // A player is checked here first: a drop beyond the reach goes nowhere.
    if (!this.isMaster() && !this.reachable(c.id, drop)) {
      this.error.set('Longe demais ou ocupado: solte o token num quadrado destacado.');
      return;
    }
    this.error.set('');
    this.state().applyMove({ encounterId: e.id, combatantId: c.id, col: drop.col, row: drop.row });
    // One save at a time per combatant: `MoveSaves` is about positions, and
    // here a square's column and row stand in for x and y.
    await this.moves.move(
      `${e.id}/${c.id}`,
      { xBp: from.col, yBp: from.row },
      { xBp: drop.col, yBp: drop.row },
      {
        save: async (to) =>
          this.state().apply(await this.api.move(this.campaignId(), e.id, c.id, to.xBp, to.yBp)),
        failed: (saved, err) => {
          this.state().applyMove({ encounterId: e.id, combatantId: c.id, col: saved.xBp, row: saved.yBp });
          this.error.set(combatErrorMessage(err, 'mover o token'));
          void this.refreshAfter(err);
        },
      },
    );
  }

  private reachable(id: string, to: Square): boolean {
    const e = this.encounter();
    const c = e?.combatants.find((x) => x.id === id);
    if (!e || !c) {
      return false;
    }
    const others = e.combatants.filter((x) => x.placed && x.id !== id).map((x) => ({ col: x.col, row: x.row }));
    return canReach({ col: c.col, row: c.row }, to, reachSquares(c.movementLeftFt), e.gridColumns, e.gridRows, others);
  }

  /** "Mover para cá" on the "Mover" page. */
  protected async confirmMove(to: Square): Promise<void> {
    const own = this.own();
    if (!own) {
      return;
    }
    this.moveError.set('');
    const ok = await this.run((e) => this.api.move(this.campaignId(), e.id, own.id, to.col, to.row));
    if (ok) {
      this.state().moving.set(false);
    } else {
      // The refusal's own words, next to the map where the person looks.
      this.moveError.set(this.error());
      this.error.set('');
    }
  }

  protected closeFullPage(): void {
    this.state().moving.set(false);
    this.state().mapOpen.set(false);
  }

  protected openMove(): void {
    this.moveError.set('');
    this.state().moving.set(true);
  }

  /** The first free square nearest the middle of the map: where "Colocar no
   * mapa" puts a combatant that came without a token. */
  protected async place(id: string): Promise<void> {
    const e = this.encounter();
    if (!e) {
      return;
    }
    const taken = new Set(e.combatants.filter((c) => c.placed).map((c) => `${c.col},${c.row}`));
    const middle: Square = { col: Math.floor(e.gridColumns / 2), row: Math.floor(e.gridRows / 2) };
    let best: Square | null = null;
    for (let row = 0; row < e.gridRows; row++) {
      for (let col = 0; col < e.gridColumns; col++) {
        const d = Math.max(Math.abs(col - middle.col), Math.abs(row - middle.row));
        const dBest = best ? Math.max(Math.abs(best.col - middle.col), Math.abs(best.row - middle.row)) : Infinity;
        if (!taken.has(`${col},${row}`) && d < dBest) {
          best = { col, row };
        }
      }
    }
    if (best) {
      const spot = best;
      await this.run((current) => this.api.move(this.campaignId(), current.id, id, spot.col, spot.row));
    }
  }

  protected leave(): void {
    this.state().dismissEnded();
  }
}
