import { ChangeDetectionStrategy, Component } from '@angular/core';

/**
 * "Graus de cobertura" (E9-01 3): what each degree means for whoever paints it, under the map while the Cobertura tool is the
 * chosen one. Static words (the SRD's +2, +5 and "total"): the server works the cover out of each attack's line, and the master
 * corrects it on the combatant. The cover is only a degree, never the name of an object (the app does not know what is drawn).
 */
@Component({
  selector: 'app-cover-degrees',
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <section class="mr-panel cd" aria-labelledby="cd-title">
      <h2 class="mr-panel__title" id="cd-title">Graus de cobertura</h2>
      <p class="cd__lead">
        O app calcula a cobertura de cada ataque pela linha entre os dois quadrados: vale a melhor cobertura que ela atravessa. O mestre corrige se quiser.
      </p>
      <ul class="cd__list">
        <li>
          <span class="mr-swatch mr-swatch--half" aria-hidden="true"></span>
          <span class="cd__text"><b>Meia cobertura</b><span>+2 na CA e nas salvaguardas de Destreza. Dá para passar por cima.</span></span>
        </li>
        <li>
          <span class="mr-swatch mr-swatch--three" aria-hidden="true"></span>
          <span class="cd__text"><b>Três quartos</b><span>+5 na CA e nas salvaguardas de Destreza. Não dá para entrar.</span></span>
        </li>
        <li>
          <span class="mr-swatch mr-swatch--wall" aria-hidden="true"></span>
          <span class="cd__text"><b>Parede (total)</b><span>Ninguém acerta quem está atrás. Bloqueia movimento, visão e luz.</span></span>
        </li>
      </ul>
    </section>
  `,
  styles: `
    :host {
      display: block;
    }

    .cd {
      display: flex;
      flex-direction: column;
      gap: var(--mr-space-3);
    }

    .cd .mr-panel__title {
      margin: 0;
    }

    .cd__lead {
      margin: 0;
      font-size: 14px;
      line-height: 19px;
      color: var(--mr-ink-muted);
    }

    .cd__list {
      display: grid;
      grid-template-columns: repeat(auto-fit, minmax(14rem, 1fr));
      gap: var(--mr-space-3) var(--mr-space-4);
      margin: 0;
      padding: 0;
      list-style: none;
    }

    .cd__list li {
      display: flex;
      align-items: flex-start;
      gap: var(--mr-space-3);
    }

    .cd__list .mr-swatch {
      width: 36px;
      height: 36px;
    }

    .cd__text {
      display: flex;
      flex-direction: column;
      min-width: 0;
      font-size: 14px;
      line-height: 19px;
      color: var(--mr-ink-muted);
    }

    .cd__text b {
      font-size: 16px;
      line-height: 22px;
      color: var(--mr-ink);
    }
  `,
})
export class CoverDegrees {}
