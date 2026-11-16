import {normalizeGlassesMode,targetLanguageLabel} from './glasses-language.mjs';
export {TARGET_LANGUAGE_LABELS,TARGET_LANGUAGE_SHORT,normalizeGlassesMode,targetLanguageLabel} from './glasses-language.mjs';
import { getTextWidth } from '@evenrealities/pretext';

export function clockText(date = new Date()) {
  return `${String(date.getHours()).padStart(2, '0')}:${String(date.getMinutes()).padStart(2, '0')}`;
}

export function glassesBatteryText(level) {
  return typeof level === 'number' && Number.isFinite(level) && level >= 0 && level <= 100
    ? `${Math.round(level)}%` : '--';
}

export function hudStatusText(level, date = new Date()) {
  return `${clockText(date)} | ${glassesBatteryText(level)}`;
}

// The native SDK exposes no text alignment. Pad using the same firmware glyph
// metrics as HUD wrapping, inside the 176 px status container.
export function rightAlignedStatusText(level,date=new Date(),width=176){
  const text=hudStatusText(level,date);
  let padding='';
  while(getTextWidth(padding+' '+text)<=width-2)padding+=' ';
  return padding+text;
}

export function hudNoticeText(message,width=396) {
  const text=String(message).replace(/\s+/gu,' ').trim();
  if(getTextWidth(text)<=width)return text;
  const chars=Array.from(text);let lo=0,hi=chars.length;
  while(lo<hi){const mid=Math.ceil((lo+hi)/2);if(getTextWidth(chars.slice(0,mid).join('')+'…')<=width)lo=mid;else hi=mid-1;}
  return chars.slice(0,lo).join('').trimEnd()+'…';
}

export function modeHeaderText({mode='ai',hint=true,source='en',target='ko',running=false,listening=running,phase='',phaseDetail='',translating=false,error='',detail='',trigger='',imageIcons=false,messageOnly=false}={}){
  detail=error || detail || (listening?'듣는중':'-');
  if(!error && translating)detail='번역 처리';
  else if(!error && phase){
    const cleanPhase=String(phase).replace(/[.…]+\s*$/u,'').trim();
    const match=cleanPhase.match(/(\d{1,3})%/u);
    const pct=match?` ${Math.min(100,Number(match[1]))}%`:'';
    const secMatch=cleanPhase.match(/(\d+)\s*초/u);
    const sec=secMatch?` ${secMatch[1]}초`:'';
    if(/프롬프트/u.test(cleanPhase))detail='프롬프트 처리'+pct;
    else if(/모델.*(?:로딩|로드)/u.test(cleanPhase))detail='모델 로딩'+pct;
    else if(/번역/u.test(cleanPhase))detail='번역 처리';
    else if(/네이버\s*검색/u.test(cleanPhase))detail='네이버 검색중';
    else if(/나무위키/u.test(cleanPhase))detail='나무위키 검색중';
    else if(/웹\s*페이지\s*읽/u.test(cleanPhase))detail='웹페이지 읽는중';
    else if(/소스\s*읽/u.test(cleanPhase))detail='소스 읽는중';
    else if(/메모리\s*검색|기억/u.test(cleanPhase))detail='기억 조회중';
    else if(/메모리\s*읽/u.test(cleanPhase))detail='메모리 읽는중';
    else if(/메모리\s*삭제/u.test(cleanPhase))detail='메모리 삭제중';
    else if(/명령어\s*실행/u.test(cleanPhase))detail='명령어 실행중';
    else if(/시간/u.test(cleanPhase))detail='시간 확인';
    else if(/위치/u.test(cleanPhase))detail='위치 확인';
    else if(/스킬/u.test(cleanPhase))detail=phaseDetail?`스킬: ${phaseDetail}`:'스킬 적용';
    else if(/도구\s*실행\s*실패/u.test(cleanPhase))detail='도구 실패';
    else if(/도구\s*실행.*완료/u.test(cleanPhase))detail='도구 완료';
    else if(/도구\s*실행\s*중/u.test(cleanPhase))detail=cleanPhase.replace(/^도구\s*실행\s*중\s*/u,'도구: ') || '도구 실행중';
    else if(/웹|검색/u.test(cleanPhase))detail='웹검색중';
    else if(/생각/u.test(cleanPhase))detail=/완료|동안/u.test(cleanPhase)?'생각 완료':'생각중'+sec;
    else if(/답변.*(?:작성|생성)/u.test(cleanPhase))detail='답변 생성';
    else if(/지식|정보/u.test(cleanPhase))detail='정보 조회중';
    else detail=cleanPhase;
    if(phaseDetail && /검색중|읽는중|조회중|실행중/u.test(detail)){
      const compactDetail=String(phaseDetail).replace(/^(?:검색어|페이지 열기|소스 읽기|메모리 검색|명령어 실행)\s*:\s*/u,'').replace(/\s+/gu,' ').trim();
      if(compactDetail)detail+=` (${compactDetail.slice(0,18)}${compactDetail.length>18?'…':''})`;
    }
  }
  if(messageOnly)return `| ${detail}`;
  const runIcon=imageIcons?'':running?'●':'○';
  const label=mode==='translate'?`[번역] ${String(source).toUpperCase()}→${String(target).toUpperCase()}`:(trigger?`[AI ${imageIcons?'':'▶'}${trigger}]`:'[AI]');
  return `${label}${runIcon?' '+runIcon:''} | ${detail}`;
}

