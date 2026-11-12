import { waitForEvenAppBridge, AudioInputSource, OsEventTypeList } from '@evenrealities/even_hub_sdk';
import { normalizeModels } from './model-list.mjs';
import { SpeechSegmenter, mergePCM, pcmLevel } from './audio-segments.mjs';
import { hudStatusText, hudNoticeText } from './hud-status.mjs';
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
    connectionMode: 'sse',
    conciseMode: true,
    voiceMode: 'manual',
    triggerWord: '이븐',
    speechFilter: 'strict',
    speakerScope: 'conversation',
    enableTTS: false,
    authToken: ''
  };

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
  let activeUtterance = null;
  let pendingPreview = null;
  let previewBusy = false;
  let draftUtteranceId = '';
  let captionFinal = '';
  let captureFailure = '';
  let captionStable = '';
  let captionDraft = '';
  let answerText = '';
  let answerHoldUntil = 0;
  let answerTimer = null;
  let processingAI = false;
  const pendingAI = [];
  const knownAIJobs = new Set();
  let pendingTrigger = false;
  let transcriptText = '';
  const transcriptContext = [];

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
    hudPageIndicator: document.getElementById('hud-page-indicator'),
    voiceMode: document.getElementById('voice-mode'),
    chatForm: document.getElementById('text-chat-form'),
    chatInput: document.getElementById('chat-input'),
    chatQuestion: document.getElementById('chat-question'),
    chatOutput: document.getElementById('chat-output'),
    triggerWord: document.getElementById('trigger-word'),
    speechFilter: document.getElementById('speech-filter'),
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
        config = { ...config, ...JSON.parse(saved) };
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
    if (els.optConciseMode) els.optConciseMode.checked = config.conciseMode !== false;
    if (els.optEnableTts) els.optEnableTts.checked = config.enableTTS !== false;
    els.voiceMode.value = config.voiceMode;
    els.triggerWord.value = config.triggerWord;
    els.speechFilter.value = config.speechFilter;
    els.speakerScope.value = config.speakerScope;
    const now = new Date();
    els.transcriptDate.value = `${now.getFullYear()}-${String(now.getMonth()+1).padStart(2,'0')}-${String(now.getDate()).padStart(2,'0')}`;
  }

  async function saveSettings(showMessage = true) {
    settingsEdited = true;
    config.serverUrl = (els.serverUrl.value || '').trim().replace(/\/+$/, '');
    config.username = (els.authUsername.value || '').trim();
    config.apiKey = (els.apiKey.value || '').trim();
    config.model = els.modelSelect.value || '';
    config.connectionMode = els.connectionMode.value;
    config.conciseMode = els.optConciseMode.checked;
    config.enableTTS = els.optEnableTts.checked;
    config.voiceMode = els.voiceMode.value === 'continuous' ? 'continuous' : 'manual';
    config.triggerWord = els.triggerWord.value.trim();
    config.speechFilter = els.speechFilter.value === 'standard' ? 'standard' : 'strict';
    config.speakerScope = els.speakerScope.value === 'self' ? 'self' : 'conversation';

    try {
      await persistSettings();
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
  async function initEvenRealitiesBridge() {
    try {
      updateConnectionState(false, 'Even 앱 연결 대기 · 브라우저 미리보기');
      bridge = await waitForEvenAppBridge();
      restoreNativeSettings();
      const result = await bridge.createStartUpPageContainer({
        containerTotalNum: 3,
        textObject: [{
          xPosition: 0, yPosition: 32, width: 576, height: 256,
          containerID: 1, containerName: 'gateway',
          content: 'DKST Gateway 준비 완료.\n한 번 탭: 녹음 시작/종료\n두 번 탭: 앱 종료',
          isEventCapture: 1,
        }, {
          xPosition: 400, yPosition: 0, width: 176, height: 28,
          containerID: 2, containerName: 'status', content: hudStatusText(null), isEventCapture: 0,
        }, {
          xPosition: 0, yPosition: 0, width: 396, height: 28,
          containerID: 3, containerName: 'notice', content: '대기', isEventCapture: 0,
        }],
      });
      if (result !== 0) throw new Error(`안경 화면 생성 실패: ${result}`);
      displayReady = true;
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

    // Listen to hardware events (tap, gesture, mic audio)
    if (typeof bridge.onEvenHubEvent === 'function') {
      bridge.onEvenHubEvent((event) => {
        if (!event) return;

        // Handle Audio Events
        if (event.audioEvent && isRecording) {
          const pcm = event.audioEvent.audioPcm;
          if (pcm && pcm.length > 0) {
            receivePCM(new Uint8Array(pcm), event.audioEvent.speakerRole);
          }
        }

        // Handle Touch / Gesture Event (Tap to toggle mic)
        const sysType = event.sysEvent
          ? (event.sysEvent.eventType ?? OsEventTypeList.CLICK_EVENT) : null;
        const textType = event.textEvent ? (event.textEvent.eventType ?? OsEventTypeList.CLICK_EVENT) : null;
        if (event.audioEvent) return;
        if (sysType === OsEventTypeList.DOUBLE_CLICK_EVENT || textType === OsEventTypeList.DOUBLE_CLICK_EVENT) {
          stopVoiceRecording().catch(console.warn).finally(() => bridge.shutDownPageContainer(1).catch(console.warn));
        } else if (sysType === OsEventTypeList.CLICK_EVENT || textType === OsEventTypeList.CLICK_EVENT) {
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
  function updateHUD(text, pageInfo = 'P 1/1') {
    if (els.hudScreen) {
      els.hudScreen.innerHTML = escapeHtml(text) + '<span class="hud-cursor"></span>';
    }
    if (els.hudPageIndicator) {
      els.hudPageIndicator.textContent = pageInfo;
    }

    if (displayReady) {
      pendingHUD.set(1, { containerID: 1, containerName: 'gateway', content: String(text).slice(0, 2000) });
      flushHUD();
    }
  }

  // Serialize and coalesce updates so streamed tokens do not flood Bluetooth.
  async function flushHUD() {
    if (sendingHUD) return;
    sendingHUD = true;
    try {
      while (pendingHUD.size && displayReady) {
        const [id, update] = pendingHUD.entries().next().value;
        pendingHUD.delete(id);
        const success = await bridge.textContainerUpgrade(update);
        if (!success) throw new Error('안경 화면 갱신 실패');
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
    const text = hudStatusText(glassesBattery);
    els.hudStatus.textContent = text;
    if (displayReady && text !== lastStatus) {
      pendingHUD.set(2, {containerID: 2, containerName: 'status', content: text});
      flushHUD();
    }
    lastStatus = text;
  }

  function setHUDNotice(message) {
    els.hudNotice.textContent = message;
    // The left header is bounded so a long wake word cannot wrap into the body.
    const content = hudNoticeText(message);
    if (displayReady) {
      pendingHUD.set(3, {containerID:3,containerName:'notice',content});
      flushHUD();
    }
  }
  function triggerLabels() {
    const seen = new Set();
    return config.triggerWord.split(/[,，;\n]/u).map(value=>value.trim()).filter(value=>{
      const key=value.normalize('NFC').replace(/\s/gu,'').toLowerCase();
      if(!key || seen.has(key))return false;
      seen.add(key);return true;
    });
  }
  function transcriptNotice(state) {
    if(micCloseUncertain || pendingMicOpen && !isRecording){setHUDNotice('마이크 정지 확인 필요');return;}
    if(micStopPromise || pendingMicCloses.size){setHUDNotice('마이크 정지 중');return;}
    if(captureFailure){setHUDNotice('전사 오류 · 미저장 음성 대기');return;}
    setHUDNotice(isRecording ? `${state} · AI 호출어: ${triggerLabels()[0] || '미설정'}` : (state==='전사 저장됨' ? '녹음 정지 · 저장 완료' : '녹음 정지'));
  }
  function renderLiveHUD() {
    if(captureFailure){updateHUD(captureFailure);return;}
    const draft = captionStable + captionDraft;
    if (processingAI || Date.now() < answerHoldUntil) {
      const tail = draft || captionFinal;
      updateHUD((answerText || '답변 생성 중...') + (isRecording && tail ? '\n… '+tail.slice(-90) : ''));
    } else {
      updateHUD([captionFinal, draft ? draft+(captionDraft ? ' …' : '') : ''].filter(Boolean).join('\n') || (isRecording ? '듣는 중...' : '녹음 정지'));
    }
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
    els.voiceMode.disabled=false;els.triggerWord.disabled=false;
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
    if (!closed) throw new Error('G2가 마이크 종료를 확인하지 않았습니다.');
    if (pendingMicOpen) {
      micCloseUncertain=true;
      setHUDNotice('마이크 정지 확인 중');
      setVoiceHint('마이크 시작 응답을 기다린 뒤 종료를 다시 확인합니다.');
    } else {
      micMayBeOpen=false;micCloseUncertain=false;
      showMicStopped();
    }
  }

  async function toggleVoiceRecording() {
    // Stop has priority even while warmup or microphone-open is awaiting a reply.
    if (isRecording || capturePreparing || pendingMicOpen || micCloseUncertain || pendingMicCloses.size) {
      await stopVoiceRecording();return;
    }
    if (micBusy || micStopPromise) return;
    if (!displayReady) { updateHUD('Even 앱에서 G2에 연결한 뒤 시작하세요.'); return; }
    micBusy=true;capturePreparing=true;
    const generation=++captureGeneration;
    const controller=new AbortController();captureAbort=controller;
    els.voiceBtn?.classList.add('active');
    els.voiceBtn?.setAttribute?.('aria-label','녹음 준비 취소');
    setHUDNotice('녹음 준비 중 · 탭하면 취소');
    try {
      await startVoiceRecording(generation,controller.signal);
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
    }
  }

  async function startVoiceRecording(generation, signal) {
    if (draining || previewBusy) { showMicStopped();setVoiceHint('이전 음성을 처리 중입니다. 잠시 후 다시 시작하세요.'); return; }
    config.voiceMode = els.voiceMode.value === 'continuous' ? 'continuous' : 'manual';
    config.triggerWord = els.triggerWord.value.trim();
    config.speechFilter = els.speechFilter.value === 'standard' ? 'standard' : 'strict';
    config.speakerScope = els.speakerScope.value === 'self' ? 'self' : 'conversation';
    const labels = triggerLabels();
    if (config.voiceMode === 'continuous' && !labels.length) throw new Error('AI 호출어를 입력하세요.');
    if (labels.length>8 || labels.some(label=>Array.from(label.replace(/\s/gu,'')).length>40)) throw new Error('AI 호출어는 최대 8개, 각각 40자 이하로 입력하세요.');
    const statusRes = await gatewayFetch(`${config.serverUrl}/api/stt/status`, { headers: getAuthHeaders(),signal });
    if(generation!==captureGeneration || signal.aborted)return;
    if (!statusRes.ok) throw new Error(`STT 상태 확인 실패: HTTP ${statusRes.status}`);
    const status = await statusRes.json();
    if(generation!==captureGeneration || signal.aborted)return;
    if (!status.active) throw new Error(status.message || '서버에서 STT 모델과 런타임을 설치하세요.');
    setHUDNotice('음성 엔진 준비 중');
    const warmup = await gatewayFetch(`${config.serverUrl}/api/stt/warmup`, {method:'POST',headers:getAuthHeaders(),signal});
    if(generation!==captureGeneration || signal.aborted)return;
    if (!warmup.ok) throw new Error(`음성 엔진 준비 실패: HTTP ${warmup.status}. 게이트웨이를 업데이트하세요.`);
    captureFailure = '';
    sessionId = newID(); pendingTrigger = false;
    activeUtterance = null; pendingPreview = null; draftUtteranceId = '';
    captionStable = captionDraft = ''; captionFinal = '';
    transcriptNotice(config.voiceMode === 'continuous' ? '실시간 전사 중' : '녹음 중');
    audioBuffer = []; recordedBytes = 0; recordingStarted = Date.now();
    segmenter = new SpeechSegmenter();
    isRecording = true;
    els.voiceMode.disabled = true; els.triggerWord.disabled = true;
    els.voiceBtn?.classList.add('active');
    updateHUD(config.voiceMode === 'continuous'
      ? '말씀하세요. 말하는 동안 자막이 갱신됩니다.' : '말씀하세요. 다시 탭하면 전송합니다.');
    micMayBeOpen=true;
    setHUDNotice('마이크 여는 중 · 탭하면 취소');
    const opening = Promise.resolve().then(()=>bridge.audioControl(true, AudioInputSource.Glasses));
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
    if (!opened) { els.voiceMode.disabled = false; els.triggerWord.disabled = false; throw new Error('G2 마이크를 켤 수 없습니다. 앱을 다시 열고 마이크 권한을 확인하세요.'); }
    capturePreparing=false;micCloseUncertain=false;
    els.voiceBtn?.setAttribute?.('aria-pressed','true');
    els.voiceBtn?.setAttribute?.('aria-label','녹음 정지');
    transcriptNotice(config.voiceMode==='continuous'?'실시간 전사 중':'녹음 중');
    captureTimer = setTimeout(() => {
      if (isRecording && !recordedBytes) {
        stopVoiceRecording().catch(console.warn);
        updateHUD('마이크는 열렸지만 오디오가 오지 않습니다.\nG2 연결과 마이크 권한을 확인하고 앱을 다시 여세요.');
      }
    }, 6000);
    if (config.voiceMode === 'manual') manualTimer = setTimeout(() => stopVoiceRecording().catch(console.warn), 25000);
    setVoiceHint('마이크 켜짐 · 오디오 수신 대기');
    if (pendingClips.length) drainClips();
  }

  function receivePCM(bytes, speakerRole = 'unknown') {
    // Preserve the audio clock and silence boundaries when excluding another speaker.
    if (config.speakerScope === 'self' && speakerRole === 'other') bytes = new Uint8Array(bytes.length);
    recordedBytes += bytes.length;
    const level = pcmLevel(bytes);
    setVoiceHint(`${config.voiceMode === 'continuous' ? '연속 전사 중' : '녹음 중'} · ${(recordedBytes/32000).toFixed(1)}초 · 음량 ${Math.round(level*1000)} · 대기 ${pendingClips.length}`);
    if (config.voiceMode === 'manual') { audioBuffer.push(bytes); return; }
    const clip = segmenter.push(bytes);
    if (clip) {
      const id = activeUtterance?.id || newID();
      activeUtterance = null; pendingPreview = null;
      enqueueClip(clip, true, recordingStarted + recordedBytes/32, id);
      return;
    }
    if (!segmenter.parts.length) { activeUtterance = null; pendingPreview = null; return; }
    if (!activeUtterance) activeUtterance = {id:newID(),revision:0,lastPreviewBytes:0,agreement:new LocalAgreement()};
    if (recordedBytes-activeUtterance.lastPreviewBytes < 32000) return;
    const snapshot = segmenter.snapshot();
    if (!snapshot) return;
    activeUtterance.lastPreviewBytes = recordedBytes;
    const endedAt = recordingStarted + recordedBytes/32;
    pendingPreview = {bytes:snapshot,sessionId,segmentId:activeUtterance.id,
      revision:++activeUtterance.revision,startedAt:new Date(endedAt-snapshot.length/32).toISOString(),
      endedAt:new Date(endedAt).toISOString(),speechFilter:config.speechFilter};
    drainPreview();
  }

  function sttPayload(clip) {
    return {audio:uint8ArrayToBase64(clip.bytes),format:'pcm',sampleRate:16000,language:'ko',
      sessionId:clip.sessionId,segmentId:clip.segmentId,revision:clip.revision || 0,
      startedAt:clip.startedAt,endedAt:clip.endedAt,triggerWord:clip.triggerWord || '',speechFilter:clip.speechFilter};
  }

  async function drainPreview() {
    if (previewBusy || draining || pendingClips.length || !pendingPreview || !isRecording) return;
    const snapshot = pendingPreview; pendingPreview = null; previewBusy = true;
    const controller = new AbortController();
    const timeout = setTimeout(() => controller.abort(),15000);
    try {
      const res = await gatewayFetch(`${config.serverUrl}/api/stt/preview`, {
        method:'POST',headers:getAuthHeaders({'Content-Type':'application/json'}),
        body:JSON.stringify(sttPayload(snapshot)),signal:controller.signal,
      });
      if (res.status===429) return;
      if (!res.ok) throw new Error(`임시 전사 HTTP ${res.status}`);
      const data = await res.json();
      if (!isRecording || sessionId!==snapshot.sessionId || activeUtterance?.id!==snapshot.segmentId ||
          data.sessionId!==snapshot.sessionId || data.segmentId!==snapshot.segmentId || data.revision!==snapshot.revision) return;
      if (data.rejected || !data.text?.trim()) return;
      const agreed = activeUtterance.agreement.update(data.text);
      draftUtteranceId = snapshot.segmentId;
      captionStable = agreed.stable; captionDraft = agreed.draft;
      transcriptNotice('실시간 전사 중');
      renderLiveHUD();
    } catch (err) {
      if (isRecording) setVoiceHint(`임시 자막 지연 · 최종 전사는 계속합니다 (${err.message})`);
    } finally {
      clearTimeout(timeout); previewBusy = false;
      if (pendingClips.length) drainClips();
      else if (pendingPreview) drainPreview();
    }
  }

  function enqueueClip(bytes, continuous, endedAt = Date.now(), segmentId = newID()) {
    pendingClips.push({ bytes, continuous, sessionId, segmentId,
      startedAt:new Date(endedAt-bytes.length/32).toISOString(),endedAt:new Date(endedAt).toISOString(),
      triggerWord:continuous?config.triggerWord:'',speechFilter:config.speechFilter });
    if (pendingClips.length > 12 && isRecording) {
      stopVoiceRecording(false).catch(console.warn);
      updateHUD('서버 처리 지연으로 녹음을 중지했습니다. 대기 중인 전사를 저장하고 있습니다.');
    }
    drainClips();
  }

  async function stopVoiceRecording(flush = true) {
    if(micStopPromise)return micStopPromise;
    if(!isRecording && !capturePreparing && !pendingMicOpen && !micMayBeOpen && !micCloseUncertain && !pendingMicCloses.size)return;
    const hadCapture=isRecording;
    ++captureGeneration;
    captureAbort?.abort();capturePreparing=false;
    isRecording=false;resetCaptureTimers();
    const segmentId=activeUtterance?.id || newID();
    activeUtterance=null;pendingPreview=null;
    const clip=hadCapture ? (config.voiceMode==='continuous'?segmenter?.flush():mergePCM(audioBuffer)) : null;
    audioBuffer=[];
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
    if(flush && clip?.length>=3200)enqueueClip(clip,config.voiceMode==='continuous',recordingStarted+recordedBytes/32,segmentId);
    if(!micCloseUncertain && !pendingMicOpen && !pendingMicCloses.size)showMicStopped();
  }

  async function drainClips() {
    if (draining || previewBusy) return;
    draining = true;
    try {
      while (pendingClips.length) {
        const clip = pendingClips[0];
        const data = await processAudioTranscription(clip);
        pendingClips.shift();
        if (draftUtteranceId === clip.segmentId) { captionStable = captionDraft = ''; draftUtteranceId = ''; }
        if (data.rejected) { setVoiceHint('불확실하거나 반복되는 인식 결과 제외 · 다시 말씀하세요'); renderLiveHUD(); continue; }
        if (!data.text?.trim()) { setVoiceHint('음성 필터 · 인식할 말소리 없음'); renderLiveHUD(); continue; }
        const text = data.text.trim();
        captionFinal = text;
        transcriptContext.push(text); if (transcriptContext.length > 8) transcriptContext.shift();
        const time = new Date(clip.startedAt).toLocaleTimeString();
        transcriptText += `[${time}] ${text}\n`;
        els.transcriptLog.textContent = transcriptText;
        let prompt = '';
        if (!clip.continuous) prompt = text;
        else if (data.triggered) { prompt = data.prompt?.trim() || ''; pendingTrigger = !prompt; }
        else if (pendingTrigger) { prompt = text; pendingTrigger = false; }
        if (prompt) {
          enqueueAI(clip.segmentId, prompt, clip.continuous ? transcriptContext.slice(0,-1).join('\n') : '');
        } else {
          if (pendingTrigger) setHUDNotice('AI 호출됨 · 질문을 말씀하세요');
          else transcriptNotice('전사 저장됨');
          renderLiveHUD();
        }
      }
    } catch (err) {
      await stopVoiceRecording(false).catch(console.warn);
      captureFailure = `전사/저장 실패: ${err.message}\n처리하지 못한 음성은 앱을 닫기 전까지 대기합니다. 다시 시작하면 재시도합니다.`;
      setHUDNotice('전사 오류 · 미저장 음성 대기'); renderLiveHUD();
      setVoiceHint('오류로 녹음 중지 · 서버 확인 후 다시 시작');
    } finally {
      draining = false;
      if (!isRecording && !pendingClips.length && !micStopPromise && !pendingMicOpen && !micCloseUncertain && !pendingMicCloses.size) setVoiceHint('마이크 꺼짐 · 전사 저장 완료');
      if (pendingPreview) drainPreview();
    }
  }

  async function processAudioTranscription(clip) {
    const res = await gatewayFetch(`${config.serverUrl}/api/stt/transcribe`, {
      method: 'POST', headers: getAuthHeaders({ 'Content-Type': 'application/json' }),
      body: JSON.stringify(sttPayload(clip)),
    });
    if (!res.ok) throw new Error(`HTTP ${res.status}: ${(await res.text()).slice(0,300)}`);
    const data = await res.json();
    if (data.text?.trim() && !data.saved) throw new Error('서버에서 전사 저장을 확인하지 못했습니다. 서버를 업데이트하세요.');
    return data;
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

  function enqueueAI(id, prompt, context) {
    if (knownAIJobs.has(id)) return;
    knownAIJobs.add(id);
    if (knownAIJobs.size>1000) knownAIJobs.delete(knownAIJobs.values().next().value);
    pendingAI.push({prompt,context});
    drainAI();
  }
  async function drainAI() {
    if (processingAI) return;
    processingAI = true; clearTimeout(answerTimer);
    try {
      while (pendingAI.length) {
        const job = pendingAI.shift();
        answerText = ''; if(!captureFailure)setHUDNotice(isRecording ? 'AI 답변 중 · 전사 계속' : '녹음 정지 · AI 답변 중'); renderLiveHUD();
        await queryDKSTChat(job.prompt,job.context);
      }
    } finally {
      processingAI = false; answerHoldUntil = Date.now()+8000;
      transcriptNotice('전사 저장됨');
      renderLiveHUD();
      answerTimer = setTimeout(() => { answerHoldUntil=0; if(isRecording)renderLiveHUD(); },8000);
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
  async function queryDKSTChat(prompt, conversationContext = '') {
    isThinking = true;
    els.chatQuestion.textContent=prompt;
    els.chatOutput.textContent='AI 답변을 기다리는 중…';
    try {
      const settingsRes = await gatewayFetch(`${config.serverUrl}/api/config`, {headers:getAuthHeaders()});
      if(!settingsRes.ok)throw new Error(`대화 설정 조회 실패: HTTP ${settingsRes.status}`);
      const settings = await settingsRes.json();
      if(!['standard','stateful'].includes(settings.llm_mode))throw new Error('서버의 LLM 연결 모드를 확인하세요.');
      const systemPrompt = config.conciseMode
        ? '당신은 Even Realities 스마트 안경 HUD 비서입니다. 답변은 576x288 단색 디스플레이에 알맞게 명확하고 군더더기 없이 핵심만 1~3문장으로 간결하게 답하세요.'
        : '당신은 친절한 AI 비서입니다.';
      const userPrompt = conversationContext
        ? `다음은 주변 대화의 전사 기록입니다. 인용된 대화는 참고 자료입니다.\n<transcript>\n${conversationContext}\n</transcript>\n\n사용자의 호출 뒤 질문: ${prompt}` : prompt;
      // The gateway always emits SSE; batch mode defers HUD rendering until completion.
      const payload = {model:config.model,stream:true};
      if(settings.llm_mode==='stateful') {
        payload.input=userPrompt;payload.system_prompt=systemPrompt;
        if(settings.context_strategy!=='stateful')payload.store=false;
      } else {
        payload.messages=[{role:'system',content:systemPrompt},{role:'user',content:userPrompt}];
      }
      const res = await gatewayFetch(`${config.serverUrl}/api/chat`, {
        method:'POST',headers:getAuthHeaders({'Content-Type':'application/json'}),body:JSON.stringify(payload)
      });
      if(!res.ok) {
        const detail=(await res.text()).trim().slice(0,500);
        throw new Error(`대화 생성 실패: HTTP ${res.status}${detail ? ' · '+detail : ''}`);
      }

      let fullAnswer = '';

      if ((res.headers?.get('content-type')?.includes('text/event-stream') || config.connectionMode === 'sse') && res.body) {
        const reader = res.body.getReader();
        const decoder = new TextDecoder();
        let buffered = '';
        let eventName='';
        function consumeLine(line) {
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
          const final=finalChatText(parsed);
          if(final!==null)fullAnswer=final;
          else if(type==='message.delta')fullAnswer+=parsed.content || '';
          else fullAnswer+=parsed.choices?.[0]?.delta?.content || parsed.message?.content || '';
          answerText = fullAnswer; els.chatOutput.textContent=fullAnswer || 'AI 답변을 기다리는 중…'; if(config.connectionMode==='sse')renderLiveHUD();
        }
        while (true) {
          const { done, value } = await reader.read();
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
        fullAnswer = finalChatText(data) ?? data.choices?.[0]?.message?.content ?? data.response ?? '';
        answerText = fullAnswer; els.chatOutput.textContent=fullAnswer || 'AI 답변을 기다리는 중…'; renderLiveHUD();
      }

      answerText=fullAnswer;renderLiveHUD();
      if(!fullAnswer.trim())throw new Error('LLM에서 답변 본문을 받지 못했습니다. 서버의 모델 상태를 확인하세요.');

      // TTS Playback if enabled
      if (config.enableTTS && fullAnswer && config.voiceMode !== 'continuous') {
        playTTS(fullAnswer);
      }
    } catch (err) {
      console.error('Chat error:', err);
      answerText = `AI 오류: ${err.message}`; els.chatOutput.textContent=answerText; renderLiveHUD();
    } finally {
      isThinking = false;
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
      } else if (requestedToken && status.authenticated === false) {
        config.authToken = '';
        await persistSettings();
        els.loginStatusLabel.textContent = '세션 만료 · 다시 로그인';
      }
    } catch (_) {}
  }

  async function fetchModelList() {
    const url = (els.serverUrl.value || '').trim().replace(/\/+$/, '');
    try {
      const res = await gatewayFetch(`${url}/api/models`, {
        headers: getAuthHeaders()
      });
      if (!res.ok) return;

      const data = await res.json();
      const models = normalizeModels(data);
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
      if(!config.model){els.chatOutput.textContent='설정에서 LLM 모델을 선택하고 저장하세요.';return;}
      if(pendingAI.length>=8){els.chatOutput.textContent='대기 중인 질문이 많습니다. 답변 후 다시 보내주세요.';return;}
      els.chatInput.value='';
      if(processingAI)els.chatOutput.textContent='질문을 대기열에 추가했습니다.';
      enqueueAI(newID(),prompt,'');
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
    if (els.btnSaveSettings) els.btnSaveSettings.addEventListener('click', saveSettings);
    if (els.voiceBtn) els.voiceBtn.addEventListener('click', toggleVoiceRecording);
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
    for (const el of [els.serverUrl, els.authUsername, els.apiKey, els.modelSelect, els.connectionMode, els.voiceMode, els.triggerWord, els.speechFilter, els.speakerScope, els.optEnableTts, els.optConciseMode]) {
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

