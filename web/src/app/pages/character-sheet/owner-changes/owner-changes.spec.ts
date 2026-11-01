import { TestBed } from '@angular/core/testing';

import { OwnerChanges } from './owner-changes';

describe('OwnerChanges', () => {
  function render(when = '8 de out., 21h10') {
    const fixture = TestBed.createComponent(OwnerChanges);
    fixture.componentRef.setInput('reason', 'Falta o equipamento.');
    fixture.componentRef.setInput('when', when);
    fixture.detectChanges();
    return fixture.nativeElement as HTMLElement;
  }

  it('tells, with no live role of its own, that the master asked for changes, with the reason quoted and the date', () => {
    const el = render();
    const notice = el.querySelector('.mr-notice')!;
    expect(notice.hasAttribute('role')).toBe(false);
    expect(notice.textContent).toContain('O mestre pediu ajustes.');
    expect(notice.querySelector('blockquote')?.textContent).toBe('“Falta o equipamento.”');
    expect(notice.textContent).toContain(
      'Pedido em 8 de out., 21h10. Ajuste a ficha e envie de novo.',
    );
  });

  it('has no buttons: they live outside the status region', () => {
    const el = render();
    expect(el.querySelector('button, a')).toBeNull();
  });

  it('leaves the date out when it is not known', () => {
    expect(render('').textContent).not.toContain('Pedido em');
  });
});
