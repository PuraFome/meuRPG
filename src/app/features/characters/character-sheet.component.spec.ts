import { render, screen } from '@testing-library/angular';
import userEvent from '@testing-library/user-event';
import { of } from 'rxjs';
import { CharacterSheetComponent } from './character-sheet.component';
import { provideNoopAnimations } from '@angular/platform-browser/animations';
import { StoreService } from '../../core/store/store.service';
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

async function setup(character: Character) {
  return render(CharacterSheetComponent, {
    inputs: { character },
    providers: [
      {
        provide: StoreService,
        useValue: {
          update: vi.fn(),
          get: vi.fn().mockReturnValue(of(character)),
        },
      },
      provideNoopAnimations(),
    ],
  });
}

describe('CharacterSheetComponent', () => {
  it('renders ability modifiers derived from the attribute scores', async () => {
    await setup(mockCharacter());

    expect(screen.getByText('Mod: +2')).toBeTruthy(); // FOR 14
    expect(screen.getByText('Mod: +3')).toBeTruthy(); // DES 16
    expect(screen.getByText('Mod: +1')).toBeTruthy(); // CON 12
    expect(screen.getByText('Mod: -1')).toBeTruthy(); // SAB 8
  });

  it('renders the canonical skill grid and a saved skill bonus', async () => {
    await setup(
      mockCharacter({
        attributes: { for: 14, des: 16, con: 12, int: 10, sab: 16, car: 11 },
        skills: [
          JSON.stringify({
            name: 'Percepção',
            ability: 'sab',
            proficient: true,
            expertise: false,
            bonusOverride: null,
          }),
        ],
      }),
    );

    // Canonical skills are always present in the grid.
    expect(screen.getByDisplayValue('Atletismo')).toBeTruthy();
    // SAB 16 (+3) + proficiência (2) => +5
    expect(screen.getAllByText('+5').length).toBeGreaterThan(0);
  });

  it('renders inventory items from JSON-serialized strings', async () => {
    await setup(
      mockCharacter({
        inventory: [
          JSON.stringify({ name: 'Espada Longa', quantity: 1, weight: 3, description: 'Afiada' }),
        ],
      }),
    );

    expect(screen.getByDisplayValue('Espada Longa')).toBeTruthy();
  });

  it('adds and removes an inventory row', async () => {
    const user = userEvent.setup();
    await setup(mockCharacter());

    expect(screen.getByText(/nenhum item no inventário/i)).toBeTruthy();

    await user.click(screen.getByText('Item'));
    expect(screen.getAllByPlaceholderText(/nome do item/i).length).toBe(1);

    await user.click(screen.getByLabelText(/remover item/i));
    expect(screen.getByText(/nenhum item no inventário/i)).toBeTruthy();
  });
});