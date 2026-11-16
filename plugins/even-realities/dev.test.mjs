import test from 'node:test';
import assert from 'node:assert/strict';
import {spawn} from 'node:child_process';
import {fileURLToPath} from 'node:url';
import {resolve,dirname} from 'node:path';
import {once} from 'node:events';
import {tmpdir} from 'node:os';
const root=dirname(fileURLToPath(import.meta.url));
test('development server builds from any cwd and serves only preview assets',{timeout:20000},async()=>{
 const child=spawn(process.execPath,[resolve(root,'dev.mjs'),'--port','0','--server','http://127.0.0.1:2806'],{cwd:tmpdir(),stdio:['ignore','pipe','pipe']});
 let log='',errors='';
 child.stdout.on('data',data=>{log+=data;});child.stderr.on('data',data=>{errors+=data;});
 try{
  let base;
  for(let i=0;i<200;i++){const match=log.match(/http:\/\/127\.0\.0\.1:(\d+)/);if(match){base=match[0];break;}if(child.exitCode!==null)throw Error(errors+log);await new Promise(r=>setTimeout(r,50));}
  assert.ok(base,errors+log);
  const page=await fetch(base);assert.equal(page.status,200);const html=await page.text();assert.match(html,/settings-connection/);assert.match(html,/__dev_reload/);
  const script=await fetch(base+'/bundle.js');assert.equal(script.status,200);assert.match(await script.text(),/127\.0\.0\.1:2806/);
  for(const name of ['mode-ai','mode-translate','status-idle','status-listening','status-transcribing']){const image=await fetch(base+'/assets/hud-icons/'+name+'.png');assert.equal(image.status,200);const bytes=Buffer.from(await image.arrayBuffer());assert.equal(bytes.readUInt32BE(16),24);assert.equal(bytes.readUInt32BE(20),24);}
  assert.equal((await fetch(base+'/style.css')).status,200);assert.equal((await fetch(base+'/assets/hud-icons/glasses-mic-on.png')).status,200);
  const manifest=await (await fetch(base+'/app.json')).json();assert.equal(manifest.permissions[0].whitelist[0],'http://127.0.0.1:2806');
  for(const path of ['/.env.eval.local','/bundle/config.json','/package.json','/plugin.js','/%2e%2e%2fpackage.json']){
   const response=await fetch(base+path);assert.ok([403,404].includes(response.status),path);
  }
  const controller=new AbortController();const live=await fetch(base+'/__dev_reload',{signal:controller.signal});assert.equal(live.headers.get('content-type'),'text/event-stream');
  const read=await live.body.getReader().read();assert.match(new TextDecoder().decode(read.value),/connected/);controller.abort();
 }finally{child.kill('SIGTERM');await once(child,'exit');}
});
test('invalid port exits with a useful error',{timeout:10000},async()=>{
 const child=spawn(process.execPath,[resolve(root,'dev.mjs'),'--port','not-a-port'],{stdio:['ignore','pipe','pipe']});let error='';
 child.stderr.on('data',data=>{error+=data;});const [code]=await once(child,'exit');assert.notEqual(code,0);assert.match(error,/포트/);
});
