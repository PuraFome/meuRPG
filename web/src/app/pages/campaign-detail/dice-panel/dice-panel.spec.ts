import { TestBed } from '@angular/core/testing';

import { DiceMode, DicePreference } from '../../../../gen/meurpg/campaigns/v1/campaigns_pb';
import { CampaignsService } from '../../../core/campaigns/campaigns.service';
import type { MemberRowVm } from '../campaign-detail.copy';
import { DicePanel } from './dice-panel';

function row(name: string, dicePreference: DicePreference, role = 'jogador'): MemberRowVm {
  return { userId: name, name, role, isViewer: false, hasName: true, dicePreference };
}

async function render(mode: DiceMode, setDiceMode = (_id: string, m: DiceMode) => Promise.resolve({ mode: m })) {
  TestBed.configureTestingModule({ providers: [{ provide: CampaignsService, useValue: { setDiceMode } }] });
  const fixture = TestBed.createComponent(DicePanel);
  fixture.componentRef.setInput('campaignId', 'camp-1');
  fixture.componentRef.setInput('savedMode', mode);
  fixture.componentRef.setInput('members', [
    row('Samuel', DicePreference.APP, 'mestre'),
    row('Vinicius', DicePreference.APP),
    row('Caio', DicePreference.PHYSICAL),
  ]);
  fixture.detectChanges();
  return fixture;
}

function click(el: HTMLElement, selector: string): void {
  (el.querySelector(selector) as HTMLElement).click();
}

describe('DicePanel', () => {
  it('lists the players choices (not the master) while players choose', async () => {
    const el: HTMLElement = (await render(DiceMode.PLAYERS_CHOOSE)).nativeElement;
    const rows = el.querySelectorAll('li');
    expect(rows.length).toBe(2);
    expect(rows[0].textContent).toContain('Vinicius');
    expect(rows[0].textContent).toContain('No app');
    expect(rows[1].textContent).toContain('Caio');
    expect(rows[1].textContent).toContain('Meus próprios dados');
    expect(el.textContent).toContain('A mudança vale a partir da próxima rolagem.');
  });

  it('hides the list when the mode is forced', async () => {
    const el: HTMLElement = (await render(DiceMode.APP)).nativeElement;
    expect(el.querySelector('li')).toBeNull();
  });

  it('saves only a changed mode and then shows the confirmed one', async () => {
    const calls: DiceMode[] = [];
    const fixture = await render(DiceMode.PLAYERS_CHOOSE, (_id, m) => {
      calls.push(m);
      return Promise.resolve({ mode: m });
    });
    const el: HTMLElement = fixture.nativeElement;
    const save = el.querySelector('button') as HTMLButtonElement;
    expect(save.disabled).toBe(true);

    const radios = el.querySelectorAll<HTMLInputElement>('input[type=radio]');
    radios[1].click();
    fixture.detectChanges();
    expect(save.disabled).toBe(false);
    expect(el.querySelector('li')).toBeNull();

    click(el, 'button');
    await fixture.whenStable();
    fixture.detectChanges();
    expect(calls).toEqual([DiceMode.APP]);
    expect(el.textContent).toContain('Salvo.');
    expect(save.disabled).toBe(true);
  });

  it('says so when the save fails', async () => {
    const fixture = await render(DiceMode.PLAYERS_CHOOSE, () => Promise.reject(new Error('boom')));
    const el: HTMLElement = fixture.nativeElement;
    el.querySelectorAll<HTMLInputElement>('input[type=radio]')[2].click();
    fixture.detectChanges();
    click(el, 'button');
    await fixture.whenStable();
    fixture.detectChanges();
    expect(el.querySelector('[role=alert]')?.textContent).toContain('Não foi possível salvar');
  });
});
