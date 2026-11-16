export const TARGET_LANGUAGE_LABELS = {
  ko: '한국어 (Korean)',
  en: '영어 (English)',
  ja: '일본어 (Japanese)',
  zh: '중국어 (Chinese)',
  es: '스페인어 (Spanish)',
  fr: '프랑스어 (French)',
  de: '독일어 (German)',
  vi: '베트남어 (Vietnamese)',
  th: '태국어 (Thai)',
};

export const TARGET_LANGUAGE_SHORT = {
  ko: '한국어',
  en: '영어',
  ja: '일본어',
  zh: '중국어',
  es: '스페인어',
  fr: '프랑스어',
  de: '독일어',
  vi: '베트남어',
  th: '태국어',
};

export function normalizeGlassesMode(mode) {
  return mode==='translate'?'translate':'ai';
}

export function targetLanguageLabel(langCode = 'ko', short = false) {
  const key = String(langCode || 'ko').trim().toLowerCase();
  if (short) return TARGET_LANGUAGE_SHORT[key] || TARGET_LANGUAGE_SHORT.ko;
  return TARGET_LANGUAGE_LABELS[key] || TARGET_LANGUAGE_LABELS.ko;
}

