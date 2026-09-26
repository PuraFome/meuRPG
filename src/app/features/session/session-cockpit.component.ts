import {
  Component,
  OnInit,
  OnDestroy,
  inject,
  signal,
  computed,
} from '@angular/core';
import { FormsModule } from '@angular/forms';
import { ActivatedRoute, Router } from '@angular/router';
import { Subject } from 'rxjs';
import { takeUntil } from 'rxjs/operators';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';
import { MatListModule } from '@angular/material/list';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatInputModule } from '@angular/material/input';
import { MatTooltipModule } from '@angular/material/tooltip';
import { MatDialog } from '@angular/material/dialog';
import { PageHeaderComponent } from '../../shared/components/page-header.component';
import { EmptyStateComponent } from '../../shared/components/empty-state.component';
import { StoreService } from '../../core/store/store.service';
import { SessionToolbarComponent } from './session-toolbar.component';
import { DiceRollerDialogComponent } from './dice-roller-dialog.component';
import { SessionBroadcastService } from './session-broadcast.service';
import { gatherCampaignEntities, groupCharactersByType } from './campaign-entities';
import type { SessionState } from '../../core/models/session';
import type { CampaignFolder } from '../../core/models/campaign';
import type { MapData } from '../../core/models/map';
import type { Character } from '../../core/models/character';
import type { BreadcrumbItem } from '../../shared/components/page-header.component';

