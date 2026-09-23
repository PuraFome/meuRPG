import {
  IsArray,
  IsIn,
  IsNotEmpty,
  IsObject,
  IsOptional,
  IsString,
  IsUUID,
} from 'class-validator';
import type { CharacterType } from '../db/characters.repository';

const CHARACTER_TYPES: readonly CharacterType[] = [
  'npc',
  'player',
  'boss',
  'minion',
];

/**
 * Every accepted field MUST carry a decorator: the global ValidationPipe
 * runs with `whitelist: true, forbidNonWhitelisted: true` (main.ts), so an
 * undecorated property is rejected with 400.
 */
export class CreateCharacterDto {
  @IsOptional()
  @IsUUID()
  id?: string;

  @IsString()
  @IsNotEmpty()
  name!: string;

  @IsIn(CHARACTER_TYPES)
  type!: CharacterType;

  @IsOptional()
  @IsString()
  description?: string;

  @IsOptional()
  @IsString()
  imageUrl?: string;

  @IsOptional()
  @IsString()
  history?: string;

  @IsOptional()
  @IsString()
  masterNotes?: string;

  @IsOptional()
  @IsObject()
  attributes?: Record<string, number>;

  @IsOptional()
  @IsArray()
  skills?: string[];

  @IsOptional()
  @IsArray()
  inventory?: string[];

  @IsOptional()
  @IsArray()
  quotes?: string[];

  @IsOptional()
  @IsObject()
  sheet?: unknown;

  @IsOptional()
  @IsObject()
  minion?: { hp: number; attack: number } | null;
}

/** Same shape as create, but every field optional (partial patch). */
export class UpdateCharacterDto {
  @IsOptional()
  @IsUUID()
  id?: string;

  @IsOptional()
  @IsString()
  @IsNotEmpty()
  name?: string;

  @IsOptional()
  @IsIn(CHARACTER_TYPES)
  type?: CharacterType;

  @IsOptional()
  @IsString()
  description?: string;

  @IsOptional()
  @IsString()
  imageUrl?: string;

  @IsOptional()
  @IsString()
  history?: string;

  @IsOptional()
  @IsString()
  masterNotes?: string;

  @IsOptional()
  @IsObject()
  attributes?: Record<string, number>;

  @IsOptional()
  @IsArray()
  skills?: string[];

  @IsOptional()
  @IsArray()
  inventory?: string[];

  @IsOptional()
  @IsArray()
  quotes?: string[];

  @IsOptional()
  @IsObject()
  sheet?: unknown;

  @IsOptional()
  @IsObject()
  minion?: { hp: number; attack: number } | null;
}
