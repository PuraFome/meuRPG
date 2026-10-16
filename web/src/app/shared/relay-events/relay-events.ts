import { Directive, HostListener, output } from '@angular/core';

/**
 * Relays the click and key events that bubble up to a container that is not a control itself:
 * a radiogroup whose buttons hold the focus and share one arrow-key handler, a table of
 * inputs, a dialog whose backdrop closes it. The controls inside are the keyboard targets
 * (each one is a button, an input, or a roving-tabindex radio); the container only listens.
 * Declaring the listener here keeps `interactive-supports-focus` for what it is meant to
 * catch: an element that is itself clickable but unreachable by keyboard.
 */
@Directive({ selector: '[appRelayEvents]' })
export class RelayEvents {
  readonly relayClick = output<MouseEvent>();
  readonly relayKeydown = output<KeyboardEvent>();
  readonly relayKeyup = output<KeyboardEvent>();

  @HostListener('click', ['$event'])
  protected onClick(event: MouseEvent): void {
    this.relayClick.emit(event);
  }

  @HostListener('keydown', ['$event'])
  protected onKeydown(event: KeyboardEvent): void {
    this.relayKeydown.emit(event);
  }

  @HostListener('keyup', ['$event'])
  protected onKeyup(event: KeyboardEvent): void {
    this.relayKeyup.emit(event);
  }
}
