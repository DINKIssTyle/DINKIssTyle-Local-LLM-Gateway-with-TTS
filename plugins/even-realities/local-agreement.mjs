// Display stabilization only. Final utterance decoding remains authoritative.
export class LocalAgreement {
  constructor() { this.previous = ''; }
  update(value) {
    const text = String(value).normalize('NFC').trim();
    const before = Array.from(this.previous), after = Array.from(text);
    let count = 0;
    while (count < before.length && count < after.length && before[count] === after[count]) count++;
    let stable = after.slice(0,count).join('');
    if (stable && count < after.length && !/[\s.,!?。！？]/u.test(after[count])) {
      const boundary = [...stable.matchAll(/[\s.,!?。！？]/gu)].at(-1);
      stable = boundary ? stable.slice(0,boundary.index+boundary[0].length) : '';
    }
    this.previous = text;
    return { text, stable, draft: text.slice(stable.length) };
  }
}
