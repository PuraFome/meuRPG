import { TestBed } from '@angular/core/testing';
import { Code, ConnectError } from '@connectrpc/connect';

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

describe('EditorBar: while the save is on its way', () => {
  it('draws both buttons dashed and off, with "Salvando...", and the alert draws a cloud when the server is out of the air', async () => {
    const fixture = TestBed.createComponent(EditorBar);
    fixture.componentRef.setInput('saveLabel', 'Salvar talento');
    fixture.componentRef.setInput('saving', true);
    fixture.detectChanges();
    const el = fixture.nativeElement as HTMLElement;
    const buttons = [...el.querySelectorAll('button')];
    expect(buttons.map((b) => b.textContent?.replace(/\s+/g, ' ').trim())).toEqual([
      'saveSalvando...',
      'Cancelar',
    ]);
    for (const b of buttons) {
      expect(b.classList.contains('mr-button--off')).toBe(true);
      expect(b.getAttribute('aria-disabled')).toBe('true');
    }

    const saver = new EntrySaver({ aOne: 'um talento' }, 'o talento');
    await saver.run(
      () => Promise.reject(new ConnectError('down', Code.Unavailable)),
      () => true,
    );
    const alerts = TestBed.createComponent(EditorAlerts);
    alerts.componentRef.setInput('saver', saver);
    alerts.detectChanges();
    const notice = (alerts.nativeElement as HTMLElement).querySelector('.mr-notice--danger')!;
    expect(notice.textContent).toContain(
      'Não deu para salvar o talento agora. Nada mudou. Tente de novo.',
    );
    expect(notice.querySelector('mat-icon')?.textContent).toBe('cloud_off');
  });
});

describe('EditorBar: the failed save', () => {
  it('draws the notice directly above the buttons, with the cloud when the server is out of the air', () => {
    const fixture = TestBed.createComponent(EditorBar);
    fixture.componentRef.setInput('saveLabel', 'Salvar talento');
    fixture.componentRef.setInput(
      'error',
      'Não deu para salvar o talento agora. Nada mudou. Tente de novo.',
    );
    fixture.componentRef.setInput('offline', true);
    fixture.detectChanges();
    const el = fixture.nativeElement as HTMLElement;
    const notice = el.querySelector('.mr-notice--danger')!;
    expect(notice.textContent).toContain('Não deu para salvar o talento agora.');
    expect(notice.querySelector('mat-icon')?.textContent).toBe('cloud_off');
    expect(notice.nextElementSibling?.classList.contains('bar')).toBe(true);
  });

  it('draws no notice when nothing failed', () => {
    const fixture = TestBed.createComponent(EditorBar);
    fixture.componentRef.setInput('saveLabel', 'Salvar talento');
    fixture.detectChanges();
    expect((fixture.nativeElement as HTMLElement).querySelector('.mr-notice--danger')).toBeNull();
  });
});

describe('EditorAlerts: the notice at the foot', () => {
  it('leaves the failure to the bar when the editor asks', async () => {
    const saver = new EntrySaver({ aOne: 'um talento' }, 'o talento');
    await saver.run(
      () => Promise.reject(new ConnectError('down', Code.Unavailable)),
      () => true,
    );
    const alerts = TestBed.createComponent(EditorAlerts);
    alerts.componentRef.setInput('saver', saver);
    alerts.componentRef.setInput('errorAtFoot', true);
    alerts.detectChanges();
    expect((alerts.nativeElement as HTMLElement).querySelector('.mr-notice--danger')).toBeNull();
  });
});
