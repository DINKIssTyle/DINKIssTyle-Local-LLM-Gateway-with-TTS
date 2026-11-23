"""Small protocol tests run without MLX or downloaded model weights."""
import unittest
from qwen_asr_worker import language_code, forced_language


class QwenLanguages(unittest.TestCase):
    def test_detection_normalizes_to_gateway_codes(self):
        for value, expected in [('Korean', 'ko'), ('Japanese', 'ja'), (['English'], 'en'),
                                ('ko-KR', 'ko'), (['', 'Chinese'], 'zh'), ('unknown', '')]:
            self.assertEqual(language_code(value), expected)

    def test_auto_does_not_force_korean(self):
        for value in [None, '', 'auto', 'AUTO']:
            self.assertIsNone(forced_language(value))

    def test_forced_language_is_model_language_name(self):
        self.assertEqual(forced_language('en'), 'English')
        self.assertEqual(forced_language('ja-JP'), 'Japanese')
        self.assertEqual(forced_language('ko'), 'Korean')
        with self.assertRaises(ValueError):
            forced_language('invalid')


if __name__ == '__main__':
    unittest.main()
