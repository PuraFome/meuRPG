import {
  Body,
  Controller,
  Delete,
  Get,
  GoneException,
  HttpCode,
  NotFoundException,
  Param,
  Patch,
  Post,
  Query,
} from '@nestjs/common';
import {
  CharactersRepository,
  CharacterType,
} from '../db/characters.repository';
import { CreateCharacterDto, UpdateCharacterDto } from './characters.dto';

/**
 * Route order matters: the specific `join-tokens` / `join/:token` handlers
 * are declared BEFORE `:id` so Express never matches "join" as an id.
 */
@Controller('characters')
export class CharactersController {
  constructor(private readonly characters: CharactersRepository) {}

  @Post()
  async create(@Body() dto: CreateCharacterDto) {
    return this.characters.create(dto);
  }

  @Get()
  async findAll(@Query('type') type?: CharacterType) {
    return this.characters.findAll(type);
  }

  @Post('join-tokens')
  async createJoinToken() {
    return this.characters.createJoinToken();
  }

  @Get('join/:token')
  async resolveJoinToken(@Param('token') token: string) {
    const joinToken = await this.characters.findJoinToken(token);
    if (!joinToken) {
      throw new GoneException('Join token is invalid or expired');
    }
    return { valid: true, expiresAt: joinToken.expiresAt };
  }

  @Post('join/:token')
  async join(@Param('token') token: string, @Body() dto: CreateCharacterDto) {
    const joinToken = await this.characters.findJoinToken(token);
    if (!joinToken) {
      throw new GoneException('Join token is invalid or expired');
    }
    // forcedType pins the redeemed character to 'player' — any client-sent
    // type in the body is ignored.
    return this.characters.create(dto, 'player');
  }

  @Get(':id')
  async findById(@Param('id') id: string) {
    const character = await this.characters.findById(id);
    if (!character) {
      throw new NotFoundException('Character not found');
    }
    return character;
  }

  @Patch(':id')
  async update(@Param('id') id: string, @Body() dto: UpdateCharacterDto) {
    const updated = await this.characters.updateById(id, dto);
    if (!updated) {
      throw new NotFoundException('Character not found');
    }
    return updated;
  }

  @Delete(':id')
  @HttpCode(204)
  async delete(@Param('id') id: string): Promise<void> {
    const deleted = await this.characters.deleteById(id);
    if (!deleted) {
      throw new NotFoundException('Character not found');
    }
  }
}
