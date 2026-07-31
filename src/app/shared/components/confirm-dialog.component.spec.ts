import { render, screen } from '@testing-library/angular';
import userEvent from '@testing-library/user-event';
import { ConfirmDialogComponent } from './confirm-dialog.component';
import { MAT_DIALOG_DATA, MatDialogRef } from '@angular/material/dialog';
import { provideNoopAnimations } from '@angular/platform-browser/animations';

describe('ConfirmDialogComponent', () => {
  const mockDialogRef = {
    close: vi.fn(),
  };

  const defaultData = {
    title: 'Confirmar Exclusão',
    message: 'Tem certeza que deseja excluir este item?',
  };

  async function setup(overrides: Record<string, unknown> = {}) {
    return render(ConfirmDialogComponent, {
      providers: [
        provideNoopAnimations(),
        { provide: MatDialogRef, useValue: mockDialogRef },
        {
          provide: MAT_DIALOG_DATA,
          useValue: { ...defaultData, ...overrides },
        },
      ],
    });
  }

  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('renders title and message', async () => {
    await setup();
    expect(screen.getByText('Confirmar Exclusão')).toBeTruthy();
    expect(
      screen.getByText('Tem certeza que deseja excluir este item?'),
    ).toBeTruthy();
  });

  it('renders default button labels', async () => {
    await setup();
    expect(screen.getByText('Cancelar')).toBeTruthy();
    expect(screen.getByText('Confirmar')).toBeTruthy();
  });

  it('renders custom button labels', async () => {
    await setup({
      confirmText: 'Sim',
      cancelText: 'Não',
    });
    expect(screen.getByText('Sim')).toBeTruthy();
    expect(screen.getByText('Não')).toBeTruthy();
  });

  it('closes with true when confirm is clicked', async () => {
    const user = userEvent.setup();
    await setup();
    await user.click(screen.getByText('Confirmar'));
    expect(mockDialogRef.close).toHaveBeenCalledWith(true);
  });

  it('closes with false when cancel is clicked', async () => {
    const user = userEvent.setup();
    await setup();
    await user.click(screen.getByText('Cancelar'));
    expect(mockDialogRef.close).toHaveBeenCalledWith(false);
  });
});
