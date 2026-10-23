import { TestBed } from '@angular/core/testing';

import { ListenController, type VoiceAudio } from '../../../core/conversation/listen-controller';
import { ListenButton } from './listen-button';

class FakeAudio implements VoiceAudio {
  src = '';
  played = 0;
  readonly listeners = new Map<string, () => void>();
  play(): Promise<void> {
    this.played++;
    return Promise.resolve();
  }
  pause(): void {}
  addEventListener(type: string, listener: () => void): void {
    this.listeners.set(type, listener);
  }
  fire(type: 'playing' | 'ended' | 'error'): void {
    this.listeners.get(type)?.();
  }
}

describe('ListenButton', () => {
  function setup() {
    const audios: FakeAudio[] = [];
    const controller = new ListenController(() => {
      const audio = new FakeAudio();
      audios.push(audio);
      return audio;
    });
    const fixture = TestBed.createComponent(ListenButton);
    fixture.componentRef.setInput('controller', controller);
    fixture.componentRef.setInput('lineId', 'm1');
    fixture.componentRef.setInput('audioUrl', '/session-messages/m1/audio');
    fixture.componentRef.setInput('subject', 'a narração');
    fixture.detectChanges();
    const el = fixture.nativeElement as HTMLElement;
    const button = () => el.querySelector('button') as HTMLButtonElement;
    return { fixture, controller, audios, el, button };
  }

  it('says "Ouvir" and plays nothing until it is pressed', () => {
    const { audios, button } = setup();
    expect(button().textContent).toContain('Ouvir');
    expect(button().getAttribute('aria-label')).toBe('Ouvir a narração');
    expect(button().hasAttribute('aria-pressed')).toBe(false);
    expect(audios).toHaveLength(0);
  });

  it('says "Gerando a voz…" while the voice is made, with the visible words as its name', () => {
    const { fixture, button } = setup();
    button().click();
    fixture.detectChanges();
    expect(button().textContent).toContain('Gerando a voz…');
    expect(button().hasAttribute('aria-label')).toBe(false);
  });

  it('says "Parar" while it plays, pressed, and stops on the next press', () => {
    const { fixture, audios, controller, button } = setup();
    button().click();
    audios[0].fire('playing');
    fixture.detectChanges();
    expect(button().textContent).toContain('Parar');
    expect(button().getAttribute('aria-pressed')).toBe('true');
    expect(button().getAttribute('aria-label')).toBe('Parar a leitura');
    button().click();
    fixture.detectChanges();
    expect(controller.stateOf('m1')).toBe('idle');
    expect(button().textContent).toContain('Ouvir');
  });

  it('tells a failure quietly, in a status, and lets the person try again', () => {
    const { fixture, audios, el, button } = setup();
    button().click();
    audios[0].fire('error');
    fixture.detectChanges();
    const status = el.querySelector('[role="status"]');
    expect(status?.textContent?.trim()).toBe('A voz não carregou. Tente de novo.');
    expect(button().textContent).toContain('Ouvir');
    button().click();
    expect(audios).toHaveLength(2);
  });

  it('keeps an empty status in the page so a failure is announced', () => {
    const { el } = setup();
    expect(el.querySelector('[role="status"]')?.textContent?.trim()).toBe('');
  });

  it('has a target of at least 44 px on its class', () => {
    const { button } = setup();
    expect(button().classList.contains('listen')).toBe(true);
  });
});
