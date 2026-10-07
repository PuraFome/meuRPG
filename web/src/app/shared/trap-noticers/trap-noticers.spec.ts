import { TestBed } from '@angular/core/testing';
import { create } from '@bufbuild/protobuf';

import {
  GetTrapNoticersResponseSchema,
  TrapNoticerSchema,
} from '../../../gen/meurpg/maps/v1/maps_pb';
import { TrapNoticers, noticerRow } from './trap-noticers';

const noticer = (extra: object) =>
  create(TrapNoticerSchema, {
    characterId: 'x',
    characterName: 'Brisa',
    passivePerception: 14,
    ...extra,
  });

describe('noticerRow', () => {
  it('reads would_notice for a character in range and passes_dc for one out of range', () => {
    expect(
      noticerRow(
        noticer({ onMap: true, inRange: true, sees: true, wouldNotice: true, passesDc: true }),
      ).verdictWord,
    ).toBe('Nota');
    expect(noticerRow(noticer({ onMap: true, inRange: true, sees: true })).verdictWord).toBe(
      'Não nota',
    );
    expect(noticerRow(noticer({ onMap: true, passesDc: true })).verdictWord).toContain(
      'Se chegar a 3 m: nota',
    );
    expect(noticerRow(noticer({ onMap: false })).verdictWord).toContain(
      'Se chegar a 3 m: não nota',
    );
    expect(noticerRow(noticer({ knows: true })).verdictWord).toBe('Já sabe');
  });

  it("writes the server's two numbers: the passive and the penalty", () => {
    expect(noticerRow(noticer({ lightPenalty: -5 })).penalty).toBe('−5 na penumbra');
    expect(noticerRow(noticer({})).penalty).toBe('');
  });
});

describe('TrapNoticers', () => {
  it('shows "Lendo…" while it waits, the error with "Tentar de novo" when the read failed, and the rows once read', () => {
    const fixture = TestBed.createComponent(TrapNoticers);
    fixture.detectChanges();
    const el = fixture.nativeElement as HTMLElement;
    expect(el.textContent).toContain('Lendo quem notaria');
    const retries: number[] = [];
    fixture.componentInstance.retry.subscribe(() => retries.push(1));
    fixture.componentRef.setInput('failed', true);
    fixture.detectChanges();
    expect(el.querySelector('[role=alert]')?.textContent).toContain(
      'Não deu para ler quem notaria',
    );
    Array.from(el.querySelectorAll('button'))
      .find((b) => b.textContent?.includes('Tentar de novo'))!
      .click();
    expect(retries).toHaveLength(1);
    fixture.componentRef.setInput('failed', false);
    fixture.componentRef.setInput(
      'noticers',
      create(GetTrapNoticersResponseSchema, {
        noticeDc: 15,
        noticers: [noticer({ onMap: true, inRange: true, lightPenalty: -5 })],
      }),
    );
    fixture.detectChanges();
    expect(el.querySelectorAll('.tn__row')).toHaveLength(1);
    expect(el.textContent).toContain('(−5 na penumbra)');
  });

  it("in the editor's panel it takes the panel title style and stacks the rows in a narrow column (the host says so); the session card keeps its layout", () => {
    const panel = TestBed.createComponent(TrapNoticers);
    panel.componentRef.setInput('panel', true);
    panel.componentRef.setInput(
      'noticers',
      create(GetTrapNoticersResponseSchema, {
        noticeDc: 15,
        noticers: [noticer({ onMap: true, inRange: true })],
      }),
    );
    panel.detectChanges();
    expect((panel.nativeElement as HTMLElement).classList).toContain('tn--panel');
    expect((panel.nativeElement as HTMLElement).querySelector('h4')?.classList).toContain(
      'mr-panel__title',
    );
    const card = TestBed.createComponent(TrapNoticers);
    card.componentRef.setInput(
      'noticers',
      create(GetTrapNoticersResponseSchema, { noticeDc: 15, noticers: [] }),
    );
    card.detectChanges();
    expect((card.nativeElement as HTMLElement).classList).not.toContain('tn--panel');
  });

  it("says what the dim light costs, and that each one's own number is beside the passive", () => {
    const fixture = TestBed.createComponent(TrapNoticers);
    fixture.componentRef.setInput(
      'noticers',
      create(GetTrapNoticersResponseSchema, {
        noticeDc: 15,
        noticers: [noticer({ onMap: true, inRange: true, lightPenalty: -5 })],
      }),
    );
    fixture.detectChanges();
    const text = (fixture.nativeElement as HTMLElement).textContent ?? '';
    expect(text).toContain('A penumbra tira 5 da passiva de quem não enxerga no escuro');
    expect(text).toContain('−5 na penumbra');
  });
});
