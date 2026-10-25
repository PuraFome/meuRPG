import { ChangeDetectionStrategy, Component } from '@angular/core';
import { TestBed } from '@angular/core/testing';

import { Fold } from './fold';

@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [Fold],
  template: `
    <app-fold title="Ganchos e anotações" [level]="3" [foldable]="foldable" [(open)]="open">
      <span foldNote>Só você vê</span>
      <p class="body">O texto</p>
    </app-fold>
  `,
})
class Host {
  foldable = true;
  open = false;
}

@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [Fold],
  template: `
    <app-fold title="Efeitos" [keep]="true" [(open)]="open">
      <p class="body">A lista</p>
    </app-fold>
  `,
})
class KeepHost {
  open = false;
}

describe('Fold', () => {
  function setup(patch: Partial<Host> = {}) {
    const fixture = TestBed.createComponent(Host);
    Object.assign(fixture.componentInstance, patch);
    fixture.detectChanges();
    const el = fixture.nativeElement as HTMLElement;
    return { fixture, el, button: () => el.querySelector<HTMLButtonElement>('button') };
  }

  it('starts folded: a button with aria-expanded false, the note beside it, no body', () => {
    const { el, button } = setup();
    expect(button()?.getAttribute('aria-expanded')).toBe('false');
    expect(button()?.textContent).toContain('Ganchos e anotações');
    expect(el.querySelector('.mr-fold__note')?.textContent).toBe('Só você vê');
    expect(el.querySelector('.body')).toBeNull();
  });

  it('is a panel with a heading of the level asked for', () => {
    const { el } = setup();
    expect(el.querySelector('app-fold')?.classList.contains('mr-panel--fold')).toBe(true);
    expect(el.querySelector('[role="heading"]')?.getAttribute('aria-level')).toBe('3');
  });

  it('opens and folds on the button, and tells the caller', () => {
    const { fixture, el, button } = setup();
    button()?.click();
    fixture.detectChanges();
    expect(button()?.getAttribute('aria-expanded')).toBe('true');
    expect(el.querySelector('.body')?.textContent).toBe('O texto');
    expect(fixture.componentInstance.open).toBe(true);
    expect(button()?.getAttribute('aria-controls')).toBe(el.querySelector('.mr-fold__body')?.id);
    button()?.click();
    fixture.detectChanges();
    expect(el.querySelector('.body')).toBeNull();
  });

  it('can be opened from outside', () => {
    const { fixture, el } = setup({ open: true });
    expect(el.querySelector('.body')).not.toBeNull();
    expect(fixture.nativeElement.querySelector('button').getAttribute('aria-expanded')).toBe(
      'true',
    );
  });

  it('is a plain title with the body always shown when it does not fold', () => {
    const { el, button } = setup({ foldable: false });
    expect(button()).toBeNull();
    expect(el.querySelector('.mr-fold__title')?.textContent).toBe('Ganchos e anotações');
    expect(el.querySelector('.body')).not.toBeNull();
  });

  it('has the quiet group row look, with an icon and the muted words', () => {
    const fixture = TestBed.createComponent(Fold);
    fixture.componentRef.setInput('title', 'Ação bônus · Usada');
    fixture.componentRef.setInput('appearance', 'row');
    fixture.componentRef.setInput('icon', 'change_history');
    fixture.componentRef.setInput('muted', true);
    fixture.detectChanges();
    const host = fixture.nativeElement as HTMLElement;
    expect(host.classList).toContain('mr-panel--fold-row');
    expect(host.classList).toContain('mr-fold--muted');
    expect(host.querySelector('.mr-fold__icon')?.textContent).toBe('change_history');
  });

  it('keeps the body mounted, hidden, when asked to (a panel that counts while folded)', () => {
    const fixture = TestBed.createComponent(KeepHost);
    fixture.detectChanges();
    const body = (fixture.nativeElement as HTMLElement).querySelector<HTMLElement>(
      '.mr-fold__body',
    )!;
    expect(body.hidden).toBe(true);
    expect(body.querySelector('.body')).not.toBeNull();
    (fixture.nativeElement as HTMLElement).querySelector('button')!.click();
    fixture.detectChanges();
    expect(body.hidden).toBe(false);
  });
});
