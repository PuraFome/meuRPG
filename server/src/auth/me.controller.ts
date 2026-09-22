import { Controller, Get } from '@nestjs/common';
import { CurrentUserId } from '../auth/current-user.decorator';

@Controller()
export class MeController {
  @Get('me')
  async me(@CurrentUserId() sub: string): Promise<{ sub: string }> {
    return { sub };
  }
}