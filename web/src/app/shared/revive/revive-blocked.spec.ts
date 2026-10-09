import { TestBed } from '@angular/core/testing';
import { provideRouter } from '@angular/router';

import { ReviveBlocked } from './revive-blocked';

describe('ReviveBlocked', () => {
  function render(playerName?: string) {
    TestBed.configureTestingModule({ providers: [provideRouter([])] });
    const fixture = TestBed.createComponent(ReviveBlocked);
    fixture.componentRef.setInput('deadName', 'Toren');
    fixture.componentRef.setInput('livingName', 'Nuvem');
    fixture.componentRef.setInput('livingLink', ['/campaigns', 'camp-1', 'characters', 'nuvem-1']);
    if (playerName) {
      fixture.componentRef.setInput('playerName', playerName);
    }
    fixture.detectChanges();
    return { fixture, el: fixture.nativeElement as HTMLElement };
  }

  it('is an alert that says who has another living character and what the master does', () => {
    const { el } = render('Davi');
    const alert = el.querySelector('[role="alert"]')!;
    expect(alert.querySelector('h2')?.textContent).toBe('Davi já tem outro personagem vivo');
    expect(alert.textContent!.replace(/\s+/g, ' ')).toContain(
      'Cada jogador tem um personagem vivo por campanha, e Davi criou Nuvem depois da morte de Toren. Para reviver Toren, o mestre arquiva Nuvem ou a marca como morta primeiro.',
    );
  });

  it('links to the living character and can be cancelled', () => {
    const { fixture, el } = render('Davi');
    const cancelled = vi.fn();
    fixture.componentInstance.cancelled.subscribe(cancelled);

    const link = el.querySelector('a')!;
    expect(link.textContent?.trim()).toBe('Abrir Nuvem');
    expect(link.getAttribute('href')).toBe('/campaigns/camp-1/characters/nuvem-1');
    el.querySelector('button')!.click();
    expect(cancelled).toHaveBeenCalledTimes(1);
  });

  it('says "o jogador" when the page does not know the name', () => {
    const { el } = render();
    expect(el.querySelector('h2')?.textContent).toBe('o jogador já tem outro personagem vivo');
  });
});
