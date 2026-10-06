import { Component, ElementRef, Injector, afterNextRender, computed, effect, inject, input, output, signal, untracked } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';

import { type Combatant, CoverDegree } from '../../../../../gen/meurpg/play/v1/combat_pb';
import { COVER_CHOICES, coverLine } from '../../../../core/combat/theatre';
import { combatantInitial, isPlayer } from '../../../../core/combat/combat-view';
import { ofThe } from '../../../../core/combat/move-plan';
import { isCreature } from '../../../../core/combat/creature-names';
import { coverMark } from '../../../../core/combat/cover';
import { CombatantToken } from '../../../../shared/combatant-token/combatant-token';

let nextId = 0;

/**
 * "Cobertura dos alvos" (RN-25, E10-04 states 2 and 4): without a map nothing draws cover, so it is what the master marks, and it holds until
 * he changes it. Each target shows its degree in a word (with the map's pictogram for half and three quarters); "Mudar" opens the four rows,
 * Sem cobertura, Meia cobertura (+2), Três quartos (+5) and Total (it cannot be aimed at), and "Salvar cobertura" sends the one chosen.
 * The server applies it: a player who attacks reads the word and the bonus when they roll, and a target with total cover leaves their list.
 */
@Component({
  selector: 'app-cover-panel',
  imports: [CombatantToken, MatButtonModule],
  template: `
    @if (editing(); as target) {
      <section class="cover" [attr.aria-labelledby]="id + '-e'">
        <h3 class="cover__title" [id]="id + '-e'">Cobertura {{ of(target) }}</h3>
        <p class="cover__text">Você escolhe; o app só guarda o grau. Quem ataca vê “Meia cobertura (marcada pelo mestre)”, com o bônus.</p>
        <div class="choices" role="radiogroup" [attr.aria-label]="'Cobertura de ' + target.label">
          @for (c of choices; track c.value) {
            <label class="choice" [class.choice--on]="draft() === c.value">
              <input type="radio" class="mr-visually-hidden" [name]="id" [checked]="draft() === c.value" (change)="draft.set(c.value)" />
              <span class="choice__dot" aria-hidden="true"></span>
              <span class="choice__mark">
                @if (c.mark) {
                  <span class="mr-swatch mr-swatch--sm" [class.mr-swatch--half]="c.mark === 'half'" [class.mr-swatch--three]="c.mark === 'three'" aria-hidden="true"></span>
                }
              </span>
              <span class="choice__words">
                <span class="choice__name">{{ c.name }}</span>
                @if (c.sub) {
                  <span class="choice__sub">{{ c.sub }}</span>
                }
              </span>
            </label>
          }
        </div>
        <div class="actions">
          <button mat-stroked-button type="button" class="btn" (click)="close()">Cancelar</button>
          <button mat-stroked-button type="button" class="btn" [disabled]="busy()" (click)="save(target)">Salvar cobertura</button>
        </div>
      </section>
    } @else {
      <section class="cover" [attr.aria-labelledby]="id + '-t'">
        <h3 class="cover__title" [id]="id + '-t'">Cobertura dos alvos</h3>
        <p class="cover__text">Sem mapa, a cobertura é a que você marca. Quem ataca vê a palavra e o bônus na hora de rolar.</p>
        @if (rows().length) {
          <ul class="targets">
            @for (r of rows(); track r.c.id) {
              <li class="target">
                <app-combatant-token [initial]="initial(r.c)" [npc]="npc(r.c)" [creature]="creature(r.c)" [size]="36" />
                <span class="target__text">
                  <b class="target__name">{{ r.c.label }}</b>
                  <span class="target__line">
                    @if (r.mark) {
                      <span class="mr-swatch mr-swatch--sm" [class.mr-swatch--half]="r.mark === 'half'" [class.mr-swatch--three]="r.mark === 'three'" aria-hidden="true"></span>
                    }
                    {{ r.line }}
                  </span>
                </span>
                <button mat-button type="button" class="target__change" [attr.data-row]="r.c.id" [attr.aria-label]="'Mudar a cobertura de ' + r.c.label" (click)="open(r.c)">Mudar</button>
              </li>
            }
          </ul>
        } @else {
          <p class="cover__text">Ninguém para mirar agora.</p>
        }
      </section>
    }
  `,
  styleUrl: './cover-panel.scss',
})
export class CoverPanel {
  private readonly host = inject<ElementRef<HTMLElement>>(ElementRef);
  private readonly injector = inject(Injector);

  /** Who can be aimed at by whoever is on turn. */
  readonly targets = input.required<readonly Combatant[]>();
  /** Everyone standing: the editor can open on anyone from the order's menu. */
  readonly everyone = input<readonly Combatant[]>([]);
  /** The order's menu asked to mark a combatant's cover: the editor opens on it (`n` makes the same one count again). */
  readonly request = input<{ id: string; n: number } | null>(null);
  readonly busy = input(false);
  /** "Salvar cobertura": the target and the degree. */
  readonly cover = output<{ id: string; cover: CoverDegree }>();

  protected readonly id = `cover-panel-${nextId++}`;
  protected readonly choices = COVER_CHOICES;
  protected readonly editing = signal<Combatant | null>(null);
  protected readonly draft = signal<CoverDegree>(CoverDegree.NONE);
  /** Which row to give the focus back to when the edit closes. */
  private lastId = '';

  protected readonly rows = computed(() =>
    this.targets().map((c) => ({ c, line: coverLine(c.coverMark), mark: coverMark(c.coverMark) })),
  );

  constructor() {
    effect(() => {
      const r = this.request();
      untracked(() => {
        const who = r ? this.everyone().find((c) => c.id === r.id) : null;
        if (who) {
          this.open(who);
        }
      });
    });
  }

  /** "do Capitão Goblin", "da Brisa". */
  protected of(c: Combatant): string {
    return ofThe([c.label]);
  }

  protected initial(c: Combatant): string {
    return combatantInitial(c.label);
  }

  protected npc(c: Combatant): boolean {
    return !isPlayer(c) && !isCreature(c);
  }

  protected creature(c: Combatant): boolean {
    return isCreature(c);
  }

  protected open(c: Combatant): void {
    this.lastId = c.id;
    this.draft.set(c.coverMark === CoverDegree.UNSPECIFIED ? CoverDegree.NONE : c.coverMark);
    this.editing.set(c);
    // The focus opens on the degree that is marked.
    afterNextRender(() => this.host.nativeElement.querySelector<HTMLInputElement>('input:checked')?.focus(), { injector: this.injector });
  }

  protected close(): void {
    this.editing.set(null);
    const id = this.lastId;
    afterNextRender(
      () => this.host.nativeElement.querySelector<HTMLButtonElement>(`[data-row="${id}"]`)?.focus(),
      { injector: this.injector },
    );
  }

  protected save(c: Combatant): void {
    if (this.draft() !== c.coverMark) {
      this.cover.emit({ id: c.id, cover: this.draft() });
    }
    this.close();
  }
}
