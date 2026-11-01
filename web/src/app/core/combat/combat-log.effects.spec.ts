import { type MessageInitShape, create } from '@bufbuild/protobuf';

import {
  CombatLogEffectChange,
  CombatLogEffectSchema,
  type CombatLogEntry,
  CombatLogEntrySchema,
  CombatLogKind,
} from '../../../gen/meurpg/play/v1/combat_pb';
import { logLine } from './combat-log';

const effect = (
  fields: MessageInitShape<typeof CombatLogEffectSchema>,
  kind = CombatLogKind.EFFECT,
): CombatLogEntry =>
  create(CombatLogEntrySchema, {
    id: 'x',
    kind,
    effect: create(CombatLogEffectSchema, {
      sourceKey: 'spell:heroism',
      sourceNamePt: 'Heroísmo',
      targetLabels: ['Brisa'],
      ...fields,
    }),
  });

const text = (e: CombatLogEntry, players = ['Brisa']) =>
  logLine(e, '', { master: false, players: new Set(players) })?.text.replace(/ /g, ' ');

describe('the lasting effect lines of the log', () => {
  it('says an effect gave temporary hit points', () => {
    expect(text(effect({ change: CombatLogEffectChange.TEMP_HP, amount: 5 }))).toBe(
      'Heroísmo deu 5 PV temporários a Brisa',
    );
    expect(
      text(
        effect({ change: CombatLogEffectChange.TEMP_HP, amount: 5, targetLabels: ['Goblin 2'] }),
      ),
    ).toBe('Heroísmo deu 5 PV temporários ao Goblin 2');
  });

  it('says an effect was added, ended or had its duration changed', () => {
    expect(text(effect({ change: CombatLogEffectChange.ADDED }))).toBe(
      'Heroísmo foi aplicado em Brisa',
    );
    expect(text(effect({ change: CombatLogEffectChange.ENDED }))).toBe('Heroísmo acabou em Brisa');
    expect(text(effect({ change: CombatLogEffectChange.DURATION_CHANGED }))).toBe(
      'A duração de Heroísmo em Brisa mudou',
    );
  });

  it('says how a saving throw against an effect went, with the numbers only for the master', () => {
    const saved = effect({
      change: CombatLogEffectChange.SAVED,
      sourceNamePt: 'Teia',
      abilityNamePt: 'Destreza',
      d20: 12,
      total: 15,
      dc: 14,
    });
    // The player reads the DC only (the server sends it when the table shows it), never the dice.
    expect(text(saved)).toBe('Brisa passou no teste de Destreza contra Teia (CD 14)');
    expect(logLine(saved, '', { master: true, players: new Set(['Brisa']) })?.text).toContain(
      '(d20 12, total 15 contra CD 14)',
    );
    expect(
      text(effect({ change: CombatLogEffectChange.SAVE_FAILED, abilityNamePt: 'Sabedoria' })),
    ).toBe('Brisa falhou no teste de Sabedoria contra Heroísmo');
  });

  it('tells the master a save he made a success, and tells nobody else', () => {
    const resisted = effect({
      change: CombatLogEffectChange.SAVED,
      sourceNamePt: 'Imobilizar Pessoa',
      targetLabels: ['Draco'],
      abilityNamePt: 'Sabedoria',
      d20: 3,
      total: 10,
      dc: 15,
      resisted: true,
      resistedLeft: 2,
    });
    expect(logLine(resisted, '', { master: true, players: new Set() })?.text).toBe(
      'O Draco falhou no teste de Sabedoria contra Imobilizar Pessoa (d20 3, total 10 contra CD 15) e usou Resistência Lendária: sucesso (restam 2)',
    );
    // A player's entry never has the flag; read as a player it is the line of a save that passed.
    expect(text(effect({ change: CombatLogEffectChange.SAVED, targetLabels: ['Draco'] }))).toBe(
      'O Draco passou no teste contra Heroísmo',
    );
  });

  it('tags a natural 20 or 1 on a save, for the master only, and never as a result', () => {
    const save = (d20: number) =>
      effect({
        change: CombatLogEffectChange.SAVE_FAILED,
        abilityNamePt: 'Destreza',
        d20,
        total: d20 + 2,
        dc: 14,
      });
    const master = (e: CombatLogEntry) =>
      logLine(e, '', { master: true, players: new Set(['Brisa']) })?.text;
    expect(master(save(20))).toContain('(d20 20, 20 natural, total 22 contra CD 14)');
    expect(master(save(1))).toContain('(d20 1, 1 natural, total 3 contra CD 14)');
    expect(master(save(12))).toContain('(d20 12, total 14 contra CD 14)');
    expect(text(save(20))).not.toContain('natural');
  });

  it('says the damage an effect dealt', () => {
    expect(
      text(effect({ change: CombatLogEffectChange.DAMAGE, amount: 7, damageTypePt: 'ígneo' })),
    ).toBe('Heroísmo causou 7 de dano ígneo a Brisa');
  });

  it('writes the exhaustion line', () => {
    expect(text(effect({ exhaustionLevel: 2 }, CombatLogKind.EXHAUSTION))).toBe(
      'Brisa agora tem exaustão de nível 2',
    );
    expect(text(effect({ exhaustionLevel: 0 }, CombatLogKind.EXHAUSTION))).toBe(
      'Brisa ficou sem exaustão',
    );
  });

  it('draws nothing for an entry that carries no effect', () => {
    expect(
      logLine(create(CombatLogEntrySchema, { id: 'x', kind: CombatLogKind.EFFECT })),
    ).toBeNull();
  });
});
