import { TestBed } from '@angular/core/testing';
import { provideRouter } from '@angular/router';

import { Privacy } from './privacy/privacy';
import { Terms } from './terms/terms';

const CONTACT = 'privacidade@meurpg.app';

function render<T>(page: new () => T) {
  TestBed.configureTestingModule({ providers: [provideRouter([])] });
  const fixture = TestBed.createComponent(page);
  fixture.detectChanges();
  return { fixture, el: fixture.nativeElement as HTMLElement };
}

/** What both pages must have, whatever their words. */
function commonChecks(page: new () => unknown, title: string) {
  it('has the title, the date and the no-legal-review line before anything else', () => {
    const { el } = render(page);
    expect(el.querySelector('h1')?.textContent).toBe(title);
    expect(el.querySelector('.legal__updated')?.textContent).toBe('Última atualização: 09/10/2026');
    expect(el.querySelector('.legal__review')?.textContent).toMatch(
      /Ainda não (foi|foram) revisad[oa]s? por um advogado\./,
    );
  });

  it('has a summary box, and a contents list whose every link reaches a heading', () => {
    const { el } = render(page);
    expect(el.querySelector('.legal__summary h2')?.textContent).toBe('Em resumo');
    expect(el.querySelectorAll('.legal__summary li').length).toBeGreaterThanOrEqual(4);
    const links = Array.from(el.querySelectorAll<HTMLAnchorElement>('app-legal-toc a'));
    const headings = Array.from(el.querySelectorAll('.legal__article h2'));
    expect(links.length).toBe(headings.length);
    headings.forEach((h) => expect(h.id).not.toBe(''));
    links.forEach((a, i) => {
      expect(a.getAttribute('href')).toContain(`#${headings[i].id}`);
    });
    // Headings are numbered in order, and the list leaves the number to its own column.
    headings.forEach((h, i) => expect(h.textContent).toMatch(new RegExp(`^${i + 1}\\. `)));
  });

  it('writes the contact address and renders only text: no script, no raw HTML', () => {
    const { el } = render(page);
    expect(el.textContent).toContain(CONTACT);
    expect(el.querySelector('script, iframe, img')).toBeNull();
  });
}

describe('Terms', () => {
  commonChecks(Terms, 'Termos de uso');

  it('has the twelve sections and links to the privacy policy and the credits', () => {
    const { el } = render(Terms);
    expect(el.querySelectorAll('.legal__article h2')).toHaveLength(12);
    const hrefs = Array.from(el.querySelectorAll('.legal__article a')).map((a) =>
      a.getAttribute('href'),
    );
    expect(hrefs).toContain('/privacy');
    expect(hrefs).toContain('/credits');
  });

  it('states what the service is and that it is not tied to the rules publisher', () => {
    const { el } = render(Terms);
    expect(el.textContent).toContain('gratuito e privado');
    expect(el.textContent).toContain('Não é afiliado à Wizards of the Coast');
    expect(el.textContent).toContain('Fica eleito o foro do domicílio do usuário.');
  });
});

describe('Privacy', () => {
  commonChecks(Privacy, 'Política de privacidade');

  it('has the fourteen sections of the LGPD notice', () => {
    const { el } = render(Privacy);
    expect(el.querySelectorAll('.legal__article h2')).toHaveLength(14);
  });

  it('names the controller, the processors, the Google data and the international transfer', () => {
    const { el } = render(Privacy);
    const t = el.textContent ?? '';
    expect(t).toContain('Controlador');
    expect(t).toContain('Cockroach Labs');
    expect(t).toContain('Google (Gemini API)');
    expect(t).toContain('os escopos openid e email');
    expect(t).toContain('transferência internacional');
    expect(t).toContain('Sem anúncios, sem rastreamento e sem venda de dados');
  });

  it('draws the retention periods as a table with headers, and the cookies as code', () => {
    const { el } = render(Privacy);
    const table = el.querySelector('table.legal-table')!;
    expect(table.querySelectorAll('thead th[scope="col"]')).toHaveLength(2);
    expect(table.querySelectorAll('tbody tr')).toHaveLength(9);
    expect(table.querySelectorAll('tbody th[scope="row"]')).toHaveLength(9);
    const cookies = Array.from(el.querySelectorAll('code')).map((c) => c.textContent);
    expect(cookies).toEqual(['__Host-meurpg_session', '__Host-meurpg_login']);
  });
});
