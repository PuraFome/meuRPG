import { render, screen } from '@testing-library/angular';
import { CharacterDetailComponent } from './character-detail.component';
import { StoreService } from '../../core/store/store.service';
import { of } from 'rxjs';
import { MatDialogModule } from '@angular/material/dialog';

function mockCharacter(overrides: Partial<{
  id: string;
  name: string;
  description: string;
  type: 'npc' | 'player' | 'boss';
  history: string;
  masterNotes?: string;
  imageUrl?: string;
  quotes: string[];
}> = {}) {
  return {
    id: '1',
    name: 'Zagreus',
    description: 'Filho de Hades',
    type: 'player' as const,
    history: '<p>História do Zagreus</p>',
    attributes: {} as Record<string, number>,
    skills: [] as string[],
    inventory: [] as string[],
    quotes: [] as string[],
    createdAt: new Date('2026-01-01'),
    updatedAt: new Date('2026-01-01'),
    ...overrides,
  };
}

describe('CharacterDetailComponent', () => {
  const mockCharacterData = mockCharacter();

  /** Increase timeout for Quill-heavy component rendering. */
  vi.setConfig({ testTimeout: 15_000 });

  const setup = () =>
    render(CharacterDetailComponent, {
      componentProperties: { id: '1' },
      imports: [MatDialogModule],
      providers: [
        {
          provide: StoreService,
          useValue: {
            get: vi.fn().mockReturnValue(of(mockCharacterData)),
            patch: vi.fn(),
          },
        },
      ],
    });

  it('renders character name and type badge', async () => {
    await setup();
    const nameEls = screen.getAllByText('Zagreus');
    expect(nameEls.length).toBeGreaterThanOrEqual(1);
    expect(screen.getByText('Jogador')).toBeTruthy();
  });

  it('renders all 4 tabs', async () => {
    await setup();
    expect(screen.getByText('História')).toBeTruthy();
    expect(screen.getByText('Ficha')).toBeTruthy();
    expect(screen.getByText('Notas do Mestre')).toBeTruthy();
    expect(screen.getByText('Falas')).toBeTruthy();
  });

  it('renders Quill editor in História tab', async () => {
    await setup();
    const editor = document.querySelector('quill-editor');
    expect(editor).toBeTruthy();
  });
});
