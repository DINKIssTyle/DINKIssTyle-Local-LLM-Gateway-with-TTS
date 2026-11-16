import {HUD_ICONS} from './hud-icons.mjs';
import {context} from 'esbuild';
import {createServer} from 'node:http';
import {readFile,writeFile,copyFile,cp,mkdir,stat} from 'node:fs/promises';
import {watchFile,unwatchFile} from 'node:fs';
import {dirname,resolve,sep,extname} from 'node:path';
import {fileURLToPath} from 'node:url';

const root=dirname(fileURLToPath(import.meta.url)),out=resolve(root,'.dev');
const args=process.argv.slice(2);
function option(name,fallback){const i=args.indexOf(name);if(i<0)return fallback;if(!args[i+1] || args[i+1].startsWith('--'))throw Error(name+' 값을 입력하세요');return args[i+1];}
for(let i=0;i<args.length;i+=2)if(!['--host','--port','--server'].includes(args[i]))throw Error('지원하지 않는 옵션: '+args[i]);
const host=option('--host','127.0.0.1'),port=Number(option('--port',process.env.PORT || '5173'));
if(!Number.isInteger(port) || port<0 || port>65535)throw Error('포트는 0~65535 사이의 숫자여야 합니다');
let gateway=option('--server',process.env.DKST_GATEWAY_URL || '');
if(gateway){const url=new URL(gateway);if(!['http:','https:'].includes(url.protocol)||url.username||url.password||url.pathname!=='/'||url.search||url.hash)throw Error('--server에는 http(s)://호스트:포트 주소를 입력하세요');gateway=url.origin;}
await mkdir(out,{recursive:true});
let closing=false;
const clients=new Set();const reload=()=>{for(const client of clients)client.write('data: reload\n\n');};
async function copyPage(){
 const html=await readFile(resolve(root,'index.html'),'utf8');
 const live='<script>new EventSource("/__dev_reload").onmessage=()=>location.reload();</script>';
 await writeFile(resolve(out,'index.html'),html.replace('</body>',live+'</body>'));
}
async function copyManifest(){
 const manifest=JSON.parse(await readFile(resolve(root,'app.json'),'utf8'));
 if(gateway)manifest.permissions.find(p=>p.name==='network').whitelist=[gateway];
 await writeFile(resolve(out,'app.json'),JSON.stringify(manifest,null,2)+'\n');
}
await Promise.all([copyPage(),copyManifest(),copyFile(resolve(root,'style.css'),resolve(out,'style.css')),cp(resolve(root,'assets/hud-icons'),resolve(out,'assets/hud-icons'),{recursive:true})]);
let initialBuild=true;
const build=await context({absWorkingDir:root,entryPoints:['plugin.js'],bundle:true,outfile:resolve(out,'bundle.js'),format:'iife',platform:'browser',target:['es2020'],sourcemap:true,
 define:{DKST_DEFAULT_SERVER_URL:JSON.stringify(gateway)},plugins:[{name:'reload',setup(api){api.onEnd(result=>{if(result.errors.length)return;if(initialBuild)initialBuild=false;else reload();});}}]});
await build.rebuild();await build.watch();
const watched=[];
function watch(name,copy){
 const path=resolve(root,name);watched.push(path);
 watchFile(path,{interval:300},async(current,previous)=>{if(current.mtimeMs===previous.mtimeMs)return;try{await copy();reload();}catch(error){console.error('다시 불러오기 실패:',error.message);}});
}
watch('index.html',copyPage);watch('style.css',()=>copyFile(resolve(root,'style.css'),resolve(out,'style.css')));watch('app.json',copyManifest);
for(const name of Object.keys(HUD_ICONS))watch('assets/hud-icons/'+name+'.png',()=>copyFile(resolve(root,'assets/hud-icons',name+'.png'),resolve(out,'assets/hud-icons',name+'.png')));
const mime={'.html':'text/html; charset=utf-8','.js':'text/javascript; charset=utf-8','.css':'text/css; charset=utf-8','.json':'application/json; charset=utf-8','.map':'application/json','.png':'image/png'};
const server=createServer(async(req,res)=>{
 let pathname;try{pathname=decodeURIComponent(new URL(req.url,'http://localhost').pathname);}catch{res.writeHead(400).end();return;}
 if(pathname==='/__dev_reload'){
  res.writeHead(200,{'Content-Type':'text/event-stream','Cache-Control':'no-cache','Connection':'keep-alive'});res.write(': connected\n\n');clients.add(res);req.on('close',()=>clients.delete(res));return;
 }
 if(req.method!=='GET' && req.method!=='HEAD'){res.writeHead(405).end();return;}
 const path=resolve(out,'.'+(pathname==='/'?'/index.html':pathname));
 if(!path.startsWith(out+sep)){res.writeHead(403).end();return;}
 try{if(!(await stat(path)).isFile())throw Error();const data=await readFile(path);res.writeHead(200,{'Content-Type':mime[extname(path)] || 'application/octet-stream','Cache-Control':'no-store','X-Content-Type-Options':'nosniff'});res.end(req.method==='HEAD'?undefined:data);}
 catch{res.writeHead(404,{'Content-Type':'text/plain; charset=utf-8'}).end('찾을 수 없는 파일입니다');}
});
server.on('error',error=>{console.error('개발 서버 시작 실패:',error.message);shutdown(1);});
await new Promise((yes,no)=>{server.once('error',no);server.listen(port,host,yes);});
const address=server.address();
console.log('Even 플러그인 개발 화면: http://'+(host==='0.0.0.0'?'localhost':host)+':'+address.port);
console.log(gateway?'게이트웨이: '+gateway:'서버 연결은 설정에서 입력하거나 --server http://호스트:포트 옵션으로 지정하세요.');
console.log('JS·HTML·CSS·아이콘 변경 시 자동으로 새로고침합니다. 종료: Ctrl+C');
async function shutdown(code=0){if(closing)return;closing=true;for(const path of watched)unwatchFile(path);for(const client of clients)client.end();server.close();await build.dispose();process.exit(code);}
process.on('SIGINT',()=>shutdown());process.on('SIGTERM',()=>shutdown());
