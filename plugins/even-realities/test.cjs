const assert = require('node:assert/strict');
const fs = require('node:fs');
const vm = require('node:vm');
const src = fs.readFileSync('plugin.js','utf8').replace(/^import .*\n/gm, '').replace('  function enqueueClip(', `  window.__queueTest={ai(){resetQueuesForModeSwitch('ai');config.glassesMode='ai';captureFailure='';isRecording=true;segmenter=new SpeechSegmenter();},resetCaptureBuffer(){resetQueuesForModeSwitch(config.glassesMode,true,true);renderLiveHUD();},ensureModel:ensureModelLoaded,gateway:gatewayFetch,refreshIcons(){queueHudIcons(true);},headerMode(mode){config.glassesMode=mode;renderLiveHUD();},iconPhase(phase,error=''){aiPhaseTitle=phase;aiPhaseDetail='';aiStatusError=error;processingAI=!!phase;renderLiveHUD();},microphone:toggleVoiceRecording,dismiss:dismissAIWindow,headIMU:handleHeadIMU,headWait(){return headTransition?.catch(()=>{});},headState(){return {paused:headPaused,imu:headImuActive,phase:headCalibrationPhase,uncertain:micCloseUncertain};},audit:hintAudit,metrics:translationMetrics,queryChat:queryDKSTChat,forceRebuild(render=true){nativeBodyLayoutKey='';if(render)renderLiveHUD();},previewDuringSave(id){activeUtterance={id,agreement:new LocalAgreement()};pendingPreview={bytes:new Uint8Array(6400),sessionId,segmentId:id,engine:activeSttEngine,revision:1,startedAt:new Date().toISOString(),endedAt:new Date().toISOString(),mode:'translate',sourceLang:'en',targetLang:'ko'};drainPreview();},stack:hintStack,addHint(title,text,auditMeta){displayHintCard({topic:title,kind:'key',evidence:title,auditMeta},text);},reviseTranscript:reconcileHintTranscript,lifetime:hintLifetime,showHint(title,text,transcript){answerAutoCloseEligible=!!text;answerVoicePrompt='';answerTitle=title;answerText=text;captionFinal=transcript;captionStable=captionDraft='';startHintLifetime();renderLiveHUD();},enableIcons(){hudImagesDisabled=false;lastHudIconKeys.clear();},sttPayload,settings:applyLiveSettings,preview:observeAIPreview,hintGate,liveTranslations,translationHistory,enqueueClip,drainAI,drainClips,stopVoiceRecording,reset(){resetQueuesForModeSwitch('translate');config.glassesMode='translate';},state(){return{received:receivedCaptureAudio,bytes:recordedBytes,captureStatus:captureStatus(),answer:answerText,title:answerTitle,ids:pendingClips.map(c=>c.segmentId),failure:captureFailure,recording:isRecording,engine:activeSttEngine,mode:config.glassesMode,active:activeUtterance?.id,previewBusy};},record(){isRecording=true;segmenter=new SpeechSegmenter();}};
  function enqueueClip(`);