export function translationHeaderText(options={}){
  return modeHeaderText({...options,mode:'translate'});
}

export function animatedDots(step = 0) {
  const count = ((Math.max(0, Math.trunc(step)) % 3) + 1);
  return '.'.repeat(count);
}

export function withAnimatedDots(label, step = 0) {
  const base = String(label || '생각 중').replace(/[.…]+\s*$/u, '').trim();
  return `${base}${animatedDots(step)}`;
}

export function extractVisibleAndReasoning(rawText) {
  const source = String(rawText || '');
  if (!source) {
    return { visibleText: '', reasoningText: '', isInlineReasoning: false };
  }

  let reasoningText = '';
  let isInlineReasoning = false;

  const thinkOpen = source.lastIndexOf('<think>');
  const thinkClose = source.lastIndexOf('</think>');
  if (thinkOpen !== -1 && thinkOpen > thinkClose) {
    isInlineReasoning = true;
    reasoningText = source.slice(thinkOpen + 7).trim();
  } else if (thinkOpen !== -1 && thinkClose > thinkOpen) {
    reasoningText = source.slice(thinkOpen + 7, thinkClose).trim();
  }

  if (source.includes('<|channel|>analysis') && !source.includes('<|channel|>final')) {
    isInlineReasoning = true;
    const parts = source.split('<|channel|>analysis');
    reasoningText = parts[parts.length - 1].split('<|channel|>')[0].trim();
  }

  const gemmaStartRegex = /<\|channel(?:\||>)?\s*thought/i;
  const gemmaEndRegex = /<channel\|>|<\/channel\|>|<\|channel(?:\||>)?\s*(?:message|final|instruction|response)/i;
  if (gemmaStartRegex.test(source)) {
    const startMatch = source.match(gemmaStartRegex);
    const remaining = source.slice(startMatch.index + startMatch[0].length);
    const endMatch = remaining.match(gemmaEndRegex);
    if (!endMatch) {
      isInlineReasoning = true;
      reasoningText = remaining.trim();
    } else {
      reasoningText = remaining.slice(0, endMatch.index).trim();
    }
  }

  let visible = source
    .replace(/<think\b[^>]*>[\s\S]*?<\/think\s*>/gi, '')
    .replace(/<think\b[^>]*>[\s\S]*$/gi, '')
    .replace(/<\|channel\|>analysis[\s\S]*?(?=<\|channel\|>final|$)/gi, '')
    .replace(/<\|channel(?:\||>)?\s*thought[\s\S]*?(?:<channel\|>|<\/channel\|>|<\|channel(?:\||>)?\s*(?:message|final|instruction|response)|$)/gi, '')
    .replace(/<\|channel(?:\||>)?\s*(?:analysis|final|message|instruction|response)\b[^>]*>?/gi, '')
    .replace(/<\/?channel\|>/gi, '');

  visible = stripToolTagsForGlasses(visible).replace(/<\|end\|>/gi, '');

  return {
    visibleText: visible.trim(),
    reasoningText,
    isInlineReasoning,
  };
}

