import { TestBed } from '@angular/core/testing';

import { RevivedNotice } from './revived-notice';

const text = (el: Element | null) => (el?.textContent ?? '').replace(/\s+/g, ' ').trim();

function mount(inputs: Record<string, unknown>) {
  const fixture = TestBed.createComponent(RevivedNotice);
  Object.entries(inputs).forEach(([k, v]) => fixture.componentRef.setInput(k, v));
  fixture.detectChanges();
  return fixture.nativeElement as HTMLElement;
}

describe('RevivedNotice', () => {
  it('keeps its polite live region in the page with nothing in it', () => {
    const el = mount({});
    const live = el.querySelector('[role=status]');
    expect(live?.getAttribute('aria-live')).toBe('polite');
    expect(text(live)).toBe('');
  });

  it('names the caster when the log says it', () => {
    const el = mount({ on: true, caster: 'Ilaria' });
    expect(text(el.querySelector('[role=status]'))).toContain(
      'Você voltou à vida. Ilaria usou Revivificar em você. Está com 1 PV.',
    );
  });

  it('does not invent a caster', () => {
    const el = mount({ on: true });
    const said = text(el.querySelector('[role=status]'));
    expect(said).toContain('Você voltou à vida. Está com 1 PV.');
    expect(said).not.toContain('usou Revivificar');
    expect(said).not.toContain('iniciativa');
  });

  it('says where it returns and the next round only when the combat gives them', () => {
    const el = mount({ on: true, place: { between: 'entre Brisa e Pensantus', round: 9 } });
    expect(text(el.querySelector('[role=status]'))).toContain(
      'Você volta à ordem de iniciativa onde estava (entre Brisa e Pensantus). O seu próximo turno é na rodada 9.',
    );
  });

  it('does not move the focus', () => {
    const before = document.activeElement;
    mount({ on: true, caster: 'Ilaria' });
    expect(document.activeElement).toBe(before);
  });
});
