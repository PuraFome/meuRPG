import { ChangeDetectionStrategy, Component } from '@angular/core';
import { RouterLink } from '@angular/router';

import { LegalToc, type LegalSection } from '../legal-toc/legal-toc';

/** The sections, in order: the table of contents lists them and the template anchors them. */
const SECTIONS: readonly LegalSection[] = [
  { id: 'quem', title: '1. Quem cuida dos seus dados' },
  { id: 'dados', title: '2. Que dados usamos e por quê' },
  { id: 'google', title: '3. Os dados da sua conta Google' },
  { id: 'onde', title: '4. Onde os dados ficam e quem nos ajuda' },
  { id: 'prazos', title: '5. Por quanto tempo guardamos' },
  { id: 'cookies', title: '6. Cookies' },
  { id: 'nao-fazemos', title: '7. O que não fazemos' },
  { id: 'quem-ve', title: '8. Quem vê o quê no app' },
  { id: 'direitos', title: '9. Seus direitos' },
  { id: 'excluir', title: '10. Excluir a conta' },
  { id: 'seguranca', title: '11. Segurança e incidentes' },
  { id: 'idade', title: '12. Idade mínima' },
  { id: 'mudancas', title: '13. Mudanças nesta política' },
  { id: 'contato', title: '14. Contato' },
];

/**
 * "/privacy": the privacy policy (LGPD).
 * Public (no sign-in): the Google consent screen links to it. The text is plain template
 * markup, so nothing is rendered from a string; it stays equal to docs/privacy.md.
 */
@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-privacy',
  imports: [RouterLink, LegalToc],
  templateUrl: './privacy.html',
  styleUrl: '../legal.scss',
})
export class Privacy {
  protected readonly sections = SECTIONS;
}
