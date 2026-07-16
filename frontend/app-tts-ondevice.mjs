/* Supertonic 3 browser inference. Models are supplied by the authenticated DKST server. */
import * as ort from './vendor/onnxruntime-web/ort.all.bundle.min.mjs';
import { loadTextToSpeech, loadVoiceStyle, writeWavFile } from './vendor/supertonic-helper.mjs';

const ASSET_ROOT = '/api/tts/on-device/assets';
const STATUS_URL = '/api/tts/on-device/status';
let enginePromise = null;
let backend = '';
const styles = new Map();

ort.env.wasm.wasmPaths = '/vendor/onnxruntime-web/';
ort.env.wasm.numThreads = 1; // Works without cross-origin isolation and avoids iOS worker limits.

const wait = (ms) => new Promise((resolve) => setTimeout(resolve, ms));

async function ensureAssets(onProgress) {
    for (;;) {
        const response = await fetch(STATUS_URL, { credentials: 'same-origin', cache: 'no-store' });
        if (!response.ok) throw new Error(`Could not check on-device TTS status (${response.status}).`);
        const status = await response.json();
        if (status.ready) return status;
        if (status.failed) throw new Error(status.message || 'Supertonic 3 model download failed.');
        onProgress?.(status.message || 'The main app is preparing Supertonic 3…');
        await wait(1000);
    }
}

async function createEngine(onProgress) {
    await ensureAssets(onProgress);
    const baseOptions = { graphOptimizationLevel: 'all' };
    if (typeof navigator !== 'undefined' && navigator.gpu) {
        try {
            onProgress?.('Loading Supertonic 3 with WebGPU…');
            const result = await loadTextToSpeech(`${ASSET_ROOT}/onnx`, {
                ...baseOptions,
                executionProviders: ['webgpu']
            }, (name, current, total) => onProgress?.(`Loading model (${current}/${total}): ${name}`));
            backend = 'WebGPU';
            return result.textToSpeech;
        } catch (error) {
            console.warn('[On-device TTS] WebGPU unavailable; using WASM.', error);
        }
    }
    onProgress?.('Loading Supertonic 3 with WASM…');
    const result = await loadTextToSpeech(`${ASSET_ROOT}/onnx`, {
        ...baseOptions,
        executionProviders: ['wasm']
    }, (name, current, total) => onProgress?.(`Loading model (${current}/${total}): ${name}`));
    backend = 'WASM';
    return result.textToSpeech;
}

async function getEngine(onProgress) {
    if (!enginePromise) {
        enginePromise = createEngine(onProgress).catch((error) => {
            enginePromise = null;
            throw error;
        });
    }
    return enginePromise;
}

async function getStyle(name) {
    const safeName = /^[MF][1-5]$/.test(String(name || '')) ? String(name) : 'F1';
    if (!styles.has(safeName)) {
        styles.set(safeName, loadVoiceStyle([`${ASSET_ROOT}/voice_styles/${safeName}.json`]));
    }
    return styles.get(safeName);
}

export async function synthesize({ text, lang, voice, steps, speed, onProgress }) {
    const engine = await getEngine(onProgress);
    const style = await getStyle(voice);
    onProgress?.(`Synthesizing on this device (${backend})…`);
    const result = await engine.call(
        String(text || ''),
        String(lang || 'ko'),
        style,
        Math.max(1, Math.min(50, Number(steps) || 5)),
        Math.max(0.5, Math.min(3, Number(speed) || 1.05)),
        0.3
    );
    const length = Math.min(result.wav.length, Math.floor(engine.sampleRate * result.duration[0]));
    return new Blob([writeWavFile(result.wav.slice(0, length), engine.sampleRate)], { type: 'audio/wav' });
}

export function getBackend() {
    return backend;
}
