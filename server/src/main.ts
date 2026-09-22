import 'reflect-metadata';
import { NestFactory } from '@nestjs/core';
import { AppModule } from './app.module';
import helmet from 'helmet';
import { ValidationPipe } from '@nestjs/common';

async function bootstrap() {
  const app = await NestFactory.create(AppModule);

  app.setGlobalPrefix('api');

  // Render terminates TLS upstream, so Express sees plain HTTP. Without
  // trusting the proxy, express-session with secure cookies silently drops
  // Set-Cookie and every OAuth handshake 403s at the state check.
  app.getHttpAdapter().getInstance().set('trust proxy', 1);

  app.getHttpAdapter().getInstance().disable('x-powered-by');
  app.use(
    helmet({
      contentSecurityPolicy: {
        useDefaults: false,
        directives: {
          defaultSrc: ["'none'"],
          frameAncestors: ["'none'"],
          baseUri: ["'none'"],
        },
      },
      strictTransportSecurity: {
        maxAge: 31536000,
        includeSubDomains: true,
      },
      xContentTypeOptions: true,
      referrerPolicy: { policy: 'no-referrer' },
      xPoweredBy: true,
    }),
  );

  const frontendOrigin = process.env.FRONTEND_ORIGIN;
  if (!frontendOrigin) {
    throw new Error('FRONTEND_ORIGIN is not configured');
  }
  const corsOrigins =
    process.env.NODE_ENV === 'production'
      ? [frontendOrigin]
      : [frontendOrigin, 'http://localhost:4200'];
  app.enableCors({ origin: corsOrigins });

  app.useGlobalPipes(
    new ValidationPipe({
      whitelist: true,
      forbidNonWhitelisted: true,
      transform: true,
    }),
  );

  const port = process.env.API_PORT || 3000;
  await app.listen(port);
  console.log(`listening on ${port}`);
}
bootstrap();