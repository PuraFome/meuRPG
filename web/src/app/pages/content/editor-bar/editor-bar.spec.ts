import { TestBed } from '@angular/core/testing';

import { EntrySaver } from '../../../core/content/entry-saver';
import { EditorAlerts, EditorBar } from './editor-bar';

describe('EditorAlerts: the stale entry', () => {
  it('is said by an alert region that was there first, the notice without a role', () => {
    const saver = new EntrySaver({ aOne: 'uma magia' }, 'a magia');
    const fixture = TestBed.createComponent(EditorAlerts);
    fixture.componentRef.setInput('saver', saver);
    fixture.detectChanges();
    const el = fixture.nativeElement as HTMLElement;
    const live = el.querySelector('.mr-live[role="alert"]')!;
    expect(live.textContent?.trim()).toBe('');
    saver.stale.set(true);
    fixture.detectChanges();
    const notice = live.querySelector('.mr-notice--danger')!;
    expect(notice.textContent).toContain('Esta entrada mudou enquanto você editava.');
    expect(notice.hasAttribute('role')).toBe(false);
  });
});

describe('EditorBar: the line beside the buttons', () => {
  const bar = (inputs: Record<string, unknown>) => {
    const fixture = TestBed.createComponent(EditorBar);
    fixture.componentRef.setInput('saveLabel', 'Salvar criatura');
    for (const [k, v] of Object.entries(inputs)) fixture.componentRef.setInput(k, v);
    fixture.detectChanges();
    return (fixture.nativeElement as HTMLElement).querySelector('.bar__note')!.textContent;
  };

  it('says that a change is live, or the hint of the editor, or why saving is off', () => {
    expect(bar({})).toBe('Mudar vale na hora, também nas fichas travadas.');
    expect(bar({ hint: 'Quem já tem a criatura fica com os números que recebeu.' })).toBe(
      'Quem já tem a criatura fica com os números que recebeu.',
    );
    expect(bar({ hint: 'Dica.', blocked: 'Responda à pergunta.' })).toBe('Responda à pergunta.');
  });
});