@Component({
  selector: 'app-session-cockpit',
  standalone: true,
  imports: [
    FormsModule,
    MatButtonModule,
    MatIconModule,
    MatListModule,
    MatFormFieldModule,
    MatInputModule,
    MatTooltipModule,
    PageHeaderComponent,
    EmptyStateComponent,
    SessionToolbarComponent,
  ],
  template: `
    <app-page-header [title]="pageTitle()" icon="event" [breadcrumbs]="breadcrumbs" />

    @if (isStarting()) {
      <div class="start-view">
        <div class="start-header">
          <mat-icon class="start-icon">cast</mat-icon>
          <h2 class="start-title">Iniciar uma sessão</h2>
          <p class="start-text">
            Escolha a campanha que vai mestrar. A sessão abre uma tela de
            apresentação para você projetar os mapas na segunda tela.
          </p>
        </div>

        @if (selectableCampaigns().length === 0) {
          <app-empty-state
            icon="create_new_folder"
            message="Nenhuma campanha ainda. Crie uma pasta na página de Campanha e associe mapas e personagens."
            actionLabel="Ir para Campanha"
            (action)="goToCampaign()"
          />
        } @else {
          <div class="campaign-grid">
            @for (folder of selectableCampaigns(); track folder.id) {
              <button class="campaign-card" (click)="startSession(folder.id)">
                <mat-icon class="card-icon">folder</mat-icon>
                <span class="card-name">{{ folder.name }}</span>
                <span class="card-meta">
                  {{ folder.entityIds.mapIds.length }} mapas ·
                  {{ folder.entityIds.characterIds.length }} personagens
                </span>
              </button>
            }
          </div>
        }
      </div>
    } @else if (!session()) {
      <div class="missing-wrapper">
        <app-empty-state
          icon="event_busy"
          message="Sessão não encontrada."
          actionLabel="Voltar"
          (action)="goToSessions()"
        />
      </div>
    } @else {
      <app-session-toolbar
        (rollDice)="openDiceRoller()"
        (openMap)="openPresentation()"
        (toggleMusic)="toggleMusic()"
        (togglePlayPause)="togglePlayPause()"
      />

      <div class="cockpit-body">
        <section class="panel maps-panel">
          <div class="panel-header">
            <mat-icon class="panel-icon">map</mat-icon>
            <span>Mapas da campanha</span>
            <span class="spacer"></span>
            <button mat-stroked-button color="primary" (click)="openPresentation()">
              <mat-icon>cast</mat-icon>
              Apresentar
            </button>
          </div>

          <div class="panel-body">
            @if (maps().length === 0) {
              <app-empty-state
                icon="map"
                message="Nenhum mapa associado à campanha. Associe mapas na página de Campanha."
                actionLabel="Ir para Campanha"
                (action)="goToCampaign()"
              />
            } @else {
              <mat-list class="map-list">
                @for (map of maps(); track map.id) {
                  <mat-list-item
                    class="map-item"
                    [class.active]="map.id === session()!.activeMapId"
                    (click)="presentMap(map.id)"
                  >
                    <mat-icon matListItemIcon>{{ mapKindIcon(map) }}</mat-icon>
                    <span matListItemTitle>{{ map.name }}</span>
                    <span matListItemLine>{{ mapKindLabel(map) }}</span>
                    @if (map.id === session()!.activeMapId) {
                      <mat-icon matListItemMeta class="live-icon">cast_connected</mat-icon>
                    }
                  </mat-list-item>
                }
              </mat-list>
            }
          </div>

          <div class="notes-section">
            <label class="notes-label">
              <mat-icon>sticky_note_2</mat-icon>
              Anotações da campanha
            </label>
            <textarea
              class="notes-input"
              rows="6"
              [ngModel]="notesDraft()"
              (ngModelChange)="onNotesChange($event)"
              placeholder="O que planejei para esta sessão: ganchos, NPCs, reviravoltas..."
            ></textarea>
          </div>
        </section>

        <section class="panel chars-panel">
          <div class="panel-header">
            <mat-icon class="panel-icon">groups</mat-icon>
            <span>Personagens</span>
          </div>

          <div class="panel-body">
            <mat-form-field appearance="outline" class="char-search" subscriptSizing="dynamic">
              <mat-icon matPrefix>search</mat-icon>
              <input
                matInput
                [ngModel]="characterSearch()"
                (ngModelChange)="characterSearch.set($event)"
                placeholder="Buscar personagem..."
              />
              @if (characterSearch()) {
                <button
                  matSuffix
                  mat-icon-button
                  aria-label="Limpar"
                  (click)="characterSearch.set('')"
                >
                  <mat-icon>close</mat-icon>
                </button>
              }
            </mat-form-field>

            @if (groupedCharacters().length === 0) {
              <app-empty-state icon="group" message="Nenhum personagem encontrado." />
            } @else {
              @for (group of groupedCharacters(); track group.type) {
                <div class="char-group">
                  <h3 class="group-title">
                    {{ group.label }}
                    <span class="group-count">{{ group.characters.length }}</span>
                  </h3>
                  @for (character of group.characters; track character.id) {
                    <div
                      class="char-row"
                      [class.active]="session()!.activeEntity?.id === character.id"
                    >
                      <button class="char-main" (click)="selectCharacter(character)">
                        <span class="char-name">{{ character.name }}</span>
                        @if (character.description) {
                          <span class="char-desc">{{ character.description }}</span>
                        }
                      </button>
                      <button
                        mat-icon-button
                        matTooltip="Abrir ficha"
                        (click)="openCharacter(character)"
                      >
                        <mat-icon>open_in_new</mat-icon>
                      </button>
                    </div>
                  }
                </div>
              }
            }
          </div>
        </section>
      </div>
    }
  `,
  styles: `
    :host {
      display: flex;
      flex-direction: column;
      height: 100%;
    }

    .start-view {
      flex: 1;
      overflow-y: auto;
      padding: 24px;
    }

    .start-header {
      text-align: center;
      max-width: 520px;
      margin: 0 auto 24px;
    }

    .start-icon {
      font-size: 2.6rem;
      width: 2.6rem;
      height: 2.6rem;
      color: #b388ff;
    }

    .start-title {
      margin: 8px 0;
      font-size: 1.4rem;
    }

    .start-text {
      margin: 0;
      opacity: 0.65;
      line-height: 1.6;
      font-size: 0.92rem;
    }

    .campaign-grid {
      display: grid;
      grid-template-columns: repeat(auto-fill, minmax(220px, 1fr));
      gap: 16px;
      max-width: 900px;
      margin: 0 auto;
    }

    .campaign-card {
      display: flex;
      flex-direction: column;
      align-items: flex-start;
      gap: 6px;
      padding: 20px;
      text-align: left;
      border: 1px solid rgba(255, 255, 255, 0.1);
      border-radius: 14px;
      background: rgba(255, 255, 255, 0.03);
      color: inherit;
      cursor: pointer;
      transition: transform 0.15s, border-color 0.15s, background 0.15s;
    }

    .campaign-card:hover {
      transform: translateY(-3px);
      border-color: rgba(124, 77, 255, 0.5);
      background: rgba(124, 77, 255, 0.08);
    }

    .card-icon {
      color: #b388ff;
    }

    .card-name {
      font-size: 1.05rem;
      font-weight: 600;
    }

    .card-meta {
      font-size: 0.78rem;
      opacity: 0.55;
    }

    .missing-wrapper {
      flex: 1;
      display: flex;
      align-items: center;
      justify-content: center;
    }

    .cockpit-body {
      flex: 1;
      min-height: 0;
      display: grid;
      grid-template-columns: 1fr 1fr;
      gap: 0;
    }

    .panel {
      display: flex;
      flex-direction: column;
      min-width: 0;
      overflow: hidden;
    }

    .maps-panel {
      border-right: 1px solid rgba(255, 255, 255, 0.08);
    }

    .panel-header {
      display: flex;
      align-items: center;
      gap: 8px;
      padding: 12px 16px;
      font-size: 0.85rem;
      font-weight: 600;
      text-transform: uppercase;
      letter-spacing: 0.5px;
      color: rgba(255, 255, 255, 0.6);
      border-bottom: 1px solid rgba(255, 255, 255, 0.06);
      background: rgba(255, 255, 255, 0.02);
      flex-shrink: 0;
    }

    .panel-icon {
      font-size: 1.1rem;
      width: 1.1rem;
      height: 1.1rem;
      opacity: 0.8;
    }

    .spacer {
      flex: 1;
    }

    .panel-body {
      flex: 1;
      overflow-y: auto;
      padding: 8px;
    }

    .map-item {
      cursor: pointer;
      border-radius: 8px;
    }

    .map-item.active {
      background: rgba(124, 77, 255, 0.18);
    }

    .live-icon {
      color: #69f0ae;
    }

    .notes-section {
      flex-shrink: 0;
      border-top: 1px solid rgba(255, 255, 255, 0.08);
      padding: 12px 16px 16px;
      display: flex;
      flex-direction: column;
      gap: 6px;
    }

    .notes-label {
      display: flex;
      align-items: center;
      gap: 6px;
      font-size: 0.78rem;
      font-weight: 600;
      text-transform: uppercase;
      letter-spacing: 0.4px;
      color: rgba(255, 255, 255, 0.55);
    }

    .notes-label mat-icon {
      font-size: 1rem;
      width: 1rem;
      height: 1rem;
    }

    .notes-input {
      width: 100%;
      resize: vertical;
      min-height: 90px;
      padding: 10px 12px;
      border-radius: 10px;
      border: 1px solid rgba(255, 255, 255, 0.12);
      background: rgba(255, 255, 255, 0.04);
      color: inherit;
      font-family: inherit;
      font-size: 0.9rem;
      line-height: 1.5;
    }

    .notes-input:focus {
      outline: none;
      border-color: rgba(124, 77, 255, 0.6);
    }

    .char-search {
      width: 100%;
      margin-bottom: 4px;
    }

    .char-group {
      margin-bottom: 12px;
    }

    .group-title {
      display: flex;
      align-items: center;
      gap: 8px;
      margin: 8px 8px 4px;
      font-size: 0.75rem;
      font-weight: 700;
      text-transform: uppercase;
      letter-spacing: 0.5px;
      color: rgba(255, 255, 255, 0.5);
    }

    .group-count {
      padding: 1px 8px;
      border-radius: 10px;
      background: rgba(255, 255, 255, 0.08);
      font-size: 0.7rem;
    }

    .char-row {
      display: flex;
      align-items: center;
      gap: 4px;
      border-radius: 8px;
      transition: background 0.15s;
    }

    .char-row:hover {
      background: rgba(255, 255, 255, 0.04);
    }

    .char-row.active {
      background: rgba(124, 77, 255, 0.18);
    }

    .char-main {
      flex: 1;
      display: flex;
      flex-direction: column;
      align-items: flex-start;
      gap: 2px;
      padding: 8px 10px;
      border: none;
      background: transparent;
      color: inherit;
      cursor: pointer;
      text-align: left;
      min-width: 0;
    }

    .char-name {
      font-size: 0.92rem;
      font-weight: 500;
    }

    .char-desc {
      font-size: 0.76rem;
      opacity: 0.55;
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
      max-width: 100%;
    }

    @media (max-width: 900px) {
      .cockpit-body {
        grid-template-columns: 1fr;
        overflow-y: auto;
      }
      .maps-panel {
        border-right: none;
        border-bottom: 1px solid rgba(255, 255, 255, 0.08);
      }
    }
  `,
})
export class SessionCockpitComponent implements OnInit, OnDestroy {
  private readonly store = inject(StoreService);
  private readonly dialog = inject(MatDialog);
  private readonly router = inject(Router);
  private readonly route = inject(ActivatedRoute);
  private readonly broadcast = inject(SessionBroadcastService);
  private readonly destroy$ = new Subject<void>();
  private readonly sessionId = this.route.snapshot.paramMap.get('id');

