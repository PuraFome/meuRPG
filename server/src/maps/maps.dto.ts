import {
  IsArray,
  IsIn,
  IsInt,
  IsNotEmpty,
  IsObject,
  IsOptional,
  IsString,
  IsUUID,
  Min,
} from 'class-validator';
import type { MapKind } from '../db/maps.repository';

const MAP_KINDS: readonly MapKind[] = ['world', 'city', 'dungeon', 'local'];

/**
 * Every accepted field MUST carry a decorator: the global ValidationPipe runs
 * with `whitelist: true, forbidNonWhitelisted: true` (main.ts), so an
 * undecorated property is rejected with 400.
 */
export class CreateMapDto {
  @IsOptional()
  @IsUUID()
  id?: string;

  @IsString()
  @IsNotEmpty()
  name!: string;

  @IsOptional()
  @IsString()
  description?: string;

  @IsOptional()
  @IsIn(MAP_KINDS)
  kind?: MapKind;

  @IsOptional()
  @IsString()
  backgroundImage?: string;

  @IsOptional()
  @IsInt()
  @Min(1)
  width?: number;

  @IsOptional()
  @IsInt()
  @Min(1)
  height?: number;

  @IsOptional()
  @IsObject()
  grid?: unknown;

  @IsOptional()
  @IsObject()
  fogOfWar?: unknown;

  @IsOptional()
  @IsArray()
  layers?: unknown[];

  @IsOptional()
  @IsArray()
  markers?: unknown[];

  @IsOptional()
  @IsArray()
  submaps?: unknown[];

  @IsOptional()
  @IsObject()
  dungeon?: unknown;
}

/** Same shape as create, but every field optional (partial patch). */
export class UpdateMapDto {
  @IsOptional()
  @IsUUID()
  id?: string;

  @IsOptional()
  @IsString()
  @IsNotEmpty()
  name?: string;

  @IsOptional()
  @IsString()
  description?: string;

  @IsOptional()
  @IsIn(MAP_KINDS)
  kind?: MapKind;

  @IsOptional()
  @IsString()
  backgroundImage?: string;

  @IsOptional()
  @IsInt()
  @Min(1)
  width?: number;

  @IsOptional()
  @IsInt()
  @Min(1)
  height?: number;

  @IsOptional()
  @IsObject()
  grid?: unknown;

  @IsOptional()
  @IsObject()
  fogOfWar?: unknown;

  @IsOptional()
  @IsArray()
  layers?: unknown[];

  @IsOptional()
  @IsArray()
  markers?: unknown[];

  @IsOptional()
  @IsArray()
  submaps?: unknown[];

  @IsOptional()
  @IsObject()
  dungeon?: unknown;
}
