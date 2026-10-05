import { Code } from '@connectrpc/connect';

import { describeConnectError } from '../connect/connect-errors';
import { type LightOption, NO_LIGHT } from './light-presets';

/** The lights a character carries (D6): a torch, a hooded lantern, the spell Luz. The other SRD sources (a candle, a lamp, Chama Contínua, Luz do Dia) are lights of the place, set by the master as points. */
export const CARRIED_KEYS: readonly string[] = ['light:torch', 'light:hooded-lantern', 'light:light-spell'];

/** The options of the sheet and of the master's select: the three, and whatever the character carries now if it is something else (set by another route), so the list never hides it. */
export function carriedOptions(all: readonly LightOption[], current: string): LightOption[] {
  return all.filter((o) => CARRIED_KEYS.includes(o.key) || o.key === current);
}

/** The name of what a character carries: "Nenhuma" when nothing. */
export function carriedName(all: readonly LightOption[], key: string): string {
  return key === NO_LIGHT ? 'Nenhuma' : (all.find((o) => o.key === key)?.name ?? 'Uma luz');
}

/** The toast after the choice (E9-04): "Você acendeu a tocha: 6 m claro + 6 m de penumbra." */
export function litToast(option: LightOption | null): string {
  return option === null
    ? 'Você deixou de carregar luz.'
    : `Você acendeu a ${option.name.toLocaleLowerCase('pt-BR')}: ${option.radii}.`;
}

/** The master's line under his select: what the character carries, and how far it reaches. */
export function carriedLine(name: string, option: LightOption | null): string {
  return option === null ? `${name} não carrega luz.` : `${name} carrega ${option.name.toLocaleLowerCase('pt-BR')} (${option.radii}).`;
}

export function carriedErrorMessage(err: unknown): string {
  return describeConnectError(err, {
    [Code.InvalidArgument]: 'Essa luz não existe mais. Feche a folha e abra de novo.',
    [Code.NotFound]: 'Esse personagem não está neste mapa. Recarregue a página.',
    [Code.PermissionDenied]: 'Você só muda a luz do seu próprio personagem.',
  });
}