  readonly campaigns = signal<CampaignFolder[]>([]);
  readonly session = signal<SessionState | null>(null);
  readonly maps = signal<MapData[]>([]);
  readonly characters = signal<Character[]>([]);
  readonly characterSearch = signal('');
  readonly notesDraft = signal('');

  private allMaps: MapData[] = [];
  private allCharacters: Character[] = [];
  private subscriptions: { unsubscribe: () => void }[] = [];
  private notesTimer: ReturnType<typeof setTimeout> | null = null;

  readonly breadcrumbs: BreadcrumbItem[] = [{ label: 'Sessão', route: '/sessao' }];

  readonly isStarting = computed(() => !this.sessionId);

  readonly pageTitle = computed(() => this.session()?.name ?? 'Sessão');

  readonly selectableCampaigns = computed(() => {
    const list = this.campaigns();
    const roots = list.filter((folder) => !folder.parentId);
    const base = roots.length > 0 ? roots : list;
    return [...base].sort((a, b) => a.name.localeCompare(b.name, 'pt-BR'));
  });

  readonly filteredCharacters = computed(() => {
    const query = this.characterSearch().trim().toLowerCase();
    if (!query) return this.characters();
    return this.characters().filter(
      (character) =>
        character.name.toLowerCase().includes(query) ||
        (character.description ?? '').toLowerCase().includes(query),
    );
  });

