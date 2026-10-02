import { DamageType as GenDamageType } from '../../../gen/meurpg/characters/v1/characters_pb';
import { DamageTypeKey } from './character-labels';

/** `DamageTypeKey` → the generated enum: the key is the enum value's name in
 * lower case (`'fire'` is `DamageType.FIRE`), `''` is UNSPECIFIED. */
export function damageTypeToGen(key: DamageTypeKey): GenDamageType {
  return key
    ? GenDamageType[key.toUpperCase() as keyof typeof GenDamageType]
    : GenDamageType.UNSPECIFIED;
}

export function damageTypeFromGen(type: GenDamageType): DamageTypeKey {
  return (type ? GenDamageType[type].toLowerCase() : '') as DamageTypeKey;
}
