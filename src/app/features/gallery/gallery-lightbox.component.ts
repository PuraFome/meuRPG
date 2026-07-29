import {
  Component,
  computed,
  effect,
  HostListener,
  input,
  OnDestroy,
  output,
  signal,
  ViewChild,
} from '@angular/core';
import { CommonModule } from '@angular/common';
import { MatIconModule } from '@angular/material/icon';
import { MatButtonModule } from '@angular/material/button';
import { IndexedDbFileRepository } from '../../core/repositories/indexed-db-file-repository';
import { AudioPlayerComponent } from '../../shared/components/audio-player.component';
import type { GalleryItem } from '../../core/models/gallery';

@Component({
  selector: 'app-gallery-lightbox',
  standalone: true,
  imports: [
    CommonModule,
    MatIconModule,
    MatButtonModule,
    AudioPlayerComponent,
  ],
  template: `
    @if (visible()) {
      <div
        class="lightbox-overlay"
        (click)="onBackdropClick($event)"
        tabindex="0"
      >
        <!-- Top bar -->
        <div class="lightbox-topbar">
          <span class="item-name">{{ currentItem()?.name }}</span>
          <button
            mat-icon-button
            class="close-btn"
            (click)="close.emit()"
            aria-label="Fechar"
          >
            <mat-icon>close</mat-icon>
          </button>
        </div>

        <!-- Main body -->
        <div class="lightbox-body">
          @if (items().length > 1) {
            <button
              class="nav-btn nav-prev"
              (click)="prev()"
              aria-label="Anterior"
            >
              <mat-icon>chevron_left</mat-icon>
            </button>
          }

          <div class="lightbox-content">
            @if (currentItem()?.type === 'image') {
              <img
                [src]="currentImageUrl()"
                [alt]="currentItem()?.name"
                class="lightbox-image"
                [class.loaded]="imageLoaded()"
                (load)="imageLoaded.set(true)"
                (error)="imageLoaded.set(true)"
              />
            } @else if (currentItem()?.type === 'audio') {
              <div class="audio-panel">
                <h3 class="audio-panel-title">
                  <mat-icon class="audio-panel-icon">audiotrack</mat-icon>
                  Faixas de Áudio
                </h3>
                <div class="audio-track-list">
                  @for (track of audioItems(); track track.id; let i = $index) {
                    <button
                      class="audio-track-item"
                      [class.active]="i === currentAudioIndex()"
                      (click)="toggleAudioTrack(i)"
                    >
                      <mat-icon class="track-play-icon">
                        {{
                          i === currentAudioIndex() && audioPlaying()
                            ? 'pause'
                            : 'play_arrow'
                        }}
                      </mat-icon>
                      <span class="track-name">{{ track.name }}</span>
                    </button>
                  }
                </div>
                @if (currentAudioSrc()) {
                  <div class="current-player">
                    <app-audio-player
                      [src]="currentAudioSrc()"
                      [title]="currentAudioTitle()"
                      (ended)="onAudioEnded()"
                    />
                  </div>
                }
              </div>
            } @else {
              <div class="other-placeholder">
                <mat-icon class="other-icon">insert_drive_file</mat-icon>
                <p class="other-name">{{ currentItem()?.name }}</p>
              </div>
            }
          </div>

          @if (items().length > 1) {
            <button
              class="nav-btn nav-next"
              (click)="next()"
              aria-label="Próximo"
            >
              <mat-icon>chevron_right</mat-icon>
            </button>
          }
        </div>

        <!-- Bottom bar -->
        <div class="lightbox-bottombar">
          <span class="counter"
            >{{ currentIndex() + 1 }} / {{ items().length }}</span
          >
        </div>
      </div>
    }
  `,
  styles: `
    :host {
      display: contents;
    }

    .lightbox-overlay {
      position: fixed;
      inset: 0;
      z-index: 1000;
      display: flex;
      flex-direction: column;
      background: rgba(0, 0, 0, 0.92);
      backdrop-filter: blur(4px);
      animation: fadeIn 0.2s ease;
    }

    @keyframes fadeIn {
      from { opacity: 0; }
      to   { opacity: 1; }
    }

    /* ── Top bar ── */
    .lightbox-topbar {
      display: flex;
      align-items: center;
      justify-content: space-between;
      padding: 12px 16px;
      min-height: 56px;
      flex-shrink: 0;
    }

    .item-name {
      font-size: 0.9rem;
      opacity: 0.7;
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
      margin-right: 12px;
    }

    .close-btn {
      flex-shrink: 0;
      color: rgba(255, 255, 255, 0.7);
      transition: color 0.15s, transform 0.15s;
    }

    .close-btn:hover {
      color: #fff;
      transform: scale(1.1);
    }

    /* ── Body ── */
    .lightbox-body {
      flex: 1;
      display: flex;
      align-items: center;
      justify-content: center;
      gap: 16px;
      padding: 8px 16px;
      min-height: 0;
    }

    .lightbox-content {
      flex: 1;
      display: flex;
      align-items: center;
      justify-content: center;
      max-width: 90vw;
      max-height: 90vh;
    }

    /* ── Image ── */
    .lightbox-image {
      max-width: 90vw;
      max-height: 85vh;
      object-fit: contain;
      border-radius: 4px;
      opacity: 0;
      transition: opacity 0.35s ease;
      box-shadow: 0 4px 40px rgba(0, 0, 0, 0.5);
    }

    .lightbox-image.loaded {
      opacity: 1;
    }

    /* ── Navigation buttons ── */
    .nav-btn {
      flex-shrink: 0;
      display: flex;
      align-items: center;
      justify-content: center;
      width: 48px;
      height: 48px;
      border-radius: 50%;
      border: none;
      background: rgba(255, 255, 255, 0.06);
      color: rgba(255, 255, 255, 0.6);
      cursor: pointer;
      transition: background 0.2s, color 0.2s, transform 0.2s;
    }

    .nav-btn:hover {
      background: rgba(255, 255, 255, 0.14);
      color: #fff;
      transform: scale(1.08);
    }

    .nav-btn:active {
      transform: scale(0.95);
    }

    .nav-btn mat-icon {
      font-size: 32px;
      width: 32px;
      height: 32px;
    }

    /* ── Audio panel ── */
    .audio-panel {
      max-width: 500px;
      width: 100%;
      padding: 8px 0;
    }

    .audio-panel-title {
      display: flex;
      align-items: center;
      gap: 8px;
      margin: 0 0 16px;
      font-size: 1rem;
      font-weight: 500;
      opacity: 0.7;
    }

    .audio-panel-icon {
      font-size: 1.2rem;
      width: 1.2rem;
      height: 1.2rem;
      opacity: 0.6;
    }

    .audio-track-list {
      display: flex;
      flex-direction: column;
      gap: 4px;
      margin-bottom: 16px;
      max-height: 400px;
      overflow-y: auto;
    }

    .audio-track-item {
      all: unset;
      display: flex;
      align-items: center;
      gap: 12px;
      padding: 10px 14px;
      border-radius: 8px;
      cursor: pointer;
      transition: background 0.15s;
      font-size: 0.88rem;
      color: rgba(255, 255, 255, 0.7);
    }

    .audio-track-item:hover {
      background: rgba(255, 255, 255, 0.06);
      color: #fff;
    }

    .audio-track-item.active {
      background: rgba(206, 147, 216, 0.12);
      color: #ce93d8;
    }

    .track-play-icon {
      font-size: 1.3rem;
      width: 1.3rem;
      height: 1.3rem;
      flex-shrink: 0;
    }

    .track-name {
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
    }

    .current-player {
      padding: 8px;
      border-radius: 8px;
      background: rgba(255, 255, 255, 0.03);
    }

    /* ── Other type ── */
    .other-placeholder {
      display: flex;
      flex-direction: column;
      align-items: center;
      gap: 12px;
      opacity: 0.5;
    }

    .other-icon {
      font-size: 3rem;
      width: 3rem;
      height: 3rem;
    }

    .other-name {
      margin: 0;
      font-size: 1rem;
      text-align: center;
    }

    /* ── Bottom bar ── */
    .lightbox-bottombar {
      display: flex;
      align-items: center;
      justify-content: center;
      padding: 12px 16px;
      min-height: 48px;
      flex-shrink: 0;
    }

    .counter {
      font-size: 0.85rem;
      font-variant-numeric: tabular-nums;
      opacity: 0.5;
    }
  `,
})
export class GalleryLightboxComponent implements OnDestroy {
  readonly visible = input(false);
  readonly items = input<GalleryItem[]>([]);
  readonly currentIndex = input(0);

