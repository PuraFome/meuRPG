import { TestBed } from '@angular/core/testing';
import { provideRouter } from '@angular/router';

import { AfterDeath } from './after-death';

describe('AfterDeath', () => {
  function render() {
    TestBed.configureTestingModule({ providers: [provideRouter([])] });
    const fixture = TestBed.createComponent(AfterDeath);
    fixture.componentRef.setInput('characterName', 'Toren');
    fixture.componentRef.setInput('campaignId', 'camp-1');
    fixture.detectChanges();
    return fixture.nativeElement as HTMLElement;
  }

  it('says what comes next, in the words of the design', () => {
    const el = render();
    expect(el.querySelector('h2')?.textContent).toBe('E agora?');
    expect(el.querySelector('p')?.textContent!.replace(/\s+/g, ' ').trim()).toBe(
      'Você pode criar um novo personagem nesta campanha. Ele precisa da aprovação do mestre, como qualquer ficha nova. Toren fica guardado.',
    );
  });

  it('links "Criar um novo personagem" to the creation of a character of the campaign', () => {
    const link = render().querySelector('a')!;
    expect(link.textContent?.trim()).toBe('Criar um novo personagem');
    expect(link.getAttribute('href')).toBe('/campaigns/camp-1/characters/new');
  });
});