  readonly groupedCharacters = computed(() =>
    groupCharactersByType(this.filteredCharacters()),
  );

  ngOnInit(): void {
    this.subscriptions.push(
      this.store.subscribe('campaigns', (items) => {
        this.campaigns.set(items as unknown as CampaignFolder[]);
        this.recomputeEntities();
      }),
      this.store.subscribe('maps', (items) => {
        this.allMaps = items as unknown as MapData[];
        this.recomputeEntities();
      }),
      this.store.subscribe('characters', (items) => {
        this.allCharacters = items as unknown as Character[];
        this.recomputeEntities();
      }),
    );

    if (this.sessionId) {
      this.store
        .get('sessions', this.sessionId)
        .pipe(takeUntil(this.destroy$))
        .subscribe((session) => {
          const typed = (session as SessionState | undefined) ?? null;
          this.session.set(typed);
          if (typed) {
            this.notesDraft.set(typed.notes ?? '');
            this.recomputeEntities();
          }
        });
    }
  }

  ngOnDestroy(): void {
    this.destroy$.next();
    this.destroy$.complete();
    this.subscriptions.forEach((subscription) => subscription.unsubscribe());
    if (this.notesTimer !== null) clearTimeout(this.notesTimer);
  }

  private recomputeEntities(): void {
    const session = this.session();
    if (!session?.campaignId) {
      this.maps.set([]);
      this.characters.set([]);
      return;
    }
    const { maps, characters } = gatherCampaignEntities(
      this.campaigns(),
      this.allMaps,
      this.allCharacters,
      session.campaignId,
    );
    this.maps.set(maps);
    this.characters.set(characters);
  }

