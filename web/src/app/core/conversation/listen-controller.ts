import { signal } from '@angular/core';

/** What the controller uses of an `HTMLAudioElement`, so a spec hands it a fake. */
export interface VoiceAudio {
  src: string;
  play(): Promise<void>;
  pause(): void;
  addEventListener(type: 'playing' | 'ended' | 'error', listener: () => void): void;
}

export type ListenState = 'idle' | 'loading' | 'playing' | 'error';

/** The words on the "Ouvir" button for each state. */
const LABELS: Record<ListenState, string> = {
  idle: 'Ouvir',
  loading: 'Gerando a voz…',
  playing: 'Parar',
  error: 'Ouvir',
};

export const LISTEN_ERROR = 'A voz não carregou. Tente de novo.';

/**
 * The voice of a narrator or NPC line ("Ouvir", MR-051): one plain audio
 * element at a time, created when a person taps and never before. It never
 * plays by itself and keeps nothing in the browser's storage; the server makes
 * the voice on the first request, which is why `loading` can take a while.
 */
export class ListenController {
  private current: { readonly id: string; readonly audio: VoiceAudio } | null = null;
  private readonly active = signal<{ id: string; state: ListenState } | null>(null);

  constructor(private readonly createAudio: () => VoiceAudio = () => new Audio()) {}

  stateOf(id: string): ListenState {
    const active = this.active();
    return active?.id === id ? active.state : 'idle';
  }

  labelOf(id: string): string {
    return LABELS[this.stateOf(id)];
  }

  /** The quiet error under the button; empty when there is none. */
  errorOf(id: string): string {
    return this.stateOf(id) === 'error' ? LISTEN_ERROR : '';
  }

  /** The button: starts the line (stopping any other), or stops it while it loads or plays. */
  toggle(id: string, audioUrl: string): void {
    const state = this.stateOf(id);
    this.stop();
    if (state === 'loading' || state === 'playing') {
      return;
    }
    const audio = this.createAudio();
    this.current = { id, audio };
    this.active.set({ id, state: 'loading' });
    // A stopped element may still say something; only the current one counts.
    const when = (next: ListenState) => () => {
      if (this.current?.audio === audio) {
        this.active.set({ id, state: next });
        this.current = next === 'playing' ? this.current : null;
      }
    };
    audio.addEventListener('playing', when('playing'));
    audio.addEventListener('ended', when('idle'));
    audio.addEventListener('error', when('error'));
    audio.src = audioUrl;
    // The browser may refuse to play (an interrupted load, a policy): the same quiet error.
    audio.play().catch(when('error'));
  }

  stop(): void {
    this.current?.audio.pause();
    this.current = null;
    this.active.set(null);
  }
}
