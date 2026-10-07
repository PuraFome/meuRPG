import { TestBed } from '@angular/core/testing';

import { MarkdownView, type MarkdownRefs, type RefOpen } from './markdown-view';

const MAP = '11111111-2222-4333-8444-555555555555';
const CHAR = 'aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee';
const IMG = '99999999-8888-4777-8666-555555555555';

function render(source: string, refs: MarkdownRefs | null = null) {
  const fixture = TestBed.createComponent(MarkdownView);
  fixture.componentRef.setInput('source', source);
  fixture.componentRef.setInput('refs', refs);
  fixture.detectChanges();
  return { fixture, el: fixture.nativeElement as HTMLElement };
}

describe('MarkdownView', () => {
  it('renders hostile text as text: no script, no handlers, no javascript: links', () => {
    const { el } = render(
      '<script>alert(1)</script> <img src=x onerror=alert(1)>\n\n[x](javascript:alert(1)) **<b>y</b>**',
    );
    expect(el.querySelector('script')).toBeNull();
    expect(el.querySelector('img')).toBeNull();
    expect(el.querySelector('b')).toBeNull();
    expect(el.querySelector('a')).toBeNull();
    expect(el.textContent).toContain('<script>alert(1)</script>');
    expect(el.textContent).toContain('[x](javascript:alert(1))');
  });

  it('renders headings with ids, bold, italic, lists and https links', () => {
    const { el } = render(
      '## Título\n\nUm **forte** e *leve* [site](https://example.com)\n\n- a\n- b\n\n1. c',
    );
    expect(el.querySelector('h2')?.id).toBe('doc-h-0');
    expect(el.querySelector('strong')?.textContent).toBe('forte');
    expect(el.querySelector('em')?.textContent).toBe('leve');
    const a = el.querySelector('a') as HTMLAnchorElement;
    expect(a.getAttribute('href')).toBe('https://example.com/');
    expect(a.getAttribute('rel')).toBe('noopener noreferrer');
    expect(el.querySelectorAll('ul li')).toHaveLength(2);
    expect(el.querySelectorAll('ol li')).toHaveLength(1);
  });

  it('draws an image with its caption, and "Imagem apagada" when the gallery lacks it', () => {
    const src = `![A Taverna](image:${IMG})`;
    const { el } = render(src);
    expect(el.querySelector('img')?.getAttribute('src')).toBe(`/images/${IMG}`);
    expect(el.querySelector('figcaption')?.textContent).toBe('A Taverna');

    const gone = render(src, { maps: null, characters: null, images: new Map() });
    expect(gone.el.querySelector('img')).toBeNull();
    expect(gone.el.textContent).toContain('Imagem apagada');
  });

  it('turns a failed image into "Imagem apagada"', () => {
    const { fixture, el } = render(`![x](image:${IMG})`);
    el.querySelector('img')!.dispatchEvent(new Event('error'));
    fixture.detectChanges();
    expect(el.textContent).toContain('Imagem apagada');
  });

  it('makes map and sheet links buttons that open, and says when the target is gone', () => {
    const src = `[Mirathel](map:${MAP}) e [Capitão](character:${CHAR})`;
    const { fixture, el } = render(src);
    const opened: RefOpen[] = [];
    fixture.componentInstance.openRef.subscribe((r) => opened.push(r));
    const buttons = el.querySelectorAll<HTMLButtonElement>('button');
    expect(buttons).toHaveLength(2);
    expect(buttons[0].getAttribute('aria-haspopup')).toBe('dialog');
    buttons[1].click();
    expect(opened).toEqual([{ kind: 'character', id: CHAR, text: 'Capitão' }]);

    const gone = render(src, { maps: new Set(), characters: new Set([CHAR]), images: null });
    expect(gone.el.textContent).toContain('Mirathel (mapa apagado)');
    expect(gone.el.querySelectorAll('button')).toHaveLength(1);
    const both = render(src, { maps: new Set([MAP]), characters: new Set(), images: null });
    expect(both.el.textContent).toContain('Capitão (ficha apagada)');
  });
});
