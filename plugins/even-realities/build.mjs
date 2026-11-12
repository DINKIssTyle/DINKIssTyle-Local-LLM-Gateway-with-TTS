import { build } from 'esbuild';
import { copyFile, mkdir, readFile, writeFile } from 'node:fs/promises';

const serverIndex = process.argv.indexOf('--server');
let serverUrl = '';
if (serverIndex !== -1) {
  const url = new URL(process.argv[serverIndex + 1]);
  if (!['http:', 'https:'].includes(url.protocol) || url.username || url.password ||
      url.pathname !== '/' || url.search || url.hash) {
    throw new Error('--server에는 http(s)://호스트:포트 형식의 서버 주소를 입력하세요.');
  }
  serverUrl = url.origin;
}

await build({
  entryPoints: ['plugin.js'],
  bundle: true,
  outfile: 'bundle.js',
  format: 'iife',
  platform: 'browser',
  target: ['es2020'],
  define: { DKST_DEFAULT_SERVER_URL: JSON.stringify(serverUrl) },
});
await mkdir('dist', { recursive: true });
for (const file of ['index.html', 'style.css', 'bundle.js']) {
  await copyFile(file, `dist/${file}`);
}
const manifest = JSON.parse(await readFile('app.json', 'utf8'));
if (serverUrl) manifest.permissions.find(p => p.name === 'network').whitelist = [serverUrl];
await writeFile('dist/app.json', JSON.stringify(manifest, null, 2) + '\n');
console.log('G2 bundle ready: bundle.js / dist/');
