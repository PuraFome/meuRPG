import { TestBed } from '@angular/core/testing';
import { provideRouter } from '@angular/router';

import { OwnerResubmit } from './owner-resubmit';

describe('OwnerResubmit', () => {
  function render(
    editLink: readonly string[] | null = ['/campaigns', 'c1', 'characters', 'x', 'edit'],
  ) {
    TestBed.configureTestingModule({ providers: [provideRouter([])] });
    const fixture = TestBed.createComponent(OwnerResubmit);
    fixture.componentRef.setInput('editLink', editLink);
    fixture.detectChanges();
    return { fixture, el: fixture.nativeElement as HTMLElement };
  }

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

  it('shows the server refusal as its own alert, and no "Editar ficha" when the sheet cannot be edited', () => {
    const { fixture, el } = render(null);
    expect(el.querySelector('[role="alert"]')).toBeNull();
    fixture.componentRef.setInput('error', 'Não foi possível concluir a ação agora.');
    fixture.detectChanges();
    expect(el.querySelector('[role="alert"]')?.textContent).toContain(
      'Não foi possível concluir a ação agora.',
    );
    expect(el.querySelector('a')).toBeNull();
  });
});
