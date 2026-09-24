import {
  Body,
  Controller,
  Delete,
  Get,
  HttpCode,
  NotFoundException,
  Param,
  Patch,
  Post,
  UseGuards,
} from '@nestjs/common';
import { MapsRepository } from '../db/maps.repository';
import { CreateMapDto, UpdateMapDto } from './maps.dto';
import { SessionGuard } from '../auth/session.guard';
import type { AuthUser } from '../auth/session.guard';
import { CurrentUser } from '../auth/current-user.decorator';

/** Owner-scoped CRUD for maps, mirroring the characters controller. */
@Controller('maps')
export class MapsController {
  constructor(private readonly maps: MapsRepository) {}

  @Post()
  @UseGuards(SessionGuard)
  async create(@Body() dto: CreateMapDto, @CurrentUser() user: AuthUser) {
    return this.maps.create(dto, user.id);
  }

  @Get()
  @UseGuards(SessionGuard)
  async findAll(@CurrentUser() user: AuthUser) {
    return this.maps.findAllForUser(user.id);
  }

  @Get(':id')
  @UseGuards(SessionGuard)
  async findById(@Param('id') id: string, @CurrentUser() user: AuthUser) {
    const map = await this.maps.findByIdForUser(id, user.id);
    if (!map) {
      throw new NotFoundException('Map not found');
    }
    return map;
  }

  @Patch(':id')
  @UseGuards(SessionGuard)
  async update(
    @Param('id') id: string,
    @Body() dto: UpdateMapDto,
    @CurrentUser() user: AuthUser,
  ) {
    const updated = await this.maps.updateByIdForUser(id, user.id, dto);
    if (!updated) {
      throw new NotFoundException('Map not found');
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
    const deleted = await this.maps.deleteByIdForUser(id, user.id);
    if (!deleted) {
      throw new NotFoundException('Map not found');
    }
  }
}
