import { TestBed } from '@angular/core/testing';
import { Code, ConnectError } from '@connectrpc/connect';
import { create } from '@bufbuild/protobuf';

import { DiceMode, DicePreference } from '../../../../gen/meurpg/campaigns/v1/campaigns_pb';
import { GroupCheckMemberViewSchema } from '../../../../gen/meurpg/play/v1/contest_types_pb';
import { ContestClient } from '../../../core/combat/contest-client';
import {
  FakeContestClient,
  checkRoll,
  groupCheck,
  textOf,
} from '../../../core/combat/contest-testing';
import { GroupCheckMaster } from './group-check-master';

const member = (over: object) =>
  create(GroupCheckMemberViewSchema, { characterId: 'x', name: 'X', ...over });

const answered = (id: string, name: string, total: number, passed: boolean) =>
  member({
    characterId: id,
    name,
    answered: true,
    roll: checkRoll({ total, modifier: 1 }),
    passedKnown: true,
    passed,
  });

const openCheck = () =>
  groupCheck({
    dc: 13,
    needed: 2,
    passedCount: 1,
    members: [
      answered('b', 'Brisa', 19, true),
      answered('t', 'Toren', 8, false),
      member({ characterId: 'r', name: 'Ragna' }),
    ],
  });

function setup(view: ReturnType<typeof groupCheck> | null = null) {
  const api = new FakeContestClient();
  api.group = view;
  TestBed.configureTestingModule({ providers: [{ provide: ContestClient, useValue: api.as() }] });
  const fixture = TestBed.createComponent(GroupCheckMaster);
  const ref = fixture.componentRef;
  ref.setInput('campaignId', 'camp');
  ref.setInput('tick', 0);
  ref.setInput('diceMode', DiceMode.PLAYERS_CHOOSE);
  ref.setInput('preference', DicePreference.APP);
  fixture.detectChanges();
  const el = fixture.nativeElement as HTMLElement;
  const settle = async () => {
    for (let i = 0; i < 4; i++) {
      await fixture.whenStable();
      fixture.detectChanges();
    }
  };
  const button = (name: string) =>
    Array.from(el.querySelectorAll<HTMLButtonElement>('button')).find((b) =>
      textOf(b).includes(name),
    );
  const type = (selector: string, value: string) => {
    const field = el.querySelector<HTMLInputElement>(selector)!;
    field.value = value;
    field.dispatchEvent(new Event('input'));
    fixture.detectChanges();
  };
  return { api, fixture, el, settle, button, type };
}

describe('GroupCheckMaster, the master asks the party for a check', () => {
  describe('asking', () => {
    it('lists the skills, a DC that may be empty and "Mostrar a CD aos jogadores"', async () => {
      const { el, settle } = setup();
      await settle();
      const text = textOf(el);
      expect(text).toContain('Teste em grupo');
      expect(text).toContain(
        'Todos os personagens rolam o mesmo teste; o grupo passa se ao menos metade passar.',
      );
      expect(text).toContain('CD (1 a 40; vazio: sem CD)');
      expect(text).toContain('Mostrar a CD aos jogadores');
      expect(Array.from(el.querySelectorAll('option')).map((o) => o.textContent?.trim())).toContain(
        'Furtividade',
      );
    });

    it('asks with the skill, the DC and show_dc, once', async () => {
      const { api, el, settle, button, type, fixture } = setup();
      await settle();
      const select = el.querySelector<HTMLSelectElement>('select')!;
      select.value = 'skill:stealth';
      select.dispatchEvent(new Event('change'));
      type('input[type="text"]', '13');
      el.querySelector<HTMLInputElement>('input[type="checkbox"]')!.click();
      fixture.detectChanges();
      api.group = openCheck();
      button('Pedir o teste')!.click();
      await settle();
      expect(api.groupRequests).toHaveLength(1);
      expect(api.groupRequests[0].request).toEqual({
        skillKey: 'skill:stealth',
        dc: 13,
        showDc: true,
      });
      expect(api.groupRequests[0].key).toEqual(expect.any(String));
      expect(textOf(el)).toContain('Teste em grupo pedido: Furtividade.');
      expect(el.querySelector('[data-testid="group-open"]')).toBeTruthy();
    });

    it('with no DC nothing is shown to the players and a bad DC cannot be sent', async () => {
      const { api, settle, button, type } = setup();
      await settle();
      type('input[type="text"]', '99');
      expect(button('Pedir o teste')!.disabled).toBe(true);
      type('input[type="text"]', '');
      button('Pedir o teste')!.click();
      await settle();
      expect(api.groupRequests[0].request).toMatchObject({ dc: 0, showDc: false });
    });

    it('tells the refusal of a second open check', async () => {
      const { api, el, settle, button } = setup();
      await settle();
      api.error = new ConnectError('open', Code.FailedPrecondition);
      button('Pedir o teste')!.click();
      await settle();
      expect(el.querySelector('[role="alert"]')).toBeTruthy();
    });
  });

  describe('waiting', () => {
    it('shows who answered, each total and verdict, and who did not', async () => {
      const { el, settle } = setup(openCheck());
      await settle();
      const text = textOf(el);
      expect(textOf(el.querySelector('h2')!)).toBe('Teste em grupo: Furtividade, CD 13');
      expect(text).toContain(
        '2 de 3 responderam. O grupo passa se ao menos metade dos convocados passar.',
      );
      expect(Array.from(el.querySelectorAll('.row')).map((r) => textOf(r))).toEqual([
        'Brisa Furtividade +1 19 Passou',
        'Toren Furtividade +1 8 Falhou',
        'Ragna Furtividade Não respondeu Rolar por Ragna',
      ]);
    });

    it('rolls for the one who did not answer, in the app, with its own key', async () => {
      const { api, el, settle, button } = setup(openCheck());
      await settle();
      button('Rolar por Ragna')!.click();
      await settle();
      el.querySelector<HTMLButtonElement>('.row__form button')!.click();
      await settle();
      expect(api.masterRolls).toEqual([
        { characterId: 'r', die: { inApp: true }, key: expect.any(String) },
      ]);
    });

    it('closes the check: whoever did not answer fails', async () => {
      const { api, el, settle, button } = setup(openCheck());
      await settle();
      api.group = groupCheck({
        open: false,
        dc: 13,
        needed: 2,
        passedCount: 1,
        verdictKnown: true,
        groupPassed: false,
        members: openCheck().members,
      });
      button('Encerrar o teste')!.click();
      await settle();
      expect(api.groupCloses).toEqual([{ id: 'gc1', key: expect.any(String) }]);
      expect(textOf(el)).toContain('1 de 3 passaram; precisa de 2. O grupo falhou.');
      button('Dispensar')!.click();
      await settle();
      expect(el.querySelector('[data-testid="group-last"]')).toBeNull();
    });

    it('says the verdict is his until the DC is shown, and reads it again on each tick', async () => {
      const view = groupCheck({
        ...openCheck(),
        verdictKnown: true,
        groupPassed: false,
        showDc: false,
        members: openCheck().members,
      } as never);
      const { api, fixture, el, settle } = setup(view);
      await settle();
      expect(textOf(el)).toContain('Veredito do grupo (só o mestre):');
      expect(textOf(el)).toContain('Os jogadores veem passou ou falhou só se você mostrar a CD.');
      const before = api.calls.filter((c) => c === 'groupCheck').length;
      fixture.componentRef.setInput('tick', 1);
      await settle();
      expect(api.calls.filter((c) => c === 'groupCheck').length).toBe(before + 1);
    });
  });
});
