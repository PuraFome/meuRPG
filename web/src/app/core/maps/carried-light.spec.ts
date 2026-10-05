import { describe, expect, it } from 'vitest';

import { carriedLine, carriedName, carriedOptions, litToast } from './carried-light';
import { lightRadii, type LightOption } from './light-presets';

const plain = (t: string) => t.replace(/\u00a0/g, ' ');
const torch: LightOption = { key: 'light:torch', name: 'Tocha', radii: lightRadii(20, 20) };
const lantern: LightOption = { key: 'light:hooded-lantern', name: 'Lanterna coberta', radii: lightRadii(30, 30) };
const spell: LightOption = { key: 'light:light-spell', name: 'Luz', radii: lightRadii(20, 20) };
const candle: LightOption = { key: 'light:candle', name: 'Vela', radii: lightRadii(5, 5) };
const daylight: LightOption = { key: 'light:daylight', name: 'Luz do Dia', radii: lightRadii(60, 60) };

describe('the light a character carries', () => {
  it('says the radii in meters, at the table\'s rate', () => {
    expect(plain(lightRadii(20, 20))).toBe('6 m claro + 6 m de penumbra');
    expect(plain(lightRadii(30, 30))).toBe('9 m claro + 9 m de penumbra');
    expect(plain(lightRadii(5, 5))).toBe('1,5 m claro + 1,5 m de penumbra');
    expect(plain(lightRadii(20, 0))).toBe('6 m claro');
  });

  it('offers the torch, the hooded lantern and the spell Luz, and what the character carries now if it is another', () => {
    const all = [candle, torch, lantern, spell, daylight];
    expect(carriedOptions(all, '').map((o) => o.name)).toEqual(['Tocha', 'Lanterna coberta', 'Luz']);
    expect(carriedOptions(all, 'light:daylight').map((o) => o.name)).toEqual(['Tocha', 'Lanterna coberta', 'Luz', 'Luz do Dia']);
  });

  it('names what is carried, "Nenhuma" for nothing', () => {
    expect(carriedName([torch], '')).toBe('Nenhuma');
    expect(carriedName([torch], 'light:torch')).toBe('Tocha');
  });

  it('tells what the choice did, in the toast and in the master\'s line', () => {
    expect(plain(litToast(torch))).toBe('Você acendeu a tocha: 6 m claro + 6 m de penumbra.');
    expect(plain(litToast(null))).toBe('Você deixou de carregar luz.');
    expect(plain(carriedLine('Toren', torch))).toBe('Toren carrega tocha (6 m claro + 6 m de penumbra).');
    expect(plain(carriedLine('Toren', null))).toBe('Toren não carrega luz.');
  });
});
