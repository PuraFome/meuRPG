import { Component, DestroyRef, ElementRef, inject, signal, viewChild } from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { FormControl, ReactiveFormsModule } from '@angular/forms';
import { MatButtonModule } from '@angular/material/button';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatIconModule } from '@angular/material/icon';
import { MatInputModule } from '@angular/material/input';
import { ActivatedRoute, Router, RouterLink } from '@angular/router';
import { Code } from '@connectrpc/connect';

import { describeConnectError } from '../../../core/connect/connect-errors';
import { MapsClient } from '../../../core/maps/maps-client';
import { GalleryPicker } from '../../../shared/gallery-picker/gallery-picker';

const NAME_MAX = 80;

/** The map's name rule (maps.proto): 1 to 80 characters, one line. */
export function mapNameError(name: string): string | null {
  const trimmed = name.trim();
  if (trimmed === '') {
    return 'Dê um nome ao mapa.';
  }
  if ([...trimmed].length > NAME_MAX) {
    return `Use até ${NAME_MAX} caracteres.`;
  }
  if (/\p{Cc}/u.test(trimmed)) {
    return 'Use um nome numa linha só.';
  }
  return null;
}

/**
 * "Novo mapa" (E5-31, MR-008): a name and an image of the gallery, chosen
 * from the picker's tiles (or uploaded on the spot). "Criar mapa" is the
 * screen's one filled button and opens the editor on the new map. The map
 * is born hidden, and the note says so.
 */
@Component({
  selector: 'app-map-new',
  imports: [
    GalleryPicker,
    MatButtonModule,
    MatFormFieldModule,
    MatIconModule,
    MatInputModule,
    ReactiveFormsModule,
    RouterLink,
  ],
  templateUrl: './map-new.html',
  styleUrl: './map-new.scss',
})
export class MapNew {
  private readonly api = inject(MapsClient);
  private readonly router = inject(Router);

  protected readonly campaignId = signal('');
  protected readonly nameControl = new FormControl('', { nonNullable: true });
  protected readonly imageId = signal<string | null>(null);
  protected readonly submitted = signal(false);
  protected readonly saving = signal(false);
  protected readonly failure = signal<string | null>(null);
  protected readonly nameMax = NAME_MAX;

  private readonly nameField = viewChild('nameField', { read: ElementRef<HTMLInputElement> });

  protected readonly nameError = signal<string | null>(null);
  protected readonly imageError = () =>
    this.submitted() && this.imageId() === null ? 'Escolha uma imagem para o mapa.' : null;

  constructor() {
    inject(ActivatedRoute)
      .paramMap.pipe(takeUntilDestroyed(inject(DestroyRef)))
      .subscribe((params) => this.campaignId.set(params.get('id') ?? ''));
  }

  protected async submit(): Promise<void> {
    this.submitted.set(true);
    this.failure.set(null);
    const imageId = this.imageId();
    const invalid = mapNameError(this.nameControl.value);
    this.nameError.set(invalid);
    if (invalid !== null) {
      // The message is the field's `mat-error` (red outline, read with the field).
      this.nameControl.setErrors({ name: true });
      this.nameControl.markAsTouched();
      this.nameField()?.nativeElement.focus();
      return;
    }
    if (imageId === null) {
      return;
    }
    this.saving.set(true);
    try {
      const map = await this.api.create(this.campaignId(), this.nameControl.value.trim(), imageId);
      await this.router.navigate(['/campaigns', this.campaignId(), 'maps', map.id]);
    } catch (err) {
      this.saving.set(false);
      this.failure.set(
        describeConnectError(err, {
          [Code.InvalidArgument]:
            'Não deu para criar o mapa: o nome ou a imagem não valem mais. Confira os dois e tente de novo.',
          [Code.NotFound]: 'Essa campanha não existe, ou você não é membro dela.',
          [Code.PermissionDenied]: 'Só o mestre da campanha cria mapas.',
          [Code.ResourceExhausted]: 'A campanha chegou ao limite de 200 mapas.',
        }),
      );
    }
  }
}
