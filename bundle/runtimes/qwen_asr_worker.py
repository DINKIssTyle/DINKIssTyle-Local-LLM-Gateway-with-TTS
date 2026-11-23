"""Offline Qwen3-ASR MLX worker; PCM snapshots in, JSON lines out.

Preview uses bounded re-decoding with the client's LocalAgreement, not a
claim of stateful microphone streaming. Whisper's acoustic VAD is shared;
Whisper decoder confidence thresholds are deliberately not used here.
"""
import argparse
import os
import sys
from whisper_worker import decode_audio, isolate_protocol_output, send, SpeechFilter, transcript_rejection

LANGUAGES = dict(zip(
    'zh en yue ar de fr es pt id it ko ru th vi ja tr hi ms nl sv da fi pl cs fil fa el hu mk ro'.split(),
    ['Chinese', 'English', 'Cantonese', 'Arabic', 'German', 'French', 'Spanish',
     'Portuguese', 'Indonesian', 'Italian', 'Korean', 'Russian', 'Thai', 'Vietnamese',
     'Japanese', 'Turkish', 'Hindi', 'Malay', 'Dutch', 'Swedish', 'Danish', 'Finnish',
     'Polish', 'Czech', 'Filipino', 'Persian', 'Greek', 'Hungarian', 'Macedonian', 'Romanian']))


def language_code(value):
    if isinstance(value, (list, tuple)):
        value = next((part for part in value if part), '')
    value = str(value or '').strip().lower()
    by_name = {name.lower(): code for code, name in LANGUAGES.items()}
    return by_name.get(value, value.split('-')[0] if value.split('-')[0] in LANGUAGES else '')


def forced_language(value):
    if str(value or 'auto').strip().lower() == 'auto':
        return None
    code = language_code(value)
    if not code:
        raise ValueError('Unsupported Qwen3-ASR language: ' + str(value))
    return LANGUAGES[code]


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--model', required=True)
    args = parser.parse_args()
    import json
    import numpy as np
    import mlx.core as mx
    from mlx_audio.stt import load
    speech_filter = SpeechFilter(os.path.join(args.model, 'silero_vad.onnx'))
    model = load(args.model)
    mx.eval(model.parameters())
    mx.synchronize()
    send({'ready': True})
    for line in sys.stdin:
        try:
            request = json.loads(line)
            language = forced_language(request.get('language'))
            audio = decode_audio(request)
            duration = len(audio) / 16000
            if float(np.sqrt(np.mean(audio * audio))) < 0.001:
                send({'text': '', 'duration': duration, 'language': language_code(language)})
                continue
            audio = speech_filter.apply(audio, request.get('speechFilter') or 'strict')
            if not len(audio):
                send({'text': '', 'duration': duration, 'rejected': True, 'reason': 'no-speech'})
                continue
            budget = min(640, max(64, int(duration * 20)))
            result = model.generate(audio, language=language, max_tokens=budget,
                                    temperature=0.0, verbose=False)
            text = str(result.text or '').strip()
            reason = transcript_rejection(text, **speech_filter.stats)
            if getattr(result, 'generation_tokens', 0) >= budget:
                reason = reason or 'incomplete-decode'
            send({'text': '' if reason else text, 'duration': duration,
                  'language': language_code(getattr(result, 'language', None)) or language_code(language),
                  'rejected': bool(reason), 'reason': reason})
        except Exception as exc:
            send({'error': str(exc)})


if __name__ == '__main__':
    isolate_protocol_output()
    try:
        main()
    except Exception as exc:
        send({'error': str(exc)})
        sys.exit(1)
