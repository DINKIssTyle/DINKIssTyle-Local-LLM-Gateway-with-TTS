import {SETTINGS_MENUS,settingsMenuItems,settingsPickerPage} from './settings-menu.mjs';
import {GlassesInputProcessor,normalizeGlassesInput,inputSegmentOptions,levelDb,GLASSES_THRESHOLDS} from './glasses-input.mjs';
import {headerLayout} from './header-layout.mjs';
import {HudIconStore,microphoneIcon,hudIconPath} from './hud-icons.mjs';
import {EnginePreparation,waitForPreparation} from './engine-preparation.mjs';
import {chatDeadline,chatFailureMessage} from './chat-deadline.mjs';
import {ROLE_MODEL_KEYS,normalizeRoleModels,resolveRoleModel,llmRequestRole,roleModelOptions,translationOnlyPrompts} from './role-models.mjs';
import {LiveTranslationQueue} from './live-translation.mjs';
import {StableTranslation} from './stable-translation.mjs';
import {TranslationSentence} from './translation-sentence.mjs';
import {TranslationHistory} from './translation-history.mjs';
import {ChatMemory,routeAIUtterance} from './ai-conversation.mjs';
import {localTranslate,translationReplyPrompts} from './chrome-translation.mjs';
import {filterSpeakerPCM} from './speaker-filter.mjs';
import { mergeTranslationDisplay, hasCompleteTranslationReplies, parseTranslationDisplay, translationReplyText, translationPhoneText } from './translation-display.mjs';
import { directQuestionIntent } from './direct-question.mjs';
import { cuePrompts, parseCueResponse, fitCue, hintPlanPrompts, parseHintPlan, hintAnswerPrompts, extractHintFact, readHintPlanResponse } from './hint-enrichment.mjs';
import { HintGate } from './hint-gate.mjs';
import { waitForEvenAppBridge, AudioInputSource, OsEventTypeList } from '@evenrealities/even_hub_sdk';
import { normalizeModels } from './model-list.mjs';
import { SpeechSegmenter, mergePCM } from './audio-segments.mjs';
import { modeHeaderText, translationHeaderText, rightAlignedStatusText, hudStatusText, hudNoticeText, withAnimatedDots, extractVisibleAndReasoning, stripSourcesForGlasses, formatToolStatusText, sliceHUDWindow, normalizeGlassesMode, nextGlassesMode, glassesModeMenuIdToMode, targetLanguageLabel, glassesModeShortLabel, glassesModeBadgeText, buildModePrompts } from './hud-status.mjs';
import { LocalAgreement } from './local-agreement.mjs';

/**
 * DKST AI Gateway Plugin for Even Realities G2
 * Powered by @evenrealities/even_hub_sdk
 */

