import { DiceMode, DicePreference } from '../../../gen/meurpg/campaigns/v1/campaigns_pb';

/** One choice of a dice radio group (RN-18): a title and one line under it. */
export interface DiceOption<T> {
  readonly value: T;
  readonly title: string;
  readonly description: string;
}

/** What the master picks in "Dados" (E6-17). */
export const DICE_MODE_OPTIONS: readonly DiceOption<DiceMode>[] = [
  {
    value: DiceMode.PLAYERS_CHOOSE,
    title: 'Cada jogador escolhe',
    description: 'Cada pessoa decide, na própria tela, se rola no app ou com os próprios dados.',
  },
  {
    value: DiceMode.APP,
    title: 'Todos rolam no app',
    description: 'O app rola e registra cada dado. Ninguém digita resultado.',
  },
  {
    value: DiceMode.PHYSICAL,
    title: 'Todos rolam os próprios dados',
    description: 'Cada jogador digita o que tirou nos dados de verdade.',
  },
];

/** What a player picks in "Como você rola os dados" (E6-18). */
export const DICE_PREFERENCE_OPTIONS: readonly DiceOption<DicePreference>[] = [
  {
    value: DicePreference.APP,
    title: 'No app',
    description: 'O app rola o d20 e o dano, e mostra a conta. Você toca em "Rolar no app".',
  },
  {
    value: DicePreference.PHYSICAL,
    title: 'Meus próprios dados',
    description: 'Você rola o d20 de verdade e digita o número. O app soma o bônus.',
  },
];

/** The short chip: "No app" / "Meus próprios dados". */
export function preferenceLabel(preference: DicePreference): string {
  return preference === DicePreference.PHYSICAL ? 'Meus próprios dados' : 'No app';
}

/** Where a player rolls, once the campaign's mode and their choice are put
 * together (the same rule as the server's EffectiveDiceMode). */
export function effectivePreference(mode: DiceMode, preference: DicePreference): DicePreference {
  if (mode === DiceMode.APP) {
    return DicePreference.APP;
  }
  if (mode === DiceMode.PHYSICAL) {
    return DicePreference.PHYSICAL;
  }
  return preference === DicePreference.PHYSICAL ? DicePreference.PHYSICAL : DicePreference.APP;
}