const elements = new Map();
function element(id) {
  if (!elements.has(id)) {
    const classes=new Set();
    const node={value:'',checked:true,textContent:'',style:{setProperty(name,value){this[name]=value;}},hidden:true,focus(){},handlers:{},options:[],classList:{add(...values){values.forEach(v=>classes.add(v));},remove(...values){values.forEach(v=>classes.delete(v));},contains(v){return classes.has(v);},toggle(v,force){const on=force===undefined?!classes.has(v):!!force;if(on)classes.add(v);else classes.delete(v);return on;}},addEventListener(type,fn){const previous=this.handlers[type];this.handlers[type]=previous?(...args)=>{const first=previous(...args);return fn(...args) ?? first;}:fn;},appendChild(opt){this.options.push(opt);if(!this.value)this.value=opt.value;}};
    let markup='';Object.defineProperty(node,'innerHTML',{get(){return markup;},set(value){markup=value;if(value==='')this.options=[];}});elements.set(id,node);
  }
  return elements.get(id);
}
let modelFailure=false;let ensureReplies=[];
let modelCatalog=[{key:'test-model',display_name:'Test Model'}];
let nextHintProof=undefined;
let llmMode='standard', chatReply=null, cueReply=null, chromeText=null, appleText='Apple 번역 결과';
let nextTranscript = '이븐 테스트';let nextPreviewText=null;
let nextRejected = false;
let holdPreview = false;
let releasePreview = null;
let holdChat = false;
let releaseChat = null;
let qwenWarmupFailure=false;
let holdWarmup=false, releaseWarmup=null;
let hintClock=0,hintJobs=[];
let rejectAutoLang=false;
let sttReplies=[];
let hintDecision={hint:false,reason:'no_hint'};
let judgeReply={hint:false},judgeError=false;let hintComparison={plan:false,fact:false};
let nativeSaved = '';
const documentHandlers = {};
let deviceHandler;
let handler; const imuCalls=[];const audioCalls=[]; const updates=[]; const requests=[]; let startup; let shutdown;let nativePage,validatePage=()=>{};const rebuilds=[];const imageUpdates=[];
const bridge={async imuControl(open,pace){imuCalls.push([open,pace]);return true;},async getDeviceInfo(){return{model:'g2',sn:'glasses-1',status:{sn:'glasses-1',connectType:'connected',batteryLevel:82}};},onDeviceStatusChanged(fn){deviceHandler=fn;return()=>{};},async getLocalStorage(){return nativeSaved;},async setLocalStorage(key,value){nativeSaved=value;return true;},async createStartUpPageContainer(p){validatePage(p);startup=nativePage=p;return 0;},async rebuildPageContainer(p){validatePage(p);nativePage=p;rebuilds.push(p);return true;},onEvenHubEvent(fn){handler=fn;},async textContainerUpgrade(p){const container=nativePage.textObject.find(c=>c.containerID===p.containerID);assert.ok(container,`text update targets missing container ${p.containerID}`);if(p.content!==undefined && !(p.containerID===3 && !p.content.trim()))container.content=p.content;if(p.textColor!==undefined)container.textColor=p.textColor;updates.push(p);return true;},async updateImageRawData(p){imageUpdates.push(p);return 'success';},async audioControl(...args){audioCalls.push(args);return true;},async shutDownPageContainer(mode){shutdown=mode;return true;}};
const chunks=['data: {"choices":[{"delta":{"con','tent":"안녕"}}]}\n\ndata: {"choices":[{"delta":{"content":"하세요"}}]}\n\ndata: [DONE]\n\n'];
// Simulate gateway-owned proof events; dedicated cases override this default.
function withHintProof(reply,body){
 if(!reply.ok || body.glasses_hint_phase!=='answer' || !body.hint_research?.require_grounding)return reply;
 const proof=nextHintProof===undefined?{version:1,mode:'known',source:'internal',topic:body.hint_research.topic,kind:body.hint_research.kind}:nextHintProof;
 nextHintProof=undefined;
 if(!proof)return reply;
 if(reply.body){const original=reply.body;return {...reply,body:{getReader(){const reader=original.getReader();let first=true;return{cancel:()=>reader.cancel?.(),read:async()=>first?(first=false,{done:false,value:Buffer.from('data: '+JSON.stringify({type:'hint.grounding',grounding:proof})+'\n\n')}):reader.read()};}}};}
 return {...reply,json:async()=>({...await reply.json(),hint_grounding:proof})};
}
const context={DKST_DEFAULT_SERVER_URL:'https://192.168.1.4:2806',waitForEvenAppBridge:async()=>bridge,AudioInputSource:{Glasses:'glasses',Phone:'phone'},OsEventTypeList:{CLICK_EVENT:0,DOUBLE_CLICK_EVENT:3},console,TextDecoder,Uint8Array,URL,AbortController,setTimeout:(fn,ms)=>{const timer=setTimeout(fn,ms);if(ms>=1000)timer.unref();return timer;},clearTimeout,alert(){},localStorage:{getItem(){return null;},setItem(){}},window:{location:{origin:'https://192.168.1.4:2806',hostname:'192.168.1.4'},btoa:s=>Buffer.from(s,'binary').toString('base64')},document:{body:{style:{overflow:''}},activeElement:{focus(){}},addEventListener(type,fn){documentHandlers[type]=fn;},readyState:'complete',getElementById:element,createElement:()=>({})},fetch:async(url,opts={})=>{
 if(url.endsWith('/api/chat') && opts.body){const wire=JSON.parse(opts.body);if(wire.glasses_request)opts={...opts,glassesTask:wire.glasses_request.task,body:JSON.stringify(context.expandGatewayRequest(wire,llmMode))};}
 requests.push({url,opts});
 if(url.includes('/api/glasses/hint-events'))return{ok:true,json:async()=>url.includes('?')?{events:[]}:{saved:true}};
 if(url.endsWith('/api/glasses/hint-compare'))return{ok:true,json:async()=>({checked:true,duplicate:hintComparison[JSON.parse(opts.body).kind]===true})};
 if(url.endsWith('/api/glasses/hint-decision')){if(hintDecision.error)return{ok:false,status:503};return{ok:true,json:async()=>hintDecision};}
 if(url.endsWith('/api/apple/status'))return{ok:true,json:async()=>({available:true,speechAvailable:true})};
 if(url.endsWith('/api/stt/apple/prepare') || url.endsWith('/api/stt/apple/stop') || url.endsWith('/api/translate/apple/prepare'))return{ok:true,json:async()=>({status:'ready'})};
 if(url.endsWith('/api/translate/chrome/prepare'))return{ok:true,json:async()=>({status:'ready'})};
 if(url.endsWith('/api/translate/apple'))return{ok:true,json:async()=>({status:'ready',text:appleText})};
 if(url.endsWith('/api/translate/chrome'))return chromeText?{ok:true,json:async()=>({status:'ready',text:chromeText,source:'en',target:'ko'})}:{ok:false,json:async()=>({status:'needs_prepare',error:'준비 필요'})};
 if(url.endsWith('/api/config'))return{ok:true,json:async()=>({llm_mode:llmMode,context_strategy:'retrieval'})};
 if(url.endsWith('/api/auth/check'))return{ok:true,json:async()=>({authenticated:opts.headers?.Authorization==='Bearer test-session'})};
 if(url.endsWith('/api/login'))return{ok:true,json:async()=>({token:'test-session'})};
 if(url.endsWith('/api/models/ensure')){if(ensureReplies.length){const reply=ensureReplies.shift();return typeof reply==='function'?await reply(opts):reply;}const model=JSON.parse(opts.body).model;return {ok:true,json:async()=>({status:'ready',model,instance_id:model})};}
 if(url.endsWith('/api/models') && modelFailure)return{ok:false,status:503};
 if(url.endsWith('/api/models'))return{ok:true,json:async()=>({models:modelCatalog})};
 if(url.includes('/api/stt/transcripts?'))return{ok:true,json:async()=>({entries:[{startedAt:'2026-11-12T05:00:00Z',endedAt:'2026-11-12T05:00:01Z',text:'긴 전사 기록 '.repeat(10000)}]})};
 if(url.endsWith('/api/stt/status?engine=qwen3-asr'))return{ok:true,json:async()=>({active:true,canDownload:true,modelId:'qwen3-asr-0.6b-mlx'})};
 if(url.endsWith('/api/stt/warmup?engine=qwen3-asr'))return{ok:!qwenWarmupFailure,status:qwenWarmupFailure?503:200,json:async()=>({ready:!qwenWarmupFailure,preview:true})};
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
 if(url.endsWith('/api/chat')){const payload=JSON.parse(opts.body);if(payload.glasses_task && !Array.isArray(payload.messages))return {ok:false,status:400,text:async()=>JSON.stringify({error:"'messages' field is required"})};if(payload.glasses_hint_phase==='judge'){
 if(judgeError)return{ok:false,status:503};
 let done=false;return{ok:true,body:{getReader:()=>({read:async()=>done?{done:true}:(done=true,{value:Buffer.from('data: '+JSON.stringify({choices:[{delta:{content:JSON.stringify(judgeReply)}}]})+'\n\ndata: [DONE]\n\n'),done:false})})}};
 }if(JSON.parse(opts.body).glasses_hint_phase==='plan'){
 const content=(Array.isArray(cueReply)?cueReply.shift():cueReply) || JSON.stringify({topic:'묵시적 갱신',question:'묵시적 갱신은 무엇을 뜻하나요?',knownFacts:['계약서에 묵시적 갱신이 나옴'],source:'internal'});
 let done=false;return{ok:true,body:{getReader:()=>({read:async()=>done?{done:true}:(done=true,{value:Buffer.from('data: '+JSON.stringify({type:'message.delta',content})+'\n\n'),done:false})})}};
 }if(chatReply){const reply=Array.isArray(chatReply)?chatReply.shift():chatReply;if(!Array.isArray(chatReply) || !chatReply.length)chatReply=null;return withHintProof(reply,JSON.parse(opts.body));}if(holdChat){holdChat=false;await new Promise((resolve,reject)=>{const aborted=()=>{const err=new Error('aborted');err.name='AbortError';reject(err);};opts.signal?.addEventListener('abort',aborted,{once:true});releaseChat=()=>{opts.signal?.removeEventListener('abort',aborted);resolve();};});}let i=0;return withHintProof({ok:true,body:{getReader:()=>({read:async()=>i<chunks.length?{value:Buffer.from(chunks[i++]),done:false}:{done:true}})}},JSON.parse(opts.body));}
 throw Error('Unexpected request '+url);
}};
const tick=()=>new Promise(r=>setImmediate(r));
const testKeepAlive=setInterval(()=>{},1000);
const testWatchdog=setTimeout(()=>{console.error('Test timed out while awaiting asynchronous integration work');process.exit(1);},60000);
(async()=>{
 const {validateEvenHubPageContainer}=await import('@evenrealities/even_hub_sdk');
 Object.assign(context,await import('./chat-images.mjs'));
 validatePage=page=>{const result=validateEvenHubPageContainer(page);assert.equal(result.valid,true,JSON.stringify(result));};
 for(const module of ['./hint-quality.mjs','./translation-direction.mjs','./audio-upload.mjs','./translation-metrics.mjs','./llm-translation.mjs','./hint-audit.mjs'])Object.assign(context,await import(module));
 Object.assign(context,await import('./head-gesture.mjs'));
 Object.assign(context,await import('./startup-stt.mjs'));
 const autoCloseModule=await import('./hint-autoclose.mjs');Object.assign(context,autoCloseModule);
 context.HintLifetime=class extends autoCloseModule.HintLifetime{constructor(options){super({...options,now:()=>hintClock,schedule:fn=>{hintJobs.push(fn);return hintJobs.length;},cancel:()=>{}});}};
 Object.assign(context,await import('./adaptive-hud.mjs'));
 const hintStackModule=await import('./hint-stack.mjs');Object.assign(context,hintStackModule);
 context.HintStack=class extends hintStackModule.HintStack{constructor(options){super({...options,now:()=>hintClock,schedule:fn=>{hintJobs.push(fn);return hintJobs.length;},cancel:()=>{}});}};
 Object.assign(context,await import('./hint-judge.mjs'));
 Object.assign(context,await import('./hint-policy.mjs'));
 Object.assign(context,await import('./hint-grounding.mjs'));
 Object.assign(context,await import('./realtime-hints.mjs'));
 Object.assign(context,await import('./settings-menu.mjs'));
 Object.assign(context,await import('./role-models.mjs'));
 Object.assign(context,await import('./live-translation.mjs'));
 Object.assign(context,await import('./stable-translation.mjs'));
 Object.assign(context,await import('./engine-preparation.mjs'));
 const {buildModePrompts}=await import('./test-support/gateway-prompts.mjs');
 Object.assign(context,await import('./test-support/gateway-prompts.mjs'));
 Object.assign(context,await import('./chat-deadline.mjs'));
 const {HUDTransport}=await import('./hud-transport.mjs');context.HUDTransport=class extends HUDTransport{constructor(options){super({...options,intervalMs:0});}};
 Object.assign(context,await import('./abortable-wait.mjs'));
 Object.assign(context,await import('./stt-final-stream.mjs'));
 Object.assign(context,await import('./header-layout.mjs'));
 Object.assign(context,await import('./hud-brightness.mjs'));
 Object.assign(context,await import('./ai-status-icon.mjs'));
 Object.assign(context,await import('./ai-response-limit.mjs'));
 Object.assign(context,await import('./hint-localization.mjs'));
 Object.assign(context,await import('./glasses-input.mjs'));
 const iconModule=await import('./hud-icons.mjs');Object.assign(context,iconModule);
 context.HudIconStore=class extends iconModule.HudIconStore{constructor(){super(async(key,{width,height})=>new Uint8Array(width*height*4).fill(255));}};
 Object.assign(context,await import('./translation-sentence.mjs'));
 Object.assign(context,await import('./translation-history.mjs'));
 Object.assign(context,await import('./ai-conversation.mjs'));
 Object.assign(context,await import('./chrome-translation.mjs'));
 Object.assign(context,await import('./speaker-filter.mjs'));
 Object.assign(context,await import('./translation-display.mjs'));
 Object.assign(context,await import('./direct-question.mjs'));
 Object.assign(context,await import('./hint-enrichment.mjs'));
 const {HintGate,hintActivityTiming}=await import('./hint-gate.mjs');context.hintActivityTiming=hintActivityTiming;context.HintGate=class extends HintGate{constructor(options){super({...options,timing:()=>null,collectContext:false,collectWaitMs:0});}};
 const {normalizeModels} = await import('./model-list.mjs');
 const {rightAlignedStatusText,modeHeaderText,translationHeaderText,cleanHintText,clockText,glassesBatteryText,hudStatusText,hudNoticeText,animatedDots,withAnimatedDots,extractVisibleAndReasoning,stripSourcesForGlasses,formatToolStatusText,wrapHUDLines,sliceHUDWindow,normalizeGlassesMode,nextGlassesMode,glassesModeMenuIdToMode,targetLanguageLabel,glassesModeShortLabel,glassesModeBadgeText}=await import('./hud-status.mjs');
 Object.assign(context,{rightAlignedStatusText,modeHeaderText,translationHeaderText,cleanHintText,hudStatusText,hudNoticeText,withAnimatedDots,extractVisibleAndReasoning,stripSourcesForGlasses,formatToolStatusText,sliceHUDWindow,normalizeGlassesMode,nextGlassesMode,glassesModeMenuIdToMode,targetLanguageLabel,glassesModeShortLabel,glassesModeBadgeText});
 assert.equal(normalizeGlassesMode('ai'),'ai');
 assert.equal(normalizeGlassesMode('translate'),'translate');
 assert.equal(normalizeGlassesMode('unknown'),'ai');
 assert.equal(normalizeGlassesMode('transcript'),'ai');
 assert.equal(nextGlassesMode('transcript'),'translate');
 assert.equal(nextGlassesMode('ai'),'translate');
 assert.equal(nextGlassesMode('translate'),'ai');
 assert.equal(glassesModeMenuIdToMode(1),'ai');
 assert.equal(glassesModeMenuIdToMode(2),'translate');
 assert.equal(glassesModeMenuIdToMode(3),null);
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
 assert.equal(hudStatusText(75,new Date(2026,10,12,16,14)),'16:14 | 75%');
 assert.equal(hudStatusText(100,new Date(2026,10,12,0,0)),'00:00 | 100%');
 assert.equal(glassesBatteryText(0),'0%');
 assert.equal(glassesBatteryText(undefined),'--');
 const {speechSegmentOptions,SpeechSegmenter,mergePCM,pcmLevel}=await import('./audio-segments.mjs');
 Object.assign(context,{speechSegmentOptions,SpeechSegmenter,mergePCM,pcmLevel});
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
 assert.equal(startup.containerTotalNum,8,'waiting has three header images without reserving an empty AI box');
 assert.equal(startup.imageObject.length,3);
 assert.ok(element('hud-state-icon').src.endsWith('status-idle.png'));assert.ok(element('hud-mode-icon').src.endsWith('mode-ai.png'));
 assert.equal(startup.imageObject.find(c=>c.containerID===5).width,24);
 const hp=startup.textObject.find(c=>c.containerID===7),hw=startup.textObject.find(c=>c.containerID===8),hm=startup.imageObject.find(c=>c.containerID===5),hn=startup.textObject.find(c=>c.containerID===3);
 assert.equal(startup.textObject.find(c=>c.containerID===8).textColor,1);
 assert.equal(element('hud-transcript-body').style.color,'rgb(0, 64, 26)');assert.equal(element('hud-transcript-body').style.textShadow,'none');
 for(const id of [2,3,4,7])assert.equal(startup.textObject.find(c=>c.containerID===id).textColor,1,'secondary HUD content must start dim');
 assert.equal(element('hud-trigger-keyword').style.opacity,'0.25');
 assert.equal(hp.content,'[');assert.ok(hw.xPosition>=hp.xPosition+hp.width);assert.ok(hm.xPosition>=hw.xPosition+hw.width);assert.ok(hn.xPosition>=hm.xPosition+hm.width);assert.equal(hn.content,' ');
 assert.ok(!startup.textObject.some(c=>c.containerID===1));
 const txArea=startup.textObject.find(c=>c.containerName==='transcript');
 assert.equal(txArea.yPosition,27);assert.equal(txArea.height,81);assert.equal(txArea.isEventCapture,1);
 assert.equal(txArea.yPosition+txArea.height,108,'compact waiting display must leave the lower canvas blank');
 for(const box of startup.textObject){assert.ok(box.yPosition+box.height<=288);assert.ok(box.xPosition+box.width<=576);}

 assert.equal(startup.textObject.find(c=>c.containerName==='transcript').containerID,4);
 assert.equal(startup.textObject.find(c=>c.containerName==='transcript').yPosition,27);
 assert.deepEqual(Array.from(startup.menuObject.menuItems,m=>m.itemName),['AI 모드','번역 모드','언어 설정','화면 표시','역할별 LLM 모델','설정']);
 assert.equal(startup.textObject.find(c=>c.containerName==='notice').yPosition,0);
 assert.ok(startup.textObject.find(c=>c.containerName==='notice').width<=startup.textObject.find(c=>c.containerName==='status').xPosition);
 assert.equal(startup.textObject.filter(c=>c.isEventCapture===1).length,1);
 await tick();
 assert.match(element('hud-status-text').textContent,/^\d{2}:\d{2} \| 82%$/);
 deviceHandler({sn:'ring-1',batteryLevel:11});
 assert.match(element('hud-status-text').textContent,/\| 82%$/,'ring battery must be ignored');
 deviceHandler({sn:'glasses-1',batteryLevel:0});
 assert.match(element('hud-status-text').textContent,/\| 0%$/);
 // A disconnected bridge keeps the phone UI responsive and sends no HUD work.
 for(let i=0;i<5;i++)await tick();
 deviceHandler({sn:'glasses-1',connectType:'disconnected'});await tick();
 const disconnectedWrites=updates.length+imageUpdates.length+rebuilds.length;
 for(let i=0;i<100;i++)context.window.__queueTest.refreshIcons();
 await tick();assert.equal(updates.length+imageUpdates.length+rebuilds.length,disconnectedWrites);
 assert.match(context.window.__queueTest.state().captureStatus,/안경 연결 대기/);
 deviceHandler({sn:'glasses-1',connectType:'connected',batteryLevel:0});
 for(let i=0;i<10;i++)await tick();
 assert.ok(rebuilds.length>0,'reconnect repaints the current layout');
 // Optional AI header: exactly one right-edge image, with no stale text updates.
 const layoutMicCalls=audioCalls.length;
 element('ai-header-layout').value='status-icon';element('ai-header-layout').handlers.change();
 for(let i=0;i<150 && nativePage.imageObject.length!==1;i++)await new Promise(resolve=>setTimeout(resolve,20));
 assert.equal(nativePage.imageObject.length,1);assert.equal(nativePage.imageObject[0].containerID,11);assert.equal(nativePage.imageObject[0].xPosition,552);
 assert.ok(!nativePage.textObject.some(c=>[2,7,8].includes(c.containerID)),'compact header removes clock, prefix and wake word');
 assert.equal(nativePage.textObject.find(c=>c.containerID===3).content,' ');
 assert.ok(element('hud-status-row').classList.contains('ai-icon-header'));
 assert.equal(JSON.parse(nativeSaved).aiHeaderLayout,'status-icon');
 assert.equal(element('ai-icon-position').disabled,false);
 for(const [position,x,label] of [[0,0,'좌 · 0px'],[123,123,'123px'],[124,124,'124px'],[276,276,'중앙 · 276px'],[400,400,'400px'],[552,552,'우 · 552px']]){
  element('ai-icon-position').value=String(position);element('ai-icon-position').handlers.input();
  for(let i=0;i<150 && nativePage.imageObject[0]?.xPosition!==x;i++)await new Promise(resolve=>setTimeout(resolve,20));
  assert.equal(nativePage.imageObject[0].xPosition,x);
  assert.equal(element('hud-status-row').style['--ai-icon-percent'],position/552*100+'%');
  assert.equal(element('hud-status-row').style['--ai-icon-compensation'],position/552*24+'px');
  assert.equal(element('ai-icon-position-value').textContent,label);
  assert.equal(JSON.parse(nativeSaved).aiIconPosition,position);
  const notice=nativePage.textObject.find(c=>c.containerID===3);
  assert.ok(notice.xPosition+notice.width<=x || notice.xPosition>=x+24,'blank touch container must leave the icon unobscured');
 }

 const beforeDragRebuilds=rebuilds.length,beforeDragSaved=nativeSaved;
 for(let x=100;x<=277;x++){element('ai-icon-position').value=String(x);element('ai-icon-position').handlers.input();}
 assert.equal(nativeSaved,beforeDragSaved,'rapid dragging does not write native storage for each pixel');
 element('ai-icon-position').handlers.change();
 for(let i=0;i<150 && nativePage.imageObject[0]?.xPosition!==277;i++)await new Promise(resolve=>setTimeout(resolve,20));
 assert.equal(nativePage.imageObject[0].xPosition,277);assert.equal(JSON.parse(nativeSaved).aiIconPosition,277);
 assert.ok(rebuilds.length<=beforeDragRebuilds+1,'a rapid drag commits one coalesced native layout');

 context.window.__queueTest.headerMode('translate');
 for(let i=0;i<150 && nativePage.imageObject.length!==3;i++)await new Promise(resolve=>setTimeout(resolve,20));
 assert.ok(nativePage.textObject.some(c=>c.containerID===2),'translation retains clock and full header with compact preference saved');
 assert.ok(!element('hud-status-row').classList.contains('ai-icon-header'));
 context.window.__queueTest.headerMode('ai');
 for(let i=0;i<150 && nativePage.imageObject.length!==1;i++)await new Promise(resolve=>setTimeout(resolve,20));
 assert.equal(nativePage.imageObject.length,1);

 for(const [phase,key] of [['생각 중','status-thinking'],['프롬프트 처리 중 (32%)','status-prompt-2'],['프롬프트 처리 중','status-prompt-unknown'],['답변 작성 중','mode-ai']]){
  context.window.__queueTest.iconPhase(phase);for(let i=0;i<3;i++)await tick();
  assert.ok(element('hud-state-icon').src.endsWith('/'+key+'.png'));assert.equal(nativePage.imageObject.length,1);
 }
 context.window.__queueTest.iconPhase('생각 중','synthetic failure');assert.ok(element('hud-state-icon').src.endsWith('/status-warning.png'));
 context.window.__queueTest.iconPhase('');
 element('ai-header-layout').value='standard';element('ai-header-layout').handlers.change();
 for(let i=0;i<150 && nativePage.imageObject.length!==3;i++)await new Promise(resolve=>setTimeout(resolve,20));
 assert.equal(nativePage.imageObject.length,3);assert.ok(nativePage.textObject.some(c=>c.containerID===2));
 assert.equal(element('ai-icon-position').disabled,true);
 assert.equal(audioCalls.length,layoutMicCalls,'layout changes must not restart microphones');
 assert.ok(!element('hud-status-row').classList.contains('ai-icon-header'));
 // Native instance resolution is shared across callers and all LLM roles.
 const readyResponse={ok:true,json:async()=>({status:'ready',model:'cold-role',instance_id:'cold-role:2'})};
 let finishModel;
 ensureReplies=[()=>new Promise(resolve=>{finishModel=()=>resolve(readyResponse);})];
 const modelCalls=requests.length,modelAbort=new AbortController();
 const prepareOne=context.window.__queueTest.ensureModel('cold-role',modelAbort.signal);await tick();
 const prepareTwo=context.window.__queueTest.ensureModel('cold-role',undefined,true);
 modelAbort.abort();await assert.rejects(prepareOne,{name:'AbortError'});
 assert.equal(requests.slice(modelCalls).filter(r=>r.url.endsWith('/api/models/ensure')).length,1,'forced retry joins an in-flight load');
 finishModel();assert.equal(await prepareTwo,true);
 for(const role of [{glasses_client:true},{glasses_task:'translation_preview',glasses_client:true},{glasses_hint:true,glasses_hint_phase:'judge'}]){
   ensureReplies=[readyResponse];
   await context.window.__queueTest.gateway('https://192.168.1.4:2806/api/chat',{method:'POST',body:JSON.stringify({model:'cold-role',...role})});
   assert.equal(JSON.parse(requests.filter(r=>r.url.endsWith('/api/chat')).at(-1).opts.body).model,'cold-role:2','chat, translation and judgment must use the verified instance');
 }
 ensureReplies=[{ok:false,status:503,text:async()=>'native model server unavailable'}];
 assert.equal(await context.window.__queueTest.ensureModel('cold-role'),false);
 assert.match(element('model-load-status').textContent,/준비 실패/);
 ensureReplies=[readyResponse];assert.equal(await context.window.__queueTest.ensureModel('cold-role'),true,'failed checks are not cached as success');
 // Isolated transport tests above must not affect subsequent chat call counts.
 requests.splice(modelCalls);
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
 assert.equal(context.window.__queueTest.state().recording,true,'first login automatically starts STT after bridge/settings readiness');
 assert.deepEqual(audioCalls[0],[true,'glasses']);
 assert.equal(audioCalls.filter(call=>call[0]===true).length,1,'startup checks must open the microphone once');
 // Emulate a device whose partial header updates ignore blank strings.
 nativePage.textObject.find(c=>c.containerID===3).content='| 오디오 수신 대기';
 handler({audioEvent:{audioPcm:Array(9600).fill(2)}});
 assert.equal(context.window.__queueTest.state().received,true);
 assert.ok(element('hud-state-icon').src.endsWith('status-listening.png'));
 for(let i=0;i<150 && nativePage.textObject.find(c=>c.containerID===3)?.content.includes('오디오 수신 대기');i++)await new Promise(resolve=>setTimeout(resolve,20));
 assert.equal(nativePage.textObject.find(c=>c.containerID===3).content,' ','first PCM must recreate a blank header even when blank partial updates are ignored');

 context.window.__queueTest.microphone(); await tick();await tick();
 const stt=requests.find(r=>r.url.endsWith('/api/stt/transcribe'));
 assert.equal(Buffer.from(JSON.parse(stt.opts.body).audio,'base64').length,9600);
 assert.equal(stt.opts.headers.Authorization,'Bearer test-session');
 assert.equal(JSON.parse(requests.find(r=>r.url.endsWith('/api/chat')).opts.body).model,'test-model');
 assert.match(element('hud-screen').innerHTML,/안녕하세요/,'fragmented SSE must be reassembled');

 for(let i=0;i<150 && !updates.some(update=>update.containerID===1 && update.content.includes('안녕하세요'));i++)await new Promise(resolve=>setTimeout(resolve,20));
 const originalChatCount=requests.filter(r=>r.url.endsWith('/api/chat')).length;
 element('voice-mode').value='continuous'; element('trigger-word').value='이븐';
 context.window.__queueTest.microphone(); await tick();await tick();
 const speech=Buffer.alloc(3200);for(let i=0;i<speech.length;i+=2)speech.writeInt16LE(5000,i);
 function utterance(speakerRole){for(let i=0;i<6;i++)handler({audioEvent:{audioPcm:speech,speakerRole}});for(let i=0;i<9;i++)handler({audioEvent:{audioPcm:Buffer.alloc(3200),speakerRole}});}
 // Brightness controls update text and bitmap luminance without cycling capture.
 const micCallsBeforeDim=audioCalls.length;
 element('hud-dim-level').value='0';element('hud-dim-level').handlers.input();
 for(let i=0;i<150 && nativePage.textObject.find(c=>c.containerID===2)?.textColor!==0;i++)await new Promise(resolve=>setTimeout(resolve,20));
 assert.equal(nativePage.textObject.find(c=>c.containerID===2).textColor,0,'lowest SDK level is sent as zero rather than omitted');
 assert.equal(element('hud-dim-value').textContent,'최저');
 for(let i=0;i<150 && !imageUpdates.some(u=>u.containerID===5 && u.imageData.every(byte=>byte===0x11));i++)await new Promise(resolve=>setTimeout(resolve,20));
 assert.ok(imageUpdates.some(u=>u.containerID===5 && u.imageData.every(byte=>byte===0x11)),'lowest icon uses the faintest non-black Gray4 value');

 element('hud-dim-level').value='2';element('hud-dim-level').handlers.input();
 for(let i=0;i<150 && nativePage.textObject.find(c=>c.containerID===2)?.textColor!==2;i++)await new Promise(resolve=>setTimeout(resolve,20));
 assert.equal(nativePage.textObject.find(c=>c.containerID===2).textColor,2);
 assert.equal(element('hud-mic-icon').style.opacity,'0.5');
 for(let i=0;i<150 && !imageUpdates.some(u=>u.containerID===5 && u.imageData.every(byte=>byte===0x88));i++)await new Promise(resolve=>setTimeout(resolve,20));
 assert.ok(imageUpdates.some(u=>u.containerID===5 && u.imageData.every(byte=>byte===0x88)),'slider also changes the actual microphone bitmap');
 assert.equal(element('hud-dim-value').textContent,'50%');
 element('hud-full-transcript').checked=true;element('hud-full-transcript').handlers.change();
 for(let i=0;i<150 && nativePage.textObject.find(c=>c.containerID===4)?.textColor!==4;i++)await new Promise(resolve=>setTimeout(resolve,20));
 assert.equal(nativePage.textObject.find(c=>c.containerID===4).textColor,4);
 assert.equal(JSON.parse(nativeSaved).hudFullTranscript,true,'exemptions persist to Even storage');
 element('hud-dim-enabled').checked=false;element('hud-dim-enabled').handlers.change();
 for(let i=0;i<150 && nativePage.textObject.find(c=>c.containerID===2)?.textColor!==4;i++)await new Promise(resolve=>setTimeout(resolve,20));
 assert.equal(nativePage.textObject.find(c=>c.containerID===2).textColor,4);
 assert.equal(element('hud-mic-icon').style.opacity,'1');assert.equal(element('hud-dim-level').disabled,true);
 element('hud-dim-level').value='1';element('hud-full-transcript').checked=false;element('hud-dim-enabled').checked=true;element('hud-dim-enabled').handlers.change();
 for(let i=0;i<150 && nativePage.textObject.find(c=>c.containerID===2)?.textColor!==1;i++)await new Promise(resolve=>setTimeout(resolve,20));
 assert.equal(nativePage.textObject.find(c=>c.containerID===2).textColor,1);
 assert.equal(audioCalls.length,micCallsBeforeDim,'display changes never restart the active microphone');
 nextTranscript='오늘 회의는 세 시입니다'; utterance();await tick();await tick();
 assert.equal(requests.filter(r=>r.url.endsWith('/api/chat')).length,originalChatCount,'ordinary conversation must not call AI');
 assert.match(element('transcript-log').textContent,/오늘 회의/);
 nextRejected=true;nextTranscript='여기 '.repeat(100);utterance();await tick();await tick();
 assert.doesNotMatch(element('transcript-log').textContent,/여기/,'rejected loops must not enter the transcript view');
 assert.equal(requests.filter(r=>r.url.endsWith('/api/chat')).length,originalChatCount);
 nextRejected=false;
 holdChat=true;nextTranscript='이븐 오늘 일정을 요약해줘';utterance();await tick();await tick();await tick();
 assert.equal(element('hud-trigger-keyword').style.opacity,'1','voice trigger brightens only its header text');
 for(let i=0;i<150 && nativePage.textObject.find(c=>c.containerID===8)?.textColor!==4;i++)await new Promise(resolve=>setTimeout(resolve,20));
 assert.equal(nativePage.textObject.find(c=>c.containerID===8).textColor,4);
 for(let i=0;i<150 && nativePage.textObject.find(c=>c.containerID===4)?.textColor!==1;i++)await new Promise(resolve=>setTimeout(resolve,20));
 assert.equal(nativePage.textObject.find(c=>c.containerID===4).textColor,1,'transcript stays at the selected dim level unless exempted');
 assert.equal(element('hud-transcript-body').style.color,'rgb(0, 64, 26)');
 assert.equal(requests.filter(r=>r.url.endsWith('/api/chat')).length,originalChatCount+1,'trigger should call the existing gateway chat');
 const triggerBody=JSON.parse(requests.filter(r=>r.url.endsWith('/api/chat')).at(-1).opts.body);
 assert.doesNotMatch(triggerBody.messages.at(-1).content,/오늘 회의/,'ambient transcripts must not enter explicit AI chat history');
 assert.match(triggerBody.messages.at(-1).content,/오늘 일정을 요약해줘/);
 const finalsBeforeAIWait=requests.filter(r=>r.url.endsWith('/api/stt/transcribe')).length;
 nextTranscript='AI 응답 중에도 전사할 문장';utterance();await tick();await tick();
 assert.equal(requests.filter(r=>r.url.endsWith('/api/stt/transcribe')).length,finalsBeforeAIWait+1,'ASR must continue while the LLM is waiting');
 assert.match(element('transcript-log').textContent,/AI 응답 중에도/);
 releaseChat();await tick();await tick();await tick();
 assert.equal(element('hud-trigger-keyword').style.opacity,'0.25','response completion restores dim wake text');
 for(let i=0;i<150 && nativePage.textObject.find(c=>c.containerID===8)?.textColor!==1;i++)await new Promise(resolve=>setTimeout(resolve,20));
 assert.equal(nativePage.textObject.find(c=>c.containerID===8).textColor,1);
 for(let i=0;i<150 && nativePage.textObject.find(c=>c.containerID===4)?.textColor!==1;i++)await new Promise(resolve=>setTimeout(resolve,20));
 assert.equal(nativePage.textObject.find(c=>c.containerID===4).textColor,1,'after the answer, ordinary transcription returns to dim');
 assert.equal(element('hud-transcript-body').style.color,'rgb(0, 64, 26)');
 const chatCount=requests.filter(r=>r.url.endsWith('/api/chat')).length;
 const finalsBeforePreview=requests.filter(r=>r.url.endsWith('/api/stt/transcribe')).length;
 nextTranscript='이븐 임시로 인식한 문장';
 for(let i=0;i<15;i++)handler({audioEvent:{audioPcm:speech}});
 await tick();await tick();
 assert.ok(requests.some(r=>r.url.endsWith('/api/stt/preview')));
 assert.equal(element('hud-trigger-keyword').style.opacity,'1','preview wake immediately activates trigger display');
 assert.match(element('hud-screen').innerHTML,/이븐: 임시로 인식한 문장_/,'wake input shows live STT with cursor immediately');
 assert.match(element('hud-screen').innerHTML,/hud-ai-borderless/,'listening wake input has no box border');
 for(let i=0;i<150 && nativePage.textObject.find(c=>c.containerID===1)?.borderWidth!==0;i++)await new Promise(resolve=>setTimeout(resolve,20));
 assert.equal(nativePage.textObject.find(c=>c.containerID===1)?.borderWidth,0,'native listening wake input must omit border');
 assert.doesNotMatch(element('hud-screen').innerHTML,/이븐 질문|hud-ai-header/,'draft question has no redundant title row');
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
 assert.match(element('hud-notice').textContent,/\[AI [^\]]+\] \| 듣는중/);
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

 context.window.__queueTest.microphone();await tick();await tick();
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
 holdWarmup=true;context.window.__queueTest.microphone();await tick();await tick();
 assert.ok(releaseWarmup);
 assert.match(element('hud-notice').textContent,/준비중/);assert.doesNotMatch(element('hud-notice').textContent,/듣는중/);
 context.window.__queueTest.microphone();await tick();await tick();
 assert.match(element('hud-notice').textContent,/\|/);
 releaseWarmup();await tick();await tick();
 assert.equal(audioCalls.filter(call=>call[0]).length,opensBeforeWarmup);
 const normalAudioControl=bridge.audioControl;
 let releaseOpen;
 bridge.audioControl=(open,...args)=>{audioCalls.push([open,...args]);return open?new Promise(resolve=>{releaseOpen=resolve;}):Promise.resolve(true);};
 context.window.__queueTest.microphone();await tick();await tick();
 assert.ok(releaseOpen);
 context.window.__queueTest.microphone();await tick();await tick();
 assert.equal(audioCalls.at(-1)[0],false,'stop must reach native MIC before open reply arrives');
 releaseOpen(true);await tick();await tick();await tick();
 assert.equal(audioCalls.at(-1)[0],false,'delayed MIC open must be closed again');
 assert.match(element('hud-notice').textContent,/\|/);
 // Stop must bypass any pending preview inference.
 bridge.audioControl=normalAudioControl;element('voice-mode').value='continuous';
 // PCM can arrive before the SDK's open acknowledgement. Do not overwrite
 // received state with the startup waiting hint when the acknowledgement lands.
 let finishEarlyOpen;
 bridge.audioControl=(open,...args)=>{audioCalls.push([open,...args]);return open?new Promise(resolve=>{finishEarlyOpen=()=>resolve(true);}):Promise.resolve(true);};
 const earlyStart=context.window.__queueTest.microphone();await tick();await tick();await tick();
 assert.equal(typeof finishEarlyOpen,'function');
 handler({audioEvent:{audioPcm:Buffer.alloc(3200)}});await tick();
 assert.equal(context.window.__queueTest.state().received,true);
 finishEarlyOpen();await earlyStart;await tick();
 assert.doesNotMatch(element('action-hint').textContent,/오디오 수신 대기/);
 assert.doesNotMatch(element('hud-header-message').textContent,/오디오 수신 대기/);
 context.window.__queueTest.resetCaptureBuffer();
 assert.equal(context.window.__queueTest.state().bytes,0);
 assert.equal(context.window.__queueTest.state().received,true,'buffer reset must not forget that the microphone is delivering PCM');
 assert.notEqual(context.window.__queueTest.state().captureStatus,'오디오 수신 대기');
 await context.window.__queueTest.stopVoiceRecording(false);bridge.audioControl=normalAudioControl;

 context.window.__queueTest.microphone();await tick();await tick();
 holdPreview=true;nextTranscript='정지 전에 도착하지 않은 자막';
 for(let i=0;i<15;i++)handler({audioEvent:{audioPcm:speech}});
 await tick();
 const opensBeforeStop=audioCalls.filter(call=>call[0]).length;
 context.window.__queueTest.microphone();await tick();await tick();
 assert.equal(audioCalls.at(-1)[0],false,'MIC stop must not wait for inference');
 assert.match(element('hud-notice').textContent,/\|/);
 releasePreview();await tick();await tick();
 assert.equal(audioCalls.filter(call=>call[0]).length,opensBeforeStop);
 // A failed close is visible; the next tap retries close instead of reopening.
 let rejectClose=true;
 bridge.audioControl=async(open,...args)=>{audioCalls.push([open,...args]);return open || !rejectClose;};
 element('voice-mode').value='manual';
 context.window.__queueTest.microphone();await tick();await tick();
 const opensBeforeRetry=audioCalls.filter(call=>call[0]).length;
 context.window.__queueTest.microphone();await tick();await tick();
 assert.match(element('hud-notice').textContent,/종료 확인 실패/);
 rejectClose=false;context.window.__queueTest.microphone();await tick();await tick();
 assert.equal(audioCalls.filter(call=>call[0]).length,opensBeforeRetry);
 assert.match(element('hud-notice').textContent,/\|/);
 bridge.audioControl=normalAudioControl;
 // Text entry uses the same AI queue and chooses the effective account protocol.
 llmMode='stateful';
 function sseReply(text){let sent=false;return{ok:true,body:{getReader:()=>({read:async()=>sent?{done:true}:(sent=true,{done:false,value:Buffer.from(text)})})}};}
 chatReply=sseReply('event: model_load.progress\r\ndata: {"type":"model_load.progress","progress":0.5}\r\n\r\nevent: tool_call.arguments\r\ndata: {"type":"tool_call.arguments","tool":"web_search","arguments":{"query":"서울 날씨"}}\r\n\r\nevent: message.delta\r\ndata: {"type":"message.delta","content":"임시 답변"}\r\n\r\ndata: {"type":"reasoning.delta","content":"비공개 추론"}\n\ndata: {"type":"chat.end","result":{"output":[{"type":"message","content":"최종 답변\\n\\n검색 출처:\\n- [기상청](https://www.kma.go.kr)"}]}}\n\ndata: {"type":"request.complete","final_assistant_content":"최종 답변\\n\\n검색 출처:\\n- [기상청](https://www.kma.go.kr)"}\n\n');
 element('chat-input').value='텍스트 질문';
 element('text-chat-form').handlers.submit({preventDefault(){}});await tick();await tick();await tick();
 let textRequest=JSON.parse(requests.filter(r=>r.url.endsWith('/api/chat')).at(-1).opts.body);
 assert.ok(textRequest.input.endsWith('텍스트 질문'));assert.match(textRequest.input,/오늘 일정을 요약해줘/,'typed chat should share explicit wake-word chat context');assert.equal(textRequest.store,false);assert.equal(textRequest.messages,undefined);
 assert.match(textRequest.system_prompt,/HUD/);assert.match(textRequest.system_prompt,/visible answer in Korean/);
 assert.equal(element('chat-input').value,'');
 assert.equal(element('chat-output').textContent,'최종 답변','terminal answer must replace drafts without duplicating content, revealing reasoning, or keeping sources');
 let terminalReads=0;
 chatReply={ok:true,body:{getReader:()=>({read:async()=>{
   if(terminalReads++)throw new TypeError('Load failed after request.complete');
   return {done:false,value:Buffer.from('data: '+JSON.stringify({type:'request.complete',final_assistant_content:'완료 신호에서 즉시 표시하는 답변'})+'\n\n')};
 }})}};
 element('chat-input').value='완료 신호 확인';element('text-chat-form').handlers.submit({preventDefault(){}});for(let i=0;i<8;i++)await tick();
 assert.equal(element('chat-output').textContent,'완료 신호에서 즉시 표시하는 답변');assert.equal(terminalReads,1);
 assert.equal(element('hud-trigger-keyword').style.opacity,'0.25','typed chat does not activate the voice trigger indicator');
 assert.match(element('hud-screen').innerHTML,/hud-ai-box/);
 assert.match(element('hud-screen').innerHTML,/hud-transcript-box/);
 assert.doesNotMatch(element('hud-screen').innerHTML,/검색 출처|kma\.go\.kr/);
 // The unified AI mode keeps ambient speech as hints, not every-utterance chat.
 handler({menuItemClickEvent:{itemID:1}});await tick();
 assert.equal(element('glasses-mode').value,'ai');
 assert.match(element('hud-screen').innerHTML,/완료 신호 확인/,'chat question becomes its pinned title');
 context.window.__queueTest.microphone();await tick();await tick();
 // Hint mode classifies saved finals without blocking the transcript queue.
 context.window.__queueTest.microphone();await tick();await tick();
 element('hint-web-search').checked=false;await element('hint-web-search').handlers.change();
 context.window.__queueTest.microphone();await tick();await tick();
 context.window.__queueTest.hintGate.settleMs=0;
 const beforeHints=requests.filter(r=>r.url.endsWith('/api/chat')).length;
 nextTranscript='응, 오늘도 수고했어';utterance();await tick();await tick();await tick();
 assert.equal(requests.filter(r=>r.url.endsWith('/api/chat')).length,beforeHints,'ordinary ambient conversation must not generate an answer');
 assert.ok(requests.some(r=>r.url.endsWith('/api/glasses/hint-decision')));
 assert.match(element('transcript-log').textContent,/오늘도 수고/,'hint filtering must not filter saved transcripts');
 context.window.__queueTest.hintGate.reset({forgetHistory:true});
 hintDecision={hint:true,category:'explain'};
 chatReply=sseReply('data: '+JSON.stringify({type:'request.complete',final_assistant_content:JSON.stringify({fact:'묵시적 갱신은 계약 조건에 따른 자동 연장을 뜻합니다.',adds_information:true})})+'\n\n');
 nextTranscript='계약서에 묵시적 갱신이라는 조항이 나온대';utterance();await tick();await tick();await tick();
 const hintBody=JSON.parse(requests.filter(r=>r.url.endsWith('/api/chat')).at(-1).opts.body);
 assert.equal(hintBody.glasses_hint,true);assert.equal(hintBody.store,false);
 assert.match(hintBody.system_prompt,/specific factual note/);assert.doesNotMatch(hintBody.input,/<conversation>|<latest_utterance>/);assert.equal(hintBody.hint_research.source,'internal');assert.match(element('chat-output').textContent,/자동 연장/);
 context.window.__queueTest.hintGate.reset({forgetHistory:true});hintDecision={error:true};
 const callsBeforeError=requests.filter(r=>r.url.endsWith('/api/chat')).length;
 nextTranscript='다음 전사는 판별 오류와 독립적으로 저장됩니다';utterance();await tick();await tick();await tick();
 assert.equal(context.window.__queueTest.state().recording,true);
 assert.match(element('hint-status').textContent,/전사 계속/);
 assert.equal(requests.filter(r=>r.url.endsWith('/api/chat')).length,callsBeforeError);
 context.window.__queueTest.hintGate.reset({forgetHistory:true});hintDecision={hint:true,category:'check'};
 chatReply=sseReply('data: {"type":"message.delta","content":"HINT_SKIP"}\n\ndata: {"type":"request.complete","final_assistant_content":"HINT_SKIP"}\n\n');
 nextTranscript='또 다른 개념이 대화에 나왔어';utterance();await tick();await tick();await tick();
 assert.doesNotMatch(element('hud-screen').innerHTML,/HINT_SKIP/);
 assert.match(element('chat-output').textContent,/자동 연장/,'skip must preserve the useful previous cue');
 // Legacy immediate facts must pass the same verified answer path as planned hints.
 context.window.__queueTest.hintGate.reset({forgetHistory:true});
 cueReply=JSON.stringify({topic:'피라미드',cue_type:'concept',source:'internal',fact:'이집트 피라미드는 왕실 무덤으로 건립되었습니다.',adds_information:true});
 chatReply=sseReply('data: '+JSON.stringify({type:'message.delta',content:JSON.stringify({fact:'이집트 피라미드는 왕실 무덤으로 건립되었습니다.',adds_information:true})})+'\n\n');
 const fastStart=requests.filter(r=>r.url.endsWith('/api/chat')).length;
 nextTranscript='피라미드 영상이 나와';utterance();await tick();await tick();await tick();
 assert.equal(requests.filter(r=>r.url.endsWith('/api/chat')).length,fastStart+2);
 assert.match(element('chat-output').textContent,/왕실 무덤/);
 assert.doesNotMatch(element('hud-screen').innerHTML,/보충할 정보 찾는 중|대화에 없는 질문 구성/);
 cueReply=null;
 // Source failures retain the last good card and never expose model speculation.
 const verifiedQuote='피라미드 건축에서는 무거운 석재를 옮길 때 경사로를 활용한 것으로 추정됩니다.';
 const quoteProof={version:1,mode:'quote',source:'web',topic:'달무늬 소자',records:[{title:'피라미드 건축',url:'https://example.com/pyramid',text:verifiedQuote}]};
 for(const [proof,result,status] of [
   [quoteProof,{fact:'달무늬 소자는 특별한 동력 장치입니다.',support_quote:verifiedQuote,topic:'달무늬 소자',adds_information:true},/출처 근거 불일치/],
   [null,{fact:'검증 없이 만들어 낸 설명입니다.',adds_information:true},/게이트웨이 업데이트 필요/]
 ]){
   context.window.__queueTest.hintGate.reset({forgetHistory:true});
   cueReply=JSON.stringify({topic:'달무늬 소자',question:'달무늬 소자는 무엇인가',knownFacts:[],source:'web'});
   nextHintProof=proof;chatReply=sseReply('data: '+JSON.stringify({type:'message.delta',content:JSON.stringify(result)})+'\n\n');
   nextTranscript='달무늬 소자가 나왔대';utterance();for(let i=0;i<6;i++)await tick();
   assert.match(element('hint-status').textContent,status);
   assert.match(element('chat-output').textContent,/왕실 무덤/);
   assert.equal(context.window.__queueTest.state().recording,true);
 }
 context.window.__queueTest.hintGate.reset({forgetHistory:true});
 nextHintProof={...quoteProof,topic:'피라미드'};chatReply=sseReply('data: '+JSON.stringify({type:'message.delta',content:JSON.stringify({fact:'출처에 없는 인과 설명입니다.',support_quote:verifiedQuote,topic:'피라미드',adds_information:true})})+'\n\n');
 nextTranscript='달무늬 소자가 피라미드에 나온대';utterance();for(let i=0;i<6;i++)await tick();
 assert.equal(element('chat-output').textContent,verifiedQuote);
 assert.match(context.window.__queueTest.state().title,/피라미드$/);
 cueReply=null;
 // Web preference first extracts the actual topic and an independent lookup question.
 context.window.__queueTest.hintGate.reset({forgetHistory:true});
 element('hint-web-search').checked=true;await element('hint-web-search').handlers.change();
 hintDecision={hint:true,category:'background',focus:'맥북 에어 M4의 무게 얘기야'};
 cueReply=JSON.stringify({topic:'맥북 에어 M4',question:'MacBook Air M4 13인치와 15인치의 무게는 각각 얼마인가?',knownFacts:[],source:'web'});
 chatReply=sseReply('data: '+JSON.stringify({type:'message.delta',content:JSON.stringify({fact:'검색 근거에서 확인한 모델 무게 정보입니다.',adds_information:true})})+'\n\n');
 const webStart=requests.filter(r=>r.url.endsWith('/api/chat')).length;
 nextTranscript='맥북 에어 M4의 무게 얘기야\n네, 알겠습니다';utterance();await tick();await tick();await tick();
 const webBody=JSON.parse(requests.filter(r=>r.url.endsWith('/api/chat')).at(-1).opts.body);
 assert.equal(requests.filter(r=>r.url.endsWith('/api/chat')).length,webStart+2);
 assert.equal(webBody.glasses_hint_phase,'answer');assert.equal(webBody.hint_research.source,'web');
 assert.equal(webBody.hint_research.query,'MacBook Air M4 13인치와 15인치의 무게는 각각 얼마인가?');
 assert.doesNotMatch(webBody.input,/네, 알겠습니다/);
 assert.match(element('chat-output').textContent,/검색 근거에서 확인한/);
 context.window.__queueTest.hintGate.reset({forgetHistory:true});
 cueReply=[JSON.stringify({topic:'맥북 에어',question:'맥북 에어 M4 무게는?',knownFacts:[],source:'web'}),JSON.stringify({topic:'맥북 에어',cue_type:'concept',source:'internal',fact:'맥북 에어는 휴대성을 중시한 노트북 제품군입니다.',adds_information:true})];
 chatReply={ok:false,status:424,text:async()=>'힌트 근거 조회 실패'};
 nextTranscript='맥북 에어 M4의 무게 얘기야';utterance();await tick();await tick();await tick();await tick();
 assert.match(element('hint-status').textContent,/명칭·출처를 확인하지 못해/);
 assert.doesNotMatch(element('chat-output').textContent,/휴대성/);cueReply=null;
 element('hint-web-search').checked=false;await element('hint-web-search').handlers.change();
 // Ambient questions and requests remain observed data, never direct AI chat.
 context.window.__queueTest.hintGate.reset({forgetHistory:true});hintDecision={hint:false};
 const directStart=requests.filter(r=>r.url.endsWith('/api/chat')).length;
 const judgeStart=requests.filter(r=>r.url.endsWith('/api/glasses/hint-decision')).length;
 nextTranscript='청동기 시대는 언제부터 언제까지인가요';utterance();await tick();await tick();await tick();
 assert.equal(requests.filter(r=>r.url.endsWith('/api/chat')).length,directStart,'ambient question must not bypass JEV');
 assert.ok(requests.filter(r=>r.url.endsWith('/api/glasses/hint-decision')).length>judgeStart);
 context.window.__queueTest.hintGate.reset({forgetHistory:true});
 nextTranscript='그 내용을 웹에서 검색해줘';utterance();await tick();await tick();await tick();
 assert.equal(requests.filter(r=>r.url.endsWith('/api/chat')).length,directStart,'ambient command must not become an AI instruction');
 context.window.__queueTest.hintGate.reset({forgetHistory:true});hintDecision={hint:true,category:'background'};
 cueReply=JSON.stringify({topic:'청동기 시대',cue_type:'concept',source:'internal',fact:'한반도 청동기 시대는 대략 기원전 2000~1500년 무렵 시작되어 기원전 4세기경까지 이어집니다.',adds_information:true});
 chatReply=sseReply('data: '+JSON.stringify({type:'message.delta',content:JSON.stringify({fact:'한반도 청동기 시대는 청동으로 도구를 만들던 시대입니다.',adds_information:true})})+'\n\n');
 nextTranscript='청동기 시대는 언제부터 언제까지인가요';utterance();await tick();await tick();await tick();
 const autonomous=JSON.parse(requests.filter(r=>r.url.endsWith('/api/chat')).at(-1).opts.body);
 assert.equal(autonomous.glasses_hint_phase,'answer');assert.equal(autonomous.hint_research.require_grounding,true);
 assert.match(element('chat-output').textContent,/한반도 청동기 시대/);cueReply=null;
 // An explicit wake-word question interrupts an optional hint without stopping capture.
 hintDecision={hint:true,category:'background'};context.window.__queueTest.hintGate.reset({forgetHistory:true});holdChat=true;
 nextTranscript='전문적인 새 개념이 대화에 등장했어';utterance();await tick();await tick();await tick();
 assert.equal(JSON.parse(requests.filter(r=>r.url.endsWith('/api/chat')).at(-1).opts.body).glasses_hint_phase,'answer');
 assert.match(element('hud-screen').innerHTML,/한반도 청동기 시대/,'private cue work must keep the previously displayed answer');
 chatReply=sseReply('data: '+JSON.stringify({type:'message.delta',content:'새 질문에 우선 답변합니다.'})+'\n\n');
 nextTranscript='이븐 피라미드는 누가 지었어';utterance();await tick();await tick();await tick();await tick();
 assert.match(element('chat-output').textContent,/새 질문에 우선/);
 assert.equal(context.window.__queueTest.state().recording,true);
 // Wake-only acknowledgement is local; the next utterance is explicit chat.
 context.window.__queueTest.hintGate.reset({forgetHistory:true});hintDecision={hint:false};
 const beforeWake=requests.filter(r=>r.url.endsWith('/api/chat')).length;
 nextTranscript='이븐';utterance();await tick();await tick();await tick();
 assert.equal(requests.filter(r=>r.url.endsWith('/api/chat')).length,beforeWake,'wake-only must not spend an LLM request');
 assert.match(element('hud-screen').innerHTML,/이븐: [_ ]/);
 assert.doesNotMatch(element('hud-screen').innerHTML,/이븐 질문|hud-ai-header/,'wake-only acknowledgement has no title row');
 for(let i=0;i<150 && nativePage.textObject.some(c=>c.containerID===9);i++)await new Promise(resolve=>setTimeout(resolve,20));
 assert.equal(nativePage.textObject.some(c=>c.containerID===9),false,'native wake input omits the title container');
 assert.equal(nativePage.textObject.find(c=>c.containerID===1).yPosition,27,'native question begins immediately below the status header');
 assert.match(element('hud-notice').textContent,/질문 대기/);
 assert.equal(element('hud-trigger-keyword').style.opacity,'1','wake-only acknowledgement stays bright while waiting for the question');
 chatReply=sseReply('data: '+JSON.stringify({type:'message.delta',content:'쿠푸왕의 피라미드에 대한 답변입니다.'})+'\n\n');
 nextTranscript='피라미드는 누가 만들었나요';utterance();await tick();await tick();await tick();
 let privateChat=JSON.parse(requests.filter(r=>r.url.endsWith('/api/chat')).at(-1).opts.body);
 assert.equal(privateChat.glasses_hint,undefined);assert.equal(privateChat.glasses_client,undefined);assert.ok(privateChat.input.endsWith('피라미드는 누가 만들었나요'));
 assert.doesNotMatch(privateChat.input,/질문의 내용에 바로 답변합니다/,'ambient answers are not private chat memory');
 assert.equal(element('hud-trigger-keyword').style.opacity,'0.25','the armed next question dims again when answered');
 const afterPrivate=requests.filter(r=>r.url.endsWith('/api/chat')).length;
 nextTranscript='일상 대화를 이어서 듣는 중입니다';utterance();await tick();await tick();await tick();
 assert.equal(requests.filter(r=>r.url.endsWith('/api/chat')).length,afterPrivate,'one armed question must return to hints afterward');
 chatReply=sseReply('data: '+JSON.stringify({type:'message.delta',content:'앞선 쿠푸왕 답변을 참고한 후속 답변입니다.'})+'\n\n');
 nextTranscript='이븐 그 사람은 언제 살았어';utterance();await tick();await tick();await tick();
 privateChat=JSON.parse(requests.filter(r=>r.url.endsWith('/api/chat')).at(-1).opts.body);
 assert.match(privateChat.input,/피라미드는 누가 만들었나요/);assert.match(privateChat.input,/쿠푸왕의 피라미드에 대한 답변/);
 assert.doesNotMatch(privateChat.input,/일상 대화를 이어서 듣는 중입니다/,'ambient transcripts stay out of explicit chat');
 assert.ok(privateChat.input.endsWith('그 사람은 언제 살았어'));
 // A new wake-only acknowledgement supersedes an unfinished chat answer.
 holdChat=true;element('chat-input').value='취소할 오래된 질문';element('text-chat-form').handlers.submit({preventDefault(){}});
 await tick();await tick();
 nextTranscript='이븐';utterance();await tick();await tick();await tick();
 assert.match(element('hud-screen').innerHTML,/이븐: [_ ]/);
 assert.doesNotMatch(element('hud-screen').innerHTML,/이븐 질문|hud-ai-header/,'wake-only acknowledgement has no title row');
 for(let i=0;i<150 && nativePage.textObject.some(c=>c.containerID===9);i++)await new Promise(resolve=>setTimeout(resolve,20));
 assert.equal(nativePage.textObject.some(c=>c.containerID===9),false,'native wake input omits the title container');
 assert.equal(nativePage.textObject.find(c=>c.containerID===1).yPosition,27,'native question begins immediately below the status header');assert.doesNotMatch(element('hud-screen').innerHTML,/AI 오류/);
 chatReply=sseReply('data: '+JSON.stringify({type:'message.delta',content:'새 질문에 대한 정상 답변입니다.'})+'\n\n');
 nextTranscript='다시 물어볼 내용';utterance();await tick();await tick();await tick();
 privateChat=JSON.parse(requests.filter(r=>r.url.endsWith('/api/chat')).at(-1).opts.body);
 assert.doesNotMatch(privateChat.input,/취소할 오래된 질문/);assert.match(element('chat-output').textContent,/새 질문에 대한 정상 답변/);
 // A cancelled voice response must not dim a newer wake-only state, and stopping that wait cancels it.
 holdChat=true;chatReply=null;nextTranscript='이븐 취소할 음성 질문';utterance();await tick();await tick();await tick();
 assert.equal(element('hud-trigger-keyword').style.opacity,'1');
 nextTranscript='이븐';utterance();await tick();await tick();await tick();await tick();
 assert.equal(element('hud-trigger-keyword').style.opacity,'1','old cancelled response must not override new question-wait brightness');
 await context.window.__queueTest.stopVoiceRecording(false);await tick();
 assert.equal(element('hud-trigger-keyword').style.opacity,'0.25','cancelled question wait returns to dim');
 context.window.__queueTest.microphone();await tick();await tick();await tick();
 chatReply={ok:false,status:503,text:async()=>'simulated voice LLM error'};
 nextTranscript='이븐 오류 테스트 질문';utterance();await tick();await tick();await tick();await tick();
 assert.equal(element('hud-trigger-keyword').style.opacity,'0.25','failed voice response must not leave a stuck bright trigger');
 // Switch to Translate mode via menu item 2
 handler({menuItemClickEvent:{itemID:2}});await tick();
 assert.equal(element('glasses-mode').value,'translate');
 assert.ok(element('hud-mode-icon').src.endsWith('mode-translate.png'));assert.equal(element('hud-trigger-keyword').hidden,false);
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
 assert.equal(replyBox.textColor,4,'translation reply choices remain readable');
 assert.equal(translationBox.textColor,4);assert.equal(translationBox.borderColor,4,'decoration is dimmer than translation text');
 assert.equal(translatedLayout.menuObject.menuItems.length,6);
 assert.match(element('hud-screen').innerHTML,/번역 · 추천 답변/);
 chatReply=sseReply('data: '+JSON.stringify({choices:[{delta:{content:JSON.stringify({translation:"커피 드시겠어요?",positive:"Yes, please.\n네, 주세요.",negative:"No, thank you.\n괜찮습니다."})}}]})+'\n\ndata: [DONE]\n\n');
 nextTranscript='Would you like some coffee?';
 utterance();await tick();await tick();await tick();
 const lastSttBody = JSON.parse(requests.filter(r=>r.url.endsWith('/api/stt/transcribe')).at(-1).opts.body);
 assert.equal(lastSttBody.language,'auto','Whisper translation into Korean must detect both conversation languages');
 const lastTranslateChat = JSON.parse(requests.filter(r=>r.url.endsWith('/api/chat')).at(-1).opts.body);
 assert.match(lastTranslateChat.messages[0].content,/positive[\s\S]*negative/);
 assert.match(element('chat-output').textContent,/긍정: Yes, please\./);
 assert.match(element('hud-screen').innerHTML,/hud-ai-body">커피 드시겠어요\?/);
 const preview=element('hud-screen').innerHTML;
 assert.match(preview,/예상 답변/);assert.match(preview,/긍정: Yes, please/);assert.match(preview,/부정: No, thank you/);
 assert.doesNotMatch(preview,/\[중립\]|Maybe later|Would you like some coffee/);
 assert.doesNotMatch(preview.split('hud-transcript-box')[0],/Yes, please|No, thank you|네, 주세요|괜찮습니다|\[번역\]/,'box must contain only translation, not reply choices');

 // Default LLM translation keeps the previous box while preparing/streaming,
 // including a reply-first JSON stream, empty output and generation failure.
 element('chrome-translation').checked=false;await element('chrome-translation').handlers.change();
 let llmTranslationRead=0,finishLLMTranslation;
 chatReply={ok:true,body:{getReader:()=>({read:async()=>{
   if(llmTranslationRead++===0)return{done:false,value:Buffer.from('data: '+JSON.stringify({type:'prompt_processing.progress',progress:.32})+'\n\n')};
   if(llmTranslationRead===2)return{done:false,value:Buffer.from('data: '+JSON.stringify({type:'message.delta',content:'{"positive":"좋아요"'})+'\n\n')};
   if(llmTranslationRead===3)return await new Promise(resolve=>{finishLLMTranslation=()=>resolve({done:false,value:Buffer.from('data: '+JSON.stringify({type:'message.delta',content:',"translation":"다음 번역문입니다.","negative":"아니요"}'})+'\n\n')});});
   return{done:true};
 }})}};
 nextTranscript='Next sentence';utterance('other');await tick();await tick();await tick();
 assert.match(element('hud-screen').innerHTML,/hud-ai-body">커피 드시겠어요\?/,'LLM startup and reply-only chunks must keep the previous translation');
 assert.match(element('hud-screen').innerHTML,/긍정: Yes, please/);
 finishLLMTranslation();await tick();await tick();await tick();
 assert.match(element('hud-screen').innerHTML,/hud-ai-body">[\s\S]*다음 번역문입니다/);
 chatReply={ok:false,status:500,text:async()=>'LLM unavailable'};
 const failedTranslationStart=requests.filter(r=>r.url.endsWith('/api/chat')).length;
 nextTranscript='Failed next sentence';utterance('other');await tick();await tick();await tick();
 assert.match(element('hud-screen').innerHTML,/hud-ai-body">[\s\S]*다음 번역문입니다/);
 assert.doesNotMatch(element('hud-screen').innerHTML,/AI 오류/);
 assert.match(element('hud-notice').textContent,/번역 오류/);
 assert.equal(requests.filter(r=>r.url.endsWith('/api/chat')).length,failedTranslationStart+1,'failed LLM translation must not be submitted twice');
 chatReply=sseReply('data: '+JSON.stringify({type:'message.delta',content:'{}'})+'\n\n');
 nextTranscript='Empty next sentence';utterance('other');await tick();await tick();await tick();
 assert.match(element('hud-screen').innerHTML,/hud-ai-body">[\s\S]*다음 번역문입니다/);
 assert.match(element('chrome-translation-status').textContent,/번역문을 받지 못했습니다/);
 // Stable LLM previews appear before final-STT handling. An unchanged final
 // reuses the successful translation and spends only one reply-only request.
 const llmLive=context.window.__queueTest.liveTranslations;llmLive.reset();
 const llmPreviewStart=requests.length;
 chatReply=sseReply('data: '+JSON.stringify({type:'request.complete',final_assistant_content:JSON.stringify({translation:'확정 전 문장 단위 번역'})})+'\n\n');
 llmLive.submit({id:'llm-stable',engine:'llm',source:'en',target:'ko',prompt:'A complete stable sentence.',draft:true});for(let i=0;i<10;i++)await tick();
 assert.match(element('hud-screen').innerHTML,/확정 전 문장 단위 번역/);
 assert.ok(!requests.slice(llmPreviewStart).some(r=>r.url.endsWith('/api/stt/transcribe')));
 chatReply=sseReply('data: '+JSON.stringify({type:'request.complete',final_assistant_content:JSON.stringify({positive:'Yes\n네 좋아요',negative:'No\n아니요 괜찮아요'})})+'\n\n');
 llmLive.submit({id:'llm-stable',engine:'llm',source:'en',target:'ko',prompt:'A complete stable sentence.',draft:false});for(let i=0;i<15;i++)await tick();
 assert.deepEqual(requests.slice(llmPreviewStart).filter(r=>r.url.endsWith('/api/chat')).map(r=>JSON.parse(r.opts.body).glasses_task),['translation_preview','translation_replies']);
 assert.match(element('hud-screen').innerHTML,/확정 전 문장 단위 번역/);assert.match(element('hud-screen').innerHTML,/긍정: Yes/);
 llmLive.reset();
 element('chrome-translation').checked=true;await element('chrome-translation').handlers.change();

 element('translate-source-lang').value='ja';await element('translate-source-lang').handlers.change();await tick();
 chatReply=sseReply('data: {"choices":[{"delta":{"content":"[번역] 안녕하세요.\\n[긍정] こんにちは — 안녕하세요\\n[부정] すみません — 죄송합니다\\n[중립] はい — 네"}}]}\n\ndata: [DONE]\n\n');
 nextTranscript='こんにちは';
 utterance();await tick();await tick();await tick();
 const jaSttBody = JSON.parse(requests.filter(r=>r.url.endsWith('/api/stt/transcribe')).at(-1).opts.body);
 assert.equal(jaSttBody.language,'auto','Whisper must detect both Japanese and Korean with selected Japanese partner');
 assert.doesNotMatch(element('hud-notice').textContent,/전사 오류/);
 // Korean wearer speech reverses the selected Japanese pair; the prior box
 // is cleared; reply romanization must not add pronunciation to the translation.
 chromeText='ありがとうございます。';
 chatReply=sseReply('data: '+JSON.stringify({choices:[{delta:{content:JSON.stringify({positive:'Cheonmaneyo.\nどういたしまして。',negative:'Aniyo.\nいいえ。',pronunciation:'아리가토 고자이마스'})}}]})+'\n\ndata: [DONE]\n\n');
 nextTranscript='감사합니다';utterance('other');await tick();await tick();await tick();await tick();
 const reverseRequest=JSON.parse(requests.filter(r=>r.url.endsWith('/api/translate/chrome')).at(-1).opts.body);
 assert.equal(reverseRequest.source,'ko');assert.equal(reverseRequest.target,'ja');
 assert.equal(context.window.__queueTest.translationHistory.entries.length,1,'a language turn starts a fresh translation box');
 assert.match(context.window.__queueTest.translationHistory.text(),/ありがとうございます/);
 assert.doesNotMatch(context.window.__queueTest.translationHistory.text(),/발음:|아리가토/);
 assert.match(element('hud-screen').innerHTML,/긍정: Cheonmaneyo/);
 assert.match(element('hud-screen').innerHTML,/どういたしまして/);
 assert.doesNotMatch(context.window.__queueTest.translationHistory.text(),/커피|안녕하세요/);
 // Local translation is visible before a slow reply LLM and the next utterance
 // can replace it without waiting for the previous LLM result.
 chromeText='Chrome이 먼저 번역한 첫 문장';chatReply=null;holdChat=true;
 nextTranscript='最初の文です';utterance('other');await tick();await tick();await tick();
 assert.match(element('hud-screen').innerHTML,/Chrome이 먼저 번역한 첫 문장/);
 assert.doesNotMatch(element('hud-screen').innerHTML,/프롬프트|생각 중|생성 중|번역 중/,'progress must stay outside the translation box');
 assert.match(element('hud-notice').textContent,/번역/);
 const replyOnly=JSON.parse(requests.filter(r=>r.url.endsWith('/api/chat')).at(-1).opts.body);
 assert.match(replyOnly.messages[0].content,/do not generate or change translation/);
 assert.equal(replyOnly.glasses_hint,true);assert.equal(replyOnly.hint_research.source,'internal');
 chromeText='Chrome의 최신 번역';
 chatReply=sseReply('data: '+JSON.stringify({type:'message.delta',content:JSON.stringify({translation:'LLM이 덮어쓰면 안 되는 문장',positive:'Hai.\n네.',negative:'Iie.\n아니요.'})})+'\n\n');
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
   if(progressRead===2)return await new Promise(resolve=>{releaseProgress=()=>resolve({done:false,value:Buffer.from('data: '+JSON.stringify({type:'message.delta',content:JSON.stringify({positive:'Hai.\n네.',negative:'Iie.\n아니요.'})})+'\n\n')});});
   return{done:true};
 }})}};
 nextTranscript='進行状況を確認します';utterance('other');await tick();await tick();await tick();
 assert.equal(element('hud-notice').textContent,'[번역] JA→KO | 프롬프트 처리 32%');
 assert.match(element('hud-screen').innerHTML,/진행률 표시 중에도 유지되는 번역/);assert.doesNotMatch(element('hud-screen').innerHTML,/프롬프트 처리/);
 releaseProgress();await tick();await tick();await tick();
 // Live Chrome hypotheses reach the translation box before the final endpoint.
 await context.window.__queueTest.stopVoiceRecording(false);
 element('stt-engine').value='chrome-online';await element('stt-engine').handlers.change();
 context.window.__queueTest.liveTranslations.intervalMs=0;
 context.window.__queueTest.microphone();await tick();await tick();await tick();
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
 chatReply=sseReply('data: '+JSON.stringify({type:'message.delta',content:JSON.stringify({positive:'Hai.\n네.',negative:'Iie.\n아니요.'})})+'\n\n');
 for(let i=0;i<16;i++)handler({audioEvent:{audioPcm:Buffer.alloc(3200),speakerRole:'other'}});
 await tick();await tick();await tick();await tick();
 assert.match(element('hud-screen').innerHTML,/최종 확정 번역/);assert.doesNotMatch(element('hud-screen').innerHTML,/다시 교정된 임시 번역/);
 assert.equal(requests.filter(r=>r.url.endsWith('/api/chat')).length,liveChatStart+1,'only the final utterance creates reply examples');
 nextPreviewText='today?';chromeText='앞 문장과 이어진 번역';
 for(let i=0;i<4;i++)handler({audioEvent:{audioPcm:speech,speakerRole:'other'}});
 await tick();await tick();await tick();
 assert.equal(JSON.parse(requests.filter(r=>r.url.endsWith('/api/translate/chrome')).at(-1).opts.body).text,'authoritative final phrase today?','the translator must receive the whole sentence across short audio endpoints');
 assert.equal(requests.filter(r=>r.url.endsWith('/api/chat')).length,liveChatStart+1,'continued drafts must not generate reply examples');
 // Local interim translation does not cancel a reply already being written,
 // and the late reply cannot restore its old translation into the box.
 const liveQueue=context.window.__queueTest.liveTranslations;
 liveQueue.reset();let finishIndependentReply,independentReads=0;
 chromeText='답변을 만드는 이전 문장 번역';
 chatReply={ok:true,body:{getReader:()=>({read:async()=>{
   if(independentReads++)return{done:true};
   return await new Promise(resolve=>{finishIndependentReply=()=>resolve({done:false,value:Buffer.from('data: '+JSON.stringify({type:'request.complete',final_assistant_content:JSON.stringify({translation:'이전 번역으로 덮어쓰지 말 것',positive:'Hai.\n독립 예시 긍정',negative:'Iie.\n독립 예시 부정'})})+'\n\n')});});
 }})}};
 liveQueue.submit({id:'reply-independent-old',engine:'chrome',source:'ja',target:'ko',prompt:'old sentence',draft:false});for(let i=0;i<8;i++)await tick();
 assert.equal(typeof finishIndependentReply,'function');
 const independentRequest=requests.filter(r=>r.url.endsWith('/api/chat')).at(-1);assert.equal(JSON.parse(independentRequest.opts.body).glasses_task,'translation_replies');
 chromeText='새 임시 문장의 번역';liveQueue.submit({id:'reply-independent-new',engine:'chrome',source:'ja',target:'ko',prompt:'new draft',draft:true});for(let i=0;i<6;i++)await tick();
 assert.equal(independentRequest.opts.signal.aborted,false,'native interim speech must not cancel the previous reply');
 finishIndependentReply();for(let i=0;i<10;i++)await tick();
 assert.match(element('hud-screen').innerHTML,/새 임시 문장의 번역/);assert.match(element('hud-screen').innerHTML,/독립 예시 긍정/);assert.doesNotMatch(element('hud-screen').innerHTML,/이전 번역으로 덮어쓰지/);
 liveQueue.reset();
 nextPreviewText=null;await context.window.__queueTest.stopVoiceRecording(false);
 element('stt-engine').value='whisper';await element('stt-engine').handlers.change();
 context.window.__queueTest.microphone();await tick();await tick();await tick();
 chromeText=null;
 context.window.__queueTest.microphone();await tick();await tick();
 handler({menuItemClickEvent:{itemID:1}});await tick();
 assert.equal(element('glasses-mode').value,'ai');
 chatReply=sseReply('data: {"choices":[{"delta":{"content":"오류 전 텍스트"}}]}\n\nevent: error\ndata: LLM error: input is required\n\n');
 element('chat-input').value='오류 확인';element('text-chat-form').handlers.submit({preventDefault(){}});await tick();await tick();await tick();
 assert.match(element('chat-output').textContent,/응답 중단: LLM error: input is required/,'plain SSE errors must preserve the original cause alongside the partial answer');
 llmMode='standard';
 chatReply={ok:false,status:401,text:async()=>'LM_STUDIO_AUTH_ERROR: invalid API key'};
 element('chat-input').value='인증 확인';element('text-chat-form').handlers.submit({preventDefault(){}});await tick();await tick();await tick();
 assert.match(element('chat-output').textContent,/HTTP 401.*invalid API key/);
 textRequest=JSON.parse(requests.filter(r=>r.url.endsWith('/api/chat')).at(-1).opts.body);
 assert.ok(textRequest.messages.at(-1).content.endsWith('인증 확인'));assert.equal(textRequest.input,undefined);
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
 // Apple must finish a segment before another preview can replace its native session.
 queue.reset();
 element('stt-engine').value='apple';await element('stt-engine').handlers.change();
 element('translation-engine').value='apple';await element('translation-engine').handlers.change();
 handler({menuItemClickEvent:{itemID:2}});await tick();await tick();
 context.window.__queueTest.microphone();await tick();await tick();await tick();
 assert.equal(queue.state().engine,'apple');assert.equal(queue.state().recording,true);
 sttReplies=[()=>new Promise(resolve=>{releaseFinal=()=>resolve(savedResponse);}),savedResponse];
 queue.enqueueClip(Buffer.alloc(6400),true,Date.now()-2000,'slow-archive');
 queue.enqueueClip(Buffer.alloc(6400),true,Date.now()-1000,'waiting-archive');
 await tick();
 const translationSwitchCalls=audioCalls.length,sttPrepares=requests.filter(r=>/\/api\/stt\/.*\/prepare$/.test(r.url)).length;
 for(const engine of ['chrome','llm','apple','chrome','apple'])await queue.settings({translationEngine:engine});
 assert.equal(audioCalls.length,translationSwitchCalls,'repeated translation engine switches leave capture open even with a pending final');
 assert.equal(requests.filter(r=>/\/api\/stt\/.*\/prepare$/.test(r.url)).length,sttPrepares,'translation engines never reprepare native STT');
 assert.equal(queue.state().ids.length,2);
 const duringArchive=requests.filter(r=>r.url.endsWith('/api/stt/preview')).length;
 nextPreviewText='Current live sentence';appleText='저장을 기다리지 않는 최신 번역';
 for(let i=0;i<4;i++)handler({audioEvent:{audioPcm:speech,speakerRole:'other'}});
 await tick();await tick();await tick();await tick();
 assert.equal(queue.state().ids.length,2,'both archive clips remain preserved while storage is held');
 assert.ok(element('hud-state-icon').src.endsWith('status-transcribing.png'));
 assert.equal(requests.filter(r=>r.url.endsWith('/api/stt/preview')).length,duringArchive,'same-engine previews wait instead of interrupting native finalization');
 releaseFinal();await tick();await tick();await tick();
 assert.equal(queue.state().ids.length,0);
 assert.ok(requests.filter(r=>r.url.endsWith('/api/stt/preview')).length>duringArchive,'latest buffered preview resumes after finalization');
 assert.match(element('hud-screen').innerHTML,/저장을 기다리지 않는 최신 번역/);
 await queue.stopVoiceRecording(false);queue.reset();nextPreviewText=null;
 element('stt-engine').value='whisper';await element('stt-engine').handlers.change();
 element('translation-engine').value='chrome';await element('translation-engine').handlers.change();
 // 429 contention is retried before dropping an AI utterance or stopping capture.
 const busyStart=requests.length;
 sttReplies=[{ok:false,status:429,text:async()=>'STT engine busy'},savedResponse];
 queue.enqueueClip(Buffer.alloc(6400),true,Date.now(),'busy-once');
 await new Promise(resolve=>setTimeout(resolve,320));await tick();
 assert.equal(queue.state().failure,'');assert.equal(queue.state().ids.length,0);
 assert.equal(requests.slice(busyStart).filter(r=>r.url.endsWith('/api/stt/transcribe') && JSON.parse(r.opts.body).segmentId==='busy-once').length,2,'busy auto-language responses must not produce a second immediate request');
 queue.ai();
 const aiBusyStart=requests.length;
 sttReplies=[{ok:false,status:429,text:async()=>'STT engine busy'},savedResponse];
 queue.enqueueClip(Buffer.alloc(6400),true,Date.now(),'ai-busy-once');
 await new Promise(resolve=>setTimeout(resolve,320));await tick();
 assert.equal(queue.state().recording,true);assert.equal(queue.state().ids.length,0);
 assert.equal(requests.slice(aiBusyStart).filter(r=>r.url.endsWith('/api/stt/transcribe') && JSON.parse(r.opts.body).segmentId==='ai-busy-once').length,2,'AI mode also retries contention rather than immediately dropping the utterance');
 queue.reset();
 const retryStart=requests.length;
 sttReplies=[{ok:false,status:503,text:async()=>'temporarily busy'},savedResponse];
 queue.enqueueClip(Buffer.alloc(6400),true,Date.now(),'retry-same-id');
 await new Promise(resolve=>setTimeout(resolve,380));await tick();
 assert.equal(queue.state().failure,'');assert.equal(queue.state().ids.length,0);
 assert.equal(requests.slice(retryStart).filter(r=>r.url.endsWith('/api/stt/transcribe') && JSON.parse(r.opts.body).segmentId==='retry-same-id').length,2);
 // Sustained transient failures retain audio and keep capture open, with a
 // scheduled retry rather than a tight loop on each incoming clip.
 queue.reset();queue.record();
 const outageTimer=context.setTimeout;let recoverOutage;
 context.setTimeout=(fn,ms)=>ms===5000?(recoverOutage=fn,{testRetry:true}):[300,600,1200].includes(ms)?outageTimer(fn,0):outageTimer(fn,ms);
 const outageStart=requests.length;
 sttReplies=Array.from({length:4},()=>({ok:false,status:503,text:async()=> 'temporary STT service outage'}));
 queue.enqueueClip(Buffer.alloc(6400),true,Date.now(),'outage-retained');
 for(let i=0;i<100 && !recoverOutage;i++)await new Promise(r=>setTimeout(r,2));
 assert.equal(typeof recoverOutage,'function');assert.equal(queue.state().recording,true);
 assert.deepEqual(Array.from(queue.state().ids),['outage-retained']);
 assert.match(element('stt-last-error').textContent,/HTTP 503/);assert.equal(element('stt-last-error').hidden,false);
 const failedAttempts=requests.slice(outageStart).filter(r=>r.url.endsWith('/api/stt/transcribe')).length;
 queue.enqueueClip(Buffer.alloc(6400),true,Date.now(),'outage-next');await tick();
 assert.equal(requests.slice(outageStart).filter(r=>r.url.endsWith('/api/stt/transcribe')).length,failedAttempts,'new audio respects the recovery backoff');
 sttReplies=[savedResponse,savedResponse];recoverOutage();for(let i=0;i<15;i++)await tick();
 assert.equal(queue.state().ids.length,0);assert.equal(queue.state().recording,true);
 for(const r of requests.slice(outageStart).filter(r=>r.url.endsWith('/api/stt/transcribe')))assert.equal(JSON.parse(r.opts.body).stream,false);
 context.setTimeout=outageTimer;
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
 // AI mode preserves pre-recognition audio so queued wake calls cannot vanish.
 queue.ai();
 const skippedResponse={ok:true,json:async()=>({text:'',saved:false,rejected:true})};
 const liveStart=requests.filter(r=>r.url.endsWith('/api/stt/transcribe')).length;
 sttReplies=[()=>new Promise(resolve=>{releaseFinal=()=>resolve(skippedResponse);}),...Array(5).fill(skippedResponse)];
 for(let i=0;i<6;i++)queue.enqueueClip(Buffer.alloc(6400),true,Date.now(),`live-${i}`);
 await tick();assert.deepEqual(Array.from(queue.state().ids),['live-0','live-1','live-2','live-3','live-4','live-5']);
 releaseFinal();for(let i=0;i<15;i++)await tick();
 assert.equal(queue.state().recording,true);assert.equal(queue.state().ids.length,0);
 const liveRequests=requests.filter(r=>r.url.endsWith('/api/stt/transcribe')).slice(liveStart).map(r=>JSON.parse(r.opts.body));
 assert.deepEqual(liveRequests.map(r=>r.segmentId),['live-0','live-1','live-2','live-3','live-4','live-5']);
 assert.ok(liveRequests.every(r=>r.realtime===true));
 for(const status of [503,401]) {
   sttReplies=[{ok:false,status,text:async()=>'simulated failure'},skippedResponse];
   queue.enqueueClip(Buffer.alloc(6400),true,Date.now(),`drop-${status}`);queue.enqueueClip(Buffer.alloc(6400),true,Date.now(),`after-${status}`);
   await new Promise(resolve=>setTimeout(resolve,260));await tick();
   assert.equal(queue.state().recording,true,`HTTP ${status} must not stop AI listening`);
   assert.equal(queue.state().failure,'');assert.equal(queue.state().ids.length,0);
 }
 let timeoutLive=true;
 context.setTimeout=(fn,ms)=>ms===120000 && timeoutLive ? (timeoutLive=false,setTimeout(fn,0)) : normalTimer(fn,ms);
 sttReplies=[opts=>new Promise((resolve,reject)=>opts.signal.addEventListener('abort',()=>{const err=new Error('aborted');err.name='AbortError';reject(err);})),skippedResponse];
 queue.enqueueClip(Buffer.alloc(6400),true,Date.now(),'live-timeout');queue.enqueueClip(Buffer.alloc(6400),true,Date.now(),'after-live-timeout');
 await new Promise(resolve=>setTimeout(resolve,260));await tick();context.setTimeout=normalTimer;
 assert.equal(queue.state().recording,true);assert.equal(queue.state().ids.length,0);assert.equal(queue.state().failure,'');
 await queue.stopVoiceRecording(false);queue.reset();
 // Chrome uses the same save/trigger pipeline without requiring Whisper warmup.
 element('stt-engine').value='chrome-online';await element('stt-engine').handlers.change();
 const warmupsBeforeChrome=requests.filter(r=>r.url.endsWith('/api/stt/warmup')).length;
 handler({menuItemClickEvent:{itemID:1}});await tick();await tick();element('voice-mode').value='continuous';
 context.window.__queueTest.microphone();await tick();await tick();await tick();
 assert.equal(queue.state().recording,true);
 assert.equal(requests.filter(r=>r.url.endsWith('/api/stt/warmup')).length,warmupsBeforeChrome);
 assert.ok(requests.some(r=>r.url.endsWith('/api/stt/chrome/prepare')));
 assert.equal(element('stt-engine').disabled,false);
 nextTranscript='크롬 엔진 전사';utterance('other');await tick();await tick();await tick();
 assert.equal(JSON.parse(requests.filter(r=>r.url.endsWith('/api/stt/transcribe')).at(-1).opts.body).engine,'chrome-online');
 assert.equal(JSON.parse(requests.filter(r=>r.url.endsWith('/api/stt/transcribe')).at(-1).opts.body).fallback,true);
 await queue.stopVoiceRecording(false);queue.reset();
 assert.equal(element('stt-engine').disabled,false);
 // Translation scrollback follows the newest five of thirty rows and supports
 // actual glasses scroll events without snapping back on the next translation.
 handler({menuItemClickEvent:{itemID:2}});await tick();
 element('chrome-translation').checked=false;await element('chrome-translation').handlers.change();
 for(let i=0;i<35;i++){
   chatReply=sseReply('data: '+JSON.stringify({type:'message.delta',content:JSON.stringify({translation:`기록 ${i}`,positive:'네',negative:'아니요'})})+'\n\n');
   element('chat-input').value=`문장 ${i}`;element('text-chat-form').handlers.submit({preventDefault(){}});
   await tick();await tick();await tick();
 }
 const scrollback=context.window.__queueTest.translationHistory;
 assert.equal(scrollback.window().totalLines,30);assert.match(scrollback.window().visibleText,/기록 34$/);
 assert.match(element('hud-screen').innerHTML,/기록 5\n/);assert.doesNotMatch(element('hud-screen').innerHTML,/기록 4\n/);
 for(let i=0;i<6;i++)handler({sysEvent:{eventType:1}});
 const pausedView=scrollback.window().visibleText;assert.equal(scrollback.follow,false);
 chatReply=sseReply('data: '+JSON.stringify({type:'message.delta',content:JSON.stringify({translation:'기록 35',positive:'네',negative:'아니요'})})+'\n\n');
 element('chat-input').value='문장 35';element('text-chat-form').handlers.submit({preventDefault(){}});
 await tick();await tick();await tick();
 assert.equal(scrollback.window().visibleText,pausedView,'new translations must preserve the user scroll position');
 for(let i=0;i<35;i++)handler({sysEvent:{eventType:2}});
 assert.equal(scrollback.follow,true);assert.match(scrollback.window().visibleText,/기록 35$/);
 // Every mode uses the same header; progress never replaces the previous answer.
 for(const [itemID,label] of [[1,'AI']]){
   handler({menuItemClickEvent:{itemID}});await tick();
   chatReply=sseReply('data: '+JSON.stringify({type:'message.delta',content:'이전 답변 유지 확인'})+'\n\n');
   element('chat-input').value='테스트 질문';element('text-chat-form').handlers.submit({preventDefault(){}});
   await tick();await tick();await tick();
   let read=0,finish;
   chatReply={ok:true,body:{getReader:()=>({read:async()=>{
     if(read++===0)return{done:false,value:Buffer.from('data: '+JSON.stringify({type:'prompt_processing.progress',progress:.32})+'\n\n')};
     if(read===2)return await new Promise(resolve=>{finish=()=>resolve({done:false,value:Buffer.from('data: '+JSON.stringify({type:'message.delta',content:'새 답변'})+'\n\n')});});
     return{done:true};
   }})}};
   element('chat-input').value='다음 질문';element('text-chat-form').handlers.submit({preventDefault(){}});
   await tick();await tick();await tick();
   assert.equal(element('hud-notice').textContent,`${label==='AI'?'[AI 이븐]':'[번역] JA→KO'} | 프롬프트 처리 32%`);
   assert.match(element('hud-screen').innerHTML,/이전 답변 유지 확인/);
   assert.doesNotMatch(element('hud-screen').innerHTML,/프롬프트 처리|생각 중/);
   finish();await tick();await tick();await tick();
   assert.match(element('hud-screen').innerHTML,/새 답변/);
   assert.equal(element('hud-notice').textContent,`${label==='AI'?'[AI 이븐]':'[번역] JA→KO'} | 대기중`);
 }
 // Apple uses native prepare/STT and local translation without Whisper warmup.
 await queue.stopVoiceRecording(false);
 handler({menuItemClickEvent:{itemID:2}});await tick();
 element('translation-engine').value='apple';await element('translation-engine').handlers.change();
 element('stt-engine').value='apple';await element('stt-engine').handlers.change();
 const beforeAppleWarmup=requests.filter(r=>r.url.endsWith('/api/stt/warmup')).length;
 context.window.__queueTest.microphone();await tick();await tick();await tick();
 assert.equal(queue.state().engine,'apple');assert.equal(queue.state().recording,true);
 assert.equal(requests.filter(r=>r.url.endsWith('/api/stt/warmup')).length,beforeAppleWarmup);
 assert.ok(requests.some(r=>r.url.endsWith('/api/stt/apple/prepare')));
 nextPreviewText='native interim';appleText='Apple의 실시간 임시 번역';
 queue.liveTranslations.intervalMs=0;
 for(let i=0;i<6;i++)handler({audioEvent:{audioPcm:speech,speakerRole:'other'}});
 await tick();await tick();await tick();
 assert.ok(requests.some(r=>r.url.endsWith('/api/stt/preview') && JSON.parse(r.opts.body).engine==='apple'));
 assert.match(element('hud-screen').innerHTML,/Apple의 실시간 임시 번역/);
 appleText='Apple의 최종 번역';nextTranscript='native final';
 chatReply=sseReply('data: '+JSON.stringify({type:'message.delta',content:JSON.stringify({positive:'네',negative:'아니요'})})+'\n\n');
 for(let i=0;i<16;i++)handler({audioEvent:{audioPcm:Buffer.alloc(3200),speakerRole:'other'}});
 await tick();await tick();await tick();
 assert.equal(JSON.parse(requests.filter(r=>r.url.endsWith('/api/stt/transcribe')).at(-1).opts.body).engine,'apple');
 assert.match(element('hud-screen').innerHTML,/Apple의 최종 번역/);assert.doesNotMatch(element('hud-screen').innerHTML,/Apple의 실시간 임시 번역/);
 assert.match(element('chrome-translation-status').textContent,/Apple/);
 nextPreviewText=null;await queue.stopVoiceRecording(false);
 await element('prepare-chrome-translation').handlers.click();
 assert.ok(requests.some(r=>r.url.endsWith('/api/translate/apple/prepare')));
 element('translation-engine').value='chrome';await element('translation-engine').handlers.change();
 element('stt-engine').value='whisper';await element('stt-engine').handlers.change();
 // Independent LLM assignments: chat, hint, translation, and reply models.
 modelCatalog.push(...['chat-role','hint-role','translate-role','reply-role','new-base'].map(key=>({key,display_name:key})));
 await element('btn-refresh-models').handlers.click();
 for(const id of ['model-ai-chat','model-ai-hint','model-translation','model-translation-reply'])assert.equal(element(id).options[0].value,'');
 for(const [id,model] of [['model-ai-chat','chat-role'],['model-ai-hint','hint-role'],['model-translation','translate-role'],['model-translation-reply','reply-role']]){
   element(id).value=model;element(id).handlers.change();
 }
 await tick();await tick();
 handler({menuItemClickEvent:{itemID:1}});await tick();
 chatReply=sseReply('data: '+JSON.stringify({type:'message.delta',content:'채팅 전용 모델의 답변'})+'\n\n');
 element('chat-input').value='모델별 채팅 질문';element('text-chat-form').handlers.submit({preventDefault(){}});await tick();await tick();await tick();
 assert.equal(JSON.parse(requests.filter(r=>r.url.endsWith('/api/chat')).at(-1).opts.body).model,'chat-role');
 queue.ai();queue.hintGate.reset({forgetHistory:true});queue.hintGate.settleMs=0;queue.hintGate.intervalMs=0;
 element('hint-web-search').checked=false;await element('hint-web-search').handlers.change();hintDecision={hint:true,category:'background'};
 cueReply=JSON.stringify({topic:'RAG',question:'RAG의 구조는?',knownFacts:[],source:'internal'});
 chatReply=sseReply('data: '+JSON.stringify({type:'message.delta',content:JSON.stringify({fact:'RAG는 관련 문서를 검색해 그 근거를 바탕으로 답변을 생성하는 방식입니다.',adds_information:true})})+'\n\n');
 const roleHintsStart=requests.filter(r=>r.url.endsWith('/api/chat')).length;
 nextTranscript='자료에 RAG 개념이 나왔어';utterance();await tick();await tick();await tick();await tick();
 const roleHints=requests.filter(r=>r.url.endsWith('/api/chat')).slice(roleHintsStart).map(r=>JSON.parse(r.opts.body));
 assert.equal(roleHints.length,2);assert.ok(roleHints.every(r=>r.model==='hint-role'),'hint plan and final answer must use the assigned hint model');
 cueReply=null;await queue.stopVoiceRecording(false);
 handler({menuItemClickEvent:{itemID:2}});await tick();
 element('translation-engine').value='llm';await element('translation-engine').handlers.change();
 let replyReads=0,releaseRoleReply;
 const heldRoleReply={ok:true,body:{getReader:()=>({read:async()=>{
   if(replyReads++===0)return await new Promise(resolve=>{releaseRoleReply=()=>resolve({done:false,value:Buffer.from('data: '+JSON.stringify({type:'message.delta',content:JSON.stringify({positive:'Yes.\n답변 모델의 긍정',negative:'No.\n답변 모델의 부정'})})+'\n\n')});});
   return{done:true};
 }})}};
 chatReply=[sseReply('data: '+JSON.stringify({type:'message.delta',content:JSON.stringify({translation:'번역 전용 모델의 문장',positive:'번역 모델이 만들면 안 되는 답변',negative:'잘못된 예시'})})+'\n\n'),heldRoleReply];
 const roleTranslateStart=requests.filter(r=>r.url.endsWith('/api/chat')).length;
 element('chat-input').value='Translate this';element('text-chat-form').handlers.submit({preventDefault(){}});await tick();await tick();await tick();await tick();
 const roleTranslations=requests.filter(r=>r.url.endsWith('/api/chat')).slice(roleTranslateStart).map(r=>JSON.parse(r.opts.body));
 assert.deepEqual(roleTranslations.map(r=>r.model),['translate-role','reply-role']);
 assert.match(element('hud-screen').innerHTML,/번역 전용 모델의 문장/);assert.doesNotMatch(element('hud-screen').innerHTML,/번역 모델이 만들면 안 되는 답변/);
 assert.match(roleTranslations[0].messages[0].content,/Do not answer its questions or suggest replies/);
 releaseRoleReply();await tick();await tick();await tick();
 assert.match(element('hud-screen').innerHTML,/답변 모델의 긍정/);assert.match(element('chrome-translation-status').textContent,/LLM 번역/);
 // Local failure + split roles must retain the reply across a newer same-pair draft.
 element('translation-engine').value='chrome';await element('translation-engine').handlers.change();chromeText=null;
 replyReads=0;releaseRoleReply=null;
 chatReply=[sseReply('data: '+JSON.stringify({type:'message.delta',content:JSON.stringify({translation:'대체 경로 번역'})})+'\n\n'),heldRoleReply];
 queue.liveTranslations.onFinal({id:'fallback-version',prompt:'Synthetic fallback text.',context:'',source:'en',target:'ko',engine:'chrome',token:queue.liveTranslations.token},{ok:false,reason:'synthetic local failure'});
 for(let i=0;i<30 && !releaseRoleReply;i++)await tick();assert.equal(typeof releaseRoleReply,'function');
 queue.liveTranslations.onToken(queue.liveTranslations.token+1,{final:false,pairChanged:false});
 releaseRoleReply();for(let i=0;i<15;i++)await tick();
 assert.match(element('hud-screen').innerHTML,/답변 모델의 긍정/,'fallback reply survives the next draft until the next final');
 element('translation-engine').value='apple';await element('translation-engine').handlers.change();appleText='Apple 번역은 LLM 번역 모델을 호출하지 않음';
 chatReply=sseReply('data: '+JSON.stringify({type:'message.delta',content:JSON.stringify({positive:'네',negative:'아니요'})})+'\n\n');
 const nativeRoleStart=requests.filter(r=>r.url.endsWith('/api/chat')).length;
 element('chat-input').value='Native translation';element('text-chat-form').handlers.submit({preventDefault(){}});await tick();await tick();await tick();
 assert.deepEqual(requests.filter(r=>r.url.endsWith('/api/chat')).slice(nativeRoleStart).map(r=>JSON.parse(r.opts.body).model),['reply-role']);
 // Failed native writes cannot count as acknowledged translation paints.
 queue.liveTranslations.onText({id:'paint-baseline',draft:true},'BASELINE PAINT CONTENT');for(let i=0;i<15;i++)await tick();
 const normalRebuild=bridge.rebuildPageContainer;bridge.rebuildPageContainer=async()=>false;
 queue.forceRebuild(false);queue.liveTranslations.onText({id:'paint-failed',draft:true},'FAILED PAINT CONTENT');for(let i=0;i<15;i++)await tick();
 assert.equal(queue.metrics.entries.get('paint-failed')?.sdk_ms,undefined,'failed rebuild is not SDK success');assert.equal(queue.metrics.entries.get('paint-failed')?.sdk_failed,1);
 bridge.rebuildPageContainer=normalRebuild;queue.forceRebuild();for(let i=0;i<15;i++)await tick();
 const normalText=bridge.textContainerUpgrade;let textFailures=0;
 bridge.textContainerUpgrade=async payload=>{if(payload.containerID===1){textFailures++;return false;}return normalText(payload);};
 queue.liveTranslations.onText({id:'paint-text-failed',draft:true},'TEXT FAILURE CONTENT');for(let i=0;i<15;i++)await tick();
 assert.ok(textFailures>0);assert.equal(queue.metrics.entries.get('paint-text-failed')?.sdk_failed,1);assert.equal(queue.metrics.entries.get('paint-text-failed')?.sdk_ms,undefined,'failed partial text push is not SDK success');
 bridge.textContainerUpgrade=normalText;let releaseOldPaint,releaseNewPaint;
 bridge.textContainerUpgrade=async payload=>{
  if(payload.containerID===1 && payload.content?.includes('NEW PAINT CONTENT'))await new Promise(resolve=>{releaseNewPaint=resolve;});
  else if(payload.containerID===1 && payload.content?.includes('OLD PAINT CONTENT'))await new Promise(resolve=>{releaseOldPaint=resolve;});
  return normalText(payload);
 };
 queue.liveTranslations.onText({id:'paint-old',draft:true},'OLD PAINT CONTENT');for(let i=0;i<15 && !releaseOldPaint;i++)await tick();assert.equal(typeof releaseOldPaint,'function');
 queue.liveTranslations.onText({id:'paint-new',draft:true},'NEW PAINT CONTENT');releaseOldPaint();for(let i=0;i<15 && !releaseNewPaint;i++)await tick();
 assert.ok(Number.isFinite(queue.metrics.entries.get('paint-old')?.sdk_ms));assert.equal(queue.metrics.entries.get('paint-new')?.sdk_ms,undefined,'old payload ACK must not acknowledge newer paint');
 releaseNewPaint();for(let i=0;i<15;i++)await tick();assert.ok(Number.isFinite(queue.metrics.entries.get('paint-new')?.sdk_ms));bridge.textContainerUpgrade=normalText;
 // Native-app JSON contract: retain PCM until the final archive response,
 // and do not let a preview interrupt the pending final inference.
 let releaseSTTSave;const streamStart=requests.length;chromeText='JSON 저장 후 표시된 번역';
 element('translation-engine').value='chrome';await element('translation-engine').handlers.change();queue.record();
 sttReplies=[opts=>{
  const request=JSON.parse(opts.body);assert.equal(request.stream,false);
  return new Promise(resolve=>{releaseSTTSave=()=>resolve({ok:true,json:async()=>({text:'Synthetic JSON final.',saved:true,sessionId:request.sessionId,segmentId:request.segmentId,language:'en',engine:request.engine})});});
 }];
 queue.enqueueClip(Buffer.alloc(6400),true,Date.now(),'json-save-final');for(let i=0;i<25 && !releaseSTTSave;i++)await tick();
 assert.equal(typeof releaseSTTSave,'function');assert.ok(queue.state().ids.includes('json-save-final'),'original audio is retained until JSON confirms saving');
 const waitingPreviewCount=requests.slice(streamStart).filter(r=>r.url.endsWith('/api/stt/preview')).length;
 queue.previewDuringSave('newer-preview-during-save');for(let i=0;i<20;i++)await tick();
 assert.equal(requests.slice(streamStart).filter(r=>r.url.endsWith('/api/stt/preview')).length,waitingPreviewCount,'JSON finalization owns the recognizer until complete');
 releaseSTTSave();for(let i=0;i<20;i++)await tick();assert.ok(!queue.state().ids.includes('json-save-final'));assert.match(element('transcript-log').textContent,/Synthetic JSON final/);
 assert.ok(requests.slice(streamStart).some(r=>r.url.endsWith('/api/stt/preview')),'the latest preview resumes after the JSON result');
 await queue.stopVoiceRecording(false);chromeText=null;
 element('model-select').value='new-base';await element('model-select').handlers.change();
 assert.match(element('model-ai-chat').options[0].textContent,/new-base/);assert.equal(element('model-ai-chat').value,'chat-role','base change must not reset pinned models');
 element('model-ai-hint').value='';await element('model-ai-hint').handlers.change();
 queue.ai();queue.hintGate.reset({forgetHistory:true});hintDecision={hint:true,category:'background'};
 cueReply=JSON.stringify({topic:'양자역학',cue_type:'concept',source:'internal',fact:'양자역학에서는 입자의 상태를 확률 진폭으로 기술하며 측정 결과는 확률로 예측합니다.',adds_information:true});
 nextTranscript='양자역학의 의미를 알려줘';utterance();await tick();await tick();await tick();
 assert.equal(JSON.parse(requests.filter(r=>r.url.endsWith('/api/chat')).at(-1).opts.body).model,'new-base','JEV-selected ambient questions use hint model inheritance');cueReply=null;
 await queue.stopVoiceRecording(false);
 // Engine/language changes prepare automatically, without pressing either button.
 handler({menuItemClickEvent:{itemID:1}});await tick();
 const chatTimer=context.setTimeout,chatDurations=[];
 context.setTimeout=(fn,ms)=>{chatDurations.push(ms);return chatTimer(fn,ms);};
 chatReply=null;holdChat=true;
 element('chat-input').value='도구와 추론이 오래 걸리는 질문';element('text-chat-form').handlers.submit({preventDefault(){}});
 await tick();await tick();
 assert.ok(chatDurations.includes(330000));assert.ok(!chatDurations.includes(45000),'explicit AI chat must not retain the 45-second cut-off');
 releaseChat();await tick();await tick();await tick();context.setTimeout=chatTimer;
 assert.doesNotMatch(element('chat-output').textContent,/AI 오류/);
 let partialRead=0;
 chatReply={ok:true,body:{getReader:()=>({read:async()=>{
   if(partialRead++===0)return{done:false,value:Buffer.from('data: '+JSON.stringify({type:'message.delta',content:'이미 받은 유용한 답변'})+'\n\n')};
   throw new TypeError('Load failed');
 }})}};
 element('chat-input').value='부분 답변 뒤 연결 끊김';element('text-chat-form').handlers.submit({preventDefault(){}});await tick();await tick();await tick();
 assert.match(element('hud-screen').innerHTML,/이미 받은 유용한 답변/,'network failure must preserve answer already received');
 assert.match(element('chat-output').textContent,/응답 중단/);assert.match(element('hint-status').textContent,/연결 오류/);
 handler({menuItemClickEvent:{itemID:2}});
 element('translate-source-lang').value='en';await element('translate-source-lang').handlers.change();
 element('translate-target-lang').value='ko';await element('translate-target-lang').handlers.change();
 element('translation-engine').value='chrome';await element('translation-engine').handlers.change();
 element('stt-engine').value='chrome-online';await element('stt-engine').handlers.change();
 await tick();await tick();await tick();
 const autoStart=requests.length;
 element('translate-source-lang').value='ja';await element('translate-source-lang').handlers.change();
 await tick();await tick();await tick();
 assert.ok(requests.slice(autoStart).some(r=>r.url.endsWith('/api/translate/chrome/prepare') && JSON.parse(r.opts.body).source==='ja' && JSON.parse(r.opts.body).target==='ko'));
 assert.ok(requests.slice(autoStart).some(r=>r.url.endsWith('/api/stt/chrome/prepare') && JSON.parse(r.opts.body).language==='ja'));
 const appleAutoStart=requests.length;
 element('translation-engine').value='apple';await element('translation-engine').handlers.change();
 element('stt-engine').value='apple';await element('stt-engine').handlers.change();
 await tick();await tick();await tick();
 assert.ok(requests.slice(appleAutoStart).some(r=>r.url.endsWith('/api/translate/apple/prepare') && JSON.parse(r.opts.body).source==='ja'));
 assert.ok(requests.slice(appleAutoStart).some(r=>r.url.endsWith('/api/stt/apple/prepare') && JSON.parse(r.opts.body).language==='ja'));
 const targetAutoStart=requests.length;
 element('translate-target-lang').value='en';await element('translate-target-lang').handlers.change();
 await tick();await tick();
 assert.ok(requests.slice(targetAutoStart).some(r=>r.url.endsWith('/api/translate/apple/prepare') && JSON.parse(r.opts.body).target==='en'));
 assert.equal(requests.slice(targetAutoStart).filter(r=>r.url.endsWith('/api/stt/apple/prepare')).length,0,'explicit source unchanged: target-only change must reuse STT preparation');
 element('translate-source-lang').value='auto';await element('translate-source-lang').handlers.change();await tick();await tick();
 const autoSourceStart=requests.length;
 element('translate-target-lang').value='ko';await element('translate-target-lang').handlers.change();await tick();await tick();
 assert.ok(requests.slice(autoSourceStart).some(r=>r.url.endsWith('/api/stt/apple/prepare') && JSON.parse(r.opts.body).language==='en'),'automatic source must be prepared again when target changes its inferred language');
 assert.match(element('hud-notice').textContent,/^\[번역\] AUTO→KO /,'header must show the selected automatic source, not the engine fallback language');
 const modeAutoStart=requests.length;
 handler({menuItemClickEvent:{itemID:1}});await tick();await tick();
 assert.ok(requests.slice(modeAutoStart).some(r=>r.url.endsWith('/api/stt/apple/prepare') && JSON.parse(r.opts.body).language==='ko'),'AI mode prepares the user language');
 assert.match(element('stt-engine-status').textContent,/Apple STT 준비 완료.*ko/);
 assert.equal(element('prepare-chrome-stt').disabled,false);assert.equal(element('prepare-chrome-translation').disabled,false);
 // AI preview and final requests must honor recognition language.
 for(const sourceLang of ['auto','en','ja','ko']) {
   for(const revision of [0,1]) {
     const payload=context.window.__queueTest.sttPayload({mode:'ai',engine:'whisper',sourceLang,targetLang:'ko',bytes:new Uint8Array(6400),revision});
     assert.equal(payload.language,sourceLang);
     assert.equal(payload.realtime,true);
   }
 }
 for(const engine of ['apple','chrome-online']) {
   for(const revision of [0,1]) {
     const clip={engine,sourceLang:'en',targetLang:'ja',bytes:new Uint8Array(6400),revision};
     assert.equal(context.window.__queueTest.sttPayload({...clip,mode:'ai'}).language,'ja');
     assert.equal(context.window.__queueTest.sttPayload({...clip,mode:'translate'}).language,'en');
   }
 }
 // Glasses-only quiet capture and gain use one immutable PCM pass.
 assert.equal(element('glasses-sensitivity').value,'high');assert.equal(element('glasses-gain').value,'1');
 element('glasses-gain').value='2';await element('glasses-gain').handlers.change();await tick();
 assert.equal(JSON.parse(nativeSaved).glassesInput.gain,2);
 context.window.__queueTest.microphone();await tick();await tick();await tick();
 assert.equal(element('glasses-sensitivity').disabled,false);assert.equal(element('glasses-gain').disabled,false);
 const gainCalls=audioCalls.length;
 element('glasses-gain').value='4';await element('glasses-gain').handlers.change();assert.equal(element('glasses-gain').value,'4');
 element('glasses-gain').value='2';await element('glasses-gain').handlers.change();assert.equal(audioCalls.length,gainCalls,'live gain changes never restart the microphone');
 const quietInput=Buffer.alloc(3200);for(let i=0;i<quietInput.length;i+=2)quietInput.writeInt16LE(150,i);
 const quietOriginal=Buffer.from(quietInput),quietPreviewStart=requests.filter(r=>r.url.endsWith('/api/stt/preview')).length;
 nextPreviewText='작은 목소리 테스트';
 for(let i=0;i<6;i++)handler({audioEvent:{audioPcm:quietInput,source:'glasses',speakerRole:'other'}});
 await tick();await tick();await tick();
 const quietPreviews=requests.filter(r=>r.url.endsWith('/api/stt/preview')).slice(quietPreviewStart);
 assert.ok(quietPreviews.length,'lower glasses gate must capture quiet input below the former threshold');
 for(const r of quietPreviews){const pcm=Buffer.from(JSON.parse(r.opts.body).audio,'base64');for(let i=0;i<pcm.length;i+=2)assert.equal(pcm.readInt16LE(i),300,'preview audio must be amplified once, not again on each cumulative request');}
 assert.deepEqual(quietInput,quietOriginal);assert.match(element('glasses-input-level').textContent,/-47 dBFS.*-41 dBFS/);
 handler({menuItemClickEvent:{itemID:2}});await tick();await tick();await new Promise(resolve=>setTimeout(resolve,160));
 element('translate-exclude-self').checked=true;await element('translate-exclude-self').handlers.change();
 const excludedStart=requests.filter(r=>r.url.endsWith('/api/stt/preview')).length;
 for(let i=0;i<6;i++)handler({audioEvent:{audioPcm:speech,source:'glasses',speakerRole:'self'}});
 await tick();await tick();assert.equal(requests.filter(r=>r.url.endsWith('/api/stt/preview')).length,excludedStart,'boosting must not bypass explicit self-speech exclusion');
 assert.match(element('glasses-input-level').textContent,/화자 필터로 제외/);
 await queue.stopVoiceRecording(false);await tick();nextPreviewText=null;handler({menuItemClickEvent:{itemID:1}});await tick();
 assert.equal(element('glasses-sensitivity').disabled,false);assert.equal(element('glasses-gain').disabled,false);
 element('glasses-sensitivity').value='very_high';await element('glasses-sensitivity').handlers.change();element('glasses-gain').value='4';await element('glasses-gain').handlers.change();await tick();
 // Phone input uses the same PCM/STT pipeline, with source pinned until stop.
 element('audio-input-source').value='phone';await element('audio-input-source').handlers.change();await tick();
 assert.equal(JSON.parse(nativeSaved).audioInputSource,'phone');
 assert.equal(element('glasses-input-settings').hidden,true);
 context.window.__queueTest.microphone();await tick();await tick();await tick();
 assert.deepEqual(audioCalls.filter(call=>call[0]).at(-1),[true,'phone']);
 assert.ok(element('hud-mic-icon').src.endsWith('phone-mic-on.png'),'image must reflect the confirmed phone microphone, not AI processing');
 assert.equal(element('audio-input-source').disabled,false);
 element('audio-input-source').value='glasses';await element('audio-input-source').handlers.change();
 assert.equal(element('audio-input-source').value,'glasses','active source changes close and reopen the microphone');
 assert.equal(queue.state().recording,true);
 element('audio-input-source').value='phone';await element('audio-input-source').handlers.change();
 const phonePreviewStart=requests.filter(r=>r.url.endsWith('/api/stt/preview')).length;
 for(let i=0;i<6;i++)handler({audioEvent:{audioPcm:quietInput,source:'phone',speakerRole:'unknown'}});
 await tick();assert.equal(requests.filter(r=>r.url.endsWith('/api/stt/preview')).length,phonePreviewStart,'phone keeps the original gate even when glasses sensitivity/gain are maximum');
 for(let i=0;i<6;i++)handler({audioEvent:{audioPcm:speech,source:'glasses',speakerRole:'unknown'}});
 await tick();assert.equal(requests.filter(r=>r.url.endsWith('/api/stt/preview')).length,phonePreviewStart,'old glasses frames must not mix into phone recording');
 nextPreviewText='휴대폰에서 들어오는 음성';
 for(let i=0;i<6;i++)handler({audioEvent:{audioPcm:speech,source:'phone',speakerRole:'unknown'}});
 await tick();await tick();
 assert.ok(requests.filter(r=>r.url.endsWith('/api/stt/preview')).length>phonePreviewStart,'phone PCM with unknown speaker uses the existing STT path');
 for(const r of requests.filter(r=>r.url.endsWith('/api/stt/preview')).slice(phonePreviewStart)){const pcm=Buffer.from(JSON.parse(r.opts.body).audio,'base64');for(let i=0;i<pcm.length;i+=2)assert.ok([150,5000].includes(pcm.readInt16LE(i)),'phone samples must remain unamplified');}
 const switchCalls=audioCalls.length;
 nextTranscript='메뉴 전환 전에 녹음한 음성';
 handler({menuItemClickEvent:{itemID:3}});handler({menuItemClickEvent:{itemID:3}});
 await tick();await tick();await tick();await tick();await tick();await tick();
 assert.equal(element('audio-input-source').value,'glasses','menu switches phone to glasses');
 assert.equal(queue.state().recording,true,'recording resumes on the new source');
 assert.equal(audioCalls[switchCalls][0],false,'old microphone is closed before opening the new one');
 assert.deepEqual(audioCalls.filter(call=>call[0]).at(-1),[true,'glasses']);
 assert.equal(audioCalls.slice(switchCalls).filter(call=>call[0]).length,1,'repeated menu taps must not race or open two microphones');
 assert.match(element('transcript-log').textContent,/메뉴 전환 전에 녹음한 음성/,'switch must flush the old recording into transcript storage');
 assert.ok(element('hud-mic-icon').src.endsWith('glasses-mic-on.png'));
 await queue.stopVoiceRecording(false);await tick();nextPreviewText=null;
 assert.equal(audioCalls.at(-1)[0],false);assert.equal(element('audio-input-source').disabled,false);
 const idleSourceOpens=audioCalls.filter(call=>call[0]).length;
 handler({menuItemClickEvent:{itemID:3}});await tick();await tick();
 assert.equal(element('audio-input-source').value,'phone');assert.equal(queue.state().recording,false,'idle menu switch must not start recording');
 assert.equal(audioCalls.filter(call=>call[0]).length,idleSourceOpens);
 assert.ok(element('hud-mic-icon').src.endsWith('phone-mic-off.png'));
 assert.ok(imageUpdates.some(u=>u.containerID===5 && u.imageData.length===288));
 assert.ok(imageUpdates.some(u=>u.containerID===6 && u.imageData.length===288));
 for(let i=0;i<150 && !imageUpdates.some(u=>u.containerID===11 && u.imageData.length===288);i++)await new Promise(resolve=>setTimeout(resolve,20));
 assert.ok(imageUpdates.some(u=>u.containerID===11 && u.imageData.length===288));
 // A native open acknowledgement without PCM must show phone/iOS guidance.
 const beforeNoAudioTimer=context.setTimeout;let noPhoneAudio;
 context.setTimeout=(fn,ms)=>ms===6000?(noPhoneAudio=fn,beforeNoAudioTimer(()=>{},60000)):beforeNoAudioTimer(fn,ms);
 context.window.__queueTest.microphone();await tick();await tick();await tick();
 context.setTimeout=beforeNoAudioTimer;assert.equal(typeof noPhoneAudio,'function');
 noPhoneAudio();await tick();await tick();await tick();
 assert.equal(queue.state().recording,false);assert.match(queue.state().failure,/휴대폰 마이크 입력이 없습니다/);assert.match(queue.state().failure,/휴대폰 설정/);
 // A failed model catalog must not prevent independent Apple capability discovery.
 const capabilitiesBefore=requests.filter(r=>r.url.endsWith('/api/apple/status')).length;
 modelFailure=true;await element('btn-refresh-models').handlers.click();await tick();modelFailure=false;
 assert.ok(requests.filter(r=>r.url.endsWith('/api/apple/status')).length>capabilitiesBefore);
 // Native engine menus must retain Apple while capability controls are initially disabled/unavailable.
 for(const [id,option] of [[6,'translation-option-apple'],[7,'stt-option-apple']]){
   element(option).disabled=true;
   handler({menuItemClickEvent:{itemID:id}});
   for(let i=0;i<150 && rebuilds.at(-1)?.textObject?.[0]?.content!==(id===6?'번역 엔진':'음성 인식 엔진');i++)await new Promise(resolve=>setTimeout(resolve,20));
   assert.equal(rebuilds.at(-1).listObject[0].itemContainer.itemCount,id===7?4:3);
   assert.ok(rebuilds.at(-1).listObject[0].itemContainer.itemName.some(label=>label.includes('Apple 로컬')));
   handler({menuItemClickEvent:{itemID:90}});await tick();element(option).disabled=false;
 }
 // Native selection pages must not apply a value on scroll or accidentally start capture.
 const menuOpens=audioCalls.filter(call=>call[0]).length;
 handler({menuItemClickEvent:{itemID:4}});
 for(let i=0;i<100 && rebuilds.at(-1)?.textObject?.[0]?.content!=='인식 언어';i++)await new Promise(resolve=>setTimeout(resolve,20));
 const pickerPage=rebuilds.at(-1);
 assert.equal(pickerPage.listObject[0].itemContainer.itemCount,10);
 assert.equal(pickerPage.textObject[0].content,'인식 언어');
 const beforePickerLanguage=JSON.parse(nativeSaved).translateSourceLang;
 handler({listEvent:{containerID:30,eventType:2,currentSelectItemIndex:3}});await tick();
 assert.equal(JSON.parse(nativeSaved).translateSourceLang,beforePickerLanguage,'scrolling only moves selection');
 handler({listEvent:{containerID:30,currentSelectItemIndex:3}});await tick();await tick();await tick();
 assert.equal(JSON.parse(nativeSaved).translateSourceLang,'ja','omitted zero click event selects the native row');
 assert.equal(audioCalls.filter(call=>call[0]).length,menuOpens,'idle selection must not open a mic');
 for(const [id,key,index,want] of [[5,'translateTargetLang',1,'en'],[6,'translationEngine',0,'llm'],[7,'sttEngine',0,'whisper'],[8,'translateExcludeSelf',1,false]]){
   handler({menuItemClickEvent:{itemID:id}});
   handler({listEvent:{containerID:30,eventType:0,currentSelectItemIndex:index}});
   await tick();await tick();await tick();
   assert.equal(JSON.parse(nativeSaved)[key],want);
 }
 await queue.settings({audioInputSource:'glasses'});
 handler({menuItemClickEvent:{itemID:9}});
 handler({listEvent:{containerID:30,currentSelectItemIndex:0}});await tick();await tick();await tick();
 assert.equal(JSON.parse(nativeSaved).glassesInput.sensitivity,'standard');
 handler({menuItemClickEvent:{itemID:4}});handler({sysEvent:{eventType:9}});await tick();
 assert.equal(shutdown,1,'picker back must not exit the app');
 // Language hub (Level 2 with value previews, 2-option inline toggle -> Level 3 -> double-tap back to Level 2 -> double-tap back to HUD).
 handler({menuItemClickEvent:{itemID:13}});
 for(let i=0;i<100 && rebuilds.at(-1)?.textObject?.[0]?.content!=='언어 설정';i++)await new Promise(resolve=>setTimeout(resolve,20));
 assert.equal(rebuilds.at(-1).textObject[0].content,'언어 설정');
 assert.equal(rebuilds.at(-1).listObject[0].itemContainer.itemCount,3);
 assert.ok(rebuilds.at(-1).listObject[0].itemContainer.itemName[0].startsWith('인식 언어 - '));
 assert.ok(rebuilds.at(-1).listObject[0].itemContainer.itemName[1].startsWith('번역 언어 - '));
 assert.equal(rebuilds.at(-1).listObject[0].itemContainer.itemName[2],'내 목소리 제외 - 제외 안함');
 // Tapping 2-option '내 목소리 제외' toggles inline without entering Level 3.
 handler({listEvent:{containerID:30,eventType:0,currentSelectItemIndex:2}});
 for(let i=0;i<100 && rebuilds.at(-1)?.listObject?.[0]?.itemContainer?.itemName?.[2]!=='내 목소리 제외 - 제외함';i++)await new Promise(resolve=>setTimeout(resolve,20));
 assert.equal(rebuilds.at(-1).textObject[0].content,'언어 설정','2-option setting must toggle inline in Level 2');
 assert.equal(JSON.parse(nativeSaved).translateExcludeSelf,true);
 // Toggle back to restore prior value.
 handler({listEvent:{containerID:30,eventType:0,currentSelectItemIndex:2}});
 for(let i=0;i<100 && rebuilds.at(-1)?.listObject?.[0]?.itemContainer?.itemName?.[2]!=='내 목소리 제외 - 제외 안함';i++)await new Promise(resolve=>setTimeout(resolve,20));
 assert.equal(JSON.parse(nativeSaved).translateExcludeSelf,false);
 handler({listEvent:{containerID:30,eventType:0,currentSelectItemIndex:0}});
 for(let i=0;i<100 && rebuilds.at(-1)?.textObject?.[0]?.content!=='인식 언어';i++)await new Promise(resolve=>setTimeout(resolve,20));
 assert.equal(rebuilds.at(-1).textObject[0].content,'인식 언어');
 handler({listEvent:{containerID:30,eventType:3,currentSelectItemIndex:0}});
 for(let i=0;i<100 && rebuilds.at(-1)?.textObject?.[0]?.content!=='언어 설정';i++)await new Promise(resolve=>setTimeout(resolve,20));
 assert.equal(rebuilds.at(-1).textObject[0].content,'언어 설정','double-tap in listEvent must return to parent Level 2');
 handler({listEvent:{containerID:30,eventType:3,currentSelectItemIndex:0}});await tick();
 // Display hub (Level 2: 3 inline toggles + Level 3 subtitleLines picker).
 handler({menuItemClickEvent:{itemID:10}});
 for(let i=0;i<100 && rebuilds.at(-1)?.textObject?.[0]?.content!=='화면 표시';i++)await new Promise(resolve=>setTimeout(resolve,20));
 assert.equal(rebuilds.at(-1).listObject[0].itemContainer.itemCount,4);
 assert.equal(rebuilds.at(-1).listObject[0].itemContainer.itemName[0],'전사문 숨기기 - 3줄');
 assert.equal(rebuilds.at(-1).listObject[0].itemContainer.itemName[1],'AI모드 상단 레이아웃 - 기본');
 assert.equal(rebuilds.at(-1).listObject[0].itemContainer.itemName[2],'번역모드 레이아웃 - 기본');
 assert.equal(rebuilds.at(-1).listObject[0].itemContainer.itemName[3],'자막 표시 줄 수 - 2줄');
 handler({listEvent:{containerID:30,eventType:0,currentSelectItemIndex:1}});
 for(let i=0;i<100 && rebuilds.at(-1)?.listObject?.[0]?.itemContainer?.itemName?.[1]!=='AI모드 상단 레이아웃 - 간결';i++)await new Promise(resolve=>setTimeout(resolve,20));
 assert.equal(JSON.parse(nativeSaved).aiHeaderLayout,'status-icon');
 handler({listEvent:{containerID:30,eventType:0,currentSelectItemIndex:1}});
 for(let i=0;i<100 && rebuilds.at(-1)?.listObject?.[0]?.itemContainer?.itemName?.[1]!=='AI모드 상단 레이아웃 - 기본';i++)await new Promise(resolve=>setTimeout(resolve,20));
 handler({listEvent:{containerID:30,eventType:0,currentSelectItemIndex:2}});
 for(let i=0;i<100 && rebuilds.at(-1)?.listObject?.[0]?.itemContainer?.itemName?.[2]!=='번역모드 레이아웃 - 간결';i++)await new Promise(resolve=>setTimeout(resolve,20));
 assert.equal(JSON.parse(nativeSaved).translationLayout,'subtitles');
 handler({listEvent:{containerID:30,eventType:0,currentSelectItemIndex:2}});
 for(let i=0;i<100 && rebuilds.at(-1)?.listObject?.[0]?.itemContainer?.itemName?.[2]!=='번역모드 레이아웃 - 기본';i++)await new Promise(resolve=>setTimeout(resolve,20));
 handler({listEvent:{containerID:30,eventType:0,currentSelectItemIndex:3}});
 for(let i=0;i<100 && rebuilds.at(-1)?.textObject?.[0]?.content!=='자막 표시 줄 수';i++)await new Promise(resolve=>setTimeout(resolve,20));
 handler({listEvent:{containerID:30,eventType:0,currentSelectItemIndex:3}});
 for(let i=0;i<100 && rebuilds.at(-1)?.textObject?.[0]?.content!=='화면 표시';i++)await new Promise(resolve=>setTimeout(resolve,20));
 assert.equal(rebuilds.at(-1).listObject[0].itemContainer.itemName[3],'자막 표시 줄 수 - 4줄');
 assert.equal(JSON.parse(nativeSaved).subtitleLines,4);
 handler({listEvent:{containerID:30,eventType:0,currentSelectItemIndex:3}});
 for(let i=0;i<100 && rebuilds.at(-1)?.textObject?.[0]?.content!=='자막 표시 줄 수';i++)await new Promise(resolve=>setTimeout(resolve,20));
 handler({listEvent:{containerID:30,eventType:0,currentSelectItemIndex:1}});
 for(let i=0;i<100 && rebuilds.at(-1)?.textObject?.[0]?.content!=='화면 표시';i++)await new Promise(resolve=>setTimeout(resolve,20));
 handler({listEvent:{containerID:30,eventType:3,currentSelectItemIndex:0}});await tick();
 // Settings hub (Level 2 with glasses gain -> Level 3 -> back to Level 2 -> back to HUD).
 handler({menuItemClickEvent:{itemID:12}});
 for(let i=0;i<100 && rebuilds.at(-1)?.textObject?.[0]?.content!=='설정';i++)await new Promise(resolve=>setTimeout(resolve,20));
 assert.equal(rebuilds.at(-1).textObject[0].content,'설정');
 assert.equal(rebuilds.at(-1).listObject[0].itemContainer.itemCount,4);
 assert.ok(rebuilds.at(-1).listObject[0].itemContainer.itemName[0].startsWith('번역 엔진 - '));
 assert.ok(rebuilds.at(-1).listObject[0].itemContainer.itemName[3].startsWith('안경 입력 증폭 - '));
 handler({listEvent:{containerID:30,eventType:0,currentSelectItemIndex:3}});
 for(let i=0;i<100 && rebuilds.at(-1)?.textObject?.[0]?.content!=='안경 입력 증폭';i++)await new Promise(resolve=>setTimeout(resolve,20));
 assert.equal(rebuilds.at(-1).textObject[0].content,'안경 입력 증폭');
 handler({listEvent:{containerID:30,eventType:0,currentSelectItemIndex:2}});
 for(let i=0;i<100 && rebuilds.at(-1)?.textObject?.[0]?.content!=='설정';i++)await new Promise(resolve=>setTimeout(resolve,20));
 assert.equal(rebuilds.at(-1).listObject[0].itemContainer.itemName[3],'안경 입력 증폭 - 2배');
 assert.equal(JSON.parse(nativeSaved).glassesInput.gain,2);
 handler({menuItemClickEvent:{itemID:90}});await tick();
 // Role-based LLM model menu (Level 2 roles -> Level 3 model select -> apply & return to Level 2).
 const savedChatRole=JSON.parse(nativeSaved).roleModels?.chat || '';
 handler({menuItemClickEvent:{itemID:11}});
 for(let i=0;i<100 && rebuilds.at(-1)?.textObject?.[0]?.content!=='역할별 LLM 모델';i++)await new Promise(resolve=>setTimeout(resolve,20));
 assert.equal(rebuilds.at(-1).listObject[0].itemContainer.itemCount,5);
 assert.ok(rebuilds.at(-1).listObject[0].itemContainer.itemName[0].startsWith('AI 채팅 - '));
 handler({listEvent:{containerID:30,eventType:0,currentSelectItemIndex:0}});
 for(let i=0;i<100 && rebuilds.at(-1)?.textObject?.[0]?.content!=='역할별 LLM · AI 채팅';i++)await new Promise(resolve=>setTimeout(resolve,20));
 assert.equal(rebuilds.at(-1).textObject[0].content,'역할별 LLM · AI 채팅');
 handler({listEvent:{containerID:30,eventType:0,currentSelectItemIndex:1}});
 for(let i=0;i<100 && rebuilds.at(-1)?.textObject?.[0]?.content!=='역할별 LLM 모델';i++)await new Promise(resolve=>setTimeout(resolve,20));
 assert.equal(rebuilds.at(-1).textObject[0].content,'역할별 LLM 모델');
 assert.ok(JSON.parse(nativeSaved).roleModels.chat,'selected role model must persist');
 // Restore previous chat role so downstream persistence assertions see the original state.
 element('model-ai-chat').value=savedChatRole;element('model-ai-chat').handlers.change();await tick();
 handler({menuItemClickEvent:{itemID:90}});await tick();
 // Record, change each phone setting, and verify a confirmed close precedes one resumed open.
 context.window.__queueTest.microphone();await tick();await tick();await tick();
 assert.equal(queue.state().recording,true);
 for(const patch of [{translateSourceLang:'ko'},{translateTargetLang:'ja'},{translationEngine:'apple'},{sttEngine:'apple'},{translateExcludeSelf:true},{glassesInput:{sensitivity:'very_high',gain:4}}]){
   const calls=audioCalls.length;
   await queue.settings(patch);
   assert.equal(queue.state().recording,true);
   if(patch.translationEngine || patch.glassesInput){assert.equal(audioCalls.length,calls,'translation and gain controls must not restart STT');}
   else{assert.equal(audioCalls[calls][0],false,'settings transaction confirms the old microphone off first');
   assert.equal(audioCalls.slice(calls).filter(call=>call[0]).length,1);}
 }
 const noOpCalls=audioCalls.length;await queue.settings({sttEngine:'apple'});
 assert.equal(audioCalls.length,noOpCalls,'selecting the current setting must not restart');
 await queue.settings({glassesInput:{sensitivity:'auto',gain:'auto'}});
 assert.equal(audioCalls.length,noOpCalls);assert.equal(JSON.parse(nativeSaved).glassesInput.gain,'auto');
 await queue.settings({glassesInput:{sensitivity:'very_high',gain:4}});
 // Audio in flight retains the old engine/language and is archived without routing under new settings.
 let finishOld;
 sttReplies=[()=>new Promise(resolve=>{finishOld=()=>resolve({ok:true,json:async()=>({text:'이전 설정 보관 음성',saved:true})});})];
 const oldRequests=requests.length;
 queue.enqueueClip(Buffer.alloc(6400),true,Date.now(),'old-settings');await tick();
 await queue.settings({sttEngine:'whisper',translateSourceLang:'en'});
 finishOld();await tick();await tick();
 const oldBody=JSON.parse(requests.slice(oldRequests).find(r=>r.url.endsWith('/api/stt/transcribe')).opts.body);
 assert.equal(oldBody.engine,'apple');assert.equal(oldBody.language,'ja');
 assert.match(element('transcript-log').textContent,/이전 설정 보관 음성/);
 assert.equal(queue.state().recording,true);
 // Rapid updates during warmup coalesce; an explicit tap prevents automatic restart.
 holdWarmup=true;
 const firstChange=queue.settings({translateSourceLang:'ja'});await tick();await tick();await tick();
 assert.equal(typeof releaseWarmup,'function');
 const latestChange=queue.settings({translateSourceLang:'en'});
 context.window.__queueTest.microphone();releaseWarmup();
 await firstChange;await latestChange;await tick();
 assert.equal(queue.state().recording,false,'manual stop during settings preparation cancels resume');
 assert.equal(JSON.parse(nativeSaved).translateSourceLang,'en','latest language wins');
 // Failed native close must leave the old setting selected and never open a replacement mic.
 context.window.__queueTest.microphone();await tick();await tick();await tick();
 assert.equal(queue.state().recording,true);
 const beforeFailedChange=audioCalls.filter(call=>call[0]).length;
 const sourceBeforeFailure=JSON.parse(nativeSaved).audioInputSource;
 bridge.audioControl=async(open,...args)=>{audioCalls.push([open,...args]);return open;};
 await queue.settings({audioInputSource:'phone'});
 assert.equal(audioCalls.filter(call=>call[0]).length,beforeFailedChange);
 assert.equal(JSON.parse(nativeSaved).audioInputSource,sourceBeforeFailure);
 assert.match(element('hint-status').textContent,/설정 변경 실패/);
 bridge.audioControl=normalAudioControl;await queue.stopVoiceRecording(false);
 await queue.settings({translateTargetLang:'ko'}); // Korean fixture bodies in the existing hint tests.
 // Small-model judgment bypasses negative/missing JEV gating and feeds the writer without re-planning.
 queue.ai();queue.hintGate.intervalMs=0;queue.hintGate.settleMs=0;queue.hintGate.generationIntervalMs=0;
 const additionalJudge='@llm/remote/'+Buffer.from('gemma-e2b').toString('base64url'),additionalHint='@llm/remote/'+Buffer.from('gemma-e4b').toString('base64url');
 element('model-hint-decision').value=additionalJudge;await element('model-hint-decision').handlers.change();
 element('model-ai-hint').value=additionalHint;await element('model-ai-hint').handlers.change();
 element('hint-context-tokens').value='300';await element('hint-context-tokens').handlers.change();
 hintDecision={hint:false};judgeReply={hint:true,kind:'key',topic:'피라미드',evidence:'피라미드',question:'기자 피라미드 건립 시기와 건축 기술',source:'internal'};
 chatReply=sseReply('data: '+JSON.stringify({choices:[{delta:{content:JSON.stringify({fact:'기자의 대피라미드는 쿠푸왕을 위해 건립됐으며 석회암 블록으로 쌓았습니다.',adds_information:true,kind:'key'})}}]})+'\n\ndata: [DONE]\n\n');
 const smallJudgeStart=requests.length;
 queue.preview({mode:'ai',segmentId:'judge-end-to-end'},{text:'피라미드가 왕의 무덤이래.'},{stable:'피라미드가 왕의 무덤이래.'});
 for(let i=0;i<15;i++)await tick();
 const judgedBodies=requests.slice(smallJudgeStart).filter(r=>r.url.endsWith('/api/chat')).map(r=>JSON.parse(r.opts.body));
 assert.ok(judgedBodies.some(b=>b.glasses_hint_phase==='judge' && b.model===additionalJudge));
 assert.ok(!requests.slice(smallJudgeStart).some(r=>r.url.endsWith('/api/glasses/hint-decision')),'LLM selection never calls dictionary/JEV candidate extraction');
 assert.ok(judgedBodies.filter(b=>b.glasses_hint_phase==='judge').every(b=>!JSON.stringify(b.messages).includes('Names')),'judge receives transcript without candidate hints');

 assert.ok(judgedBodies.some(b=>b.glasses_hint_phase==='answer' && b.model===additionalHint));
 assert.ok(!judgedBodies.some(b=>b.glasses_hint_phase==='plan'),'do not spend another writer request planning');
 assert.match(element('chat-output').textContent,/쿠푸왕/);assert.match(element('hint-judge-status').textContent,/300토큰/);
 assert.match(queue.state().title,/^\[정보\] 피라미드/);
 assert.match(element('hint-evidence-log').textContent,/피라미드.*대화 이해에 필요한 정보/);
 assert.equal(queue.hintGate.entries.length,1);
 queue.hintGate.observe('피라미드가 왕의 무덤이래.',{id:'judge-end-to-end'});
 assert.equal(queue.hintGate.entries.length,1,'final transcript promotes the stable preview without duplication');
 const firstHintAnswer=queue.state().answer,comparisonStart=requests.length;
 const readyComparison=()=>{for(const item of queue.hintGate.recent)item.at=Date.now()-9000;queue.hintGate.lastDecision=-Infinity;queue.hintGate.nextHint=-Infinity;queue.hintGate.nextGeneration=-Infinity;};
 hintComparison.fact=true;readyComparison();
 judgeReply={hint:true,kind:'key',topic:'피라미드',evidence:'피라미드',question:'쿠푸왕 무덤의 석재 종류와 건축 재료',source:'internal'};
 chatReply=sseReply('data: '+JSON.stringify({choices:[{delta:{content:JSON.stringify({fact:'대피라미드의 왕의 방에는 주변 석회암과 다른 화강암이 사용됐습니다.',adds_information:true,kind:'key'})}}]})+'\n\ndata: [DONE]\n\n');
 queue.hintGate.observe('피라미드의 돌은 어떤 재료인지 궁금하대.',{id:'semantic-fact-duplicate'});for(let i=0;i<25;i++)await tick();
 assert.match(queue.state().answer,/왕의 방.*화강암/,'a distinct finished hint must display without a second semantic veto');
 assert.ok(!requests.slice(comparisonStart).some(r=>r.url.endsWith('/api/glasses/hint-compare') && JSON.parse(r.opts.body).kind==='fact'),'do not perform a post-generation semantic check');
 const transcriptCheck=requests.slice(comparisonStart).find(r=>r.url.endsWith('/api/glasses/hint-compare') && JSON.parse(r.opts.body).kind==='transcript');
 assert.ok(transcriptCheck,'compare against observed speech only, never old generated cards');
 assert.ok(JSON.parse(transcriptCheck.opts.body).history.some(text=>text.includes('어떤 재료')));
 assert.ok(!JSON.parse(transcriptCheck.opts.body).history.some(text=>text.includes('쿠푸왕을 위해')));
 const beforeEcho=queue.state().answer;
 hintComparison.transcript=true;readyComparison();
 judgeReply={span:1,topic:'피라미드',need:'이미 들은 건축재료를 표현만 바꿔 설명',kind:'key',source:'internal'};
 chatReply=sseReply('data: '+JSON.stringify({choices:[{delta:{content:JSON.stringify({fact:'피라미드 건축에는 석재를 쌓는 방식이 쓰였습니다.',adds_information:true,kind:'key'})}}]})+'\n\ndata: [DONE]\n\n');
 queue.hintGate.observe('피라미드는 돌을 쌓아 만드는 건축물이래.',{id:'observed-paraphrase'});for(let i=0;i<25;i++)await tick();
 assert.equal(queue.state().answer,beforeEcho,'a paraphrase of observed speech never replaces a useful card');
 assert.match(element('hint-status').textContent,/이미 설명된 내용/);
 hintComparison.transcript=false;
 const planComparisonStart=requests.length;hintComparison={plan:true,fact:false};readyComparison();
 judgeReply={hint:true,kind:'key',topic:'피라미드',evidence:'피라미드',question:'피라미드 무덤의 주인 이름과 신분',source:'internal'};
 queue.hintGate.observe('피라미드는 누구의 무덤인지 궁금하대.',{id:'semantic-plan-duplicate'});for(let i=0;i<25;i++)await tick();
 assert.ok(!requests.slice(planComparisonStart).some(r=>r.url.endsWith('/api/chat') && JSON.parse(r.opts.body).glasses_hint_phase==='answer'),'repeated research is rejected before spending a writer request');
 for(const request of requests.slice(comparisonStart).filter(r=>r.url.endsWith('/api/chat')))assert.ok(!request.opts.body.includes(firstHintAnswer),'old displayed facts must not enter judge/writer prompts');
 hintComparison={plan:false,fact:false};

 queue.hintGate.reset({forgetHistory:true});judgeReply={hint:false};const smallJudgeSkipStart=requests.length;
 queue.hintGate.observe('점심 맛있었어. 내일 만나자.',{id:'judge-skip'});for(let i=0;i<10;i++)await tick();
 assert.ok(!requests.slice(smallJudgeSkipStart).some(r=>r.url.endsWith('/api/chat') && JSON.parse(r.opts.body).glasses_hint_phase==='answer'));
 for(const [topic,need,text] of [
  ['그 조건을 만족하는 거','그 의미 설명','그 조건을 만족하는 거라면 만들 수 있다고 합니다.'],
  ['도구 특허','등록한 특허의 구체적인 내용','그 제작자는 그걸로 도구 특허를 등록했다고 합니다.']
 ]){
  queue.hintGate.reset({forgetHistory:true});const unresolvedStart=requests.length;
  judgeReply={span:1,topic,need,kind:'term',source:'internal'};
  queue.hintGate.observe(text,{id:'unresolved-'+topic});for(let i=0;i<15;i++)await tick();
  const sent=requests.slice(unresolvedStart).filter(r=>r.url.endsWith('/api/chat')).map(r=>JSON.parse(r.opts.body));
  assert.equal(sent.filter(body=>body.glasses_hint_phase==='judge').length,1,'a semantically unresolved selection skips without a format retry');
  assert.ok(!sent.some(body=>body.glasses_hint_phase==='answer'),'unresolved references never start lookup/writer requests');
 }
 queue.hintGate.reset({forgetHistory:true});judgeError=true;hintDecision={error:true};
 queue.hintGate.observe('청동기 시대에는 청동을 사용했대.',{id:'judge-error'});for(let i=0;i<10;i++)await tick();
 assert.equal(queue.state().recording,true,'judge errors must never stop transcription');assert.match(element('hint-judge-status').textContent,/지연\/오류/);
 judgeError=false;hintDecision={hint:false};
 // A grammatical sentence mislabelled as a topic cannot start a source lookup.
 queue.hintGate.reset({forgetHistory:true});queue.ai();
 const sentenceTopicStart=requests.length;
 judgeReply={span:1,topic:'나무는 색이 달라집니다.',need:'mechanism',kind:'term',source:'internal'};
 queue.hintGate.observe('나무는 색이 달라집니다.',{id:'sentence-topic-repair'});for(let i=0;i<20;i++)await tick();
 const sentenceRequests=requests.slice(sentenceTopicStart).filter(r=>r.url.endsWith('/api/chat')).map(r=>JSON.parse(r.opts.body));
 assert.equal(sentenceRequests.filter(body=>body.glasses_hint_phase==='judge').length,2);
 assert.ok(!sentenceRequests.some(body=>body.glasses_hint_phase==='answer'));
 assert.equal(queue.state().recording,true,'bad topic retries never stop STT');
 // A continuation without the topic name must not discard a finished hint.
 // Queue a second distinct hint before the first finishes; wait BEFORE its writer call.
 queue.ai();queue.hintGate.intervalMs=0;queue.hintGate.settleMs=0;queue.hintGate.generationIntervalMs=0;
 const originalHintNow=queue.hintGate.now;let displayClock=Date.now();queue.hintGate.now=()=>displayClock;
 const originalChunks=chunks.slice();
 const firstVisibleFact='기자의 대피라미드는 쿠푸왕을 위해 세워진 왕실 무덤입니다.';
 chunks.splice(0,chunks.length,'data: '+JSON.stringify({choices:[{delta:{content:JSON.stringify({fact:firstVisibleFact,adds_information:true,kind:'key'})}}]})+'\n\ndata: [DONE]\n\n');
 holdChat=true;chatReply=null;releaseChat=null;
 judgeReply={hint:true,continuity:'unknown',kind:'key',topic:'피라미드',evidence:'피라미드',question:'피라미드 건립 목적과 대상 왕',source:'internal'};
 const visibilityStart=requests.length;
 queue.hintGate.observe('피라미드의 건립 목적이 궁금하대',{id:'visible-inflight'});for(let i=0;i<20;i++)await tick();
 assert.equal(typeof releaseChat,'function');
 judgeReply={hint:true,continuity:'same',kind:'key',topic:'큰 돌',evidence:'큰 돌',question:'피라미드의 석재 운반 기술과 경사로 원리',source:'internal'};
 queue.hintGate.lastDecision=-Infinity;queue.hintGate.nextGeneration=-Infinity;
 queue.hintGate.observe('그렇게 큰 돌을 어떻게 높이 옮겼을까',{id:'same-topic-followup'});for(let i=0;i<20;i++)await tick();
 const secondVisibleFact='경사로는 무거운 석재를 높은 곳까지 옮기는 데 활용된 것으로 추정됩니다.';
 chatReply=sseReply('data: '+JSON.stringify({choices:[{delta:{content:JSON.stringify({fact:secondVisibleFact,adds_information:true,kind:'key'})}}]})+'\n\ndata: [DONE]\n\n');
 releaseChat();for(let i=0;i<25;i++)await tick();
 assert.equal(queue.state().answer,firstVisibleFact,'a same-topic continuation without a literal topic name must not hide the first generated hint');
 assert.equal(requests.slice(visibilityStart).filter(r=>r.url.endsWith('/api/chat') && JSON.parse(r.opts.body).glasses_hint_phase==='answer').length,1,'second hint must wait before generation rather than generate and get discarded');
 chunks.splice(0,chunks.length,...originalChunks);
 displayClock+=8001;await queue.drainAI();for(let i=0;i<15;i++)await tick();
 assert.equal(queue.state().answer,secondVisibleFact,'distinct second hint must display after the spacing interval');
 assert.equal(requests.slice(visibilityStart).filter(r=>r.url.endsWith('/api/chat') && JSON.parse(r.opts.body).glasses_hint_phase==='answer').length,2);
 // A started writer survives a confirmed topic switch and source-age expiry.
 queue.hintGate.reset({forgetHistory:true});releaseChat=null;
 const completedOldTopic='피라미드의 내부에는 무게를 분산하는 공간이 배치되어 있습니다.';
 chunks.splice(0,chunks.length,'data: '+JSON.stringify({type:'request.complete',final_assistant_content:JSON.stringify({fact:completedOldTopic,adds_information:true,kind:'key'})})+'\n\n');
 holdChat=true;chatReply=null;
 judgeReply={hint:true,continuity:'unknown',kind:'key',topic:'피라미드',evidence:'피라미드',question:'피라미드 내부 공간의 구조',source:'internal'};
 queue.hintGate.observe('피라미드 내부 구조 이야기',{id:'committed-before-switch'});for(let i=0;i<20;i++)await tick();
 assert.equal(typeof releaseChat,'function');
 displayClock+=46000;queue.hintGate.lastDecision=-Infinity;queue.hintGate.nextGeneration=-Infinity;
 judgeReply={hint:false,continuity:'changed',topic:'청동기 시대',evidence:'청동기 시대'};
 queue.hintGate.observe('이제 청동기 시대 이야기로 바뀌었어',{id:'changed-during-writer'});for(let i=0;i<20;i++)await tick();
 assert.equal(queue.hintGate.currentTopic,'청동기 시대');
 releaseChat();for(let i=0;i<25;i++)await tick();
 assert.equal(queue.state().answer,completedOldTopic,'the writer already started, so show its verified completion');
 assert.equal(queue.hintGate.currentTopic,'청동기 시대','finishing an older card must not overwrite the classifier topic');
 // Two-card stack: a new selected topic must not abort the first definition
 // when its draft clause is corrected but the actual term remains unchanged.
 queue.hintGate.reset({forgetHistory:true});queue.stack.clear();
 element('hint-card-count').value='2';await element('hint-card-count').handlers.change();queue.stack.clear();
 const stackFirst='해양의 염분은 지역과 수심에 따라 달라지며 해류의 움직임에도 영향을 줍니다.';
 const stackSecond='대기는 온도 변화에 따라 층으로 구분하며 각 층에서 나타나는 현상이 다릅니다.';
 chunks.splice(0,chunks.length,'data: '+JSON.stringify({type:'request.complete',final_assistant_content:JSON.stringify({fact:stackFirst,adds_information:true,kind:'term'})})+'\n\n');
 holdChat=true;chatReply=null;releaseChat=null;
 judgeReply={span:1,topic:'해양',need:'해양의 기본 정의와 염분의 역할',kind:'term',source:'internal'};
 queue.hintGate.observe('해양이라고 하는 동네 말이죠?',{id:'stack-definition-draft',preview:true});for(let i=0;i<20;i++)await tick();
 assert.equal(typeof releaseChat,'function');
 queue.hintGate.validatePreview('stack-definition-draft','해양이라고 하는 종류의 환경입니다.');
 queue.hintGate.lastDecision=-Infinity;queue.hintGate.nextGeneration=-Infinity;
 judgeReply={span:1,topic:'대기',need:'대기의 층을 구분하는 기준',kind:'term',source:'internal'};
 queue.hintGate.observe('다음에는 대기 이야기를 해봅시다.',{id:'stack-second-topic'});for(let i=0;i<20;i++)await tick();
 releaseChat();for(let i=0;i<25;i++)await tick();
 assert.ok(queue.stack.cards.some(card=>card.text===stackFirst),'first definition must finish despite a corrected clause and next candidate');
 chatReply=sseReply('data: '+JSON.stringify({type:'request.complete',final_assistant_content:JSON.stringify({fact:stackSecond,adds_information:true,kind:'term'})})+'\n\n');
 displayClock+=8001;await queue.drainAI();for(let i=0;i<25;i++)await tick();
 assert.equal(queue.stack.cards.length,2);assert.deepEqual(Array.from(queue.stack.cards,card=>card.text),[stackSecond,stackFirst]);
 element('hint-card-count').value='1';await element('hint-card-count').handlers.change();
 chunks.splice(0,chunks.length,...originalChunks);
 // Long first sentences must scroll instead of being rejected by viewport fitting.
 queue.hintGate.reset({forgetHistory:true});
 const longHint='피라미드 건축의 구조와 자재 운송 및 하중 분산 원리를 자세히 설명하는 긴 문장으로 '+ '여러 석재의 배치와 경사로를 통한 운반 방법을 함께 살펴보며 '.repeat(12)+'완성됩니다.';
 assert.ok(context.sliceHUDWindow(longHint,0,6,570).maxOffset>0);
 judgeReply={hint:true,kind:'key',topic:'피라미드',evidence:'피라미드',question:'피라미드의 여러 건축 원리',source:'internal'};
 let finalReads=0,finalCancels=0;
 chatReply={ok:true,body:{getReader:()=>({cancel:async()=>{finalCancels++;},read:async()=>{
   if(finalReads++)throw new TypeError('Load failed after completed response');
   return {done:false,value:Buffer.from('data: '+JSON.stringify({type:'request.complete',final_assistant_content:JSON.stringify({fact:longHint,adds_information:true,kind:'key'})})+'\n\n')};
 }})}};
 queue.hintGate.observe('피라미드 구조를 살펴보는 중',{id:'long-completed-card'});for(let i=0;i<25;i++)await tick();
 assert.equal(queue.state().answer,longHint,'a first sentence exceeding six lines must remain complete and scrollable');
 assert.equal(finalReads,1,'request.complete must display without waiting for socket closure');assert.equal(finalCancels,1);
 assert.doesNotMatch(element('hint-status').textContent,/Load failed|출처.*불일치/);
 queue.dismiss();readyComparison();
 judgeReply={hint:true,kind:'key',topic:'피라미드',evidence:'피라미드',question:'석재 운반 장비와 작업 인력의 조직 방식',source:'internal'};
 chatReply=sseReply('data: '+JSON.stringify({type:'request.complete',final_assistant_content:JSON.stringify({fact:longHint,adds_information:true,kind:'key'})})+'\n\n');
 queue.hintGate.observe('피라미드 작업 인력 이야기',{id:'finished-after-card-closed'});for(let i=0;i<25;i++)await tick();
 assert.equal(queue.state().answer,longHint,'an already generated answer must not disappear because its identical older card is closed');
 chunks.splice(0,chunks.length,...originalChunks);
 queue.hintGate.now=originalHintNow;await queue.stopVoiceRecording(false);
 // The model may ignore its schema. Conversation-only tasks must never spend a writer request or replace a useful card.
 queue.ai();queue.hintGate.collectContext=true;queue.hintGate.windowMs=20000;queue.hintGate.settleMs=1000;queue.hintGate.collectWaitMs=4000;queue.hintGate.intervalMs=0;queue.hintGate.generationIntervalMs=0;
 for(const [kind,source] of [['schedule','conversation'],['recap','conversation'],['recap','internal'],['key','conversation']]){
   queue.hintGate.reset({forgetHistory:true});
   judgeReply={hint:true,kind,source,topic:'회의 준비',evidence:'금요일 오후 3시',question:'대화 정리',continuity:'same'};
   const before=requests.length,previousAnswer=queue.state().answer;
   queue.hintGate.observe('금요일 오후 3시에 회의하고 노트북을 가져갑니다.',{id:'disabled-'+kind+'-'+source});
   queue.hintGate.pending.due=0;clearTimeout(queue.hintGate.timer);await queue.hintGate.pump();for(let i=0;i<15;i++)await tick();
   assert.ok(!requests.slice(before).some(r=>r.url.endsWith('/api/chat') && JSON.parse(r.opts.body).glasses_hint_phase==='answer'),'conversation recaps/schedules must not reach the writer');
   assert.equal(queue.state().answer,previousAnswer,'rejected summaries retain the useful previous display');
 }
 queue.hintGate.reset({forgetHistory:true});queue.hintGate.collectContext=false;queue.hintGate.settleMs=0;
 const definition='EBITDA는 이자와 세금, 감가상각비를 차감하기 전의 이익을 뜻합니다.';
 judgeReply={hint:true,kind:'term',source:'web',topic:'EBITDA',evidence:'EBITDA',question:'EBITDA의 뜻',continuity:'unknown'};
 nextHintProof={version:1,mode:'quote',source:'web',kind:'term',topic:'EBITDA',records:[{title:'EBITDA',url:'https://example.com/ebitda',text:definition}]};
 chatReply=sseReply('data: '+JSON.stringify({type:'request.complete',final_assistant_content:JSON.stringify({adds_information:true,kind:'key',topic:'EBITDA',fact:definition,support_quote:definition})})+'\n\n');
 queue.hintGate.observe('EBITDA가 무슨 뜻인지 모르겠어요.',{id:'term-not-key'});for(let i=0;i<25;i++)await tick();
 assert.match(queue.state().title,/^\[용어\]/);assert.equal(queue.state().answer,definition,'writer default key must not erase a selected term definition');
 // Preserve original quote verification, then localize only the verified display.
 await queue.settings({translateTargetLang:'en',translationEngine:'chrome'});
 queue.hintGate.reset({forgetHistory:true});queue.hintGate.collectContext=false;queue.hintGate.settleMs=0;
 chromeText='EBITDA means earnings before interest, taxes, depreciation and amortization.';
 judgeReply={hint:true,kind:'term',source:'web',topic:'EBITDA',evidence:'EBITDA',question:'EBITDA 뜻',continuity:'unknown'};
 nextHintProof={version:1,mode:'quote',source:'web',kind:'term',topic:'EBITDA',records:[{title:'EBITDA',url:'https://example.com/ebitda',text:definition}]};
 chatReply=sseReply('data: '+JSON.stringify({type:'request.complete',final_assistant_content:JSON.stringify({adds_information:true,kind:'term',topic:'EBITDA',fact:definition,support_quote:definition})})+'\n\n');
 const localizationRequests=requests.length;
 queue.hintGate.observe('EBITDA의 의미를 설명해 줬어요.',{id:'localized-proof'});for(let i=0;i<30;i++)await tick();
 assert.equal(queue.state().answer,chromeText,'verified Korean quote is displayed in the chosen English target');
 const localizedRequest=requests.slice(localizationRequests).find(r=>r.url.endsWith('/api/translate/chrome'));
 assert.ok(localizedRequest);assert.equal(JSON.parse(localizedRequest.opts.body).text,definition,'only the verified source text reaches translation');assert.equal(JSON.parse(localizedRequest.opts.body).target,'en');
 const localizedJudge=requests.slice(localizationRequests).filter(r=>r.url.endsWith('/api/chat')).map(r=>JSON.parse(r.opts.body)).find(b=>b.glasses_hint_phase==='judge');
 assert.match(localizedJudge.messages[0].content,/Cards are written in English/);
 chromeText=null;await queue.settings({translateTargetLang:'ko'});
 // Contextual aid: ordinary movie-going leads to current rankings, not a
 // definition of the word movie or a literal-name search in the source title.
 queue.ai();queue.hintGate.collectContext=false;queue.hintGate.settleMs=0;
 const rankDate='가상 검증일 기준 국내 극장 관객 수 집계 자료입니다.';
 const rankResult='가상 작품 A가 일별 관객 수 순위에서 첫 번째입니다.';
 const rankingProof={version:1,mode:'quote',source:'web',task_oriented:true,topic:'영화',kind:'key',task:'현재 한국 극장 관객 수 순위와 집계 기준일',records:[{title:'가상 국내 관객 수 집계',url:'https://example.com/fictional-chart',text:rankDate+' '+rankResult,spans:[rankDate,rankResult]}]};
 nextHintProof=rankingProof;
 judgeReply={span:1,topic:'영화',need:rankingProof.task,kind:'key',source:'web'};
 chatReply=sseReply('data: '+JSON.stringify({type:'request.complete',final_assistant_content:'{"selections":[1,2]}'})+'\n\n');
 const contextAidStart=requests.length;
 queue.hintGate.observe('주말에는 극장에서 영화 한 편 볼까 해요.',{id:'context-movie-card'});for(let i=0;i<35;i++)await tick();
 const contextPayload=requests.slice(contextAidStart).filter(r=>r.url.endsWith('/api/chat')).map(r=>JSON.parse(r.opts.body)).find(body=>body.glasses_hint_phase==='answer');
 assert.ok(contextPayload.hint_research.task_oriented);assert.equal(contextPayload.hint_research.query,rankingProof.task);
 assert.match(queue.state().title,/가상 국내 관객 수 집계/);
 assert.equal(queue.state().answer,rankDate+' '+rankResult);
 assert.equal(queue.hintGate.currentTopic,'영화','verified source headline must not become the next conversation topic');
 await queue.audit.flush();for(let i=0;i<20;i++)await tick();
 const provenanceEvent=requests.filter(r=>r.url.endsWith('/api/glasses/hint-events') && r.opts.body).flatMap(r=>JSON.parse(r.opts.body).events).find(e=>e.reason==='hint_provenance' && e.segmentId==='context-movie-card');
 assert.ok(provenanceEvent,'a displayed hint must persist its trigger and selected source');
 const provenance=JSON.parse(provenanceEvent.text);
 assert.equal(provenance.trigger.segmentId,'context-movie-card');
 assert.equal(provenance.sources[0].url,rankingProof.records[0].url);
 assert.deepEqual(provenance.sources[0].sentences,[1,2]);
 assert.match(provenance.quote,/가상 작품 A/);
 assert.ok(provenanceEvent.jobId);

 await queue.stopVoiceRecording(false);
 element('model-hint-decision').value='__jev__';await element('model-hint-decision').handlers.change();
 await queue.stopVoiceRecording(false);
 // Keep persisted settings expected by the restore tests below.
 await queue.settings({audioInputSource:'phone',sttEngine:'apple',translationEngine:'apple'});
 const originalImageUpdate=bridge.updateImageRawData;
 bridge.updateImageRawData=async()=> 'sendFailed';
 queue.refreshIcons();
 for(let i=0;i<100 && !updates.some(u=>u.containerID===3 && /^\[(?:AI|번역) 폰 꺼짐\]/u.test(u.content || ''));i++)await new Promise(resolve=>setTimeout(resolve,20));
 assert.ok(updates.some(u=>u.containerID===3 && /^\[(?:AI|번역) 폰 꺼짐\]/u.test(u.content || '')),'image failure must retain text updates with explicit source/mic state');
 bridge.updateImageRawData=originalImageUpdate;

 // AI uses its content height; transcription stays at most three rows with blank space below.
 const adaptiveSnapshots=[];
 for(const [title,text,height] of [['','',0],['피라미드','기자의 대피라미드는 쿠푸왕을 위해 건립됐습니다.',66],['청동기 시대','첫 번째 줄\n두 번째 줄\n세 번째 줄',120],['긴 힌트','1줄\n2줄\n3줄\n4줄\n5줄\n6줄',174]]){
   queue.showHint(title,text,'전사 기록 '.repeat(200));
   for(let i=0;i<150 && (((nativePage.textObject.find(c=>c.containerID===1)?.height || 0)+(nativePage.textObject.find(c=>c.containerID===9)?.height || 0))!==height || (title && !nativePage.textObject.find(c=>c.containerID===9)?.content.startsWith(title)));i++)await new Promise(resolve=>setTimeout(resolve,20));
   const hintBox=nativePage.textObject.find(c=>c.containerID===1),tx=nativePage.textObject.find(c=>c.containerID===4);
   assert.equal((hintBox?.height || 0)+(nativePage.textObject.find(c=>c.containerID===9)?.height || 0),height);assert.equal(tx.yPosition,27+height);assert.equal(tx.height,81);assert.ok(tx.yPosition+tx.height<=288);
   if(title){
     assert.ok(nativePage.textObject.find(c=>c.containerID===9).content.startsWith(title),'title is separate from scrolling body');
     assert.equal(nativePage.textObject.find(c=>c.containerID===9).borderWidth,1,'title frame provides a separator line');
     assert.ok(element('hud-screen').innerHTML.includes('>'+title+'</span>'),'phone preview uses the actual hint title');
   }else assert.ok(!element('hud-screen').innerHTML.includes('hud-ai-box'),'waiting must hide the empty AI box');
   assert.ok(tx.content.split('\n').length<=3,'transcript must never show more than three native rows');
   adaptiveSnapshots.push({title,html:element('hud-screen').innerHTML});
 }
 await new Promise(resolve=>setTimeout(resolve,1200));
 const sameHeightRebuilds=rebuilds.length;
 queue.showHint('다른 제목','a\nb\nc\nd\ne\nf','전사 기록 '.repeat(200));await new Promise(resolve=>setTimeout(resolve,1200));
 assert.equal(rebuilds.length,sameHeightRebuilds,'same height only upgrades content, avoiding needless native page rebuilds');
 fs.writeFileSync('/private/tmp/dkst-adaptive-snapshots.json',JSON.stringify(adaptiveSnapshots));
 // Display-only hiding never stops recognition or transcript storage.
 queue.record();const hidingMicCalls=audioCalls.length;
 element('hide-transcript').checked=true;await element('hide-transcript').handlers.change();
 queue.showHint('','','전사 기록 '.repeat(200));
 for(let i=0;i<150 && nativePage.textObject.some(c=>c.containerID===4 || c.containerID===1);i++)await new Promise(resolve=>setTimeout(resolve,20));
 assert.ok(!nativePage.textObject.some(c=>c.containerID===4));assert.ok(!nativePage.textObject.some(c=>c.containerID===1));
 assert.equal(nativePage.textObject.find(c=>c.containerID===3).isEventCapture,1,'header receives taps when both content boxes are hidden');
 assert.ok(!element('hud-screen').innerHTML.includes('hud-transcript-box'));
 assert.equal(queue.state().recording,true);assert.equal(audioCalls.length,hidingMicCalls);
 nextTranscript='숨긴 상태에서도 저장되는 전사';queue.enqueueClip(Buffer.alloc(6400),true,Date.now(),'hidden-transcript');for(let i=0;i<8;i++)await tick();
 assert.match(element('transcript-log').textContent,/숨긴 상태에서도 저장되는 전사/);assert.equal(queue.state().recording,true);
 queue.showHint('넓은 힌트','1줄\n2줄\n3줄\n4줄\n5줄\n6줄\n7줄\n8줄','숨김');
 for(let i=0;i<150 && nativePage.textObject.find(c=>c.containerID===1)?.height!==222;i++)await new Promise(resolve=>setTimeout(resolve,20));
 assert.equal(nativePage.textObject.find(c=>c.containerID===1).height,222,'hidden transcript allows up to eight hint body rows');
 const callsBeforeMenu=audioCalls.length;
 handler({menuItemClickEvent:{itemID:10}});handler({listEvent:{containerID:30,currentSelectItemIndex:0}});handler({listEvent:{containerID:30,eventType:3,currentSelectItemIndex:0}});
 for(let i=0;i<150 && !nativePage.textObject.some(c=>c.containerID===4);i++)await new Promise(resolve=>setTimeout(resolve,20));
 assert.equal(element('hide-transcript').checked,false);assert.equal(nativePage.textObject.find(c=>c.containerID===4).height,81);
 assert.equal(queue.state().recording,true);assert.equal(audioCalls.length,callsBeforeMenu,'native hide/show setting must not restart the microphone');
 const captionRetention=element('caption-retention');
 assert.equal(captionRetention.value,'0');
 captionRetention.value='3';await captionRetention.handlers.change();
 nextTranscript='시간이 지나면 사라질 전사';queue.enqueueClip(Buffer.alloc(6400),true,Date.now(),'expiring-transcript');
 for(let i=0;i<8;i++)await tick();
 assert.match(element('hud-screen').innerHTML,/시간이 지나면 사라질 전사/);
 await new Promise(resolve=>setTimeout(resolve,3100));
 assert.doesNotMatch(element('hud-screen').innerHTML,/시간이 지나면 사라질 전사/);
 assert.match(element('transcript-log').textContent,/시간이 지나면 사라질 전사/);
 assert.equal(JSON.parse(nativeSaved).captionRetention,3);
 assert.equal(audioCalls.length,callsBeforeMenu);
 captionRetention.value='0';await captionRetention.handlers.change();
 element('hide-transcript').checked=true;await element('hide-transcript').handlers.change();await tick();
 assert.equal(JSON.parse(nativeSaved).hideTranscript,true);await queue.stopVoiceRecording(false);
 // Auto-close counts actual body rows, updates only the final line, and never stops recording.
 queue.record();const timerMicCalls=audioCalls.length;
 element('hide-transcript').checked=false;await element('hide-transcript').handlers.change();
 element('hint-seconds-per-line').value='2.5';await element('hint-seconds-per-line').handlers.change();
 element('hint-auto-close').checked=true;await element('hint-auto-close').handlers.change();
 queue.showHint('제목','첫 줄\n둘째 줄\n셋째 줄','닫혀도 남는 전사');
 assert.equal(queue.lifetime.remaining,8);assert.equal(queue.state().answer,'첫 줄\n둘째 줄\n셋째 줄');
 for(let i=0;i<150 && !nativePage.textObject.find(c=>c.containerID===9)?.content.endsWith('8초');i++)await new Promise(resolve=>setTimeout(resolve,20));
 assert.ok(nativePage.textObject.find(c=>c.containerID===9).content.endsWith('8초'));
 assert.ok(element('hud-screen').innerHTML.includes('>8초</span>'));const timerSnapshots=[element('hud-screen').innerHTML];
 const timerRebuilds=rebuilds.length,oldHintTick=hintJobs.at(-1);
 hintClock=2500;oldHintTick();
 for(let i=0;i<150 && !nativePage.textObject.find(c=>c.containerID===9)?.content.endsWith('5초');i++)await new Promise(resolve=>setTimeout(resolve,20));
 assert.ok(nativePage.textObject.find(c=>c.containerID===9).content.endsWith('5초'));assert.equal(rebuilds.length,timerRebuilds,'second ticks must not rebuild the page');
 timerSnapshots.push(element('hud-screen').innerHTML);
 queue.showHint('새 제목','새 힌트','닫혀도 남는 전사');const newTick=hintJobs.at(-1);
 hintClock=3500;oldHintTick();assert.equal(queue.state().answer,'새 힌트','old timer cannot close its replacement');
 hintClock=6000;newTick();
 for(let i=0;i<150 && nativePage.textObject.some(c=>c.containerID===1);i++)await new Promise(resolve=>setTimeout(resolve,20));
 assert.equal(queue.state().answer,'');assert.equal(queue.state().title,'');assert.equal(queue.state().recording,true);
 assert.match(element('hud-screen').innerHTML,/닫혀도 남는 전사/);assert.equal(audioCalls.length,timerMicCalls);
 timerSnapshots.push(element('hud-screen').innerHTML);fs.writeFileSync('/private/tmp/dkst-autoclose-snapshots.json',JSON.stringify(timerSnapshots));
 queue.showHint('','직접 AI 채팅 답변','전사');assert.equal(queue.lifetime.active,true);
 const directTick=hintJobs.at(-1);hintClock+=100000;directTick();assert.equal(queue.state().answer,'','completed direct AI chat now uses the same auto-close option');
 chatReply=null;holdChat=true;element('chat-input').value='자동 닫기 적용할 AI 질문';element('text-chat-form').handlers.submit({preventDefault(){}});await tick();await tick();await tick();
 assert.equal(queue.lifetime.active,false,'timer must not run while AI is generating its answer');
 releaseChat();for(let i=0;i<10;i++)await tick();
 assert.equal(queue.lifetime.active,true,'completed typed AI answer starts the common timer');
 assert.equal(element('hud-trigger-keyword').style.opacity,'0.25','typed AI answer never activates voice wake brightness');
 const typedTick=hintJobs.at(-1);hintClock+=100000;typedTick();assert.equal(queue.state().answer,'');
 element('hint-auto-close').checked=false;await element('hint-auto-close').handlers.change();
 queue.showHint('제목','유지할 힌트','전사');assert.equal(queue.lifetime.active,false);assert.ok(!element('hud-screen').innerHTML.includes('초</span>'));
 const audioBeforeAIExemption=audioCalls.length;
 element('hud-full-ai').checked=false;element('hud-full-ai').handlers.change();
 for(let i=0;i<150 && nativePage.textObject.find(c=>c.containerID===1)?.textColor!==1;i++)await new Promise(resolve=>setTimeout(resolve,20));
 assert.equal(nativePage.textObject.find(c=>c.containerID===1).textColor,1);
 assert.equal(nativePage.textObject.find(c=>c.containerID===9).textColor,1);
 assert.equal(element('hud-ai-body').style.color,'rgb(0, 64, 26)');
 assert.equal(queue.state().answer,'유지할 힌트','dimming never replaces displayed content');
 element('hud-full-ai').checked=true;element('hud-full-ai').handlers.change();
 for(let i=0;i<150 && (nativePage.textObject.find(c=>c.containerID===1)?.textColor!==4 || nativePage.textObject.find(c=>c.containerID===9)?.textColor!==4);i++)await new Promise(resolve=>setTimeout(resolve,20));
 assert.equal(nativePage.textObject.find(c=>c.containerID===1).textColor,4);
 assert.equal(nativePage.textObject.find(c=>c.containerID===9).textColor,4);
 assert.equal(audioCalls.length,audioBeforeAIExemption);
 element('hud-dim-level').value='0';element('hud-dim-level').handlers.input();
 for(let i=0;i<150 && nativePage.textObject.find(c=>c.containerID===2)?.textColor!==0;i++)await new Promise(resolve=>setTimeout(resolve,20));
 for(const id of [1,9]){assert.equal(nativePage.textObject.find(c=>c.containerID===id).borderWidth,1);assert.equal(nativePage.textObject.find(c=>c.containerID===id).borderColor,4,'lowest status brightness must retain the content frame');}
 element('hud-dim-level').value='1';element('hud-dim-level').handlers.input();
 for(let i=0;i<150 && nativePage.textObject.find(c=>c.containerID===2)?.textColor!==1;i++)await new Promise(resolve=>setTimeout(resolve,20));


 element('hint-auto-close').checked=true;await element('hint-auto-close').handlers.change();
 assert.equal(JSON.parse(nativeSaved).hintAutoClose,true);assert.equal(JSON.parse(nativeSaved).hintSecondsPerLine,2.5);
 await queue.stopVoiceRecording(false);
 // Calibrate in arbitrary SDK coordinates; IMU reports must never be mistaken for a tap.
 const beforeDisabledIMU=audioCalls.length;handler({sysEvent:{eventType:8,imuData:{x:5,y:0,z:0}}});await tick();
 assert.equal(audioCalls.length,beforeDisabledIMU);assert.equal(imuCalls.length,0,'disabled head gesture must not stream IMU');
 element('head-gesture-enabled').checked=true;await element('head-gesture-enabled').handlers.change();
 assert.deepEqual(imuCalls.at(-1),[true,100]);assert.equal(element('head-calibrate').disabled,false);
 let headAt=0;
 const imuSamples=(y,count=4)=>{for(let i=0;i<count;i++){headAt+=100;queue.headIMU({x:5,y,z:0},headAt);}};
 await element('head-calibrate').handlers.click();imuSamples(0,8);
 assert.equal(queue.headState().phase,'readyRaised');assert.match(element('head-calibrate').textContent,/올린 자세/);
 await element('head-calibrate').handlers.click();imuSamples(30,8);
 assert.equal(queue.headState().phase,'idle');assert.deepEqual(JSON.parse(nativeSaved).headGestureCalibration,{neutral:[5,0,0],raised:[5,30,0]});
 imuSamples(0);assert.equal(queue.headState().paused,false,'lowering after calibration must not toggle');
 const headWave=async()=>{headAt+=2000;imuSamples(0);imuSamples(30);imuSamples(0);await queue.headWait();await tick();};
 const idleHeadOpens=audioCalls.filter(call=>call[0]).length;
 await headWave();
 for(let i=0;i<150 && nativePage.textObject[0]?.containerName!=='head-blank';i++)await new Promise(resolve=>setTimeout(resolve,20));
 assert.equal(queue.headState().paused,true);assert.equal(nativePage.containerTotalNum,1);assert.equal(nativePage.textObject[0].content,'');assert.equal(nativePage.imageObject,undefined);
 assert.equal(queue.headState().imu,true,'IMU stays live while the display is blank');assert.equal(queue.state().recording,false);
 const blankUpdates=updates.length;queue.refreshIcons();deviceHandler({sn:'glasses-1',connectType:'connected',batteryLevel:78});await new Promise(resolve=>setTimeout(resolve,250));
 assert.equal(updates.length,blankUpdates,'status/image updates must not repaint the sleeping screen');
 await headWave();
 for(let i=0;i<150 && nativePage.textObject[0]?.containerName==='head-blank';i++)await new Promise(resolve=>setTimeout(resolve,20));
 assert.equal(queue.headState().paused,false);assert.equal(queue.state().recording,false,'waking an idle app must not start a microphone');assert.equal(audioCalls.filter(call=>call[0]).length,idleHeadOpens);
 // Wake during an outstanding blank-page acknowledgement must still restore the normal page.
 const headRebuild=bridge.rebuildPageContainer;let releaseBlankAck;
 bridge.rebuildPageContainer=async page=>{const result=await headRebuild(page);if(page.textObject[0]?.containerName==='head-blank')await new Promise(resolve=>{releaseBlankAck=resolve;});return result;};
 await headWave();
 for(let i=0;i<150 && !releaseBlankAck;i++)await new Promise(resolve=>setTimeout(resolve,20));
 assert.equal(typeof releaseBlankAck,'function');await element('btn-head-resume').handlers.click();releaseBlankAck();
 for(let i=0;i<150 && nativePage.textObject[0]?.containerName==='head-blank';i++)await new Promise(resolve=>setTimeout(resolve,20));
 assert.notEqual(nativePage.textObject[0]?.containerName,'head-blank');assert.equal(queue.headState().paused,false);bridge.rebuildPageContainer=headRebuild;
 // A late IMU-enable acknowledgement must not undo a subsequent option-off request.
 element('head-gesture-enabled').checked=false;await element('head-gesture-enabled').handlers.change();
 const headIMUControl=bridge.imuControl;let releaseIMUEnable;
 bridge.imuControl=async(open,pace)=>{imuCalls.push([open,pace]);if(open)await new Promise(resolve=>{releaseIMUEnable=resolve;});return true;};
 element('head-gesture-enabled').checked=true;const enablingHead=element('head-gesture-enabled').handlers.change();await tick();
 element('head-gesture-enabled').checked=false;const disablingHead=element('head-gesture-enabled').handlers.change();releaseIMUEnable();await enablingHead;await disablingHead;
 assert.equal(queue.headState().imu,false);assert.deepEqual(imuCalls.at(-1),[false,100]);bridge.imuControl=headIMUControl;
 element('head-gesture-enabled').checked=true;await element('head-gesture-enabled').handlers.change();
 // Pausing active capture closes the source, archives the final segment, and cancels optional AI work.
 context.window.__queueTest.microphone();await tick();await tick();await tick();assert.equal(queue.state().recording,true);
 nextTranscript='고개 제스처 직전의 저장 음성';for(let i=0;i<8;i++)handler({audioEvent:{audioPcm:speech,source:'phone'}});await tick();
 holdChat=true;chatReply=null;element('chat-input').value='일시정지로 취소할 질문';element('text-chat-form').handlers.submit({preventDefault(){}});await tick();await tick();
 const activePauseCalls=audioCalls.length;await headWave();for(let i=0;i<8;i++)await tick();
 assert.equal(queue.headState().paused,true);assert.equal(queue.state().recording,false);assert.equal(audioCalls[activePauseCalls][0],false);
 assert.match(element('transcript-log').textContent,/고개 제스처 직전의 저장 음성/,'the captured tail must be archived even while blank');
 const pausedStt=requests.filter(r=>/\/api\/stt\/(?:preview|transcribe)$/.test(r.url)).length;
 for(let i=0;i<12;i++)handler({audioEvent:{audioPcm:speech,source:'phone'}});await tick();
 assert.equal(requests.filter(r=>/\/api\/stt\/(?:preview|transcribe)$/.test(r.url)).length,pausedStt,'no new STT audio while paused');
 await headWave();assert.equal(queue.headState().paused,false);assert.equal(queue.state().recording,true);assert.deepEqual(audioCalls.filter(call=>call[0]).at(-1),[true,'phone']);
 // A failed native close never permits a second microphone to open on wake.
 const headAudioControl=bridge.audioControl;
 bridge.audioControl=async(open,...args)=>{audioCalls.push([open,...args]);return open;};
 await headWave();assert.equal(queue.headState().paused,true);assert.equal(queue.headState().uncertain,true);
 const blockedHeadOpens=audioCalls.filter(call=>call[0]).length;await element('btn-head-resume').handlers.click();
 assert.equal(queue.headState().paused,true);assert.equal(audioCalls.filter(call=>call[0]).length,blockedHeadOpens);
 bridge.audioControl=headAudioControl;await element('btn-head-resume').handlers.click();
 assert.equal(queue.headState().paused,false);assert.equal(queue.state().recording,true);
 // Disabling while paused restores the screen/capture before removing the only motion wake path.
 await headWave();assert.equal(queue.headState().paused,true);
 element('head-gesture-enabled').checked=false;await element('head-gesture-enabled').handlers.change();
 assert.equal(queue.headState().paused,false);assert.equal(queue.state().recording,true);assert.equal(queue.headState().imu,false);assert.deepEqual(imuCalls.at(-1),[false,100]);
 await queue.stopVoiceRecording(false);
 element('head-gesture-enabled').checked=true;await element('head-gesture-enabled').handlers.change();
 assert.equal(JSON.parse(nativeSaved).headGestureEnabled,true);

 // Native title is separate from the scrolling body.
 element('head-gesture-enabled').checked=false;await element('head-gesture-enabled').handlers.change();
 queue.ai();hintDecision={hint:false};judgeError=false;
 queue.showHint('고정된 제목','첫 줄\n둘째 줄\n셋째 줄\n넷째 줄\n다섯째 줄\n여섯째 줄\n마지막 줄','남는 전사');
 for(let i=0;i<150 && !nativePage.textObject.find(c=>c.containerID===9)?.content.startsWith('고정된 제목');i++)await new Promise(resolve=>setTimeout(resolve,20));
 const fixedHeader={...nativePage.textObject.find(c=>c.containerID===9)},beforeBody=nativePage.textObject.find(c=>c.containerID===1).content;
 assert.equal(fixedHeader.yPosition,27);assert.equal(fixedHeader.height,33);assert.equal(fixedHeader.borderWidth,1);assert.ok(fixedHeader.content.endsWith('초'));
 for(let i=0;i<3;i++)handler({sysEvent:{eventType:2}});
 for(let i=0;i<150 && nativePage.textObject.find(c=>c.containerID===1)?.content===beforeBody;i++)await new Promise(resolve=>setTimeout(resolve,20));
 assert.equal(nativePage.textObject.find(c=>c.containerID===9).content,fixedHeader.content);
 assert.equal(nativePage.textObject.find(c=>c.containerID===9).yPosition,27);
 assert.ok(!nativePage.textObject.find(c=>c.containerID===1).content.includes('고정된 제목'));
 fs.writeFileSync('/private/tmp/dkst-pinned-panel-preview.json',JSON.stringify({html:element('hud-screen').innerHTML,native:nativePage}));
 const closeTapMic=audioCalls.length;handler({textEvent:{}});await tick();await tick();
 assert.equal(queue.state().answer,'');assert.equal(queue.state().recording,true);assert.equal(audioCalls.length,closeTapMic,'tap closes the response without muting');
 handler({sysEvent:{eventType:0}});await tick();assert.match(queue.state().answer,/이븐: [_ ]/,'tap from a closed window arms a question');
 assert.equal(queue.lifetime.active,false);
 const manualTapStarts=requests.filter(r=>r.url.endsWith('/api/stt/transcribe')).length;
 nextTranscript='탭으로 호출한 질문입니다';
 for(let i=0;i<6;i++)handler({audioEvent:{audioPcm:speech}});
 await tick();
 assert.equal(requests.filter(r=>r.url.endsWith('/api/stt/transcribe')).length,manualTapStarts,'speech without a silence boundary remains buffered');
 sttReplies=[{ok:false,status:503,text:async()=>'전사 서비스 일시 지연'}];
 handler({sysEvent:{eventType:0}});await tick();
 assert.match(queue.state().answer,/질문 전사 중/);
 for(let i=0;i<100 && queue.state().title!=='탭으로 호출한 질문입니다';i++)await new Promise(resolve=>setTimeout(resolve,10));
 const manualRequests=requests.filter(r=>r.url.endsWith('/api/stt/transcribe')).slice(manualTapStarts).map(r=>JSON.parse(r.opts.body));
 assert.equal(manualRequests.length,2);
 assert.equal(manualRequests[0].segmentId,manualRequests[1].segmentId);
 assert.equal(manualRequests[0].audio,manualRequests[1].audio);
 assert.equal(queue.state().title,'탭으로 호출한 질문입니다');
 handler({sysEvent:{eventType:0}});await tick();
 assert.equal(queue.state().answer,'','third tap closes the answer');
 assert.equal(audioCalls.length,closeTapMic,'manual send and close keep the microphone on');
 element('hide-transcript').checked=false;await element('hide-transcript').handlers.change();
 element('trigger-transcript-only').checked=true;await element('trigger-transcript-only').handlers.change();
 assert.equal(element('hide-transcript').checked,false);
 nextTranscript='평소에 말한 일반 대화도 보여야 합니다';queue.enqueueClip(Buffer.alloc(6400),true,Date.now(),'ambient-hidden-by-trigger');for(let i=0;i<10;i++)await tick();
 assert.match(element('transcript-log').textContent,/평소에 말한 일반 대화/);assert.match(element('hud-screen').innerHTML,/hud-transcript-body[^>]*>평소에 말한 일반 대화/,'wake option must not hide ordinary speech when transcript hiding is off');
 nextTranscript='이븐';queue.enqueueClip(Buffer.alloc(6400),true,Date.now(),'wake-awaiting');for(let i=0;i<8;i++)await tick();
 assert.ok(element('hud-screen').innerHTML.includes('hud-transcript-box'),'ordinary transcript remains visible even before a wake question');
 assert.equal(element('hud-trigger-keyword').style.opacity,'1');
 handler({sysEvent:{eventType:0}});await tick();assert.equal(queue.state().recording,true);assert.equal(element('hud-trigger-keyword').style.opacity,'0.25','tap cancels question waiting');assert.equal(queue.state().answer,'');
 holdChat=true;chatReply=null;nextTranscript='이븐 피라미드를 누가 지었나요';queue.enqueueClip(Buffer.alloc(6400),true,Date.now(),'wake-with-question');for(let i=0;i<10;i++)await tick();
 assert.match(element('hud-screen').innerHTML,/피라미드를 누가 지었나요/);assert.ok(!element('hud-screen').innerHTML.includes('이븐 피라미드를'));
 assert.equal(element('hud-trigger-keyword').style.opacity,'1');
 releaseChat();for(let i=0;i<10;i++)await tick();
 assert.equal(queue.state().title,'피라미드를 누가 지었나요');
 assert.match(element('hud-screen').innerHTML,/hud-transcript-body">이븐 피라미드를 누가 지었나요/,'finished wake request returns to the normal transcript');
 nextTranscript='응답 도중 상관없는 새로운 전사';queue.enqueueClip(Buffer.alloc(6400),true,Date.now(),'other-transcript');for(let i=0;i<10;i++)await tick();
 assert.ok(element('hud-screen').innerHTML.includes('상관없는 새로운 전사'),'later ambient speech must not be replaced by an old question');assert.match(element('transcript-log').textContent,/상관없는 새로운 전사/);
 const voiceCloseMic=audioCalls.length;handler({sysEvent:{eventType:0}});await tick();await tick();assert.equal(queue.state().answer,'');assert.equal(queue.state().recording,true);assert.equal(audioCalls.length,voiceCloseMic);
 assert.ok(element('hud-screen').innerHTML.includes('hud-transcript-box'),'closing the AI card must not hide normal transcription');
 queue.showHint('기존 힌트','잠시 보이는 힌트','전사');holdChat=true;chatReply=null;
 element('chat-input').value='닫아서 취소할 새 질문';element('text-chat-form').handlers.submit({preventDefault(){}});for(let i=0;i<5;i++)await tick();
 const lateReply=releaseChat;handler({textEvent:{eventType:0}});lateReply();for(let i=0;i<12;i++)await tick();
 assert.equal(queue.state().answer,'');assert.equal(queue.state().recording,true,'late response cannot reopen a dismissed window');
 // Hiding alone must still hide an active wake question.
 element('hide-transcript').checked=true;await element('hide-transcript').handlers.change();
 element('trigger-transcript-only').checked=false;await element('trigger-transcript-only').handlers.change();
 holdChat=true;chatReply=null;nextTranscript='이븐 숨겨 둘 질문입니다';queue.enqueueClip(Buffer.alloc(6400),true,Date.now(),'hide-only-question');for(let i=0;i<10;i++)await tick();
 assert.ok(!element('hud-screen').innerHTML.includes('hud-transcript-box'),'hide on / wake exception off hides even the active question');releaseChat();for(let i=0;i<10;i++)await tick();queue.dismiss();
 element('trigger-transcript-only').checked=true;await element('trigger-transcript-only').handlers.change();
 // Both options can stay enabled: show the authorized question only before the panel appears.
 element('hide-transcript').checked=true;await element('hide-transcript').handlers.change();
 assert.equal(element('trigger-transcript-only').checked,true);assert.equal(element('hide-transcript').checked,true);
 queue.dismiss();nextTranscript='이븐';queue.enqueueClip(Buffer.alloc(6400),true,Date.now(),'both-options-wake-only');for(let i=0;i<10;i++)await tick();
 assert.match(queue.state().answer,/이븐: [_ ]/);
 holdChat=true;chatReply=null;nextTranscript='피라미드 높이를 알려줘';queue.enqueueClip(Buffer.alloc(6400),true,Date.now(),'both-options-followup');for(let i=0;i<12;i++)await tick();
 assert.match(element('hud-screen').innerHTML,/hud-ai-body">이븐: 피라미드 높이를 알려줘/,'wake-only acknowledgement must yield to the question preview');
 releaseChat();for(let i=0;i<12;i++)await tick();queue.dismiss();holdChat=true;chatReply=null;
 nextTranscript='이븐 청동기 시대가 언제인가요';queue.enqueueClip(Buffer.alloc(6400),true,Date.now(),'both-options-question');for(let i=0;i<10;i++)await tick();
 assert.match(element('hud-screen').innerHTML,/hud-ai-body">이븐: 청동기 시대가 언제인가요/,'show the sent question while waiting for the first answer');
 releaseChat();for(let i=0;i<10;i++)await tick();
 assert.equal(queue.state().title,'청동기 시대가 언제인가요');
 assert.ok(!element('hud-screen').innerHTML.includes('hud-transcript-box'),'never duplicate the title beneath a visible AI answer when both options are on');
 for(let i=0;i<150 && nativePage.textObject.some(c=>c.containerID===4);i++)await new Promise(resolve=>setTimeout(resolve,20));
 assert.ok(!nativePage.textObject.some(c=>c.containerID===4),'native answer also omits the transcript container');
 queue.showHint('청동기 시대','청동 도구를 사용하던 시기입니다.','숨길 전사');
 assert.ok(!element('hud-screen').innerHTML.includes('hud-transcript-box'),'the same rule applies to automatic hints');
 assert.equal(JSON.parse(nativeSaved).triggerTranscriptOnly,true);assert.equal(JSON.parse(nativeSaved).hideTranscript,true);
 queue.dismiss();assert.ok(!element('hud-screen').innerHTML.includes('hud-transcript-box'));

 // A speculative wake prepares only settings. Final confirmation sends exactly one chat.
 queue.ai();
 const previewWakeStart=requests.length;
 queue.preview({mode:'ai',segmentId:'preview-wake'},{text:'이븐 새로운 질문입니다'},{stable:'이븐 새로운 질문입니다'});
 for(let i=0;i<8;i++)await tick();
 assert.match(queue.state().answer,/이븐: 새로운 질문입니다/, 'preview wake must immediately display trigger word input box');
 assert.equal(element('hud-trigger-keyword').style.opacity,'1','preview wake must brighten wake keyword immediately');
 assert.equal(requests.slice(previewWakeStart).filter(r=>r.url.endsWith('/api/config')).length,0);
 assert.equal(requests.slice(previewWakeStart).filter(r=>r.url.endsWith('/api/chat')).length,0,'preview must never send an unconfirmed voice question');
 nextTranscript='이븐 새로운 질문입니다';queue.enqueueClip(Buffer.alloc(6400),true,Date.now(),'preview-wake');
 for(let i=0;i<20;i++)await tick();
 assert.equal(requests.slice(previewWakeStart).filter(r=>r.url.endsWith('/api/config')).length,0,'server owns settings; preview and final need no configuration fetch');
 assert.equal(requests.slice(previewWakeStart).filter(r=>r.url.endsWith('/api/chat')).length,1);
 assert.doesNotMatch(element('voice-latency').textContent,/설정 조회:/,'prompt requests no longer fetch client chat settings');
 assert.match(element('voice-latency').textContent,/호출어 감지→전송/);
 queue.dismiss();
 // Two cards: bounded native frames, no text-container overflow, independent closing.
 queue.dismiss();queue.ai();
 element('hint-card-count').value='2';await element('hint-card-count').handlers.change();
 element('hint-auto-close').checked=false;await element('hint-auto-close').handlers.change();
 element('hide-transcript').checked=false;await element('hide-transcript').handlers.change();
 element('trigger-transcript-only').checked=false;await element('trigger-transcript-only').handlers.change();
 queue.addHint('첫 번째 제목','오래된 본문 첫 줄\n오래된 본문 둘째 줄\n오래된 본문 셋째 줄\n오래된 본문 넷째 줄');
 queue.addHint('두 번째 제목','최신 본문 첫 줄\n최신 본문 둘째 줄\n최신 본문 셋째 줄\n최신 본문 넷째 줄');
 for(let i=0;i<150 && !nativePage.textObject.some(c=>c.containerID===10);i++)await new Promise(r=>setTimeout(r,20));
 assert.equal(queue.stack.cards.length,2);assert.ok(nativePage.textObject.length<=8,'two cards and transcript must obey the SDK eight-text limit');
 assert.equal(nativePage.containerTotalNum,nativePage.textObject.length+nativePage.imageObject.length);
 assert.ok(nativePage.textObject.some(c=>c.containerID===4),'transcript remains available with two cards');
 assert.match(nativePage.textObject.find(c=>c.containerID===9).content,/두 번째 제목/);
 assert.match(nativePage.textObject.find(c=>c.containerID===10).content,/첫 번째 제목/);
 const nativeCards=nativePage.textObject.filter(c=>[1,9,10,12].includes(c.containerID));
 assert.equal(nativeCards.length,4,'each card must have its own title and body');
 for(let i=0;i<nativeCards.length;i++)for(let j=i+1;j<nativeCards.length;j++)assert.ok(nativeCards[i].yPosition+nativeCards[i].height<=nativeCards[j].yPosition || nativeCards[j].yPosition+nativeCards[j].height<=nativeCards[i].yPosition,'native card rectangles must never overlap');
 assert.ok(nativePage.textObject.every(c=>c.yPosition+c.height<=288));
 const stackSnapshots=[{html:element('hud-screen').innerHTML,native:JSON.parse(JSON.stringify(nativePage))}];
 const bodies=nativePage.textObject.find(c=>c.containerID===1).content;
 assert.ok(bodies.includes('최신 본문'));assert.ok(!bodies.includes('오래된 본문'));assert.match(nativePage.textObject.find(c=>c.containerID===12).content,/오래된 본문/);assert.ok(!bodies.includes('제목'),'both titles stay outside the scrolling body');
 const topBeforeOlderScroll=nativePage.textObject.find(c=>c.containerID===1).content;
 const olderBefore=nativePage.textObject.find(c=>c.containerID===12).content;
 queue.stack.scroll(1,queue.stack.cards[1].id);
 for(let i=0;i<100 && nativePage.textObject.find(c=>c.containerID===12).content===olderBefore;i++)await new Promise(r=>setTimeout(r,10));
 assert.notEqual(nativePage.textObject.find(c=>c.containerID===12).content,olderBefore,'older body must receive independent scroll updates');
 assert.equal(nativePage.textObject.find(c=>c.containerID===1).content,topBeforeOlderScroll);
 element('ai-header-layout').value='status-icon';await element('ai-header-layout').handlers.change();
 for(let i=0;i<100 && nativePage.textObject.some(c=>c.containerID===8);i++)await new Promise(r=>setTimeout(r,10));
 assert.ok(nativePage.textObject.some(c=>c.containerID===12));assert.ok(nativePage.textObject.length<=8);
 element('ai-header-layout').value='standard';await element('ai-header-layout').handlers.change();
 for(let i=0;i<100 && !nativePage.textObject.some(c=>c.containerID===8);i++)await new Promise(r=>setTimeout(r,10));
 assert.equal(nativePage.textObject.length,8);assert.ok(!nativePage.textObject.some(c=>c.containerID===7),'dense full header merges bracket and wake label');
 queue.addHint('세 번째 제목','교체된 새 본문');assert.deepEqual(Array.from(queue.stack.cards,c=>c.title),['[정보] 세 번째 제목','[정보] 두 번째 제목']);
 const cardMicCalls=audioCalls.length;handler({textEvent:{eventType:0}});await tick();
 assert.equal(queue.stack.cards.length,1);assert.match(queue.state().title,/두 번째 제목/);assert.equal(queue.state().recording,true);assert.equal(audioCalls.length,cardMicCalls,'tap closes a visible hint without muting');
 queue.addHint('폰에서 닫을 이전 카드','폰에서 선택할 본문');queue.addHint('네 번째 제목','폰에서 남길 본문');const oldCardID=queue.stack.cards[1].id;
 element('hud-screen').handlers.click({target:{closest:()=>({dataset:{hintCard:oldCardID}})}});await tick();
 assert.equal(queue.stack.cards.length,1);assert.match(queue.state().title,/네 번째 제목/,'phone closes the selected older card');
 element('hint-card-count').value='1';await element('hint-card-count').handlers.change();
 assert.equal(queue.stack.cards.length,0);assert.match(queue.state().title,/네 번째 제목/,'switching to one card keeps the newest hint');
 element('hint-card-count').value='2';await element('hint-card-count').handlers.change();
 assert.equal(queue.stack.cards.length,1,'enabling a stack adopts the currently displayed hint');
 queue.dismiss();
 const revisionMeta={id:'revision-card',preview:true,text:'가상 장치의 원리',focusText:'가상 장치',hintPlan:{taskOriented:true,anchor:'가상 장치',topic:'장치 원리'}};
 queue.addHint('수정될 카드','임시 전사에서 만든 설명입니다.',revisionMeta);
 queue.addHint('다른 카드','변경되지 않은 다른 발화의 설명입니다.',{...revisionMeta,id:'other-card'});
 queue.reviseTranscript('revision-card','아예 다른 물건으로 최종 수정되었습니다.');
 assert.equal(queue.stack.cards.length,1);assert.match(queue.stack.cards[0].title,/다른 카드/,'only the card tied to revised evidence is retracted');
 element('hint-card-count').value='1';await element('hint-card-count').handlers.change();
 queue.dismiss();queue.addHint('단일 수정 카드','잘못된 전사에서 만든 설명입니다.',revisionMeta);
 queue.reviseTranscript('revision-card','다른 내용으로 최종 수정되었습니다.');
 assert.equal(queue.state().answer,'','a corrected final transcript retracts the single visible hint too');
 element('hint-card-count').value='2';await element('hint-card-count').handlers.change();
 queue.dismiss();hintClock=100000;
 element('hint-auto-close').checked=true;await element('hint-auto-close').handlers.change();element('hint-seconds-per-line').value='1';await element('hint-seconds-per-line').handlers.change();
 queue.addHint('먼저 닫힐 카드','앞 카드 첫 줄\n앞 카드 둘째 줄');hintClock=101000;
 queue.addHint('나중에 닫힐 카드','뒤 카드 첫 줄\n뒤 카드 둘째 줄\n뒤 카드 셋째 줄');const survivorDeadline=queue.stack.cards[0].deadline;
 hintClock=102000;hintJobs.at(-1)();assert.equal(queue.stack.cards.length,1);assert.equal(queue.stack.cards[0].deadline,survivorDeadline);assert.equal(queue.stack.remaining(queue.stack.cards[0]),2);
 hintClock=104000;hintJobs.at(-1)();assert.equal(queue.stack.cards.length,0);assert.equal(queue.state().answer,'');assert.equal(queue.state().recording,true);
 element('hint-auto-close').checked=false;await element('hint-auto-close').handlers.change();
 element('hide-transcript').checked=true;await element('hide-transcript').handlers.change();
 queue.addHint('긴 첫 카드',Array.from({length:8},(_,i)=>`첫 카드 본문 ${i}`).join('\n'));queue.addHint('긴 두 번째 카드',Array.from({length:8},(_,i)=>`둘째 카드 본문 ${i}`).join('\n'));
 for(let i=0;i<150 && nativePage.textObject.some(c=>c.containerID===4);i++)await new Promise(r=>setTimeout(r,20));
 assert.ok(!nativePage.textObject.some(c=>c.containerID===4));assert.ok(nativePage.textObject.length<=8);
 stackSnapshots.push({html:element('hud-screen').innerHTML,native:JSON.parse(JSON.stringify(nativePage))});
 fs.writeFileSync('/private/tmp/dkst-card-stack-preview.json',JSON.stringify(stackSnapshots));
 nextTranscript='이븐';queue.enqueueClip(Buffer.alloc(6400),true,Date.now(),'stack-wake-only');for(let i=0;i<15;i++)await tick();
 assert.equal(queue.stack.cards.length,0,'explicit wake acknowledgement retains its single answer window');assert.match(queue.state().answer,/이븐: [_ ]/);
 queue.dismiss();
 element('trigger-transcript-only').checked=true;await element('trigger-transcript-only').handlers.change();
 element('hint-auto-close').checked=true;await element('hint-auto-close').handlers.change();element('hint-seconds-per-line').value='2.5';await element('hint-seconds-per-line').handlers.change();
 element('head-gesture-enabled').checked=true;await element('head-gesture-enabled').handlers.change();await queue.stopVoiceRecording(false);
 // Subtitle-only translation uses bottom rows and the shared AI icon position.
 queue.enableIcons();queue.headerMode('translate');queue.translationHistory.reset();
 element('translation-layout').value='subtitles';await element('translation-layout').handlers.change();
 assert.equal(element('ai-icon-position').disabled,false,'subtitle icon position remains editable with standard AI header');
 for(const rows of [1,2,3,4,5]) {
   queue.translationHistory.reset();queue.translationHistory.update('subtitles',Array.from({length:8},(_,i)=>`번역 자막 ${i+1}`).join('\n'));
   element('subtitle-lines').value=String(rows);await element('subtitle-lines').handlers.change();
   for(let i=0;i<100 && (nativePage.textObject[0]?.height!==rows*27 || nativePage.textObject.length!==1);i++)await new Promise(r=>setTimeout(r,10));
   assert.deepEqual(Array.from(nativePage.textObject,c=>c.containerID),[1]);
   const caption=nativePage.textObject[0];assert.equal(caption.borderWidth,0);assert.equal(caption.paddingLength,0);
   assert.equal(caption.yPosition+caption.height,288);assert.equal(caption.height,rows*27);
   assert.equal(caption.content.split('\n').length,rows);assert.ok(caption.content.endsWith('번역 자막 8'));
   assert.deepEqual(Array.from(nativePage.imageObject,c=>c.containerID),[11]);
   assert.equal(nativePage.imageObject[0].yPosition+27,caption.yPosition);
   assert.equal(element('hud-status-row').hidden,true);assert.equal(element('hud-page-indicator').hidden,true);
   assert.doesNotMatch(element('hud-screen').innerHTML,/예상 답변|hud-ai-box|hud-transcript-box/);
 }
 for(const x of [0,276,552]){
   element('ai-icon-position').value=String(x);await element('ai-icon-position').handlers.change();
   for(let i=0;i<100 && nativePage.imageObject[0]?.xPosition!==x;i++)await new Promise(r=>setTimeout(r,10));
   assert.equal(nativePage.imageObject[0].xPosition,x,'translation uses the shared AI slider');
 }
 queue.translationHistory.reset();queue.translationHistory.update('short-subtitle','한 줄 자막');queue.headerMode('translate');
 for(let i=0;i<100 && !nativePage.textObject[0].content.endsWith('한 줄 자막');i++)await new Promise(r=>setTimeout(r,10));
 assert.equal(nativePage.textObject[0].content,'한 줄 자막','a new subtitle page starts at its first row');
 assert.equal(JSON.parse(nativeSaved).translationLayout,'subtitles');assert.equal(JSON.parse(nativeSaved).subtitleLines,5);
 // Rolling context survives history pruning (30-row memory cap).
 element('subtitle-lines').value='4';await element('subtitle-lines').handlers.change();queue.translationHistory.reset();
 for(let i=1;i<=41;i++){
   queue.translationHistory.update('page-'+i,'자막 '+i,{final:true});queue.headerMode('translate');
   const count=Math.min(i,4);
   const expected=Array.from({length:count},(_,j)=>'자막 '+(i-count+1+j)).join('\n');
   for(let n=0;n<100 && nativePage.textObject[0].content!==expected;n++)await new Promise(r=>setTimeout(r,5));
   assert.equal(nativePage.textObject[0].content,expected,'only the oldest row scrolls away, including after history pruning');
 }
 // A single newly wrapped syllable must not replace all of the readable context.
 element('subtitle-lines').value='2';await element('subtitle-lines').handlers.change();queue.translationHistory.reset();
 queue.translationHistory.update('orphan-ending','이전 줄입니다.\n안녕하세');queue.headerMode('translate');
 for(let i=0;i<100 && nativePage.textObject[0].content!=='이전 줄입니다.\n안녕하세';i++)await new Promise(r=>setTimeout(r,5));
 assert.equal(nativePage.textObject[0].content,'이전 줄입니다.\n안녕하세');
 queue.translationHistory.update('orphan-ending','이전 줄입니다.\n안녕하세\n요.',{final:true});queue.headerMode('translate');
 for(let i=0;i<100 && nativePage.textObject[0].content!=='안녕하세\n요.';i++)await new Promise(r=>setTimeout(r,5));
 assert.equal(nativePage.textObject[0].content,'안녕하세\n요.','overflowing sentence ending retains the preceding line');
 const subtitleChatStart=requests.filter(r=>r.url.endsWith('/api/chat')).length;
 for(const engine of ['llm','apple','chrome'])queue.liveTranslations.onFinal({id:'subtitle-'+engine,prompt:'Synthetic subtitle.',context:'',source:'en',target:'ko',engine,token:queue.liveTranslations.token},{ok:true,text:'자막 번역'});
 await tick();await tick();
 assert.equal(requests.filter(r=>r.url.endsWith('/api/chat')).length,subtitleChatStart,'successful subtitles never request reply generation');
 chatReply=sseReply('data: '+JSON.stringify({type:'message.delta',content:JSON.stringify({translation:'대체 번역 자막'})})+'\n\ndata: [DONE]\n\n');
 queue.liveTranslations.onFinal({id:'subtitle-fallback',prompt:'Synthetic fallback subtitle.',context:'',source:'en',target:'ko',engine:'apple',token:queue.liveTranslations.token},{ok:false,reason:'synthetic failure'});
 await tick();await tick();await tick();
 const subtitleRequests=requests.filter(r=>r.url.endsWith('/api/chat')).slice(subtitleChatStart).map(r=>JSON.parse(r.opts.body));
 assert.equal(subtitleRequests.length,1);assert.equal(requests.filter(r=>r.url.endsWith('/api/chat')).at(-1).opts.glassesTask,'translation_only','local fallback requests only translation, never combined replies');
 captionRetention.value='3';await captionRetention.handlers.change();
 queue.liveTranslations.onText({id:'expiring-translation',draft:false},'시간이 지나면 사라질 번역');
 assert.match(element('hud-screen').innerHTML,/시간이 지나면 사라질 번역/);
 await new Promise(resolve=>setTimeout(resolve,3100));
 assert.doesNotMatch(element('hud-screen').innerHTML,/시간이 지나면 사라질 번역/);
 assert.doesNotMatch(element('chat-output').textContent,/시간이 지나면 사라질 번역/);
 captionRetention.value='0';await captionRetention.handlers.change();

 element('translation-layout').value='standard';await element('translation-layout').handlers.change();
 for(let i=0;i<100 && !nativePage.textObject.some(c=>c.containerID===2);i++)await new Promise(r=>setTimeout(r,10));
 assert.ok(nativePage.textObject.some(c=>c.containerID===2),'standard header restores');assert.ok(nativePage.textObject.some(c=>c.containerID===4),'reply examples restore');
 assert.equal(element('hud-status-row').hidden,false);queue.headerMode('ai');
 // Closing a wake preview also cancels its later preview and final transcript.
 queue.ai();queue.showHint('','','');element('glasses-mode').value='ai';
 const dismissedPreviewChatCount=requests.filter(r=>r.url.endsWith('/api/chat')).length;
 const dismissedSnapshot={mode:'ai',segmentId:'dismissed-preview-wake'};
 queue.preview(dismissedSnapshot,{text:'이븐 취소할 질문'},{stable:'이븐 취소할 질문'});
 assert.match(queue.state().answer,/취소할 질문/);
 const dismissedPreviewMic=audioCalls.length;handler({sysEvent:{eventType:0}});await tick();
 assert.equal(queue.state().answer,'');assert.equal(audioCalls.length,dismissedPreviewMic);
 queue.preview(dismissedSnapshot,{text:'이븐 취소할 질문의 늦은 수정'},{stable:'이븐 취소할 질문의 늦은 수정'});
 assert.equal(queue.state().answer,'','late wake preview cannot reopen the closed window');
 sttReplies=[{ok:true,json:async()=>({text:'이븐 취소할 질문',saved:true,triggered:true,prompt:'취소할 질문'})}];
 queue.enqueueClip(Buffer.alloc(6400),true,Date.now(),dismissedSnapshot.segmentId);for(let i=0;i<12;i++)await tick();
 assert.equal(queue.state().answer,'','canceled wake final cannot reopen the closed window');
 assert.equal(requests.filter(r=>r.url.endsWith('/api/chat')).length,dismissedPreviewChatCount);
 // Tap must not let an older in-flight transcript consume the next question.
 queue.ai();queue.showHint('','','');element('glasses-mode').value='ai';holdChat=false;chatReply=null;
 let finishPriorTapAudio;
 sttReplies=[()=>new Promise(resolve=>{finishPriorTapAudio=()=>resolve({ok:true,json:async()=>({text:'탭 전 주변 대화',saved:true,triggered:false})});}),{ok:true,json:async()=>({text:'호출어 없이 탭한 뒤 질문',saved:true,triggered:false})}];
 queue.enqueueClip(Buffer.alloc(6400),true,Date.now(),'before-tap-inflight');await tick();
 const beforeTapAudioCalls=audioCalls.length;
 handler({sysEvent:{eventType:0}});await tick();
 assert.match(queue.state().answer,/이븐: [_ ]/);assert.equal(audioCalls.length,beforeTapAudioCalls,'AI tap must not mute an active mic');
 finishPriorTapAudio();for(let i=0;i<10;i++)await tick();
 assert.match(queue.state().answer,/이븐: [_ ]/,'pre-tap speech cannot consume the armed question');
 queue.enqueueClip(Buffer.alloc(6400),true,Date.now(),'after-tap-question');for(let i=0;i<15;i++)await tick();
 assert.equal(queue.state().title,'호출어 없이 탭한 뒤 질문');assert.equal(queue.state().recording,true);
 // A delayed finalized wake call must still route to chat, not be discarded.
 sttReplies=[{ok:true,json:async()=>({text:'이븐 지연된 질문',saved:true,triggered:true,prompt:'지연된 질문'})}];
 queue.enqueueClip(Buffer.alloc(6400),true,Date.now()-30000,'delayed-wake');for(let i=0;i<15;i++)await tick();
 assert.equal(queue.state().title,'지연된 질문','age alone must not drop a wake before recognition');
 queue.dismiss();await queue.stopVoiceRecording(false);
 const stoppedTapOpens=audioCalls.filter(c=>c[0]).length;
 handler({sysEvent:{eventType:0}});handler({sysEvent:{eventType:0}});
 for(let i=0;i<20;i++)await tick();
 assert.equal(queue.state().recording,true,'AI tap starts STT when stopped');assert.equal(queue.state().answer,'','second tap during microphone opening cancels the window');
 handler({sysEvent:{eventType:0}});await tick();assert.match(queue.state().answer,/이븐: [_ ]/);
 const secondTapMic=audioCalls.length;handler({sysEvent:{eventType:0}});await tick();assert.equal(queue.state().answer,'');assert.equal(audioCalls.length,secondTapMic,'closing listening window preserves STT');
 assert.equal(audioCalls.filter(c=>c[0]).length,stoppedTapOpens+1,'rapid taps open one microphone');
 await queue.stopVoiceRecording(false);
 const originalCaptionSettings=JSON.parse(nativeSaved);
 await queue.settings({sttEngine:'whisper',translationEngine:'apple',translateSourceLang:'ja',translateTargetLang:'ko'});
 element('hide-transcript').checked=true;await element('hide-transcript').handlers.change();
 element('trigger-transcript-only').checked=false;await element('trigger-transcript-only').handlers.change();
 const foreignTranslation=element('ai-foreign-translation');
 assert.equal(foreignTranslation.checked,false);
 foreignTranslation.checked=true;await foreignTranslation.handlers.change();
 assert.equal(JSON.parse(nativeSaved).aiForeignTranslation,true);
 const aiPCM={mode:'ai',sourceLang:'ja',targetLang:'ko',bytes:new Uint8Array(6400),revision:1};
 assert.equal(queue.sttPayload({...aiPCM,engine:'whisper'}).language,'auto');
 for(const engine of ['apple','chrome-online'])assert.equal(queue.sttPayload({...aiPCM,engine}).language,'ko');
 await queue.microphone();
 assert.equal(queue.state().recording,true);assert.equal(queue.state().engine,'whisper');
 const captionRequests=requests.length;
 appleText='외국어 발화의 한국어 번역입니다.';
 sttReplies=[{ok:true,json:async()=>({text:'This is an English caption.',language:'en',engine:'whisper',saved:true})}];
 queue.enqueueClip(Buffer.alloc(6400),true,Date.now(),'foreign-caption');
 for(let i=0;i<15;i++)await tick();
 assert.match(element('hud-screen').innerHTML,/외국어 발화의 한국어 번역입니다/);
 assert.match(element('hud-screen').innerHTML,/hud-transcript-label">번역</);
 assert.doesNotMatch(element('hud-screen').innerHTML,/This is an English caption/);
 for(let i=0;i<100 && !nativePage.textObject.some(c=>c.containerID===4 && c.content.includes('외국어 발화'));i++)await new Promise(r=>setTimeout(r,10));
 assert.ok(nativePage.textObject.some(c=>c.containerID===4 && c.content.includes('외국어 발화')));
 for(let i=0;i<100 && !Number.isFinite(queue.metrics.entries.get('foreign-caption')?.sdk_ms);i++)await new Promise(r=>setTimeout(r,10));
 const captionMetrics=queue.metrics.entries.get('foreign-caption');
 for(const key of ['stt_queue_ms','stt_http_ms','translation_queue_ms','translation_http_ms','sdk_ms']){
   assert.ok(Number.isFinite(captionMetrics?.[key]),'AI caption metrics include '+key);
 }
 assert.match(element('transcript-log').textContent,/This is an English caption/);
 const captionCalls=requests.slice(captionRequests);
 assert.equal(JSON.parse(captionCalls.find(r=>r.url.endsWith('/api/stt/transcribe')).opts.body).language,'auto');
 const captionTranslation=JSON.parse(captionCalls.find(r=>r.url.endsWith('/api/translate/apple')).opts.body);
 assert.equal(captionTranslation.source,'en');assert.equal(captionTranslation.target,'ko');
 const sameLanguageRequests=requests.length;
 sttReplies=[{ok:true,json:async()=>({text:'같은 언어는 그대로 표시합니다.',language:'ko',engine:'whisper',saved:true})}];
 queue.enqueueClip(Buffer.alloc(6400),true,Date.now(),'same-language-caption');
 for(let i=0;i<15;i++)await tick();
 assert.doesNotMatch(element('hud-screen').innerHTML,/같은 언어는 그대로 표시합니다|외국어 발화/);
 assert.match(element('transcript-log').textContent,/같은 언어는 그대로 표시합니다/);
 assert.ok(!requests.slice(sameLanguageRequests).some(r=>r.url.endsWith('/api/translate/apple')));
 await queue.stopVoiceRecording(false);
 foreignTranslation.checked=false;await foreignTranslation.handlers.change();
 element('hide-transcript').checked=originalCaptionSettings.hideTranscript;await element('hide-transcript').handlers.change();
 element('trigger-transcript-only').checked=originalCaptionSettings.triggerTranscriptOnly;await element('trigger-transcript-only').handlers.change();
 const {sttEngine,translationEngine,translateSourceLang,translateTargetLang}=originalCaptionSettings;
 await queue.settings({sttEngine,translationEngine,translateSourceLang,translateTargetLang});

 // Qwen follows local-model preparation and detection, never Chrome's locale API.
 await queue.stopVoiceRecording(false);
 const qwenStart=requests.length;
 await queue.settings({sttEngine:'qwen3-asr',glassesMode:'translate',translateSourceLang:'auto',translateTargetLang:'ko'});
 for(let i=0;i<6;i++)await tick();
 assert.equal(element('stt-engine').value,'qwen3-asr');
 assert.ok(requests.slice(qwenStart).some(r=>r.url.endsWith('/api/stt/warmup?engine=qwen3-asr')));
 assert.ok(!requests.slice(qwenStart).some(r=>r.url.endsWith('/api/stt/chrome/prepare') || r.url.endsWith('/api/stt/chrome/stop')));
 assert.equal(queue.sttPayload({engine:'qwen3-asr',mode:'translate',sourceLang:'ja',targetLang:'ko',bytes:new Uint8Array(3200)}).language,'auto');
 assert.equal(queue.sttPayload({engine:'qwen3-asr',mode:'translate',sourceLang:'ko',targetLang:'en',bytes:new Uint8Array(3200)}).language,'ko');
 assert.equal(queue.sttPayload({engine:'qwen3-asr',mode:'ai',aiForeignTranslation:true,targetLang:'ko',bytes:new Uint8Array(3200)}).language,'auto');
 qwenWarmupFailure=true;
 await queue.microphone();for(let i=0;i<8;i++)await tick();
 assert.equal(queue.state().engine,'whisper','explicit fallback uses Whisper when Qwen preparation fails');
 await queue.stopVoiceRecording(false);qwenWarmupFailure=false;
 await queue.settings({sttEngine,translationEngine,translateSourceLang,translateTargetLang});
 console.log('PASS: Qwen local warmup, language payloads, no Chrome requests and preparation fallback');
 await queue.settings({glassesMode:'ai',connectionMode:'sse'});
 queue.enableIcons();
 const screenshotPath='/api/computer/screenshots/0123456789ab-00112233445566778899aabb.png';
 const secondScreenshotPath='/api/computer/screenshots/0123456789ab-ffeeddccbbaa998877665544.png';
 const frame=new Uint8Array(288*144/2).fill(0x88);
 const requestedShots=[];
 context.fetchChatImage=async(_fetch,server,path,headers)=>{requestedShots.push({server,path,headers});return frame;};
 const screenshotStart=imageUpdates.length;
 chatReply={ok:true,body:{getReader(){
  let sent=false;
  return {read:async()=>sent?{done:true}:(sent=true,{done:false,value:Buffer.from('data: '+JSON.stringify({type:'request.complete',final_assistant_content:'캡처 결과\n\n![창]('+screenshotPath+')\n\n![두 번째 창]('+secondScreenshotPath+')'})+'\n\n')})};
 }}};
 await queue.queryChat('화면 캡처','','ai');
 for(let i=0;i<30 && !imageUpdates.slice(screenshotStart).some(update=>update.containerID===14);i++)await tick();
 assert.deepEqual(requestedShots.map(shot=>shot.path),[screenshotPath,secondScreenshotPath]);
 for(const shot of requestedShots)assert.match(shot.headers.Authorization,/^Bearer /);
 const screenshotContainer=nativePage.imageObject.find(container=>container.containerID===13);
 assert.equal(screenshotContainer.width,288);
 assert.equal(screenshotContainer.height,144);
 assert.equal(nativePage.imageObject.find(container=>container.containerID===14).xPosition,288);
 assert.equal(imageUpdates.slice(screenshotStart).find(update=>update.containerID===14).imageData, frame);
 assert.equal(nativePage.textObject.find(container=>container.containerID===1).yPosition,171);
 assert.equal(imageUpdates.slice(screenshotStart).find(update=>update.containerID===13).imageData, frame);
 assert.equal(queue.state().answer,'캡처 결과');
 queue.dismiss();
 for(let i=0;i<20;i++)await tick();
 assert.ok(!nativePage.imageObject.some(container=>container.containerID===13),'dismissing the answer removes its screenshot');
 console.log('PASS: authenticated chat screenshot attachment renders in a validated Gray4 HUD container');
 const restoredPrepareStart=requests.length;
 nativeSaved=JSON.stringify({...JSON.parse(nativeSaved),aiHeaderLayout:'status-icon',aiIconPosition:276,translationLayout:'subtitles',subtitleLines:3});
 elements.clear();
 vm.runInNewContext(src,context);await tick();await tick();
 assert.equal(element('server-url').value,'https://192.168.1.4:2806');
 assert.equal(element('ai-header-layout').value,'status-icon','AI header choice restores with native settings');
 assert.equal(element('ai-icon-position').value,'276','saved center position restores');
 assert.equal(element('translation-layout').value,'subtitles');assert.equal(element('subtitle-lines').value,'3','subtitle settings restore');
 assert.equal(element('head-gesture-enabled').checked,true);assert.equal(context.window.__queueTest.headState().imu,true,'saved head gesture starts IMU after startup');
 assert.match(element('login-status-label').textContent,/인증됨/,'new WebView with empty browser storage must restore native login');
 assert.equal(element('auth-password').value,'');
 assert.equal(element('hide-transcript').checked,true,'both display options restore independently');
 assert.equal(element('trigger-transcript-only').checked,true,'trigger-only display restores from native storage');
 assert.equal(element('hint-card-count').value,'2','card count restores from native storage');
 assert.equal(element('hint-auto-close').checked,true);assert.equal(element('hint-seconds-per-line').value,'2.5','per-line duration restores from native storage');
 assert.equal(element('model-ai-chat').value,'chat-role');assert.equal(element('model-translation-reply').value,'reply-role');
 assert.equal(element('audio-input-source').value,'phone','phone source must survive native settings restore');
 assert.equal(element('glasses-sensitivity').value,'very_high');assert.equal(element('glasses-gain').value,'4');
 assert.ok(requests.slice(restoredPrepareStart).some(r=>r.url.endsWith('/api/stt/apple/prepare')),'restored native settings prepare after authenticated reconnect');
 const legacy=JSON.parse(nativeSaved);delete legacy.translationLayout;delete legacy.subtitleLines;delete legacy.aiHeaderLayout;delete legacy.aiIconPosition;delete legacy.translationEngine;delete legacy.audioInputSource;delete legacy.glassesInput;legacy.chromeTranslation=false;nativeSaved=JSON.stringify(legacy);
 elements.clear();vm.runInNewContext(src,context);await tick();await tick();
 assert.equal(element('translation-layout').value,'standard');assert.equal(element('subtitle-lines').value,'2');
 assert.equal(element('ai-header-layout').value,'standard','missing layout preference defaults to existing full header');
 assert.equal(element('ai-icon-position').value,'552','missing position defaults to the original right edge');
 assert.equal(element('translation-engine').value,'llm','legacy native preferences must preserve LLM translation after asynchronous restore');
 assert.equal(element('chrome-translation').checked,false);
 assert.equal(element('audio-input-source').value,'glasses','legacy settings default to glasses');
 assert.equal(element('glasses-sensitivity').value,'high');assert.equal(element('glasses-gain').value,'1','legacy settings must use relaxed glasses detection without enabling gain');
 nativeSaved=JSON.stringify({...JSON.parse(nativeSaved),authToken:'expired-session'});
 elements.clear();vm.runInNewContext(src,context);await tick();await tick();
 assert.match(element('login-status-label').textContent,/세션 만료/);
 assert.equal(JSON.parse(nativeSaved).authToken,'','expired tokens must be removed from native storage');
 console.log('PASS: backlog preservation/retry/timeout/mode races/backpressure, text chat/account protocol/LM Studio SSE/error details, MIC stop during warmup/open/preview, close retry, native settings/login restore, long transcript modal, incremental drafts/local agreement/stale rejection/AI independence, continuous capture/trigger/transcript queue, model formats/labels/chat model ID, startup, auth, tap/scroll/audio events, PCM/STT, fragmented SSE, serialized HUD, exit confirmation');
})().catch(e=>{console.error(e);process.exitCode=1;}).finally(()=>{clearInterval(testKeepAlive);clearTimeout(testWatchdog);});
