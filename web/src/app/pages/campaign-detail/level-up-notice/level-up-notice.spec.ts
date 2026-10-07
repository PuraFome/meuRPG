import { TestBed } from '@angular/core/testing';
import { provideRouter } from '@angular/router';
import { create } from '@bufbuild/protobuf';
import { timestampFromMs } from '@bufbuild/protobuf/wkt';

import { LevelUpChoicesSchema, LevelUpHitPointsMethod, LevelUpSchema } from '../../../../gen/meurpg/characters/v1/characters_pb';
import { LevelUpClient } from '../../../core/levelup/levelup-client';
import { LevelUpFeed } from '../../../core/levelup/levelup-feed';
import { LevelUpChanges } from '../characters/level-up-changes';
import { LevelUpNotice } from './level-up-notice';

const levelUp = (name: string, to: number, ago: number) =>
  create(LevelUpSchema, {
    id: `${name}${to}`,
    characterId: name,
    characterName: name,
    toLevel: to,
    createdAt: timestampFromMs(Date.now() - ago),
    namesPt: { 'spell:light': 'Luz' },
    choices: create(LevelUpChoicesSchema, { abilityIncrease: { intelligence: 2 }, cantripKeys: ['spell:light'], hitPoints: { method: LevelUpHitPointsMethod.AVERAGE, value: 4 } }),
  });

describe("the master's level-up notice and \"O que mudou\" (MR-040)", () => {
  const list = vi.fn();

  async function notice(items: ReturnType<typeof levelUp>[]) {
    list.mockReset().mockResolvedValue({ levelUps: items, nextPageToken: '' });
    TestBed.configureTestingModule({ providers: [provideRouter([]), LevelUpFeed, { provide: LevelUpClient, useValue: { list } }] });
    const fixture = TestBed.createComponent(LevelUpNotice);
    fixture.componentRef.setInput('campaignId', 'camp-1');
    fixture.detectChanges();
    await fixture.whenStable();
    fixture.detectChanges();
    return fixture;
  }
  const text = (f: { nativeElement: HTMLElement }) => f.nativeElement.textContent?.replace(/\s+/g, ' ') ?? '';

  it('tells one fresh level-up by name, in a polite status region', async () => {
    const f = await notice([levelUp('Pensantus', 4, 1000)]);
    expect(text(f)).toContain('Pensantus subiu para o nível 4.')
    expect(text(f)).toContain('Ver o que mudou');
    expect(f.nativeElement.querySelector('[role=status]')).not.toBeNull();
  });

  it('counts several, and says nothing about one older than a day', async () => {
    const two = await notice([levelUp('Pensantus', 4, 1000), levelUp('Brisa', 4, 2000)]);
    expect(text(two)).toContain('2 personagens subiram de nível.');
    TestBed.resetTestingModule();
    const old = await notice([levelUp('Pensantus', 4, 25 * 60 * 60 * 1000)]);
    expect(text(old)).not.toContain('subiu');
  });

  it('is dismissed in memory, and a new level-up brings it back', async () => {
    const f = await notice([levelUp('Pensantus', 4, 1000)]);
    (f.nativeElement.querySelector('button[aria-label="Dispensar o aviso"]') as HTMLButtonElement).click();
    f.detectChanges();
    expect(text(f)).not.toContain('subiu');
    list.mockResolvedValue({ levelUps: [levelUp('Brisa', 4, 10), levelUp('Pensantus', 4, 1000)], nextPageToken: '' });
    await TestBed.inject(LevelUpFeed).refresh();
    f.detectChanges();
    expect(text(f)).toContain('2 personagens subiram de nível.');
  });

  it('opens what the player chose, with when, and says there is nothing to approve', () => {
    TestBed.resetTestingModule();
    TestBed.configureTestingModule({ providers: [provideRouter([])] });
    const f = TestBed.createComponent(LevelUpChanges);
    f.componentRef.setInput('levelUp', levelUp('Pensantus', 4, 1000));
    f.componentRef.setInput('campaignId', 'camp-1');
    f.detectChanges();
    const t = text(f);
    expect(t).toContain('O que Pensantus escolheu no nível 4');
    expect(t).toContain('Habilidades+2 em Inteligência');
    expect(t).toContain('Truque novoLuz');
    expect(t).toContain('Confirmado em');
    expect(t).toContain('Nada fica à espera do seu OK');
    expect((f.nativeElement as HTMLElement).querySelector('a')?.getAttribute('href')).toBe('/campaigns/camp-1/characters/Pensantus');
  });
});
