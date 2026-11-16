import { build } from 'esbuild';
import { createHash } from 'node:crypto';
import { copyFile, cp, mkdir, readFile, writeFile } from 'node:fs/promises';

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
await cp('assets/hud-icons','dist/assets/hud-icons',{recursive:true});
for (const file of ['index.html', 'style.css', 'bundle.js']) {
  await copyFile(file, `dist/${file}`);
}
const manifest = JSON.parse(await readFile('app.json', 'utf8'));
// A repack at the same version can still change assets. Fingerprint both files
// so webviews never reuse the previous JS/CSS under an unchanged entry URL.
let html=await readFile('index.html','utf8');
for(const [attribute,file] of [['href','style.css'],['src','bundle.js']]){
  const hash=createHash('sha256').update(await readFile(file)).digest('hex').slice(0,12);
  const pattern=new RegExp(`${attribute}="${file.replace('.', '[.]')}(?:[?][^"]*)?"`);
  html=html.replace(pattern,`${attribute}="${file}?v=${manifest.version}-${hash}"`);
}
html=html.replace(/id="plugin-build-version">[^<]*/,'id="plugin-build-version">'+manifest.version);
await writeFile('dist/index.html',html);
if (serverUrl) manifest.permissions.find(p => p.name === 'network').whitelist = [serverUrl];
await writeFile('dist/app.json', JSON.stringify(manifest, null, 2) + '\n');
console.log('G2 bundle ready: bundle.js / dist/');
