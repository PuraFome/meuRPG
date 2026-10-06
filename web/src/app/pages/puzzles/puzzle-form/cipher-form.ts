import { ChangeDetectionStrategy, Component, DestroyRef, OnInit, effect, inject, input, output, signal, untracked } from '@angular/core';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatInputModule } from '@angular/material/input';

import {
  type CipherMethod,
  type Draft,
  CIPHER_MESSAGE_MAX,
  KEYWORD_MAX,
  KEYWORD_MIN,
  SHIFT_MAX,
  SHIFT_MIN,
  cipherSolutionOf,
  textLength,
} from '../../../core/puzzles/puzzle-draft';
import { puzzleErrorMessage } from '../../../core/puzzles/puzzle-errors';
import { PuzzlesClient } from '../../../core/puzzles/puzzles-client';
import { type ClueChoice, SolveTargets } from '../../../core/puzzles/solve-targets';
import { SecretPill } from '../../../shared/puzzle-boards/secret-pill';
import { type PickOption, PickGroup } from '../fields/pick-group';
import { Stepper } from '../fields/stepper';

/** How long a change of the message or the key waits before the server ciphers it (the master is still typing). */
const PREVIEW_PAUSE_MS = 300;

const METHODS: readonly PickOption<CipherMethod>[] = [
  { value: 'shift', title: 'Deslocamento' },
  { value: 'keyword', title: 'Palavra-chave' },
];

type Preview = { readonly status: 'idle' | 'loading' | 'ready' | 'error'; readonly text: string; readonly message: string };

/**
 * The form of the cipher (MR-038, RN-27, E10-12 state 3): the plain message, how its letters are swapped, and the scene clue the key lives
 * in. The message and the key are "Só você vê". "Como os jogadores a veem" is the ciphered message the server makes (`PreviewPuzzleCipher`,
 * after a short pause): the browser never ciphers, so what the master reads is exactly what the players will. The key is a clue of a
 * scene (MR-029), optional: the master writes it in a scene of a map, and the group finds it in the adventure; the form only links it.
 */
@Component({
  selector: 'app-cipher-form',
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [MatFormFieldModule, MatInputModule, PickGroup, SecretPill, Stepper],
  providers: [SolveTargets],
  template: `
    <section class="part" aria-labelledby="msg-title">
      <h3 class="part__title"><span id="msg-title">A mensagem</span><app-secret-pill /></h3>
      <mat-form-field appearance="outline" subscriptSizing="dynamic" class="full" [class.field-bad]="messageError()">
        <mat-label>Mensagem</mat-label>
        <textarea matInput rows="2" name="cipher-message" [value]="draft().cipherMessage" [attr.aria-invalid]="messageError() ? 'true' : null" (input)="patch.emit({ cipherMessage: $any($event.target).value })"></textarea>
        @if (messageError()) {
          <mat-hint class="field-error" role="alert">{{ messageError() }}</mat-hint>
        } @else {
          <mat-hint align="end">{{ length(draft().cipherMessage) }}&nbsp;de&nbsp;{{ messageMax }}</mat-hint>
        }
      </mat-form-field>
    </section>

    <section class="part" aria-labelledby="key-title">
      <h3 class="part__title"><span id="key-title">Como as letras são trocadas</span><app-secret-pill /></h3>
      <app-pick-group legend="Como as letras são trocadas" [hideLegend]="true" layout="segments" [options]="methods" [value]="draft().cipherMethod" (valueChange)="patch.emit({ cipherMethod: $event })" />
      @if (draft().cipherMethod === 'shift') {
        <app-stepper label="Letras adiante" noun="letra" [value]="draft().shift" [min]="shiftMin" [max]="shiftMax" (valueChange)="patch.emit({ shift: $event })" />
        <p class="part__help">Cada letra anda {{ draft().shift }} {{ draft().shift === 1 ? 'casa' : 'casas' }} para a frente no alfabeto, e as do fim voltam ao começo.</p>
        @if (keyError()) {
          <p class="part__bad field-error" role="alert">{{ keyError() }}</p>
        }
      } @else {
        <mat-form-field appearance="outline" subscriptSizing="dynamic" class="full" [class.field-bad]="keyError()">
          <mat-label>Palavra-chave</mat-label>
          <input matInput type="text" name="keyword" autocomplete="off" spellcheck="false" [value]="draft().keyword" [attr.aria-invalid]="keyError() ? 'true' : null" (input)="patch.emit({ keyword: $any($event.target).value })" />
          @if (keyError()) {
            <mat-hint class="field-error" role="alert">{{ keyError() }}</mat-hint>
          } @else {
            <mat-hint>De {{ keywordMin }} a {{ keywordMax }} letras diferentes: o alfabeto da cifra começa por elas.</mat-hint>
          }
        </mat-form-field>
      }
    </section>

    <section class="part" aria-labelledby="seen-title">
      <h3 class="part__title" id="seen-title">Como os jogadores a veem</h3>
      <!-- The last result stays on screen while the next one is asked for (no flicker, and nothing announced at each keystroke). -->
      @if (preview().text) {
        <p class="cipher" lang="pt" [attr.aria-busy]="preview().status === 'loading' ? 'true' : null">{{ preview().text }}</p>
      } @else if (preview().status === 'loading') {
        <p class="part__help">Cifrando a mensagem...</p>
      } @else if (preview().status !== 'error') {
        <p class="part__help">Escreva a mensagem e a chave: a carta aparece aqui, como os jogadores vão lê-la.</p>
      }
      @if (preview().status === 'error') {
        <p class="part__bad field-error" role="alert">{{ preview().message }}</p>
      }
      <p class="part__help">Cada letra é trocada por outra, sempre a mesma. Acentos e maiúsculas não contam na resposta.</p>
    </section>

    <section class="part" aria-labelledby="clue-title">
      <h3 class="part__title" id="clue-title">A chave, como pista da cena</h3>
      @if (clues().status === 'loading') {
        <p class="part__help" role="status">Procurando as pistas das cenas...</p>
      } @else if (clues().status === 'error') {
        <p class="part__bad field-error" role="alert">Não deu para ler as pistas. Volte e tente de novo.</p>
      } @else if (clues().list.length === 0 && draft().keyClueId === '') {
        <p class="part__help">Nenhuma cena da campanha tem pistas ainda. Escreva a chave como pista de uma cena no editor do mapa, e volte aqui para ligá-la.</p>
      } @else {
        <mat-form-field appearance="outline" subscriptSizing="dynamic" class="full" [class.field-bad]="clueError()">
          <mat-label>Qual pista guarda a chave</mat-label>
          <select matNativeControl [value]="draft().keyClueId" [attr.aria-invalid]="clueError() ? 'true' : null" (change)="patch.emit({ keyClueId: $any($event.target).value })">
            <option value="" [selected]="draft().keyClueId === ''">Nenhuma (você diz a chave na mesa)</option>
            @for (c of clues().list; track c.id) {
              <option [value]="c.id" [selected]="c.id === draft().keyClueId">{{ c.pointName }}: {{ shorten(c.text) }}</option>
            }
          </select>
          @if (clueError()) {
            <mat-hint class="field-error" role="alert">{{ clueError() }}</mat-hint>
          }
        </mat-form-field>
      }
      <p class="part__help">A chave vira uma pista da cena: você a põe numa cena ou numa nota de uma sala, e os jogadores a encontram. Quem a acha a lê nas suas notas.</p>
    </section>
  `,
  styleUrl: './cipher-form.scss',
})
export class CipherForm implements OnInit {
  private readonly api = inject(PuzzlesClient);
  private readonly targets = inject(SolveTargets);
  private readonly destroyRef = inject(DestroyRef);
  protected readonly methods = METHODS;
  protected readonly messageMax = CIPHER_MESSAGE_MAX;
  protected readonly keywordMin = KEYWORD_MIN;
  protected readonly keywordMax = KEYWORD_MAX;
  protected readonly shiftMin = SHIFT_MIN;
  protected readonly shiftMax = SHIFT_MAX;

