"""Protocol regressions without loading a model or third-party dependencies."""
import contextlib
import io
import json
from pathlib import Path
import subprocess
import sys
import unittest

from whisper_worker import send, transcript_rejection


class WhisperProtocolTests(unittest.TestCase):
    def test_nonfinite_metrics_produce_strict_json_and_rejected_transcript(self):
        for confidence in (float('nan'), float('inf'), float('-inf')):
            reason = transcript_rejection('오늘 회의는 세 시입니다.', confidence)
            self.assertEqual(reason, 'invalid-confidence')
            output = io.StringIO()
            with contextlib.redirect_stdout(output):
                send({'text': '', 'rejected': True, 'reason': reason,
                      'averageLogprob': confidence, 'metrics': [confidence]})
            result = json.loads(output.getvalue(), parse_constant=self.fail)
            self.assertIsNone(result['averageLogprob'])
            self.assertEqual(result['metrics'], [None])

    def test_library_output_cannot_corrupt_ready_or_inference_replies(self):
        script = '''
import os
from whisper_worker import isolate_protocol_output, send
isolate_protocol_output()
print('Native library startup message', flush=True)
os.write(1, b'Native C stdout message\\n')
send({'ready': True})
print('No speech diagnostic', flush=True)
send({'text': '안녕하세요', 'duration': 1.0})
send({'error': 'engine failure'})
'''
        result = subprocess.run([sys.executable, '-u', '-c', script],
                                cwd=Path(__file__).parent, capture_output=True,
                                text=True, encoding='utf-8', check=True, timeout=10)
        replies = [json.loads(line) for line in result.stdout.splitlines()]
        self.assertEqual(replies, [{'ready': True}, {'text': '안녕하세요', 'duration': 1.0},
                                   {'error': 'engine failure'}])
        self.assertIn('Native library startup message', result.stderr)
        self.assertIn('Native C stdout message', result.stderr)
        self.assertIn('No speech diagnostic', result.stderr)


if __name__ == '__main__':
    unittest.main()
