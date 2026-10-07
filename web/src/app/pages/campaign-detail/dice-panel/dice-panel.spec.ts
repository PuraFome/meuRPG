import { TestBed } from '@angular/core/testing';
import { provideRouter } from '@angular/router';

import { DiceMode, DicePreference } from '../../../../gen/meurpg/campaigns/v1/campaigns_pb';
import { CampaignsService } from '../../../core/campaigns/campaigns.service';
import type { MemberRowVm } from '../campaign-detail.copy';
import { DicePanel } from './dice-panel';

function row(name: string, dicePreference: DicePreference, role = 'jogador'): MemberRowVm {
  return { userId: name, name, role, isViewer: false, hasName: true, dicePreference };
}

async function render(
  mode: DiceMode,
  setDiceMode = (_id: string, m: DiceMode) => Promise.resolve({ mode: m }),
) {
  TestBed.configureTestingModule({
    providers: [provideRouter([]), { provide: CampaignsService, useValue: { setDiceMode } }],
  });
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

describe('DicePanel', () => {
  it('lists the players choices (not the master) while players choose', async () => {
    const el: HTMLElement = (await render(DiceMode.PLAYERS_CHOOSE)).nativeElement;
    const rows = el.querySelectorAll('li');
    expect(rows.length).toBe(2);
    expect(rows[0].textContent).toContain('Vinicius');
    expect(rows[0].textContent).toContain('No app');
    expect(rows[1].textContent).toContain('Caio');
    expect(rows[1].textContent).toContain('Meus próprios dados');
  });

  it('hides the list when the mode is forced', async () => {
    const el: HTMLElement = (await render(DiceMode.APP)).nativeElement;
    expect(el.querySelector('li')).toBeNull();
  });

  it('shows the mode in force and edits nothing: the link goes to "Regras da mesa"', async () => {
    const fixture = await render(DiceMode.PHYSICAL);
    const el: HTMLElement = fixture.nativeElement;
    expect(el.textContent).toContain('Todos rolam os próprios dados');
    expect(el.querySelector('input[type=radio]')).toBeNull();
    expect(el.querySelector('button')).toBeNull();
    expect(el.querySelector('a[href="/campaigns/camp-1/rules"]')?.textContent).toContain(
      'Mudar em Regras da mesa',
    );
  });
});
