import test from 'node:test';
import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import vm from 'node:vm';

class Element {
 constructor(){this.innerHTML='';this.textContent='';this.dataset={};this.value='';this.events={};this.classes=new Set();this.classList={add:v=>this.classes.add(v),remove:v=>this.classes.delete(v),contains:v=>this.classes.has(v)};}
 addEventListener(type,fn){this.events[type]=fn;}
}
const day={date:'2026-11-21',count:2,startedAt:'2026-11-20T15:00:00Z',endedAt:'2026-11-21T08:00:00Z',preview:'<img src=x onerror=alert(1)> 원문'};
function setup(fetch){
 const list=new Element(),input=new Element(),ids=Object.fromEntries(['transcript-day-modal','transcript-day-title','transcript-day-body'].map(id=>[id,new Element()]));
 let user={id:'alice'},confirm=true,timer;
 const requests=[],toasts=[];
 const window={document:{getElementById:id=>ids[id],addEventListener(){}},Intl,confirm:()=>confirm,navigator:{clipboard:{writeText:async text=>{window.copied=text;}}},setTimeout:fn=>(timer=fn,1),fetch:async(url,options)=>{requests.push({url,options});return fetch(url,options);}};
 vm.runInNewContext(readFileSync(new URL('./app-transcript-library.js',import.meta.url),'utf8'),{window,URLSearchParams,AbortController,clearTimeout});
 const escape=text=>String(text).replaceAll('&','&amp;').replaceAll('<','&lt;').replaceAll('>','&gt;').replaceAll('"','&quot;');
 const controller=window.DKSTTranscriptLibrary.createTranscriptLibrary({list,searchInput:input,getCurrentUser:()=>user,t:key=>key,escapeHtml:escape,escapeAttr:escape,showToast:(...args)=>toasts.push(args),fallbackCopyTextToClipboard(){}});
 const click=async(selector,dataset)=>{const target={closest:value=>value===selector?{dataset}:null};return list.events.click({target,stopPropagation(){}});};
 return {controller,list,input,ids,requests,toasts,window,click,flush:()=>timer(),setUser:value=>user=value,setConfirm:value=>confirm=value};
}
const response=data=>({ok:true,json:async()=>data});
const tick=()=>new Promise(resolve=>setImmediate(resolve));

test('daily preview escapes source text; opening reads chronological plain text and copies the full day',async()=>{
 const ui=setup(async url=>url.includes('transcript-days')?response({items:[day],next:''}):response({entries:[{startedAt:day.startedAt,text:'<script>原文</script>\n다음 줄'}]}));
 ui.controller.setActive(true);await tick();
 assert.match(ui.list.innerHTML,/&lt;img/);assert.doesNotMatch(ui.list.innerHTML,/<img src=x/);
 await ui.click('[data-transcript-open]',{transcriptOpen:day.date});await tick();
 assert.ok(ui.ids['transcript-day-modal'].classes.has('active'));
 assert.match(ui.ids['transcript-day-body'].textContent,/<script>原文<\/script>\n다음 줄/);
 await ui.ids['transcript-day-modal'].events.click({target:{closest:key=>key==='[data-transcript-copy]'?{}:null}});
 assert.equal(ui.window.copied,ui.ids['transcript-day-body'].textContent);
});

test('search waits for input and ignores a stale response that finishes after the new query',async()=>{
 let release;
 const ui=setup(async url=>new URL(url,'http://test').searchParams.get('q')==='new'?response({items:[{...day,preview:'new result'}],next:''}):new Promise(resolve=>release=()=>resolve(response({items:[day],next:''}))));
 ui.controller.setActive(true);
 ui.controller.search('new');await ui.flush();release();await tick();
 assert.match(ui.list.innerHTML,/new result/);assert.doesNotMatch(ui.list.innerHTML,/onerror/);
});

test('delete requires confirmation, scopes date/timezone, keeps failures and refreshes successes',async()=>{
 let removed=false,fail=true;
 const ui=setup(async(url,options)=>{
  if(options.method==='DELETE'){
   const query=new URL(url,'http://test').searchParams;assert.equal(query.get('date'),day.date);assert.ok(query.get('tz'));
   if(fail)return {ok:false,status:500};removed=true;return response({deleted:2});
  }
  return response({items:removed?[]:[day],next:''});
 });
 ui.controller.setActive(true);await tick();ui.setConfirm(false);
 await ui.click('[data-transcript-delete]',{transcriptDelete:day.date});await tick();assert.equal(ui.requests.length,1);
 ui.setConfirm(true);await ui.click('[data-transcript-delete]',{transcriptDelete:day.date});await tick();
 assert.match(ui.list.innerHTML,/2026/);assert.equal(ui.toasts.at(-1)[1],true);
 fail=false;await ui.click('[data-transcript-delete]',{transcriptDelete:day.date});await tick();
 assert.match(ui.list.innerHTML,/transcripts.empty/);assert.equal(removed,true);
});

test('account switch and closing the library invalidate late detail results',async()=>{
 let release;
 const ui=setup(async url=>url.includes('transcript-days')?response({items:[day],next:''}):new Promise(resolve=>release=()=>resolve(response({entries:[{startedAt:day.startedAt,text:'alice private text'}]}))));
 ui.controller.setActive(true);await tick();await ui.click('[data-transcript-open]',{transcriptOpen:day.date});
 ui.setUser({id:'bob'});ui.controller.close();release();await tick();
 assert.equal(ui.ids['transcript-day-body'].textContent,'');assert.ok(!ui.ids['transcript-day-modal'].classes.has('active'));
});

test('saved-turn search and transcript search stay independent across tab changes and saved-title refreshes',async()=>{
 const ui=setup(async()=>response({items:[day],next:''}));
 const savedTab=new Element(),transcriptTab=new Element();
 savedTab.dataset.libraryTab='saved';transcriptTab.dataset.libraryTab='transcripts';
 for(const element of [savedTab,transcriptTab,ui.list])element.setAttribute=(key,value)=>{element[key]=value;};
 ui.window.document.querySelectorAll=()=>[savedTab,transcriptTab];
 ui.window.document.querySelector=()=>savedTab;
 vm.runInNewContext(readFileSync(new URL('./app-saved-library.js',import.meta.url),'utf8'),{window:ui.window});
 const controller=ui.window.DKSTSavedLibrary.createSavedLibraryController({refs:{savedLibraryList:ui.list,savedLibrarySearchInput:ui.input},deps:{transcripts:ui.controller,t:key=>key,escapeHtml:String,escapeAttr:String}});
 controller.handleSavedLibrarySearch('saved needle');
 transcriptTab.events.click();await tick();
 controller.handleSavedLibrarySearch('speech needle');await ui.flush();
 controller.renderSavedLibraryList();assert.match(ui.list.innerHTML,/2026/);
 savedTab.events.click();assert.equal(ui.input.value,'saved needle');assert.equal(ui.input.placeholder,'library.searchPlaceholder');
 transcriptTab.events.click();assert.equal(ui.input.value,'speech needle');assert.equal(ui.list['aria-labelledby'],'library-transcripts-tab');
 await tick();
});
