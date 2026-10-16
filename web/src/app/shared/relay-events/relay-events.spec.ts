import { ChangeDetectionStrategy, Component } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { describe, expect, it } from 'vitest';

import { RelayEvents } from './relay-events';

@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [RelayEvents],
  template: `<div
    appRelayEvents
    (relayClick)="log.push('click')"
    (relayKeydown)="log.push('down:' + $event.key)"
    (relayKeyup)="log.push('up:' + $event.key)"
  >
    <button type="button">inner</button>
  </div>`,
})
class Host {
  readonly log: string[] = [];
}

describe('RelayEvents', () => {
  it('relays the click and the keys that bubble up from a control inside', () => {
    const fixture = TestBed.createComponent(Host);
    fixture.detectChanges();
    const button = fixture.nativeElement.querySelector('button') as HTMLButtonElement;
    button.dispatchEvent(new KeyboardEvent('keydown', { key: 'ArrowRight', bubbles: true }));
    button.dispatchEvent(new KeyboardEvent('keyup', { key: 'ArrowRight', bubbles: true }));
    button.click();
    expect(fixture.componentInstance.log).toEqual(['down:ArrowRight', 'up:ArrowRight', 'click']);
  });
});
