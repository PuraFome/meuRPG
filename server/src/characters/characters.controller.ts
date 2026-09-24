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
  UseGuards,
} from '@nestjs/common';
import {
  CharactersRepository,
  CharacterType,
} from '../db/characters.repository';
import { CreateCharacterDto, UpdateCharacterDto } from './characters.dto';
import { SessionGuard } from '../auth/session.guard';
import type { AuthUser } from '../auth/session.guard';
import { CurrentUser } from '../auth/current-user.decorator';

/**
 * Route order matters: the specific `join-tokens` / `join/:token` handlers
 * are declared BEFORE `:id` so Express never matches "join" as an id.
 *
 * Every handler below is owner-scoped and guarded — except the join handlers,
 * which stay public so a visitor can redeem a share link (the created
 * character is attributed to the link's creator).
 */
@Controller('characters')
export class CharactersController {
  constructor(private readonly characters: CharactersRepository) {}

  @Post()
  @UseGuards(SessionGuard)
  async create(@Body() dto: CreateCharacterDto, @CurrentUser() user: AuthUser) {
    return this.characters.create(dto, user.id);
  }

  @Get()
  @UseGuards(SessionGuard)
  async findAll(
    @CurrentUser() user: AuthUser,
    @Query('type') type?: CharacterType,
  ) {
    return this.characters.findAllForUser(user.id, type);
  }

  @Post('join-tokens')
  @UseGuards(SessionGuard)
  async createJoinToken(@CurrentUser() user: AuthUser) {
    return this.characters.createJoinToken(user.id);
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
    // The redeemed character belongs to the invite creator; forcedType pins
    // it to 'player' — any client-sent type in the body is ignored.
    return this.characters.create(dto, joinToken.createdBy, 'player');
  }

  @Get(':id')
  @UseGuards(SessionGuard)
  async findById(@Param('id') id: string, @CurrentUser() user: AuthUser) {
    const character = await this.characters.findByIdForUser(id, user.id);
    if (!character) {
      throw new NotFoundException('Character not found');
    }
    return character;
  }

  @Patch(':id')
  @UseGuards(SessionGuard)
  async update(
    @Param('id') id: string,
    @Body() dto: UpdateCharacterDto,
    @CurrentUser() user: AuthUser,
  ) {
    const updated = await this.characters.updateByIdForUser(id, user.id, dto);
    if (!updated) {
      throw new NotFoundException('Character not found');
    }
    return updated;
  }

  @Delete(':id')
  @HttpCode(204)
  @UseGuards(SessionGuard)
  async delete(
    @Param('id') id: string,
    @CurrentUser() user: AuthUser,
  ): Promise<void> {
    const deleted = await this.characters.deleteByIdForUser(id, user.id);
    if (!deleted) {
      throw new NotFoundException('Character not found');
    }
  }
}
