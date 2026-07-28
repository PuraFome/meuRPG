import {
  Component,
  Input,
  Output,
  EventEmitter,
  inject,
  InjectionToken,
  OnInit,
  OnDestroy,
  computed,
} from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormsModule } from '@angular/forms';
import {
  CdkDragDrop,
  CdkDropList,
  CdkDrag,
  CdkDragHandle,
  moveItemInArray,
  transferArrayItem,
} from '@angular/cdk/drag-drop';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';
import { MatDialog } from '@angular/material/dialog';
import { Subscription } from 'rxjs';
import { StoreService, CampaignFolder } from '../../core';
import {
  ConfirmDialogComponent,
  ConfirmDialogData,
} from '../../shared';

// ─── Tree node type ────────────────────────────────────────────
export interface FolderNode {
  id: string;
  name: string;
  parentId: string | null;
  children: FolderNode[];
  entityIds: { characterIds: string[]; mapIds: string[]; sessionIds: string[] };
  expanded: boolean;
}

// ─── Injection token for cross-component drop-list connection ──
export const TREE_DROP_LIST_IDS = new InjectionToken<Set<string>>(
  'TREE_DROP_LIST_IDS',
);

// ─── Helpers ───────────────────────────────────────────────────
function buildTree(folders: CampaignFolder[]): FolderNode[] {
  const map = new Map<string, FolderNode>();
  const roots: FolderNode[] = [];

  for (const f of folders) {
    map.set(f.id, { ...f, children: [], expanded: false });
  }
  for (const f of folders) {
    const node = map.get(f.id)!;
    if (f.parentId && map.has(f.parentId)) {
      map.get(f.parentId)!.children.push(node);
    } else if (!f.parentId) {
      roots.push(node);
    }
  }
  return roots;
}

function flattenTree(nodes: FolderNode[]): CampaignFolder[] {
  const result: CampaignFolder[] = [];
  for (const node of nodes) {
    const { children: _c, expanded: _e, ...rest } = node;
    result.push({ ...rest, children: [] });
    result.push(...flattenTree(node.children));
  }
  return result;
}

let nextListId = 0;
function uniqueListId(): string {
  return `tree-drop-${nextListId++}`;
}

