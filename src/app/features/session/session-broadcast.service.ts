import { Injectable } from '@angular/core';

export interface SessionBroadcast {
  sessionId: string;
  campaignId: string | null;
  activeMapId: string | null;
  revision: number;
}

const CHANNEL_NAME = 'meurpg-session-live';
const STORAGE_KEY = 'meurpg_session_live';

/**
 * Sincroniza o estado da sessão entre a aba do mestre e a aba de apresentação
 * no mesmo navegador. Usa BroadcastChannel com fallback para o evento `storage`
 * (que também cobre navegadores sem BroadcastChannel).
 */
@Injectable({ providedIn: 'root' })
export class SessionBroadcastService {
  private channel: BroadcastChannel | null = null;

  private getChannel(): BroadcastChannel | null {
    if (typeof BroadcastChannel === 'undefined') return null;
    if (!this.channel) this.channel = new BroadcastChannel(CHANNEL_NAME);
    return this.channel;
  }

  /** Publica o estado atual para as outras abas. */
  publish(message: SessionBroadcast): void {
    try {
      localStorage.setItem(STORAGE_KEY, JSON.stringify(message));
    } catch {
      // localStorage indisponível: o BroadcastChannel abaixo cobre o caso.
    }
    this.getChannel()?.postMessage(message);
  }

  /** Escuta mudanças publicadas por outras abas. Retorna uma função de limpeza. */
  subscribe(handler: (message: SessionBroadcast) => void): () => void {
    const channel = this.getChannel();
    const onMessage = (event: MessageEvent) => handler(event.data as SessionBroadcast);
    const onStorage = (event: StorageEvent) => {
      if (event.key !== STORAGE_KEY || !event.newValue) return;
      try {
        handler(JSON.parse(event.newValue) as SessionBroadcast);
      } catch {
        // entrada corrompida: ignora
      }
    };

    channel?.addEventListener('message', onMessage);
    window.addEventListener('storage', onStorage);
    return () => {
      channel?.removeEventListener('message', onMessage);
      window.removeEventListener('storage', onStorage);
    };
  }

  /** Último estado publicado, usado como carga inicial pela aba de apresentação. */
  lastPublished(): SessionBroadcast | null {
    try {
      const raw = localStorage.getItem(STORAGE_KEY);
      return raw ? (JSON.parse(raw) as SessionBroadcast) : null;
    } catch {
      return null;
    }
  }
}
