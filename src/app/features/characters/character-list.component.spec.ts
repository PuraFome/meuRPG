import { render, screen, waitFor } from '@testing-library/angular';
import userEvent from '@testing-library/user-event';
import { CharacterListComponent } from './character-list.component';
import { StoreService } from '../../core/store/store.service';
import { of } from 'rxjs';

function mockCharacter(overrides: Partial<{
  id: string;
  name: string;
  description: string;
  type: 'npc' | 'player' | 'boss' | 'minion';
}> = {}) {
  return {
    id: '1',
    name: 'Zagreus',
    description: 'Filho de Hades',
    type: 'player' as const,
    attributes: {} as Record<string, number>,
    skills: [] as string[],
    inventory: [] as string[],
    quotes: [] as string[],
    createdAt: new Date('2026-01-01'),
    updatedAt: new Date('2026-01-01'),
    ...overrides,
  };
}

const mockCharacters = [
  mockCharacter({ id: '1', name: 'Zagreus', type: 'player' }),
  mockCharacter({ id: '2', name: 'Cerberus', type: 'boss' }),
  mockCharacter({ id: '3', name: 'Oráculo', type: 'npc' }),
  mockCharacter({ id: '4', name: 'Lacaio', type: 'minion' }),
];

describe('CharacterListComponent', () => {
  it('renders empty state when no characters', async () => {
    await render(CharacterListComponent, {
      providers: [
        {
          provide: StoreService,
          useValue: { getAll: vi.fn().mockReturnValue(of([])) },
        },
      ],
    });

    await waitFor(() => {
      expect(screen.getByText(/nenhum personagem/i)).toBeTruthy();
    });
  });

  it('renders characters from store', async () => {
    await render(CharacterListComponent, {
      providers: [
        {
          provide: StoreService,
          useValue: { getAll: vi.fn().mockReturnValue(of(mockCharacters)) },
        },
      ],
    });

    await waitFor(() => {
      expect(screen.getByText('Zagreus')).toBeTruthy();
      expect(screen.getByText('Cerberus')).toBeTruthy();
      expect(screen.getByText('Oráculo')).toBeTruthy();
      expect(screen.getByText('Lacaio')).toBeTruthy();
    });
  });

  it('filters by type chip click', async () => {
    const user = userEvent.setup();
    await render(CharacterListComponent, {
      providers: [
        {
          provide: StoreService,
          useValue: { getAll: vi.fn().mockReturnValue(of(mockCharacters)) },
        },
      ],
    });

    // Wait for characters to render
    await waitFor(() => {
      expect(screen.getByText('Zagreus')).toBeTruthy();
    });

    // Click NPC chip (use getAllByText to avoid matching the NPC type badge on cards)
    const chip = screen.getAllByText('NPC')[0];
    await user.click(chip);

    await waitFor(() => {
      expect(screen.getByText('Oráculo')).toBeTruthy();
      expect(screen.queryByText('Zagreus')).toBeNull();
      expect(screen.queryByText('Cerberus')).toBeNull();
    });
  });

  it('filters by minion type chip click', async () => {
    const user = userEvent.setup();
    await render(CharacterListComponent, {
      providers: [
        {
          provide: StoreService,
          useValue: { getAll: vi.fn().mockReturnValue(of(mockCharacters)) },
        },
      ],
    });

    await waitFor(() => {
      expect(screen.getByText('Lacaio')).toBeTruthy();
    });

    const chip = screen.getAllByText('Minion')[0];
    await user.click(chip);

    await waitFor(() => {
      expect(screen.getByText('Lacaio')).toBeTruthy();
      expect(screen.queryByText('Zagreus')).toBeNull();
      expect(screen.queryByText('Cerberus')).toBeNull();
      expect(screen.queryByText('Oráculo')).toBeNull();
    });
  });

  it('filters by name search input', async () => {
    const user = userEvent.setup();
    await render(CharacterListComponent, {
      providers: [
        {
          provide: StoreService,
          useValue: { getAll: vi.fn().mockReturnValue(of(mockCharacters)) },
        },
      ],
    });

    await waitFor(() => {
      expect(screen.getByText('Zagreus')).toBeTruthy();
    });

    const searchInput = screen.getByPlaceholderText(/buscar/i);
    await user.type(searchInput, 'Oráculo');

    await waitFor(() => {
      expect(screen.getByText('Oráculo')).toBeTruthy();
      expect(screen.queryByText('Zagreus')).toBeNull();
      expect(screen.queryByText('Cerberus')).toBeNull();
    });
  });
});
