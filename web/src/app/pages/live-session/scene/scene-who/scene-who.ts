import { ChangeDetectionStrategy, Component, computed, inject, input, signal } from '@angular/core';
import { timestampDate } from '@bufbuild/protobuf/wkt';
import { MatBottomSheet } from '@angular/material/bottom-sheet';
import { MatButtonModule } from '@angular/material/button';
import { MatDialog } from '@angular/material/dialog';
import { MatIconModule } from '@angular/material/icon';

import {
  choosablePlayers,
  insideSummary,
  outsideLine,
  sceneChooserLine,
} from '../../../../core/play/scene-audience';
import { SceneClient } from '../../../../core/play/scene-client';
import { sceneAudienceErrorMessage } from '../../../../core/play/scene-errors';
import type { SceneState } from '../../../../core/play/scene-state';
import { formatClock } from '../../../../shared/session-time/session-time';
import { openSheet } from '../../../../shared/sheet/sheet-host';
import { KeepSwitch } from '../../shown-image-panel/keep-switch';
import { WhoChooser, type WhoChoice, type WhoData } from '../who-chooser/who-chooser';

/**
 * "Aberta desde 20:20 para Pensantus Brisa" and who is out (MR-015, RN-10, C3-01, C3-02, C7-06): the
 * first lines of the master's open scene. A scene open to the whole table says so in one line; a scene
 * open to only some players names them (a count and the initials past three), says who is out ("Toren
 * não está nesta cena.") and carries the switch "Os jogadores podem entrar". "Quem está na cena" opens
 * the chooser of players (a sheet on a phone); the answer is the scene as it is now, applied at once.
 * Which players are in is the master's alone: a hidden scene never reaches a player (RN-10).
 */
@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-scene-who',
  imports: [KeepSwitch, MatButtonModule, MatIconModule],
  template: `
    @if (state().scene()) {
      <div class="who">
        <p class="who__since">
          <mat-icon aria-hidden="true">cast</mat-icon>
          @if (partial()) {
            <span class="who__lead">Aberta desde {{ clock() }} para</span>
            @if (summary().count) {
              <span class="who__count">{{ summary().count }}</span>
            }
            <ul class="who__chips" [class.who__chips--stack]="!!summary().count" aria-label="Quem está na cena">
              @for (name of summary().chips; track name) {
                <li class="who__chip" [attr.title]="summary().count ? name : null">
                  <span class="who__mark" aria-hidden="true">{{ initial(name) }}</span>
                  @if (!summary().count) {
                    {{ name }}
                  } @else {
                    <span class="mr-visually-hidden">{{ name }}</span>
                  }
                </li>
              }
            </ul>
          } @else {
            <span class="who__lead">{{ everyoneLine() }}</span>
          }
        </p>
        <button matButton type="button" class="who__edit" (click)="choose()">
          <mat-icon aria-hidden="true">groups</mat-icon>Quem está na cena
        </button>
      </div>

      @if (partial()) {
        <div class="who__second">
          @if (state().joined(); as joined) {
            <p class="who__out who__out--joined"><mat-icon aria-hidden="true">check</mat-icon>{{ joined }}</p>
          } @else if (outside()) {
            <p class="who__out"><mat-icon aria-hidden="true">visibility_off</mat-icon>{{ outside() }}</p>
          } @else {
            <p class="who__out">Todos os jogadores da mesa já estão na cena.</p>
          }
          <app-keep-switch
            class="who__join"
            id="join"
            label="Os jogadores podem entrar"
            [compact]="true"
            [checked]="state().playersMayJoin()"
            [disabled]="joining()"
            (toggle)="setMayJoin($event)"
          />
        </div>
      }

      @if (error()) {
        <div class="mr-notice mr-notice--danger" role="alert">
          <mat-icon aria-hidden="true">error</mat-icon>
          <p>{{ error() }}</p>
        </div>
      }
      <p class="mr-visually-hidden" role="status" aria-live="polite">{{ status() }}</p>
    }
  `,
  styleUrl: './scene-who.scss',
})
export class SceneWho {
  private readonly api = inject(SceneClient);
  private readonly dialog = inject(MatDialog);
  private readonly bottomSheet = inject(MatBottomSheet);

  readonly campaignId = input.required<string>();
  readonly state = input.required<SceneState>();

  protected readonly joining = signal(false);
  protected readonly error = signal('');
  protected readonly status = signal('');

  protected readonly audience = computed(() => this.state().audience());
  /** The scene is open to only some players. */
  protected readonly partial = computed(() => {
    const audience = this.audience();
    return !!audience && !audience.everyone;
  });
  protected readonly summary = computed(() => {
    const audience = this.audience();
    return audience ? insideSummary(audience) : { chips: [], count: '' };
  });
  protected readonly outside = computed(() => {
    const audience = this.audience();
    return audience ? outsideLine(audience) : '';
  });
  protected readonly clock = computed(() => {
    const at = this.state().scene()?.openedAt;
    return at ? formatClock(timestampDate(at)) : '';
  });
  protected readonly everyoneLine = computed(() =>
    this.clock() ? `Aberta para os jogadores desde ${this.clock()}` : 'Aberta para os jogadores',
  );

  protected initial(name: string): string {
    return name.trim().charAt(0).toUpperCase();
  }

  /** "Quem está na cena": the chooser opens with the players now in; "Cancelar" changes nothing. */
  protected choose(): void {
    const audience = this.audience();
    const data: WhoData = {
      title: 'Quem está na cena?',
      subtitle: 'Você sempre está. Quem não for escolhido não vê a cena nem sabe que ela existe.',
      players: choosablePlayers(audience),
      selected: audience?.inside.map((p) => p.userId) ?? [],
      line: sceneChooserLine,
    };
    openSheet<WhoChooser, WhoData, WhoChoice | undefined>(
      this.dialog,
      this.bottomSheet,
      WhoChooser,
      { data, ariaLabel: data.title, labelledBy: 'who-title' },
    ).subscribe((choice) => {
      if (choice) {
        void this.setAudience(choice);
      }
    });
  }

  private async setAudience(choice: WhoChoice): Promise<void> {
    this.error.set('');
    try {
      const scene = await this.api.setAudience(
        this.campaignId(),
        choice.everyone ? { everyone: true } : { userIds: choice.userIds },
      );
      this.state().apply(scene);
      this.status.set(
        choice.everyone ? 'Toda a mesa está na cena.' : 'A lista de quem está na cena mudou.',
      );
    } catch (err) {
      this.error.set(sceneAudienceErrorMessage(err));
      void this.state().refresh();
    }
  }

  protected async setMayJoin(value: boolean): Promise<void> {
    if (this.joining()) {
      return;
    }
    this.joining.set(true);
    this.error.set('');
    try {
      this.state().apply(await this.api.setPlayersMayJoin(this.campaignId(), value));
      this.status.set(
        value ? 'Os jogadores podem entrar na cena.' : 'Os jogadores não podem entrar na cena.',
      );
    } catch (err) {
      this.error.set(sceneAudienceErrorMessage(err));
      void this.state().refresh();
    } finally {
      this.joining.set(false);
    }
  }
}
