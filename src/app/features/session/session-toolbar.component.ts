import { Component, Output, EventEmitter } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';
import { MatTooltipModule } from '@angular/material/tooltip';

@Component({
  selector: 'app-session-toolbar',
  standalone: true,
  imports: [MatButtonModule, MatIconModule, MatTooltipModule],
  template: `
    <div class="toolbar">
      <div class="toolbar-group">
        <span class="toolbar-label">Ferramentas</span>
      </div>

      <div class="toolbar-actions">
        <button
          mat-mini-fab
          class="tool-btn"
          (click)="rollDice.emit()"
          matTooltip="Rolar Dados (ex: 1d20, 3d6)"
          aria-label="Rolar Dados"
        >
          <mat-icon>casino</mat-icon>
        </button>
        <span class="btn-label">Rolar Dados</span>

        <button
          mat-mini-fab
          class="tool-btn"
          (click)="openMap.emit()"
          matTooltip="Abrir o mapa principal da campanha"
          aria-label="Abrir Mapa"
        >
          <mat-icon>map</mat-icon>
        </button>
        <span class="btn-label">Abrir Mapa</span>

        <button
          mat-mini-fab
          class="tool-btn"
          (click)="toggleMusic.emit()"
          matTooltip="Tocar música de fundo"
          aria-label="Tocar Música"
        >
          <mat-icon>music_note</mat-icon>
        </button>
        <span class="btn-label">Tocar Música</span>

        <button
          mat-mini-fab
          class="tool-btn"
          (click)="togglePlayPause.emit()"
          matTooltip="Pausar / Continuar sessão"
          aria-label="Play / Pause"
        >
          <mat-icon>play_arrow</mat-icon>
        </button>
        <span class="btn-label">Play / Pause</span>
      </div>
    </div>
  `,
  styles: `
    :host {
      display: block;
      flex-shrink: 0;
    }

    .toolbar {
      display: flex;
      align-items: center;
      gap: 12px;
      padding: 8px 16px;
      background: rgba(255, 255, 255, 0.02);
      border-bottom: 1px solid rgba(255, 255, 255, 0.06);
    }

    .toolbar-group {
      display: flex;
      align-items: center;
      gap: 8px;
      margin-right: 4px;
    }

    .toolbar-label {
      font-size: 0.7rem;
      font-weight: 600;
      text-transform: uppercase;
      letter-spacing: 0.5px;
      color: rgba(255, 255, 255, 0.3);
      padding-right: 12px;
      border-right: 1px solid rgba(255, 255, 255, 0.08);
    }

    .toolbar-actions {
      display: flex;
      align-items: center;
      gap: 6px;
      flex-wrap: wrap;
    }

    .tool-btn {
      --mdc-fab-container-color: rgba(255, 255, 255, 0.04);
      --mdc-fab-icon-size: 20px;
      width: 36px;
      height: 36px;
      transition: all 0.15s ease;
    }

    .tool-btn:hover {
      --mdc-fab-container-color: rgba(255, 255, 255, 0.1);
    }

    .btn-label {
      font-size: 0.7rem;
      color: rgba(255, 255, 255, 0.45);
      margin-right: 12px;
      white-space: nowrap;
    }

    .btn-label:last-child {
      margin-right: 0;
    }
  `,
})
export class SessionToolbarComponent {
  @Output() rollDice = new EventEmitter<void>();
  @Output() openMap = new EventEmitter<void>();
  @Output() toggleMusic = new EventEmitter<void>();
  @Output() togglePlayPause = new EventEmitter<void>();
}