  readonly close = output<void>();
  readonly indexChange = output<number>();

  @ViewChild(AudioPlayerComponent) audioPlayer?: AudioPlayerComponent;

  private readonly fileRepo = new IndexedDbFileRepository();
  private blobUrls = new Map<string, string>();

  readonly currentItem = computed(() => {
    const all = this.items();
    const idx = this.currentIndex();
    return all[idx] ?? null;
  });

  readonly audioItems = computed(() =>
    this.items().filter((i) => i.type === 'audio'),
  );

  readonly currentImageUrl = signal('');
  readonly imageLoaded = signal(false);

  readonly currentAudioIndex = signal(-1);
  readonly currentAudioSrc = signal('');
  readonly currentAudioTitle = signal('');
  readonly audioPlaying = signal(false);

  constructor() {
    effect(() => {
      if (this.visible() && this.currentItem()) {
        this.loadCurrentItem();
      }
    });
  }

  ngOnDestroy(): void {
    this.revokeAllBlobUrls();
  }

  // ── Navigation ──

  prev(): void {
    const len = this.items().length;
    if (len <= 1) return;
    this.navigateTo((this.currentIndex() - 1 + len) % len);
  }

  next(): void {
    const len = this.items().length;
    if (len <= 1) return;
    this.navigateTo((this.currentIndex() + 1) % len);
  }

