(function () {
    const STORAGE_KEY = 'appConfig';
    const WEB_HAPTICS_MODULE_URL = './vendor/web-haptics.index.mjs';
    const PRESETS = {
        success: [50, 50, 50],
        nudge: [80, 80, 50],
        error: [50, 50, 50, 50, 50],
        buzz: 1000
    };

    let enabled = true;
    let webHapticsInstance = null;
    let webHapticsLoadPromise = null;

    function loadEnabledState() {
        try {
            const raw = localStorage.getItem(STORAGE_KEY);
            if (!raw) return true;
            const parsed = JSON.parse(raw);
            return parsed?.hapticsEnabled !== false;
        } catch (error) {
            console.warn('[Haptics] Failed to read stored preference:', error);
            return true;
        }
    }

    function supportsHaptics() {
        return typeof navigator !== 'undefined' && typeof navigator.vibrate === 'function';
    }

    async function ensureWebHaptics() {
        if (webHapticsInstance) return webHapticsInstance;
        if (webHapticsLoadPromise) return webHapticsLoadPromise;

        webHapticsLoadPromise = import(WEB_HAPTICS_MODULE_URL)
            .then((mod) => {
                if (!mod?.WebHaptics) {
                    throw new Error('WebHaptics export missing');
                }
                webHapticsInstance = new mod.WebHaptics();
                return webHapticsInstance;
            })
            .catch((error) => {
                console.warn('[Haptics] Failed to load web-haptics, falling back to navigator.vibrate():', error);
                webHapticsLoadPromise = null;
                return null;
            });

        return webHapticsLoadPromise;
    }

    function normalizeInput(input) {
        if (Array.isArray(input) || typeof input === 'number') {
            return input;
        }
        return PRESETS[input] || null;
    }

    async function trigger(input) {
        if (!enabled) return false;
        const normalizedInput = typeof input === 'string' ? input : normalizeInput(input);
        if (normalizedInput == null) return false;

        try {
            const instance = await ensureWebHaptics();
            if (instance?.trigger) {
                await instance.trigger(normalizedInput);
                return true;
            }
        } catch (error) {
            console.warn('[Haptics] web-haptics trigger failed, trying navigator.vibrate():', error);
        }

        if (!supportsHaptics()) return false;
        const pattern = normalizeInput(input);
        if (pattern == null) return false;
        try {
            return navigator.vibrate(pattern);
        } catch (error) {
            console.warn('[Haptics] Trigger failed:', error);
            return false;
        }
    }

    function setEnabled(nextEnabled) {
        enabled = nextEnabled !== false;
    }

    enabled = loadEnabledState();

    window.DKSTHaptics = {
        trigger,
        setEnabled,
        isEnabled() {
            return enabled;
        },
        isSupported() {
            return supportsHaptics() || !!webHapticsInstance;
        }
    };
})();
