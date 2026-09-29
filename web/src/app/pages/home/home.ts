import { Component, inject, signal } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatCardModule } from '@angular/material/card';
import { MatProgressSpinnerModule } from '@angular/material/progress-spinner';
import { ConnectError } from '@connectrpc/connect';

import { ServerInfoService } from '../../core/system/server-info.service';

/** State of the one call this page makes, kept as a single signal so the
 * template only ever renders one of the three cases (no impossible states). */
type ServerInfoState =
  | { status: 'loading' }
  | { status: 'ready'; version: string; commit: string }
  | { status: 'error'; message: string };

@Component({
  selector: 'app-home',
  imports: [MatButtonModule, MatCardModule, MatProgressSpinnerModule],
  templateUrl: './home.html',
  styleUrl: './home.scss',
})
export class Home {
  private readonly serverInfo = inject(ServerInfoService);

  protected readonly state = signal<ServerInfoState>({ status: 'loading' });

  constructor() {
    this.load();
  }

  protected retry(): void {
    this.state.set({ status: 'loading' });
    this.load();
  }

  private load(): void {
    // A single two-argument `.then` (rather than `.then().catch()`) settles
    // in one microtask either way, which keeps success and failure equally
    // fast and easy to await in tests.
    this.serverInfo.getServerInfo().then(
      (res) => this.state.set({ status: 'ready', version: res.version, commit: res.commit }),
      (err: unknown) => {
        // ConnectError.from() also handles plain network failures (the
        // fetch failing before it ever reaches the server), not just RPCs
        // that came back with an error.
        this.state.set({ status: 'error', message: ConnectError.from(err).message });
      },
    );
  }
}
