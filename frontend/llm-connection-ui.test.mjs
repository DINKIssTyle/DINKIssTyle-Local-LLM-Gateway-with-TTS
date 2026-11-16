import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import vm from 'node:vm';
import test from 'node:test';
class Element {
 constructor(tag){this.tag=tag;this.children=[];this.style={};this.dataset={};this.value='';this.checked=false;}
 appendChild(child){this.children.push(child);return child;}
 append(...children){this.children.push(...children);}
 replaceChildren(...children){this.children=[...children];}
 setAttribute(name,value){this[name]=value;}
 focus(){}
 querySelector(id){return this.ids?.[id] || null;}
}
const elements=(root,tag)=>root.children.flatMap(child=>child && typeof child==='object'?[...(child.tag===tag?[child]:[]),...elements(child,tag)]:[]);
function setup(lang='ko'){const created=[],window={},document={documentElement:{lang},createElement:tag=>{const e=new Element(tag);created.push(e);return e;},createTextNode:text=>({textContent:text,children:[]}),querySelectorAll:selector=>created.filter(e=>({ '[data-llm-i18n]':e.dataset.llmI18n,'[data-llm-placeholder]':e.dataset.llmPlaceholder,'[data-llm-title]':e.dataset.llmTitle })[selector])};vm.runInNewContext(readFileSync(new URL('./llm-connection-ui.js',import.meta.url),'utf8'),{window,document,crypto:{randomUUID:()=> 'new-connection'}});return {document,ui:window.DKSTConnectionUI,root:new Element('div'),editor:Object.assign(new Element('div'),{ids:{'#llm-connection-editor':new Element('div'),'#llm-connection-edit-title':new Element('div'),'#llm-connection-editor-save':new Element('button')}})};}
test('compact list opens one editor; preserves keys when omitted and persists multiple connections',async()=>{
 const {ui,root,editor}=setup();let rows=[{id:'first',name:'First',endpoint:'https://one.example/v1',mode:'standard',has_api_key:true},{id:'second',name:'Second',endpoint:'https://two.example/v1',mode:'standard'}],saved=[];
 await ui.mountConnections(root,{editor,load:async()=>rows,save:async next=>{saved.push(next);rows=next;}});
 assert.equal(elements(root,'input').length,0,'list must not expand every connection form');assert.equal(root.children[0].children.length,2);
 root.children[0].children[0].children[0].onclick();assert.equal(editor.style.display,'flex');
 let inputs=elements(editor.ids['#llm-connection-editor'],'input');assert.equal(inputs.find(i=>i.type==='password').value,'');assert.match(inputs.find(i=>i.type==='password').placeholder,/등록됨/);
 inputs[0].value='Renamed';await root.saveConnectionEditor();assert.equal(saved[0][0].name,'Renamed');assert.equal(Object.hasOwn(saved[0][0],'api_key'),false,'blank edit never clears a stored key');assert.equal(saved[0].length,2);assert.equal(editor.style.display,'none');
 root.addConnection();inputs=elements(editor.ids['#llm-connection-editor'],'input');inputs[0].value='Third';inputs[1].value='https://three.example/v1';inputs.find(i=>i.type==='password').value='new-secret';await root.saveConnectionEditor();assert.equal(rows.length,3);assert.equal(rows[2].api_key,'new-secret');assert.equal(rows[2].name,'Third');
});
test('canceling connection deletion does not save; keys UI patches only explicit input',async()=>{
 const {ui,root,editor}=setup();let saves=0;
 await ui.mountConnections(root,{editor,load:async()=>[{id:'one',name:'One'}],save:async()=>saves++,confirmRemove:async()=>false});
 await root.children[0].children[0].children[1].onclick();assert.equal(saves,0);
 let patch;
 await ui.mountKeys(root,{load:async()=>({connections:[{id:'one',name:'One',has_api_key:true},{id:'two',name:'Two'}]}),save:async keys=>{patch=keys;}});
 const inputs=elements(root,'input');inputs.find(i=>i.type==='password').value='private-key';await root.saveConnectionKeys();assert.equal(patch.one,'private-key');assert.equal(Object.hasOwn(patch,'two'),false);assert.ok(inputs.filter(i=>i.type==='password').every(i=>i.value===''));
});
test('native settings uses existing LLM card button and account-style list/editor modals',()=>{
 const html=readFileSync(new URL('./index.html',import.meta.url),'utf8');
 const card=html.slice(html.indexOf('<!-- LLM Settings -->'),html.indexOf('<!-- Startup Settings -->'));
 assert.match(card,/openAdditionalLLMModal/);assert.doesNotMatch(card,/server-llm-connections/,'forms must not live in main dashboard card');
 const modal=html.slice(html.indexOf('id="additional-llm-modal"'),html.indexOf('id="additional-llm-edit-modal"'));
 assert.match(modal,/max-width:520px/);assert.match(modal,/server-llm-connections/);assert.match(modal,/LLM 추가/);assert.match(modal,/justify-content:space-between/);
});

test('Korean/English labels switch without replacing unsaved fields or user connection names',async()=>{
 const {ui,root,editor,document}=setup('en');
 await ui.mountConnections(root,{editor,load:async()=>[{id:'office',name:'회사 연결',endpoint:'https://office.example'}],save:async()=>{}});
 assert.equal(root.children[0].children[0].children[0].children[0].textContent,'회사 연결');
 root.addConnection();const form=editor.ids['#llm-connection-editor'];
 assert.ok(elements(form,'span').some(e=>e.textContent==='Connection name'));
 const input=elements(form,'input')[0];input.value='Unsaved name';document.documentElement.lang='ko';ui.refreshI18n();
 assert.ok(elements(form,'span').some(e=>e.textContent==='연결 이름'));assert.equal(input.value,'Unsaved name');
 await ui.mountKeys(root,{load:async()=>[],save:async()=>{},hideWhenEmpty:true});assert.equal(root.hidden,true);assert.equal(root.style.display,'none');
});
test('keys are managed only in server accounts and the default-key help precedes additional keys',()=>{
 const web=readFileSync(new URL('./web.html',import.meta.url),'utf8'),plugin=readFileSync(new URL('../plugins/even-realities/index.html',import.meta.url),'utf8'),native=readFileSync(new URL('./index.html',import.meta.url),'utf8');
 assert.ok(!web.includes('cfg-llm-keys'));assert.ok(!plugin.includes('additional-llm-keys'));
 assert.ok(native.indexOf('data-i18n="edit.help.apiKey"')<native.indexOf('id="edit-user-llm-keys"'));
 const section=native.slice(native.indexOf('<!-- API Key Section -->'),native.indexOf('<!-- Memory Retention Section -->'));
 assert.ok(!section.includes('margin-top: -8px'));
});

test('switching accounts while keys load cannot leave the previous account save callback active',async()=>{
 const {ui,root}=setup();let finishOld,oldWrites=0,newWrites=0;
 const old=ui.mountKeys(root,{load:()=>new Promise(resolve=>{finishOld=()=>resolve([{id:'old',name:'Old account'}]);}),save:async()=>oldWrites++});
 await ui.mountKeys(root,{load:async()=>[{id:'new',name:'New account'}],save:async()=>newWrites++});finishOld();await old;
 const fields=elements(root,'input');fields.find(e=>e.type==='password').value='new-key';await root.saveConnectionKeys();
 assert.equal(oldWrites,0);assert.equal(newWrites,1);
});
