import { TestBed } from '@angular/core/testing';

import { DiceMode, DicePreference } from '../../../../../gen/meurpg/campaigns/v1/campaigns_pb';
import { create } from '@bufbuild/protobuf';
import { RollModeKind, RollNoteSchema } from '../../../../../gen/meurpg/play/v1/contest_types_pb';
import type { CheckDie } from '../../../../core/combat/contest-client';
import { textOf } from '../../../../core/combat/contest-testing';
import { CheckRollForm } from './check-roll-form';

function form(inputs: Record<string, unknown> = {}) {
  const fixture = TestBed.createComponent(CheckRollForm);
  const ref = fixture.componentRef;
  ref.setInput('checkName', 'Força (Atletismo)');
  ref.setInput('modifier', 5);
  ref.setInput('diceMode', DiceMode.PLAYERS_CHOOSE);
  ref.setInput('preference', DicePreference.APP);
  for (const [k, v] of Object.entries(inputs)) {
    ref.setInput(k, v);
  }
  fixture.detectChanges();
  const rolls: CheckDie[] = [];
  let deferred = 0;
  fixture.componentInstance.roll.subscribe((d) => rolls.push(d));
  fixture.componentInstance.defer.subscribe(() => deferred++);
  const el = fixture.nativeElement as HTMLElement;
  const button = (name: string) =>
    Array.from(el.querySelectorAll<HTMLButtonElement>('button')).find((b) =>
      textOf(b).includes(name),
    );
  const type = async (values: string[]) => {
    fixture.detectChanges();
    await fixture.whenStable();
    fixture.detectChanges();
    const fields = Array.from(el.querySelectorAll<HTMLInputElement>('input[type="text"]'));
    values.forEach((v, i) => {
      fields[i].value = v;
      fields[i].dispatchEvent(new Event('input'));
    });
    fixture.detectChanges();
    await fixture.whenStable();
  };
  return { fixture, el, button, rolls, type, deferred: () => deferred };
}

describe('CheckRollForm', () => {
  it('rolls in the app', () => {
    const { button, rolls } = form();
    button('Rolar no app')!.click();
    expect(rolls).toEqual([{ inApp: true }]);
  });

  it('sends the typed d20 as one face', async () => {
    const { button, rolls, type, el } = form();
    button('Digitar o resultado')!.click();
    await type(['14']);
    expect(textOf(el.querySelector('label')!)).toBe('Role 1d20 para Força (Atletismo) (+5)');
    button('Confirmar')!.click();
    expect(rolls).toEqual([{ faces: [14] }]);
  });

  it('asks for the pair with advantage or disadvantage, and says which one counts', async () => {
    const dis = form({ mode: RollModeKind.DISADVANTAGE });
    dis.button('Rolar 2d20 no app')!.click();
    expect(dis.rolls).toEqual([{ inApp: true }]);
    dis.button('Digitar o resultado')!.click();
    await dis.type(['15', '8']);
    expect(textOf(dis.el)).toContain('Conta o menor.');
    dis.button('Confirmar')!.click();
    expect(dis.rolls[1]).toEqual({ faces: [15, 8] });
    TestBed.resetTestingModule();
    const adv = form({ mode: RollModeKind.ADVANTAGE });
    adv.button('Digitar o resultado')!.click();
    await adv.type(['15', '8']);
    expect(textOf(adv.el)).toContain('Conta o maior.');
  });

  it('offers the master’s roll only when the sheet names it, and not while typing', async () => {
    const none = form();
    expect(none.button('Deixar o mestre rolar por mim')).toBeUndefined();
    TestBed.resetTestingModule();
    const some = form({ deferLabel: 'Deixar o mestre rolar por mim' });
    some.button('Deixar o mestre rolar por mim')!.click();
    expect(some.deferred()).toBe(1);
    some.button('Digitar o resultado')!.click();
    await some.fixture.whenStable();
    some.fixture.detectChanges();
    expect(some.button('Deixar o mestre rolar por mim')).toBeUndefined();
  });

  it('follows the campaign’s dice mode: physical dice only, or the app only', () => {
    const physical = form({ diceMode: DiceMode.PHYSICAL });
    expect(physical.button('Rolar no app')).toBeUndefined();
    expect(physical.el.querySelector('input[type="text"]')).toBeTruthy();
    TestBed.resetTestingModule();
    const app = form({ diceMode: DiceMode.APP });
    expect(app.button('Digitar o resultado')).toBeUndefined();
  });

  it('says the mode and why above the buttons, one sentence each, and nothing for a normal roll', () => {
    const note = (labelPt: string, advantage: boolean) =>
      create(RollNoteSchema, { kind: 'x', labelPt, advantage });
    const { el } = form({
      mode: RollModeKind.ADVANTAGE,
      notes: [note('Ajuda de Orla', true)],
    });
    expect(Array.from(el.querySelectorAll('.why li'), (li) => textOf(li))).toEqual([
      'Vantagem',
      'Vantagem: Ajuda de Orla',
    ]);
    TestBed.resetTestingModule();
    expect(form().el.querySelector('.why')).toBeNull();
  });
});
