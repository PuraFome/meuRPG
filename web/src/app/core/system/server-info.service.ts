import { Injectable, inject } from '@angular/core';
import { createClient } from '@connectrpc/connect';

import { SystemService } from '../../../gen/meurpg/system/v1/system_pb';
import { CONNECT_TRANSPORT } from '../connect/transport';

/**
 * Thin wrapper around the generated SystemService Connect client. Other
 * modules should not call `createClient` directly, so every RPC-backed
 * feature can grow this same one-service-per-module shape.
 */
@Injectable({ providedIn: 'root' })
export class ServerInfoService {
  private readonly client = createClient(SystemService, inject(CONNECT_TRANSPORT));

  /** Returns the version and commit of the running backend build. */
  getServerInfo() {
    return this.client.getServerInfo({});
  }
}
