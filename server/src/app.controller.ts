import { Controller, Get } from '@nestjs/common';

@Controller()
export class AppController {
  @Get('health')
  async health(): Promise<{ status: string }> {
    return { status: 'ok' };
  }
}