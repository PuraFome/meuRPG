import { create } from '@bufbuild/protobuf';
import { TestBed } from '@angular/core/testing';
import { MAT_DIALOG_DATA, MatDialogRef } from '@angular/material/dialog';

import { OpportunityOfferSchema } from '../../../../../gen/meurpg/play/v1/combat_pb';
import { CombatClient } from '../../../../core/combat/combat-client';
import { combatant, encounter } from '../../../../core/combat/combat-testing';
import { CombatState } from '../../../../core/combat/combat-state';
import { OpportunitySheet, type OpportunitySheetData } from './opportunity-sheet';

const plain = (t: string | null | undefined) => (t ?? '').replace(/ /g, ' ').replace(/\s+/g, ' ').trim();

// Goblin 2 leaves Toren's reach (E9-13, frames C and G).
const offer = create(OpportunityOfferSchema, {
  id: 'o1',
  moverId: 'g2',
  moverLabel: 'Goblin 2',
  reactorId: 'toren',
  reactorLabel: 'Toren',
  forYou: true,
});

function setup(opts: { present?: boolean } = {}) {
  const state = new CombatState();
  state.apply(encounter({ combatants: [combatant({ id: 'g2', label: 'Goblin 2' })], opportunityOffers: opts.present === false ? [] : [offer] }));
  const declined: string[] = [];
  const closed: unknown[] = [];
  const data: OpportunitySheetData = {
    campaignId: 'c',
    encounterId: 'enc',
    offer,
    round: 2,
    attacks: [{ key: 'attack:longsword', name: 'Espada longa', detail: 'Espada longa +5 · 1d8 + 3 cortante', attack: null }],
    state,
  };
  TestBed.configureTestingModule({
    providers: [
      { provide: MAT_DIALOG_DATA, useValue: data },
      { provide: MatDialogRef, useValue: { close: (r: unknown) => closed.push(r) } },
      {
        provide: CombatClient,
        useValue: {
          declineOpportunity: async (_c: string, _e: string, id: string) => {
            declined.push(id);
            return encounter({ opportunityOffers: [] });
          },
        },
      },
    ],
  });
  const fixture = TestBed.createComponent(OpportunitySheet);
  fixture.detectChanges();
  return { fixture, el: fixture.nativeElement as HTMLElement, declined, closed };
}

describe('OpportunitySheet', () => {
  it('asks the player, names the weapon and the cost, and never shows an armor class', () => {
    const { el } = setup();
    const text = plain(el.textContent);
    expect(text).toContain('Ataque de oportunidade');
    expect(text).toContain('Goblin 2 · Rodada 2');
    expect(text).toContain('O Goblin 2 está saindo do seu alcance. Ataque de oportunidade?');
    expect(text).toContain('Gasta a sua reação. Espada longa +5 · 1d8 + 3 cortante.');
    expect(text).not.toMatch(/CA \d/);
  });

  it('has the safe "Não atacar" first, marked for the initial focus, and a close-less frame', () => {
    const { el } = setup();
    const buttons = Array.from(el.querySelectorAll<HTMLButtonElement>('.btn'));
    expect(buttons.map((b) => plain(b.textContent))).toEqual(['Não atacar', 'Atacar com Espada longa']);
    expect(buttons[0].hasAttribute('data-initial-focus')).toBe(true);
    expect(el.querySelector('[aria-label="Fechar"]')).toBeNull();
  });

  it('turns the offer down, and hands the attack over on "Atacar"', async () => {
    const { fixture, el, declined, closed } = setup();
    const [no, yes] = Array.from(el.querySelectorAll<HTMLButtonElement>('.btn'));
    yes.click();
    expect(closed).toEqual([{ attackKey: 'attack:longsword' }]);
    no.click();
    await fixture.whenStable();
    expect(declined).toEqual(['o1']);
    expect(closed[1]).toBeNull();
  });

  it('says so when the master answered first, and only closes', () => {
    const { el } = setup({ present: false });
    expect(plain(el.textContent)).toContain('O mestre respondeu por você');
    expect(Array.from(el.querySelectorAll('.btn'), (b) => plain(b.textContent))).toEqual(['Fechar']);
  });
});
