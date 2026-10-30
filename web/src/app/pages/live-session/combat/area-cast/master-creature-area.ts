import {
  ChangeDetectionStrategy,
  Component,
  computed,
  effect,
  inject,
  signal,
} from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { create } from '@bufbuild/protobuf';
import { MatIconModule } from '@angular/material/icon';

import {
  AreaPlacement,
  type Combatant,
  type CreatureActionResult,
  PreviewSpellAreaResponseSchema,
  TargetInReachSchema,
} from '../../../../../gen/meurpg/play/v1/combat_pb';
import { type CreatureAction } from '../../../../../gen/meurpg/rules/v1/rules_pb';
import { AreaFlow } from '../../../../core/combat/area-flow';
import { shapeText, stepOneTitle } from '../../../../core/combat/area-text';
import { CombatClient } from '../../../../core/combat/combat-client';
import { combatErrorMessage } from '../../../../core/combat/combat-errors';
import type { CombatState } from '../../../../core/combat/combat-state';
import { actionName, areaShapeOf } from '../../../../core/combat/creature-action';
import { AreaMap } from '../../../../shared/area-picker/area-map';
import { AreaStep } from '../../../../shared/area-picker/area-step';
import { SheetFrame } from '../../../../shared/sheet/sheet-frame/sheet-frame';
import { injectSheet } from '../../../../shared/sheet/sheet-host';
import type { CastMapData } from '../cast-sheet/cast-sheet';

/** What the page hands the picker of a creature action's area. */
export interface MasterCreatureAreaData {
  readonly campaignId: string;
  readonly encounterId: string;
  readonly actor: Combatant;
  readonly action: CreatureAction;
  readonly placement: AreaPlacement;
  readonly targetIds: readonly string[];
  readonly map: CastMapData;
  readonly state: CombatState;
}

/** What the picker answers when the action was used: the action and what it did. */
export interface CreatureActionOutcome {
  readonly action: CreatureAction;
  readonly result: CreatureActionResult;
}

/**
 * "Hell Hound usa Sopro de Fogo": the master places the area of a creature's action on the map with the same map as the
 * spells' picker (a point, or one of eight directions) and uses it. The server takes who is inside, so the dialog lists nobody:
 * the result says who it reached. Nothing is spent until "Usar".
 */
@Component({
  selector: 'app-master-creature-area',
  imports: [AreaMap, AreaStep, MatButtonModule, MatIconModule, SheetFrame],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <app-sheet-frame [title]="data.actor.label + ' usa ' + name" [subtitle]="subtitle" [phone]="inSheet" (closed)="close()">
      <div class="col">
        <app-area-step [n]="1" [title]="stepOne" />
        <app-area-map
          [image]="data.map.image"
          [mapName]="data.map.mapName"
          [columns]="data.map.columns"
          [rows]="data.map.rows"
          [layers]="data.map.layers"
          [combatants]="combatants()"
          [master]="true"
          [casterId]="data.actor.id"
          [spell]="name"
          [placement]="data.placement"
          [area]="shape"
          [rangeFt]="0"
          [placed]="flow.placed()"
          [preview]="null"
          [seen]="seen()"
          [autofocus]="true"
          (place)="flow.place($event)"
          (confirm)="use()"
          (dismiss)="close()"
        />
        @if (error()) {
          <div class="mr-notice mr-notice--danger" role="alert">
            <mat-icon aria-hidden="true">error</mat-icon>
            <p>{{ error() }}</p>
          </div>
        }
      </div>
      <div foot class="foot">
        <button mat-button type="button" class="quiet" [disabled]="busy()" (click)="close()">Cancelar</button>
        <button
          mat-flat-button
          type="button"
          class="btn"
          [class.mr-button--off]="!flow.ready() || busy()"
          [disabled]="busy() || !flow.ready()"
          disabledInteractive
          [attr.aria-label]="'Usar ' + name"
          (click)="use()"
        >
          <mat-icon aria-hidden="true">bolt</mat-icon>Usar {{ name }}
        </button>
      </div>
    </app-sheet-frame>
  `,
  styles: `
    :host {
      display: block;
    }

    .col {
      display: flex;
      flex-direction: column;
      gap: var(--mr-space-3);
      min-width: 0;
    }

    .foot {
      display: flex;
      flex-wrap: wrap;
      justify-content: flex-end;
      gap: var(--mr-space-2);

      @media (max-width: 767.98px) {
        flex-direction: column-reverse;
      }
    }

    .btn,
    .quiet {
      --mat-button-filled-container-height: 48px;
      --mat-button-text-container-height: 48px;
    }

    .quiet {
      --mat-button-text-label-text-color: var(--mr-accent-text);
      font-weight: 700;
    }
  `,
})
export class MasterCreatureArea {
  private readonly api = inject(CombatClient);
  private readonly sheet = injectSheet<MasterCreatureAreaData, CreatureActionOutcome | null>();
  protected readonly data = this.sheet.data;
  protected readonly inSheet = this.sheet.inSheet;

  protected readonly name = actionName(this.data.action);
  protected readonly shape = areaShapeOf(this.data.action);
  protected readonly stepOne = stepOneTitle(this.data.placement);
  protected readonly subtitle = `${shapeText(this.shape)} · ${
    this.data.placement === AreaPlacement.DIRECTION
      ? 'escolha a direção no mapa'
      : 'escolha o ponto no mapa'
  }`;
  /** The browser draws the outline; the server takes who is inside when the action is used, so the answer here is empty. */
  protected readonly flow = new AreaFlow(this.data.placement, () =>
    Promise.resolve(create(PreviewSpellAreaResponseSchema, {})),
  );
  protected readonly busy = signal(false);
  protected readonly error = signal('');

  protected readonly combatants = computed(() => this.data.state.encounter()?.combatants ?? []);
  /** Everyone placed on the map, hidden ones too: "Centrar em…" for the master. */
  protected readonly seen = computed(() =>
    this.combatants()
      .filter((c) => c.placed && c.id !== this.data.actor.id)
      .map((c) => create(TargetInReachSchema, { combatantId: c.id, label: c.label })),
  );

  constructor() {
    effect(() => this.sheet.lock(this.busy()));
  }

  protected async use(): Promise<void> {
    const area = this.flow.choice();
    if (this.busy() || !area) {
      return;
    }
    this.busy.set(true);
    this.error.set('');
    try {
      const res = await this.api.casts.useCreatureAction(
        this.data.campaignId,
        this.data.encounterId,
        this.data.actor.id,
        this.data.action.key,
        this.data.targetIds,
        area,
      );
      this.data.state.apply(res.encounter);
      this.sheet.close({ action: this.data.action, result: res.result });
    } catch (err) {
      this.error.set(combatErrorMessage(err, 'usar a ação'));
    } finally {
      this.busy.set(false);
    }
  }

  protected close(): void {
    if (!this.busy()) {
      this.sheet.close(null);
    }
  }
}
