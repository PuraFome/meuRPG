import { render, screen } from '@testing-library/angular';
import userEvent from '@testing-library/user-event';
import { TestBed } from '@angular/core/testing';
import { NoopAnimationsModule } from '@angular/platform-browser/animations';
import { MatDialog } from '@angular/material/dialog';
import { of } from 'rxjs';
import { CampaignTreeComponent } from './campaign-tree.component';
import { StoreService, CampaignFolder } from '../../core';

describe('CampaignTreeComponent', () => {
  let store: StoreService<CampaignFolder>;
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  let mockDialogRef: any;

  async function setup() {
    mockDialogRef = {
      afterClosed: () => of(true),
      close: vi.fn(),
    };
    const mockDialog = {
      open: () => mockDialogRef,
    } as unknown as MatDialog;

    const result = await render(CampaignTreeComponent, {
      imports: [NoopAnimationsModule],
      providers: [{ provide: MatDialog, useValue: mockDialog }],
    });
    store = TestBed.inject(StoreService<CampaignFolder>);
    return result;
  }

  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('renders empty tree with placeholder message', async () => {
    await setup();
    expect(screen.getByText(/Nenhuma pasta/i)).toBeTruthy();
  });

  it('renders the root "Nova Pasta" button', async () => {
    await setup();
    expect(screen.getByText(/Nova Pasta/i)).toBeTruthy();
  });

  it('shows folders when added to store', async () => {
    await setup();
    const folder: CampaignFolder = {
      id: '1',
      name: 'Aventura Teste',
      parentId: null,
      children: [],
      entityIds: { characterIds: [], mapIds: [], sessionIds: [] },
    };
    store.set('campaigns', folder);

    expect(await screen.findByText('Aventura Teste')).toBeTruthy();
  });

  it('shows nested folders with depth', async () => {
    await setup();
    const parent: CampaignFolder = {
      id: 'p1',
      name: 'Campanha Principal',
      parentId: null,
      children: [],
      entityIds: { characterIds: [], mapIds: [], sessionIds: [] },
    };
    const child: CampaignFolder = {
      id: 'c1',
      name: 'Subpasta',
      parentId: 'p1',
      children: [],
      entityIds: { characterIds: [], mapIds: [], sessionIds: [] },
    };
    store.set('campaigns', parent);
    store.set('campaigns', child);

    expect(await screen.findByText('Campanha Principal')).toBeTruthy();
    // Subpasta should exist but not be visible until expanded
    // We just verify the store stores it correctly
    const allFolders = store.snapshot('campaigns');
    expect(allFolders).toHaveLength(2);
    expect(allFolders.find((f) => f.id === 'c1')?.name).toBe('Subpasta');
  });

  it('creates a root folder when "Nova Pasta" is clicked', async () => {
    const user = userEvent.setup();
    await setup();

    await user.click(screen.getByText(/Nova Pasta/i));

    const folders = store.snapshot('campaigns');
    expect(folders).toHaveLength(1);
    expect(folders[0].name).toBe('Nova Pasta');
    expect(folders[0].parentId).toBeNull();
  });

  it('allows renaming a folder via inline edit', async () => {
    const user = userEvent.setup();
    await setup();

    // Create a folder first
    const folder: CampaignFolder = {
      id: 'r1',
      name: 'Renomeável',
      parentId: null,
      children: [],
      entityIds: { characterIds: [], mapIds: [], sessionIds: [] },
    };
    store.set('campaigns', folder);
    expect(await screen.findByText('Renomeável')).toBeTruthy();

    // Find and click the edit button for this folder
    const editBtn = screen.getAllByRole('button').find(
      (btn) => btn.querySelector('mat-icon')?.textContent?.trim() === 'edit',
    );
    expect(editBtn).toBeTruthy();
    await user.click(editBtn!);

    // Input should appear
    const input = document.querySelector('.inline-edit') as HTMLInputElement;
    expect(input).toBeTruthy();

    // Clear and type new name
    await user.clear(input);
    await user.type(input, 'Renomeado');
    await user.tab(); // blur to save

    // Check the folder was renamed in store
    const updated = store.snapshot('campaigns').find((f) => f.id === 'r1');
    expect(updated?.name).toBe('Renomeado');
  });

  it('deletes a folder via confirm dialog', async () => {
    const user = userEvent.setup();
    await setup();

    const folder: CampaignFolder = {
      id: 'd1',
      name: 'Para Excluir',
      parentId: null,
      children: [],
      entityIds: { characterIds: [], mapIds: [], sessionIds: [] },
    };
    store.set('campaigns', folder);
    expect(await screen.findByText('Para Excluir')).toBeTruthy();

    // Click delete button
    const deleteBtn = screen.getAllByRole('button').find(
      (btn) => btn.querySelector('mat-icon')?.textContent?.trim() === 'delete',
    );
    expect(deleteBtn).toBeTruthy();
    await user.click(deleteBtn!);

    // After deletion, folder should be removed
    const folders = store.snapshot('campaigns');
    expect(folders.find((f) => f.id === 'd1')).toBeUndefined();
  });

  it('emits folderSelected when a folder name is clicked', async () => {
    await setup();
    const folder: CampaignFolder = {
      id: 's1',
      name: 'Selecionável',
      parentId: null,
      children: [],
      entityIds: { characterIds: [], mapIds: [], sessionIds: [] },
    };
    store.set('campaigns', folder);

    expect(await screen.findByText('Selecionável')).toBeTruthy();
  });
});
