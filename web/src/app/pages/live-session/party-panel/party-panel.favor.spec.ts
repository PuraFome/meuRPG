import { ChangeDetectionStrategy, Component, input } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { afterEach, vi } from 'vitest';

import { FavorClient } from '../../../core/play/favor-client';
import { FavorState } from '../../../core/play/favor-state';
import { FakeFavorClient, openFavor } from '../../../core/play/favor-testing';
import { pensantusVitals } from '../../../core/play/live-testing';
import { SessionXp } from '../session-xp/session-xp';
import { PartyPanel } from './party-panel';

@Component({
  selector: 'app-session-xp',
  template: '',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
class SessionXpStub {
  readonly campaignId = input('');
  readonly campaignName = input('');
}

// "Grupo": the badge, the counter "1 com Inspiração" and "Dar Inspiração" in the row's menu (RN-43, board 581-02).

const flat = (n: Element | null | undefined) => n?.textContent?.replace(/\s+/g, ' ').trim() ?? '';
const pens = () => pensantusVitals({ characterId: 'pens-c', name: 'Pensantus' });
const tor = () => pensantusVitals({ characterId: 'tor-c', name: 'Tor' });

function mount(holders: Record<string, string> | null) {
  const client = new FakeFavorClient();
  TestBed.configureTestingModule({
    providers: [{ provide: FavorClient, useValue: client.asClient() }],
  }).overrideComponent(PartyPanel, {
    remove: { imports: [SessionXp] },
    add: { imports: [SessionXpStub] },
  });
  const favor = TestBed.inject(FavorState);
  if (holders) {
    openFavor(favor, holders);
  }
  const fixture = TestBed.createComponent(PartyPanel);
  fixture.componentRef.setInput('campaignId', 'camp');
  fixture.componentRef.setInput('party', [pens(), tor()]);
  fixture.detectChanges();
  return { fixture, favor, el: fixture.nativeElement as HTMLElement };
}

function menuOf(fixture: ReturnType<typeof mount>['fixture'], name: string): HTMLElement[] {
  (fixture.nativeElement as HTMLElement)
    .querySelector<HTMLButtonElement>(`button[aria-label="Mais ações de ${name}"]`)!
    .click();
  fixture.detectChanges();
  return Array.from(document.querySelectorAll<HTMLElement>('.mat-mdc-menu-item'));
}

describe("PartyPanel: the master's Inspiração", () => {
  // A menu left open in one test must not answer the next one.
  afterEach(() =>
    document.querySelectorAll('.cdk-overlay-container').forEach((c) => c.replaceChildren()),
  );

  it('shows the badge by the name of the holder and "1 com Inspiração" at the title', () => {
    const { el } = mount({ 'pens-c': 'Pensantus' });
    const badges = el.querySelectorAll('.js-favor-badge');
    expect(badges).toHaveLength(1);
    expect(flat(badges[0])).toBe('starInspiração');
    expect(badges[0].closest('li')?.textContent).toContain('Pensantus');
    expect(flat(el.querySelector('.js-favor-count'))).toBe('star1 com Inspiração');
  });

  it('counts two holders, and shows neither badge nor counter when nobody has it', () => {
    expect(
      flat(mount({ 'pens-c': 'Pensantus', 'tor-c': 'Tor' }).el.querySelector('.js-favor-count')),
    ).toBe('star2 com Inspiração');
    TestBed.resetTestingModule();
    const { el } = mount({});
    expect(el.querySelector('.js-favor-badge')).toBeNull();
    expect(el.querySelector('.js-favor-count')).toBeNull();
  });

  it('gives it from the first item of the row menu', () => {
    const { fixture, favor } = mount({});
    const grant = vi.spyOn(favor, 'grant').mockResolvedValue();
    const items = menuOf(fixture, 'Tor');
    expect(flat(items[0])).toBe('star_border Dar Inspiração');
    expect(flat(items[1])).toContain('Chamar a atenção');
    items[0].click();
    expect(grant).toHaveBeenCalledWith('tor-c', 'Tor');
  });

  it('draws the item dashed with the reason for a holder, and sends nothing', () => {
    const { fixture, favor } = mount({ 'pens-c': 'Pensantus' });
    const grant = vi.spyOn(favor, 'grant').mockResolvedValue();
    const items = menuOf(fixture, 'Pensantus');
    expect(flat(items[0])).toBe('star_border Dar Inspiração Pensantus já tem Inspiração.');
    expect(items[0].getAttribute('aria-disabled')).toBe('true');
    items[0].click();
    expect(grant).not.toHaveBeenCalled();
  });

  it('offers no item outside an open session', () => {
    const { fixture } = mount(null);
    const items = menuOf(fixture, 'Tor');
    expect(items.some((i) => flat(i).includes('Dar Inspiração'))).toBe(false);
  });

  it('says what the grant answered on the row of that character only', () => {
    const { fixture, favor, el } = mount({});
    favor.note.set({
      characterId: 'tor-c',
      text: 'Essa personagem já tem Inspiração.',
      refused: true,
    });
    fixture.detectChanges();
    const notes = el.querySelectorAll('.js-favor-note');
    expect(notes).toHaveLength(1);
    expect(notes[0].closest('li')?.textContent).toContain('Tor');
  });
});
