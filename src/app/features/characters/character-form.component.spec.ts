import { render, screen } from '@testing-library/angular';
import userEvent from '@testing-library/user-event';
import { By } from '@angular/platform-browser';
import { of } from 'rxjs';
import { CharacterFormComponent } from './character-form.component';
import { CharacterSheetFieldsComponent } from './character-sheet-fields.component';
import { StoreService } from '../../core/store/store.service';
import { CharactersService } from '../../core/services/characters.service';
import { AuthService } from '../../core/auth/auth.service';
import { RouterTestingModule } from '@angular/router/testing';
import { provideNoopAnimations } from '@angular/platform-browser/animations';
import type { Character } from '../../core/models/character';

const MOCK_CHARACTER: Character = {
  id: 'mock-id',
  name: 'Test Character',
  description: '',
  type: 'player',
  attributes: {},
  skills: [],
  inventory: [],
  quotes: [],
  createdAt: new Date(),
  updatedAt: new Date(),
};

async function setup(inputs: { mode?: 'default' | 'join'; joinToken?: string | null } = {}) {
  const storeSet = vi.fn();
  const charactersJoin = vi.fn(() => of(MOCK_CHARACTER));

  const result = await render(CharacterFormComponent, {
    componentProperties: {
      mode: inputs.mode ?? 'default',
      joinToken: inputs.joinToken ?? null,
    },
    imports: [RouterTestingModule],
    providers: [
      { provide: StoreService, useValue: { set: storeSet } },
      { provide: CharactersService, useValue: { join: charactersJoin } },
      { provide: AuthService, useValue: { load: vi.fn() } },
      provideNoopAnimations(),
    ],
  });

  const fields = result.fixture.debugElement.query(By.directive(CharacterSheetFieldsComponent))
    .componentInstance as CharacterSheetFieldsComponent;

  return { ...result, storeSet, charactersJoin, fields };
}

describe('CharacterFormComponent', () => {
  it('rolling attributes populates six options in the roll pool', async () => {
    const { fields } = await setup();

    fields.onRollAttributes();

    expect(fields.rolledPool.length).toBe(6);
    fields.rolledPool.forEach((v) => {
      expect(v).toBeGreaterThanOrEqual(3);
      expect(v).toBeLessThanOrEqual(18);
    });
  });

  it('assigning all six attribute keys sets each control and empties the pool', async () => {
    const { fixture, fields } = await setup();

    fields.onRollAttributes();
    await fixture.whenStable();

    const component = fixture.componentInstance;
    const pool = [...fields.rolledPool];

    for (const attr of fields.attributeKeys) {
      fields.onAssign(attr, pool.pop()!);
    }
    await fixture.whenStable();

    for (const attr of fields.attributeKeys) {
      const control = component.characterForm.get('attributes')?.get(attr);
      expect(control?.value).toBe(fields.assigned[attr]);
    }

    expect(fields.rolledPool.length).toBe(0);
  });

  it('join mode hides the type select and calls join() not store.set on submit', async () => {
    const user = userEvent.setup();
    const { storeSet, charactersJoin, fixture, fields } = await setup({
      mode: 'join',
      joinToken: 'tok',
    });

    expect(screen.queryByRole('combobox', { name: /Tipo/i })).toBeNull();

    const readonlyInput = screen.getByLabelText('Tipo') as HTMLInputElement;
    expect(readonlyInput.value).toBe('Jogador');
    expect(readonlyInput.readOnly).toBe(true);

    await user.type(screen.getByLabelText('Nome') as HTMLInputElement, 'Join Hero');

    for (const attr of fields.attributeKeys) {
      fixture.componentInstance.characterForm.get('attributes')?.get(attr)?.setValue(12);
    }
    await fixture.whenStable();

    await user.click(screen.getByRole('button', { name: /Salvar/i }));
    await fixture.whenStable();

    expect(charactersJoin).toHaveBeenCalledWith('tok', expect.any(Object));
    expect(storeSet).not.toHaveBeenCalled();
  });

  it('built character type is "player" in join mode', async () => {
    const user = userEvent.setup();
    const { charactersJoin, fixture, fields } = await setup({
      mode: 'join',
      joinToken: 'tok',
    });

    await user.type(screen.getByLabelText('Nome') as HTMLInputElement, 'Join Hero');

    for (const attr of fields.attributeKeys) {
      fixture.componentInstance.characterForm.get('attributes')?.get(attr)?.setValue(12);
    }
    await fixture.whenStable();

    await user.click(screen.getByRole('button', { name: /Salvar/i }));
    await fixture.whenStable();

    const [, character] = charactersJoin.mock.calls[0];
    expect(character.type).toBe('player');
  });

  it('invalid form (empty name) disables submit and never persists', async () => {
    const { storeSet, charactersJoin, fixture } = await setup({
      mode: 'join',
      joinToken: 'tok',
    });

    const submitButton = screen.getByRole('button', { name: /Salvar/i });
    expect(submitButton.disabled).toBe(true);

    fixture.componentInstance.onSubmit();
    await fixture.whenStable();

    expect(charactersJoin).not.toHaveBeenCalled();
    expect(storeSet).not.toHaveBeenCalled();
  });
});