import { TestBed } from '@angular/core/testing';
import { create } from '@bufbuild/protobuf';

import { CreatureSummarySchema } from '../../../gen/meurpg/rules/v1/rules_pb';
import { CreaturesClient } from '../../core/creatures/creatures-client';
import { CreaturePick } from './creature-pick';

describe('CreaturePick, as the base of a table creature', () => {
  const found = [
    create(CreatureSummarySchema, { key: 'monster:wolf', namePt: 'Lobo', typePt: 'fera' }),
    create(CreatureSummarySchema, { key: 'creature:lobo-do-vale@mesa', namePt: 'Lobo do Vale' }),
  ];

  async function setup(srdOnly: boolean) {
    vi.useFakeTimers();
    TestBed.resetTestingModule();
    TestBed.configureTestingModule({
      providers: [
        {
          provide: CreaturesClient,
          useValue: { search: vi.fn().mockResolvedValue({ creatures: found, total: 2 }) },
        },
      ],
    });
    const fixture = TestBed.createComponent(CreaturePick);
    fixture.componentRef.setInput('campaignId', 'camp-1');
    fixture.componentRef.setInput('label', 'Criatura do SRD que serve de base');
    fixture.componentRef.setInput('placeholder', 'Procure');
    fixture.componentRef.setInput('srdOnly', srdOnly);
    fixture.detectChanges();
    const el = fixture.nativeElement as HTMLElement;
    const input = el.querySelector('input')!;
    input.value = 'lob';
    input.dispatchEvent(new Event('input'));
    await vi.advanceTimersByTimeAsync(300);
    fixture.detectChanges();
    return el;
  }

  afterEach(() => vi.useRealTimers());

  it('takes its own label and placeholder', async () => {
    const el = await setup(false);
    expect(el.querySelector('.pick__label')!.textContent).toBe('Criatura do SRD que serve de base');
    expect(el.querySelector('input')!.getAttribute('placeholder')).toBe('Procure');
  });

  it('offers the SRD creatures only when asked: a table creature is never a base', async () => {
    expect((await setup(false)).querySelectorAll('[role="option"]')).toHaveLength(2);
    const el = await setup(true);
    const options = Array.from(el.querySelectorAll('[role="option"]'));
    expect(options).toHaveLength(1);
    expect(options[0].textContent).toContain('Lobo');
    expect(options[0].textContent).not.toContain('Lobo do Vale');
  });
});
