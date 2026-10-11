import { signal } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { vi } from 'vitest';

import { Nudger } from '../../../core/combat/nudger';
import { NudgeButton } from './nudge-button';

class FakeNudger {
  readonly cooling = signal<ReadonlySet<string>>(new Set());
  readonly message = signal('');
  readonly messageFor = signal('');
  readonly call = vi.fn().mockResolvedValue(undefined);
}

function mount(compact = false) {
  const nudger = new FakeNudger();
  TestBed.configureTestingModule({ providers: [{ provide: Nudger, useValue: nudger }] });
  const fixture = TestBed.createComponent(NudgeButton);
  fixture.componentRef.setInput('campaignId', 'camp');
  fixture.componentRef.setInput('characterId', 'pens');
  fixture.componentRef.setInput('name', 'Pensantus');
  fixture.componentRef.setInput('compact', compact);
  fixture.detectChanges();
  const el = fixture.nativeElement as HTMLElement;
  return { fixture, el, nudger, button: () => el.querySelector<HTMLButtonElement>('button')! };
}

describe('NudgeButton', () => {
  it('is named for the character and calls the client with the campaign and the character', () => {
    const { button, nudger } = mount();
    expect(button().getAttribute('aria-label')).toBe('Chamar a atenção de Pensantus');
    expect(button().textContent).toContain('Chamar a atenção');
    button().click();
    expect(nudger.call).toHaveBeenCalledWith('camp', 'pens');
  });

  it('has an icon-only form with the same name', () => {
    const { button } = mount(true);
    expect(button().getAttribute('aria-label')).toBe('Chamar a atenção de Pensantus');
    expect(button().textContent).not.toContain('Chamar a atenção');
  });

  it('is off while this character is in the wait, and not for another', () => {
    const { fixture, button, nudger } = mount();
    nudger.cooling.set(new Set(['toren']));
    fixture.detectChanges();
    expect(button().disabled).toBe(false);
    nudger.cooling.set(new Set(['pens']));
    fixture.detectChanges();
    expect(button().disabled).toBe(true);
  });

  it('says the confirmation or the refusal in its polite live region, only for its character', () => {
    const { fixture, el, nudger } = mount();
    const live = el.querySelector('[role=status]')!;
    expect(live.getAttribute('aria-live')).toBe('polite');
    nudger.messageFor.set('toren');
    nudger.message.set('Chamado.');
    fixture.detectChanges();
    expect(live.textContent?.trim()).toBe('');
    nudger.messageFor.set('pens');
    nudger.message.set('Espere alguns segundos para chamar de novo.');
    fixture.detectChanges();
    expect(live.textContent?.trim()).toBe('Espere alguns segundos para chamar de novo.');
  });
});
