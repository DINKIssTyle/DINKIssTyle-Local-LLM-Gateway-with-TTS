(function (global) {
 'use strict';
 async function mount(root, {userId, api, language = () => document.documentElement.lang}) {
  root.disposeGoogle?.();
  let active = true, timer, status, busy = false;
  root.disposeGoogle = () => { active = false; clearTimeout(timer); root.saveGoogleSettings = null; };
  root.replaceChildren(); root.saveGoogleSettings = null;
  const tr = (ko, en) => language() === 'ko' ? ko : en;
  const el = (tag, text) => { const e = document.createElement(tag); if (text) e.textContent = text; return e; };
  const labels = [];
  const label = (tag, ko, en) => { const e = el(tag, tr(ko, en)); labels.push(() => e.textContent = tr(ko, en)); return e; };
  root.refreshGoogleLanguage = () => { labels.forEach(f => f()); if (status) showStatus(); };
  const heading = label('div', 'Google 연동', 'Google integration'); heading.style.cssText = 'font-weight:600;margin-bottom:12px'; root.append(heading);
  const help = label('p', '선택한 게이트웨이 계정에서 검색·읽기만 허용합니다. 체크 변경은 즉시 저장됩니다. 서비스를 추가하면 Google 권한을 업데이트하세요.', 'Allows search and reading for this gateway account. Checkbox changes save immediately. Update Google permissions after adding services.'); help.style.cssText = 'font-size:12px;color:var(--text-secondary);line-height:1.5'; root.append(help);
  const state = el('p'); state.style.cssText = 'font-size:12px;white-space:pre-wrap'; state.setAttribute('role', 'status'); state.setAttribute('aria-live', 'polite'); root.append(state);
  const checks = {};
  for (const [id, name] of [['gmail','Gmail'],['drive','Google Drive'],['calendar','Google Calendar']]) {
   const row = el('label'); row.style.cssText = 'display:flex;align-items:center;gap:8px;margin:8px 0;cursor:pointer';
   const input = el('input'); input.type = 'checkbox'; input.style.cssText = 'width:18px;height:18px'; input.disabled = true;
   checks[id] = input; row.append(input, el('span', name)); root.append(row);
  }
  const details = el('details'); details.style.cssText = 'margin:16px 0;font-size:12px';
  details.append(label('summary', 'Google OAuth 클라이언트 설정', 'Google OAuth client configuration'));
  const setup = label('p', 'Google Cloud에서 세 API를 사용 설정하고 OAuth 동의 화면과 데스크톱 앱 클라이언트를 만드세요. 테스트 모드에서는 연결할 Google 계정을 테스트 사용자로 등록하세요.', 'Enable all three APIs in Google Cloud, configure OAuth consent, and create a Desktop app client. In testing mode, add the Google account as a test user.'); setup.style.lineHeight = '1.5'; details.append(setup);
  const link = el('a', 'Google OAuth'); link.href = 'https://developers.google.com/identity/protocols/oauth2/native-app'; link.target = '_blank'; link.rel = 'noopener noreferrer'; link.style.color='var(--accent-color, #58a6ff)'; details.append(link);
  const field = (ko, en, type) => { const l = el('label'); l.className='form-group'; const title=label('span', ko, en); title.style.cssText='display:block;margin-bottom:6px'; l.append(title); l.style.cssText='display:block;margin-top:12px'; const input=el('input');input.type=type;input.autocomplete='off';input.disabled=true;l.append(input);details.append(l);return input; };
  // Field labels use child spans so language changes preserve input elements.
  const clientID = field('클라이언트 ID', 'Client ID', 'text');
  const secret = field('클라이언트 보안 비밀번호', 'Client secret', 'password');
  const secretHelp = label('p', '등록 후 비워 두면 기존 값을 유지합니다.', 'After configuration, leave blank to keep the stored value.'); details.append(secretHelp); root.append(details);
  const actions = el('div'); actions.style.cssText = 'display:flex;flex-wrap:wrap;gap:8px'; root.append(actions);
  const error = el('p'); error.style.cssText = 'font-size:12px;color:var(--text-secondary);white-space:pre-wrap'; error.setAttribute('role','alert'); root.append(error);
  const buttons = [];
  const button = (ko,en,fn) => { const b = label('button',ko,en); b.type='button';b.className='btn btn-secondary'; b.disabled=true; b.onclick=()=>run(fn);buttons.push(b);actions.append(b);return b; };
  const services = () => Object.keys(checks).filter(k => checks[k].checked);
  function showStatus() {
   const missing = status.services.filter(s => !status.authorized_services.includes(s));
   state.textContent = status.pending ? tr('브라우저에서 Google 로그인을 완료하세요.','Complete Google sign-in in your browser.') : status.connected ? tr('연결됨: ','Connected: ') + status.email : tr('연결되지 않음','Not connected');
   if (status.connected && missing.length) state.textContent += '\n' + tr('권한 업데이트 필요: ','Permission update required: ') + missing.join(', ');
   if (status.message) state.textContent += '\n' + status.message;
   disconnect.disabled = busy || (!status.connected && !status.pending);
  }
  function setBusy(value) { busy=value; [...Object.values(checks),clientID,secret,...buttons].forEach(e=>e.disabled=value); if(status)showStatus(); buttons.forEach(b=>b.style.opacity=b.disabled?'0.5':'1'); }
  async function refresh() { const next=await api.GetUserGoogleStatus(userId); if(!active)return; status=next; showStatus(); }
  async function save() {
   if (!active || !status) throw new Error(tr('Google 설정을 먼저 불러와야 합니다.','Google settings must finish loading first.'));
   await api.SetUserGoogleSettings(userId,{client_id:clientID.value.trim(),client_secret:secret.value.trim(),services:services()});
   if (!active)return; secret.value=''; await refresh();
  }
  async function run(fn) {
   if(!active || busy)return; setBusy(true);error.textContent='';
   try { await fn(); } catch(e) { if(active){error.textContent=String(e?.message || e); if(status)Object.entries(checks).forEach(([k,input])=>input.checked=status.services.includes(k));} }
   finally {if(active)setBusy(false);}
  }
  button('설정 저장','Save configuration',save);
  button('Google 연결 / 권한 업데이트','Connect / update permissions',async()=>{await save();if(!active)return;await api.ConnectUserGoogle(userId);await refresh();schedule();});
  const disconnect = button('연결 해제','Disconnect',async()=>{await api.DisconnectUserGoogle(userId);await refresh();});
  Object.values(checks).forEach(input=>input.onchange=()=>run(save));
  function schedule(){clearTimeout(timer);if(active && status?.pending)timer=setTimeout(async()=>{try{await refresh();}catch(e){if(active)error.textContent=String(e?.message || e);}schedule();},1500);}
  try {
   await refresh();if(!active)return;
   clientID.value=status.client_id || '';Object.entries(checks).forEach(([k,input])=>input.checked=status.services.includes(k));
   details.open=!status.client_id;secret.placeholder=status.has_client_secret?tr('등록됨 · 비워 두면 유지','Configured · leave blank to keep'):'';
   root.saveGoogleSettings=async()=>{if(busy)throw new Error(tr('Google 작업이 진행 중입니다. 잠시 후 저장하세요.','A Google operation is in progress. Save again when it finishes.'));await runSave();};
   async function runSave(){setBusy(true);try{await save();}finally{if(active)setBusy(false);}}
   setBusy(false);schedule();
  } catch(e) {if(active)error.textContent=String(e?.message || e);}
 }
 global.DKSTGoogleUI={mount};
})(window);
