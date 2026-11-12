import { execFileSync } from 'node:child_process';
import { fileURLToPath } from 'node:url';

const serverUrl = process.argv[2];
if (!serverUrl) {
  console.error('사용법: npm run pack -- https://게이트웨이주소:8080');
  process.exit(1);
}
execFileSync(process.execPath, ['build.mjs', '--server', serverUrl], { stdio: 'inherit' });
execFileSync(fileURLToPath(new URL('./node_modules/.bin/evenhub', import.meta.url)), [
  'pack', 'dist/app.json', 'dist', '--sdk-ver', '0.0.16', '-o', 'dkst-ai-gateway.ehpk',
], { stdio: 'inherit' });
