import { ChangeDetectionStrategy, Component, computed, input, output } from '@angular/core';
import { MatIconModule } from '@angular/material/icon';

import type { PickableNpc } from '../../core/images/imagegen-form';
import { initialsOf } from '../../core/play/stage-view';

/**
 * "Quem aparece na imagem" (E10-07 1, 9 and 10): the NPCs and enemies the players see now, as checkable chips with their portraits. The
 * list comes from `GetMapImageReference`, so a creature the players do not see is simply not here (and the server refuses it if asked); the
 * sentence under the list says why. A marked NPC's portrait goes to Google as a character reference, counted against the four. A chip with a
 * portrait that would be a fifth is dashed and says so instead of being refused later. It is a group of checkboxes: Tab walks the chips and
 * Space marks.
 */
@Component({
  selector: 'app-npc-picks',
  imports: [MatIconModule],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <div class="head">
      <h3 class="head__title" id="npc-title" tabindex="-1" data-field="npc">Quem aparece na imagem</h3>
      @if (npcs().length > 0) {
        <span class="head__count">{{ marked().length }} de {{ npcs().length }} marcados</span>
      }
    </div>
    @if (npcs().length > 0) {
      <ul class="chips" aria-labelledby="npc-title">
        @for (npc of npcs(); track npc.characterId) {
          <li>
            <button
              type="button"
              role="checkbox"
              class="chip"
              [class.chip--on]="isOn(npc)"
              [class.chip--off]="isFull(npc)"
              [attr.aria-checked]="isOn(npc)"
              [attr.aria-disabled]="isFull(npc) ? 'true' : null"
              (click)="toggle(npc)"
            >
              @if (isOn(npc)) {
                <mat-icon class="chip__check" aria-hidden="true">check</mat-icon>
              }
              <span class="chip__face" aria-hidden="true">
                @if (npc.portraitImageId) {
                  <img [src]="'/images/' + npc.portraitImageId + '/thumb'" alt="" />
                } @else {
                  {{ initials(npc.name) }}
                }
              </span>
              <span class="chip__name">{{ npc.name }}</span>
            </button>
          </li>
        }
      </ul>
      @if (full()) {
        <p class="note">Até {{ maxPortraits() }} retratos vão como referência: tire um para marcar outro.</p>
      }
    } @else {
      <p class="note">Nenhum NPC ou inimigo que os jogadores vejam agora.</p>
    }
    <p class="note">
      Só aparecem os que os jogadores veem agora: uma criatura que eles não veem não está na lista, para a imagem não entregar o que ainda não descobriram. Os retratos da galeria dos marcados vão como referência.
    </p>
  `,
  styleUrl: './npc-picks.scss',
})
export class NpcPicks {
  readonly npcs = input.required<readonly PickableNpc[]>();
  readonly marked = input.required<readonly string[]>();
  readonly maxPortraits = input(4);
  readonly toggled = output<string>();

  protected readonly markedPortraits = computed(
    () =>
      new Set(
        this.npcs()
          .filter((n) => this.marked().includes(n.characterId) && n.portraitImageId !== '')
          .map((n) => n.portraitImageId),
      ).size,
  );
  protected readonly full = computed(() => this.markedPortraits() >= this.maxPortraits());

  protected isOn(npc: PickableNpc): boolean {
    return this.marked().includes(npc.characterId);
  }

  /** A fifth portrait: the chip waits for one to be unmarked. */
  protected isFull(npc: PickableNpc): boolean {
    return !this.isOn(npc) && npc.portraitImageId !== '' && this.full();
  }

  protected initials(name: string): string {
    return initialsOf(name);
  }

  protected toggle(npc: PickableNpc): void {
    if (!this.isFull(npc)) {
      this.toggled.emit(npc.characterId);
    }
  }
}
