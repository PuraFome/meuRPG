import { Component, Input, OnDestroy, OnInit, inject, signal } from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormsModule, ReactiveFormsModule, FormArray, FormControl, FormGroup, Validators } from '@angular/forms';
import { Router } from '@angular/router';
import { BehaviorSubject, Subject } from 'rxjs';
import { debounceTime, distinctUntilChanged, takeUntil } from 'rxjs/operators';
import { MatTabsModule } from '@angular/material/tabs';
import { MatIconModule } from '@angular/material/icon';
import { MatButtonModule } from '@angular/material/button';
import { MatDialog, MatDialogModule } from '@angular/material/dialog';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatInputModule } from '@angular/material/input';
import { MatTooltipModule } from '@angular/material/tooltip';
import { QuillModule } from 'ngx-quill';
import { AvatarCropDialogComponent } from './avatar-crop-dialog.component';
import { ConfirmDialogComponent } from '../../shared/components/confirm-dialog.component';
import { StoreService } from '../../core/store/store.service';
import { PageHeaderComponent } from '../../shared/components/page-header.component';
import { CharacterSheetComponent } from './character-sheet.component';
import { LoadingSpinnerComponent } from '../../shared/components/loading-spinner.component';
import { EmptyStateComponent } from '../../shared/components/empty-state.component';
import type { Character } from '../../core/models/character';
import type { BreadcrumbItem } from '../../shared/components/page-header.component';

