import { DOCUMENT } from '@angular/common';
import {
  ChangeDetectionStrategy,
  Component,
  DestroyRef,
  ElementRef,
  InjectionToken,
  effect,
  inject,
  input,
  output,
  untracked,
  viewChild,
} from '@angular/core';

import type { TrayRequest } from './roll-animator';
import { DiceSoundSetting } from './dice-sound-setting';
import type { Tray, TrayOptions } from './tray/dice-tray';

/** The widest the tray gets, in px (a tablet and up), and its height at most; on a phone it spans the overlay. */
const MAX_WIDTH = 520;
const MAX_HEIGHT = 280;
/** The tray's height against its width on a phone. */
const HEIGHT_RATIO = 0.55;
/** A width to start from when the host has no layout yet. */
const FALLBACK_WIDTH = 320;
const FONTS = ['800 24px Alegreya', '700 13px "Alegreya Sans"'];

/** How the tray module is loaded: with the first roll, never with the app. A test hands in a fake. */
export const TRAY_LOADER = new InjectionToken<
  () => Promise<{ createTray: (canvas: HTMLCanvasElement, options?: TrayOptions) => Tray }>
>('TRAY_LOADER', { providedIn: 'root', factory: () => () => import('./tray/dice-tray') });

/** The tray never takes more than this share of the window's height, so the words and the buttons fit under it. */
const MAX_SHARE_OF_WINDOW = 0.5;

/** The tray's size for the width the overlay gives it, in tray pixels (and the window's height, for a phone on its side). */
export function traySize(
  available: number,
  windowHeight = Infinity,
): { width: number; height: number } {
  const width = Math.max(1, Math.min(MAX_WIDTH, Math.floor(available) || FALLBACK_WIDTH));
  const height = Math.min(
    MAX_HEIGHT,
    Math.round(width * HEIGHT_RATIO),
    Math.floor(windowHeight * MAX_SHARE_OF_WINDOW),
  );
  return { width, height: Math.max(1, height) };
}

/**
 * The dice tray of the roll overlay: one `<canvas>` on which `tray/dice-tray` throws the dice (a plain TypeScript
 * module, loaded with the first roll so the initial bundle does not carry it). The component only starts and stops
 * it: the animation loop draws on the canvas and never writes a signal. It tells its parent once, when the dice have
 * landed. A new request replaces the roll in flight. The picture is for the eyes: the sentence a screen reader hears
 * is the overlay's live region.
 */
@Component({
  selector: 'app-roll-tray',
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `<canvas #canvas class="tray" aria-hidden="true"></canvas>`,
  styles: `
    :host {
      display: block;
      width: min(100%, 520px);
    }
    .tray {
      display: block;
      width: 100%;
      height: auto;
    }
  `,
})
export class RollTray {
  readonly request = input.required<TrayRequest>();
  /** Nothing moves: the dice appear at rest with their results. */
  readonly reduced = input(false);
  /** The dice have landed (once per roll). */
  readonly landed = output();

  private readonly canvas = viewChild.required<ElementRef<HTMLCanvasElement>>('canvas');
  private readonly host = inject<ElementRef<HTMLElement>>(ElementRef).nativeElement;
  private readonly doc = inject(DOCUMENT);
  private readonly soundSetting = inject(DiceSoundSetting);
  private readonly load = inject(TRAY_LOADER);
  private tray: Tray | null = null;
  private loading: Promise<Tray | null> | null = null;
  private gone = false;
  /** The roll the screen last asked for, to ignore a late answer of an older one. */
  private latest = 0;
  /** A tap asked for the end of the roll before the tray was there. */
  private skipped = false;
  private stopWatching: (() => void) | null = null;

  constructor() {
    inject(DestroyRef).onDestroy(() => {
      this.gone = true;
      this.stopWatching?.();
      this.tray?.destroy();
      this.tray = null;
    });
    effect(() => {
      const request = this.request();
      const reduced = this.reduced();
      untracked(() => void this.start(request, reduced));
    });
  }

  /** Lands the dice at once (a tap or Esc while they roll). */
  finish(): void {
    this.skipped = true;
    this.tray?.finish();
  }

  /**
   * Switches the sound on or off. Call it from the tap itself: browsers start audio only after a gesture, and the
   * tray starts the audio context in this call.
   */
  setSound(on: boolean): void {
    this.tray?.setSound(on);
  }

  private async start(request: TrayRequest, reduced: boolean): Promise<void> {
    this.latest = request.id;
    this.skipped = false;
    const tray = await this.ensureTray();
    if (this.gone || request.id !== this.latest) {
      return;
    }
    if (!tray) {
      this.landed.emit(); // no canvas: the words still show, the roll is never held up
      return;
    }
    tray.setReducedMotion(reduced);
    this.fitTo(tray);
    const pending = tray.roll(request.dice);
    if (this.skipped) {
      tray.finish();
    }
    const out = await pending.catch(() => null);
    if (!this.gone && request.id === this.latest && !out?.cancelled) {
      this.landed.emit();
    }
  }

  private ensureTray(): Promise<Tray | null> {
    this.loading ??= this.create();
    return this.loading;
  }

  /** Loads the module, makes the tray and keeps it fitted, in the right colours and fonts. */
  private async create(): Promise<Tray | null> {
    try {
      const { createTray } = await this.load();
      if (this.gone) {
        return null;
      }
      const size = this.sizeNow();
      const tray = createTray(this.canvas().nativeElement, {
        ...size,
        reducedMotion: this.reduced(),
        sound: this.soundSetting.on(),
      });
      this.tray = tray;
      this.watch(tray);
      void this.whenFontsLoad(tray);
      return tray;
    } catch {
      return null;
    }
  }

  private sizeNow(): { width: number; height: number } {
    return traySize(this.host.getBoundingClientRect().width, this.doc.defaultView?.innerHeight);
  }

  private fitTo(tray: Tray): void {
    const size = this.sizeNow();
    if (size.width !== tray.size.width || size.height !== tray.size.height) {
      tray.resize(size.width, size.height);
    }
  }

  /** Follows the overlay's width and the colour scheme. */
  private watch(tray: Tray): void {
    const win = this.doc.defaultView;
    const observer =
      typeof ResizeObserver === 'function' ? new ResizeObserver(() => this.fitTo(tray)) : null;
    observer?.observe(this.host);
    const scheme =
      win && typeof win.matchMedia === 'function'
        ? win.matchMedia('(prefers-color-scheme: dark)')
        : null;
    const onScheme = (): void => tray.refreshTheme();
    scheme?.addEventListener?.('change', onScheme);
    this.stopWatching = () => {
      observer?.disconnect();
      scheme?.removeEventListener?.('change', onScheme);
    };
  }

  /** The canvas draws with the app's own Alegreya: it paints again once they are there. */
  private async whenFontsLoad(tray: Tray): Promise<void> {
    const fonts = this.doc.fonts as FontFaceSet | undefined;
    if (!fonts || typeof fonts.load !== 'function') {
      return;
    }
    try {
      await Promise.all(FONTS.map((f) => fonts.load(f)));
      if (!this.gone) {
        tray.redraw();
      }
    } catch {
      // The fallback fonts stay: the numbers still read.
    }
  }
}
