import { ChangeDetectionStrategy, Component, computed, input } from '@angular/core';
import { MatIconModule } from '@angular/material/icon';

import type { ImageRun } from '../../core/images/imagegen-run';

/** The words that go under "Cancelar" / "Parar de esperar" for each moment, said in the footer by the dialog. */
export const WAIT_NOTES = {
  sending:
    'Se você cancelar agora, o pedido é parado e a vaga do mês volta, a não ser que ele já tenha saído.',
  waiting:
    'Se o pedido ainda não saiu do servidor, a vaga do mês volta. Se já saiu, o Google já o recebeu: a vaga continua gasta e a imagem, se chegar, vai para a galeria.',
  canceling: 'Cancelando o pedido…',
} as const;

/**
 * "Gerando a imagem" (E10-07 4): the request as the master wrote it, a line that says what is happening, how long it has been (apart from the
 * line, so a screen reader hears the sentence once and not the counter) and an indeterminate bar that stands still for whoever asked for less
 * motion. Never a spinner without words. After 60 seconds the line says the service is slow and that the master can wait or stop waiting.
 */
@Component({
  selector: 'app-generate-running',
  imports: [MatIconModule],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <div class="req">
      <p class="req__meta">Seu pedido · {{ summary() }}</p>
      <p class="req__text">{{ prompt() }}</p>
    </div>
    <div class="line">
      <mat-icon class="line__icon" aria-hidden="true">sync</mat-icon>
      <p class="line__words" role="status">{{ headline() }}</p>
    </div>
    <p class="sub">{{ sub() }}</p>
    <div class="bar" aria-hidden="true"><span class="bar__run"></span></div>
  `,
  styleUrl: './generate-running.scss',
})
export class GenerateRunning {
  readonly run = input.required<ImageRun>();
  /** "Arte da cena · Pintura a óleo · 2 NPCs". */
  readonly summary = input.required<string>();
  readonly prompt = input.required<string>();

  protected readonly headline = computed(() => {
    const run = this.run();
    if (run.slow()) {
      return 'O serviço está demorando. Pode esperar ou parar de esperar.';
    }
    switch (run.phase()) {
      case 'sending':
        return 'Enviando o pedido…';
      case 'canceling':
        return 'Cancelando o pedido…';
      default:
        return 'Gerando a imagem…';
    }
  });

  protected readonly sub = computed(() => {
    const run = this.run();
    switch (run.phase()) {
      case 'sending':
        return 'Só um instante.';
      case 'canceling':
        return 'Só um instante.';
      default:
        return `${run.seconds()} s. Costuma levar de 10 a 20 s.`;
    }
  });
}