export function stripToolTagsForGlasses(text) {
  return String(text || '')
    // Closed and streaming <tool_call>, <tool_code>, <function=...>, <parameter=...>
    .replace(/<(?:tool_call|tool_code|tool_use|tool_result|function_call)\b[^>]*>[\s\S]*?(?:<\/(?:tool_call|tool_code|tool_use|tool_result|function_call)\s*>|$)/gi, '')
    .replace(/<\/(?:tool_call|tool_code|tool_use|tool_result|function_call)\s*>/gi, '')
    // Closed and streaming <|tool_call|>, <|tool_code|>, <|call:...|>, <|tool_response|>
    .replace(/<\|(?:tool_call|tool_code|tool_response|call:[^|>]*)\|?>[\s\S]*?(?:<\|\/?(?:tool_call|tool_code|tool_response|call|end)\|>|$)/gi, '')
    .replace(/<\|\/?(?:tool_call|tool_code|tool_response|call(?::[^|>]*)?)\|?>/gi, '')
    // Closed and streaming snake_case tool tags like <search_web>...</search_web>, <get_current_time>{}</get_current_time>
    .replace(/<([a-z]+_[a-z0-9_]*)\b[^>]*>[\s\S]*?(?:<\/\1\s*>|$)/gi, '')
    .replace(/<\/[a-z]+_[a-z0-9_]*\s*>/gi, '')
    // Unclosed partial tag at the very end of a stream chunk (e.g. "<tool_c" or "<search_")
    .replace(/<(?:tool_|function|parameter|\|call:|[a-z]+_[a-z0-9_]*)[^>]*$/i, '')
    .trim();
}

export function stripSourcesForGlasses(text) {
  const { visibleText } = extractVisibleAndReasoning(text);
  if (!visibleText) return '';

  let cleaned = visibleText;

  // Strip trailing source/reference sections (e.g. "검색 출처:\n- ...", "출처:\n...", "Sources:\n...")
  cleaned = cleaned.replace(
    /(?:^|\n)\s*(?:#{1,6}\s*|\*{1,2}\s*|\[)?(?:검색\s*출처|참고\s*출처|참고\s*자료|참고\s*문헌|출처|Sources?|References?)(?:\]|\*{1,2})?\s*:?\s*(?:\n[\s\S]*|$)/giu,
    ''
  );

  // Strip standalone lines starting with 출처 / 검색 출처 / 참고 자료 / Sources
  cleaned = cleaned.replace(
    /^\s*(?:[-*•]\s*)?(?:\*{1,2}|\[)?(?:검색\s*출처|참고\s*출처|참고\s*자료|참고\s*문헌|출처|Sources?|References?)(?:\]|\*{1,2})?\s*:\s*.*$/gimu,
    ''
  );

  // Strip trailing bullet list items that are purely markdown links or URLs
  cleaned = cleaned.replace(
    /(?:\n\s*[-*•]\s*(?:\[[^\]]+\]\(https?:\/\/[^)\s]+\)|https?:\/\/\S+)\s*)+$/giu,
    ''
  );

  // Replace inline markdown links [label](url): drop if label is URL/source keyword, otherwise keep label
  cleaned = cleaned.replace(/\[([^\]]+)\]\((https?:\/\/[^)\s]+)\)/giu, (_, label) => {
    const trimmed = String(label || '').trim();
    if (!trimmed) return '';
    if (/^(?:https?:\/\/|www\.)/i.test(trimmed)) return '';
    if (/^(?:출처|검색\s*출처|참고|링크|원문|기사|source|link|reference)$/iu.test(trimmed)) return '';
    return trimmed;
  });

  // Strip inline source parentheticals/brackets like (출처: ...), [출처: ...], (https://...)
  cleaned = cleaned.replace(/\s*\((?:검색\s*출처|참고\s*출처|참고|출처|Sources?|References?)\s*:[^)]+\)/giu, '');
  cleaned = cleaned.replace(/\s*\[(?:검색\s*출처|참고\s*출처|참고|출처|Sources?|References?)\s*:[^\]]+\]/giu, '');
  cleaned = cleaned.replace(/\s*\(https?:\/\/[^)\s]+\)/giu, '');
  cleaned = cleaned.replace(/\s*\[https?:\/\/[^\]\s]+\]/giu, '');

  // Remove bare URLs
  cleaned = cleaned.replace(/\s*https?:\/\/\S+/giu, '');

  // Clean up excessive blank lines and trailing whitespace
  cleaned = cleaned
    .replace(/[ \t]+\n/g, '\n')
    .replace(/\n{3,}/g, '\n\n')
    .trim();

  return cleaned;
}