@Component({
  selector: 'app-character-detail',
  standalone: true,
  imports: [
    CommonModule,
    FormsModule,
    ReactiveFormsModule,
    MatTabsModule,
    MatIconModule,
    MatButtonModule,
    MatDialogModule,
    MatFormFieldModule,
    MatInputModule,
    MatTooltipModule,
    QuillModule,
    PageHeaderComponent,
    CharacterSheetComponent,
    LoadingSpinnerComponent,
    EmptyStateComponent,
  ],
  template: `
    <!-- Breadcrumbs + Back -->
    <app-page-header
      [breadcrumbs]="breadcrumbs"
      [title]="character?.name ?? 'Personagens'"
      icon="person"
    >
      <button mat-icon-button actions (click)="goBack()" aria-label="Voltar">
        <mat-icon>arrow_back</mat-icon>
      </button>
      @if (character) {
        <button
          mat-icon-button
          actions
          (click)="deleteCharacter()"
          color="warn"
          aria-label="Excluir personagem"
          matTooltip="Excluir personagem"
        >
          <mat-icon>delete</mat-icon>
        </button>
      }
    </app-page-header>

    @if (loading()) {
      <app-loading-spinner [isLoading]="true" message="Carregando personagem..." />
    } @else if (error()) {
      <div class="error-state">
        <app-empty-state
          icon="person_off"
          [message]="error() ?? 'Erro desconhecido'"
          actionLabel="Voltar"
          (action)="goBack()"
        />
      </div>
    } @else if (character) {
      <div class="detail-page">
        <!-- Header: avatar + name + type -->
        <div class="detail-header">
          <div class="avatar-container">
            @if (character.imageUrl) {
              <img [src]="character.imageUrl" class="avatar-image" alt="Avatar" />
            } @else {
              <div class="avatar-placeholder" [class]="'type-' + character.type">
                {{ character.name.charAt(0).toUpperCase() }}
              </div>
            }
            <button
              class="avatar-upload-btn"
              mat-mini-fab
              (click)="openAvatarUpload()"
              aria-label="Alterar avatar"
            >
              <mat-icon>camera_alt</mat-icon>
            </button>
          </div>
          <div class="header-info">
            <h1 class="character-name">{{ character.name }}</h1>
            <span class="type-badge" [class]="'type-badge type-' + character.type">
              {{ typeLabel(character.type) }}
            </span>
          </div>
        </div>

        <!-- 4 Tabs -->
        <mat-tab-group>
          <mat-tab label="História">
            <div class="tab-content">
              <quill-editor
                [(ngModel)]="character.history"
                (onContentChanged)="onHistoryChange($event.html ?? '')"
                [modules]="quillModules"
                placeholder="Escreva a história do personagem..."
              />
            </div>
          </mat-tab>
          <mat-tab label="Ficha">
            <div class="tab-content">
              <app-character-sheet [character]="character" />
            </div>
          </mat-tab>
          <mat-tab label="Notas do Mestre">
            <div class="tab-content">
              @if (isMasterRole$ | async) {
                <quill-editor
                  [(ngModel)]="character.masterNotes"
                  (onContentChanged)="onMasterNotesChange($event.html ?? '')"
                  [modules]="quillModules"
                  placeholder="Escreva notas do mestre..."
                />
              } @else {
                <div class="restricted-content">
                  <mat-icon class="restricted-icon">lock</mat-icon>
                  <p>Apenas o Mestre pode ver esta aba</p>
                </div>
              }
            </div>
          </mat-tab>
          <mat-tab label="Falas">
            <div class="tab-content">
              <div class="quotes-toolbar">
                <button mat-stroked-button (click)="addQuote()">
                  <mat-icon>add</mat-icon>
                  Adicionar Fala
                </button>
              </div>

              <div [formGroup]="quotesForm" class="quotes-form">
                <div formArrayName="items" class="quotes-list">
                  @for (item of quotesItems.controls; track item; let i = $index) {
                    <div [formGroupName]="i" class="quote-item">
                      <mat-form-field appearance="fill" class="quote-text-field">
                        <mat-label>Texto</mat-label>
                        <input
                          matInput
                          formControlName="texto"
                          placeholder="O que o personagem diz..."
                        />
                        @if (item.get('texto')?.invalid && item.get('texto')?.touched) {
                          <mat-error>Texto é obrigatório</mat-error>
                        }
                      </mat-form-field>
                      <mat-form-field appearance="fill" class="quote-context-field">
                        <mat-label>Contexto (opcional)</mat-label>
                        <input
                          matInput
                          formControlName="contexto"
                          placeholder="Em que situação..."
                        />
                      </mat-form-field>
                      <div class="quote-actions">
                        <button
                          mat-icon-button
                          (click)="copyQuote(item.value.texto)"
                          matTooltip="Copiar texto"
                          aria-label="Copiar texto"
                        >
                          <mat-icon>content_copy</mat-icon>
                        </button>
                        <button
                          mat-icon-button
                          (click)="removeQuote(i)"
                          color="warn"
                          matTooltip="Remover fala"
                          aria-label="Remover fala"
                        >
                          <mat-icon>delete</mat-icon>
                        </button>
                      </div>
                    </div>
                  }
                </div>
              </div>

              @if (quotesItems.length === 0) {
                <div class="empty-quotes">
                  <mat-icon>chat</mat-icon>
                  <p>Nenhuma fala cadastrada</p>
                </div>
              }
            </div>
          </mat-tab>
        </mat-tab-group>
      </div>
    }
  `,
  styles: `
    :host {
      display: block;
      max-width: 960px;
      margin: 0 auto;
    }

    .detail-page {
      padding: 8px 0;
    }

    /* ── Header ──────────────────────────────── */

    .detail-header {
      display: flex;
      align-items: center;
      gap: 16px;
      margin-bottom: 24px;
      flex-wrap: wrap;
    }

    .avatar-container {
      position: relative;
      flex-shrink: 0;
    }

    .avatar-image {
      width: 64px;
      height: 64px;
      border-radius: 50%;
      object-fit: cover;
      display: block;
    }

    .avatar-placeholder {
      width: 64px;
      height: 64px;
      border-radius: 50%;
      display: flex;
      align-items: center;
      justify-content: center;
      font-size: 1.75rem;
      font-weight: 700;
      color: #fff;
      text-shadow: 0 2px 6px rgba(0, 0, 0, 0.3);
    }

    .avatar-placeholder.type-player {
      background: linear-gradient(135deg, #065f46, #059669);
    }

    .avatar-placeholder.type-npc {
      background: linear-gradient(135deg, #155e75, #0891b2);
    }

    .avatar-placeholder.type-boss {
      background: linear-gradient(135deg, #7f1d1d, #dc2626);
    }

    .avatar-upload-btn {
      position: absolute;
      bottom: -4px;
      right: -4px;
      width: 28px;
      height: 28px;
      line-height: 28px;
    }

    .avatar-upload-btn .mat-icon {
      font-size: 16px;
      width: 16px;
      height: 16px;
      line-height: 16px;
    }

    .header-info {
      display: flex;
      align-items: center;
      gap: 12px;
      flex-wrap: wrap;
    }

    .character-name {
      margin: 0;
      font-size: 1.75rem;
      font-weight: 700;
      background: linear-gradient(135deg, #e0e0e0 0%, #b388ff 100%);
      -webkit-background-clip: text;
      -webkit-text-fill-color: transparent;
      background-clip: text;
    }

    .type-badge {
      display: inline-block;
      padding: 2px 10px;
      border-radius: 12px;
      font-size: 0.75rem;
      font-weight: 600;
      text-transform: uppercase;
      letter-spacing: 0.5px;
      color: #fff;
    }

    .type-badge.type-player {
      background: #059669;
    }

    .type-badge.type-npc {
      background: #0891b2;
    }

    .type-badge.type-boss {
      background: #dc2626;
    }

    /* ── Tabs ────────────────────────────────── */

    .tab-content {
      padding-top: 16px;
    }

    /* ── Restricted content (master role) ────── */

    .restricted-content {
      display: flex;
      flex-direction: column;
      align-items: center;
      justify-content: center;
      min-height: 160px;
      opacity: 0.5;
      gap: 8px;
    }

    .restricted-icon {
      font-size: 2rem;
      width: 2rem;
      height: 2rem;
    }

    .restricted-content p {
      margin: 0;
      font-size: 1rem;
    }

    /* ── Quotes (Falas) ──────────────────────── */

    .quotes-toolbar {
      margin-bottom: 16px;
    }

    .quotes-form {
      margin-bottom: 16px;
    }

    .quotes-list {
      display: flex;
      flex-direction: column;
      gap: 16px;
    }

    .quote-item {
      display: flex;
      align-items: flex-start;
      gap: 12px;
      padding: 12px;
      border: 1px solid rgba(255, 255, 255, 0.08);
      border-radius: 8px;
      background: rgba(0, 0, 0, 0.12);
    }

    .quote-text-field {
      flex: 2;
    }

    .quote-context-field {
      flex: 1;
    }

    .quote-actions {
      display: flex;
      flex-direction: column;
      gap: 4px;
      padding-top: 4px;
    }

    @media (max-width: 560px) {
      .quote-item {
        flex-direction: column;
        gap: 8px;
      }
    }

    .empty-quotes {
      display: flex;
      flex-direction: column;
      align-items: center;
      justify-content: center;
      min-height: 120px;
      opacity: 0.5;
      gap: 8px;
    }

    .empty-quotes mat-icon {
      font-size: 2rem;
      width: 2rem;
      height: 2rem;
    }

    .empty-quotes p {
      margin: 0;
      font-size: 1rem;
    }

    /* ── Error ─────────────────────────────── */

    .error-state {
      display: flex;
      justify-content: center;
      align-items: center;
      min-height: 300px;
    }
  `,
})
export class CharacterDetailComponent implements OnInit, OnDestroy {
  @Input() id = '';

