import { Module } from '@nestjs/common';
import { RepositoriesModule } from '../db/repositories.module';
import { AuthModule } from '../auth/auth.module';
import { MapsController } from './maps.controller';

@Module({
  imports: [RepositoriesModule, AuthModule],
  controllers: [MapsController],
})
export class MapsModule {}
