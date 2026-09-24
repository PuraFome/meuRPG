import { Controller, Get, UseGuards } from '@nestjs/common';
import { SessionGuard } from './session.guard';
import { CurrentUser } from './current-user.decorator';
import type { AuthUser } from './session.guard';

@Controller()
@UseGuards(SessionGuard)
export class MeController {
  @Get('me')
  async me(@CurrentUser() user: AuthUser): Promise<{
    sub: string;
    email: string;
    name: string;
  }> {
    return { sub: user.sub, email: user.email, name: user.name };
  }
}
