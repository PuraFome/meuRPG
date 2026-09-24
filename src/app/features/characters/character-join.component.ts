import { Component, Input, OnInit, inject } from '@angular/core';
import { CommonModule } from '@angular/common';
import { MatProgressSpinnerModule } from '@angular/material/progress-spinner';
import { CharactersService } from '../../core/services/characters.service';
import { Character } from '../../core/models/character';
import { CharacterFormComponent } from './character-form.component';
import { EmptyStateComponent } from '../../shared/components/empty-state.component';

@Component({
  selector: 'app-character-join',
  standalone: true,
  imports: [CommonModule, MatProgressSpinnerModule, CharacterFormComponent, EmptyStateComponent],
  template: `
    @if (loading) {
      <mat-spinner [diameter]="40" />
    } @else if (expired) {
      <app-empty-state icon="link_off" message="Link expirado ou inválido" />
    } @else if (valid && !joined) {
      <app-character-form mode="join" [joinToken]="token" (saved)="onJoined($event)" />
    } @else if (joined) {
      <div class="success-panel">
        <h2>Personagem {{ joined.name }} enviado!</h2>
      </div>
    }
  `,
  styles: [
    `
      :host {
        display: block;
        max-width: 720px;
        margin: 0 auto;
        padding: 24px;
      }
      .success-panel {
        text-align: center;
        padding: 48px 16px;
      }
      .success-panel h2 {
        color: #c4b5fd;
      }
    `,
  ],
})
export class CharacterJoinComponent implements OnInit {
  @Input() token = '';

  loading = true;
  valid = false;
  expired = false;
  joined: Character | null = null;

  private readonly characters = inject(CharactersService);

  ngOnInit(): void {
    this.characters.validateJoinToken(this.token).subscribe({
      next: () => {
        this.valid = true;
        this.loading = false;
      },
      error: () => {
        this.expired = true;
        this.loading = false;
      },
    });
  }

  onJoined(c: Character): void {
    this.joined = c;
  }
}
