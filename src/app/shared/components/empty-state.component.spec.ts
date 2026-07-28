import { render, screen } from '@testing-library/angular';
import userEvent from '@testing-library/user-event';
import { EmptyStateComponent } from './empty-state.component';

describe('EmptyStateComponent', () => {
  it('renders the message', async () => {
    await render(EmptyStateComponent, {
      inputs: { message: 'Nenhum item encontrado' },
    });
    expect(screen.getByText('Nenhum item encontrado')).toBeTruthy();
  });

  it('renders default icon', async () => {
    await render(EmptyStateComponent, {
      inputs: { message: 'Vazio' },
    });
    // MatIcon renders as text 'inbox' by default
    expect(screen.getByText('inbox')).toBeTruthy();
  });

  it('renders custom icon when provided', async () => {
    await render(EmptyStateComponent, {
      inputs: { message: 'Vazio', icon: 'search_off' },
    });
    expect(screen.getByText('search_off')).toBeTruthy();
  });

  it('does not show action button when no actionLabel', async () => {
    await render(EmptyStateComponent, {
      inputs: { message: 'Vazio' },
    });
    expect(screen.queryByRole('button')).toBeNull();
  });

  it('shows action button when actionLabel is provided', async () => {
    await render(EmptyStateComponent, {
      inputs: { message: 'Vazio', actionLabel: 'Adicionar' },
    });
    expect(screen.getByText('Adicionar')).toBeTruthy();
  });

  it('emits action event when button is clicked', async () => {
    const user = userEvent.setup();
    const { fixture } = await render(EmptyStateComponent, {
      inputs: { message: 'Vazio', actionLabel: 'Adicionar' },
    });
    const spy = vi.fn();
    fixture.componentInstance.action.subscribe(spy);
    const btn = screen.getByText('Adicionar');
    await user.click(btn);
    expect(spy).toHaveBeenCalledTimes(1);
  });
});