  private readonly store = inject(StoreService<Character>);
  private readonly router = inject(Router);
  private readonly dialog = inject(MatDialog);

  character: Character | null = null;
  readonly loading = signal(true);
  readonly error = signal<string | null>(null);

  /** Hardcoded to true for MVP — replace with real auth check later. */
  readonly isMasterRole$ = new BehaviorSubject(true);

  readonly quillModules = {
    toolbar: [
      ['bold', 'italic'],
      [{ list: 'ordered' }, { list: 'bullet' }],
      [{ header: [1, 2, 3, false] }],
    ],
  };

  // ── Save subjects (debounced) ──────────────────────────────
  private readonly historySaveSubject = new Subject<string>();
  private readonly masterNotesSaveSubject = new Subject<string>();
  private readonly destroy$ = new Subject<void>();

  // ── Quotes form ────────────────────────────────────────────
  readonly quotesForm = new FormGroup({
    items: new FormArray<FormGroup>([]),
  });

  get quotesItems(): FormArray<FormGroup> {
    return this.quotesForm.get('items') as FormArray<FormGroup>;
  }

  private quotesInitialized = false;

  // ── Breadcrumbs ────────────────────────────────────────────
  get breadcrumbs(): BreadcrumbItem[] {
    return [{ label: 'Personagens', route: '/personagens' }];
  }

  constructor() {
    // Debounced save for history
    this.historySaveSubject.pipe(
      debounceTime(1000),
      distinctUntilChanged(),
      takeUntil(this.destroy$),
    ).subscribe((html) => {
      if (this.character?.id) {
        this.store.patch('characters', this.character.id, {
          history: html,
          updatedAt: new Date(),
        });
      }
    });

    // Debounced save for master notes
    this.masterNotesSaveSubject.pipe(
      debounceTime(1000),
      distinctUntilChanged(),
      takeUntil(this.destroy$),
    ).subscribe((html) => {
      if (this.character?.id) {
        this.store.patch('characters', this.character.id, {
          masterNotes: html,
          updatedAt: new Date(),
        });
      }
    });

    // Debounced save for quotes
    this.quotesItems.valueChanges.pipe(
      debounceTime(1000),
      takeUntil(this.destroy$),
    ).subscribe(() => {
      if (this.quotesInitialized) {
        this.saveQuotes();
      }
    });
  }

