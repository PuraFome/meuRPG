import { create } from '@bufbuild/protobuf';

import {
  AttackTargetsSchema,
  CombatantKind,
  CombatantSide,
  CombatantState,
  GetTurnOptionsResponseSchema,
  TargetInReachSchema,
} from '../../../gen/meurpg/play/v1/combat_pb';
import { combatant, encounter } from './combat-testing';
import {
  HELP_TASKS,
  attackIntro,
  attackRow,
  helpAllies,
  helpTargets,
  helpedText,
  taskRow,
} from './help-view';

const plain = (s: string) => s.replace(/\u00a0/g, ' ');
const party = CombatantSide.PARTY;

function table() {
  return encounter({
    combatants: [
      combatant({ id: 'o', label: 'Orla', kind: CombatantKind.PLAYER, side: party }),
      combatant({ id: 'v', label: 'Toren', kind: CombatantKind.PLAYER, side: party }),
      combatant({
        id: 'n',
        label: 'Nael',
        kind: CombatantKind.PLAYER,
        side: party,
        defeated: true,
      }),
      combatant({
        id: 'd',
        label: 'Ragna',
        kind: CombatantKind.PLAYER,
        side: party,
        state: CombatantState.DEAD,
      }),
      combatant({ id: 'h', label: 'Hobgoblin', side: CombatantSide.ENEMY }),
      combatant({ id: 'g', label: 'Goblin 1', side: CombatantSide.ENEMY }),
      combatant({ id: 'z', label: 'Goblin 2', side: CombatantSide.ENEMY }),
    ],
  });
}

const options = create(GetTurnOptionsResponseSchema, {
  attackTargets: [
    create(AttackTargetsSchema, {
      attackKey: 'a',
      targets: [
        create(TargetInReachSchema, { combatantId: 'g', distanceFt: 15 }),
        create(TargetInReachSchema, { combatantId: 'h', distanceFt: 5 }),
      ],
    }),
  ],
});

describe('what the Help sheet lists', () => {
  it('lists the allies that are standing, never the helper', () => {
    expect(helpAllies(table(), 'o').map((a) => a.label)).toEqual(['Toren']);
    expect(helpAllies(table(), 'nobody')).toEqual([]);
  });

  it('lists the creatures the helper sees with their distance, the ones within 1,5 m first', () => {
    const rows = helpTargets(options, table(), 'o');
    expect(rows.map((r) => [r.label, r.distanceFt, r.reachable])).toEqual([
      ['Hobgoblin', 5, true],
      ['Goblin 1', 15, false],
    ]);
    expect(helpTargets(null, table(), 'o')).toEqual([]);
  });

  it('measures the reach on the grid for a helper with no attack (a familiar), as the server does', () => {
    const e = encounter({
      combatants: [
        combatant({
          id: 'f',
          label: 'Nanquim',
          kind: CombatantKind.CREATURE,
          side: party,
          placed: true,
          col: 5,
          row: 3,
        }),
        combatant({
          id: 't',
          label: 'Toren',
          kind: CombatantKind.PLAYER,
          side: party,
          placed: true,
          col: 3,
          row: 3,
        }),
        combatant({
          id: 'g',
          label: 'Goblin',
          side: CombatantSide.ENEMY,
          placed: true,
          col: 4,
          row: 4,
        }),
        combatant({
          id: 'h',
          label: 'Hobgoblin',
          side: CombatantSide.ENEMY,
          placed: true,
          col: 8,
          row: 7,
        }),
      ],
    });
    const rows = helpTargets(create(GetTurnOptionsResponseSchema, {}), e, 'f');
    expect(rows.map((r) => [r.label, r.distanceFt, r.reachable])).toEqual([
      ['Goblin', 5, true], // the diagonal neighbour
      ['Hobgoblin', 25, false], // 3 across and 4 down: 5 squares
    ]);
  });

  it('names the official tasks, with Percepção and Arcanismo among them', () => {
    const names = HELP_TASKS.map((t) => t.name);
    expect(names).toContain('Percepção');
    expect(names).toContain('Arcanismo');
    expect(HELP_TASKS.find((t) => t.name === 'Furtividade')?.key).toBe('skill:stealth');
    expect(HELP_TASKS).toHaveLength(18);
  });

  it('writes the rows of a check help as the board does', () => {
    const [toren] = helpAllies(table(), 'o');
    expect(taskRow(toren, 'Percepção')).toEqual({
      name: 'Toren · Percepção',
      sub: 'Vantagem no próximo teste de Percepção dele.',
    });
  });

  it('writes the rows of an attack help: within reach, and refused with the reason when farther', () => {
    const [toren] = helpAllies(table(), 'o');
    const [near, far] = helpTargets(options, table(), 'o');
    expect(plain(attackRow(toren, near).sub)).toBe('O Hobgoblin está a 1,5 m de você.');
    expect(attackRow(toren, near).name).toBe('Toren ataca o Hobgoblin');
    expect(attackRow(toren, near).blocked).toBe('');
    expect(plain(attackRow(toren, far).sub)).toBe('A 4,5 m de você.');
    expect(plain(attackRow(toren, far).blocked)).toBe('Longe demais: no máximo 1,5 m.');
    expect(plain(attackIntro(toren))).toBe(
      'O alvo precisa estar a até 1,5 m de você. O primeiro ataque de Toren contra ele terá vantagem.',
    );
  });

  it('leaves the reach to the master where the combat has no map', () => {
    const noMap = create(GetTurnOptionsResponseSchema, {
      attackTargets: [
        create(AttackTargetsSchema, {
          attackKey: 'a',
          targets: [create(TargetInReachSchema, { combatantId: 'h' })],
        }),
      ],
    });
    const [row] = helpTargets(noMap, table(), 'o');
    expect(row.reachable).toBe(true);
    const [toren] = helpAllies(table(), 'o');
    expect(attackRow(toren, row).sub).toBe('');
  });

  it('says what was done', () => {
    const [toren] = helpAllies(table(), 'o');
    const [target] = helpTargets(options, table(), 'o');
    expect(helpedText(toren, 'Percepção', null)).toEqual({
      lead: 'Você ajudou Toren.',
      rest: 'Ele terá vantagem no próximo teste de Percepção.',
    });
    expect(helpedText(toren, null, target).rest).toBe(
      'O primeiro ataque dele contra o Hobgoblin terá vantagem.',
    );
  });
});
