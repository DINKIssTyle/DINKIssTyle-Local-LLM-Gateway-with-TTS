const assert = require('node:assert/strict');
const fs = require('node:fs');
const vm = require('node:vm');
const src = fs.readFileSync('plugin.js','utf8').replace(/^import .*\n/gm, '').replace('  function enqueueClip(', `  window.__queueTest={ai(){resetQueuesForModeSwitch('ai');config.glassesMode='ai';captureFailure='';isRecording=true;segmenter=new SpeechSegmenter();},hintGate,liveTranslations,enqueueClip,drainClips,stopVoiceRecording,reset(){resetQueuesForModeSwitch('transcript');config.glassesMode='transcript';},state(){return{ids:pendingClips.map(c=>c.segmentId),failure:captureFailure,recording:isRecording,engine:activeSttEngine,mode:config.glassesMode,active:activeUtterance?.id,previewBusy};},record(){isRecording=true;segmenter=new SpeechSegmenter();}};
  function enqueueClip(`);
const elements = new Map();
function element(id) {
  if (!elements.has(id)) elements.set(id, {value:'', checked:true, textContent:'', innerHTML:'', style:{}, hidden:true, focus(){}, handlers:{}, options:[], classList:{add(){},remove(){},toggle(){}}, addEventListener(type, fn){this.handlers[type]=fn;}, appendChild(opt){ this.options.push(opt); if(!this.value) this.value=opt.value; }});
  return elements.get(id);
}
let llmMode='standard', chatReply=null, cueReply=null, chromeText=null;
let nextTranscript = '테스트';let nextPreviewText=null;
let nextRejected = false;
let holdPreview = false;
let releasePreview = null;
let holdChat = false;
let releaseChat = null;
let holdWarmup=false, releaseWarmup=null;
let rejectAutoLang=false;
let sttReplies=[];
let hintDecision={hint:false,reason:'no_hint'};
let nativeSaved = '';
const documentHandlers = {};
let deviceHandler;
let handler; const audioCalls=[]; const updates=[]; const requests=[]; let startup; let shutdown;const rebuilds=[];
const bridge={async getDeviceInfo(){return{model:'g2',sn:'glasses-1',status:{sn:'glasses-1',connectType:'connected',batteryLevel:82}};},onDeviceStatusChanged(fn){deviceHandler=fn;return()=>{};},async getLocalStorage(){return nativeSaved;},async setLocalStorage(key,value){nativeSaved=value;return true;},async createStartUpPageContainer(p){startup=p;return 0;},async rebuildPageContainer(p){rebuilds.push(p);return true;},onEvenHubEvent(fn){handler=fn;},async textContainerUpgrade(p){updates.push(p);return true;},async audioControl(...args){audioCalls.push(args);return true;},async shutDownPageContainer(mode){shutdown=mode;return true;}};
const chunks=['data: {"choices":[{"delta":{"con','tent":"안녕"}}]}\n\ndata: {"choices":[{"delta":{"content":"하세요"}}]}\n\ndata: [DONE]\n\n'];
const context={DKST_DEFAULT_SERVER_URL:'https://192.168.1.4:2806',waitForEvenAppBridge:async()=>bridge,AudioInputSource:{Glasses:0},OsEventTypeList:{CLICK_EVENT:0,DOUBLE_CLICK_EVENT:3},console,TextDecoder,Uint8Array,URL,AbortController,setTimeout:(fn,ms)=>{const timer=setTimeout(fn,ms);if(ms>=1000)timer.unref();return timer;},clearTimeout,alert(){},localStorage:{getItem(){return null;},setItem(){}},window:{location:{origin:'https://192.168.1.4:2806',hostname:'192.168.1.4'},btoa:s=>Buffer.from(s,'binary').toString('base64')},document:{body:{style:{overflow:''}},activeElement:{focus(){}},addEventListener(type,fn){documentHandlers[type]=fn;},readyState:'complete',getElementById:element,createElement:()=>({})},fetch:async(url,opts={})=>{
 requests.push({url,opts});
 if(url.endsWith('/api/glasses/hint-decision')){if(hintDecision.error)return{ok:false,status:503};return{ok:true,json:async()=>hintDecision};}
 if(url.endsWith('/api/translate/chrome'))return chromeText?{ok:true,json:async()=>({status:'ready',text:chromeText,source:'en',target:'ko'})}:{ok:false,json:async()=>({status:'needs_prepare',error:'준비 필요'})};
 if(url.endsWith('/api/config'))return{ok:true,json:async()=>({llm_mode:llmMode,context_strategy:'retrieval'})};
 if(url.endsWith('/api/auth/check'))return{ok:true,json:async()=>({authenticated:opts.headers?.Authorization==='Bearer test-session'})};
 if(url.endsWith('/api/login'))return{ok:true,json:async()=>({token:'test-session'})};
 if(url.endsWith('/api/models'))return{ok:true,json:async()=>({models:[{key:'test-model',display_name:'Test Model'}]})};
 if(url.includes('/api/stt/transcripts?'))return{ok:true,json:async()=>({entries:[{startedAt:'2026-11-12T05:00:00Z',endedAt:'2026-11-12T05:00:01Z',text:'긴 전사 기록 '.repeat(10000)}]})};
 if(url.endsWith('/api/stt/warmup')){const response={ok:true,json:async()=>({ready:true,preview:true})};if(holdWarmup){holdWarmup=false;return await new Promise(resolve=>{releaseWarmup=()=>resolve(response);});}return response;}
 if(url.endsWith('/api/stt/preview')){
  const request=JSON.parse(opts.body), text=nextPreviewText ?? nextTranscript;
  const response={ok:true,status:200,json:async()=>({sessionId:request.sessionId,segmentId:request.segmentId,revision:request.revision,stage:'draft',text,engine:request.engine,stableText:'',draftText:text,saved:false,triggered:false})};
  if(holdPreview){holdPreview=false;return await new Promise(resolve=>{releasePreview=()=>resolve(response);});}
  return response;
 }
 if(url.endsWith('/api/stt/chrome/prepare'))return{ok:true,json:async()=>({status:'ready'})};
 if(url.endsWith('/api/stt/chrome/stop'))return{ok:true,json:async()=>({status:'ready'})};
 if(url.endsWith('/api/stt/status'))return{ok:true,json:async()=>({active:true})};
 if(url.endsWith('/api/stt/transcribe')) {
  const reqBody=JSON.parse(opts.body);
  if(sttReplies.length){const reply=sttReplies.shift();return typeof reply==='function'?await reply(opts):reply;}
  if(rejectAutoLang && reqBody.language==='auto') return {ok:false,status:500,text:async()=>'Transcription failed: Whisper: Unsupported language: auto'};
  const word=reqBody.triggerWord; const triggered=!!word && nextTranscript.includes(word);
  return{ok:true,json:async()=>({text:nextRejected?'':nextTranscript,saved:!nextRejected,rejected:nextRejected,triggered:nextRejected?false:triggered,prompt:triggered ? nextTranscript.split(word).slice(1).join(word).trim() : ''})};
 }
 if(url.endsWith('/api/chat')){if(JSON.parse(opts.body).glasses_hint_phase==='plan'){
 const content=cueReply || JSON.stringify({topic:'묵시적 갱신',question:'묵시적 갱신은 무엇을 뜻하나요?',knownFacts:['계약서에 묵시적 갱신이 나옴'],source:'internal'});
 let done=false;return{ok:true,body:{getReader:()=>({read:async()=>done?{done:true}:(done=true,{value:Buffer.from('data: '+JSON.stringify({type:'message.delta',content})+'\n\n'),done:false})})}};
 }if(chatReply){const reply=chatReply;chatReply=null;return reply;}if(holdChat){holdChat=false;await new Promise((resolve,reject)=>{const aborted=()=>{const err=new Error('aborted');err.name='AbortError';reject(err);};opts.signal?.addEventListener('abort',aborted,{once:true});releaseChat=()=>{opts.signal?.removeEventListener('abort',aborted);resolve();};});}let i=0;return{ok:true,body:{getReader:()=>({read:async()=>i<chunks.length?{value:Buffer.from(chunks[i++]),done:false}:{done:true}})}};}
 throw Error('Unexpected request '+url);
}};
const tick=()=>new Promise(r=>setImmediate(r));
(async()=>{
 Object.assign(context,await import('./live-translation.mjs'));
 Object.assign(context,await import('./chrome-translation.mjs'));
 Object.assign(context,await import('./speaker-filter.mjs'));
 Object.assign(context,await import('./translation-display.mjs'));
 Object.assign(context,await import('./direct-question.mjs'));
 Object.assign(context,await import('./hint-enrichment.mjs'));
 const {HintGate}=await import('./hint-gate.mjs');context.HintGate=HintGate;
 const {normalizeModels} = await import('./model-list.mjs');
 const {translationHeaderText,cleanHintText,clockText,glassesBatteryText,hudStatusText,hudNoticeText,animatedDots,withAnimatedDots,extractVisibleAndReasoning,stripSourcesForGlasses,formatToolStatusText,wrapHUDLines,sliceHUDWindow,normalizeGlassesMode,nextGlassesMode,glassesModeMenuIdToMode,targetLanguageLabel,glassesModeShortLabel,glassesModeBadgeText,buildModePrompts}=await import('./hud-status.mjs');
 Object.assign(context,{translationHeaderText,cleanHintText,hudStatusText,hudNoticeText,withAnimatedDots,extractVisibleAndReasoning,stripSourcesForGlasses,formatToolStatusText,sliceHUDWindow,normalizeGlassesMode,nextGlassesMode,glassesModeMenuIdToMode,targetLanguageLabel,glassesModeShortLabel,glassesModeBadgeText,buildModePrompts});
 assert.equal(normalizeGlassesMode('ai'),'ai');
 assert.equal(normalizeGlassesMode('translate'),'translate');
 assert.equal(normalizeGlassesMode('unknown'),'transcript');
 assert.equal(nextGlassesMode('transcript'),'ai');
 assert.equal(nextGlassesMode('ai'),'translate');
 assert.equal(nextGlassesMode('translate'),'transcript');
 assert.equal(glassesModeMenuIdToMode(1),'transcript');
 assert.equal(glassesModeMenuIdToMode(2),'ai');
 assert.equal(glassesModeMenuIdToMode(3),'translate');
 assert.equal(glassesModeBadgeText('translate','en'),'번역 · 추천 답변 (→영어)');
 assert.equal(cleanHintText('explain\n청동기 시대 설명'),'청동기 시대 설명');
 assert.equal(cleanHintText('## **background**: 피라미드 소개'),'피라미드 소개');
 assert.equal(cleanHintText('[recall]\n과거 약속'),'과거 약속');
 assert.equal(cleanHintText('본문의 explain 단어는 유지'),'본문의 explain 단어는 유지');
 const translatePrompts = buildModePrompts('translate',{targetLang:'ko',prompt:'Would you like some coffee?'});
 assert.match(translatePrompts.systemPrompt,/translation[\s\S]*positive[\s\S]*negative/);
 assert.match(translatePrompts.userPrompt,/Would you like some coffee\?/);
 assert.equal(hudNoticeText('전사 저장됨 · AI 호출어: 웬즈데이'),'전사 저장됨 · AI 호출어: 웬즈데이');
 assert.ok(hudNoticeText('전사 저장됨 · AI 호출어: '+ '긴호출어'.repeat(10)).endsWith('…'));
 assert.equal(animatedDots(0),'.');
 assert.equal(animatedDots(1),'..');
 assert.equal(animatedDots(2),'...');
 assert.equal(withAnimatedDots('생각 중...',0),'생각 중.');
 assert.equal(withAnimatedDots('모델 로딩 중',2),'모델 로딩 중...');
 assert.equal(
   stripSourcesForGlasses('<think>내부 생각</think>서울의 현재 기온은 18도입니다. [기상청](https://www.kma.go.kr) 참고 (출처: 기상청)\n\n검색 출처:\n- [날씨 안내](https://example.com/weather)\n- https://example.com/raw'),
   '서울의 현재 기온은 18도입니다. 기상청 참고'
 );
 assert.deepEqual(formatToolStatusText({type:'tool_call.arguments',tool:'web_search',arguments:{query:'서울 날씨'}}),{
   title:'웹 검색 중',
   detail:'검색어: 서울 날씨'
 });
 assert.deepEqual(formatToolStatusText({type:'tool_call.success',tool:'get_current_time'}),{
   title:'현재 시간을 확인했습니다.',
   detail:''
 });
 const wrappedWin = sliceHUDWindow('첫째 줄입니다.\n둘째 줄입니다.\n셋째 줄입니다.\n넷째 줄입니다.\n다섯째 줄입니다.\n여섯째 줄입니다.', 1, 4, 562);
 assert.equal(wrappedWin.totalLines, 6);
 assert.equal(wrappedWin.maxOffset, 2);
 assert.ok(wrappedWin.visibleText.includes('둘째 줄입니다.'));
 assert.ok(!wrappedWin.visibleText.includes('첫째 줄입니다.'));
 const {getTextWidth}=await import('@evenrealities/pretext');
 for(const sample of ['한글표시폭확인'.repeat(30),'Wi il MW 12345 '.repeat(30),'한국어 English 日本語 中文 '.repeat(20)]) {
   const lines=wrapHUDLines(sample,562);
   assert.ok(lines.length>1);
   for(const line of lines)assert.ok(getTextWidth(line)<=562,`HUD line overflow: ${getTextWidth(line)}`);
   assert.equal(lines.join('').replace(/\s/g,''),sample.replace(/\s/g,''),'wrapping must preserve all non-space text');
 }
 const koreanLine=wrapHUDLines('가'.repeat(100),562)[0];
 assert.ok(koreanLine.length>22,'Korean must use more than the old 22-character limit');
 assert.ok(getTextWidth(koreanLine)>530,'line must use nearly the full AI inner width');
 assert.deepEqual(wrapHUDLines('첫 줄\n\n둘째 줄',562),['첫 줄','','둘째 줄']);
 const denseWindow=sliceHUDWindow('가'.repeat(300));
 assert.equal(denseWindow.lines.length,5,'AI window uses five 27px rows');
 const {LocalAgreement}=await import('./local-agreement.mjs');Object.assign(context,{LocalAgreement});
 const agreement=new LocalAgreement();
 assert.equal(agreement.update('오늘 회의는').stable,'');
 const agreed=agreement.update('오늘 회의는 오후');
 assert.equal(agreed.stable,'오늘 회의는');
 assert.equal(agreed.stable+agreed.draft,'오늘 회의는 오후');
 assert.equal(clockText(new Date(2026,10,12,9,5)), '09:05');
 assert.equal(hudStatusText(75,new Date(2026,10,12,16,14)),'16:14 / 75%');
 assert.equal(hudStatusText(100,new Date(2026,10,12,0,0)),'00:00 / 100%');
 assert.equal(glassesBatteryText(0),'0%');
 assert.equal(glassesBatteryText(undefined),'--');
 const {SpeechSegmenter,mergePCM,pcmLevel}=await import('./audio-segments.mjs');
 Object.assign(context,{SpeechSegmenter,mergePCM,pcmLevel});
 const vad=new SpeechSegmenter();
 const voiced=Buffer.alloc(3200);for(let i=0;i<voiced.length;i+=2)voiced.writeInt16LE(5000,i);
 for(let i=0;i<20;i++)assert.equal(vad.push(Buffer.alloc(3200)),null,'silence must not generate clips');
 for(let i=0;i<6;i++)assert.equal(vad.push(voiced),null);
 let segmented=null;for(let i=0;i<9;i++)segmented=vad.push(Buffer.alloc(3200)) || segmented;
 assert.ok(segmented && segmented.length>=48000,'utterance should include speech and silence boundary');
 assert.equal(vad.flush(),null,'completed utterance must not be duplicated');
 vad.push(voiced);for(let i=0;i<9;i++)assert.equal(vad.push(Buffer.alloc(3200)),null,'brief clicks must not trigger speech');
 const capped=new SpeechSegmenter({maxMs:1000});let cappedClip=null;
 for(let i=0;i<10;i++)cappedClip=capped.push(voiced)||cappedClip;
 assert.equal(cappedClip.length,32000,'long speech must be split at the duration limit');

 context.normalizeModels = normalizeModels;
 assert.deepEqual(normalizeModels({models:[{key:'qwen/test',display_name:'Qwen Test'},{key:'other-model',name:'Other'}]}),[
   {id:'qwen/test',label:'Qwen Test'},{id:'other-model',label:'Other'}
 ]);
 assert.deepEqual(normalizeModels({data:[{id:'openai-model'}]}),[{id:'openai-model',label:'openai-model'}]);
 assert.deepEqual(normalizeModels(['plain-model',{id:{bad:true},key:'valid-key'},{id:'plain-model'},null,{}]),[
   {id:'plain-model',label:'plain-model'},{id:'valid-key',label:'valid-key'}
 ]);
 assert.deepEqual(normalizeModels({models:[{name:{bad:true}},{id:'[object Object]'}]}),[]);
 vm.runInNewContext(src,context);
 await tick();
 assert.equal(startup.textObject[0].isEventCapture,1);
 assert.equal(startup.containerTotalNum,4);
 assert.equal(startup.textObject[0].yPosition,27);
 assert.equal(startup.textObject[0].height,168);
 assert.equal(startup.textObject[0].borderWidth,1);
 assert.equal(Math.floor((startup.textObject[0].height-2*(startup.textObject[0].paddingLength+startup.textObject[0].borderWidth))/27),6,'AI area must fit six native text rows');
 const aiArea=startup.textObject[0],txArea=startup.textObject.find(c=>c.containerName==='transcript');
 assert.equal(aiArea.yPosition,27);assert.equal(aiArea.yPosition+aiArea.height,txArea.yPosition,'body sections must have no unused gap');
 assert.equal(txArea.yPosition+txArea.height,288,'transcript must reach the supported canvas bottom');
 for(const box of startup.textObject){assert.ok(box.yPosition+box.height<=288);assert.ok(box.xPosition+box.width<=576);}

 assert.equal(startup.textObject.find(c=>c.containerName==='transcript').containerID,4);
 assert.equal(startup.textObject.find(c=>c.containerName==='transcript').yPosition,195);
 assert.deepEqual(Array.from(startup.menuObject.menuItems,m=>m.itemName),['전사 모드','AI 모드','번역 모드']);
 assert.equal(startup.textObject.find(c=>c.containerName==='notice').yPosition,0);
 assert.ok(startup.textObject.find(c=>c.containerName==='notice').width<=startup.textObject.find(c=>c.containerName==='status').xPosition);
 assert.equal(startup.textObject.filter(c=>c.isEventCapture===1).length,1);
 await tick();
 assert.match(element('hud-status-text').textContent,/^\d{2}:\d{2} \/ 82%$/);
 deviceHandler({sn:'ring-1',batteryLevel:11});
 assert.match(element('hud-status-text').textContent,/\/ 82%$/,'ring battery must be ignored');
 deviceHandler({sn:'glasses-1',batteryLevel:0});
 assert.match(element('hud-status-text').textContent,/\/ 0%$/);
 assert.equal(element('login-status-label').textContent,'');
 assert.equal(element('server-url').value,'https://192.168.1.4:2806');
 handler({audioEvent:{audioPcm:[1,2]}}); handler({textEvent:{eventType:1}}); await tick();
 assert.equal(audioCalls.length,0,'audio and scroll must not trigger recording');
 element('auth-username').value='tester'; element('auth-password').value='password';
 await element('btn-login').handlers.click();
 const login=requests.find(r=>r.url.endsWith('/api/login'));
 assert.deepEqual(JSON.parse(login.opts.body),{id:'tester',password:'password',remember_me:true});
 assert.equal(element('model-select').options[0].textContent,'Test Model');
 assert.equal(element('model-select').value,'test-model');
 assert.equal(JSON.parse(nativeSaved).authToken,'test-session','token must persist in native SDK storage');
 assert.equal(JSON.parse(nativeSaved).serverUrl,'https://192.168.1.4:2806');
 handler({sysEvent:{eventSource:1}}); await tick();
 assert.deepEqual(audioCalls[0],[true,0]);
 handler({audioEvent:{audioPcm:Array(6400).fill(1)}});
 handler({sysEvent:{eventType:0}}); await tick();await tick();
 const stt=requests.find(r=>r.url.endsWith('/api/stt/transcribe'));
 assert.equal(Buffer.from(JSON.parse(stt.opts.body).audio,'base64').length,6400);
 assert.equal(stt.opts.headers.Authorization,'Bearer test-session');
 assert.equal(JSON.parse(requests.find(r=>r.url.endsWith('/api/chat')).opts.body).model,'test-model');
 assert.match(element('hud-screen').innerHTML,/안녕하세요/,'fragmented SSE must be reassembled');

 await new Promise(r=>setTimeout(r,350));
 const originalChatCount=requests.filter(r=>r.url.endsWith('/api/chat')).length;
 element('voice-mode').value='continuous'; element('trigger-word').value='이븐';
 handler({sysEvent:{eventType:0}}); await tick();await tick();
 const speech=Buffer.alloc(3200);for(let i=0;i<speech.length;i+=2)speech.writeInt16LE(5000,i);
 function utterance(speakerRole){for(let i=0;i<6;i++)handler({audioEvent:{audioPcm:speech,speakerRole}});for(let i=0;i<9;i++)handler({audioEvent:{audioPcm:Buffer.alloc(3200),speakerRole}});}
 nextTranscript='오늘 회의는 세 시입니다'; utterance();await tick();await tick();
 assert.equal(requests.filter(r=>r.url.endsWith('/api/chat')).length,originalChatCount,'ordinary conversation must not call AI');
 assert.match(element('transcript-log').textContent,/오늘 회의/);
 nextRejected=true;nextTranscript='여기 '.repeat(100);utterance();await tick();await tick();
 assert.doesNotMatch(element('transcript-log').textContent,/여기/,'rejected loops must not enter the transcript view');
 assert.equal(requests.filter(r=>r.url.endsWith('/api/chat')).length,originalChatCount);
 nextRejected=false;
 holdChat=true;nextTranscript='이븐 오늘 일정을 요약해줘';utterance();await tick();await tick();await tick();
 assert.equal(requests.filter(r=>r.url.endsWith('/api/chat')).length,originalChatCount+1,'trigger should call the existing gateway chat');
 const triggerBody=JSON.parse(requests.filter(r=>r.url.endsWith('/api/chat')).at(-1).opts.body);
 assert.match(triggerBody.messages.at(-1).content,/오늘 회의/);
 assert.match(triggerBody.messages.at(-1).content,/오늘 일정을 요약해줘/);
 const finalsBeforeAIWait=requests.filter(r=>r.url.endsWith('/api/stt/transcribe')).length;
 nextTranscript='AI 응답 중에도 전사할 문장';utterance();await tick();await tick();
 assert.equal(requests.filter(r=>r.url.endsWith('/api/stt/transcribe')).length,finalsBeforeAIWait+1,'ASR must continue while the LLM is waiting');
 assert.match(element('transcript-log').textContent,/AI 응답 중에도/);
 releaseChat();await tick();await tick();
 const chatCount=requests.filter(r=>r.url.endsWith('/api/chat')).length;
 const finalsBeforePreview=requests.filter(r=>r.url.endsWith('/api/stt/transcribe')).length;
 nextTranscript='이븐 임시로 인식한 문장';
 for(let i=0;i<15;i++)handler({audioEvent:{audioPcm:speech}});
 await tick();await tick();
 assert.ok(requests.some(r=>r.url.endsWith('/api/stt/preview')));
 assert.match(element('hud-screen').innerHTML,/임시로 인식/,'draft must appear before silence/end of utterance');
 assert.equal(requests.filter(r=>r.url.endsWith('/api/stt/transcribe')).length,finalsBeforePreview);
 assert.equal(requests.filter(r=>r.url.endsWith('/api/chat')).length,chatCount,'draft trigger must not call AI');
 assert.doesNotMatch(element('transcript-log').textContent,/임시로 인식/,'draft must not enter saved history');
 nextTranscript='확정된 실제 문장';
 for(let i=0;i<10;i++)handler({audioEvent:{audioPcm:speech}});
 await tick();await tick();
 const previews=requests.filter(r=>r.url.endsWith('/api/stt/preview')).slice(-2).map(r=>JSON.parse(r.opts.body));
 assert.equal(previews[0].segmentId,previews[1].segmentId);
 assert.ok(previews[1].revision>previews[0].revision);
 for(let i=0;i<9;i++)handler({audioEvent:{audioPcm:Buffer.alloc(3200)}});
 await tick();await tick();
 assert.match(element('transcript-log').textContent,/확정된 실제 문장/);
 assert.match(element('hud-notice').textContent,/전사 저장됨.*AI 호출어/);
 // An in-flight draft completed after endpointing must not repaint the final.
 holdPreview=true;nextTranscript='늦게 도착한 가짜 초안';
 for(let i=0;i<15;i++)handler({audioEvent:{audioPcm:speech}});
 await tick();
 nextTranscript='마지막 확정 문장';
 for(let i=0;i<9;i++)handler({audioEvent:{audioPcm:Buffer.alloc(3200)}});
 await tick();assert.ok(releasePreview);
 releasePreview();await tick();await tick();
 assert.doesNotMatch(element('hud-screen').innerHTML,/늦게 도착한 가짜/);
 assert.match(element('transcript-log').textContent,/마지막 확정 문장/);

 handler({sysEvent:{eventType:0}});await tick();await tick();
 handler({sysEvent:{eventType:3}}); await tick();await tick(); assert.equal(shutdown,1);
 await new Promise(r=>setTimeout(r,400));
 assert.ok(updates.some(update=>update.containerID===1 && update.content.includes('안녕하세요')));
 element('btn-transcripts').handlers.click();await tick();
 assert.equal(element('transcript-modal').hidden,false);
 assert.equal(context.document.body.style.overflow,'hidden');
 assert.ok(element('transcript-log').textContent.length>50000);
 element('btn-close-transcripts').handlers.click();
 assert.equal(element('transcript-modal').hidden,true);
 assert.equal(context.document.body.style.overflow,'');

 // A stop during engine warmup cancels the intent; delayed warmup cannot open MIC.
 element('voice-mode').value='manual';
 const opensBeforeWarmup=audioCalls.filter(call=>call[0]).length;
 holdWarmup=true;handler({sysEvent:{eventType:0}});await tick();await tick();
 assert.ok(releaseWarmup);
 handler({sysEvent:{eventType:0}});await tick();await tick();
 assert.match(element('hud-notice').textContent,/녹음 정지/);
 releaseWarmup();await tick();await tick();
 assert.equal(audioCalls.filter(call=>call[0]).length,opensBeforeWarmup);
 const normalAudioControl=bridge.audioControl;
 let releaseOpen;
 bridge.audioControl=(open,...args)=>{audioCalls.push([open,...args]);return open?new Promise(resolve=>{releaseOpen=resolve;}):Promise.resolve(true);};
 handler({sysEvent:{eventType:0}});await tick();await tick();
 assert.ok(releaseOpen);
 handler({sysEvent:{eventType:0}});await tick();await tick();
 assert.equal(audioCalls.at(-1)[0],false,'stop must reach native MIC before open reply arrives');
 releaseOpen(true);await tick();await tick();await tick();
 assert.equal(audioCalls.at(-1)[0],false,'delayed MIC open must be closed again');
 assert.match(element('hud-notice').textContent,/녹음 정지/);
 // Stop must bypass any pending preview inference.
 bridge.audioControl=normalAudioControl;element('voice-mode').value='continuous';
 handler({sysEvent:{eventType:0}});await tick();await tick();
 holdPreview=true;nextTranscript='정지 전에 도착하지 않은 자막';
 for(let i=0;i<15;i++)handler({audioEvent:{audioPcm:speech}});
 await tick();
 const opensBeforeStop=audioCalls.filter(call=>call[0]).length;
 handler({sysEvent:{eventType:0}});await tick();await tick();
 assert.equal(audioCalls.at(-1)[0],false,'MIC stop must not wait for inference');
 assert.match(element('hud-notice').textContent,/녹음 정지/);
 releasePreview();await tick();await tick();
 assert.equal(audioCalls.filter(call=>call[0]).length,opensBeforeStop);
 // A failed close is visible; the next tap retries close instead of reopening.
 let rejectClose=true;
 bridge.audioControl=async(open,...args)=>{audioCalls.push([open,...args]);return open || !rejectClose;};
 element('voice-mode').value='manual';
 handler({textEvent:{eventType:0}});await tick();await tick();
 const opensBeforeRetry=audioCalls.filter(call=>call[0]).length;
 handler({textEvent:{eventType:0}});await tick();await tick();
 assert.match(element('hud-notice').textContent,/종료 확인 실패/);
 rejectClose=false;handler({sysEvent:{eventType:0}});await tick();await tick();
 assert.equal(audioCalls.filter(call=>call[0]).length,opensBeforeRetry);
 assert.match(element('hud-notice').textContent,/녹음 정지/);
 bridge.audioControl=normalAudioControl;
 // Text entry uses the same AI queue and chooses the effective account protocol.
 llmMode='stateful';
 function sseReply(text){let sent=false;return{ok:true,body:{getReader:()=>({read:async()=>sent?{done:true}:(sent=true,{done:false,value:Buffer.from(text)})})}};}
 chatReply=sseReply('event: model_load.progress\r\ndata: {"type":"model_load.progress","progress":0.5}\r\n\r\nevent: tool_call.arguments\r\ndata: {"type":"tool_call.arguments","tool":"web_search","arguments":{"query":"서울 날씨"}}\r\n\r\nevent: message.delta\r\ndata: {"type":"message.delta","content":"임시 답변"}\r\n\r\ndata: {"type":"reasoning.delta","content":"비공개 추론"}\n\ndata: {"type":"chat.end","result":{"output":[{"type":"message","content":"최종 답변\\n\\n검색 출처:\\n- [기상청](https://www.kma.go.kr)"}]}}\n\ndata: {"type":"request.complete","final_assistant_content":"최종 답변\\n\\n검색 출처:\\n- [기상청](https://www.kma.go.kr)"}\n\n');
 element('chat-input').value='텍스트 질문';
 element('text-chat-form').handlers.submit({preventDefault(){}});await tick();await tick();await tick();
 let textRequest=JSON.parse(requests.filter(r=>r.url.endsWith('/api/chat')).at(-1).opts.body);
 assert.equal(textRequest.input,'텍스트 질문');assert.equal(textRequest.store,false);assert.equal(textRequest.messages,undefined);
 assert.match(textRequest.system_prompt,/HUD/);
 assert.equal(element('chat-input').value,'');
 assert.equal(element('chat-output').textContent,'최종 답변','terminal answer must replace drafts without duplicating content, revealing reasoning, or keeping sources');
 assert.match(element('hud-screen').innerHTML,/hud-ai-box/);
 assert.match(element('hud-screen').innerHTML,/hud-transcript-box/);
 assert.doesNotMatch(element('hud-screen').innerHTML,/검색 출처|kma\.go\.kr/);
 // Glasses contextual menu switches to AI mode (continuous AI without trigger word) and Translate mode
 element('ai-hint-mode').value='all';
 handler({menuItemClickEvent:{itemID:2}});await tick();
 assert.equal(element('glasses-mode').value,'ai');
 assert.match(element('hud-screen').innerHTML,/AI 연속 대화/);
 handler({sysEvent:{eventType:0}});await tick();await tick();
 const chatsBeforeAIMode = requests.filter(r=>r.url.endsWith('/api/chat')).length;
 chatReply=sseReply('data: {"choices":[{"delta":{"content":"네, 호출어 없이 바로 답변드립니다."}}]}\n\ndata: [DONE]\n\n');
 nextTranscript='호출어 없이 바로 질문합니다';
 utterance();await tick();await tick();await tick();
 assert.equal(requests.filter(r=>r.url.endsWith('/api/chat')).length,chatsBeforeAIMode+1,'AI mode must call /api/chat for every utterance without trigger word');
 assert.match(element('chat-output').textContent,/호출어 없이 바로 답변/);
 // Hint mode classifies saved finals without blocking the transcript queue.
 handler({sysEvent:{eventType:0}});await tick();await tick();
 element('hint-web-search').checked=false;element('hint-web-search').handlers.change();
 element('ai-hint-mode').value='hints';handler({sysEvent:{eventType:0}});await tick();await tick();
 context.window.__queueTest.hintGate.settleMs=0;
 const beforeHints=requests.filter(r=>r.url.endsWith('/api/chat')).length;
 nextTranscript='응, 오늘도 수고했어';utterance();await tick();await tick();await tick();
 assert.equal(requests.filter(r=>r.url.endsWith('/api/chat')).length,beforeHints,'ordinary ambient conversation must not generate an answer');
 assert.ok(requests.some(r=>r.url.endsWith('/api/glasses/hint-decision')));
 assert.match(element('transcript-log').textContent,/오늘도 수고/,'hint filtering must not filter saved transcripts');
 context.window.__queueTest.hintGate.reset();
 hintDecision={hint:true,category:'explain'};
 chatReply=sseReply('data: '+JSON.stringify({type:'request.complete',final_assistant_content:JSON.stringify({fact:'묵시적 갱신은 계약 조건에 따른 자동 연장을 뜻합니다.',adds_information:true})})+'\n\n');
 nextTranscript='계약서에 묵시적 갱신이라는 조항이 나온대';utterance();await tick();await tick();await tick();
 const hintBody=JSON.parse(requests.filter(r=>r.url.endsWith('/api/chat')).at(-1).opts.body);
 assert.equal(hintBody.glasses_hint,true);assert.equal(hintBody.store,false);
 assert.match(hintBody.system_prompt,/정보 비서/);assert.doesNotMatch(hintBody.input,/<conversation>|<latest_utterance>/);assert.equal(hintBody.hint_research.source,'internal');assert.match(element('chat-output').textContent,/자동 연장/);
 context.window.__queueTest.hintGate.reset();hintDecision={error:true};
 const callsBeforeError=requests.filter(r=>r.url.endsWith('/api/chat')).length;
 nextTranscript='다음 전사는 판별 오류와 독립적으로 저장됩니다';utterance();await tick();await tick();await tick();
 assert.equal(context.window.__queueTest.state().recording,true);
 assert.match(element('hint-status').textContent,/전사 계속/);
 assert.equal(requests.filter(r=>r.url.endsWith('/api/chat')).length,callsBeforeError);
 context.window.__queueTest.hintGate.reset();hintDecision={hint:true,category:'check'};
 chatReply=sseReply('data: {"type":"message.delta","content":"HINT_SKIP"}\n\ndata: {"type":"request.complete","final_assistant_content":"HINT_SKIP"}\n\n');
 nextTranscript='또 다른 개념이 대화에 나왔어';utterance();await tick();await tick();await tick();
 assert.doesNotMatch(element('hud-screen').innerHTML,/HINT_SKIP/);
 assert.match(element('chat-output').textContent,/자동 연장/,'skip must preserve the useful previous cue');
 // JEV-selected stable knowledge needs only one LLM request and no research.
 context.window.__queueTest.hintGate.reset();
 cueReply=JSON.stringify({topic:'피라미드',cue_type:'concept',source:'internal',fact:'이집트 피라미드는 왕실 무덤으로 건립되었습니다.',adds_information:true});
 const fastStart=requests.filter(r=>r.url.endsWith('/api/chat')).length;
 nextTranscript='피라미드 영상이 나와';utterance();await tick();await tick();await tick();
 assert.equal(requests.filter(r=>r.url.endsWith('/api/chat')).length,fastStart+1);
 assert.match(element('chat-output').textContent,/왕실 무덤/);
 assert.doesNotMatch(element('hud-screen').innerHTML,/보충할 정보 찾는 중|대화에 없는 질문 구성/);
 cueReply=null;
 // Web preference routes selected public topics directly to evidence + answer,
 // without a planning LLM or a model-generated source choice.
 context.window.__queueTest.hintGate.reset();
 element('hint-web-search').checked=true;element('hint-web-search').handlers.change();
 hintDecision={hint:true,category:'background',focus:'맥북 에어 M4의 무게 얘기야'};
 chatReply=sseReply('data: '+JSON.stringify({type:'message.delta',content:JSON.stringify({fact:'검색 근거에서 확인한 모델 무게 정보입니다.',adds_information:true})})+'\n\n');
 const webStart=requests.filter(r=>r.url.endsWith('/api/chat')).length;
 nextTranscript='맥북 에어 M4의 무게 얘기야\n네, 알겠습니다';utterance();await tick();await tick();await tick();
 const webBody=JSON.parse(requests.filter(r=>r.url.endsWith('/api/chat')).at(-1).opts.body);
 assert.equal(requests.filter(r=>r.url.endsWith('/api/chat')).length,webStart+1);
 assert.equal(webBody.glasses_hint_phase,'answer');assert.equal(webBody.hint_research.source,'web');
 assert.equal(webBody.hint_research.query,'맥북 에어 M4의 무게 얘기야');
 assert.match(element('chat-output').textContent,/검색 근거에서 확인한/);
 context.window.__queueTest.hintGate.reset();
 cueReply=JSON.stringify({topic:'맥북 에어',cue_type:'concept',source:'internal',fact:'맥북 에어는 휴대성을 중시한 노트북 제품군입니다.',adds_information:true});
 chatReply={ok:false,status:424,text:async()=>'힌트 근거 조회 실패'};
 nextTranscript='맥북 에어 M4의 무게 얘기야';utterance();await tick();await tick();await tick();await tick();
 assert.match(element('hint-status').textContent,/웹검색 사용 불가 · 내부 지식 힌트/);
 assert.match(element('chat-output').textContent,/휴대성/);cueReply=null;
 element('hint-web-search').checked=false;element('hint-web-search').handlers.change();
 // Clear questions bypass optional hint classification and enrichment planning.
 hintDecision={error:true};
 let directStart=requests.filter(r=>r.url.endsWith('/api/chat')).length;
 const judgeStart=requests.filter(r=>r.url.endsWith('/api/glasses/hint-decision')).length;
 chatReply=sseReply('data: '+JSON.stringify({type:'message.delta',content:'질문의 내용에 바로 답변합니다.'})+'\n\n');
 nextTranscript='청동기 시대는 언제부터 언제까지인가요';utterance();await tick();await tick();await tick();
 assert.equal(requests.filter(r=>r.url.endsWith('/api/chat')).length,directStart+1,'direct question must skip hint planning');
 assert.equal(requests.filter(r=>r.url.endsWith('/api/glasses/hint-decision')).length,judgeStart);
 let directBody=JSON.parse(requests.filter(r=>r.url.endsWith('/api/chat')).at(-1).opts.body);
 assert.equal(directBody.glasses_client,true);assert.equal(directBody.glasses_hint_phase,'direct');assert.equal(directBody.glasses_hint,true);
 assert.equal(directBody.hint_research,undefined);assert.match(element('chat-output').textContent,/바로 답변/);
 chatReply=sseReply('data: '+JSON.stringify({type:'message.delta',content:'검색 근거로 답변합니다.'})+'\n\n');
 nextTranscript='그 내용을 웹에서 검색해줘';utterance();await tick();await tick();await tick();
 directBody=JSON.parse(requests.filter(r=>r.url.endsWith('/api/chat')).at(-1).opts.body);
 assert.equal(directBody.hint_research.source,'web');assert.match(directBody.hint_research.query,/검색해줘/);
 assert.match(directBody.input,/질문의 내용에 바로 답변합니다/,'follow-up must include recent direct answers');
 // A direct question interrupts an optional hint without stopping capture.
 hintDecision={hint:true,category:'background'};context.window.__queueTest.hintGate.reset();holdChat=true;
 nextTranscript='전문적인 새 개념이 대화에 등장했어';utterance();await tick();await tick();await tick();
 assert.equal(JSON.parse(requests.filter(r=>r.url.endsWith('/api/chat')).at(-1).opts.body).glasses_hint_phase,'answer');
 assert.match(element('hud-screen').innerHTML,/검색 근거로 답변합니다/,'private cue work must keep the previously displayed answer');
 chatReply=sseReply('data: '+JSON.stringify({type:'message.delta',content:'새 질문에 우선 답변합니다.'})+'\n\n');
 nextTranscript='피라미드는 누가 지었어';utterance();await tick();await tick();await tick();await tick();
 assert.match(element('chat-output').textContent,/새 질문에 우선/);
 assert.equal(context.window.__queueTest.state().recording,true);
 // Switch to Translate mode via glasses menu item 3
 handler({menuItemClickEvent:{itemID:3}});await tick();
 assert.equal(element('glasses-mode').value,'translate');
 await new Promise(r=>setTimeout(r,800));
 const beforeSelf=requests.filter(r=>r.url.endsWith('/api/stt/transcribe')).length;
 utterance('self');await tick();await tick();
 assert.equal(requests.filter(r=>r.url.endsWith('/api/stt/transcribe')).length,beforeSelf,'self speech must not start translation');
 const translatedLayout=rebuilds.at(-1);
 assert.ok(translatedLayout,'entering translation must rebuild the native layout');
 const translationBox=translatedLayout.textObject[0],replyBox=translatedLayout.textObject.find(c=>c.containerID===4);
 assert.equal(Math.floor((translationBox.height-6)/27),5);
 assert.ok(replyBox.height>=4*27);assert.equal(translationBox.yPosition+translationBox.height,replyBox.yPosition);
 assert.equal(replyBox.yPosition+replyBox.height,288);
 assert.equal(translatedLayout.menuObject.menuItems.length,3);
 assert.match(element('hud-screen').innerHTML,/번역 · 추천 답변/);
 chatReply=sseReply('data: '+JSON.stringify({choices:[{delta:{content:JSON.stringify({translation:"커피 드시겠어요?",positive:"네, 주세요.\nYes, please.",negative:"괜찮습니다.\nNo, thank you."})}}]})+'\n\ndata: [DONE]\n\n');
 nextTranscript='Would you like some coffee?';
 utterance();await tick();await tick();await tick();
 const lastSttBody = JSON.parse(requests.filter(r=>r.url.endsWith('/api/stt/transcribe')).at(-1).opts.body);
 assert.equal(lastSttBody.language,'en','Translate mode with target ko and auto source must resolve to safe 2-letter ISO language code en');
 const lastTranslateChat = JSON.parse(requests.filter(r=>r.url.endsWith('/api/chat')).at(-1).opts.body);
 assert.match(lastTranslateChat.system_prompt,/positive[\s\S]*negative/);
 assert.match(element('chat-output').textContent,/긍정: 네, 주세요\./);
 assert.match(element('hud-screen').innerHTML,/hud-ai-body">커피 드시겠어요\?/);
 const preview=element('hud-screen').innerHTML;
 assert.match(preview,/예상 답변/);assert.match(preview,/긍정: 네, 주세요/);assert.match(preview,/부정: 괜찮습니다/);
 assert.doesNotMatch(preview,/\[중립\]|Maybe later|Would you like some coffee/);
 assert.doesNotMatch(preview.split('hud-transcript-box')[0],/Yes, please|No, thank you|네, 주세요|괜찮습니다|\[번역\]/,'box must contain only translation, not reply choices');

 element('translate-source-lang').value='ja';element('translate-source-lang').handlers.change();await tick();
 chatReply=sseReply('data: {"choices":[{"delta":{"content":"[번역] 안녕하세요.\\n[긍정] こんにちは — 안녕하세요\\n[부정] すみません — 죄송합니다\\n[중립] はい — 네"}}]}\n\ndata: [DONE]\n\n');
 nextTranscript='こんにちは';
 utterance();await tick();await tick();await tick();
 const jaSttBody = JSON.parse(requests.filter(r=>r.url.endsWith('/api/stt/transcribe')).at(-1).opts.body);
 assert.equal(jaSttBody.language,'ja','Translate mode must use selected source language code');
 assert.doesNotMatch(element('hud-notice').textContent,/전사 오류/);
 // Local translation is visible before a slow reply LLM and the next utterance
 // can replace it without waiting for the previous LLM result.
 chromeText='Chrome이 먼저 번역한 첫 문장';chatReply=null;holdChat=true;
 nextTranscript='最初の文です';utterance('other');await tick();await tick();await tick();
 assert.match(element('hud-screen').innerHTML,/Chrome이 먼저 번역한 첫 문장/);
 assert.doesNotMatch(element('hud-screen').innerHTML,/프롬프트|생각 중|생성 중|번역 중/,'progress must stay outside the translation box');
 assert.match(element('hud-notice').textContent,/번역/);
 const replyOnly=JSON.parse(requests.filter(r=>r.url.endsWith('/api/chat')).at(-1).opts.body);
 assert.match(replyOnly.system_prompt,/번역문을 생성·수정하지 마세요/);
 assert.equal(replyOnly.glasses_hint,true);assert.equal(replyOnly.hint_research.source,'internal');
 chromeText='Chrome의 최신 번역';
 chatReply=sseReply('data: '+JSON.stringify({type:'message.delta',content:JSON.stringify({translation:'LLM이 덮어쓰면 안 되는 문장',positive:'네.\nはい。',negative:'아니요.\nいいえ。'})})+'\n\n');
 nextTranscript='次の文です';utterance('other');await tick();await tick();await tick();await tick();
 assert.match(element('hud-screen').innerHTML,/Chrome의 최신 번역/);
 assert.doesNotMatch(element('hud-screen').innerHTML,/LLM이 덮어쓰면/);
 assert.match(element('chrome-translation-status').textContent,/답변 예시 완료/);
 chromeText='LLM 실패에도 유지되는 번역문';chatReply={ok:false,status:500,text:async()=>'LLM unavailable'};
 nextTranscript='翻訳は残ります';utterance('other');await tick();await tick();await tick();
 assert.match(element('hud-screen').innerHTML,/LLM 실패에도 유지되는 번역문/);
 assert.match(element('chrome-translation-status').textContent,/답변 예시 생성 실패/);
 // Prompt progress updates only the last header field, never the translation.
 chromeText='진행률 표시 중에도 유지되는 번역';
 let progressRead=0,releaseProgress;
 chatReply={ok:true,body:{getReader:()=>({read:async()=>{
   if(progressRead++===0)return{done:false,value:Buffer.from('data: '+JSON.stringify({type:'prompt_processing.progress',progress:.32})+'\n\n')};
   if(progressRead===2)return await new Promise(resolve=>{releaseProgress=()=>resolve({done:false,value:Buffer.from('data: '+JSON.stringify({type:'message.delta',content:JSON.stringify({positive:'네.\nはい。',negative:'아니요.\nいいえ。'})})+'\n\n')});});
   return{done:true};
 }})}};
 nextTranscript='進行状況を確認します';utterance('other');await tick();await tick();await tick();
 assert.equal(element('hud-notice').textContent,'[번역] JA→KO | 작동중 | 프롬프트 처리 32%');
 assert.match(element('hud-screen').innerHTML,/진행률 표시 중에도 유지되는 번역/);assert.doesNotMatch(element('hud-screen').innerHTML,/프롬프트 처리/);
 releaseProgress();await tick();await tick();await tick();
 // Live Chrome hypotheses reach the translation box before the final endpoint.
 await context.window.__queueTest.stopVoiceRecording(false);
 element('stt-engine').value='chrome-online';element('stt-engine').handlers.change();
 context.window.__queueTest.liveTranslations.intervalMs=0;
 handler({sysEvent:{eventType:0}});await tick();await tick();await tick();
 assert.equal(context.window.__queueTest.state().engine,'chrome-online');assert.equal(context.window.__queueTest.state().recording,true);
 const liveChatStart=requests.filter(r=>r.url.endsWith('/api/chat')).length;
 const liveFinalStart=requests.filter(r=>r.url.endsWith('/api/stt/transcribe')).length;
 const keptReplyPanel=element('hud-screen').innerHTML.split('id="hud-transcript-body"')[1];
 nextPreviewText='interim phrase';chromeText='말하는 동안 표시되는 임시 번역';
 for(let i=0;i<4;i++)handler({audioEvent:{audioPcm:speech,speakerRole:'other'}});
 await tick();await tick();await tick();
 assert.match(element('hud-screen').innerHTML,/말하는 동안 표시되는 임시 번역/);
 assert.equal(element('hud-screen').innerHTML.split('id="hud-transcript-body"')[1],keptReplyPanel,'interim translation must preserve the last positive/negative pair');
 assert.equal(requests.filter(r=>r.url.endsWith('/api/chat')).length,liveChatStart,'interim translation must not call the LLM');
 assert.equal(requests.filter(r=>r.url.endsWith('/api/stt/transcribe')).length,liveFinalStart,'interim translation must precede final STT/save');
 nextPreviewText='corrected interim phrase';chromeText='다시 교정된 임시 번역';
 for(let i=0;i<3;i++)handler({audioEvent:{audioPcm:speech,speakerRole:'other'}});
 await tick();await tick();await tick();assert.match(element('hud-screen').innerHTML,/다시 교정된 임시 번역/);
 nextTranscript='authoritative final phrase';chromeText='최종 확정 번역';
 chatReply=sseReply('data: '+JSON.stringify({type:'message.delta',content:JSON.stringify({positive:'네.\nはい。',negative:'아니요.\nいいえ。'})})+'\n\n');
 for(let i=0;i<9;i++)handler({audioEvent:{audioPcm:Buffer.alloc(3200),speakerRole:'other'}});
 await tick();await tick();await tick();await tick();
 assert.match(element('hud-screen').innerHTML,/최종 확정 번역/);assert.doesNotMatch(element('hud-screen').innerHTML,/다시 교정된 임시 번역/);
 assert.equal(requests.filter(r=>r.url.endsWith('/api/chat')).length,liveChatStart+1,'only the final utterance creates reply examples');
 nextPreviewText=null;await context.window.__queueTest.stopVoiceRecording(false);
 element('stt-engine').value='whisper';element('stt-engine').handlers.change();
 handler({sysEvent:{eventType:0}});await tick();await tick();await tick();
 chromeText=null;
 handler({sysEvent:{eventType:0}});await tick();await tick();
 handler({menuItemClickEvent:{itemID:1}});await tick();
 assert.equal(element('glasses-mode').value,'transcript');
 chatReply=sseReply('data: {"choices":[{"delta":{"content":"오류 전 텍스트"}}]}\n\nevent: error\ndata: LLM error: input is required\n\n');
 element('chat-input').value='오류 확인';element('text-chat-form').handlers.submit({preventDefault(){}});await tick();await tick();await tick();
 assert.match(element('chat-output').textContent,/AI 오류: LLM error: input is required/,'plain SSE errors must preserve the original cause');
 llmMode='standard';
 chatReply={ok:false,status:401,text:async()=>'LM_STUDIO_AUTH_ERROR: invalid API key'};
 element('chat-input').value='인증 확인';element('text-chat-form').handlers.submit({preventDefault(){}});await tick();await tick();await tick();
 assert.match(element('chat-output').textContent,/HTTP 401.*invalid API key/);
 textRequest=JSON.parse(requests.filter(r=>r.url.endsWith('/api/chat')).at(-1).opts.body);
 assert.equal(textRequest.messages.at(-1).content,'인증 확인');assert.equal(textRequest.input,undefined);
 chatReply=sseReply('data: {"type":"reasoning.delta","content":"생각만"}\n\n');
 element('chat-input').value='빈 응답';element('text-chat-form').handlers.submit({preventDefault(){}});await tick();await tick();await tick();
 assert.match(element('chat-output').textContent,/답변 본문을 받지 못했습니다/);
 // Queue preservation, transient retry, timeout, and stale completion regressions.
 const queue=context.window.__queueTest;
 await queue.stopVoiceRecording(false);await tick();queue.reset();
 const savedResponse={ok:true,json:async()=>({text:'대기열 테스트',saved:true})};
 let releaseFinal;
 sttReplies=[()=>new Promise(resolve=>{releaseFinal=()=>resolve(savedResponse);})];
 const queueStart=requests.filter(r=>r.url.endsWith('/api/stt/transcribe')).length;
 for(let i=0;i<6;i++)queue.enqueueClip(Buffer.alloc(6400),true,Date.now(),`queue-${i}`);
 await tick();assert.equal(queue.state().ids.length,6,'backlog must not discard audio after two segments');
 releaseFinal();await tick();await tick();await tick();
 assert.equal(queue.state().ids.length,0);
 const sentIDs=requests.filter(r=>r.url.endsWith('/api/stt/transcribe')).slice(queueStart).map(r=>JSON.parse(r.opts.body).segmentId);
 assert.deepEqual(sentIDs,Array.from({length:6},(_,i)=>`queue-${i}`),'every queued clip must be processed once in order');
 const retryStart=requests.length;
 sttReplies=[{ok:false,status:503,text:async()=>'temporarily busy'},savedResponse];
 queue.enqueueClip(Buffer.alloc(6400),true,Date.now(),'retry-same-id');
 await new Promise(resolve=>setTimeout(resolve,380));await tick();
 assert.equal(queue.state().failure,'');assert.equal(queue.state().ids.length,0);
 assert.equal(requests.slice(retryStart).filter(r=>r.url.endsWith('/api/stt/transcribe') && JSON.parse(r.opts.body).segmentId==='retry-same-id').length,2);
 // A final response from a cancelled mode must not dequeue or render a new mode's clip.
 sttReplies=[()=>new Promise(resolve=>{releaseFinal=()=>resolve(savedResponse);}),savedResponse];
 queue.enqueueClip(Buffer.alloc(6400),true,Date.now(),'old-mode');await tick();queue.reset();
 queue.enqueueClip(Buffer.alloc(6400),true,Date.now(),'new-mode');releaseFinal();await tick();await tick();
 assert.equal(queue.state().ids.length,0);
 assert.ok(requests.some(r=>r.url.endsWith('/api/stt/transcribe') && JSON.parse(r.opts.body).segmentId==='new-mode'));
 // Simulate the request timeout without waiting two minutes.
 const normalTimer=context.setTimeout;
 let timeoutNext=true;
 context.setTimeout=(fn,ms)=>ms===120000 && timeoutNext ? (timeoutNext=false,setTimeout(fn,0)) : normalTimer(fn,ms);
 sttReplies=[opts=>new Promise((resolve,reject)=>opts.signal.addEventListener('abort',()=>{const err=new Error('aborted');err.name='AbortError';reject(err);})),savedResponse];
 queue.enqueueClip(Buffer.alloc(6400),true,Date.now(),'timeout-preserved');
 await new Promise(resolve=>setTimeout(resolve,380));await tick();
 assert.equal(queue.state().ids.length,0);assert.equal(queue.state().failure,'');context.setTimeout=normalTimer;
 // Permanent auth errors preserve audio and stop capture, rather than spinning retries.
 sttReplies=[{ok:false,status:401,text:async()=>'expired session'}];
 queue.enqueueClip(Buffer.alloc(6400),true,Date.now(),'auth-preserved');await tick();await tick();
 assert.deepEqual(Array.from(queue.state().ids),['auth-preserved']);assert.match(queue.state().failure,/401/);
 sttReplies=[savedResponse];await queue.drainClips();assert.equal(queue.state().ids.length,0);
 queue.reset();
 sttReplies=[()=>new Promise(resolve=>{releaseFinal=()=>resolve(savedResponse);})];
 queue.record();queue.enqueueClip(Buffer.alloc(32000*120),true,Date.now(),'pressure-preserved');await tick();await tick();
 assert.equal(queue.state().recording,false,'two-minute backlog pauses capture');
 assert.ok(queue.state().ids.includes('pressure-preserved'),'backpressure must preserve the pending audio');
 releaseFinal();await tick();await tick();
 // AI mode drops failures/backlog and keeps the microphone running.
 queue.ai();
 const skippedResponse={ok:true,json:async()=>({text:'',saved:false,rejected:true})};
 const liveStart=requests.filter(r=>r.url.endsWith('/api/stt/transcribe')).length;
 sttReplies=[()=>new Promise(resolve=>{releaseFinal=()=>resolve(skippedResponse);}),skippedResponse];
 for(let i=0;i<6;i++)queue.enqueueClip(Buffer.alloc(6400),true,Date.now(),`live-${i}`);
 await tick();assert.deepEqual(Array.from(queue.state().ids),['live-0','live-5']);
 releaseFinal();await tick();await tick();await tick();
 assert.equal(queue.state().recording,true);assert.equal(queue.state().ids.length,0);
 const liveRequests=requests.filter(r=>r.url.endsWith('/api/stt/transcribe')).slice(liveStart).map(r=>JSON.parse(r.opts.body));
 assert.deepEqual(liveRequests.map(r=>r.segmentId),['live-0','live-5']);
 assert.ok(liveRequests.every(r=>r.realtime===true));
 for(const status of [503,401]) {
   sttReplies=[{ok:false,status,text:async()=>'simulated failure'},skippedResponse];
   queue.enqueueClip(Buffer.alloc(6400),true,Date.now(),`drop-${status}`);queue.enqueueClip(Buffer.alloc(6400),true,Date.now(),`after-${status}`);
   await new Promise(resolve=>setTimeout(resolve,260));await tick();
   assert.equal(queue.state().recording,true,`HTTP ${status} must not stop AI listening`);
   assert.equal(queue.state().failure,'');assert.equal(queue.state().ids.length,0);
 }
 let timeoutLive=true;
 context.setTimeout=(fn,ms)=>ms===12000 && timeoutLive ? (timeoutLive=false,setTimeout(fn,0)) : normalTimer(fn,ms);
 sttReplies=[opts=>new Promise((resolve,reject)=>opts.signal.addEventListener('abort',()=>{const err=new Error('aborted');err.name='AbortError';reject(err);})),skippedResponse];
 queue.enqueueClip(Buffer.alloc(6400),true,Date.now(),'live-timeout');queue.enqueueClip(Buffer.alloc(6400),true,Date.now(),'after-live-timeout');
 await new Promise(resolve=>setTimeout(resolve,260));await tick();context.setTimeout=normalTimer;
 assert.equal(queue.state().recording,true);assert.equal(queue.state().ids.length,0);assert.equal(queue.state().failure,'');
 await queue.stopVoiceRecording(false);queue.reset();
 // Chrome uses the same save/trigger pipeline without requiring Whisper warmup.
 element('stt-engine').value='chrome-online';element('stt-engine').handlers.change();
 const warmupsBeforeChrome=requests.filter(r=>r.url.endsWith('/api/stt/warmup')).length;
 handler({menuItemClickEvent:{itemID:1}});element('voice-mode').value='continuous';
 handler({sysEvent:{eventType:0}});await tick();await tick();await tick();
 assert.equal(queue.state().recording,true);
 assert.equal(requests.filter(r=>r.url.endsWith('/api/stt/warmup')).length,warmupsBeforeChrome);
 assert.ok(requests.some(r=>r.url.endsWith('/api/stt/chrome/prepare')));
 assert.equal(element('stt-engine').disabled,true);
 nextTranscript='크롬 엔진 전사';utterance('other');await tick();await tick();await tick();
 assert.equal(JSON.parse(requests.filter(r=>r.url.endsWith('/api/stt/transcribe')).at(-1).opts.body).engine,'chrome-online');
 assert.equal(JSON.parse(requests.filter(r=>r.url.endsWith('/api/stt/transcribe')).at(-1).opts.body).fallback,true);
 await queue.stopVoiceRecording(false);queue.reset();
 assert.equal(element('stt-engine').disabled,false);
 elements.clear();
 vm.runInNewContext(src,context);await tick();await tick();
 assert.equal(element('server-url').value,'https://192.168.1.4:2806');
 assert.match(element('login-status-label').textContent,/인증됨/,'new WebView with empty browser storage must restore native login');
 assert.equal(element('auth-password').value,'');
 nativeSaved=JSON.stringify({...JSON.parse(nativeSaved),authToken:'expired-session'});
 elements.clear();vm.runInNewContext(src,context);await tick();await tick();
 assert.match(element('login-status-label').textContent,/세션 만료/);
 assert.equal(JSON.parse(nativeSaved).authToken,'','expired tokens must be removed from native storage');
 console.log('PASS: backlog preservation/retry/timeout/mode races/backpressure, text chat/account protocol/LM Studio SSE/error details, MIC stop during warmup/open/preview, close retry, native settings/login restore, long transcript modal, incremental drafts/local agreement/stale rejection/AI independence, continuous capture/trigger/transcript queue, model formats/labels/chat model ID, startup, auth, tap/scroll/audio events, PCM/STT, fragmented SSE, serialized HUD, exit confirmation');
})().catch(e=>{console.error(e);process.exitCode=1;});