  ngOnInit(): void {
    this.loading.set(true);
    this.error.set(null);

    if (this.id) {
      this.store.get('characters', this.id).pipe(
        takeUntil(this.destroy$),
      ).subscribe((c) => {
        this.loading.set(false);
        if (c) {
          this.character = c;
          this.initQuotesForm();
        } else {
          this.error.set('Personagem não encontrado');
        }
      });
    } else {
      this.loading.set(false);
      this.error.set('Personagem não encontrado');
    }
  }

  ngOnDestroy(): void {
    this.destroy$.next();
    this.destroy$.complete();
  }

  // ── Labels ────────────────────────────────────
  typeLabel(type: 'npc' | 'player' | 'boss'): string {
    switch (type) {
      case 'player':
        return 'Jogador';
      case 'npc':
        return 'NPC';
      case 'boss':
        return 'Boss';
    }
  }

  // ── Navigation ────────────────────────────────
  goBack(): void {
    this.router.navigate(['/personagens']);
  }

  // ── Delete ────────────────────────────────────
  deleteCharacter(): void {
    const character = this.character;
    if (!character) return;

    const dialogRef = this.dialog.open(ConfirmDialogComponent, {
      data: {
        title: 'Excluir personagem',
        message: `Tem certeza que deseja excluir "${character.name}"?`,
        confirmText: 'Excluir',
        cancelText: 'Cancelar',
      },
      width: '420px',
      maxWidth: '95vw',
    });

    dialogRef.afterClosed().pipe(takeUntil(this.destroy$)).subscribe((confirmed: boolean) => {
      if (confirmed) {
        this.store.delete('characters', character.id);
        this.router.navigate(['/personagens']);
      }
    });
  }

  // ── History ────────────────────────────────────
  onHistoryChange(html: string): void {
    this.historySaveSubject.next(html);
  }

  // ── Master Notes ───────────────────────────────
  onMasterNotesChange(html: string): void {
    this.masterNotesSaveSubject.next(html);
  }

  // ── Avatar Upload ──────────────────────────────
  openAvatarUpload(): void {
    const input = document.createElement('input');
    input.type = 'file';
    input.accept = 'image/png,image/jpeg,image/gif,image/webp';
    input.addEventListener('change', () => {
      const file = input.files?.[0];
      if (!file) return;

      const reader = new FileReader();
      reader.onload = (e) => {
        const dataUrl = e.target?.result as string;
        if (!dataUrl) return;

        const dialogRef = this.dialog.open(AvatarCropDialogComponent, {
          data: dataUrl,
          width: '500px',
          maxWidth: '95vw',
        });

        dialogRef.afterClosed().pipe(takeUntil(this.destroy$)).subscribe((croppedUrl?: string) => {
          if (croppedUrl && this.character?.id) {
            this.character.imageUrl = croppedUrl;
            this.store.patch('characters', this.character.id, {
              imageUrl: croppedUrl,
              updatedAt: new Date(),
            });
          }
        });
      };
      reader.readAsDataURL(file);
    });
    input.click();
  }

  // ── Quotes (Falas) ────────────────────────────
  private createQuoteGroup(texto = '', contexto = ''): FormGroup {
    return new FormGroup({
      texto: new FormControl(texto, Validators.required),
      contexto: new FormControl(contexto),
    });
  }

  private initQuotesForm(): void {
    this.quotesInitialized = false;
    const quotes = this.character?.quotes ?? [];

    // Clear existing items
    while (this.quotesItems.length) {
      this.quotesItems.removeAt(0);
    }

    for (const q of quotes) {
      let texto = '';
      let contexto = '';
      try {
        const parsed = JSON.parse(q);
        texto = parsed.texto ?? q;
        contexto = parsed.contexto ?? '';
      } catch {
        texto = q;
      }
      this.quotesItems.push(this.createQuoteGroup(texto, contexto));
    }

    this.quotesInitialized = true;
  }

  addQuote(): void {
    this.quotesItems.push(this.createQuoteGroup('', ''));
    // Focus the newly added text field
    const lastIndex = this.quotesItems.length - 1;
    const textoControl = this.quotesItems.at(lastIndex).get('texto');
    if (textoControl) {
      textoControl.markAsTouched();
    }
  }

  removeQuote(index: number): void {
    this.quotesItems.removeAt(index);
  }

  copyQuote(text: string): void {
    navigator.clipboard.writeText(text).catch(() => {
      // Clipboard not available — silently ignore
    });
  }

  private saveQuotes(): void {
    if (!this.character?.id) return;
    const serialized = this.quotesItems.value.map(
      (q: { texto: string; contexto?: string }) =>
        JSON.stringify({ texto: q.texto, contexto: q.contexto }),
    );
    this.store.patch('characters', this.character.id, {
      quotes: serialized,
      updatedAt: new Date(),
    });
  }
}
