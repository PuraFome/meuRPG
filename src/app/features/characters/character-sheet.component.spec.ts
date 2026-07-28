import { render, screen, waitFor } from '@testing-library/angular';
import userEvent from '@testing-library/user-event';
import { CharacterSheetComponent } from './character-sheet.component';
import { StoreService } from '../../core/store/store.service';
import { of } from 'rxjs';
import type { Character } from '../../core/models/character';

function mockCharacter(overrides: Partial<Character> = {}): Character {
  return {
    id: 'char-1',
    name: 'Zagreus',
    description: 'Filho de Hades',
    type: 'player',
    attributes: { for: 14, des: 16, con: 12, int: 10, sab: 8, car: 11 },
    skills: [],
    inventory: [],
    quotes: [],
    createdAt: new Date('2026-01-01'),
    updatedAt: new Date('2026-01-01'),
    ...overrides,
  };
}

describe('CharacterSheetComponent', () => {
  it('renders attribute fields with values from character input', async () => {
    const character = mockCharacter({
      attributes: { for: 14, des: 16, con: 12, int: 10, sab: 8, car: 11 },
    });

    await render(CharacterSheetComponent, {
      inputs: { character },
      providers: [
        {
          provide: StoreService,
          useValue: {
            update: vi.fn(),
            get: vi.fn().mockReturnValue(of(character)),
          },
        },
      ],
    });

    // Check bonus indicators are rendered
    expect(screen.getByText('Bônus: 2')).toBeTruthy();  // FOR 14 -> +2
    expect(screen.getByText('Bônus: 3')).toBeTruthy();  // DES 16 -> +3
    expect(screen.getByText('Bônus: 1')).toBeTruthy();  // CON 12 -> +1
    expect(screen.getByText('Bônus: -1')).toBeTruthy(); // SAB 8 -> -1
    // INT 10 -> 0 and CAR 11 -> 0 => two matches
    const zeroBonuses = screen.getAllByText('Bônus: 0');
    expect(zeroBonuses.length).toBe(2);
  });

  it('renders skills from JSON-serialized strings', async () => {
    const character = mockCharacter({
      skills: [
        JSON.stringify({ name: 'Atletismo', bonus: 3, attribute: 'for' }),
        JSON.stringify({ name: 'Percepção', bonus: 1, attribute: 'sab' }),
      ],
    });

    await render(CharacterSheetComponent, {
      inputs: { character },
      providers: [
        {
          provide: StoreService,
          useValue: { update: vi.fn() },
        },
      ],
    });

    expect(screen.getByDisplayValue('Atletismo')).toBeTruthy();
    expect(screen.getByDisplayValue('Percepção')).toBeTruthy();
  });

  it('renders inventory from JSON-serialized strings', async () => {
    const character = mockCharacter({
      inventory: [
        JSON.stringify({ name: 'Espada Longa', quantity: 1, weight: 3, description: 'Uma espada afiada' }),
        JSON.stringify({ name: 'Poção de Cura', quantity: 3, weight: 0.5, description: '' }),
      ],
    });

    await render(CharacterSheetComponent, {
      inputs: { character },
      providers: [
        {
          provide: StoreService,
          useValue: { update: vi.fn() },
        },
      ],
    });

    expect(screen.getByDisplayValue('Espada Longa')).toBeTruthy();
    expect(screen.getByDisplayValue('Poção de Cura')).toBeTruthy();
  });

  it('adds and removes skill rows', async () => {
    const user = userEvent.setup();
    const character = mockCharacter();

    await render(CharacterSheetComponent, {
      inputs: { character },
      providers: [
        {
          provide: StoreService,
          useValue: { update: vi.fn() },
        },
      ],
    });

    // Initially no skills
    expect(screen.getByText(/nenhuma perícia/i)).toBeTruthy();

    // Click add skill
    const addButtons = screen.getAllByText('Adicionar');
    await user.click(addButtons[0]); // First "Adicionar" is for skills

    // Now a skill row should appear
    const nameInputs = screen.getAllByPlaceholderText(/nome da perícia/i);
    expect(nameInputs.length).toBe(1);

    // Click remove
    const deleteButtons = screen.getAllByLabelText(/remover perícia/i);
    await user.click(deleteButtons[0]);

    // Wait for remove animation + state update
    await waitFor(() => {
      expect(screen.getByText(/nenhuma perícia/i)).toBeTruthy();
    });
  });

  it('renders empty state for a new character', async () => {
    const character = mockCharacter();

    await render(CharacterSheetComponent, {
      inputs: { character },
      providers: [
        {
          provide: StoreService,
          useValue: { update: vi.fn() },
        },
      ],
    });

    expect(screen.getByText(/nenhuma perícia/i)).toBeTruthy();
    expect(screen.getByText(/nenhum item/i)).toBeTruthy();
  });
});
