(function (global) {
    'use strict';

    function element(tag, className = '') {
        const value = document.createElement(tag);
        value.className = className;
        return value;
    }

    async function mount(root, {userId, api, confirmRegenerate = async () => true}) {
        root.disposeOpenClaw?.();
        let alive = true;
        let busy = false;
        let ready = false;
        let feedback = "";
        let saved;
        const labels = [];
        const ko = () => document.documentElement.lang !== 'en';
        const text = (node, korean, english) => {
            labels.push(() => { node.textContent = ko() ? korean : english; });
            labels.at(-1)();
            return node;
        };
        root.disposeOpenClaw = () => {
            alive = false;
            root.replaceChildren();
            root.saveOpenClawSettings = null;
            root.refreshOpenClawLanguage = null;
        };
        root.refreshOpenClawLanguage = () => { labels.forEach(update => update()); updateStatus(); };

        const section = element('section', 'openclaw-settings');
        const heading = element('label', 'openclaw-settings__heading');
        const toggle = element('input', 'openclaw-settings__toggle');
        toggle.type = 'checkbox';
        toggle.setAttribute('role', 'switch');
        heading.append(text(element('span'), 'openclaw 연결', 'openclaw connection'), toggle);
        const help = text(element('p', 'openclaw-settings__help'),
            'Even 요청은 이 계정의 모델·개인 메모리·도구 권한으로 처리됩니다. 변경 사항은 즉시 적용됩니다.',
            "Even requests use this account's model, personal memory and tool permissions. Changes apply immediately.");
        const modelGroup = element('div');
        const modelLabel = text(element('label', 'openclaw-settings__model-label'), '대상 모델', 'Target model');
        const modelRow = element('div', 'openclaw-settings__model-row');
        const wrap = element('div', 'openclaw-settings__select-wrap');
        const model = element('select');
        model.id = 'account-openclaw-model';
        modelLabel.setAttribute('for', model.id);
        wrap.append(model);
        const refresh = element('button', 'btn btn-secondary openclaw-settings__refresh');
        refresh.type = 'button';
        const icon = element('span', 'material-icons-round');
        icon.textContent = 'refresh';
        icon.setAttribute('aria-hidden', 'true');
        refresh.append(icon);
        labels.push(() => {
            refresh.title = ko() ? '모델 목록 새로고침' : 'Refresh models';
            refresh.setAttribute('aria-label', refresh.title);
        });
        modelRow.append(wrap, refresh);
        modelGroup.append(modelLabel, modelRow);

        const endpointLabel = text(element('label', 'openclaw-settings__model-label'), 'Even 서버 주소', 'Even server URL');
        const endpoint = element('input', 'openclaw-settings__field');
        endpoint.type = 'url'; endpoint.autocomplete = 'off'; endpoint.spellcheck = false;
        endpoint.id = 'account-openclaw-endpoint'; endpointLabel.setAttribute('for', endpoint.id);
        const endpointHelp = text(element('p', 'openclaw-settings__help'),
            '외부 접속용 도메인·포트를 포함한 전체 주소를 입력하세요. 비워 두면 로컬 주소를 자동으로 사용합니다.',
            'Enter the full external URL, including its domain and port. Leave blank to use the automatic local URL.');
        const endpointGroup = element('div'); endpointGroup.append(endpointLabel, endpoint, endpointHelp);
        const tokenLabel = text(element('label', 'openclaw-settings__model-label'), 'Even API Token', 'Even API Token');
        const token = element('input', 'openclaw-settings__field');
        token.type = 'password'; token.readOnly = true; token.autocomplete = 'off';
        token.id = 'account-openclaw-token'; tokenLabel.setAttribute('for', token.id);
        const tokenGroup = element('div'); tokenGroup.append(tokenLabel, token);
        const tokenHelp = text(element('p', 'openclaw-settings__help'),
            '발급된 토큰은 지금만 복사할 수 있습니다. 창을 닫으면 다시 표시되지 않습니다. 재발급하거나 연결을 끄면 이전 토큰은 무효화됩니다.',
            'Copy a new token before closing this window; it is shown only once. Regenerating or disabling the connection revokes the old token.');
        const actions = element('div', 'openclaw-settings__actions');
        const issue = element('button', 'btn btn-secondary btn-compact'); issue.type = 'button';
        const copyToken = text(element('button', 'btn btn-secondary btn-compact'), '토큰 복사', 'Copy token'); copyToken.type = 'button';
        const copyEndpoint = text(element('button', 'btn btn-secondary btn-compact'), '주소 복사', 'Copy URL'); copyEndpoint.type = 'button';
        actions.append(issue, copyToken, copyEndpoint);
        const status = element('p', 'openclaw-settings__status'); status.setAttribute('role', 'status');
        const error = element('p', 'openclaw-settings__error'); error.setAttribute('role', 'alert');
        section.append(heading, help, modelGroup, endpointGroup, tokenGroup, tokenHelp, actions, status, error);
        root.replaceChildren(section);

        function updateStatus() {
            if (!alive) return;
            toggle.disabled = busy || !ready;
            model.disabled = busy || !ready;
            endpoint.disabled = busy || !ready;
            refresh.disabled = busy || !ready;
            issue.disabled = busy || !ready || !model.value;
            copyToken.disabled = busy || !token.value;
            copyEndpoint.disabled = busy || !ready || !(endpoint.value.trim() || saved?.endpoint);
            issue.textContent = saved?.has_token ? (ko() ? '토큰 재발급' : 'Regenerate token') : (ko() ? '토큰 발급' : 'Issue token');
            status.textContent = feedback || (!ready ? (ko() ? '계정 연결 확인 중…' : 'Loading account connection…')
                : saved?.enabled && saved?.has_token ? (ko() ? '연결 준비 완료 · 이 계정의 토큰을 Even에 입력하세요.' : "Ready · enter this account's token in Even.")
                : saved?.enabled ? (ko() ? '토큰을 발급하면 연결할 수 있습니다.' : 'Issue a token to connect.')
                : (ko() ? '이 계정의 openclaw 연결이 꺼져 있습니다.' : 'openclaw is disabled for this account.'));
        }
        async function save() {
            if (!alive || !ready) throw Error(ko() ? '계정 설정을 불러오는 중입니다.' : 'Account settings are loading.');
            await api.SetUserOpenClawSettings(userId, toggle.checked, model.value, endpoint.value.trim());
            if (!alive) return;
            const current = await api.GetUserOpenClawSettings(userId);
            if (!alive) return;
            saved = current;
            endpoint.value = current.custom_endpoint || '';
            endpoint.placeholder = current.endpoint;
            if (!toggle.checked) token.value = '';
        }
        root.saveOpenClawSettings = save;
        async function run(action) {
            if (!alive || busy) return;
            busy = true; feedback = ''; error.textContent = ''; updateStatus();
            try { await action(); }
            catch (e) {
                if (alive) {
                    error.textContent = String(e?.message || e);
                    toggle.checked = !!saved?.enabled;
                    model.value = saved?.model || '';
                    endpoint.value = saved?.custom_endpoint || '';
                }
            } finally { if (alive) { busy = false; updateStatus(); } }
        }
        async function loadModels() {
            const models = await api.GetOpenClawModels(userId);
            if (!alive) return;
            const selected = model.value || saved.model;
            model.replaceChildren();
            const option = (id, name) => { const node = element('option'); node.value = id; node.textContent = name; model.append(node); };
            option('', ko() ? '모델 선택' : 'Select model');
            for (const row of models) option(row.id, row.name);
            if (selected && !models.some(row => row.id === selected)) option(selected, selected + (ko() ? ' (현재 목록에 없음)' : ' (unavailable)'));
            model.value = selected || '';
        }
        toggle.onchange = () => run(save);
        model.onchange = () => run(save);
        endpoint.onchange = () => run(save);
        refresh.onclick = () => run(loadModels);
        issue.onclick = () => run(async () => {
            if (saved.has_token && !await confirmRegenerate()) return;
            if (!alive) return;
            toggle.checked = true;
            await save();
            if (!alive) return;
            const issued = await api.IssueUserOpenClawToken(userId);
            if (!alive) return;
            token.value = issued;
            saved.has_token = true;
        });
        async function copy(value) {
            if (global.runtime?.ClipboardSetText) await global.runtime.ClipboardSetText(value);
            else await global.navigator.clipboard.writeText(value);
            if (alive) feedback = ko() ? '복사했습니다.' : 'Copied.';
        }
        copyToken.onclick = () => run(async () => { await copy(token.value); });
        copyEndpoint.onclick = () => run(async () => { await save(); if (alive) await copy(saved.endpoint); });
        updateStatus();
        try {
            const initial = await api.GetUserOpenClawSettings(userId);
            if (!alive) return;
            saved = initial; toggle.checked = initial.enabled; endpoint.value = initial.custom_endpoint || ''; endpoint.placeholder = initial.endpoint;
            const current = element('option'); current.value = initial.model || ''; current.textContent = initial.model || (ko() ? '모델 선택' : 'Select model');
            model.append(current); model.value = initial.model || '';
            token.placeholder = initial.has_token ? (ko() ? '이미 발급됨 · 필요하면 재발급하세요' : 'Issued · regenerate if needed') : '';
            ready = true;
            await loadModels();
        } catch (e) { if (alive) error.textContent = String(e?.message || e); }
        if (alive) root.refreshOpenClawLanguage();
    }
    global.DKSTOpenClawUI = {mount};
})(window);
