import { Component, signal, computed, inject } from '@angular/core';
import { CommonModule } from '@angular/common';
import {
  CdkDropListGroup,
} from '@angular/cdk/drag-drop';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';
import { StoreService } from '../../core';
import type { CampaignFolder } from '../../core';
import { CampaignTreeComponent } from './campaign-tree.component';
import { FolderContentComponent } from './folder-content.component';

@Component({
  selector: 'app-campaign',
  standalone: true,
  imports: [
    CommonModule,
    CdkDropListGroup,
    MatButtonModule,
    MatIconModule,
    CampaignTreeComponent,
    FolderContentComponent,
  ],
  template: `
    <div cdkDropListGroup class="campaign-layout">
      <!-- Left: Folder tree -->
      <aside class="tree-panel">
        <app-campaign-tree
          [nodes]="treeNodes()"
          [selectedId]="selectedFolderId()"
          (folderSelected)="onFolderSelected($event)"
          (entityDroppedOnFolder)="onEntityDroppedOnFolder($event)"
        />
      </aside>

      <!-- Divider -->
      <div class="panel-divider"></div>

      <!-- Right: Folder content -->
      <main class="content-panel">
        <app-folder-content [folder]="selectedFolder()" />
      </main>
    </div>
  `,
  styles: [
    `
      :host {
        display: block;
        height: 100%;
      }

      .campaign-layout {
        display: grid;
        grid-template-columns: 320px 1px 1fr;
        gap: 0;
        height: 100%;
        min-height: calc(100vh - 112px);
      }

      .tree-panel {
        overflow-y: auto;
        padding: 8px 12px 16px 0;
      }

      .panel-divider {
        background: rgba(255, 255, 255, 0.08);
        width: 1px;
        height: 100%;
      }

      .content-panel {
        overflow-y: auto;
        padding: 8px 0 16px 24px;
      }

      @media (max-width: 768px) {
        .campaign-layout {
          grid-template-columns: 1fr;
        }
        .panel-divider {
          display: none;
        }
        .tree-panel {
          max-height: 300px;
          padding-right: 0;
        }
        .content-panel {
          padding-left: 0;
        }
      }
    `,
  ],
})
export class CampaignComponent {
  private readonly store = inject(StoreService);

  readonly selectedFolderId = signal<string | null>(null);

  readonly treeNodes = signal<any[]>([]);

  readonly selectedFolder = computed(() => {
    const id = this.selectedFolderId();
    if (!id) return null;
    const folders = this.store.snapshot('campaigns') as CampaignFolder[];
    return folders.find((f) => f.id === id) ?? null;
  });

  onFolderSelected(id: string) {
    this.selectedFolderId.set(id);
  }

  /**
   * Handle entity dropped onto a folder (copy reference, not move).
   */
  onEntityDroppedOnFolder(event: {
    targetFolderId: string;
    entityType: 'character' | 'map' | 'session';
    entityId: string;
  }) {
    const folders = this.store.snapshot('campaigns') as CampaignFolder[];
    const target = folders.find((f) => f.id === event.targetFolderId);
    if (!target) return;

    const ids = { ...target.entityIds };
    const entityId = event.entityId;
    let changed = false;

    switch (event.entityType) {
      case 'character':
        if (!ids.characterIds.includes(entityId)) {
          ids.characterIds = [...ids.characterIds, entityId];
          changed = true;
        }
        break;
      case 'map':
        if (!ids.mapIds.includes(entityId)) {
          ids.mapIds = [...ids.mapIds, entityId];
          changed = true;
        }
        break;
      case 'session':
        if (!ids.sessionIds.includes(entityId)) {
          ids.sessionIds = [...ids.sessionIds, entityId];
          changed = true;
        }
        break;
    }

    if (changed) {
      this.store.patch('campaigns', target.id, {
        entityIds: ids,
      } as Partial<CampaignFolder>);
    }
  }
}