(function () {
  'use strict';

  // --- Configuration & State ---
  const STORAGE_KEY = 'dkst_even_realities_settings';
  let config = {
    serverUrl: DKST_DEFAULT_SERVER_URL || window.location.origin,
    username: '',
    apiKey: '',
    model: '',
    roleModels:{},
    connectionMode: 'sse',
    conciseMode: true,
    aiHintMode: true,
    hintSensitivity: 'balanced',
    hintWebSearch: true,
    glassesMode: 'ai',
    translateSourceLang: 'auto',
    translateTargetLang: 'ko',
    translateExcludeSelf: true,
    chromeTranslation: true,
    translationEngine: "",
    voiceMode: 'continuous',
    triggerWord: '이븐',
    speechFilter: 'strict',
    sttEngine: 'whisper',
    audioInputSource:'glasses',
    glassesInput:{sensitivity:'high',gain:1},
    sttFallback: true,
    speakerScope: 'conversation',
    enableTTS: false,
    authToken: ''
  };

  let activeSttEngine='whisper';
  let activeAudioInputSource=AudioInputSource.Glasses;
  let activeGlassesInput=normalizeGlassesInput(config.glassesInput),activeInputProcessor=new GlassesInputProcessor(activeGlassesInput),lastInputMeterAt=0;
  let bridge = null;
  let settingsEdited = false;
  let pendingNativeSave = false;
  let previousBodyOverflow = '';
  let recordReturnFocus = null;
  let displayReady = false;
  let micBusy = false;
  let capturePreparing = false;
  let captureGeneration = 0;
  let captureAbort = null;
  let captureTask=null,inputSwitchPromise=null;
  let pendingSettings={}, resumeAfterSettings=false, recognitionEpoch=0;
  let settingsPicker=null, nativePickerKey='',nativeMenuSource='';
  let pendingMicOpen = null;
  let micStopPromise = null;
  let micMayBeOpen = false;
  const pendingMicCloses = new Set();
  let micCloseUncertain = false;
  const pendingHUD = new Map();
  let statusTimer = null;
  let glassesSN = null;
  let glassesBattery = null;
  let unsubscribeDeviceStatus = null;
  let lastStatus = '';
  let sendingHUD = false;
  let isRecording = false;
  let audioBuffer = [];
  let isThinking = false;
  let sessionId = '';
  let recordedBytes = 0;
  let recordingStarted = 0;
  let captureTimer = null;
  let manualTimer = null;
  let segmenter = null;
  let pendingClips = [];
  let draining = false;
  let activeFinalClip = null;
  let activeUtterance = null;
  let pendingPreview = null;
  let previewBusy = false;
  let draftUtteranceId = '';
  let captionFinal = '';
  let captureFailure = '';
  let captionStable = '';
  let captionDraft = '';
  let answerText = '';
  let translationDisplay={translation:'',positive:'',negative:''};
  let translationToken=0,translating=false,liveTranslationDraft=false,translationFailure='';
  let answerHoldUntil = 0;
  let answerTimer = null;
  let processingAI = false;
  let activeAIJob = null;
  const pendingAI = [];
  const knownAIJobs = new Set();
  let pendingTrigger = false;
  let transcriptText = '';
  const transcriptContext = [];
  const chatMemory=new ChatMemory();
  let chatEpoch=1;
  function chatScope(){return [config.serverUrl,config.username,resolveRoleModel('chat',config)].join('\0');}
  let availableModels=[];
  function aiVisibleLines(){return normalizeGlassesMode(config.glassesMode)==='translate'?5:6;}
  let nativeTranslateLayout=false,nativeHeaderLayoutKey='';
  const AI_SCROLL_START_MS = 5000;
  const AI_SCROLL_LINE_MS = 2500;
  // Inner width: 576 - 2 × (2px padding + 1px border).
  const AI_BOX_WIDTH_PX = 570;
  const TRANSCRIPT_VISIBLE_LINES = 3;
  const TRANSCRIPT_WIDTH_PX = 576;
  let aiPhaseTitle = '';
  let aiPhaseDetail = '';
  let aiReasoningPreview = '';
  let aiReasoningStartMs = 0;
  let aiAnimStep = 0;
  let aiAnimTimer = null;
  let aiScrollOffset = 0;
  let aiScrollTimer = null;
  let lastAIHUDContent = '';
  let lastTranscriptHUDContent = '';
  let transcriptStatusMessage = '';
  let autoLangUnsupported = false;
  let previewAbortController = null;
  let clipAbortController = null;
  let aiAbortController = null;
  let aiAbortedByModeSwitch = false;
  const MAX_PENDING_CLIPS = 64;
  const MAX_PENDING_AUDIO_BYTES = 32000 * 120; // Pause after two minutes of unsaved PCM.

  // --- DOM Elements ---
  const els = {
    serverUrl: document.getElementById('server-url'),
    btnTestConnection: document.getElementById('btn-test-connection'),
    authUsername: document.getElementById('auth-username'),
    authPassword: document.getElementById('auth-password'),
    btnLogin: document.getElementById('btn-login'),
    loginStatusLabel: document.getElementById('login-status-label'),
    apiKey: document.getElementById('api-key'),
    modelSelect: document.getElementById('model-select'),
    roleModels:{chat:document.getElementById('model-ai-chat'),hint:document.getElementById('model-ai-hint'),translation:document.getElementById('model-translation'),translationReply:document.getElementById('model-translation-reply')},
    btnRefreshModels: document.getElementById('btn-refresh-models'),
    connectionMode: document.getElementById('connection-mode'),
    optConciseMode: document.getElementById('opt-concise-mode'),
    optEnableTts: document.getElementById('opt-enable-tts'),
    btnSaveSettings: document.getElementById('btn-save-settings'),
    connectionStatus: document.getElementById('connection-status'),
    connectionText: document.getElementById('connection-text'),
    hudScreen: document.getElementById('hud-screen'),
    hudStatus: document.getElementById('hud-status-text'),
    hudNotice: document.getElementById('hud-notice'),
    hudMicIcon:document.getElementById('hud-mic-icon'),hudTriggerIcon:document.getElementById('hud-trigger-icon'),
    hudModeLabel:document.getElementById('hud-mode-label'),hudTriggerKeyword:document.getElementById('hud-trigger-keyword'),hudHeaderMessage:document.getElementById('hud-header-message'),
    hudPageIndicator: document.getElementById('hud-page-indicator'),
    glassesMode: document.getElementById('glasses-mode'),
    translateExcludeSelf: document.getElementById('translate-exclude-self'),
    chromeTranslation: document.getElementById('chrome-translation'),
    translationEngine:document.getElementById('translation-engine'),
    btnPrepareChrome: document.getElementById('prepare-chrome-translation'),
    chromeTranslationStatus: document.getElementById('chrome-translation-status'),
    translateSourceLang: document.getElementById('translate-source-lang'),
    translateTargetLang: document.getElementById('translate-target-lang'),
    translateLangRow: document.getElementById('translate-lang-row'),
    modeBtnAi: document.getElementById('mode-btn-ai'),
    modeBtnTranslate: document.getElementById('mode-btn-translate'),
    modeDescription: document.getElementById('mode-description'),
    voiceMode: document.getElementById('voice-mode'),
    hintSensitivity: document.getElementById('hint-sensitivity'),
    hintWebSearch: document.getElementById('hint-web-search'),
    hintStatus: document.getElementById('hint-status'),
    chatForm: document.getElementById('text-chat-form'),
    chatInput: document.getElementById('chat-input'),
    chatQuestion: document.getElementById('chat-question'),
    chatOutput: document.getElementById('chat-output'),
    triggerWord: document.getElementById('trigger-word'),
    speechFilter: document.getElementById('speech-filter'),
    sttEngine: document.getElementById('stt-engine'),
    audioInputSource:document.getElementById('audio-input-source'),
    glassesInputSettings:document.getElementById('glasses-input-settings'),glassesSensitivity:document.getElementById('glasses-sensitivity'),glassesGain:document.getElementById('glasses-gain'),glassesInputMeter:document.getElementById('glasses-input-meter'),glassesInputLevel:document.getElementById('glasses-input-level'),
    sttFallback: document.getElementById('stt-fallback'),
    sttEngineStatus: document.getElementById('stt-engine-status'),
    btnPrepareStt: document.getElementById('prepare-chrome-stt'),
    speakerScope: document.getElementById('speaker-scope'),
    transcriptModal: document.getElementById('transcript-modal'),
    btnCloseTranscripts: document.getElementById('btn-close-transcripts'),
    transcriptDate: document.getElementById('transcript-date'),
    transcriptLog: document.getElementById('transcript-log'),
    btnTranscripts: document.getElementById('btn-transcripts'),
    btnExportTranscripts: document.getElementById('btn-export-transcripts'),
    voiceBtn: document.getElementById('voice-btn'),
    actionHint: document.getElementById('action-hint')
  };

  const hintGate=new HintGate({
    decide:async(job,signal)=>{
      const res=await gatewayFetch(`${config.serverUrl}/api/glasses/hint-decision`,{
        method:'POST',headers:getAuthHeaders({'Content-Type':'application/json'}),signal,
        body:JSON.stringify({text:job.text,context:job.context,sensitivity:config.hintSensitivity,shown:hintGate.shownFocus(),shownInfo:hintGate.diversityContext()})
      });
      if(!res.ok)throw new Error(`HTTP ${res.status}${res.status===404?' · 게이트웨이 업데이트 필요':''}`);
      return res.json();
    },
    onHint:job=>{
      if(!isRecording || config.glassesMode!=='ai' || pendingTrigger || (activeAIJob && !activeAIJob.hint) || pendingAI.some(job=>!job.hint))return false;
      enqueueAI(job.id,job.text,job.context,'ai',config.translateTargetLang,{...job,hint:true});
      return true;
    },
    onStatus:text=>{els.hintStatus.textContent=text;}
  });

  function localEngine(){return config.translationEngine || (config.chromeTranslation===false?'llm':'chrome');}
  function localLabel(){return localEngine()==='apple'?'Apple':'Chrome';}
  function engineSpec(kind,overrides={}){
    return {kind,server:config.serverUrl,headers:getAuthHeaders({'Content-Type':'application/json'}),
      engine:kind==='stt'?config.sttEngine:localEngine(),
      language:normalizeGlassesMode(config.glassesMode)==='translate'?resolveTranslateSourceLang({sourceLang:config.translateSourceLang,targetLang:config.translateTargetLang}):'ko',
      source:resolveTranslateSourceLang({sourceLang:config.translateSourceLang,targetLang:config.translateTargetLang}),target:config.translateTargetLang || 'ko',...overrides};
  }
  function canonicalSpec(spec){
    if(spec.kind==='stt'){const {kind,server,headers,engine,language}=spec;return {kind,server,headers,engine,language:engine==='whisper'?'ko':language};}
    const {kind,server,headers,engine,source,target}=spec;return {kind,server,headers,engine,source,target};
  }
  async function runEnginePreparation(spec,signal){
    const {kind,server,headers,engine}=spec;
    if(kind==='stt' && engine==='whisper'){
      const response=await gatewayFetch(`${server}/api/stt/status`,{headers,signal});
      const status=await response.json();
      if(!response.ok || !status.active)throw Error(status.message || '서버에서 STT 모델과 런타임을 설치하세요.');
      const warmup=await gatewayFetch(`${server}/api/stt/warmup`,{method:'POST',headers,signal});
      if(!warmup.ok)throw Error(`음성 엔진 준비 실패: HTTP ${warmup.status}`);
      return;
    }
    const path=kind==='stt'?`/api/stt/${engine==='apple'?'apple':'chrome'}/prepare`:`/api/translate/${engine}/prepare`;
    const body=kind==='stt'?{engine,language:spec.language}:{source:spec.source,target:spec.target};
    const response=await gatewayFetch(server+path,{method:'POST',headers,signal,body:JSON.stringify(body)});
    const result=await response.json();
    if(!response.ok || result.status!=='ready')throw Error(result.error || `HTTP ${response.status}`);
  }
  function preparationStatus(kind,state,spec,error){
    const stt=kind==='stt',el=stt?els.sttEngineStatus:els.chromeTranslationStatus;
    const label=spec.engine==='apple'?'Apple':spec.engine==='whisper'?'Whisper':'Chrome';
    const detail=stt?spec.language:`${spec.source} → ${spec.target}`;
    el.textContent=state==='preparing'?`${label} ${stt?'STT':'번역'} 자동 준비 중 · ${detail}${spec.engine==='apple'?' · 최초 준비 시 Mac의 언어팩 창을 확인해 주세요':''}`:
      state==='ready'?`${label} ${stt?'STT':'로컬 번역'} 준비 완료 · ${detail}`:
      `${stt?'STT':'번역'} 준비 실패 · ${error?.name==='AbortError'?'준비 시간 초과':error?.message}${stt?'':' · 기존 LLM 번역 사용'}`;
    (stt?els.btnPrepareStt:els.btnPrepareChrome).disabled=state==='preparing';
  }
  const sttPreparation=new EnginePreparation(runEnginePreparation,(...args)=>preparationStatus('stt',...args));
  const translationPreparation=new EnginePreparation(runEnginePreparation,(...args)=>preparationStatus('translation',...args));
  function prepareSelectedStt(signal,force=false,engine=config.sttEngine){return waitForPreparation(sttPreparation.ensure(canonicalSpec(engineSpec('stt',{engine})),{force}),signal);}
  function prepareSelectedTranslation(signal,force=false){
    if(localEngine()==='llm'){translationPreparation.clear();els.btnPrepareChrome.disabled=false;els.chromeTranslationStatus.textContent='LLM 번역은 별도 언어팩 준비가 필요 없습니다';return Promise.resolve();}
    return waitForPreparation(translationPreparation.ensure(canonicalSpec(engineSpec('translation')),{force}),signal);
  }
  function autoPrepareEngines(){
    if(!config.serverUrl)return;
    prepareSelectedStt().catch(()=>{});
    prepareSelectedTranslation().catch(()=>{});
  }
  async function translateLocal(request,signal,engine=localEngine()){
    const spec=canonicalSpec(engineSpec('translation',{engine,source:request.source,target:request.target}));
    try{
      await waitForPreparation(translationPreparation.ensure(spec),signal);
      let result=await localTranslate(gatewayFetch,spec.server,spec.headers,request,signal,8000,engine);
      if(!result.ok){
        // Server restart or removed language assets invalidate client readiness.
        await waitForPreparation(translationPreparation.ensure(spec,{force:true}),signal);
        result=await localTranslate(gatewayFetch,spec.server,spec.headers,request,signal,8000,engine);
      }
      return result;
    }catch(error){if(signal?.aborted)throw error;return {ok:false,reason:error.message};}
  }
  const translationHistory=new TranslationHistory();
  const translationSentence=new TranslationSentence();
  const stableTranslation=new StableTranslation((job,signal)=>translateLocal({text:job.prompt,source:job.source,target:job.target},signal,job.engine));
  let previewRetryTimer=null;
  const liveTranslations=new LiveTranslationQueue({
    translate:(job,signal)=>config.chromeTranslation===false?Promise.resolve({ok:false,reason:'로컬 번역 비활성'}):stableTranslation.run(job,signal),
    onToken:token=>{
      translationToken=token;
      if(activeAIJob?.translationToken)aiAbortController?.abort();
      pendingAI.splice(0,pendingAI.length,...pendingAI.filter(job=>!job.translationToken));
    },
    onText:(job,text)=>{
      if(normalizeGlassesMode(config.glassesMode)!=='translate')return;
      liveTranslationDraft=job.draft;
      translationFailure='';translationDisplay={...translationDisplay,translation:text};answerText=text;
      translationHistory.update(job.historyId || job.id,text,{final:!job.draft,revision:job.id});
      els.chatOutput.textContent=translationPhoneText(translationDisplay);
      els.chromeTranslationStatus.textContent=job.draft?`${localLabel()} 실시간 번역 · 인식에 따라 수정됩니다`:`${localLabel()} 확정 번역 · 답변 예시 생성 중`;renderLiveHUD();
    },
    onFinal:(job,result)=>{
      liveTranslationDraft=false;
      if(normalizeGlassesMode(config.glassesMode)!=='translate')return;
      enqueueAI(job.id+'-translation-'+job.token,job.prompt,job.context,'translate',job.target,{chromeTranslationText:result.ok?result.text:'',chromeAttempted:true,translationSource:job.source,translationToken:job.token,translationHistoryId:job.historyId || job.id,translationRevisionId:job.id});
    },
    onFailure:(job,reason)=>{
      if(config.chromeTranslation!==false)els.chromeTranslationStatus.textContent=job.draft?`실시간 번역 대기 · ${reason}`:`기존 LLM 번역 사용 · ${reason}`;
    },
    onBusy:busy=>{translating=busy || liveTranslationDraft;renderLiveHUD();}
  });

  const hudIcons=new HudIconStore(),lastHudIconKeys=new Map();
  let hudImagesDisabled=false,lastFallbackHeader='';
  function hudMicrophoneState(){
    const active=isRecording || capturePreparing || micMayBeOpen || pendingMicOpen || micCloseUncertain;
    const source=active?(activeAudioInputSource===AudioInputSource.Phone?'phone':'glasses'):config.audioInputSource;
    const on=isRecording && !capturePreparing && !pendingMicOpen && !micCloseUncertain;
    return {source,on,key:microphoneIcon(source,on)};
  }
  function currentHeaderLayout(translate=normalizeGlassesMode(config.glassesMode)==='translate'){return headerLayout(translate,{trigger:triggerLabels()[0] || '',source:config.translateSourceLang || 'auto',target:config.translateTargetLang || 'ko'});}
  function nativeHUDNotice(){
    const {source,on}=hudMicrophoneState(),translate=normalizeGlassesMode(config.glassesMode)==='translate';
    const fallback=hudImagesDisabled?`[${source==='phone'?'폰':'안경'} ${on?'녹음':'꺼짐'}] `:'';
    return hudNoticeText(fallback+currentHUDHeader(true),currentHeaderLayout(translate).width);
  }
  function queueHudIcons(force=false){
    const mic=hudMicrophoneState(),translate=normalizeGlassesMode(config.glassesMode)==='translate';
    if(els.hudMicIcon){els.hudMicIcon.src=hudIconPath(mic.key);els.hudMicIcon.alt=`${mic.source==='phone'?'휴대폰':'안경'} 마이크 ${mic.on?'켜짐':'꺼짐'}`;}
    if(els.hudTriggerIcon)els.hudTriggerIcon.hidden=translate;
    const layout=currentHeaderLayout(translate);
    els.hudModeLabel.textContent=layout.prefix.content;
    els.hudTriggerKeyword.textContent=layout.wake?.content || '';els.hudTriggerKeyword.hidden=translate;
    els.hudHeaderMessage.textContent=currentHUDHeader(true);
    if(!displayReady)return;
    if(typeof bridge.updateImageRawData!=='function')hudImagesDisabled=true;
    if(hudImagesDisabled){
      const content=nativeHUDNotice();
      if(content!==lastFallbackHeader){lastFallbackHeader=content;pendingHUD.set(3,{containerID:3,containerName:'notice',content});flushHUD();}
      return;
    }
    const icons=[[5,'mic-state',mic.key],...(!translate?[[6,'ai-trigger','ai-trigger']]:[])];
    if(translate){pendingHUD.delete(6);lastHudIconKeys.delete(6);}
    for(const [id,name,key] of icons)if(force || lastHudIconKeys.get(id)!==key){
      lastHudIconKeys.set(id,key);pendingHUD.set(id,{containerID:id,containerName:name,icon:key});
    }
    if(pendingHUD.size)flushHUD();
  }

  // --- Lifecycle & Initialization ---
  async function init() {
    loadSettings();
    bindEvents();
    initEvenRealitiesBridge();
    if (config.serverUrl) {
      checkLoginStatus();
    }
  }

  // --- Settings Persistence ---
  function loadSettings(nativeValue) {
    try {
      const saved = nativeValue === undefined ? localStorage.getItem(STORAGE_KEY) : nativeValue;
      if (saved) {
        const incoming=JSON.parse(saved);
        config = { ...config, ...incoming };
        config.roleModels=normalizeRoleModels(incoming.roleModels);
        config.audioInputSource=incoming.audioInputSource==='phone'?'phone':'glasses';
        config.glassesInput=normalizeGlassesInput(incoming.glassesInput);
        if(!Object.prototype.hasOwnProperty.call(incoming,'translationEngine'))config.translationEngine=incoming.chromeTranslation===false?'llm':'chrome';
      }
      if (/^https?:\/\/(localhost|127\.0\.0\.1)(:|\/|$)/.test(config.serverUrl) &&
          !['localhost', '127.0.0.1'].includes(window.location.hostname)) {
        config.serverUrl = DKST_DEFAULT_SERVER_URL || window.location.origin;
      }
      if (config.connectionMode === 'websocket') config.connectionMode = 'sse';
    } catch (e) {
      console.warn('Failed to load settings:', e);
    }

    if (els.serverUrl) els.serverUrl.value = config.serverUrl || '';
    if (els.authUsername) els.authUsername.value = config.username || '';
    if (els.apiKey) els.apiKey.value = config.apiKey || '';
    if (els.connectionMode) els.connectionMode.value = config.connectionMode || 'sse';
    config.aiHintMode=true;config.voiceMode='continuous';
    els.hintSensitivity.value=config.hintSensitivity || 'balanced';
    if(els.hintWebSearch)els.hintWebSearch.checked=config.hintWebSearch!==false;
    if (els.optConciseMode) els.optConciseMode.checked = config.conciseMode !== false;
    if (els.optEnableTts) els.optEnableTts.checked = config.enableTTS !== false;
    config.glassesMode = normalizeGlassesMode(config.glassesMode);
    config.translateSourceLang = config.translateSourceLang || 'auto';
    config.translateTargetLang = config.translateTargetLang || 'ko';
    if (els.glassesMode) els.glassesMode.value = config.glassesMode;
    if (els.translateSourceLang) els.translateSourceLang.value = config.translateSourceLang;
    if (els.translateTargetLang) els.translateTargetLang.value = config.translateTargetLang;
    els.voiceMode.value = config.voiceMode;
    els.triggerWord.value = config.triggerWord;
    els.speechFilter.value = config.speechFilter;
    config.roleModels=normalizeRoleModels(config.roleModels);renderRoleModelSelectors();
    config.audioInputSource=config.audioInputSource==='phone'?'phone':'glasses';
    els.audioInputSource.value=config.audioInputSource;
    config.glassesInput=normalizeGlassesInput(config.glassesInput);syncGlassesInputUI();
    els.sttEngine.value=config.sttEngine || 'whisper';els.sttFallback.checked=config.sttFallback!==false;
    els.speakerScope.value = config.speakerScope;
    if(els.translateExcludeSelf)els.translateExcludeSelf.checked=config.translateExcludeSelf!==false;
    config.translationEngine=['llm','chrome','apple'].includes(config.translationEngine)?config.translationEngine:(config.chromeTranslation===false?'llm':'chrome');
    config.chromeTranslation=config.translationEngine!=='llm';
    if(els.translationEngine)els.translationEngine.value=config.translationEngine;
    if(els.chromeTranslation)els.chromeTranslation.checked=config.chromeTranslation;
    const now = new Date();
    els.transcriptDate.value = `${now.getFullYear()}-${String(now.getMonth()+1).padStart(2,'0')}-${String(now.getDate()).padStart(2,'0')}`;
    syncModeUI();
  }

  function isContinuousCaptureMode(mode = config.glassesMode) {
    const normalized = normalizeGlassesMode(mode);
    if (normalized === 'ai' || normalized === 'translate') return true;
    return config.voiceMode === 'continuous';
  }

  function abortPendingPreview() {
    clearTimeout(previewRetryTimer);previewRetryTimer=null;
    pendingPreview = null;
    if (previewAbortController) {
      previewAbortController.abort();
      previewAbortController = null;
    }
  }

  function stopChromeSegment(id){
    if(activeSttEngine==='whisper' || !id)return;
    gatewayFetch(`${config.serverUrl}/api/stt/${activeSttEngine==='apple'?'apple':'chrome'}/stop`,{method:'POST',headers:getAuthHeaders({'Content-Type':'application/json'}),body:JSON.stringify({sessionId,segmentId:id})}).catch(()=>{});
  }
  function resetQueuesForModeSwitch(nextMode, preserveAudio=false, preserveDisplay=false) {
    chatEpoch++;translationFailure='';translationSentence.reset();if(!preserveDisplay)translationHistory.reset();stableTranslation.reset();
    stopChromeSegment(activeUtterance?.id);
    liveTranslationDraft=false;liveTranslations.reset();
    if(!preserveDisplay)translationDisplay={translation:'',positive:'',negative:''};
    pendingTrigger=false;
    hintGate.reset();
    abortPendingPreview();
    activeUtterance = null;
    draftUtteranceId = '';
    captionStable = '';
    captionDraft = '';
    if(!preserveAudio)pendingClips.length = 0;
    if (clipAbortController && !preserveAudio) {
      clipAbortController.abort();
      clipAbortController = null;
    }
    pendingAI.length = 0;
    if (aiAbortController) {
      aiAbortedByModeSwitch = true;
      aiAbortController.abort();
      aiAbortController = null;
    }
    clearTimeout(answerTimer);
    answerHoldUntil = 0;
    stopAIAutoScroll();
    stopThinkingAnimation();
    if(!preserveDisplay){answerText = '';aiScrollOffset = 0;}
    aiPhaseTitle = '';
    aiPhaseDetail = '';
    aiReasoningPreview = '';
    if (isRecording) {
      audioBuffer = [];
      recordedBytes = 0;
      recordingStarted = Date.now();
      segmenter = new SpeechSegmenter(inputSegmentOptions(activeSttEngine,nextMode,activeAudioInputSource,activeGlassesInput));
      clearTimeout(manualTimer);
      manualTimer = null;
      if (!isContinuousCaptureMode(nextMode)) {
        manualTimer = setTimeout(() => stopVoiceRecording().catch(console.warn), 25000);
      }
    }
  }

  function syncModeUI() {
    const mode = normalizeGlassesMode(config.glassesMode);
    const sourceLang = config.translateSourceLang || 'auto';
    const targetLang = config.translateTargetLang || 'ko';
    if (els.glassesMode && els.glassesMode.value !== mode) els.glassesMode.value = mode;
    if (els.translateSourceLang && els.translateSourceLang.value !== sourceLang) els.translateSourceLang.value = sourceLang;
    if (els.translateTargetLang && els.translateTargetLang.value !== targetLang) els.translateTargetLang.value = targetLang;
    if (els.translateLangRow) els.translateLangRow.hidden = mode !== 'translate';
    for (const [btn, key] of [
      [els.modeBtnAi, 'ai'],
      [els.modeBtnTranslate, 'translate'],
    ]) {
      if (!btn) continue;
      const active = mode === key;
      btn.classList?.toggle?.('active', active);
      btn.setAttribute?.('aria-selected', active ? 'true' : 'false');
    }
    if (els.modeDescription) {
      if (mode === 'ai') {
        els.modeDescription.textContent = 'AI 모드 · 평소에는 정보 힌트, 호출어 뒤 질문은 이전 AI 채팅을 참고해 답변합니다';
      } else if (mode === 'translate') {
        els.modeDescription.textContent = `번역 모드 (→${targetLanguageLabel(targetLang, true)}) · 실시간 번역 + 긍정/부정 답변 표시`;
      }
    }
    renderLiveHUD();
  }

  function setGlassesMode(nextMode, persist = false) {
    if(persist)return applyLiveSettings({glassesMode:normalizeGlassesMode(nextMode)});
    const prevMode = normalizeGlassesMode(config.glassesMode);
    const mode = normalizeGlassesMode(nextMode);
    const modeChanged = prevMode !== mode;
    config.glassesMode = mode;
    pendingTrigger = false;
    captureFailure = '';
    if (modeChanged) {
      resetQueuesForModeSwitch(mode);
    }
    syncModeUI();
    setHUDNotice();
    if (persist) {
      autoPrepareEngines();
      persistSettings().catch(console.warn);
    }
  }

  async function saveSettings(showMessage = true) {
    if(inputSwitchPromise)await inputSwitchPromise;
    settingsEdited = true;
    config.serverUrl = (els.serverUrl.value || '').trim().replace(/\/+$/, '');
    config.username = (els.authUsername.value || '').trim();
    config.apiKey = (els.apiKey.value || '').trim();
    config.model = els.modelSelect.value || '';
    config.roleModels=Object.fromEntries(ROLE_MODEL_KEYS.map(role=>[role,els.roleModels[role].value || '']));
    config.connectionMode = els.connectionMode.value;
    config.conciseMode = els.optConciseMode.checked;
    config.aiHintMode=true;
    config.hintSensitivity=els.hintSensitivity.value || 'balanced';
    config.hintWebSearch=els.hintWebSearch?.checked!==false;
    hintGate.reset();
    config.enableTTS = els.optEnableTts.checked;
    if (els.glassesMode?.value) config.glassesMode = normalizeGlassesMode(els.glassesMode.value);
    if (els.translateSourceLang?.value) config.translateSourceLang = els.translateSourceLang.value;
    if (els.translateTargetLang?.value) config.translateTargetLang = els.translateTargetLang.value;
    config.voiceMode = 'continuous';
    config.triggerWord = els.triggerWord.value.trim();
    config.speechFilter = els.speechFilter.value === 'standard' ? 'standard' : 'strict';
    config.audioInputSource=els.audioInputSource.value==='phone'?'phone':'glasses';
    config.glassesInput=readGlassesInputControls();
    config.sttEngine=['chrome-online','apple'].includes(els.sttEngine.value)?els.sttEngine.value:'whisper';config.sttFallback=els.sttFallback.checked!==false;
    config.speakerScope = els.speakerScope.value === 'self' ? 'self' : 'conversation';
    config.translateExcludeSelf=els.translateExcludeSelf?.checked!==false;
    config.chromeTranslation=els.chromeTranslation?.checked!==false;
    syncModeUI();

    try {
      await persistSettings();
      autoPrepareEngines();
      if (showMessage !== false) alert('설정이 저장되었습니다.');
    } catch (err) {
      if (showMessage !== false) alert(`설정 저장 실패: ${err.message}`);
      else throw err;
    }
  }

  async function storageCall(promise) {
    let timer;
    try {
      return await Promise.race([promise, new Promise((_, reject) => {
        timer = setTimeout(() => reject(new Error('Even 앱 저장소 응답 시간 초과')), 8000);
      })]);
    } finally { clearTimeout(timer); }
  }

  async function persistSettings() {
    delete config.password;
    const value = JSON.stringify(config);
    try { localStorage.setItem(STORAGE_KEY, value); } catch (_) {}
    if (bridge && typeof bridge.setLocalStorage === 'function') {
      const success = await storageCall(bridge.setLocalStorage(STORAGE_KEY, value));
      if (!success) throw new Error('Even 앱 저장소가 저장을 거절했습니다.');
      pendingNativeSave = false;
    } else {
      pendingNativeSave = true;
    }
  }

  async function restoreNativeSettings() {
    if (typeof bridge.getLocalStorage !== 'function') return;
    try {
      if (pendingNativeSave) { await persistSettings(); await checkLoginStatus(); return; }
      const saved = await storageCall(bridge.getLocalStorage(STORAGE_KEY));
      if (saved && !settingsEdited) loadSettings(saved);
      else if (!saved && !settingsEdited) await persistSettings();
      await checkLoginStatus();
    } catch (err) {
      console.warn('Native settings restore failed:', err);
      setVoiceHint('설정 복원 실패 · 주소와 로그인 상태를 확인하세요');
    }
  }

  // --- Even Realities SDK Bridge ---
  function hudPage(translate,layout=currentHeaderLayout(translate)){return {
        containerTotalNum: translate?6:8,
        imageObject:[{containerID:5,containerName:'mic-state',xPosition:layout.micIconX,yPosition:1,width:24,height:24},...(!translate?[{containerID:6,containerName:'ai-trigger',xPosition:layout.wakeIconX,yPosition:1,width:24,height:24}]:[])],
        textObject: [{
          xPosition: 0, yPosition: 27, width: 576, height: translate?141:168,
          borderWidth: 1, borderColor: 4, borderRadius: 4, paddingLength: 2,
          containerID: 1, containerName: 'gateway',
          content: lastAIHUDContent || '',
          isEventCapture: 1,
        }, {
          xPosition: 400, yPosition: 0, width: 176, height: 27,
          paddingLength: 0, borderWidth: 0,
          containerID: 2, containerName: 'status', content: rightAlignedStatusText(glassesBattery), isEventCapture: 0,
        }, {
          xPosition: layout.x, yPosition: 0, width: layout.width, height: 27,
          paddingLength: 0, borderWidth: 0,
          containerID: 3, containerName: 'notice', content: nativeHUDNotice(), isEventCapture: 0,
        }, {
          xPosition: 0, yPosition: translate?168:195, width: 576, height: translate?120:93,
          paddingLength: 0, borderWidth: 0,
          containerID: 4, containerName: 'transcript',
          content: lastTranscriptHUDContent || '',
          isEventCapture: 0,
        },{containerID:7,containerName:'mode-label',xPosition:layout.prefix.x,yPosition:0,width:layout.prefix.width,height:27,paddingLength:0,borderWidth:0,content:layout.prefix.content,isEventCapture:0},
        ...(!translate?[{containerID:8,containerName:'wake-word',xPosition:layout.wake.x,yPosition:0,width:layout.wake.width,height:27,paddingLength:0,borderWidth:0,content:layout.wake.content,isEventCapture:0}]:[])],
        menuObject: {
          menuItems: [
            { itemID: 1, itemName: 'AI 모드' },
            { itemID: 2, itemName: '번역 모드' },
            { itemID: 3, itemName: '입력 소스 전환' },
            ...settingsMenuItems(config.audioInputSource==='glasses'),
          ],
        },
      };}

  function openSettingsPicker(id){
    const menu=SETTINGS_MENUS[id];if(!menu)return;
    if(id===9 && config.audioInputSource!=='glasses'){setVoiceHint('안경 입력 감지 민감도는 안경 입력에서만 적용됩니다.');return;}
    // Keep engines discoverable even before the asynchronous capability check.
    const options=menu.options;
    settingsPicker={...menu,options,value:id===9?config.glassesInput.sensitivity:config[menu.key],index:0,pageKey:String(id)};
    flushHUD();
  }
  function closeSettingsPicker(){settingsPicker=null;flushHUD();}
  function selectSettingsPicker(index){
    const picker=settingsPicker;
    if(!picker || !Number.isInteger(index) || !picker.options[index])return;
    const value=picker.options[index][0];closeSettingsPicker();
    applyLiveSettings({[picker.key]:picker.key==='glassesInput'?{...config.glassesInput,sensitivity:value}:value});
  }

  async function initEvenRealitiesBridge() {
    try {
      updateConnectionState(false, 'Even 앱 연결 대기 · 브라우저 미리보기');
      bridge = await waitForEvenAppBridge();
      restoreNativeSettings();
      nativeTranslateLayout=normalizeGlassesMode(config.glassesMode)==='translate';
      const initialHeaderLayout=currentHeaderLayout(nativeTranslateLayout);
      const result = await bridge.createStartUpPageContainer(hudPage(nativeTranslateLayout,initialHeaderLayout));
      if (result !== 0) throw new Error(`안경 화면 생성 실패: ${result}`);
      displayReady = true;nativeHeaderLayoutKey=initialHeaderLayout.key;nativeMenuSource=config.audioInputSource;
      queueHudIcons(true);
      updateConnectionState(true, 'G2 화면 연결됨');
      setupBridgeEvents();
      startHUDStatus();
    } catch (err) {
      console.warn('Even Realities Bridge init notice:', err);
      updateConnectionState(false, `G2 연결 실패: ${err.message}`);
    }
  }

  function setupBridgeEvents() {
    if (!bridge) return;

    // Listen to hardware events (tap, gesture, menu, mic audio)
    if (typeof bridge.onEvenHubEvent === 'function') {
      bridge.onEvenHubEvent((event) => {
        if (!event) return;

        // Handle Audio Events
        if (event.audioEvent && isRecording) {
          const source=event.audioEvent.source;
          if([AudioInputSource.Glasses,AudioInputSource.Phone].includes(source) && source!==activeAudioInputSource)return;
          const pcm = event.audioEvent.audioPcm;
          if (pcm && pcm.length > 0) {
            receivePCM(new Uint8Array(pcm), event.audioEvent.speakerRole);
          }
        }
        if (event.audioEvent) return;

        // Handle Glasses Contextual Menu (전사 / AI / 번역 모드 선택)
        if (event.menuItemClickEvent) {
          if(event.menuItemClickEvent.itemID===90){closeSettingsPicker();return;}
          if(SETTINGS_MENUS[event.menuItemClickEvent.itemID]){openSettingsPicker(event.menuItemClickEvent.itemID);return;}
          if(event.menuItemClickEvent.itemID===3){switchAudioInputSource();return;}
          const selectedMode = glassesModeMenuIdToMode(event.menuItemClickEvent.itemID);
          if (selectedMode) {
            setGlassesMode(selectedMode, true);
          }
          return;
        }

        if(settingsPicker){
          const list=event.listEvent;
          if(list){
            if(list.containerID!==30 && list.containerName!=='settings-list')return;
            if(Number.isInteger(list.currentSelectItemIndex))settingsPicker.index=list.currentSelectItemIndex;
            if((list.eventType ?? 0)===0)selectSettingsPicker(settingsPicker.index);
          }else{
            const type=event.sysEvent?.eventType ?? event.textEvent?.eventType ?? null;
            if(type===0)selectSettingsPicker(settingsPicker.index);
            else if(type===3 || type===9)closeSettingsPicker();
          }
          return;
        }
        // Handle Touch / Gesture Event
        const sysType = event.sysEvent
          ? (event.sysEvent.eventType ?? OsEventTypeList.CLICK_EVENT) : null;
        const textType = event.textEvent ? (event.textEvent.eventType ?? OsEventTypeList.CLICK_EVENT) : null;
        const evType = sysType ?? textType;

        if (evType === OsEventTypeList.DOUBLE_CLICK_EVENT) {
          stopVoiceRecording().catch(console.warn).finally(() => bridge.shutDownPageContainer(1).catch(console.warn));
        } else if (evType === (OsEventTypeList.LONG_PRESS_EVENT ?? 9)) {
          setGlassesMode(nextGlassesMode(config.glassesMode), true);
        } else if (evType === (OsEventTypeList.SCROLL_TOP_EVENT ?? 1)) {
          if (answerText) {
            stopAIAutoScroll();
            if(normalizeGlassesMode(config.glassesMode)==='translate')translationHistory.scroll(-1);
            else aiScrollOffset = Math.max(0, aiScrollOffset - 1);
            renderLiveHUD();
          }
        } else if (evType === (OsEventTypeList.SCROLL_BOTTOM_EVENT ?? 2)) {
          if (answerText) {
            stopAIAutoScroll();
            const win = sliceHUDWindow(answerText, aiScrollOffset, aiVisibleLines(), AI_BOX_WIDTH_PX);
            if(normalizeGlassesMode(config.glassesMode)==='translate')translationHistory.scroll(1);
            else aiScrollOffset = Math.min(win.maxOffset, aiScrollOffset + 1);
            renderLiveHUD();
          }
        } else if (evType === OsEventTypeList.CLICK_EVENT) {
          toggleVoiceRecording();
        }
      });
    }
  }

  function updateConnectionState(connected, label) {
    if (!els.connectionStatus || !els.connectionText) return;
    els.connectionStatus.classList.toggle('disconnected', !connected);
    els.connectionText.textContent = label || (connected ? '연결됨' : '연결 안 됨');
  }

  // --- Display / HUD Rendering (576x288 Micro-LED) ---
  function formatLiveAIStatus() {
    const headline = withAnimatedDots(aiPhaseTitle || '생각 중', aiAnimStep);
    const lines = [headline];
    if (aiPhaseDetail) lines.push(aiPhaseDetail);
    if (aiReasoningPreview) {
      const compact = aiReasoningPreview.replace(/\s+/g, ' ').trim().slice(-42);
      if (compact) lines.push(`💭 ${compact}`);
    }
    return lines.join('\n');
  }

  function startThinkingAnimation() {
    stopThinkingAnimation();
    aiAnimStep = 0;
    function tick() {
      if (!isThinking && !processingAI) {
        aiAnimTimer = null;
        return;
      }
      aiAnimStep = (aiAnimStep + 1) % 3;
      renderLiveHUD();
      aiAnimTimer = setTimeout(tick, 450);
    }
    aiAnimTimer = setTimeout(tick, 450);
  }

  function stopThinkingAnimation() {
    if (aiAnimTimer) {
      clearTimeout(aiAnimTimer);
      aiAnimTimer = null;
    }
  }

  function stopAIAutoScroll() {
    if (aiScrollTimer) {
      clearTimeout(aiScrollTimer);
      aiScrollTimer = null;
    }
  }

  function startAIAutoScroll(maxOffset) {
    stopAIAutoScroll();
    aiScrollOffset = 0;
    if (maxOffset <= 0) return;
    function stepScroll() {
      if (processingAI || isThinking || !answerText) {
        aiScrollTimer = null;
        return;
      }
      const win = sliceHUDWindow(answerText, aiScrollOffset, aiVisibleLines(), AI_BOX_WIDTH_PX);
      if (aiScrollOffset < win.maxOffset) {
        aiScrollOffset += 1;
        renderLiveHUD();
        aiScrollTimer = setTimeout(stepScroll, AI_SCROLL_LINE_MS);
      } else {
        aiScrollTimer = null;
      }
    }
    aiScrollTimer = setTimeout(stepScroll, AI_SCROLL_START_MS);
  }

  function buildTranscriptSectionText() {
    if (captureFailure) return captureFailure;
    const draft = captionStable + captionDraft;
    const transcriptLines = [
      captionFinal,
      draft ? draft + (captionDraft ? ' …' : '') : ''
    ].filter(Boolean);
    if (transcriptLines.length) {
      return transcriptLines.join('\n');
    }
    if (transcriptStatusMessage) return transcriptStatusMessage;
    return isRecording
      ? (config.voiceMode === 'continuous' ? '듣는 중... 말하는 동안 자막이 갱신됩니다.' : '듣는 중... 다시 탭하면 전송합니다.')
      : '녹음 정지';
  }

  function renderHUDContainers(aiFullText, aiWindowText, aiMetaLabel, transcriptFullText, pageInfo = 'P 1/1', scrollRatio = 0) {
    const badgeLabel = glassesModeBadgeText(config.glassesMode, config.translateTargetLang);
    const translateMode=normalizeGlassesMode(config.glassesMode)==='translate';
    const bottomPhoneText=translateMode?translationReplyText(translationDisplay):transcriptFullText;
    if (els.hudScreen) {
      const aiCursor = (isThinking || processingAI) && !(translateMode && config.chromeTranslation!==false) ? '<span class="hud-cursor"></span>' : '';
      const txCursor = (!translateMode && !isThinking && !processingAI && isRecording) ? '<span class="hud-cursor"></span>' : '';
      els.hudScreen.innerHTML =
        `<div class="hud-ai-box">` +
          `<div class="hud-ai-header">` +
            `<span id="hud-ai-badge" class="hud-ai-badge">${escapeHtml(badgeLabel)}</span>` +
            `<span id="hud-ai-meta" class="hud-ai-meta">${escapeHtml(translateMode && config.chromeTranslation!==false?'':aiMetaLabel)}</span>` +
          `</div>` +
          `<div id="hud-ai-body" class="hud-ai-body">${escapeHtml(aiFullText)}${aiCursor}</div>` +
        `</div>` +
        `<div class="hud-transcript-box${translateMode?' hud-translation-replies':''}">` +
          `<div class="hud-transcript-label">${translateMode?'예상 답변':'전사'}</div>` +
          `<div id="hud-transcript-body" class="hud-transcript-body">${escapeHtml(bottomPhoneText)}${txCursor}</div>` +
        `</div>`;
      const aiBodyEl = typeof document.getElementById === 'function' ? document.getElementById('hud-ai-body') : null;
      if (aiBodyEl && typeof aiBodyEl.scrollHeight === 'number' && typeof aiBodyEl.clientHeight === 'number') {
        const maxScroll = Math.max(0, aiBodyEl.scrollHeight - aiBodyEl.clientHeight);
        aiBodyEl.scrollTop = Math.round(maxScroll * Math.max(0, Math.min(1, scrollRatio)));
      }
    }
    if (els.hudPageIndicator) {
      els.hudPageIndicator.textContent = pageInfo;
    }

    if (displayReady) {
      const nextAI = String(aiWindowText || '').slice(0, 2000);
      const nextTx = String(transcriptFullText || '').slice(0, 2000);
      if (nextAI !== lastAIHUDContent) {
        lastAIHUDContent = nextAI;
        pendingHUD.set(1, { containerID: 1, containerName: 'gateway', content: nextAI });
      }
      if (nextTx !== lastTranscriptHUDContent) {
        lastTranscriptHUDContent = nextTx;
        pendingHUD.set(4, { containerID: 4, containerName: 'transcript', content: nextTx });
      }
      if (pendingHUD.size || nativeTranslateLayout!==translateMode) flushHUD();
    }
  }

  function updateHUD(text, pageInfo = 'P 1/1') {
    transcriptStatusMessage = String(text || '');
    renderLiveHUD(pageInfo);
  }

  // Serialize and coalesce updates so streamed tokens do not flood Bluetooth.
  async function flushHUD() {
    if (sendingHUD) return;
    sendingHUD = true;
    try {
      while (displayReady) {
        if(settingsPicker){
          const picker=settingsPicker;
          if(nativePickerKey!==picker.pageKey){
            if(!await bridge.rebuildPageContainer(settingsPickerPage(picker)))throw Error('설정 선택 화면 생성 실패');
            nativePickerKey=picker.pageKey;
            continue;
          }
          break; // Retain coalesced HUD updates until the picker closes.
        }
        const translate=normalizeGlassesMode(config.glassesMode)==='translate';
        const header=currentHeaderLayout(translate);
        if(nativePickerKey || nativeMenuSource!==config.audioInputSource || nativeTranslateLayout!==translate || nativeHeaderLayoutKey!==header.key){
          const success=await bridge.rebuildPageContainer(hudPage(translate,header));
          if(!success)throw new Error('안경 화면 배치 변경 실패');
          nativePickerKey='';nativeMenuSource=config.audioInputSource;nativeTranslateLayout=translate;nativeHeaderLayoutKey=header.key;
          lastHudIconKeys.clear();pendingHUD.delete(5);pendingHUD.delete(6);queueHudIcons(true);
          continue;
        }
        if(!pendingHUD.size)break;
        const [id, update] = pendingHUD.entries().next().value;
        pendingHUD.delete(id);
        if(update.icon){
          try{
            const imageData=await hudIcons.load(update.icon);
            const result=await bridge.updateImageRawData({containerID:update.containerID,containerName:update.containerName,imageData});
            if(result!=='success')throw Error(`아이콘 전송 실패: ${result}`);
          }catch(error){
            console.warn('HUD image unavailable; keeping text updates:',error);
            hudImagesDisabled=true;pendingHUD.delete(5);pendingHUD.delete(6);queueHudIcons();
          }
        }else{
          const success = await bridge.textContainerUpgrade(update);
          if (!success) throw new Error('안경 화면 갱신 실패');
        }
        await new Promise(resolve => setTimeout(resolve, 150));
      }
    } catch (err) {
      console.warn('Bridge display push error:', err);
      updateConnectionState(false, err.message);
    } finally {
      sendingHUD = false;
    }
  }

  function renderHUDStatus() {
    const date=new Date();
    const text = hudStatusText(glassesBattery,date);
    els.hudStatus.textContent = text;
    if (displayReady && text !== lastStatus) {
      pendingHUD.set(2, {containerID: 2, containerName: 'status', content: rightAlignedStatusText(glassesBattery,date)});
      flushHUD();
    }
    lastStatus = text;
  }

  function currentTranslationHeader(messageOnly=false){
    return translationHeaderText({messageOnly,imageIcons:true,source:config.translateSourceLang || 'auto',target:config.translateTargetLang,
      running:isRecording || capturePreparing || processingAI || isThinking || translating,
      phase:(isThinking || processingAI)?aiPhaseTitle:'',phaseDetail:(isThinking || processingAI)?aiPhaseDetail:'',translating,
      error:captureFailure || micCloseUncertain?'오류':translationFailure});
  }
  function setHUDNotice(message) {
    queueHudIcons();
    message=currentHUDHeader();
    if(els.hudNotice.textContent===message)return;
    els.hudNotice.textContent = message;
    // The left header is bounded so a long wake word cannot wrap into the body.
    const content = nativeHUDNotice();
    if (displayReady) {
      pendingHUD.set(3, {containerID:3,containerName:'notice',content});
      flushHUD();
    }
  }
  function currentHUDHeader(messageOnly=false){
    const mode=normalizeGlassesMode(config.glassesMode);
    if(mode==='translate')return currentTranslationHeader(messageOnly);
    const error=micCloseUncertain?'마이크 종료 확인 실패':captureFailure?'전사 오류':'';
    const detail=micStopPromise || pendingMicCloses.size?'마이크 정지중':capturePreparing?'녹음 준비중':pendingTrigger?'질문 대기':draining || pendingClips.length?'전사 처리':'';
    return modeHeaderText({mode,messageOnly,imageIcons:true,hint:config.aiHintMode!==false,listening:isRecording,
      running:isRecording || capturePreparing || processingAI || isThinking || draining || pendingClips.length>0,
      phase:(isThinking || processingAI)?aiPhaseTitle:'',phaseDetail:(isThinking || processingAI)?aiPhaseDetail:'',error,detail,trigger:triggerLabels()[0] || ''});
  }
  function triggerLabels() {
    const seen = new Set();
    return config.triggerWord.split(/[,，;\n]/u).map(value=>value.trim()).filter(value=>{
      const key=value.normalize('NFC').replace(/\s/gu,'').toLowerCase();
      if(!key || seen.has(key))return false;
      seen.add(key);return true;
    });
  }
  function transcriptNotice() { setHUDNotice(); }
  function renderLiveHUD(overridePageInfo) {
    queueHudIcons();
    const chromeTranslateMode=normalizeGlassesMode(config.glassesMode)==='translate' && config.chromeTranslation!==false;
    const notice=currentHUDHeader();if(els.hudNotice.textContent!==notice)setHUDNotice(notice);
    const transcriptRaw = buildTranscriptSectionText();
    const transcriptWin = sliceHUDWindow(transcriptRaw, Infinity, TRANSCRIPT_VISIBLE_LINES, TRANSCRIPT_WIDTH_PX);
    const transcriptGlassesText = normalizeGlassesMode(config.glassesMode)==='translate'
      ? translationReplyText(translationDisplay,TRANSCRIPT_WIDTH_PX) : transcriptWin.visibleText || transcriptRaw;


    if(normalizeGlassesMode(config.glassesMode)==='translate' && translationHistory.entries.length){
      const win=translationHistory.window();
      const ratio=win.maxOffset>0?win.offsetLine/win.maxOffset:0;
      renderHUDContainers(win.fullText,win.visibleText,'',transcriptGlassesText,overridePageInfo || `P ${win.page}/${win.totalPages}`,ratio);
      return;
    }

    if (answerText) {
      const win = sliceHUDWindow(answerText, chromeTranslateMode && liveTranslationDraft?Infinity:aiScrollOffset, aiVisibleLines(), AI_BOX_WIDTH_PX);
      const pageInfo = overridePageInfo || `P ${win.page}/${win.totalPages}`;
      const ratio = win.maxOffset > 0 ? win.offsetLine / win.maxOffset : 0;
      const meta = win.maxOffset > 0
        ? `자동 스크롤 ${win.offsetLine + 1}/${win.maxOffset + 1}`
        : '응답 완료';
      renderHUDContainers(answerText, win.visibleText, meta, transcriptGlassesText, pageInfo, ratio);
      return;
    }

    const mode = normalizeGlassesMode(config.glassesMode);
    let idleAI = 'AI 대기 중 · 호출어로 부르거나 텍스트로 질문하세요.';
    if (mode === 'ai') {
      idleAI = 'AI 모드 · 평소에는 힌트, 호출어 뒤 질문에는 AI 채팅으로 답합니다.';
    } else if (mode === 'translate') {
      const langShort = targetLanguageLabel(config.translateTargetLang, true);
      idleAI = `번역 모드 (→${langShort}) · 말하면 번역과 긍정/부정 답변을 표시합니다.`;
    }
    if(chromeTranslateMode)idleAI='';
    renderHUDContainers(idleAI, idleAI, '대기', transcriptGlassesText, overridePageInfo || 'P 1/1', 0);
  }

  function applyGlassesStatus(status) {
    if (!glassesSN || status?.sn !== glassesSN) return;
    if (status.connectType === 'disconnected' || status.connectType === 'none') glassesBattery = null;
    else if (typeof status.batteryLevel === 'number') glassesBattery = status.batteryLevel;
    renderHUDStatus();
  }
  async function refreshGlassesBattery() {
    if (typeof bridge?.getDeviceInfo !== 'function') return;
    try {
      const info = await storageCall(bridge.getDeviceInfo());
      if (!info) { glassesBattery = null; renderHUDStatus(); return; }
      if (!['g1', 'g2'].includes(info.model)) return;
      if (glassesSN !== info.sn) glassesBattery = null;
      glassesSN = info.sn;
      applyGlassesStatus(info.status);
    } catch (err) { console.warn('Glasses battery unavailable:', err.message); }
  }
  function startHUDStatus() {
    renderHUDStatus();
    if (typeof bridge.onDeviceStatusChanged === 'function') {
      unsubscribeDeviceStatus = bridge.onDeviceStatusChanged(status => {
        if (!glassesSN) refreshGlassesBattery();
        else applyGlassesStatus(status);
      });
    }
    refreshGlassesBattery();
    function tickStatus() {
      renderHUDStatus();
      refreshGlassesBattery();
      statusTimer = setTimeout(tickStatus, 60000 - Date.now()%60000);
    }
    statusTimer = setTimeout(tickStatus, 60000 - Date.now()%60000);
    window.addEventListener?.('pagehide', () => {
      clearTimeout(statusTimer);
      unsubscribeDeviceStatus?.();
      displayReady = false;
      pendingHUD.clear();
    }, {once:true});
  }

  // --- Voice capture, continuous VAD and durable transcript pipeline ---
  function newID() { return window.crypto?.randomUUID?.() || `${Date.now()}-${Math.random().toString(36).slice(2)}`; }
  function resetCaptureTimers() {
    clearTimeout(captureTimer); clearTimeout(manualTimer);
    captureTimer = manualTimer = null;
  }
  function setVoiceHint(text) { if (els.actionHint) els.actionHint.textContent = text; }

  async function microphoneCall(promise) {
    let timer;
    try {
      return await Promise.race([promise,new Promise((_,reject)=>{
        timer=setTimeout(()=>reject(new Error('마이크 응답 시간 초과')),5000);
      })]);
    } finally {clearTimeout(timer);}
  }
  function showMicStopped() {
    els.voiceBtn?.classList.remove('active');
    els.voiceBtn?.setAttribute?.('aria-pressed','false');
    els.voiceBtn?.setAttribute?.('aria-label','녹음 시작');
    els.voiceMode.disabled=false;els.triggerWord.disabled=false;els.sttEngine.disabled=false;els.audioInputSource.disabled=false;syncGlassesInputUI();
    if (!captureFailure && !micCloseUncertain) transcriptStatusMessage = '';
    setHUDNotice(pendingClips.length || draining ? '녹음 정지 · 남은 전사 처리 중' : '녹음 정지');
    setVoiceHint(pendingClips.length || draining ? '마이크 꺼짐 · 남은 음성만 처리 중' : '마이크 꺼짐 · 탭하면 녹음 시작');
    if(!captureFailure && !pendingClips.length && !draining)renderLiveHUD();
  }
  async function confirmMicrophoneOff() {
    const generation=captureGeneration;
    const closing=Promise.resolve().then(()=>bridge.audioControl(false));
    pendingMicCloses.add(closing);
    closing.then(()=>pendingMicCloses.delete(closing),()=>pendingMicCloses.delete(closing));
    const closed = await microphoneCall(closing);
    if(generation!==captureGeneration)return;
    if (!closed) throw new Error('Even 앱이 마이크 종료를 확인하지 않았습니다.');
    if (pendingMicOpen) {
      micCloseUncertain=true;
      setHUDNotice('마이크 정지 확인 중');
      setVoiceHint('마이크 시작 응답을 기다린 뒤 종료를 다시 확인합니다.');
    } else {
      micMayBeOpen=false;micCloseUncertain=false;
      showMicStopped();
    }
  }

  async function toggleVoiceRecording({fromSourceSwitch=false}={}) {
    if(inputSwitchPromise && !fromSourceSwitch){resumeAfterSettings=false;await stopVoiceRecording();return;}
    // Stop has priority even while warmup or microphone-open is awaiting a reply.
    if (isRecording || capturePreparing || pendingMicOpen || micCloseUncertain || pendingMicCloses.size) {
      await stopVoiceRecording();return;
    }
    if (micBusy || micStopPromise || (inputSwitchPromise && !fromSourceSwitch)) return;
    if (!displayReady) { updateHUD('Even 앱에서 G2에 연결한 뒤 시작하세요.'); return; }
    micBusy=true;capturePreparing=true;
    const generation=++captureGeneration;
    const controller=new AbortController();captureAbort=controller;
    els.voiceBtn?.classList.add('active');
    els.voiceBtn?.setAttribute?.('aria-label','녹음 준비 취소');
    setHUDNotice('녹음 준비 중 · 탭하면 취소');
    let startingTask;
    try {
      startingTask=startVoiceRecording(generation,controller.signal);
      captureTask=startingTask;await startingTask;
    } catch (err) {
      if(generation!==captureGeneration || controller.signal.aborted){
        if(pendingMicOpen){micCloseUncertain=true;setHUDNotice('마이크 상태 확인 필요');setVoiceHint('마이크 시작 응답이 없습니다. 앱을 완전히 닫아주세요.');}
        return;
      }
      isRecording=false;resetCaptureTimers();
      if(micMayBeOpen || pendingMicOpen) await stopVoiceRecording(false);
      else showMicStopped();
      setVoiceHint('녹음 실패 · 마이크 권한과 G2 연결을 확인하세요');
      updateHUD(`마이크 오류: ${err.message}`);
    } finally {
      if(captureAbort===controller)captureAbort=null;
      capturePreparing=false;micBusy=false;
      if(captureTask===startingTask)captureTask=null;
    }
  }

  function syncCaptureSettings(){
    syncModeUI();syncGlassesInputUI();
    els.audioInputSource.value=config.audioInputSource;
    els.sttEngine.value=config.sttEngine;
    els.translationEngine.value=config.translationEngine;
    els.chromeTranslation.checked=config.translationEngine!=='llm';
    els.translateExcludeSelf.checked=config.translateExcludeSelf!==false;
  }
  async function applyLiveSettings(patch){
    if(!Object.entries(patch).some(([key,value])=>JSON.stringify(value)!==JSON.stringify(pendingSettings[key] ?? config[key])))return inputSwitchPromise;
    pendingSettings={...pendingSettings,...patch};settingsEdited=true;
    if(inputSwitchPromise){
      // Cancel a superseded engine warmup; the transaction will open only the latest choice.
      if(capturePreparing)stopVoiceRecording(false).catch(console.warn);
      return inputSwitchPromise;
    }
    resumeAfterSettings=isRecording || capturePreparing;
    const work=(async()=>{
      // Let inputSwitchPromise be assigned before any asynchronous work can finish.
      await Promise.resolve();
      try{
        do{
          const starting=captureTask;
          if(isRecording || capturePreparing || micMayBeOpen || pendingMicOpen || micCloseUncertain || pendingMicCloses.size)await stopVoiceRecording(true);
          if(starting)await starting.catch(()=>{});
          if(micMayBeOpen || pendingMicOpen || micCloseUncertain || pendingMicCloses.size)throw Error('기존 마이크 종료를 확인한 뒤 다시 변경해 주세요.');
          const next=pendingSettings;pendingSettings={};
          const preserveDisplay=config.glassesMode==='translate' && (!next.glassesMode || next.glassesMode==='translate');
          Object.assign(config,next);config.chromeTranslation=config.translationEngine!=='llm';
          recognitionEpoch++;
          resetQueuesForModeSwitch(config.glassesMode,true,preserveDisplay);
          syncCaptureSettings();await persistSettings();
          // Translation preparation may fall back to LLM; STT readiness is checked before opening.
          await prepareSelectedTranslation().catch(()=>{});
          if(Object.keys(pendingSettings).length)continue;
          if(resumeAfterSettings)await toggleVoiceRecording({fromSourceSwitch:true});
          else autoPrepareEngines();
        }while(Object.keys(pendingSettings).length);
      }catch(error){pendingSettings={};syncCaptureSettings();els.hintStatus.textContent=`설정 변경 실패 · ${error.message}`;setVoiceHint(`설정 변경 실패 · ${error.message}`);}
    })();
    inputSwitchPromise=work;
    try{await work;}finally{if(inputSwitchPromise===work){inputSwitchPromise=null;resumeAfterSettings=false;syncGlassesInputUI();}}
  }
  function switchAudioInputSource(){
    if(inputSwitchPromise)return inputSwitchPromise;
    return applyLiveSettings({audioInputSource:config.audioInputSource==='phone'?'glasses':'phone'});
  }


  async function startVoiceRecording(generation, signal) {
    translationSentence.reset();
    abortPendingPreview();
    hintGate.reset();
    config.aiHintMode=true;
    config.hintSensitivity=els.hintSensitivity.value || 'balanced';
    config.hintWebSearch=els.hintWebSearch?.checked!==false;
    if (els.glassesMode?.value) config.glassesMode = normalizeGlassesMode(els.glassesMode.value);
    if (els.translateSourceLang?.value) config.translateSourceLang = els.translateSourceLang.value;
    if (els.translateTargetLang?.value) config.translateTargetLang = els.translateTargetLang.value;
    config.voiceMode = 'continuous';
    config.triggerWord = els.triggerWord.value.trim();
    config.speechFilter = els.speechFilter.value === 'standard' ? 'standard' : 'strict';
    config.audioInputSource=els.audioInputSource.value==='phone'?'phone':'glasses';
    config.glassesInput=readGlassesInputControls();
    config.sttEngine=['chrome-online','apple'].includes(els.sttEngine.value)?els.sttEngine.value:'whisper';config.sttFallback=els.sttFallback.checked!==false;
    config.speakerScope = els.speakerScope.value === 'self' ? 'self' : 'conversation';
    config.translateExcludeSelf=els.translateExcludeSelf?.checked!==false;
    config.chromeTranslation=els.chromeTranslation?.checked!==false;
    const mode = normalizeGlassesMode(config.glassesMode);
    const continuousCapture = isContinuousCaptureMode(mode);
    const labels = triggerLabels();
    if (mode === 'ai' && (labels.length>8 || labels.some(label=>Array.from(label.replace(/\s/gu,'')).length>40))) throw new Error('AI 호출어는 최대 8개, 각각 40자 이하로 입력하세요.');
    activeSttEngine=config.sttEngine;els.sttEngine.disabled=false;
    activeAudioInputSource=config.audioInputSource==='phone'?AudioInputSource.Phone:AudioInputSource.Glasses;
    activeGlassesInput=normalizeGlassesInput(config.glassesInput);activeInputProcessor=new GlassesInputProcessor(activeGlassesInput);lastInputMeterAt=0;
    els.glassesInputMeter.value=-72;els.glassesInputLevel.textContent='오디오 수신 대기';
    const captureAudioSource=activeAudioInputSource;els.audioInputSource.disabled=false;syncGlassesInputUI();
    if(activeSttEngine!=='whisper'){
      try{await prepareSelectedStt(signal,true);}
      catch(error){if(signal.aborted)throw error;if(!config.sttFallback)throw error;const failedEngine=activeSttEngine;activeSttEngine='whisper';els.sttEngineStatus.textContent=`${failedEngine==='apple'?'Apple':'Chrome'} 준비 실패 · Whisper 사용 (${error.message})`;}
    }
    if(activeSttEngine==='whisper'){
    setHUDNotice('음성 엔진 준비 중');
    await prepareSelectedStt(signal,true,'whisper');
    }else els.sttEngineStatus.textContent=activeSttEngine==='chrome-online'?'Chrome 온라인 STT · 음성은 Google로 전송됩니다':'Apple 로컬 STT · Mac에서 음성 처리 중';
    if(generation!==captureGeneration || signal.aborted)return;
    captureFailure = '';
    for(const clip of pendingClips)clip.attempts=0;
    sessionId = newID(); pendingTrigger = false;
    activeUtterance = null; pendingPreview = null; draftUtteranceId = '';
    captionStable = captionDraft = ''; captionFinal = '';
    transcriptNotice(continuousCapture ? '실시간 전사 중' : '녹음 중');
    audioBuffer = []; recordedBytes = 0; recordingStarted = Date.now();
    segmenter = new SpeechSegmenter(inputSegmentOptions(activeSttEngine,mode,activeAudioInputSource,activeGlassesInput));
    isRecording = true;
    els.voiceMode.disabled = true; els.triggerWord.disabled = true;els.sttEngine.disabled=false;
    els.voiceBtn?.classList.add('active');
    updateHUD(continuousCapture
      ? '말씀하세요. 말하는 동안 자막이 갱신됩니다.' : '말씀하세요. 다시 탭하면 전송합니다.');
    micMayBeOpen=true;
    setHUDNotice('마이크 여는 중 · 탭하면 취소');
    const opening = Promise.resolve().then(()=>bridge.audioControl(true,captureAudioSource));
    pendingMicOpen=opening;
    // A delayed successful open must never undo a newer stop request.
    opening.then(async opened=>{
      if(pendingMicOpen===opening)pendingMicOpen=null;
      if(!opened)micMayBeOpen=false;
      if(generation!==captureGeneration && opened){
        try {
          if(micStopPromise)await micStopPromise;
          await confirmMicrophoneOff();
        } catch(err) {
          micCloseUncertain=true;setHUDNotice('마이크 종료 확인 실패');setVoiceHint('다시 탭해 종료를 재시도하거나 앱을 완전히 닫아주세요.');
        }
      }
    },()=>{if(pendingMicOpen===opening)pendingMicOpen=null;});
    const opened = await microphoneCall(opening);
    if(generation!==captureGeneration || signal.aborted)return;
    if (!opened) { els.voiceMode.disabled = false; els.triggerWord.disabled = false; throw new Error(`${captureAudioSource===AudioInputSource.Phone?'휴대폰':'안경'} 마이크를 켤 수 없습니다. Even 앱의 마이크 권한을 확인하세요.`); }
    capturePreparing=false;micCloseUncertain=false;
    els.voiceBtn?.setAttribute?.('aria-pressed','true');
    els.voiceBtn?.setAttribute?.('aria-label','녹음 정지');
    transcriptNotice(continuousCapture?'실시간 전사 중':'녹음 중');
    captureTimer = setTimeout(() => {
      if (isRecording && !recordedBytes) {
        captureFailure=captureAudioSource===AudioInputSource.Phone?'휴대폰 마이크 입력이 없습니다.\niOS 설정에서 Even 앱의 마이크 권한을 확인한 뒤 휴대폰의 녹음 버튼으로 다시 시작해 주세요.':'마이크는 열렸지만 오디오가 오지 않습니다.\nG2 연결과 마이크 권한을 확인하고 앱을 다시 여세요.';
        stopVoiceRecording().then(()=>setVoiceHint(captureFailure)).catch(console.warn);
        updateHUD(captureFailure);
      }
    }, 6000);
    if (!continuousCapture) manualTimer = setTimeout(() => stopVoiceRecording().catch(console.warn), 25000);
    setVoiceHint(`${captureAudioSource===AudioInputSource.Phone?'휴대폰':'안경'} 마이크 켜짐 · 오디오 수신 대기`);
    if (pendingClips.length) drainClips();
  }

  function glassesInputLocked(){return isRecording || capturePreparing || pendingMicOpen || micMayBeOpen || micCloseUncertain || pendingMicCloses.size || inputSwitchPromise;}
  function readGlassesInputControls(){return normalizeGlassesInput({sensitivity:els.glassesSensitivity.value,gain:els.glassesGain.value});}
  function syncGlassesInputUI(){
    const settings=normalizeGlassesInput(config.glassesInput);
    els.glassesInputSettings.hidden=config.audioInputSource!=='glasses';
    els.glassesSensitivity.value=settings.sensitivity;els.glassesGain.value=String(settings.gain);
    els.glassesSensitivity.disabled=config.audioInputSource!=='glasses';els.glassesGain.disabled=!!glassesInputLocked();
  }
  function updateGlassesInputMeter(signal,excluded){
    if(activeAudioInputSource!==AudioInputSource.Glasses || Date.now()-lastInputMeterAt<150)return;
    lastInputMeterAt=Date.now();
    const db=value=>Number.isFinite(levelDb(value))?`${Math.round(levelDb(value))} dBFS`:'무음';
    els.glassesInputMeter.value=Math.max(-72,Math.min(0,levelDb(signal.outputLevel)));
    const detected=signal.rawLevel>=GLASSES_THRESHOLDS[activeGlassesInput.sensitivity];
    els.glassesInputLevel.textContent=`수신 ${db(signal.rawLevel)} → 적용 후 ${db(signal.outputLevel)} · ${excluded?'화자 필터로 제외':detected?'입력 감지':'감지 기준 미만'}${signal.limited?' · 큰 소리 증폭 제한':''}`;
  }

  function receivePCM(bytes, speakerRole = 'unknown') {
    // Preserve the audio clock and silence boundaries when excluding a speaker.
    const signal=activeInputProcessor.process(bytes,activeAudioInputSource);
    bytes=filterSpeakerPCM(signal.bytes,speakerRole,config);
    const excluded=bytes!==signal.bytes;
    // Gain changes STT input volume, not which background frames count as speech.
    const level=excluded?0:signal.rawLevel;
    updateGlassesInputMeter(signal,excluded);
    recordedBytes += bytes.length;
    const continuousCapture = isContinuousCaptureMode();
    setVoiceHint(`${continuousCapture ? '연속 감지 중' : '녹음 중'} · ${(recordedBytes/32000).toFixed(1)}초 · 음량 ${Math.round(level*1000)} · 대기 ${pendingClips.length}`);
    if (!continuousCapture) {
      audioBuffer.push(bytes);
      if(activeSttEngine!=='whisper'){
        if(!activeUtterance)activeUtterance={id:newID(),revision:0,lastPreviewBytes:0,agreement:new LocalAgreement()};
        if(recordedBytes-activeUtterance.lastPreviewBytes>=8000){
          activeUtterance.lastPreviewBytes=recordedBytes;
          pendingPreview={bytes:mergePCM(audioBuffer),sessionId,segmentId:activeUtterance.id,engine:activeSttEngine,revision:++activeUtterance.revision,startedAt:new Date(recordingStarted).toISOString(),endedAt:new Date(recordingStarted+recordedBytes/32).toISOString(),mode:normalizeGlassesMode(config.glassesMode),sourceLang:config.translateSourceLang,targetLang:config.translateTargetLang,speechFilter:config.speechFilter};
          drainPreview();
        }
      }
      return;
    }
    const clip = segmenter.push(bytes,level);
    if (clip) {
      const id = activeUtterance?.id || newID();
      activeUtterance = null; abortPendingPreview();
      enqueueClip(clip, true, recordingStarted + recordedBytes/32, id);
      return;
    }
    if (!segmenter.parts.length) { activeUtterance = null; pendingPreview = null; return; }
    if (!activeUtterance) activeUtterance = {id:newID(),revision:0,lastPreviewBytes:0,agreement:new LocalAgreement()};
    if (recordedBytes-activeUtterance.lastPreviewBytes < (activeSttEngine==='whisper'?32000:8000)) return;
    const snapshot = segmenter.snapshot();
    if (!snapshot) return;
    activeUtterance.lastPreviewBytes = recordedBytes;
    const endedAt = recordingStarted + recordedBytes/32;
    pendingPreview = {bytes:snapshot,sessionId,segmentId:activeUtterance.id,engine:activeSttEngine,
      revision:++activeUtterance.revision,startedAt:new Date(endedAt-snapshot.length/32).toISOString(),
      endedAt:new Date(endedAt).toISOString(),speechFilter:config.speechFilter,
      mode:normalizeGlassesMode(config.glassesMode),
      sourceLang:config.translateSourceLang || 'auto',
      targetLang:config.translateTargetLang || 'ko'};
    drainPreview();
  }

  function fallbackTranslateSourceLang(clip) {
    const target = String(clip?.targetLang || config.translateTargetLang || 'ko').toLowerCase();
    return target === 'ko' ? 'en' : 'ko';
  }

  function resolveTranslateSourceLang(clip) {
    const explicit = String(clip?.sourceLang || config.translateSourceLang || 'auto').trim().toLowerCase();
    if (/^[a-z]{2}$/.test(explicit)) return explicit;
    return fallbackTranslateSourceLang(clip);
  }

  function sttPayload(clip, overrideLang = '') {
    let lang = overrideLang;
    if (!lang) {
      if (clip.mode === 'translate') {
        lang = resolveTranslateSourceLang(clip);
      } else {
        lang = 'ko';
      }
    }
    return {engine:clip.engine || activeSttEngine,fallback:config.sttFallback!==false,audio:uint8ArrayToBase64(clip.bytes),format:'pcm',sampleRate:16000,language:lang,
      realtime:clip.mode==='ai',sessionId:clip.sessionId,segmentId:clip.segmentId,revision:clip.revision || 0,
      startedAt:clip.startedAt,endedAt:clip.endedAt,triggerWord:clip.triggerWord || '',speechFilter:clip.speechFilter};
  }

  async function drainPreview() {
    if (previewBusy || previewRetryTimer || !pendingPreview || !isRecording) return;
    // Chrome recognition shares a continuous browser stream across segments;
    // archive playback must stay ordered there. Apple can rebuild full snapshots.
    if((pendingPreview.mode!=='translate' || String(pendingPreview.engine).startsWith('chrome-')) && (draining || pendingClips.length))return;
    const snapshot = pendingPreview; pendingPreview = null; previewBusy = true;
    const controller = new AbortController();
    previewAbortController = controller;
    const timeout = setTimeout(() => controller.abort(),15000);
    try {
      const payload = sttPayload(snapshot);
      let res = await gatewayFetch(`${config.serverUrl}/api/stt/preview`, {
        method:'POST',headers:getAuthHeaders({'Content-Type':'application/json'}),
        body:JSON.stringify(payload),signal:controller.signal,
      });
      if (!res.ok && res.status !== 429 && payload.language === 'auto') {
        autoLangUnsupported = true;
        res = await gatewayFetch(`${config.serverUrl}/api/stt/preview`, {
          method:'POST',headers:getAuthHeaders({'Content-Type':'application/json'}),
          body:JSON.stringify(sttPayload(snapshot, fallbackTranslateSourceLang(snapshot))),signal:controller.signal,
        });
      }
      if (res.status===429) {
        // The native engine may still be finalizing audio. Keep only the newest
        // snapshot and retry independently of transcript persistence/backlog.
        if(isRecording && sessionId===snapshot.sessionId && activeUtterance?.id===snapshot.segmentId){
          if(!pendingPreview)pendingPreview=snapshot;
          previewRetryTimer=setTimeout(()=>{previewRetryTimer=null;drainPreview();},300);
        }
        return;
      }
      if (!res.ok) throw new Error(`임시 전사 HTTP ${res.status}`);
      const data = await res.json();
      if (!isRecording || sessionId!==snapshot.sessionId || activeUtterance?.id!==snapshot.segmentId ||
          data.sessionId!==snapshot.sessionId || data.segmentId!==snapshot.segmentId || data.revision!==snapshot.revision) return;
      if (data.rejected || !data.text?.trim()) return;
      const agreed = (data.engine==='apple' || data.engine?.startsWith('chrome-'))
        ? typeof data.stableText==='string' && typeof data.draftText==='string'
          ? {stable:data.stableText+(data.stableText && data.draftText?' ':''),draft:data.draftText}
          : {stable:'',draft:data.text}
        : activeUtterance.agreement.update(data.text);
      draftUtteranceId = snapshot.segmentId;
      captionStable = agreed.stable; captionDraft = agreed.draft;
      if(snapshot.mode==='translate' && config.chromeTranslation!==false){
        enqueueTranslation(snapshot.segmentId,data.text,'',resolveTranslateSourceLang(snapshot),snapshot.targetLang,{sessionId:snapshot.sessionId,draft:true,stableText:agreed.stable.trim(),revision:snapshot.revision,joinSentence:String(snapshot.engine||'').startsWith('chrome-') && isContinuousCaptureMode(),startedAt:snapshot.startedAt,endedAt:snapshot.endedAt});
      }
      transcriptStatusMessage = '';
      transcriptNotice('실시간 전사 중');
      renderLiveHUD();
    } catch (err) {
      if (isRecording && !controller.signal.aborted) setVoiceHint(`임시 자막 지연 · 최종 전사는 계속합니다 (${err.message})`);
    } finally {
      clearTimeout(timeout);
      if (previewAbortController === controller) previewAbortController = null;
      previewBusy = false;
      if (pendingClips.length) drainClips();
      if (pendingPreview) drainPreview();
    }
  }

  function enqueueClip(bytes, continuous, endedAt = Date.now(), segmentId = newID()) {
    abortPendingPreview();
    const mode = normalizeGlassesMode(config.glassesMode);
    const sourceLang = config.translateSourceLang || 'auto';
    const targetLang = config.translateTargetLang || 'ko';
    const triggerWord = mode==='ai' ? config.triggerWord : '';
    if(mode==='ai') {
      // Keep one active inference and only the newest waiting clip.
      pendingClips=pendingClips.filter(clip=>clip.mode!=='ai' || clip===activeFinalClip);
    }
    pendingClips.push({ epoch:recognitionEpoch, bytes, continuous, sessionId, segmentId, mode, sourceLang, targetLang,engine:activeSttEngine,
      startedAt:new Date(endedAt-bytes.length/32).toISOString(),endedAt:new Date(endedAt).toISOString(),
      triggerWord, speechFilter:config.speechFilter });
    if(mode!=='ai' && isRecording && (pendingClips.length>=MAX_PENDING_CLIPS || pendingClips.reduce((sum,clip)=>sum+clip.bytes.length,0)>=MAX_PENDING_AUDIO_BYTES)) {
      // Preserve every queued segment; bound growth by stopping capture, not deleting audio.
      stopVoiceRecording().then(()=>{
        if(!captureFailure && !isRecording) {
          setHUDNotice('처리 지연 · 녹음 일시정지');
          setVoiceHint('대기 음성을 전사·저장하고 있습니다. 처리 후 탭하면 다시 녹음합니다.');
        }
      }).catch(console.warn);
    }
    drainClips();
  }

  async function stopVoiceRecording(flush = true) {
    if(micStopPromise)return micStopPromise;
    if(!isRecording && !capturePreparing && !pendingMicOpen && !micMayBeOpen && !micCloseUncertain && !pendingMicCloses.size)return;
    hintGate.reset();
    const hadCapture=isRecording;
    const continuousCapture = isContinuousCaptureMode();
    ++captureGeneration;
    captureAbort?.abort();capturePreparing=false;
    isRecording=false;resetCaptureTimers();
    const segmentId=activeUtterance?.id || newID();
    if(!flush)stopChromeSegment(segmentId);
    activeUtterance=null;abortPendingPreview();
    const clip=hadCapture ? (continuousCapture?segmenter?.flush():mergePCM(audioBuffer)) : null;
    audioBuffer=[];
    if((!flush || !clip) && liveTranslations.hasDraft()){liveTranslationDraft=false;liveTranslations.reset();}
    setHUDNotice('마이크 정지 중');
    setVoiceHint('마이크를 끄고 있습니다. 남은 전사는 계속 처리합니다.');
    els.voiceBtn?.setAttribute?.('aria-label','마이크 정지 중');
    const stopping=(async()=>{
      try {
        if(micMayBeOpen || pendingMicOpen || micCloseUncertain)await confirmMicrophoneOff();
        else showMicStopped();
      } catch(err) {
        micCloseUncertain=true;
        setHUDNotice('마이크 종료 확인 실패');
        setVoiceHint('다시 탭해 종료를 재시도하거나 앱을 완전히 닫아주세요.');
        updateHUD(`마이크 종료를 확인하지 못했습니다.\n${err.message}\n다시 탭하거나 앱을 완전히 닫아주세요.`);
      }
    })();
    micStopPromise=stopping;
    try {await stopping;} finally {if(micStopPromise===stopping)micStopPromise=null;}
    if(flush && clip?.length>=3200)enqueueClip(clip,continuousCapture,recordingStarted+recordedBytes/32,segmentId);
    if(!micCloseUncertain && !pendingMicOpen && !pendingMicCloses.size)showMicStopped();
  }

  async function drainClips() {
    if (previewBusy && (normalizeGlassesMode(config.glassesMode)!=='translate' || activeSttEngine.startsWith('chrome-'))) abortPendingPreview();
    if (draining) return;
    draining = true;
    try {
      while (pendingClips.length) {
        const clip = pendingClips[0];
        activeFinalClip=clip;
        if(clip.mode==='ai' && Date.now()-Date.parse(clip.endedAt)>20000){pendingClips.shift();continue;}
        let data;
        try {
          data = await processAudioTranscription(clip);
        } catch (err) {
          // A mode switch can clear the old request while new audio is queued.
          // Never shift a new segment in response to an old cancellation/failure.
          if(!pendingClips.includes(clip))continue;
          if(clip.mode==='ai') {
            pendingClips.splice(pendingClips.indexOf(clip),1);
            els.hintStatus.textContent=`음성 처리 지연 · 해당 구간 생략, 새 대화 계속 (${err.message})`;
            setHUDNotice('지난 음성 생략 · 계속 듣는 중');
            setVoiceHint('실시간 AI · 처리하지 못한 구간은 버리고 최신 대화를 듣습니다.');
            // Avoid a hot loop on repeated server failures without stopping capture.
            await new Promise(resolve=>setTimeout(resolve,200));
            continue;
          }
          if(err.busy){await new Promise(resolve=>setTimeout(resolve,300));continue;}
          clip.attempts=(clip.attempts || 0)+1;
          if(err.retryable!==false && clip.attempts<=3) {
            setHUDNotice(`전사 재시도 ${clip.attempts}/3 · 음성 보관 중`);
            setVoiceHint(`일시적인 전사 지연 · ${pendingClips.length}개 보관 · 자동 재시도합니다.`);
            await new Promise(resolve=>setTimeout(resolve,300 * 2**(clip.attempts-1)));
            continue;
          }
          throw err;
        }
        if(!pendingClips.includes(clip))continue;
        pendingClips.splice(pendingClips.indexOf(clip),1);
        if (draftUtteranceId === clip.segmentId) { captionStable = captionDraft = ''; draftUtteranceId = ''; }
        if (data.rejected) { setVoiceHint('불확실하거나 반복되는 인식 결과 제외 · 다시 말씀하세요'); renderLiveHUD(); continue; }
        if (!data.text?.trim()) { setVoiceHint('음성 필터 · 인식할 말소리 없음'); renderLiveHUD(); continue; }
        const text = data.text.trim();

        const time = new Date(clip.startedAt).toLocaleTimeString();
        transcriptText += `[${time}] ${text}\n`;
        els.transcriptLog.textContent = transcriptText;
        if(clip.epoch!==undefined && clip.epoch!==recognitionEpoch)continue;
        captionFinal=text;transcriptStatusMessage='';
        transcriptContext.push(text);if(transcriptContext.length>8)transcriptContext.shift();
        const clipMode = normalizeGlassesMode(clip.mode || config.glassesMode);
        const clipTargetLang = clip.targetLang || config.translateTargetLang || 'ko';
        let prompt = '';
        let directIntent={direct:false,search:false};
        let chatSession=false;
        if(clipMode==='ai') {
          const route=routeAIUtterance(data,{waiting:pendingTrigger,name:triggerLabels()[0] || data.matchedTrigger || 'AI'});
          pendingTrigger=route.waiting;
          if(route.kind==='ack'){
            chatEpoch++;hintGate.reset();aiAbortController?.abort();pendingAI.length=0;
            answerText=route.ack;els.chatOutput.textContent=route.ack;
            els.hintStatus.textContent='호출어 확인 · 다음 문장을 AI 채팅 질문으로 받습니다';
            setHUDNotice('질문 대기');renderLiveHUD();continue;
          }
          if(route.kind==='chat'){
            chatSession=true;prompt=route.prompt;
            directIntent={...directQuestionIntent(prompt),direct:true};
            hintGate.reset();if(activeAIJob?.hint)aiAbortController?.abort();
            els.hintStatus.textContent='AI 채팅 · 이전 질문·답변 참조';
          }else if(isRecording && normalizeGlassesMode(config.glassesMode)==='ai'){
            // Ambient questions are observed conversation too. Only the wake
            // route (or typed input) authorizes a conversational AI response.
            hintGate.observe(text,{id:clip.segmentId,at:Date.parse(clip.endedAt)});
          }
        }else if(clipMode==='translate'){prompt=text;pendingTrigger=false;}
        if (prompt) {
          const context=chatSession?'':clip.continuous?transcriptContext.slice(0,-1).slice(-2).join('\n').slice(-800):'';
          if(clipMode==='translate')enqueueTranslation(clip.segmentId,prompt,context,resolveTranslateSourceLang(clip),clipTargetLang,{sessionId:clip.sessionId,joinSentence:String(clip.engine||'').startsWith('chrome-') && clip.continuous,startedAt:clip.startedAt,endedAt:clip.endedAt});
          else enqueueAI(clip.segmentId,prompt,context,clipMode,clipTargetLang,directIntent.direct?{directQuestion:true,forceSearch:directIntent.search,chatSession,chatEpoch:chatSession?chatEpoch:undefined}:{});
        } else {
          if (pendingTrigger) setHUDNotice('AI 호출됨 · 질문을 말씀하세요');
          else transcriptNotice('전사 저장됨');
          renderLiveHUD();
        }
      }
    } catch (err) {
      await stopVoiceRecording(false).catch(console.warn);
      captureFailure = `전사/저장 실패: ${err.message}\n처리하지 못한 음성 ${pendingClips.length}개를 앱 안에 보관 중입니다. 다시 탭하면 재시도합니다. 앱을 닫으면 미저장 음성은 사라집니다.`;
      setHUDNotice('전사 오류 · 미저장 음성 대기'); renderLiveHUD();
      setVoiceHint('오류로 녹음 중지 · 서버 확인 후 다시 시작');
    } finally {
      activeFinalClip=null;
      draining = false;
      renderLiveHUD();
      if (!isRecording && !pendingClips.length && !micStopPromise && !pendingMicOpen && !micCloseUncertain && !pendingMicCloses.size) setVoiceHint('마이크 꺼짐 · 전사 저장 완료');
      if (pendingPreview) drainPreview();
    }
  }

  async function processAudioTranscription(clip) {
    const controller = new AbortController();
    clipAbortController = controller;
    let timedOut=false;
    // Worker inference allows 90s plus cold-start time; a 20s client limit caused
    // abandoned requests and repeated inference under load.
    const timeout = setTimeout(() => {timedOut=true;controller.abort();}, clip.mode==='ai'?12000:120000);
    try {
      const payload = sttPayload(clip);
      let res = await gatewayFetch(`${config.serverUrl}/api/stt/transcribe`, {
        method: 'POST', headers: getAuthHeaders({ 'Content-Type': 'application/json' }),
        body: JSON.stringify(payload), signal: controller.signal,
      });
      if (!res.ok && payload.language === 'auto') {
        autoLangUnsupported = true;
        res = await gatewayFetch(`${config.serverUrl}/api/stt/transcribe`, {
          method: 'POST', headers: getAuthHeaders({ 'Content-Type': 'application/json' }),
          body: JSON.stringify(sttPayload(clip, fallbackTranslateSourceLang(clip))), signal: controller.signal,
        });
      }
      if (!res.ok) {
        const err=new Error(`HTTP ${res.status}: ${(await res.text()).slice(0,300)}`);
        err.busy=res.status===429;
        err.retryable=[408,425,429].includes(res.status) || res.status>=500;
        throw err;
      }
      const data = await res.json();
      if(data.warning)els.sttEngineStatus.textContent=data.warning;
      if (data.text?.trim() && !data.saved) {
        const err=new Error('서버에서 전사 저장을 확인하지 못했습니다. 서버를 업데이트하세요.');
        err.retryable=false;throw err;
      }
      return data;
    } catch(err) {
      if(timedOut){const timeoutError=new Error('전사 서버 응답 시간 초과 · 음성 보관 중');timeoutError.retryable=true;throw timeoutError;}
      throw err;
    } finally {
      clearTimeout(timeout);
      if (clipAbortController === controller) clipAbortController = null;
    }
  }

  function openTranscriptModal() {
    recordReturnFocus = document.activeElement;
    previousBodyOverflow = document.body.style.overflow;
    document.body.style.overflow = 'hidden';
    els.transcriptModal.hidden = false;
    els.btnCloseTranscripts.focus();
  }
  function closeTranscriptModal() {
    els.transcriptModal.hidden = true;
    document.body.style.overflow = previousBodyOverflow;
    recordReturnFocus?.focus?.();
  }

  async function loadTranscripts() {
    const date = els.transcriptDate.value;
    const tz = Intl.DateTimeFormat().resolvedOptions().timeZone;
    const res = await gatewayFetch(`${config.serverUrl}/api/stt/transcripts?date=${encodeURIComponent(date)}&tz=${encodeURIComponent(tz)}`, {headers:getAuthHeaders()});
    if (!res.ok) throw new Error(`전사 조회 실패: HTTP ${res.status}`);
    const data = await res.json();
    transcriptText = data.entries.map(entry => `[${new Date(entry.startedAt).toLocaleTimeString()} – ${new Date(entry.endedAt).toLocaleTimeString()}] ${entry.text}`).join('\n');
    els.transcriptLog.textContent = transcriptText || '저장된 전사 기록이 없습니다.';
  }

  function enqueueTranslation(id,prompt,context,source,target,options={}){
    if(options.sessionId && options.sessionId!==sessionId)return;
    const job={id,prompt,context,source,target,engine:localEngine(),...options};
    const sentence=options.joinSentence?translationSentence.update(job):job;
    if(sentence)liveTranslations.submit(sentence);
  }

  function enqueueAI(id, prompt, context, mode = config.glassesMode, targetLang = config.translateTargetLang, options = {}) {
    if(options.replyOnly && pendingAI.some(job=>job.mode==='translate' && !job.replyOnly))return;
    if (knownAIJobs.has(id)) return;
    knownAIJobs.add(id);
    if (knownAIJobs.size>1000) knownAIJobs.delete(knownAIJobs.values().next().value);
    const normMode = normalizeGlassesMode(mode);
    const normLang = targetLang || 'ko';
    if(options.chatSession){
      for(const dropped of pendingAI)if(dropped.hint)hintGate.complete(dropped);
      pendingAI.splice(0,pendingAI.length,...pendingAI.filter(job=>job.chatSession));
      if(pendingAI.length>=8){els.hintStatus.textContent='AI 채팅 질문 8개 대기 중 · 답변 후 다시 말씀하세요';return;}
    }else if(pendingAI.some(job=>job.chatSession)){
      if(options.hint)hintGate.complete(options);
      return;
    }
    if (pendingAI.length > 0 && !options.chatSession) {
      const last = pendingAI[pendingAI.length - 1];
      if(options.hint){for(const dropped of pendingAI)if(dropped.hint)hintGate.complete(dropped);pendingAI.length=0;}
      else if (last.mode === normMode && last.targetLang === normLang && !last.hint && !last.replyOnly && !options.replyOnly && !options.translationToken && !last.translationToken) {
        last.prompt = `${last.prompt}\n${prompt}`.trim();
        if (context) last.context = context;
        return;
      }
      pendingAI.length = 0;
    }
    pendingAI.push({...options,chatEpoch:options.chatEpoch || (normMode==='ai'?chatEpoch:undefined),translationHistoryId:options.translationHistoryId || (normMode==='translate'?id:undefined),prompt, context, mode: normMode, targetLang: normLang});
    drainAI();
  }
  async function drainAI() {
    if (processingAI) return;
    processingAI = true; clearTimeout(answerTimer); stopAIAutoScroll();
    aiAbortedByModeSwitch = false;
    try {
      while (pendingAI.length) {
        const job = pendingAI.shift();
        if(job.hint && !hintGate.fresh(job)){hintGate.complete(job);continue;}
        activeAIJob=job;
        if(job.translationToken && job.translationToken!==translationToken || job.chatEpoch && job.chatEpoch!==chatEpoch)continue;
        if(job.mode==='translate')translationFailure='';
        aiScrollOffset = 0;
        aiPhaseTitle = job.mode === 'translate' ? '번역 및 답변 생성 중' : '생각 중';
        aiPhaseDetail = '';
        aiReasoningPreview = '';
        if(!job.hint && !captureFailure)setHUDNotice(isRecording ? 'AI 답변 중 · 전사 계속' : '녹음 정지 · AI 답변 중');
        renderLiveHUD();
        await queryDKSTChat(job.prompt, job.context, job.mode, job.targetLang,job);
        activeAIJob=null;
      }
    } finally {
      activeAIJob=null;
      processingAI = false;
      stopThinkingAnimation();
      if (!aiAbortedByModeSwitch) {
        const win = sliceHUDWindow(answerText, 0, aiVisibleLines(), AI_BOX_WIDTH_PX);
        aiScrollOffset = 0;
        const holdMs = Math.max(8000, AI_SCROLL_START_MS + win.maxOffset * AI_SCROLL_LINE_MS + 4000);
        answerHoldUntil = Date.now() + holdMs;
        transcriptNotice(isRecording ? (isContinuousCaptureMode() ? '실시간 전사 중' : '녹음 중') : '전사 저장됨');
        renderLiveHUD();
        if (win.maxOffset > 0 && normalizeGlassesMode(config.glassesMode)!=='translate') {
          startAIAutoScroll(win.maxOffset);
        }
        answerTimer = setTimeout(() => { answerHoldUntil=0; if(isRecording)renderLiveHUD(); }, holdMs);
      }
      aiAbortedByModeSwitch = false;
      if (pendingAI.length) drainAI();
    }
  }

  function finalChatText(data) {
    if(typeof data.final_assistant_content==='string')return data.final_assistant_content;
    const output=data.result?.output || data.output;
    if(!Array.isArray(output))return null;
    const messages=output.filter(item=>item.type==='message' && typeof item.content==='string');
    return messages.length ? messages.map(item=>item.content).join('') : null;
  }

  // --- LLM Chat Execution ---
  async function queryDKSTChat(prompt, conversationContext = '', mode = config.glassesMode, targetLang = config.translateTargetLang, options = {}) {
    const activeMode = normalizeGlassesMode(mode);
    const modelConfig={model:config.model,roleModels:normalizeRoleModels(config.roleModels)};
    const requestChatScope=chatScope();
    const chatContext=options.chatSession?chatMemory.context(requestChatScope):'';
    let enrichmentPlan=null;
    let repliesReplaced=false;
    let receivedVisibleAnswer=false;
    let chromeText=options.chromeTranslationText || '';let chromeSource=options.translationSource || resolveTranslateSourceLang({targetLang});
    let cueDisplayed=false;let cueTopic='';
    function showCue(cue,status){
      const fitted=cue?.fact?fitCue(cue.fact):'';
      if(!fitted || !hintGate.relevant(options,cue.topic) || hintGate.duplicateCue(fitted) || hintGate.duplicateTopic(cue.topic))return false;
      answerText=fitted;cueDisplayed=true;cueTopic=cue.topic;
      els.chatQuestion.textContent=cue.topic?`💡 자동 힌트 · ${cue.topic}`:'💡 자동 힌트';
      els.chatOutput.textContent=fitted;els.hintStatus.textContent=status;renderLiveHUD();return true;
    }
    const controller = new AbortController();
    aiAbortController = controller;
    const deadline=chatDeadline(controller,{chatSession:options.chatSession===true,hint:options.hint===true,setTimer:setTimeout,clearTimer:clearTimeout});
    if (!options.hint) {
      isThinking = true;
      stopAIAutoScroll();
      aiPhaseTitle = activeMode === 'translate' ? '번역 및 답변 생성 중' : '생각 중';
      aiPhaseDetail = '대화 설정 확인 중';
      aiReasoningPreview = '';
      aiReasoningStartMs = 0;
      els.chatQuestion.textContent=prompt;
      if(!chromeText)els.chatOutput.textContent=answerText;
      startThinkingAnimation();
      renderLiveHUD();
    }
    try {
      if(activeMode==='translate' && config.chromeTranslation!==false && !chromeText && !options.chromeAttempted){
        aiPhaseTitle=localLabel()+' 로컬 번역 중';renderLiveHUD();
        const local=await translateLocal({text:prompt,source:chromeSource,target:targetLang || config.translateTargetLang},controller.signal);
        if(local.ok){
          chromeText=local.text;translationDisplay={...translationDisplay,translation:chromeText};answerText=chromeText;
          translationHistory.update(options.translationHistoryId,chromeText,{final:true,revision:options.translationRevisionId || options.translationHistoryId});
          els.chatOutput.textContent=translationPhoneText(translationDisplay);
          els.chromeTranslationStatus.textContent=localLabel()+' 로컬 번역 완료 · 답변 예시 생성 중';
          aiPhaseTitle='긍정·부정 답변 생성 중';renderLiveHUD();
        }else els.chromeTranslationStatus.textContent=`기존 LLM 번역 사용 · ${local.reason}`;
      }
      const settingsRes = await gatewayFetch(`${config.serverUrl}/api/config`, {headers:getAuthHeaders(), signal: controller.signal});
      deadline.touch();
      if(!settingsRes.ok)throw new Error(`대화 설정 조회 실패: HTTP ${settingsRes.status}`);
      const settings = await settingsRes.json();
      if(!['standard','stateful'].includes(settings.llm_mode))throw new Error('서버의 LLM 연결 모드를 확인하세요.');
      const chatPayload=(prompts,phase)=>{
        const isAmbient=!options.chatSession;
        const role=llmRequestRole({mode:activeMode,chatSession:options.chatSession,translated:!!chromeText});
        const model=resolveRoleModel(role,modelConfig);
        if(!model)throw Error('설정에서 기본 모델 또는 해당 역할의 LLM 모델을 선택하세요.');
        const body={model,stream:true};
        if(isAmbient)body.glasses_client=true;
        if(chromeText){body.glasses_hint=true;body.glasses_hint_phase='answer';body.hint_research={source:'internal'};}
        if(isAmbient && (options.hint || options.directQuestion)){body.glasses_hint=true;body.glasses_hint_phase=options.directQuestion?'direct':phase;}
        if(settings.llm_mode==='stateful') {
          body.input=prompts.userPrompt;body.system_prompt=prompts.systemPrompt;
          if((isAmbient && (options.hint || options.directQuestion)) || settings.context_strategy!=='stateful')body.store=false;
        } else body.messages=[{role:'system',content:prompts.systemPrompt},{role:'user',content:prompts.userPrompt}];
        return body;
      };
      if(options.hint) {
        aiPhaseTitle='정보 조회 중';aiPhaseDetail='';setHUDNotice();
        els.hintStatus.textContent='보충할 정보 찾는 중';
        const planRes=await gatewayFetch(`${config.serverUrl}/api/chat`,{
          method:'POST',headers:getAuthHeaders({'Content-Type':'application/json'}),signal:controller.signal,
          body:JSON.stringify(chatPayload(cuePrompts(prompt,conversationContext,options.category,hintGate.diversityContext(),{preferWeb:config.hintWebSearch!==false,keyword:options.keyword || ''}),'plan'))
        });
        const cue=parseCueResponse(await readHintPlanResponse(planRes),[conversationContext,prompt].join('\n'));
        if(cue)els.hintStatus.textContent=`주제: ${cue.topic} · ${cue.question || '내부 지식 힌트'}`;
        enrichmentPlan=cue;
        if(cue?.fact){showCue(cue,'대화 도움 표시 · 계속 듣는 중');return;}
        if(!enrichmentPlan || !hintGate.fresh(options)){
          els.hintStatus.textContent='새로 보탤 도움 없음 · 계속 듣는 중';return;
        }
        aiPhaseTitle=enrichmentPlan.source==='web'?'웹에서 근거 찾는 중':enrichmentPlan.source==='memory'?'기억에서 근거 찾는 중':'내부 지식에서 정보 찾는 중';
        aiPhaseDetail=enrichmentPlan.topic || '';setHUDNotice();
        els.hintStatus.textContent=enrichmentPlan.source==='web'?'웹에서 근거 찾는 중':enrichmentPlan.source==='memory'?'기억에서 근거 찾는 중':'내부 지식에서 정보 찾는 중';
      }
      if(options.hint && enrichmentPlan?.source==='web'){els.hintStatus.textContent='선별한 내용 웹검색 중';}
      const separateLLMReplies=activeMode==='translate' && !chromeText && resolveRoleModel('translation',modelConfig)!==resolveRoleModel('translationReply',modelConfig);
      const prompts=options.hint ? hintAnswerPrompts(enrichmentPlan,hintGate.diversityContext()) : separateLLMReplies?translationOnlyPrompts(prompt,chromeSource,targetLang || config.translateTargetLang,conversationContext):chromeText?translationReplyPrompts(prompt,chromeText,chromeSource,targetLang || config.translateTargetLang):buildModePrompts(activeMode,{
        chatSession:options.chatSession===true,conciseMode:config.conciseMode,targetLang:targetLang || config.translateTargetLang || 'ko',prompt,
        conversationContext:options.chatSession?chatContext:conversationContext
      });
      const payload=chatPayload(prompts,'answer');
      if(options.hint)payload.hint_research={query:enrichmentPlan.question,source:enrichmentPlan.source};
      if(options.directQuestion && options.forceSearch && !options.chatSession)payload.hint_research={query:Array.from(prompt).slice(0,300).join(''),source:'web'};
      if (!options.hint) {
        aiPhaseTitle = activeMode === 'translate' ? '번역 및 답변 생성 중' : '생각 중';
        aiPhaseDetail = '';
        if(!chromeText)els.chatOutput.textContent = answerText;
        renderLiveHUD();
      }
      const res = await gatewayFetch(`${config.serverUrl}/api/chat`, {
        method:'POST',headers:getAuthHeaders({'Content-Type':'application/json'}),body:JSON.stringify(payload), signal: controller.signal
      });
      if(options.hint && enrichmentPlan?.source==='web' && res.status===424){
        // Evidence lookup failed before generation. Spend at most one LLM
        // request on a clearly labeled stable-knowledge fallback, never retry web.
        els.hintStatus.textContent='웹검색 사용 불가 · 내부 지식 확인';
        const fallback=await gatewayFetch(`${config.serverUrl}/api/chat`,{
          method:'POST',headers:getAuthHeaders({'Content-Type':'application/json'}),signal:controller.signal,
          body:JSON.stringify(chatPayload(cuePrompts(prompt,conversationContext,options.category,hintGate.diversityContext()),'plan'))
        });
        const cue=parseCueResponse(await readHintPlanResponse(fallback),[conversationContext,prompt].join('\n'));
        if(!showCue(cue,'웹검색 사용 불가 · 내부 지식 힌트'))els.hintStatus.textContent='웹 근거 조회 실패 · 전사 계속';
        return;
      }
      if(!res.ok) {
        const detail=(await res.text()).trim().slice(0,500);
        throw new Error(`대화 생성 실패: HTTP ${res.status}${detail ? ' · '+detail : ''}`);
      }

      let rawAnswer = '';
      let rawReasoning = '';
      function setVisibleAnswer(raw,commitReplies=false) {
        if(options.translationToken && options.translationToken!==translationToken || options.chatEpoch && options.chatEpoch!==chatEpoch)return;
        if(activeMode==='translate') {
          const previousTranslation=translationDisplay.translation;
          const parsedTranslation=parseTranslationDisplay(raw);
          if(separateLLMReplies){parsedTranslation.positive='';parsedTranslation.negative='';}
          if(commitReplies && !chromeText && (!parsedTranslation.translation || ['…','...'].includes(parsedTranslation.translation)))throw Error('LLM에서 번역문을 받지 못했습니다.');
          translationDisplay=mergeTranslationDisplay(translationDisplay,parsedTranslation,{commitReplies});
          if(commitReplies && hasCompleteTranslationReplies(parsedTranslation))repliesReplaced=true;
          if(!translationDisplay.translation && config.chromeTranslation!==false)translationDisplay.translation=previousTranslation;
          if(chromeText)translationDisplay.translation=chromeText;
          answerText=translationDisplay.translation;
          if(chromeText || parsedTranslation.translation)translationHistory.update(options.translationHistoryId,answerText,{final:commitReplies || !!chromeText,revision:options.translationRevisionId || options.translationHistoryId});
          els.chatOutput.textContent=translationPhoneText(translationDisplay);
        } else {
          const visible=stripSourcesForGlasses(raw);
          if(visible)receivedVisibleAnswer=true;
          if(visible || commitReplies)answerText=visible;
          els.chatOutput.textContent=answerText;
        }
      }

      function applyStreamStatusEvent(type, parsed) {
        if (type === 'model_load.start') {
          aiPhaseTitle = '모델 로딩 중';
          aiPhaseDetail = '모델을 메모리에 불러오는 중';
          return true;
        }
        if (type === 'model_load.progress') {
          const pct = typeof parsed.progress === 'number' ? Math.round(parsed.progress * 100) : null;
          aiPhaseTitle = pct !== null ? `모델 로딩 중 (${pct}%)` : '모델 로딩 중';
          aiPhaseDetail = '모델을 메모리에 불러오는 중';
          return true;
        }
        if (type === 'model_load.end') {
          aiPhaseTitle = '모델 로드 완료';
          aiPhaseDetail = '프롬프트 처리 중';
          return true;
        }
        if (type === 'prompt_processing.progress') {
          const pct = typeof parsed.progress === 'number' ? Math.round(parsed.progress * 100) : null;
          aiPhaseTitle = pct !== null ? `프롬프트 처리 중 (${pct}%)` : '프롬프트 처리 중';
          aiPhaseDetail = '';
          return true;
        }
        if (type === 'skill.applied') {
          const names = Array.isArray(parsed.skills)
            ? parsed.skills.map(s => String(s?.display_name || s?.name || s?.namespace || '').trim()).filter(Boolean)
            : [];
          aiPhaseTitle = '스킬 적용됨';
          aiPhaseDetail = names.join(', ');
          return true;
        }
        if (type === 'reasoning.start') {
          if (!aiReasoningStartMs) aiReasoningStartMs = Date.now();
          aiPhaseTitle = '생각 중';
          aiPhaseDetail = '';
          return true;
        }
        if (type === 'reasoning.delta') {
          if (!aiReasoningStartMs) aiReasoningStartMs = Date.now();
          const chunk = parsed.content || parsed.reasoning_content || parsed.text || '';
          rawReasoning += chunk;
          const elapsedMs = Number(parsed.total_elapsed_ms || parsed.elapsed_ms || (Date.now() - aiReasoningStartMs));
          const sec = Number.isFinite(elapsedMs) && elapsedMs >= 1000 ? Math.round(elapsedMs / 1000) : 0;
          aiPhaseTitle = sec > 0 ? `생각 중 (${sec}초)` : '생각 중';
          aiPhaseDetail = '';
          aiReasoningPreview = rawReasoning;
          return true;
        }
        if (type === 'reasoning.end') {
          const elapsedMs = Number(parsed.total_elapsed_ms || parsed.elapsed_ms || (aiReasoningStartMs ? Date.now() - aiReasoningStartMs : 0));
          const sec = Number.isFinite(elapsedMs) && elapsedMs >= 1000 ? Math.round(elapsedMs / 1000) : 0;
          aiPhaseTitle = sec > 0 ? `${sec}초 동안 생각함` : '생각 완료';
          aiPhaseDetail = '답변 작성 중';
          aiReasoningPreview = '';
          return true;
        }
        if (type.startsWith('tool_call.')) {
          const toolInfo = formatToolStatusText({ ...parsed, type });
          aiPhaseTitle = toolInfo.title;
          aiPhaseDetail = toolInfo.detail;
          aiReasoningPreview = '';
          return true;
        }
        if (type === 'generation.phase') {
          if (parsed.phase === 'thinking') {
            aiPhaseTitle = '생각 중';
            return true;
          }
          if (parsed.phase === 'tool_call' && parsed.tool) {
            const toolInfo = formatToolStatusText({ ...parsed, type: 'tool_call.start' });
            aiPhaseTitle = toolInfo.title;
            aiPhaseDetail = toolInfo.detail;
            return true;
          }
          if (parsed.phase === 'answering') {
            aiPhaseTitle = '답변 작성 중';
            aiPhaseDetail = '';
            aiReasoningPreview = '';
            return true;
          }
        }
        return false;
      }

      if ((res.headers?.get('content-type')?.includes('text/event-stream') || config.connectionMode === 'sse') && res.body) {
        const reader = res.body.getReader();
        const decoder = new TextDecoder();
        let buffered = '';
        let eventName='';
        function consumeLine(line) {
          if(options.chatEpoch && options.chatEpoch!==chatEpoch)return;
          line=line.replace(/\r$/,'');
          if(!line){eventName='';return;}
          if(line.startsWith('event:')){eventName=line.slice(6).trim();return;}
          if (!line.startsWith('data:')) return;
          const dataStr = line.slice(5).trim();
          if (!dataStr || dataStr === '[DONE]') return;
          if(eventName==='error')throw new Error(dataStr);
          const parsed = JSON.parse(dataStr);
          if (parsed.error) throw new Error(typeof parsed.error === 'string' ? parsed.error : parsed.error.message || JSON.stringify(parsed.error));
          const type=parsed.type || eventName;
          if(type==='error')throw new Error(parsed.message || 'LLM 응답 오류');

          if (parsed.choices?.[0]?.delta?.reasoning_content) {
            if (!aiReasoningStartMs) aiReasoningStartMs = Date.now();
            rawReasoning += parsed.choices[0].delta.reasoning_content;
            aiPhaseTitle = '생각 중';
            aiReasoningPreview = rawReasoning;
          }

          const statusChanged = applyStreamStatusEvent(type, parsed);

          const final=finalChatText(parsed);
          if(final!==null)rawAnswer=final;
          else if(type==='message.delta')rawAnswer+=parsed.content || '';
          else rawAnswer+=parsed.choices?.[0]?.delta?.content || parsed.message?.content || '';

          const extracted = extractVisibleAndReasoning(rawAnswer);
          if (extracted.isInlineReasoning) {
            aiPhaseTitle = '생각 중';
            aiReasoningPreview = extracted.reasoningText;
          } else if (extracted.visibleText) {
            aiPhaseTitle = '답변 작성 중';
            aiPhaseDetail = '';
            aiReasoningPreview = '';
          }

          if(options.translationToken && options.translationToken!==translationToken || options.chatEpoch && options.chatEpoch!==chatEpoch)return;
          if(options.hint){if(statusChanged || extracted.isInlineReasoning || extracted.visibleText)setHUDNotice();return;} // Optional hints only update header status until complete.
          setVisibleAnswer(rawAnswer);
          if(config.connectionMode==='sse' || statusChanged)renderLiveHUD();
        }
        while (true) {
          const { done, value } = await reader.read();
          if(value?.length)deadline.touch();
          buffered += decoder.decode(value, { stream: !done });
          const lines = buffered.split('\n');
          buffered = lines.pop();
          for (const line of lines) consumeLine(line);
          if (done) {
            if (buffered.trim()) consumeLine(buffered);
            break;
          }
        }
      } else {
        const data = await res.json();
        if(data.error)throw new Error(typeof data.error==='string'?data.error:data.error.message || JSON.stringify(data.error));
        rawAnswer = finalChatText(data) ?? data.choices?.[0]?.message?.content ?? data.response ?? '';
        if(!options.hint){
          setVisibleAnswer(rawAnswer);
          renderLiveHUD();
        }
      }

      if(options.translationToken && options.translationToken!==translationToken || options.chatEpoch && options.chatEpoch!==chatEpoch)return;
      const enrichedFact=options.hint?extractHintFact(rawAnswer,[conversationContext,prompt].join('\n'),enrichmentPlan):'';
      if(options.hint && (!hintGate.relevant(options,enrichmentPlan.topic) || !fitCue(enrichedFact) || hintGate.duplicateCue(fitCue(enrichedFact)) || hintGate.duplicateTopic(enrichmentPlan.topic))){els.hintStatus.textContent='현재 대화에 필요한 새 정보 없음 · 생략';return;}
      if(options.hint){answerText=fitCue(enrichedFact);cueDisplayed=true;cueTopic=enrichmentPlan.topic;els.chatQuestion.textContent=cueTopic?`💡 자동 힌트 · ${cueTopic}`:'💡 자동 힌트';}else setVisibleAnswer(rawAnswer,true);
      if(options.hint)els.hintStatus.textContent=enrichmentPlan.source==='web'?'웹검색 근거로 보충한 정보':enrichmentPlan.source==='memory'?'저장된 기억으로 보충한 정보':'내부 지식으로 보충한 정보';
      els.chatOutput.textContent = activeMode==='translate'?translationPhoneText(translationDisplay):answerText;
      if(chromeText)els.chromeTranslationStatus.textContent=repliesReplaced?`${options.translatedBy || localLabel()} 번역 · 답변 예시 완료`:hasCompleteTranslationReplies(translationDisplay)?`${options.translatedBy || localLabel()} 번역 완료 · 이전 답변 예시 유지`:`${options.translatedBy || localLabel()} 번역 완료 · 답변 예시를 받지 못했습니다`;
      renderLiveHUD();
      if(!answerText.trim())throw new Error('LLM에서 답변 본문을 받지 못했습니다. 서버의 모델 상태를 확인하세요.');

      if(separateLLMReplies){
        els.chromeTranslationStatus.textContent='LLM 번역 완료 · 지정한 번역 답변 모델로 생성 중';
        enqueueAI(newID(),prompt,conversationContext,'translate',targetLang || config.translateTargetLang,{replyOnly:true,chromeTranslationText:answerText,chromeAttempted:true,translationSource:chromeSource,translationToken:options.translationToken,translationHistoryId:options.translationHistoryId,translationRevisionId:options.translationRevisionId,translatedBy:'LLM'});
      }
      if(options.chatSession && requestChatScope===chatScope()){
        chatMemory.remember(requestChatScope,prompt,answerText);
        els.hintStatus.textContent='직접 질문 답변 완료 · 계속 듣는 중';
      }

      // TTS Playback if enabled
      if (!options.hint && config.enableTTS && answerText && config.voiceMode !== 'continuous') {
        playTTS(answerText);
      }
    } catch (err) {
      if(options.translationToken && options.translationToken!==translationToken || options.chatEpoch && options.chatEpoch!==chatEpoch)return;
      if (controller.signal.aborted && aiAbortedByModeSwitch) {
        return;
      }
      const failure=chatFailureMessage(err,deadline);
      if(options.hint && controller.signal.aborted && pendingAI.some(job=>job.directQuestion))return;
      if(options.hint){els.hintStatus.textContent=`힌트 생성 실패 · 전사 계속 (${failure})`;return;}
      if(chromeText){els.chromeTranslationStatus.textContent=`번역 완료 · 답변 예시 생성 실패 (${failure})`;els.chatOutput.textContent=translationPhoneText(translationDisplay);renderLiveHUD();return;}
      if(activeMode==='translate'){
        translationFailure='번역 오류';answerText=translationDisplay.translation;
        els.chromeTranslationStatus.textContent=`LLM 번역 실패 · ${answerText?'이전 번역 유지':'새 번역 대기'} (${failure})`;
        els.chatOutput.textContent=translationPhoneText(translationDisplay);renderLiveHUD();return;
      }
      console.error('Chat error:', err);
      els.hintStatus.textContent=`AI 채팅 중단 · ${failure}`;
      if(!receivedVisibleAnswer)answerText=`AI 오류: ${failure}`;
      els.chatOutput.textContent=receivedVisibleAnswer?`${answerText}\n\n응답 중단: ${failure}`:answerText;renderLiveHUD();
    } finally {
      if(options.hint)hintGate.complete(options,{displayed:cueDisplayed,text:cueDisplayed?answerText:'',topic:cueTopic});
      deadline.stop();
      if (aiAbortController === controller) aiAbortController = null;
      isThinking = false;
      stopThinkingAnimation();
      aiPhaseTitle = '';
      aiPhaseDetail = '';
      aiReasoningPreview = '';
      if (!isRecording && !captureFailure && !micCloseUncertain && !pendingMicOpen) setVoiceHint('마이크 꺼짐 · 탭하면 녹음 시작');
    }
  }

  // --- TTS Support ---
  async function playTTS(text) {
    try {
      const res = await gatewayFetch(`${config.serverUrl}/api/tts`, {
        method: 'POST',
        headers: getAuthHeaders({ 'Content-Type': 'application/json' }),
        body: JSON.stringify({
          text: text.slice(0, 300), // Limit length for glasses glance
          lang: 'ko',
          format: 'wav'
        })
      });
      if (!res.ok) return;

      const blob = await res.blob();
      const audioUrl = URL.createObjectURL(blob);
      const audio = new Audio(audioUrl);
      audio.addEventListener('ended', () => URL.revokeObjectURL(audioUrl), { once: true });
      try { await audio.play(); } catch (err) { URL.revokeObjectURL(audioUrl); throw err; }
    } catch (e) {
      console.warn('TTS playback notice:', e);
    }
  }

  // --- Server Connection & Auth ---
  function gatewayFetch(url, options = {}) {
    // Packaged apps authenticate with bearer tokens; never send WebView cookies.
    return fetch(url, { ...options, credentials: 'omit' });
  }

  function getAuthHeaders(baseHeaders = {}) {
    const headers = { ...baseHeaders };
    if (config.authToken) {
      headers['Authorization'] = `Bearer ${config.authToken}`;
    } else if (config.apiKey) {
      headers['Authorization'] = `Bearer ${config.apiKey}`;
    }
    return headers;
  }

  async function testServerConnection() {
    const url = (els.serverUrl.value || '').trim().replace(/\/+$/, '');
    if (!url) {
      alert('서버 주소를 입력하세요.');
      return;
    }

    els.btnTestConnection.textContent = '연결 중...';
    try {
      const res = await gatewayFetch(`${url}/api/health`, { method: 'GET' });
      if (res.ok) {
        alert('서버 연결 성공! 정상 통신 가능합니다.');
      } else {
        alert(`서버 응답 오류: ${res.status}`);
      }
    } catch (err) {
      alert(`서버 연결 실패:\n${err.message}`);
    } finally {
      els.btnTestConnection.textContent = '테스트';
    }
  }

  async function login() {
    const url = (els.serverUrl.value || '').trim().replace(/\/+$/, '');
    const username = (els.authUsername.value || '').trim();
    const password = (els.authPassword.value || '').trim();

    if (!username || !password) {
      alert('아이디와 비밀번호를 입력하세요.');
      return;
    }

    els.btnLogin.textContent = '로그인 중...';
    try {
      const res = await gatewayFetch(`${url}/api/login`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ id: username, password, remember_me: true })
      });

      if (!res.ok) {
        throw new Error(res.status === 401
          ? '아이디 또는 비밀번호가 올바르지 않습니다. (HTTP 401)'
          : `서버 응답 오류: HTTP ${res.status}`);
      }

      const data = await res.json();
      if (!data.token) throw new Error('서버가 인증 토큰을 반환하지 않았습니다.');
      config.authToken = data.token;
      config.username = username;
      await saveSettings(false);

      if (els.loginStatusLabel) {
        els.loginStatusLabel.textContent = `인증됨 (${username})`;
        els.loginStatusLabel.style.color = 'var(--accent-green)';
      }
      alert('로그인 성공!');
      await fetchModelList();
      autoPrepareEngines();
    } catch (err) {
      const networkFailure = err instanceof TypeError || /load failed|failed to fetch/i.test(err.message);
      alert(networkFailure
        ? `서버 요청이 차단되었거나 연결되지 않았습니다.\n주소: ${url}\n서버 CORS 설정, 네트워크 권한, HTTP/HTTPS 주소를 확인하세요.`
        : `로그인 오류: ${err.message}`);
      if (els.loginStatusLabel) {
        els.loginStatusLabel.textContent = networkFailure ? '서버 연결 실패' : '로그인 실패';
        els.loginStatusLabel.style.color = 'var(--accent-red)';
      }
    } finally {
      els.btnLogin.innerHTML = '<span class="material-icons-round" style="font-size: 16px;">→</span> 로그인';
    }
  }

  async function checkLoginStatus() {
    const requestedToken = config.authToken;
    const requestedURL = config.serverUrl;
    try {
      const res = await gatewayFetch(`${config.serverUrl}/api/auth/check`, {
        headers: getAuthHeaders()
      });
      const status = res.ok ? await res.json() : {};
      if (requestedToken !== config.authToken || requestedURL !== config.serverUrl) return;
      if (status.authenticated && els.loginStatusLabel) {
        els.loginStatusLabel.textContent = `인증됨 (${config.username || 'User'})`;
        els.loginStatusLabel.style.color = 'var(--accent-green)';
        fetchModelList();
        autoPrepareEngines();
      } else if (requestedToken && status.authenticated === false) {
        config.authToken = '';
        await persistSettings();
        els.loginStatusLabel.textContent = '세션 만료 · 다시 로그인';
      }
    } catch (_) {}
  }

  async function fetchAppleCapabilities(){
    try{
      const response=await gatewayFetch(`${config.serverUrl}/api/apple/status`,{headers:getAuthHeaders()});
      const status=await response.json();
      document.getElementById('stt-option-apple').disabled=!response.ok || !status.available || !status.speechAvailable;
      document.getElementById('translation-option-apple').disabled=!response.ok || !status.available;
    }catch{}
  }
  function renderRoleModelSelectors(){
    for(const role of ROLE_MODEL_KEYS){
      const select=els.roleModels[role],selected=config.roleModels?.[role] || '';
      if(!select)continue;
      select.innerHTML='';
      for(const model of roleModelOptions(availableModels,config.model,selected)){
        const option=document.createElement('option');option.value=model.id;option.textContent=model.label;select.appendChild(option);
      }
      select.value=selected;
    }
  }
  async function fetchModelList() {
    fetchAppleCapabilities();
    const url = (els.serverUrl.value || '').trim().replace(/\/+$/, '');
    try {
      const res = await gatewayFetch(`${url}/api/models`, {
        headers: getAuthHeaders()
      });
      if (!res.ok) return;

      const data = await res.json();
      const models = normalizeModels(data);
      availableModels=models;
      const previousModel = els.modelSelect.value || config.model;

      els.modelSelect.innerHTML = '';
      if (models.length > 0) {
        models.forEach((m) => {
          const opt = document.createElement('option');
          opt.value = m.id;
          opt.textContent = m.label;
          els.modelSelect.appendChild(opt);
        });
        els.modelSelect.value = models.some(m => m.id === previousModel)
          ? previousModel : models[0].id;
        config.model = els.modelSelect.value;
      } else {
        els.modelSelect.innerHTML = '<option value="">등록된 모델 없음</option>';
        config.model = '';
      }
      renderRoleModelSelectors();
    } catch (e) {
      console.warn('Failed to fetch models:', e);
    }
  }

  // --- Helpers ---
  function bindEvents() {
    els.chatForm.addEventListener('submit',event=>{
      event.preventDefault();
      const prompt=els.chatInput.value.trim();
      if(!prompt)return;
      const role=llmRequestRole({mode:normalizeGlassesMode(config.glassesMode),chatSession:true,translated:normalizeGlassesMode(config.glassesMode)==='translate' && localEngine()!=='llm'});
      if(!resolveRoleModel(role,config)){els.chatOutput.textContent='설정에서 기본 모델 또는 해당 역할의 LLM 모델을 선택하세요.';return;}
      if(pendingAI.length>=8){els.chatOutput.textContent='대기 중인 질문이 많습니다. 답변 후 다시 보내주세요.';return;}
      els.chatInput.value='';
      if(processingAI)els.chatOutput.textContent='질문을 대기열에 추가했습니다.';
      const chatSession=normalizeGlassesMode(config.glassesMode)==='ai';
      if(chatSession){pendingTrigger=false;hintGate.reset();if(activeAIJob?.hint)aiAbortController?.abort();}
      enqueueAI(newID(),prompt,'',config.glassesMode,config.translateTargetLang,chatSession?{directQuestion:true,chatSession:true,chatEpoch,forceSearch:directQuestionIntent(prompt).search}:{});
    });
    const viewport=window.visualViewport;
    if(viewport) {
      const updateViewport=()=>{
        const gap=viewport.scale===1 ? Math.max(0,window.innerHeight-viewport.height-viewport.offsetTop) : 0;
        document.body.style.setProperty('--keyboard-gap',`${gap}px`);
        if(gap>0 && /^(INPUT|TEXTAREA|SELECT)$/.test(document.activeElement?.tagName))document.activeElement.scrollIntoView({block:'nearest'});
      };
      viewport.addEventListener('resize',updateViewport);
      viewport.addEventListener('scroll',updateViewport);
      updateViewport();
    }
    if (els.btnTestConnection) els.btnTestConnection.addEventListener('click', testServerConnection);
    if (els.btnLogin) els.btnLogin.addEventListener('click', login);
    if (els.btnRefreshModels) els.btnRefreshModels.addEventListener('click', fetchModelList);
    els.modelSelect.addEventListener('change',()=>{settingsEdited=true;config.model=els.modelSelect.value;renderRoleModelSelectors();persistSettings().catch(console.warn);});
    for(const role of ROLE_MODEL_KEYS)els.roleModels[role]?.addEventListener('change',()=>{
      settingsEdited=true;config.roleModels={...normalizeRoleModels(config.roleModels),[role]:els.roleModels[role].value};persistSettings().catch(console.warn);
    });

    if (els.btnSaveSettings) els.btnSaveSettings.addEventListener('click', saveSettings);
    if (els.voiceBtn) els.voiceBtn.addEventListener('click', toggleVoiceRecording);
    if (els.modeBtnAi) els.modeBtnAi.addEventListener('click', () => setGlassesMode('ai', true));
    if (els.modeBtnTranslate) els.modeBtnTranslate.addEventListener('click', () => setGlassesMode('translate', true));
    if (els.glassesMode) {
      els.glassesMode.addEventListener('change', () => {
        settingsEdited = true;
        setGlassesMode(els.glassesMode.value, true);
      });
    }
    if (els.translateSourceLang) {
      els.translateSourceLang.addEventListener('change', () => {
        settingsEdited = true;
        return applyLiveSettings({translateSourceLang:els.translateSourceLang.value || 'auto'});
      });
    }
    if (els.translateTargetLang) {
      els.translateTargetLang.addEventListener('change', () => {
        settingsEdited = true;
        return applyLiveSettings({translateTargetLang:els.translateTargetLang.value || 'ko'});
      });
    }
    els.btnTranscripts.addEventListener('click', () => { openTranscriptModal(); loadTranscripts().catch(err => { els.transcriptLog.textContent = err.message; }); });
    els.btnCloseTranscripts.addEventListener('click', closeTranscriptModal);
    els.transcriptModal.addEventListener('click', event => { if (event.target === els.transcriptModal) closeTranscriptModal(); });
    document.addEventListener('keydown', event => {
      if (els.transcriptModal.hidden) return;
      if (event.key === 'Escape') closeTranscriptModal();
      if (event.key === 'Tab') {
        const first = els.btnCloseTranscripts, last = els.btnExportTranscripts;
        if (event.shiftKey && document.activeElement === first) { event.preventDefault(); last.focus(); }
        else if (!event.shiftKey && document.activeElement === last) { event.preventDefault(); first.focus(); }
      }
    });
    els.glassesSensitivity.addEventListener('change',()=>applyLiveSettings({glassesInput:readGlassesInputControls()}));
    els.glassesGain.addEventListener('change',()=>{
      if(glassesInputLocked()){syncGlassesInputUI();return;}
      settingsEdited=true;config.glassesInput=readGlassesInputControls();syncGlassesInputUI();persistSettings().catch(console.warn);
    });
    els.audioInputSource.addEventListener('change',()=>applyLiveSettings({audioInputSource:els.audioInputSource.value==='phone'?'phone':'glasses'}));
    els.sttEngine.addEventListener('change',()=>applyLiveSettings({sttEngine:els.sttEngine.value}));
    els.sttFallback.addEventListener('change',()=>{settingsEdited=true;config.sttFallback=els.sttFallback.checked;persistSettings().catch(console.warn);});
    els.btnPrepareStt.addEventListener('click',()=>prepareSelectedStt(undefined,true).catch(()=>{}));
    els.chromeTranslation?.addEventListener('change',()=>applyLiveSettings({translationEngine:els.chromeTranslation.checked?'chrome':'llm'}));
    els.translationEngine?.addEventListener('change',()=>applyLiveSettings({translationEngine:els.translationEngine.value}));
    els.btnPrepareChrome?.addEventListener('click',()=>{
      if(localEngine()==='llm'){els.chromeTranslationStatus.textContent='LLM 번역은 별도 언어팩 준비가 필요 없습니다';return;}
      return prepareSelectedTranslation(undefined,true).catch(()=>{});
    });
    els.hintWebSearch?.addEventListener('change',()=>{settingsEdited=true;config.hintWebSearch=els.hintWebSearch.checked;persistSettings().catch(console.warn);});
    els.translateExcludeSelf?.addEventListener('change',()=>{
      return applyLiveSettings({translateExcludeSelf:els.translateExcludeSelf.checked});
    });
    for (const el of [els.hintSensitivity, els.serverUrl, els.authUsername, els.apiKey, els.modelSelect, els.connectionMode, els.voiceMode, els.triggerWord, els.speechFilter, els.speakerScope, els.optEnableTts, els.optConciseMode]) {
      if (!el) continue;
      el.addEventListener('input', () => { settingsEdited = true; });
      el.addEventListener('change', () => { settingsEdited = true; });
    }
    els.btnExportTranscripts.addEventListener('click', async () => {
      try {
        await loadTranscripts();
        const url = URL.createObjectURL(new Blob([transcriptText], {type:'text/plain;charset=utf-8'}));
        const link = document.createElement('a'); link.href=url; link.download=`transcript-${els.transcriptDate.value}.txt`; link.click();
        setTimeout(() => URL.revokeObjectURL(url), 1000);
      } catch(err) { alert(err.message); }
    });
  }

  function uint8ArrayToBase64(bytes) {
    let binary = '';
    const len = bytes.byteLength;
    for (let i = 0; i < len; i++) {
      binary += String.fromCharCode(bytes[i]);
    }
    return window.btoa(binary);
  }

  function escapeHtml(str) {
    return String(str)
      .replace(/&/g, '&amp;')
      .replace(/</g, '&lt;')
      .replace(/>/g, '&gt;')
      .replace(/"/g, '&quot;')
      .replace(/'/g, '&#039;');
  }

  // Run on DOM Ready
  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', init);
  } else {
    init();
  }
})();