  startSession(campaignId: string): void {
    const folder = this.campaigns().find((item) => item.id === campaignId);
    const now = new Date();
    const session: SessionState = {
      id: crypto.randomUUID(),
      name: folder ? `Sessão — ${folder.name}` : 'Sessão',
      campaignId,
      activeMapId: null,
      quickReferences: [],
      toolbarShortcuts: [],
      notes: '',
      createdAt: now,
      updatedAt: now,
    };
    this.store.set('sessions', session);
    this.router.navigate(['/sessao', session.id]);
    const url = `${window.location.origin}${window.location.pathname}#/apresentar/${session.id}`;
    window.open(url, '_blank', 'noopener');
  }

  presentMap(mapId: string): void {
    const session = this.session();
    if (!session) return;
    this.store.patch('sessions', session.id, {
      activeMapId: mapId,
      updatedAt: new Date(),
    } as Partial<SessionState>);
    this.broadcast.publish({
      sessionId: session.id,
      campaignId: session.campaignId ?? null,
      activeMapId: mapId,
      revision: Date.now(),
    });
  }

  openPresentation(): void {
    const session = this.session();
    if (!session) return;
    if (!session.activeMapId && this.maps().length > 0) {
      this.presentMap(this.maps()[0].id);
    }
    const url = `${window.location.origin}${window.location.pathname}#/apresentar/${session.id}`;
    window.open(url, '_blank', 'noopener');
  }

  selectCharacter(character: Character): void {
    const session = this.session();
    if (!session) return;
    this.store.patch('sessions', session.id, {
      activeEntity: { type: 'character', id: character.id },
      updatedAt: new Date(),
    } as Partial<SessionState>);
  }

  openCharacter(character: Character): void {
    const url = `${window.location.origin}${window.location.pathname}#/personagens/${character.id}`;
    window.open(url, '_blank', 'noopener');
  }

  onNotesChange(value: string): void {
    this.notesDraft.set(value);
    if (this.notesTimer !== null) clearTimeout(this.notesTimer);
    this.notesTimer = setTimeout(() => {
      this.notesTimer = null;
      const session = this.session();
      if (!session) return;
      this.store.patch('sessions', session.id, {
        notes: this.notesDraft(),
        updatedAt: new Date(),
      } as Partial<SessionState>);
    }, 600);
  }

  openDiceRoller(): void {
    this.dialog.open(DiceRollerDialogComponent, {
      width: '400px',
      maxWidth: '90vw',
      panelClass: 'dice-roller-dialog',
    });
  }

  toggleMusic(): void {}

  togglePlayPause(): void {}

  goToCampaign(): void {
    this.router.navigate(['/campanha']);
  }

  goToSessions(): void {
    this.router.navigate(['/sessao']);
  }

  mapKindLabel(map: MapData): string {
    switch (map.kind) {
      case 'city':
        return 'Cidade';
      case 'dungeon':
        return 'Masmorra';
      case 'local':
        return 'Local';
      default:
        return 'Mundo';
    }
  }

  mapKindIcon(map: MapData): string {
    switch (map.kind) {
      case 'city':
        return 'location_city';
      case 'dungeon':
        return 'castle';
      case 'local':
        return 'storefront';
      default:
        return 'public';
    }
  }
}
