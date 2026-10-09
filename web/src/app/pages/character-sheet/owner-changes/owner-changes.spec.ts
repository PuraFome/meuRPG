import { TestBed } from '@angular/core/testing';
import { provideRouter } from '@angular/router';

import { OwnerChanges } from './owner-changes';

describe('OwnerChanges', () => {
  function render(
    editLink: readonly string[] | null = ['/campaigns', 'c1', 'characters', 'x', 'edit'],
  ) {
    TestBed.configureTestingModule({ providers: [provideRouter([])] });
    const fixture = TestBed.createComponent(OwnerChanges);
    fixture.componentRef.setInput('reason', 'Falta o equipamento.');
    fixture.componentRef.setInput('when', '8 de out., 21h10');
    fixture.componentRef.setInput('editLink', editLink);
    fixture.detectChanges();
    return { fixture, el: fixture.nativeElement as HTMLElement };
  }

  it('tells, as an alert, that the master asked for changes, with the reason quoted and the date', () => {
    const { el } = render();
    const alert = el.querySelector('[role="alert"]')!;
    expect(alert.textContent).toContain('O mestre pediu ajustes.');
    expect(alert.querySelector('blockquote')?.textContent).toBe('“Falta o equipamento.”');
    expect(alert.textContent).toContain(
      'Pedido em 8 de out., 21h10. Ajuste a ficha e envie de novo.',
    );
  });

  it('has "Enviar de novo" as the only filled button, and "Editar ficha" beside it', () => {
    const { el } = render();
    expect(el.querySelectorAll('.mat-mdc-unelevated-button')).toHaveLength(1);
    expect(el.querySelector('.mat-mdc-unelevated-button')?.textContent?.trim()).toBe(
      'Enviar de novo',
    );
    expect(el.querySelector('a')?.getAttribute('href')).toBe('/campaigns/c1/characters/x/edit');
  });

  it('asks to resubmit only when the button is pressed, and not at all while busy', () => {
    const { fixture, el } = render();
    const resubmit = vi.fn();
    fixture.componentInstance.resubmit.subscribe(resubmit);
    expect(resubmit).not.toHaveBeenCalled();
    el.querySelector<HTMLButtonElement>('button')!.click();
    expect(resubmit).toHaveBeenCalledTimes(1);

    fixture.componentRef.setInput('busy', true);
    fixture.detectChanges();
    expect(el.querySelector<HTMLButtonElement>('button')!.disabled).toBe(true);
  });

  it('shows the server refusal, and no "Editar ficha" when the sheet cannot be edited', () => {
    const { fixture, el } = render(null);
    fixture.componentRef.setInput('error', 'Não foi possível concluir a ação agora.');
    fixture.detectChanges();
    expect(el.textContent).toContain('Não foi possível concluir a ação agora.');
    expect(el.querySelector('a')).toBeNull();
  });
});
