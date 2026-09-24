import { Module } from '@nestjs/common';
import { AuthController } from './auth.controller';
import { MeController } from './me.controller';
import { SessionGuard } from './session.guard';
import { RepositoriesModule } from '../db/repositories.module';

@Module({
  imports: [RepositoriesModule],
  controllers: [AuthController, MeController],
  providers: [SessionGuard],
  exports: [SessionGuard],
})
export class AuthModule {}
