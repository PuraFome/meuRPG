import { TestBed } from '@angular/core/testing';

import { SheetSaveFoot } from './sheet-save-foot';

function setup(inputs: { inSheet?: boolean; busy?: boolean } = {}) {
  const fixture = TestBed.createComponent(SheetSaveFoot);
  fixture.componentRef.setInput('lock', 'Só o mestre muda.');
  for (const [k, v] of Object.entries(inputs)) {
    fixture.componentRef.setInput(k, v);
  }
  fixture.detectChanges();
  const el = fixture.nativeElement as HTMLElement;
  const buttons = () => Array.from(el.querySelectorAll('button'));
  return { fixture, el, buttons };
}

describe('SheetSaveFoot', () => {
  it('has "Salvar" beside "Cancelar" in a dialog, marked as equal width', () => {
    const { el, buttons } = setup();
    expect(buttons().map((b) => b.textContent!.replace(/\s+/g, ' ').trim())).toEqual([
      'save Salvar',
      'Cancelar',
    ]);
    expect(el.querySelector('.pair')!.getAttribute('data-equal-width')).toBe('.pair__btn');
    expect(el.querySelector('.lock')!.textContent).toContain('Só o mestre muda.');
  });

  it('has "Salvar" alone on a phone', () => {
    const { buttons } = setup({ inSheet: true });
    expect(buttons()).toHaveLength(1);
  });

  it('says "Salvando…" with a turning ring and turns both buttons off while busy', () => {
    const { el, buttons } = setup({ busy: true });
    expect(buttons()[0].textContent).toContain('Salvando…');
    expect(el.querySelector('.pair__spin')).not.toBeNull();
    expect(buttons()[0].getAttribute('aria-disabled')).toBe('true');
    expect(buttons()[1].disabled).toBe(true);
    expect(buttons()[0].classList).toContain('pair__btn--busy');
    expect(buttons()[0].contains(el.querySelector('.pair__spin'))).toBe(true);
    expect(buttons()[0].querySelector('mat-icon')).toBeNull();
  });

  it('emits what was pressed', () => {
    const { fixture, buttons } = setup();
    const seen: string[] = [];
    fixture.componentInstance.saved.subscribe(() => seen.push('save'));
    fixture.componentInstance.cancelled.subscribe(() => seen.push('cancel'));
    buttons().forEach((b) => b.click());
    expect(seen).toEqual(['save', 'cancel']);
  });
});
