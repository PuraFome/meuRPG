import { render, screen, waitFor } from '@testing-library/angular';
import userEvent from '@testing-library/user-event';
import { of, throwError } from 'rxjs';
import { CharacterInvitesComponent } from './character-invites.component';
import { CharactersService } from '../../core/services/characters.service';
import { environment } from '../../../environments/environment';

const PROD_INVITE_BASE = 'https://purafome.github.io/meuRPG/';
const TOKEN = 'abc123token';
const EXPECTED_LINK = `${PROD_INVITE_BASE}#/personagens/convidar/${TOKEN}`;

function renderInvites(createJoinToken: ReturnType<typeof vi.fn>) {
  return render(CharacterInvitesComponent, {
    providers: [{ provide: CharactersService, useValue: { createJoinToken } }],
  });
}

describe('CharacterInvitesComponent', () => {
  const originalInviteBaseUrl = environment.inviteBaseUrl;

  beforeEach(() => {
    environment.inviteBaseUrl = PROD_INVITE_BASE;
  });

  afterEach(() => {
    environment.inviteBaseUrl = originalInviteBaseUrl;
    vi.restoreAllMocks();
  });

  it('shows a hint and no link before generating', async () => {
    const createJoinToken = vi.fn().mockReturnValue(of({}));
    await renderInvites(createJoinToken);

    expect(screen.getByText(/nenhum link gerado/i)).toBeTruthy();
    expect(createJoinToken).not.toHaveBeenCalled();
  });

  it('generates and renders the exact prod hash link', async () => {
    const user = userEvent.setup();
    const createJoinToken = vi.fn().mockReturnValue(
      of({ token: TOKEN, expiresAt: '2026-10-01T12:00:00.000Z', type: 'player' }),
    );
    await renderInvites(createJoinToken);

    await user.click(screen.getByRole('button', { name: /enviar para jogador/i }));

    expect(createJoinToken).toHaveBeenCalledTimes(1);

    await waitFor(() => {
      const field = screen.getByLabelText('Link de convite') as HTMLInputElement;
      expect(field.value).toBe(EXPECTED_LINK);
    });

    expect(screen.queryByText(/não foi possível gerar/i)).toBeNull();
  });

  it('copies the generated link to the clipboard and confirms', async () => {
    const user = userEvent.setup();
    const writeText = vi.fn().mockResolvedValue(undefined);
    Object.defineProperty(navigator, 'clipboard', {
      value: { writeText },
      configurable: true,
      writable: true,
    });

    const createJoinToken = vi.fn().mockReturnValue(
      of({ token: TOKEN, expiresAt: '2026-10-01T12:00:00.000Z', type: 'player' }),
    );
    await renderInvites(createJoinToken);

    await user.click(screen.getByRole('button', { name: /enviar para jogador/i }));

    await waitFor(() => {
      expect(screen.getByRole('button', { name: /copiar/i })).toBeTruthy();
    });

    await user.click(screen.getByRole('button', { name: /copiar/i }));

    expect(writeText).toHaveBeenCalledWith(EXPECTED_LINK);

    await waitFor(() => {
      expect(screen.getByText(/link copiado!/i)).toBeTruthy();
    });
  });

  it('renders an error and no link when the API fails', async () => {
    const user = userEvent.setup();
    const createJoinToken = vi
      .fn()
      .mockReturnValue(throwError(() => new Error('boom')));
    await renderInvites(createJoinToken);

    await user.click(screen.getByRole('button', { name: /enviar para jogador/i }));

    await waitFor(() => {
      expect(screen.getByText(/não foi possível gerar o link/i)).toBeTruthy();
    });

    expect(screen.queryByLabelText('Link de convite')).toBeNull();
    expect(screen.queryByRole('button', { name: /copiar/i })).toBeNull();
  });
});
