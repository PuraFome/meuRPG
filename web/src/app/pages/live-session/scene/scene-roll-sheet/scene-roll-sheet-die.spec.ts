import { create } from '@bufbuild/protobuf';
import { TestBed } from '@angular/core/testing';
import { MAT_DIALOG_DATA, MatDialogRef } from '@angular/material/dialog';

import { DiceMode, DicePreference } from '../../../../../gen/meurpg/campaigns/v1/campaigns_pb';
import { SceneRollSchema } from '../../../../../gen/meurpg/play/v1/scene_pb';
import { RollDieSchema } from '../../../../../gen/meurpg/rules/v1/rules_pb';
import { SceneClient, type SceneDie } from '../../../../core/play/scene-client';
import { SceneState } from '../../../../core/play/scene-state';
import { FakeSceneClient, playerScene } from '../../../../core/play/scene-testing';
import { SceneRollSheet, type SceneRollSheetData } from './scene-roll-sheet';

const made = create(SceneRollSchema, {
  id: 'r1',
  actionId: 'a1',
  characterName: 'Pensantus',
  roll: { diceCount: 1, diceSides: 20, faces: [11], modifier: 6, total: 22, physical: false },
});
const die = (key: string, name: string, over: Record<string, unknown> = {}) =>
  create(RollDieSchema, {
    key,
    namePt: name,
    applies: 'check',
    sides: 8,
    resource: 'dados_de_vigor',
    ...over,
  });

describe('SceneRollSheet: a table die on the check (MR-025)', () => {
  beforeEach(() => {
    Element.prototype.scrollTo = vi.fn() as never;
  });

  function setup(preference = DicePreference.APP) {
    const api = new FakeSceneClient();
    const sent: { die: SceneDie; extra: readonly number[] | undefined; maneuver: string }[] = [];
    api.roll = async (
      _c: string,
      _a: string,
      d: SceneDie,
      _k: string,
      extra?: readonly number[],
      maneuver = '',
    ) => {
      sent.push({ die: d, extra, maneuver });
      return made;
    };
    const state = new SceneState(
      () => api.get(),
      () => false,
    );
    // The action is an Investigação check (skill:investigation).
    const data: SceneRollSheetData = {
      campaignId: 'c1',
      action: playerScene().actions[0],
      diceMode: DiceMode.PLAYERS_CHOOSE,
      preference,
      state,
      rollDice: [
        die('feature:fit@mesa', 'Faro fino', { targets: ['skill.investigation'] }),
        die('feature:other@mesa', 'Passo leve', { targets: ['skill.stealth'] }),
        die('feature:save@mesa', 'Mente firme', { applies: 'save' }),
      ],
    };
    TestBed.configureTestingModule({
      providers: [
        { provide: SceneClient, useValue: api },
        { provide: MAT_DIALOG_DATA, useValue: data },
        { provide: MatDialogRef, useValue: { close: vi.fn() } },
      ],
    });
    const fixture = TestBed.createComponent(SceneRollSheet);
    fixture.detectChanges();
    const el = fixture.nativeElement as HTMLElement;
    const settle = async () => {
      for (let i = 0; i < 3; i++) {
        await fixture.whenStable();
        fixture.detectChanges();
      }
    };
    const button = (name: string) =>
      Array.from(el.querySelectorAll<HTMLButtonElement>('button')).find((b) =>
        b.textContent?.includes(name),
      );
    const tick = () => {
      el.querySelector<HTMLInputElement>('app-roll-die-offer input[type=checkbox]')!.click();
      fixture.detectChanges();
    };
    return { el, button, tick, settle, sent, fixture };
  }

  it('offers only the die that fits this check, with its cost', () => {
    const { el } = setup();
    const offer = el.querySelector('app-roll-die-offer')!;
    expect(offer.querySelectorAll('input[type=checkbox]')).toHaveLength(1);
    expect(offer.textContent).toContain('Usar Faro fino');
    expect(offer.textContent).toContain('d8 · gasta 1 uso de dados de vigor');
    expect(el.textContent).not.toContain('Passo leve');
    expect(el.textContent).not.toContain('Mente firme');
  });

  it('rolls with no die by default, and sends its key when ticked', async () => {
    const plain = setup();
    plain.button('Rolar no app')!.click();
    await plain.settle();
    expect(plain.sent[0].maneuver).toBe('');
    TestBed.resetTestingModule();
    const used = setup();
    used.tick();
    used.button('Rolar no app')!.click();
    await used.settle();
    expect(used.sent[0].maneuver).toBe('feature:fit@mesa');
  });

  it("asks the die's face last with physical dice and sends it as the last extra face", async () => {
    const { el, button, tick, settle, sent, fixture } = setup(DicePreference.PHYSICAL);
    tick();
    button('Digitar o resultado')?.click();
    fixture.detectChanges();
    expect(el.textContent).toContain('Resultado do d8 (Faro fino)');
    const fields = el.querySelectorAll<HTMLInputElement>('input[type=text]');
    const type = (field: HTMLInputElement, text: string) => {
      field.value = text;
      field.dispatchEvent(new Event('input'));
      fixture.detectChanges();
    };
    // The die's field comes with the extra dice; the d20 field is the roll picker's.
    const dieField = el.querySelector<HTMLInputElement>('app-extra-dice input')!;
    type(dieField, '5');
    type(fields[fields.length - 1] === dieField ? fields[0] : fields[fields.length - 1], '11');
    button('Confirmar')!.click();
    await settle();
    expect(sent[0].die).toEqual({ face: 11 });
    expect(sent[0].extra).toEqual([5]);
    expect(sent[0].maneuver).toBe('feature:fit@mesa');
  });
});
