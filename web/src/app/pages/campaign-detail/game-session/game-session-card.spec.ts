import { Injectable } from '@angular/core';
import { ComponentFixture, TestBed } from '@angular/core/testing';
import { Code, ConnectError } from '@connectrpc/connect';

import { GameSessionCard } from './game-session-card';
import { GameSessionSource, GameSessionVm, StartGameSessionResultVm } from './game-session-card.types';

@Injectable()
class FakeGameSessionSource {
  getCurrentSessionResult: Promise<GameSessionVm | null> = Promise.resolve(null);
  readonly startGameSession = vi.fn();
  readonly endGameSession = vi.fn();

  getCurrentSession(): Promise<GameSessionVm | null> {
    return this.getCurrentSessionResult;
  }
}

function session(sessionNumber: number, id = `sess-${sessionNumber}`): GameSessionVm {
  return { id, sessionNumber, startedAt: new Date('2026-09-29T12:00:00Z'), endedAt: null };
}

function startResult(sessionNumber: number, lockedSheetCount: number): StartGameSessionResultVm {
  return { session: session(sessionNumber), lockedSheetCount };
}

describe('GameSessionCard', () => {
  let fake: FakeGameSessionSource;

  beforeEach(() => {
    TestBed.configureTestingModule({
      imports: [GameSessionCard],
      providers: [{ provide: GameSessionSource, useClass: FakeGameSessionSource }],
    });
    fake = TestBed.inject(GameSessionSource) as unknown as FakeGameSessionSource;
  });

  async function render(): Promise<{ el: HTMLElement; fixture: ComponentFixture<GameSessionCard> }> {
    const fixture = TestBed.createComponent(GameSessionCard);
    fixture.componentRef.setInput('campaignId', 'camp-1');
    fixture.detectChanges();
    await fixture.whenStable();
    fixture.detectChanges();
    return { el: fixture.nativeElement as HTMLElement, fixture };
  }

  it('shows "Iniciar sessão" when no session is open', async () => {
    fake.getCurrentSessionResult = Promise.resolve(null);
    const { el } = await render();
    expect(el.textContent).toContain('Iniciar sessão');
    expect(el.textContent).not.toContain('Encerrar sessão');
  });

  it('shows "Sessão N em andamento" and "Encerrar sessão" when one is open', async () => {
    fake.getCurrentSessionResult = Promise.resolve(session(3));
    const { el } = await render();
    expect(el.textContent).toContain('Sessão 3 em andamento');
    expect(el.textContent).toContain('Encerrar sessão');
    expect(el.textContent).not.toContain('Iniciar sessão');
  });

  it('starts a session, switches to "em andamento", and shows the locked-sheet count', async () => {
    fake.getCurrentSessionResult = Promise.resolve(null);
    fake.startGameSession.mockResolvedValue(startResult(1, 4));
    const { el, fixture } = await render();

    const button = Array.from(el.querySelectorAll('button')).find((b) =>
      b.textContent?.includes('Iniciar sessão'),
    ) as HTMLButtonElement;
    button.click();
    await fixture.whenStable();
    fixture.detectChanges();

    expect(fake.startGameSession).toHaveBeenCalledWith('camp-1');
    expect(el.textContent).toContain('Sessão 1 em andamento');
    expect(el.textContent).toContain('4 fichas travadas.');
  });

  it('shows a clear message when starting fails because one is already open', async () => {
    fake.getCurrentSessionResult = Promise.resolve(null);
    fake.startGameSession.mockRejectedValue(new ConnectError('open', Code.FailedPrecondition));
    const { el, fixture } = await render();

    const button = Array.from(el.querySelectorAll('button')).find((b) =>
      b.textContent?.includes('Iniciar sessão'),
    ) as HTMLButtonElement;
    button.click();
    await fixture.whenStable();
    fixture.detectChanges();

    expect(el.textContent).toContain('Já existe uma sessão em andamento nesta campanha.');
  });

  it('ends a session (by its id) and switches back to the "nenhuma sessão" state', async () => {
    fake.getCurrentSessionResult = Promise.resolve(session(2, 'sess-2'));
    fake.endGameSession.mockResolvedValue({ ...session(2, 'sess-2'), endedAt: new Date() });
    const { el, fixture } = await render();

    const button = Array.from(el.querySelectorAll('button')).find((b) =>
      b.textContent?.includes('Encerrar sessão'),
    ) as HTMLButtonElement;
    button.click();
    await fixture.whenStable();
    fixture.detectChanges();

    expect(fake.endGameSession).toHaveBeenCalledWith('camp-1', 'sess-2');
    expect(el.textContent).toContain('Nenhuma sessão em andamento');
  });
});