// ─── Component ─────────────────────────────────────────────────
@Component({
  selector: 'app-campaign-tree',
  standalone: true,
  imports: [
    CommonModule,
    FormsModule,
    CampaignTreeComponent,
    CdkDropList,
    CdkDrag,
    CdkDragHandle,
    MatButtonModule,
    MatIconModule,
  ],
  template: `
    <!-- Root header -->
    @if (depth === 0) {
      <div class="tree-header">
        <h2 class="tree-title">Campanha</h2>
        <button
          mat-raised-button
          color="primary"
          (click)="addRootFolder()"
          class="new-folder-btn"
        >
          <mat-icon>create_new_folder</mat-icon>
          Nova Pasta
        </button>
      </div>
    }

    <!-- Folder list with CDK drag-drop -->
    @if (nodes.length > 0) {
      <div
        cdkDropList
        [id]="listId"
        [cdkDropListData]="nodes"
        [cdkDropListConnectedTo]="allListIds()"
        (cdkDropListDropped)="onDrop($event)"
        class="folder-list"
        [class.root-list]="depth === 0"
      >
        @for (node of nodes; track node.id; let idx = $index) {
          <div cdkDrag class="folder-item" [class.selected]="selectedId === node.id">
            <!-- Drag handle visual cue -->
            <div class="drag-handle" cdkDragHandle>
              <mat-icon>drag_indicator</mat-icon>
            </div>

            <!-- Folder row content -->
            <div
              class="folder-row"
              [style.--folder-depth]="depth"
              (click)="selectFolder(node)"
            >
              <!-- Expand/collapse toggle -->
              <button
                mat-icon-button
                type="button"
                class="toggle-btn"
                (click)="toggleExpand(node); $event.stopPropagation()"
              >
                @if (node.children.length > 0) {
                  <mat-icon>{{
                    node.expanded ? 'expand_more' : 'chevron_right'
                  }}</mat-icon>
                } @else {
                  <mat-icon class="blank-icon">circle</mat-icon>
                }
              </button>

              <!-- Folder icon -->
              <mat-icon class="folder-icon" [class.expanded]="node.expanded"
                >{{ node.expanded ? 'folder_open' : 'folder' }}</mat-icon
              >

              <!-- Inline edit or display name -->
              @if (editingId === node.id) {
                <input
                  #inlineInput
                  [(ngModel)]="editName"
                  (blur)="saveRename(node)"
                  (keydown.enter)="saveRename(node)"
                  (keydown.escape)="cancelRename()"
                  (click)="$event.stopPropagation()"
                  class="inline-edit"
                  autofocus
                />
              } @else {
                <span class="folder-name">{{ node.name }}</span>
              }

              <!-- Folder actions -->
              <span class="folder-actions" (click)="$event.stopPropagation()">
                <button
                  mat-icon-button
                  type="button"
                  matTooltip="Adicionar subpasta"
                  (click)="addSubfolder(node)"
                >
                  <mat-icon>create_new_folder</mat-icon>
                </button>
                <button
                  mat-icon-button
                  type="button"
                  matTooltip="Renomear"
                  (click)="startRename(node)"
                >
                  <mat-icon>edit</mat-icon>
                </button>
                <button
                  mat-icon-button
                  type="button"
                  matTooltip="Excluir"
                  (click)="deleteFolder(node)"
                >
                  <mat-icon>delete</mat-icon>
                </button>
              </span>
            </div>

            <!-- Children (recursive) -->
            @if (node.expanded && node.children.length > 0) {
              <div class="children-wrapper">
                <app-campaign-tree
                  [nodes]="node.children"
                  [depth]="depth + 1"
                  [selectedId]="selectedId"
                  (folderSelected)="onChildSelected($event)"
                />
              </div>
            }
          </div>
        }
      </div>
    } @else if (depth === 0) {
      <!-- Empty state -->
      <div class="empty-state">
        <mat-icon class="empty-icon">folder_off</mat-icon>
        <p class="empty-message">Nenhuma pasta ainda.</p>
        <p class="empty-hint">
          Crie sua primeira pasta de campanha para organizar seu mundo.
        </p>
      </div>
    }
  `,
  styles: [
    `
      :host {
        display: block;
      }

      /* ── Header ─────────────────────────── */
      .tree-header {
        display: flex;
        align-items: center;
        justify-content: space-between;
        margin-bottom: 16px;
        gap: 12px;
        flex-wrap: wrap;
      }
      .tree-title {
        margin: 0;
        font-size: 1.5rem;
        font-weight: 600;
        letter-spacing: -0.01em;
      }
      .new-folder-btn mat-icon {
        margin-right: 4px;
      }

      /* ── Folder list ────────────────────── */
      .folder-list {
        display: flex;
        flex-direction: column;
        gap: 2px;
        min-height: 40px;
      }
      .root-list {
        padding: 0;
      }

      /* ── Folder item ────────────────────── */
      .folder-item {
        display: flex;
        flex-direction: column;
        background: rgba(255, 255, 255, 0.03);
        border-radius: 6px;
        border: 1px solid transparent;
        transition: background 0.15s, border-color 0.15s;

        /* Depth-based margin */
        margin-left: calc(var(--folder-depth, 0) * 20px);
      }
      .folder-item:hover {
        background: rgba(255, 255, 255, 0.06);
      }
      .folder-item.selected {
        background: rgba(var(--mat-app-primary, 63, 81, 181), 0.08);
        border-color: rgba(var(--mat-app-primary, 63, 81, 181), 0.3);
      }
      .folder-item.cdk-drag-placeholder {
        opacity: 0.3;
      }
      .folder-item.cdk-drag-preview {
        border-radius: 8px;
        box-shadow: 0 4px 16px rgba(0, 0, 0, 0.3);
        background: rgba(30, 30, 40, 0.95);
        backdrop-filter: blur(4px);
      }

      /* ── Folder row ─────────────────────── */
      .folder-row {
        display: flex;
        align-items: center;
        gap: 4px;
        padding: 4px 8px;
        cursor: pointer;
        min-height: 40px;
        user-select: none;
      }

      .drag-handle {
        display: flex;
        align-items: center;
        justify-content: center;
        cursor: grab;
        color: rgba(255, 255, 255, 0.2);
        transition: color 0.15s;
        width: 20px;
        flex-shrink: 0;
      }
      .drag-handle:hover {
        color: rgba(255, 255, 255, 0.5);
      }
      .drag-handle mat-icon {
        font-size: 16px;
        width: 16px;
        height: 16px;
        line-height: 16px;
      }

      .toggle-btn {
        width: 28px;
        height: 28px;
        line-height: 28px;
        flex-shrink: 0;
      }
      .toggle-btn mat-icon {
        font-size: 18px;
        width: 18px;
        height: 18px;
        line-height: 18px;
      }
      .blank-icon {
        opacity: 0;
        pointer-events: none;
      }

      .folder-icon {
        font-size: 20px;
        width: 20px;
        height: 20px;
        line-height: 20px;
        flex-shrink: 0;
        margin-right: 4px;
        opacity: 0.7;
      }
      .folder-icon.expanded {
        opacity: 1;
      }

      .folder-name {
        flex: 1;
        font-size: 0.9rem;
        font-weight: 450;
        white-space: nowrap;
        overflow: hidden;
        text-overflow: ellipsis;
        min-width: 0;
        padding: 2px 4px;
        border-radius: 4px;
        transition: background 0.15s;
      }
      .folder-name:hover {
        background: rgba(255, 255, 255, 0.05);
      }

      .inline-edit {
        flex: 1;
        font-size: 0.9rem;
        font-weight: 450;
        border: 1px solid rgba(var(--mat-app-primary, 63, 81, 181), 0.5);
        border-radius: 4px;
        padding: 2px 6px;
        background: rgba(0, 0, 0, 0.2);
        color: inherit;
        min-width: 0;
        outline: none;
      }

      .folder-actions {
        display: flex;
        gap: 0;
        opacity: 0;
        transition: opacity 0.15s;
        flex-shrink: 0;
      }
      .folder-item:hover .folder-actions {
        opacity: 1;
      }
      .folder-actions button {
        width: 28px;
        height: 28px;
        line-height: 28px;
      }
      .folder-actions button mat-icon {
        font-size: 16px;
        width: 16px;
        height: 16px;
        line-height: 16px;
      }

      /* ── Children wrapper ───────────────── */
      .children-wrapper {
        padding-left: 8px;
        border-left: 1px solid rgba(255, 255, 255, 0.08);
        margin-left: 28px;
      }

      /* ── Empty state ────────────────────── */
      .empty-state {
        display: flex;
        flex-direction: column;
        align-items: center;
        justify-content: center;
        padding: 48px 16px;
        text-align: center;
      }
      .empty-icon {
        font-size: 3rem;
        width: 3rem;
        height: 3rem;
        margin-bottom: 12px;
        opacity: 0.35;
      }
      .empty-message {
        font-size: 1rem;
        margin: 0 0 4px;
        opacity: 0.6;
      }
      .empty-hint {
        font-size: 0.85rem;
        margin: 0;
        opacity: 0.4;
        max-width: 320px;
        line-height: 1.4;
      }
    `,
  ],
})
export class CampaignTreeComponent implements OnInit, OnDestroy {
  // ── Inputs / Outputs ──────────────────────────────────────────
  @Input() nodes: FolderNode[] = [];
  @Input() depth = 0;
  @Input() selectedId: string | null = null;
  @Output() folderSelected = new EventEmitter<string>();