  private navigateTo(index: number): void {
    if (index === this.currentIndex()) return;
    this.imageLoaded.set(false);
    this.resetAudio();
    this.indexChange.emit(index);
  }

  // ── Keyboard ──

  @HostListener('document:keydown', ['$event'])
  onKeydown(event: KeyboardEvent): void {
    if (!this.visible()) return;
    if (
      event.target instanceof HTMLInputElement ||
      event.target instanceof HTMLTextAreaElement
    ) {
      return;
    }

    switch (event.key) {
      case 'Escape':
        event.preventDefault();
        this.close.emit();
        break;
      case 'ArrowLeft':
        event.preventDefault();
        this.prev();
        break;
      case 'ArrowRight':
        event.preventDefault();
        this.next();
        break;
    }
  }

  // ── Backdrop click ──

  onBackdropClick(event: MouseEvent): void {
    if ((event.target as HTMLElement).classList.contains('lightbox-overlay')) {
      this.close.emit();
    }
  }

  // ── Audio controls ──

  toggleAudioTrack(index: number): void {
    const audioItems = this.audioItems();
    if (index < 0 || index >= audioItems.length) return;

    // Same track toggle
    if (index === this.currentAudioIndex()) {
      this.audioPlayer?.togglePlay();
      this.audioPlaying.update((v) => !v);
      return;
    }

    // Load new track
    const track = audioItems[index];
    this.currentAudioIndex.set(index);
    this.currentAudioTitle.set(track.name);

    this.getBlobUrl(track.id).then((url) => {
      if (url) {
        this.currentAudioSrc.set(url);
        this.audioPlaying.set(true);
        // Play after DOM update and audio metadata loads
        setTimeout(() => {
          this.audioPlayer?.togglePlay();
        }, 100);
      }
    });
  }

  onAudioEnded(): void {
    this.audioPlaying.set(false);
    const audioItems = this.audioItems();
    const currentIdx = this.currentAudioIndex();
    if (currentIdx >= 0 && currentIdx < audioItems.length - 1) {
      // Auto-advance to next track
      this.currentAudioIndex.set(-1); // force fresh load
      this.toggleAudioTrack(currentIdx + 1);
    } else {
      this.currentAudioIndex.set(-1);
      this.currentAudioSrc.set('');
    }
  }

  // ── Internal ──

  private resetAudio(): void {
    if (this.audioPlayer?.playing()) {
      this.audioPlayer.togglePlay();
    }
    this.currentAudioIndex.set(-1);
    this.currentAudioSrc.set('');
    this.currentAudioTitle.set('');
    this.audioPlaying.set(false);
  }

  private async loadCurrentItem(): Promise<void> {
    const item = this.currentItem();
    if (!item || !this.visible()) return;

    if (item.type === 'image') {
      const url = await this.getBlobUrl(item.id);
      if (url) {
        this.currentImageUrl.set(url);
        // If already cached, mark as loaded immediately
        if (this.blobUrls.has(item.id)) {
          this.imageLoaded.set(true);
        }
      }
    }
  }

  private async getBlobUrl(id: string): Promise<string | null> {
    if (this.blobUrls.has(id)) return this.blobUrls.get(id)!;

    try {
      const record = await this.fileRepo.get(id);
      if (record) {
        const url = URL.createObjectURL(record.data);
        this.blobUrls.set(id, url);
        return url;
      }
    } catch {
      // Ignore load failures
    }
    return null;
  }

  private revokeAllBlobUrls(): void {
    for (const url of this.blobUrls.values()) {
      URL.revokeObjectURL(url);
    }
    this.blobUrls.clear();
  }
}
