import { copyFile, mkdir } from 'node:fs/promises';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';

const here = dirname(fileURLToPath(import.meta.url));
const root = dirname(here);
const src = join(root, 'node_modules', 'sql.js', 'dist', 'sql-wasm-browser.wasm');
const destDir = join(root, 'public', 'sqlite');
const dest = join(destDir, 'sql-wasm-browser.wasm');

try {
  await mkdir(destDir, { recursive: true });
  await copyFile(src, dest);
  console.log(`copied ${src} -> ${dest}`);
} catch (err) {
  if (err && err.code === 'ENOENT' && err.path === src) {
    console.warn(`skip: ${src} not present (run npm install sql.js first)`);
    process.exit(0);
  }
  throw err;
}