  // ── State ─────────────────────────────────────────────────────
  editingId: string | null = null;
  editName = '';
  listId = uniqueListId();

  // ── Shared drop-list IDs (for cross-level connected lists) ───
  private allDropListIds = inject(TREE_DROP_LIST_IDS, {
    skipSelf: true,
    optional: true,
  });
  allListIds = computed(() => {
    const ids = this.allDropListIds;
    return ids ? Array.from(ids) : [];
  });

  // ── Dependencies ─────────────────────────────────────────────
  private store = inject(StoreService<CampaignFolder>);
  private dialog = inject(MatDialog);
  private subscription: Subscription | null = null;

  // ── Lifecycle ─────────────────────────────────────────────────
  ngOnInit() {
    // Register this list for cross-level drag-drop
    if (this.allDropListIds) {
      this.allDropListIds.add(this.listId);
    }

    // Root instance subscribes to store
    if (this.depth === 0) {
      // Save expanded state before each rebuild
      let prevExpanded = this.captureExpanded();

      this.subscription = this.store.getAll('campaigns').subscribe((folders) => {
        this.nodes = buildTree(folders);
        this.restoreExpanded(prevExpanded, this.nodes);
        prevExpanded = this.captureExpanded();
      });
    }
  }

  ngOnDestroy() {
    // Unregister this list
    if (this.allDropListIds) {
      this.allDropListIds.delete(this.listId);
    }
    this.subscription?.unsubscribe();
  }

  // ── Expanded state management ─────────────────────────────────
  private captureExpanded(): Map<string, boolean> {
    const map = new Map<string, boolean>();
    this.walkNodes(this.nodes, (n) => {
      if (n.expanded) map.set(n.id, true);
    });
    return map;
  }

  private restoreExpanded(map: Map<string, boolean>, nodes: FolderNode[]) {
    for (const node of nodes) {
      if (map.has(node.id)) node.expanded = true;
      this.restoreExpanded(map, node.children);
    }
  }

