import {abortableWait} from './abortable-wait.mjs';
import {rgbaToGray4} from './hud-icons.mjs';

// Even Hub SDK 0.0.16 image container limits.
export const CHAT_IMAGE_WIDTH = 288;
export const CHAT_IMAGE_HEIGHT = 144;
export const CHAT_IMAGE_LIMIT = 2;

export function splitChatImages(text) {
  const paths = [];
  const visible = String(text || '').replace(/!\[[^\]\r\n]*\]\((\/api\/computer\/screenshots\/[a-f0-9]{12}-[a-f0-9]{24}\.png)\)/gu, (_, path) => {
    if (!paths.includes(path)) paths.push(path);
    return '';
  });
  return {text: visible.trim(), paths: paths.slice(0, CHAT_IMAGE_LIMIT)};
}

export function fitChatImage(width, height) {
  if (!(width > 0 && height > 0)) throw Error('Invalid screenshot dimensions');
  const scale = Math.min(CHAT_IMAGE_WIDTH / width, CHAT_IMAGE_HEIGHT / height);
  const w = Math.max(1, Math.round(width * scale));
  const h = Math.max(1, Math.round(height * scale));
  return {x: Math.floor((CHAT_IMAGE_WIDTH - w) / 2), y: Math.floor((CHAT_IMAGE_HEIGHT - h) / 2), width: w, height: h};
}

export function chatImageContainers(count) {
  return Array.from({length: count}, (_, index) => ({
    containerID: 13 + index,
    containerName: 'chat-image-' + (index + 1),
    xPosition: count === 1 ? CHAT_IMAGE_WIDTH / 2 : index * CHAT_IMAGE_WIDTH,
    yPosition: 27,
    width: CHAT_IMAGE_WIDTH,
    height: CHAT_IMAGE_HEIGHT,
  }));
}

export async function fetchChatImage(fetchImage, server, path, headers, signal) {
  const response = await fetchImage(new URL(path, server).href, {headers, signal, redirect: 'error'});
  if (!response.ok || !response.headers.get('content-type')?.toLowerCase().startsWith('image/png')) {
    throw Error('Screenshot download failed');
  }
  const bitmapURL = URL.createObjectURL(await response.blob());
  try {
    const image = new Image();
    await abortableWait(new Promise((resolve, reject) => {
      image.onload = resolve;
      image.onerror = () => reject(Error('Screenshot decode failed'));
      image.src = bitmapURL;
    }), signal);
    if (signal?.aborted) throw signal.reason;
    const fit = fitChatImage(image.naturalWidth, image.naturalHeight);
    const canvas = document.createElement('canvas');
    canvas.width = CHAT_IMAGE_WIDTH;
    canvas.height = CHAT_IMAGE_HEIGHT;
    const ctx = canvas.getContext('2d', {willReadFrequently: true});
    ctx.drawImage(image, fit.x, fit.y, fit.width, fit.height);
    const rgba = ctx.getImageData(0, 0, canvas.width, canvas.height).data;
    return rgbaToGray4(rgba, canvas.width, canvas.height);
  } finally {
    URL.revokeObjectURL(bitmapURL);
  }
}
