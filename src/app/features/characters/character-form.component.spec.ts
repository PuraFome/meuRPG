import { render, screen } from '@testing-library/angular';
import userEvent from '@testing-library/user-event';
import { of } from 'rxjs';
import { CharacterFormComponent } from './character-form.component';
import { StoreService } from '../../core/store/store.service';
import { CharactersService } from '../../core/services/characters.service';
import { AuthService } from '../../core/auth/auth.service';
import { Router } from '@angular/router';
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

  return { ...result, storeSet, charactersJoin };
}

describe('CharacterFormComponent', () => {
  it('click "Rolar Atributos (4d6)" populates six options in rolledPool', async () => {
    const user = userEvent.setup();
    const { fixture } = await setup();

    await user.click(screen.getByRole('button', { name: /Rolar Atributos \(4d6\)/i }));

    const component = fixture.componentInstance;
    expect(component.rolledPool.length).toBe(6);
    component.rolledPool.forEach((v) => {
      expect(v).toBeGreaterThanOrEqual(3);
      expect(v).toBeLessThanOrEqual(18);
    });
  });

  it('assigning all six attributeKeys sets each control and empties rolledPool', async () => {
    const user = userEvent.setup();
    const { fixture } = await setup();

    await user.click(screen.getByRole('button', { name: /Rolar Atributos \(4d6\)/i }));
    await fixture.whenStable();

    const component = fixture.componentInstance;
    const pool = [...component.rolledPool];

    for (const attr of component.attributeKeys) {
      const value = pool.pop()!;
      component.onAssign(attr, value);
    }
    await fixture.whenStable();

    for (const attr of component.attributeKeys) {
      const control = component.characterForm.get('attributes')?.get(attr);
      expect(control?.value).toBe(component.assigned[attr]);
    }

    expect(component.rolledPool.length).toBe(0);
  });

  it('join mode hides type mat-select and calls join() not store.set on submit', async () => {
    const user = userEvent.setup();
    const { storeSet, charactersJoin, fixture } = await setup({
      mode: 'join',
      joinToken: 'tok',
    });

    expect(screen.queryByRole('combobox', { name: /Tipo/i })).toBeNull();

    const readonlyInput = screen.getByLabelText('Tipo') as HTMLInputElement;
    expect(readonlyInput.value).toBe('Jogador');
    expect(readonlyInput.readOnly).toBe(true);

    await user.type(screen.getByLabelText('Nome') as HTMLInputElement, 'Join Hero');

    const component = fixture.componentInstance;
    component.onRollAttributes();
    await fixture.whenStable();

    for (const attr of component.attributeKeys) {
      const value = component.rolledPool[0] ?? 10;
      component.onAssign(attr, value);
    }
    await fixture.whenStable();

    await user.click(screen.getByRole('button', { name: /Salvar/i }));
    await fixture.whenStable();

    expect(charactersJoin).toHaveBeenCalledWith('tok', expect.any(Object));
    expect(storeSet).not.toHaveBeenCalled();
  });

  it('built character type is "player" in join mode', async () => {
    const user = userEvent.setup();
    const { charactersJoin, fixture } = await setup({
      mode: 'join',
      joinToken: 'tok',
    });

    await user.type(screen.getByLabelText('Nome') as HTMLInputElement, 'Join Hero');

    const component = fixture.componentInstance;
    component.onRollAttributes();
    await fixture.whenStable();

    for (const attr of component.attributeKeys) {
      const value = component.rolledPool[0] ?? 10;
      component.onAssign(attr, value);
    }
    await fixture.whenStable();

    await user.click(screen.getByRole('button', { name: /Salvar/i }));
    await fixture.whenStable();

    const [, character] = charactersJoin.mock.calls[0];
    expect(character.type).toBe('player');
  });

  it('invalid form (empty name) calls neither join nor store.set', async () => {
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
