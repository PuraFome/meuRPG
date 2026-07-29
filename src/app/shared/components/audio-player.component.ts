import {
  Component,
  ElementRef,
  EventEmitter,
  Input,
  Output,
  ViewChild,
  AfterViewInit,
  OnDestroy,
  signal,
} from '@angular/core';
import { MatIconModule } from '@angular/material/icon';
import { MatButtonModule } from '@angular/material/button';

@Component({
  selector: 'app-audio-player',
  standalone: true,
  imports: [MatIconModule, MatButtonModule],
  template: `
    <div class="audio-player">
      <audio
        #audioEl
        [src]="src"
        (timeupdate)="onTimeUpdate()"
        (loadedmetadata)="onLoaded()"
        (ended)="onEnded()"
        (error)="onError()"
      ></audio>

      @if (error()) {
        <div class="error-state">
          <mat-icon class="error-icon">error_outline</mat-icon>
          <span>Erro ao carregar áudio</span>
        </div>
      } @else {
        <div class="controls">
          <button
            mat-icon-button
            (click)="togglePlay()"
            [attr.aria-label]="playing() ? 'Pausar' : 'Reproduzir'"
          >
            <mat-icon>{{ playing() ? 'pause' : 'play_arrow' }}</mat-icon>
          </button>

          <span class="time">{{ currentTime }}</span>

          <input
            type="range"
            class="seek-slider"
            [min]="0"
            [max]="durationSeconds()"
            [value]="currentSeconds()"
            (input)="onSeek($event)"
            aria-label="Posição da faixa"
          />

          <span class="time">{{ duration }}</span>

          <button
            mat-icon-button
            (click)="toggleLoop()"
            [class.active]="loop()"
            [attr.aria-label]="loop() ? 'Desativar repetição' : 'Ativar repetição'"
          >
            <mat-icon>repeat</mat-icon>
          </button>

          <div class="volume-control">
            <button mat-icon-button (click)="toggleMute()" aria-label="Volume">
              <mat-icon>{{
                muted() ? 'volume_off' : volume() === 0 ? 'volume_mute' : volume() < 0.5 ? 'volume_down' : 'volume_up'
              }}</mat-icon>
            </button>
            <input
              type="range"
              class="volume-slider"
              min="0"
              max="1"
              step="0.05"
              [value]="muted() ? 0 : volume()"
              (input)="onVolumeChange($event)"
              aria-label="Volume"
            />
          </div>
        </div>

        @if (title) {
          <div class="title">{{ title }}</div>
        }
      }
    </div>
  `,
  styles: [
    `
      .audio-player {
        background: rgba(255, 255, 255, 0.04);
        border-radius: 8px;
        padding: 8px 12px;
        max-width: 480px;
      }
      .controls {
        display: flex;
        align-items: center;
        gap: 8px;
      }
      .time {
        font-size: 0.8rem;
        opacity: 0.65;
        min-width: 40px;
        font-variant-numeric: tabular-nums;
      }
      .seek-slider {
        flex: 1;
        height: 4px;
        cursor: pointer;
        accent-color: #ce93d8;
      }
      .volume-control {
        display: flex;
        align-items: center;
        gap: 4px;
      }
      .volume-slider {
        width: 64px;
        height: 4px;
        cursor: pointer;
        accent-color: #ce93d8;
      }
      .title {
        font-size: 0.85rem;
        opacity: 0.7;
        margin-top: 4px;
        padding-left: 4px;
      }
      .active {
        color: #ce93d8;
      }
      .error-state {
        display: flex;
        align-items: center;
        gap: 8px;
        padding: 12px;
        color: #f44336;
        font-size: 0.9rem;
      }
      .error-icon {
        font-size: 1.2rem;
        width: 1.2rem;
        height: 1.2rem;
      }
    `,
  ],
})
export class AudioPlayerComponent implements AfterViewInit, OnDestroy {
  @Input() src = '';
  @Input() title?: string;
  @Output() ended = new EventEmitter<void>();

  @ViewChild('audioEl') audioEl!: ElementRef<HTMLAudioElement>;

  private audio!: HTMLAudioElement;

  readonly playing = signal(false);
  readonly currentSeconds = signal(0);
  readonly durationSeconds = signal(0);
  readonly volume = signal(1);
  readonly muted = signal(false);
  readonly loop = signal(false);
  readonly error = signal(false);

  ngAfterViewInit(): void {
    this.audio = this.audioEl.nativeElement;
  }

  ngOnDestroy(): void {
    this.audio?.pause();
  }

  get currentTime(): string {
    return this.formatTime(this.currentSeconds());
  }

  get duration(): string {
    return this.formatTime(this.durationSeconds());
  }

  private formatTime(seconds: number): string {
    if (isNaN(seconds) || seconds === 0) return '0:00';
    const m = Math.floor(seconds / 60);
    const s = Math.floor(seconds % 60);
    return `${m}:${s.toString().padStart(2, '0')}`;
  }

  togglePlay(): void {
    if (this.audio.paused) {
      this.audio.play().catch(() => {});
      this.playing.set(true);
    } else {
      this.audio.pause();
      this.playing.set(false);
    }
  }

  onTimeUpdate(): void {
    this.currentSeconds.set(this.audio.currentTime);
  }

  onLoaded(): void {
    this.durationSeconds.set(this.audio.duration);
    this.error.set(false);
  }

  onEnded(): void {
    this.playing.set(false);
    this.currentSeconds.set(0);
    this.ended.emit();
  }

  onError(): void {
    this.error.set(true);
    this.playing.set(false);
  }

  onSeek(event: Event): void {
    const target = event.target as HTMLInputElement;
    const value = parseFloat(target.value);
    this.audio.currentTime = value;
    this.currentSeconds.set(value);
  }

  onVolumeChange(event: Event): void {
    const target = event.target as HTMLInputElement;
    const value = parseFloat(target.value);
    this.audio.volume = value;
    this.volume.set(value);
    this.muted.set(value === 0);
  }

  toggleMute(): void {
    this.muted.update((m) => !m);
    this.audio.muted = this.muted();
  }

  toggleLoop(): void {
    this.loop.update((l) => !l);
    this.audio.loop = this.loop();
  }
}