function extractToolValue(args, keys) {
  if (!args || typeof args !== 'object') return '';
  for (const key of keys) {
    const val = args[key];
    if (typeof val === 'string' && val.trim()) return val.trim();
    if (typeof val === 'number' && Number.isFinite(val)) return String(val);
  }
  if (Array.isArray(args.queries)) {
    const first = args.queries.find(q => typeof q === 'string' && q.trim());
    if (first) return first.trim();
  }
  return '';
}

export function formatToolStatusText(event = {}) {
  const type = String(event.type || '');
  const rawTool = String(event.tool_name || event.tool || '').trim();
  const normalized = rawTool.toLowerCase();
  const args = event.arguments && typeof event.arguments === 'object' ? event.arguments : null;

  if (type === 'tool_call.failure') {
    const reason = String(event.reason || '알 수 없는 오류').trim();
    return { title: '도구 실행 실패', detail: reason };
  }

  if (type === 'tool_call.success') {
    if (normalized === 'get_current_time') {
      return { title: '현재 시간을 확인했습니다.', detail: '' };
    }
    if (normalized === 'get_current_location' || normalized === 'get_user_location') {
      return { title: '사용자 위치를 확인했습니다.', detail: '' };
    }
    const evidence = Array.isArray(event.evidence) ? event.evidence.filter(e => e?.url) : [];
    if (evidence.length > 0) {
      return { title: '도구 실행이 완료되었습니다.', detail: `검색 완료 · 출처 ${evidence.length}개 확인` };
    }
    return { title: '도구 실행이 완료되었습니다.', detail: '답변 작성 중' };
  }

  const queryLike = extractToolValue(args, ['query', 'keyword', 'title', 'input', 'prompt', 'text']);
  const url = extractToolValue(args, ['url']);
  const sourceId = extractToolValue(args, ['source_id']);
  const command = extractToolValue(args, ['command']);
  const memoryId = extractToolValue(args, ['memory_id']);

  switch (normalized) {
    case 'get_current_time':
      return { title: '현재 시간을 확인했습니다.', detail: '' };
    case 'get_current_location':
    case 'get_user_location':
      return { title: '사용자 위치를 확인했습니다.', detail: '' };
    case 'search_web':
    case 'search_web_multi':
    case 'web_search':
      return { title: '웹 검색 중', detail: queryLike ? `검색어: ${queryLike}` : '' };
    case 'naver_search':
      return { title: '네이버 검색 중', detail: queryLike ? `검색어: ${queryLike}` : '' };
    case 'namu_wiki':
      return { title: '나무위키 검색 중', detail: queryLike ? `검색어: ${queryLike}` : '' };
    case 'read_web_page':
    case 'open_url':
      return { title: '웹 페이지 읽는 중', detail: url ? `페이지 열기: ${url}` : '' };
    case 'read_buffered_source':
      return {
        title: '소스 읽는 중',
        detail: (queryLike || sourceId) ? `소스 읽기: ${queryLike || sourceId}` : ''
      };
    case 'search_memory':
    case 'memory_search':
      return { title: '메모리 검색 중', detail: queryLike ? `메모리 검색: ${queryLike}` : '' };
    case 'read_memory':
      return { title: '메모리 읽는 중', detail: memoryId ? `메모리 읽기: ID ${memoryId}` : '' };
    case 'delete_memory':
      return { title: '메모리 삭제 중', detail: memoryId ? `메모리 삭제: ID ${memoryId}` : '' };
    case 'execute_command':
      return { title: '명령어 실행 중', detail: command ? `명령어 실행: ${command}` : '' };
    default: {
      const label = rawTool
        ? rawTool.replace(/[_-]+/g, ' ').trim()
        : '도구';
      return { title: `도구 실행 중 (${label})`, detail: queryLike || url || command || '' };
    }
  }
}

