import { TestBed } from '@angular/core/testing';

import { freeRoll } from '../../../core/free-dice/free-dice-testing';
import { FreeRollLog } from './free-roll-log';

const text = (el: Element | null | undefined) =>
  (el?.textContent ?? '').replace(/\s+/g, ' ').trim();

function mount(inputs: Record<string, unknown>) {
  const fixture = TestBed.createComponent(FreeRollLog);
  fixture.componentRef.setInput('rolls', []);
  fixture.componentRef.setInput('listState', 'ready');
  for (const [k, v] of Object.entries(inputs)) {
    fixture.componentRef.setInput(k, v);
  }
  fixture.detectChanges();
  return { fixture, el: fixture.nativeElement as HTMLElement };
}

describe('FreeRollLog', () => {
  it('is a section named "Registro da sessão"', () => {
    const { el } = mount({});
    const section = el.querySelector('section');
    expect(text(section?.querySelector('h2'))).toBe('Registro da sessão');
    expect(section?.getAttribute('aria-labelledby')).toBe(section?.querySelector('h2')?.id);
  });

  it('says in the app\'s tone that nobody rolled yet', () => {
    const { el } = mount({});
    expect(text(el)).toContain('Ninguém rolou dados nesta sessão ainda. As rolagens da mesa aparecem aqui.');
  });

  it('lists the rolls newest first with the roller in bold', () => {
    const { el } = mount({
      rolls: [
        freeRoll('a', { logText: 'Rolagem livre · Brisa: 2d6 + 3 = 11 (4, 4)' }),
        freeRoll('b', { logText: 'Rolagem livre · Tor: 1d20 + 6 = 19 (13)' }),
      ],
    });
    const rows = Array.from(el.querySelectorAll('li'));
    expect(rows.map(text)).toEqual([
      'Rolagem livre · Brisa: 2d6 + 3 = 11 (4, 4)',
      'Rolagem livre · Tor: 1d20 + 6 = 19 (13)',
    ]);
    expect(text(rows[0].querySelector('strong'))).toBe('Rolagem livre · Brisa:');
  });

  it('marks a roll only the master sees with "Só você"', () => {
    const { el } = mount({
      isMaster: true,
      rolls: [freeRoll('s', { secret: true, logText: 'Rolagem livre · Mestre: 1d20 = 14 (14)' })],
    });
    expect(text(el.querySelector('li'))).toContain('Só você');
  });

  it('shows the newest ten and the rest on request', () => {
    const rolls = Array.from({ length: 14 }, (_, i) => freeRoll(`r${i}`, { logText: `Rolagem livre · Brisa: roll ${i}` }));
    const { fixture, el } = mount({ rolls });
    expect(el.querySelectorAll('li')).toHaveLength(10);
    const more = Array.from(el.querySelectorAll('button')).find((b) => text(b).includes('Mostrar as outras 4'));
    expect(more).toBeDefined();
    more!.click();
    fixture.detectChanges();
    expect(el.querySelectorAll('li')).toHaveLength(14);
  });

  it('says the log could not load, with "Tentar de novo", and says it is loading', () => {
    const failed = mount({ listState: 'failed' });
    expect(text(failed.el)).toContain('Não deu para carregar o registro.');
    let retried = 0;
    failed.fixture.componentInstance.retry.subscribe(() => retried++);
    Array.from(failed.el.querySelectorAll('button')).find((b) => text(b) === 'Tentar de novo')!.click();
    expect(retried).toBe(1);
    const loading = mount({ listState: 'loading' });
    expect(text(loading.el)).toContain('Carregando o registro…');
  });
});