  readonly campaignId = input.required<string>();
  readonly draft = input.required<Draft>();
  /** What is wrong with the message, the key and the linked clue, once the master tried to save (then the server's). */
  readonly messageError = input('');
  readonly keyError = input('');
  readonly clueError = input('');
  /** The message and the key are good enough to be ciphered (the form's own checks pass). */
  readonly cipherable = input(false);
  readonly patch = output<Partial<Draft>>();

  protected readonly preview = signal<Preview>({ status: 'idle', text: '', message: '' });
  protected readonly clues = signal<{ readonly status: 'loading' | 'ready' | 'error'; readonly list: readonly ClueChoice[] }>({ status: 'loading', list: [] });

  private lastKey = '';
  private timer: ReturnType<typeof setTimeout> | undefined;
  private ticket = 0;

  constructor() {
    this.destroyRef.onDestroy(() => clearTimeout(this.timer));
    // A change of the message or the key asks the server for the ciphered message after a pause; a stale answer never shows.
    effect(() => {
      const d = this.draft();
      const ok = this.cipherable();
      const key = ok ? JSON.stringify(cipherSolutionOf(d)) : '';
      untracked(() => {
        if (key === this.lastKey) {
          return;
        }
        this.lastKey = key;
        this.ticket++;
        clearTimeout(this.timer);
        if (key === '') {
          this.preview.set({ status: 'idle', text: '', message: '' });
          return;
        }
        this.preview.update((p) => ({ ...p, status: 'loading', message: '' }));
        this.timer = setTimeout(() => void this.run(d), PREVIEW_PAUSE_MS);
      });
    });
  }

  ngOnInit(): void {
    this.targets.use(this.campaignId());
    this.targets.clues().then(
      (list) => this.clues.set({ status: 'ready', list }),
      () => this.clues.set({ status: 'error', list: [] }),
    );
  }

  private async run(draft: Draft): Promise<void> {
    const ticket = ++this.ticket;
    try {
      const text = await this.api.previewCipher(this.campaignId(), cipherSolutionOf(draft));
      if (ticket === this.ticket) {
        this.preview.set({ status: 'ready', text, message: '' });
      }
    } catch (err) {
      if (ticket === this.ticket) {
        this.preview.set({ status: 'error', text: '', message: puzzleErrorMessage(err, 'cifrar a mensagem') });
      }
    }
  }

  protected length(text: string): number {
    return textLength(text.trim());
  }

  protected shorten(text: string): string {
    const one = text.replace(/\s+/g, ' ').trim();
    return one.length > 60 ? `${one.slice(0, 59)}…` : one;
  }
}