export function wrapHUDLines(text, maxWidth = 562) {
  const source = String(text || '').replace(/\r\n?/g, '\n');
  if (!source.trim()) return [];
  const rawLines = source.split('\n');
  const wrapped = [];

  for (const rawLine of rawLines) {
    const trimmedRight = rawLine.trimEnd();
    if (!trimmedRight) {
      if (wrapped.length > 0 && wrapped[wrapped.length - 1] !== '') {
        wrapped.push('');
      }
      continue;
    }
    let current = '';
    for (const char of Array.from(trimmedRight)) {
      if (current && getTextWidth(current + char) > maxWidth) {
        wrapped.push(current.trimEnd());
        current = char === ' ' ? '' : char;
      } else {
        current += char;
      }
    }
    if (current.trimEnd()) {
      wrapped.push(current.trimEnd());
    }
  }

  while (wrapped.length > 0 && wrapped[wrapped.length - 1] === '') {
    wrapped.pop();
  }
  return wrapped;
}

export function sliceHUDWindow(text, offsetLine = 0, visibleLines = 5, maxWidth = 562) {
  const lines = wrapHUDLines(text, maxWidth);
  const totalLines = lines.length;
  if (totalLines === 0) {
    return {
      lines: [],
      visibleText: '',
      totalLines: 0,
      offsetLine: 0,
      maxOffset: 0,
      page: 1,
      totalPages: 1,
    };
  }
  const maxOffset = Math.max(0, totalLines - visibleLines);
  const clampedOffset = offsetLine === Infinity
    ? maxOffset
    : Math.max(0, Math.min(maxOffset, Math.trunc(offsetLine) || 0));
  const slice = lines.slice(clampedOffset, clampedOffset + visibleLines);
  const totalPages = Math.max(1, Math.ceil(totalLines / visibleLines));
  const page = maxOffset === 0
    ? 1
    : Math.min(totalPages, Math.floor((clampedOffset / maxOffset) * (totalPages - 1)) + 1);

  return {
    lines: slice,
    visibleText: slice.join('\n'),
    totalLines,
    offsetLine: clampedOffset,
    maxOffset,
    page,
    totalPages,
  };
}

export const GLASSES_MODES = ['ai', 'translate'];

export function nextGlassesMode(mode) {
  const current = normalizeGlassesMode(mode);
  const idx = GLASSES_MODES.indexOf(current);
  return GLASSES_MODES[(idx + 1) % GLASSES_MODES.length];
}

export function glassesModeMenuIdToMode(itemID) {
  const n = Number(itemID);
  if (n === 1) return 'ai';
  if (n === 2) return 'translate';
  return null;
}

export function glassesModeShortLabel(mode, targetLang = 'ko') {
  const normalized = normalizeGlassesMode(mode);
  if (normalized === 'ai') return 'AI 모드';
  if (normalized === 'translate') return `번역→${targetLanguageLabel(targetLang, true)}`;
  return 'AI 모드';
}

export function glassesModeBadgeText(mode, targetLang = 'ko') {
  const normalized = normalizeGlassesMode(mode);
  if (normalized === 'ai') return 'AI 모드';
  if (normalized === 'translate') return `번역 · 추천 답변 (→${targetLanguageLabel(targetLang, true)})`;
  return 'AI 응답';
}

export function cleanHintText(text) {
  let result=stripSourcesForGlasses(text).trim();
  // Strip echoed routing labels only at the beginning, never words in the body.
  const label='(?:explain|background|recall|reply|check|hint)';
  const wrapped=new RegExp('^(?:#{1,6}\\s*)?(?:\\['+label+'\\]|\\*\\*'+label+'\\*\\*)[ \\t]*[:：-]?[ \\t]*\\n*','i');
  const plain=new RegExp('^(?:#{1,6}\\s*)?'+label+'(?:[ \\t]*[:：][ \\t]*|[ \\t]*\\n+|$)','i');
  for(let i=0;i<3;i++) {const next=result.replace(wrapped,'').replace(plain,'').trim();if(next===result)break;result=next;}
  return result;
}

export {buildModePrompts} from './prompts.mjs';
