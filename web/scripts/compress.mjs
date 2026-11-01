// Writes a Brotli (.br) and a gzip (.gz) copy next to every .js and .css file of the production build.
// The Go server (backend/internal/platform/httpserver/static.go) serves them to clients that accept them.
// A copy is kept only when it is smaller than the original.
// index.html is never compressed.
import { brotliCompressSync, constants, gzipSync } from 'node:zlib';
import { existsSync, readdirSync, readFileSync, rmSync, writeFileSync } from 'node:fs';
import { dirname, extname, join, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

const root = resolve(dirname(fileURLToPath(import.meta.url)), '..', 'dist', 'web', 'browser');
const extensions = new Set(['.js', '.css']);

if (!existsSync(root)) {
  console.error(
    `compress.mjs: ${root} does not exist. The Angular build did not write its output; run it first (npm run build) and read its errors.`,
  );
  process.exit(1);
}

const variants = [
  {
    suffix: '.br',
    encode: (data) =>
      brotliCompressSync(data, {
        params: {
          [constants.BROTLI_PARAM_QUALITY]: constants.BROTLI_MAX_QUALITY,
          [constants.BROTLI_PARAM_SIZE_HINT]: data.length,
        },
      }),
  },
  { suffix: '.gz', encode: (data) => gzipSync(data, { level: 9 }) },
];

function* walk(dir) {
  for (const entry of readdirSync(dir, { withFileTypes: true })) {
    const path = join(dir, entry.name);
    if (entry.isDirectory()) {
      yield* walk(path);
    } else if (entry.isFile() && extensions.has(extname(entry.name))) {
      yield path;
    }
  }
}

for (const file of walk(root)) {
  const data = readFileSync(file);
  for (const { suffix, encode } of variants) {
    const encoded = encode(data);
    if (encoded.length < data.length) {
      writeFileSync(file + suffix, encoded);
    } else {
      rmSync(file + suffix, { force: true });
    }
  }
}
