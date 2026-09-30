import { Component, inject, signal } from '@angular/core';
import { FormBuilder, ReactiveFormsModule, Validators } from '@angular/forms';
import { MatButtonModule } from '@angular/material/button';
import { MatCardModule } from '@angular/material/card';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatInputModule } from '@angular/material/input';
import { MatListModule } from '@angular/material/list';
import { MatProgressSpinnerModule } from '@angular/material/progress-spinner';
import { MatSelectModule } from '@angular/material/select';
import { Router, RouterLink } from '@angular/router';
import { Code } from '@connectrpc/connect';

import { Campaign, XpMode } from '../../../gen/meurpg/campaigns/v1/campaigns_pb';
import { roleLabel } from '../../core/campaigns/campaign-labels';
import { CampaignsService } from '../../core/campaigns/campaigns.service';
import { describeConnectError } from '../../core/connect/connect-errors';

type ListState =
  | { status: 'loading' }
  | { status: 'ready'; campaigns: Campaign[] }
  | { status: 'error'; message: string };

type CreateState = { status: 'idle' } | { status: 'saving' } | { status: 'error'; message: string };

/**
 * "Minhas campanhas" (guarded by authGuard — see app.routes.ts): MR-001's
 * list, with an empty state and the "Nova campanha" form.
 */
@Component({
  selector: 'app-campaigns',
  imports: [
    MatButtonModule,
    MatCardModule,
    MatFormFieldModule,
    MatInputModule,
    MatListModule,
    MatProgressSpinnerModule,
    MatSelectModule,
    ReactiveFormsModule,
    RouterLink,
  ],
  templateUrl: './campaigns.html',
  styleUrl: './campaigns.scss',
})
export class Campaigns {
  private readonly campaigns = inject(CampaignsService);
  private readonly router = inject(Router);
  private readonly fb = inject(FormBuilder);

  protected readonly XpMode = XpMode;
  protected readonly roleLabel = roleLabel;

  protected readonly state = signal<ListState>({ status: 'loading' });
  protected readonly createState = signal<CreateState>({ status: 'idle' });

  protected readonly form = this.fb.nonNullable.group({
    name: ['', [Validators.required, Validators.maxLength(80)]],
    xpMode: this.fb.control<XpMode | null>(null, Validators.required),
  });

  constructor() {
    this.load();
  }

  private load(): void {
    this.campaigns.listMyCampaigns().then(
      (res) => this.state.set({ status: 'ready', campaigns: res.campaigns }),
      (err: unknown) => {
        this.state.set({
          status: 'error',
          message: describeConnectError(err, {}),
        });
      },
    );
  }

  protected async submit(): Promise<void> {
    if (this.form.invalid) {
      this.form.markAllAsTouched();
      return;
    }
    const { name, xpMode } = this.form.getRawValue();
    this.createState.set({ status: 'saving' });
    try {
      const res = await this.campaigns.createCampaign(name.trim(), xpMode!);
      this.createState.set({ status: 'idle' });
      const id = res.campaign?.id;
      if (id) {
        await this.router.navigate(['/campanhas', id]);
      }
    } catch (err) {
      this.createState.set({
        status: 'error',
        message: describeConnectError(err, {
          [Code.InvalidArgument]: 'O nome da campanha precisa ter de 1 a 80 caracteres.',
        }),
      });
    }
  }
}
