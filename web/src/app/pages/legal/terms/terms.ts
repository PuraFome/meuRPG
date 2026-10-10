import { ChangeDetectionStrategy, Component } from '@angular/core';
import { RouterLink } from '@angular/router';

import { LegalToc, type LegalSection } from '../legal-toc/legal-toc';

/** The sections, in order: the table of contents lists them and the template anchors them. */
const SECTIONS: readonly LegalSection[] = [
  { id: 'servico', title: '1. O que é o MeuRPG' },
  { id: 'quem-pode', title: '2. Quem pode usar' },
  { id: 'login', title: '3. Entrar com o Google' },
  { id: 'campanhas', title: '4. Campanhas, mestre e jogadores' },
  { id: 'uso', title: '5. Uso aceitável' },
  { id: 'conteudo', title: '6. Seu conteúdo e as imagens' },
  { id: 'srd', title: '7. Regras do SRD e créditos' },
  { id: 'garantia', title: '8. Disponibilidade e garantia' },
  { id: 'encerramento', title: '9. Encerramento e exclusão da conta' },
  { id: 'mudancas', title: '10. Mudanças nestes termos' },
  { id: 'lei', title: '11. Lei aplicável' },
  { id: 'contato', title: '12. Contato' },
];

/**
 * "/terms": the terms of use.
 * Public (no sign-in): the Google consent screen links to it. The text is plain template
 * markup, so nothing is rendered from a string; it stays equal to docs/privacy.md.
 */
@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-terms',
  imports: [RouterLink, LegalToc],
  templateUrl: './terms.html',
  styleUrl: '../legal.scss',
})
export class Terms {
  protected readonly sections = SECTIONS;
}