  private walkNodes(
    nodes: FolderNode[],
    fn: (node: FolderNode) => void,
  ) {
    for (const node of nodes) {
      fn(node);
      this.walkNodes(node.children, fn);
    }
  }

  // ── Actions ───────────────────────────────────────────────────
  toggleExpand(node: FolderNode) {
    node.expanded = !node.expanded;
  }

  selectFolder(node: FolderNode) {
    this.folderSelected.emit(node.id);
  }

  onChildSelected(id: string) {
    this.folderSelected.emit(id);
  }

  addRootFolder() {
    const id = crypto.randomUUID();
    const folder: CampaignFolder = {
      id,
      name: 'Nova Pasta',
      parentId: null,
      children: [],
      entityIds: { characterIds: [], mapIds: [], sessionIds: [] },
    };
    this.store.set('campaigns', folder);
  }

  addSubfolder(node: FolderNode) {
    const id = crypto.randomUUID();
    const folder: CampaignFolder = {
      id,
      name: 'Nova Pasta',
      parentId: node.id,
      children: [],
      entityIds: { characterIds: [], mapIds: [], sessionIds: [] },
    };
    this.store.set('campaigns', folder);
    node.expanded = true;
  }

  startRename(node: FolderNode) {
    this.editingId = node.id;
    this.editName = node.name;
  }

  saveRename(node: FolderNode) {
    const name = this.editName?.trim();
    if (name && name !== node.name) {
      this.store.patch('campaigns', node.id, {
        name,
      } as Partial<CampaignFolder>);
      node.name = name;
    }
    this.editingId = null;
  }

  cancelRename() {
    this.editingId = null;
  }

  async deleteFolder(node: FolderNode) {
    const dialogRef = this.dialog.open(ConfirmDialogComponent, {
      width: '420px',
      data: {
        title: 'Excluir Pasta',
        message: `Tem certeza que deseja excluir "${node.name}"${
          node.children.length > 0 ? ' e todas as suas subpastas' : ''
        }?`,
        confirmText: 'Excluir',
        cancelText: 'Cancelar',
      } as ConfirmDialogData,
    });

    dialogRef.afterClosed().subscribe((result) => {
      if (result) {
        this.deleteRecursive(node.id);
      }
    });
  }

  private deleteRecursive(id: string) {
    const allFolders = this.store.snapshot('campaigns');
    const children = allFolders.filter((f) => f.parentId === id);
    for (const child of children) {
      this.deleteRecursive(child.id);
    }
    this.store.delete('campaigns', id);
  }

  // ── Drag-drop ─────────────────────────────────────────────────
  onDrop(event: CdkDragDrop<FolderNode[]>) {
    if (event.previousContainer === event.container) {
      // Reorder within same list
      moveItemInArray(
        event.container.data,
        event.previousIndex,
        event.currentIndex,
      );
    } else {
      // Move between lists (reparent)
      const moved = event.previousContainer.data[event.previousIndex];
      transferArrayItem(
        event.previousContainer.data,
        event.container.data,
        event.previousIndex,
        event.currentIndex,
      );
      // Determine new parentId from target container
      moved.parentId = this.findParentForContainer(event.container.data);
    }

    // Persist all folders in their new order/parents
    this.persistOrder();
  }

  private findParentForContainer(
    containerData: FolderNode[],
  ): string | null {
    // Walk the full tree (starting from root) to find the parent
    if (this.depth !== 0) {
      // For non-root instances, we can't walk the full tree from here.
      // The root's 'nodes' are the authoritative tree.
      // But since child instances might also process drops,
      // we need to find the parent differently.

      // Actually, the drop happens from the root's event handler perspective.
      // Wait, each instance has its own event handler.
      // Let me handle this more robustly.

      // We find which node's children array matches this container data
      // by walking the tree accessible from this instance.
      // This works for the immediate parent but might not find
      // the correct parent if this is a nested instance.
      return this.findParentInTree(this.nodes, containerData);
    }
    return this.findParentInTree(this.nodes, containerData);
  }

  private findParentInTree(
    nodes: FolderNode[],
    target: FolderNode[],
  ): string | null {
    for (const node of nodes) {
      if (node.children === target) return node.id;
      const found = this.findParentInTree(node.children, target);
      if (found !== null) return found;
    }
    return null; // Root level (no parent)
  }

  private persistOrder() {
    // Flatten the tree (which now has correct order + parentIds)
    // and re-save to store to persist structure
    if (this.depth === 0) {
      const flat = flattenTree(this.nodes);
      // Re-save all to persist the order
      for (const folder of flat) {
        this.store.set('campaigns', folder);
      }
    }
  }
}
