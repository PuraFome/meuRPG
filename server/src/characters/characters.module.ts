import { Module } from '@nestjs/common';
import { RepositoriesModule } from '../db/repositories.module';
import { AuthModule } from '../auth/auth.module';
import { CharactersController } from './characters.controller';

@Module({
  imports: [RepositoriesModule, AuthModule],
  controllers: [CharactersController],
})
export class CharactersModule {}
