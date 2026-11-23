import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import vm from 'node:vm';
import test from 'node:test';
class Element {
 constructor(tag){this.tag=tag;this.children=[];this.style={setProperty(name,value){this[name]=value;}};this.value='';this.checked=false;this.disabled=false;}
 append(...children){this.children.push(...children);}
 replaceChildren(...children){this.children=[...children];}
 setAttribute(name,value){this[name]=value;}
}
const elements=(root,tag)=>root.children.flatMap(e=>e&&typeof e==='object'?[...(e.tag===tag?[e]:[]),...elements(e,tag)]:[]);
function setup(){const window={},document={documentElement:{lang:'ko'},createElement:tag=>new Element(tag)};vm.runInNewContext(readFileSync(new URL('./google-integration-ui.js',import.meta.url),'utf8'),{window,document,setTimeout,clearTimeout});return {ui:window.DKSTGoogleUI,root:new Element('div'),document};}
const initial=()=>({client_id:'desktop.apps.googleusercontent.com',has_client_secret:true,client_secret_length:35,services:['gmail'],authorized_services:['gmail'],connected:true,email:'alice@example.com',pending:false,message:''});
test('service changes save for the selected account; connect saves first; language changes preserve drafts',async()=>{
 const {ui,root,document}=setup();let status=initial();const calls=[];
 const api={GetUserGoogleStatus:async()=>status,SetUserGoogleSettings:async(user,settings)=>{calls.push(['save',user,settings]);status={...status,services:settings.services};},ConnectUserGoogle:async user=>calls.push(['connect',user]),DisconnectUserGoogle:async user=>{calls.push(['disconnect',user]);status={...status,connected:false};}};
 await ui.mount(root,{userId:'alice',api});
 const inputs=elements(root,'input'),checks=inputs.filter(i=>i.type==='checkbox');assert.equal(checks.length,3);assert.equal(checks[0].checked,true);
 checks[0].checked=false;await checks[0].onchange();assert.equal(calls[0][1],'alice');assert.equal(calls[0][2].services.length,0);assert.equal(calls[0][2].client_secret,'');
 const secret=inputs.find(i=>i.type==='password');assert.match(secret.placeholder,/^•+$/);assert.equal(secret.style['--password-dot-count'],35);assert.equal(secret.value,'');secret.value='draft-secret';document.documentElement.lang='en';root.refreshGoogleLanguage();assert.equal(secret.value,'draft-secret');assert.ok(elements(root,'input').includes(secret));
 await elements(root,'button').find(b=>b.textContent==='Connect / update permissions').onclick();assert.deepEqual(calls.slice(-2).map(c=>c[0]),['save','connect']);assert.equal(secret.value,'');
 root.disposeGoogle();
});
test('switching accounts ignores the previous account loading and removes its save callback',async()=>{
 const {ui,root}=setup();let finishOld,oldSaves=0,newSaves=0;
 const old=ui.mount(root,{userId:'alice',api:{GetUserGoogleStatus:()=>new Promise(resolve=>{finishOld=resolve;}),SetUserGoogleSettings:async()=>oldSaves++}});
 await ui.mount(root,{userId:'bob',api:{GetUserGoogleStatus:async()=>({...initial(),email:'bob@example.com'}),SetUserGoogleSettings:async()=>newSaves++}});
 finishOld(initial());await old;await root.saveGoogleSettings();assert.equal(oldSaves,0);assert.equal(newSaves,1);root.disposeGoogle();
});
test('failed service save restores the last server selection and propagates settings-save failures',async()=>{
 const {ui,root}=setup();await ui.mount(root,{userId:'alice',api:{GetUserGoogleStatus:async()=>initial(),SetUserGoogleSettings:async()=>{throw new Error('disk full');}}});
 const checks=elements(root,'input').filter(i=>i.type==='checkbox');checks[0].checked=false;await checks[0].onchange();assert.equal(checks[0].checked,true);assert.ok(elements(root,'p').some(p=>p.textContent==='disk full'));
 await assert.rejects(root.saveGoogleSettings(),/disk full/);root.disposeGoogle();
});
