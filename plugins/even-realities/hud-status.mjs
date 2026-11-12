export function clockText(date = new Date()) {
  return `${String(date.getHours()).padStart(2, '0')}:${String(date.getMinutes()).padStart(2, '0')}`;
}

export function glassesBatteryText(level) {
  return typeof level === 'number' && Number.isFinite(level) && level >= 0 && level <= 100
    ? `${Math.round(level)}%` : '--';
}

export function hudStatusText(level, date = new Date()) {
  return `${clockText(date)} / ${glassesBatteryText(level)}`;
}

export function hudNoticeText(message) {
  let text = '', units = 0;
  for (const char of Array.from(message)) {
    const width = char.codePointAt(0) <= 255 ? 1 : 2;
    if (units + width > 34) return text.trimEnd() + '…';
    text += char; units += width;
  }
  return text;
}
