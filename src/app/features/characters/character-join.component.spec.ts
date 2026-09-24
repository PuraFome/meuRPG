import { render, screen, waitFor } from '@testing-library/angular';
import userEvent from '@testing-library/user-event';
import { of, throwError } from 'rxjs';
import { CharacterJoinComponent } from './character-join.component';
import { CharactersService } from '../../core/services/characters.service';
import { AuthService } from '../../core/auth/auth.service';
import { Character } from '../../core/models/character';

const TOKEN = 'valid-token-123';
const CHARACTER_NAME = 'Gandalf';

function mockCharacter(name: string): Character {
  return {
    id: 'char-1',
    name,
    description: '',
    type: 'player',
    attributes: { for: 10, des: 10, con: 10, int: 10, sab: 10, car: 10 },
    skills: [],
    inventory: [],
    quotes: [],
    createdAt: new Date(),
    updatedAt: new Date(),
  };
}

describe('CharacterJoinComponent', () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  it('renders the character form in join mode when the token is valid', async () => {
    const validateJoinToken = vi.fn().mockReturnValue(
      of({ valid: true, expiresAt: '2026-10-01T12:00:00.000Z' }),
    );
    const joinFn = vi.fn().mockReturnValue(of(mockCharacter(CHARACTER_NAME)));

    await render(CharacterJoinComponent, {
      componentInputs: { token: TOKEN },
      providers: [
        {
          provide: CharactersService,
          useValue: { validateJoinToken, join: joinFn },
        },
        { provide: AuthService, useValue: { load: vi.fn() } },
      ],
    });

    await waitFor(() => {
      expect(validateJoinToken).toHaveBeenCalledWith(TOKEN);
    });

    expect(screen.getByLabelText('Nome')).toBeTruthy();
  });

  it('renders the expired state when validateJoinToken returns a 410 error', async () => {
    const validateJoinToken = vi.fn().mockReturnValue(
      throwError(() => ({ status: 410 })),
    );

    await render(CharacterJoinComponent, {
      componentInputs: { token: TOKEN },
      providers: [
        {
          provide: CharactersService,
          useValue: { validateJoinToken, join: vi.fn() },
        },
        { provide: AuthService, useValue: { load: vi.fn() } },
      ],
    });

    await waitFor(() => {
      expect(screen.getByText('Link expirado ou inválido')).toBeTruthy();
    });

    expect(screen.queryByLabelText('Nome')).toBeNull();
  });

  it('shows the success panel after the form emits saved', async () => {
    const user = userEvent.setup();
    const validateJoinToken = vi.fn().mockReturnValue(
      of({ valid: true, expiresAt: '2026-10-01T12:00:00.000Z' }),
    );
    const character = mockCharacter(CHARACTER_NAME);
    const joinFn = vi.fn().mockReturnValue(of(character));

    await render(CharacterJoinComponent, {
      componentInputs: { token: TOKEN },
      providers: [
        {
          provide: CharactersService,
          useValue: { validateJoinToken, join: joinFn },
        },
        { provide: AuthService, useValue: { load: vi.fn() } },
      ],
    });

    await waitFor(() => {
      expect(screen.getByLabelText('Nome')).toBeTruthy();
    });

    const nameInput = screen.getByLabelText('Nome');
    await user.type(nameInput, CHARACTER_NAME);

    const saveButton = screen.getByRole('button', { name: /salvar/i });
    await user.click(saveButton);

    await waitFor(() => {
      expect(screen.getByText(`Personagem ${CHARACTER_NAME} enviado!`)).toBeTruthy();
    });
  });
});
