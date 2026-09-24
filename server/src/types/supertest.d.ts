/**
 * supertest 7.x ships no bundled type declarations and @types/supertest is
 * not installed (adding dependencies is out of scope for this task). This
 * shorthand ambient module makes the import `any` so `tsc --noEmit` stays
 * green under `strict`/`noImplicitAny`.
 */
declare module 'supertest';
