import { TestBed } from '@angular/core/testing';

import { PendingLine } from './pending-line';
import type { PendingSend } from '../../../core/conversation/conversation-state';
import type { SendSessionMessageRequest } from '../../../../gen/meurpg/play/v1/session_messages_pb';

function pending(status: PendingSend['status'], error = ''): PendingSend {
  return {
    key: 'k1',
    request: {} as SendSessionMessageRequest,
    text: 'Eu vou até a porta dos fundos.',
    status,
    error,
  };
}

describe('PendingLine', () => {
  function render(p: PendingSend) {
    const fixture = TestBed.createComponent(PendingLine);
    fixture.componentRef.setInput('pending', p);
    fixture.componentRef.setInput('label', 'Pensantus · você');
    const retried = vi.fn();
    fixture.componentInstance.retry.subscribe(retried);
    fixture.detectChanges();
    return { el: fixture.nativeElement as HTMLElement, retried };
  }

  it('shows the line as the own one while it goes', () => {
    const { el } = render(pending('sending'));
    expect(el.querySelector('.line__bubble')?.textContent).toBe('Eu vou até a porta dos fundos.');
    expect(el.querySelector('[role="alert"]')).toBeNull();
    expect(el.querySelector('button')).toBeNull();
  });

  it('keeps the line and tells, in a danger notice, that it did not go, with "Tentar de novo"', () => {
    const { el, retried } = render(pending('failed', 'Não foi enviada. A conexão caiu.'));
    expect(el.querySelector('.line__bubble')?.textContent).toBe('Eu vou até a porta dos fundos.');
    const notice = el.querySelector('[role="alert"]') as HTMLElement;
    expect(notice.querySelector('strong')?.textContent).toBe('Não foi enviada.');
    expect(notice.textContent).toContain('A conexão caiu.');
    (el.querySelector('button') as HTMLButtonElement).click();
    expect(el.querySelector('button')?.textContent).toContain('Tentar de novo');
    expect(retried).toHaveBeenCalledTimes(1);
  });
});
