(function(global){
 'use strict';
 const EN={"추가 LLM 키 확인 중…":"Loading additional LLM keys…","등록된 추가 LLM 연결이 없습니다.":"No additional LLM connections configured.","API 키":"API key","개인 키 입력 · 없으면 서버 기본 키 사용":"Personal key · uses the server key if unset","개인 키 삭제":"Remove personal key","추가 LLM 키 저장":"Save additional LLM keys","저장되었습니다.":"Saved.","저장 실패":"Save failed","추가 LLM 키를 불러오지 못했습니다.":"Could not load additional LLM keys.","연결 확인 중…":"Loading connections…","등록된 추가 LLM이 없습니다.":"No additional LLM connections.","연결 삭제":"Delete connection","삭제":"Delete","삭제되었습니다.":"Deleted.","삭제 실패":"Delete failed","추가 연결은 최대 8개까지 등록할 수 있습니다.":"You can add up to 8 connections.","LLM 연결 편집":"Edit LLM connection","LLM 추가":"Add LLM","연결 이름":"Connection name","API 주소":"API endpoint","예: 사내 LLM, OpenAI":"For example: Office LLM, OpenAI","API 방식":"API format","OpenAI 호환":"OpenAI compatible","서버 기본 API 키 (선택)":"Server API key (optional)","계정별 키를 사용하면 비워 두세요":"Leave blank to use per-account keys","LM Studio 모델 자동 로딩":"Automatically load LM Studio models","서버 기본 API 키 삭제":"Remove server API key","LLM 연결을 불러오지 못했습니다.":"Could not load LLM connections.","추가 LLM 연결은 최대 8개입니다":"You can add up to 8 connections.","연결 ID가 잘못되었거나 중복되었습니다":"Invalid or duplicate connection ID.","연결 주소는 http(s) API 주소로 입력하세요":"Enter an HTTP(S) API endpoint.","연결 이름은 1~80자입니다":"Connection names must be 1–80 characters.","계정을 찾을 수 없습니다":"Account not found.","로그인이 필요합니다":"Sign in to continue.","연결 수가 너무 많습니다":"Too many connections.","등록되지 않은 연결 또는 잘못된 키":"Unknown connection or invalid API key."};
 const tr=text=>(document.documentElement?.lang || 'ko').startsWith('en')?(EN[text] || text):text;
 function setText(e,text){e.textContent=tr(text);if(EN[text])e.dataset.llmI18n=text;return e;}
 const node=(tag,text,translate=true)=>{const e=document.createElement(tag);if(text!==undefined){if(tag==='label' && translate){e.appendChild(setText(document.createElement('span'),text));}else if(translate)setText(e,text);else e.textContent=text;}return e;};
 function setFailure(e,prefix,message){delete e.dataset.llmI18n;e.textContent=tr(prefix)+' · '+tr(message);}
 function placeholder(input,text){input.placeholder=tr(text);input.dataset.llmPlaceholder=text;}
 function secretPlaceholder(input,configured,emptyText,length){
  input.style.setProperty('--password-dot-count', Number.isSafeInteger(length) && length>0?length:12);
  input.setAttribute('data-configured-secret',String(!!configured));
  placeholder(input,configured?'••••••••••••':emptyText);
 }
 function refreshI18n(){
  if(!document.querySelectorAll)return;
  document.querySelectorAll('[data-llm-i18n]').forEach(e=>{e.textContent=(e.dataset.llmPrefix?e.dataset.llmPrefix+' ':'')+tr(e.dataset.llmI18n);});
  document.querySelectorAll('[data-llm-placeholder]').forEach(e=>{e.placeholder=tr(e.dataset.llmPlaceholder);});
  document.querySelectorAll('[data-llm-title]').forEach(e=>{e.title=tr(e.dataset.llmTitle);e.setAttribute('aria-label',(e.dataset.llmAriaPrefix?e.dataset.llmAriaPrefix+' ':'')+tr('삭제'));});
 }
 if(global.MutationObserver && document.documentElement)new global.MutationObserver(refreshI18n).observe(document.documentElement,{attributes:true,attributeFilter:['lang']});
 function field(parent,label,type,value){const group=node('label');group.className='form-group';group.style.cssText='display:grid;gap:6px;margin:10px 0';const caption=node('span',Array.isArray(label)?label[1]:label);caption.style.cssText='font-size:12px;color:var(--text-secondary)';if(Array.isArray(label)){caption.dataset.llmPrefix=label[0];caption.textContent=label[0]+' '+tr(label[1]);}group.appendChild(caption);const input=node('input');input.type=type;input.value=value || '';input.autocomplete=type==='password'?'new-password':'off';input.style.width='100%';group.appendChild(input);parent.appendChild(group);return input;}
 async function mountKeys(root,{load,save,onSaved=()=>{},hideWhenEmpty=false}){
  if(!root)return;const generation=(root.llmKeysGeneration || 0)+1;root.llmKeysGeneration=generation;root.saveConnectionKeys=null;root.hidden=false;root.style.display='grid';root.dataset.loading='true';root.replaceChildren(node('p','추가 LLM 키 확인 중…'));
  try{
   const result=await load();if(root.llmKeysGeneration!==generation)return;const rows=Array.isArray(result)?result:result.connections || [];root.replaceChildren();
   if(!rows.length){if(hideWhenEmpty){root.hidden=true;root.style.display='none';}else root.appendChild(node('p','등록된 추가 LLM 연결이 없습니다.'));return;}
   const fields=[];
   for(const row of rows){const input=field(root,[row.name,'API 키'],'password','');secretPlaceholder(input,row.has_api_key,'개인 키 입력 · 없으면 서버 기본 키 사용',row.api_key_length);const clear=node('input');clear.type='checkbox';clear.style.width='auto';const label=node('label');label.style.cssText='display:flex;align-items:center;gap:8px;margin:0';label.append(clear,node('span','개인 키 삭제'));root.appendChild(label);fields.push({id:row.id,input,clear});}
   const status=node('p'),button=node('button','추가 LLM 키 저장');button.type='button';button.className='btn btn-secondary';root.append(button,status);
   root.saveConnectionKeys=async()=>{const keys={};for(const f of fields){if(f.clear.checked)keys[f.id]='';else if(f.input.value.trim())keys[f.id]=f.input.value.trim();}if(!Object.keys(keys).length)return;await save(keys);for(const f of fields){if(Object.hasOwn(keys,f.id))secretPlaceholder(f.input,!!keys[f.id],'개인 키 입력 · 없으면 서버 기본 키 사용',Array.from(keys[f.id]).length);f.input.value='';f.clear.checked=false;}setText(status,'저장되었습니다.');};
   button.onclick=async()=>{button.disabled=true;try{await root.saveConnectionKeys();await onSaved();}catch(e){setFailure(status,'저장 실패',e.message);}finally{button.disabled=false;}};
  }catch(e){if(root.llmKeysGeneration===generation)root.replaceChildren(node('p',tr('추가 LLM 키를 불러오지 못했습니다.')+' '+tr(e.message)));}finally{if(root.llmKeysGeneration===generation)root.dataset.loading='false';}
 }
 async function mountConnections(root,{load,save,editor,confirmRemove=async()=>true}){
  if(!root || !editor)return;const generation=(root.llmConnectionGeneration || 0)+1;root.llmConnectionGeneration=generation;
  root.replaceChildren(node('p','연결 확인 중…'));root.addConnection=null;let busy=false;
  try{
   let rows=(await load()) || [];if(root.llmConnectionGeneration!==generation)return;const list=node('div'),status=node('p');list.className='user-list';status.style.cssText='color:var(--text-secondary);font-size:13px;margin:8px 0 0';root.replaceChildren(list,status);
   const render=()=>{
    list.replaceChildren();
    if(!rows.length){const empty=node('p','등록된 추가 LLM이 없습니다.');empty.style.cssText='color:var(--text-secondary);padding:12px 0;margin:0';list.appendChild(empty);}
    for(const row of rows){
     const item=node('div');item.className='user-item';item.style.cssText='display:flex;align-items:center;gap:12px;padding:14px 0';
     const edit=node('button');edit.type='button';edit.style.cssText='flex:1;min-width:0;background:none;border:0;padding:0;text-align:left;color:inherit;cursor:pointer';
     const name=node('div',row.name,false),address=node('div',row.endpoint,false);name.style.cssText='font-weight:600;font-size:14px';address.style.cssText='font-size:12px;color:var(--text-secondary);overflow:hidden;text-overflow:ellipsis;white-space:nowrap;margin-top:4px';edit.append(name,address);edit.onclick=()=>openEditor(row);
     const remove=node('button');remove.type='button';remove.className='icon-btn';remove.title=tr('연결 삭제');remove.dataset.llmTitle='연결 삭제';remove.dataset.llmAriaPrefix=row.name;remove.setAttribute('aria-label',row.name+' '+tr('삭제'));const icon=node('span','delete');icon.className='material-icons-round';remove.appendChild(icon);
     remove.onclick=async()=>{if(busy || !await confirmRemove(row))return;busy=true;remove.disabled=true;try{await save(rows.filter(r=>r.id!==row.id));rows=await load();setText(status,'삭제되었습니다.');render();}catch(e){setFailure(status,'삭제 실패',e.message);}finally{busy=false;remove.disabled=false;}};
     item.append(edit,remove);list.appendChild(item);
    }
   };
   function openEditor(existing){
    if(busy)return;
    if(!existing && rows.length>=8){setText(status,'추가 연결은 최대 8개까지 등록할 수 있습니다.');return;}
    const row=existing || {id:'llm_'+crypto.randomUUID().replace(/-/g,''),name:'',endpoint:'',mode:'standard'};
    const form=editor.querySelector('#llm-connection-editor');form.replaceChildren();setText(editor.querySelector('#llm-connection-edit-title'),existing?'LLM 연결 편집':'LLM 추가');
    const name=field(form,'연결 이름','text',row.name),endpoint=field(form,'API 주소','url',row.endpoint);placeholder(name,'예: 사내 LLM, OpenAI');endpoint.placeholder='https://api.example.com/v1';
    const group=node('label','API 방식');group.style.cssText='display:grid;gap:6px;margin:12px 0';const mode=node('select');for(const [value,label] of [['standard','OpenAI 호환'],['stateful','LM Studio Native']]){const option=node('option',label);option.value=value;mode.appendChild(option);}mode.value=row.mode || 'standard';group.appendChild(mode);form.appendChild(group);
    const key=field(form,'서버 기본 API 키 (선택)','password','');secretPlaceholder(key,row.has_api_key,'계정별 키를 사용하면 비워 두세요',row.api_key_length);
    const manage=node('input');manage.type='checkbox';manage.checked=row.manage_models===true;manage.style.width='auto';const manageLabel=node('label');manageLabel.style.cssText='display:flex;gap:8px;align-items:center;margin:16px 0';manageLabel.append(manage,node('span','LM Studio 모델 자동 로딩'));form.appendChild(manageLabel);
    const clear=node('input');clear.type='checkbox';clear.style.width='auto';if(row.has_api_key){const clearLabel=node('label');clearLabel.style.cssText='display:flex;gap:8px;align-items:center;margin:12px 0';clearLabel.append(clear,node('span','서버 기본 API 키 삭제'));form.appendChild(clearLabel);}
    const error=node('p');error.style.cssText='color:var(--danger,#f85149);font-size:13px;margin-bottom:0';form.appendChild(error);editor.style.display='flex';name.focus();
    root.saveConnectionEditor=async()=>{
     if(busy)return;busy=true;const button=editor.querySelector('#llm-connection-editor-save');button.disabled=true;
     try{
      const updated={id:row.id,name:name.value.trim(),endpoint:endpoint.value.trim(),mode:mode.value,manage_models:manage.checked};if(clear.checked)updated.api_key='';else if(key.value.trim())updated.api_key=key.value.trim();
      const next=existing?rows.map(r=>r.id===row.id?updated:r):[...rows,updated];await save(next);rows=await load();render();editor.style.display='none';setText(status,'저장되었습니다.');
     }catch(e){setFailure(error,'저장 실패',e.message);}finally{busy=false;button.disabled=false;}
    };
   }
   root.addConnection=()=>openEditor(null);render();
  }catch(e){if(root.llmConnectionGeneration===generation)root.replaceChildren(node('p',tr('LLM 연결을 불러오지 못했습니다.')+' '+tr(e.message)));}
 }
 async function jsonResponse(response){if(!response.ok)throw Error((await response.text()).slice(0,200));return response.json();}
 global.DKSTConnectionUI={mountKeys,mountConnections,jsonResponse,refreshI18n,tr};
})(window);
