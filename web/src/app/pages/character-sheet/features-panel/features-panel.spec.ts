import { TestBed } from '@angular/core/testing';

import { GearVm } from '../../../core/characters/gear';
import { FullSheetVm, ItemModifierVm } from '../character-sheet.types';
import { FeaturesPanel } from './features-panel';

const SHIELD_HINT = {
  sourceKey: 'equipment:shield',
  text: 'O escudo não conta na CA enquanto uma arma de duas mãos está equipada: guarde a arma de duas mãos.',
};
const OTHER_HINT = { sourceKey: 'feature:unarmored-defense', text: 'Sem armadura.' };

const shield = (bonus?: number): GearVm =>
  ({ key: 'shield', namePt: 'Escudo', kind: 'shield', bonus }) as GearVm;

const shieldMod = (reasonPt: string): ItemModifierVm => ({
  target: 'ac',
  value: 1,
  sourceKey: 'equipment:shield',
  sourceNamePt: 'Escudo',
  counts: false,
  reasonPt,
});

function reminders(over: Partial<FullSheetVm>): string[] {
  const fixture = TestBed.createComponent(FeaturesPanel);
  fixture.componentRef.setInput('sheet', {
    kind: 'full',
    features: [],
    hints: [SHIELD_HINT, OTHER_HINT],
    gear: [],
    itemModifiers: [],
    shieldIdle: false,
    resistances: [],
    languages: [],
    proficiencies: [],
    ...over,
  } as unknown as FullSheetVm);
  fixture.detectChanges();
  return Array.from((fixture.nativeElement as HTMLElement).querySelectorAll('.hints li')).map(
    (li) => li.textContent!.trim(),
  );
}

describe('FeaturesPanel "Lembretes"', () => {
  it('lists every hint while the shield is not idle', () => {
    expect(reminders({})).toEqual([SHIELD_HINT.text, OTHER_HINT.text]);
  });

  it("leaves the idle shield's hint out when the combat column's note says it", () => {
    expect(reminders({ shieldIdle: true, gear: [shield()] })).toEqual([OTHER_HINT.text]);
  });

  it("leaves it out when the shield's own line says why", () => {
    expect(
      reminders({
        shieldIdle: true,
        gear: [shield(1)],
        itemModifiers: [shieldMod('Uma arma de duas mãos equipada deixa o escudo parado.')],
      }),
    ).toEqual([OTHER_HINT.text]);
  });

  it('keeps it when the combat column says no reason for the shield', () => {
    expect(
      reminders({ shieldIdle: true, gear: [shield(1)], itemModifiers: [shieldMod('')] }),
    ).toEqual([SHIELD_HINT.text, OTHER_HINT.text]);
  });
});

describe('FeaturesPanel, a feat or option that does not apply now (board 580-05)', () => {
  it('marks the lost row with a "Não vale agora" tag, and only that row', () => {
    const fixture = TestBed.createComponent(FeaturesPanel);
    const row = (name: string, lost: boolean) => ({
      name,
      sourcePt: 'Opção de Truques de lâmina',
      description: '',
      descriptionPt: '',
      descriptionPtMissing: false,
      descriptionPtOnly: true,
      summaryPt: '',
      lost,
    });
    fixture.componentRef.setInput('sheet', {
      kind: 'full',
      features: [row('Golpe do Corvo', false), row('Asa Partida', true)],
      hints: [],
      gear: [],
      itemModifiers: [],
      shieldIdle: false,
      resistances: [],
      languages: [],
      proficiencies: [],
    } as unknown as FullSheetVm);
    fixture.detectChanges();
    const rows = Array.from(
      (fixture.nativeElement as HTMLElement).querySelectorAll<HTMLElement>('.feature'),
    );
    expect(rows[0].querySelector('.feature__lost')).toBeNull();
    expect(rows[1].querySelector('.feature__lost')?.textContent?.trim()).toBe('Não vale agora');
  });
});
