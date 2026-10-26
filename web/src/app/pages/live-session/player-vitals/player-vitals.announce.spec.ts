import { ComponentFixture, TestBed } from '@angular/core/testing';
import { provideRouter } from '@angular/router';

import { pensantusVitals } from '../../../core/play/live-testing';
import { PlayerVitals } from './player-vitals';

const hit = { id: 'e1', text: 'A Espada voadora 2 acertou você: 9 de dano cortante.' };

describe('PlayerVitals: what the card says when the hit points change', () => {
  function setup() {
    TestBed.configureTestingModule({ providers: [provideRouter([])] });
    const fixture = TestBed.createComponent(PlayerVitals);
    fixture.componentRef.setInput('campaignId', 'camp');
    fixture.componentRef.setInput('sheet', { armorClass: 15, summary: 'Mago 3', senses: [] });
    const vitals = (revision: number, current: number) =>
      fixture.componentRef.setInput(
        'vitals',
        pensantusVitals({ revision, hitPointsCurrent: current, hitPointsMax: 21 }),
      );
    vitals(1, 21);
    fixture.detectChanges();
    return { fixture, vitals };
  }
  const say = (f: ComponentFixture<PlayerVitals>) =>
    (f.nativeElement as HTMLElement)
      .querySelector('[role=status].mr-visually-hidden')
      ?.textContent?.trim();

  it("says the attack's damage when an attack hurt the character", () => {
    const { fixture, vitals } = setup();
    fixture.componentRef.setInput('lastHit', hit);
    vitals(2, 12);
    fixture.detectChanges();
    expect(say(fixture)).toBe(
      'A Espada voadora 2 acertou você: 9 de dano cortante. 12 de 21 pontos de vida, 0 temporários.',
    );
  });

  it('keeps "O mestre ajustou" for the master\'s own adjustment, also after a hit was announced', () => {
    const { fixture, vitals } = setup();
    fixture.componentRef.setInput('lastHit', hit);
    vitals(2, 12);
    fixture.detectChanges();
    vitals(3, 21);
    fixture.detectChanges();
    expect(say(fixture)).toBe('O mestre ajustou: 21 de 21 pontos de vida, 0 temporários.');
  });

  it('turns the adjustment into the attack when the log arrives just after the hit points', () => {
    const { fixture, vitals } = setup();
    vitals(2, 12);
    fixture.detectChanges();
    expect(say(fixture)).toContain('O mestre ajustou');
    fixture.componentRef.setInput('lastHit', hit);
    fixture.detectChanges();
    expect(say(fixture)).toBe(
      'A Espada voadora 2 acertou você: 9 de dano cortante. 12 de 21 pontos de vida, 0 temporários.',
    );
  });

  it('does not announce an old hit again when the log is read again', () => {
    const { fixture, vitals } = setup();
    fixture.componentRef.setInput('lastHit', hit);
    vitals(2, 12);
    fixture.detectChanges();
    fixture.componentRef.setInput('lastHit', { ...hit });
    fixture.detectChanges();
    expect(say(fixture)).toContain('acertou você');
    vitals(3, 14);
    fixture.detectChanges();
    expect(say(fixture)).toBe('O mestre ajustou: 14 de 21 pontos de vida, 0 temporários.');
  });
});
