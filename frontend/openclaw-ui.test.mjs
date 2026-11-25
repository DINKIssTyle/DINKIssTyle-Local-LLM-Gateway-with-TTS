import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import vm from 'node:vm';
import test from 'node:test';

class Element {
    constructor(tag) { this.tag = tag; this.children = []; this.value = ''; this.checked = false; this.disabled = false; }
    append(...children) { this.children.push(...children); }
    replaceChildren(...children) { this.children = [...children]; }
    setAttribute(name, value) { this[name] = value; }
}
const elements = (root, tag) => root.children.flatMap(e => e && typeof e === 'object' ? [...(e.tag === tag ? [e] : []), ...elements(e, tag)] : []);
function setup() {
    const copied = [];
    const window = {navigator: {clipboard: {writeText: async value => copied.push(value)}}};
    const document = {documentElement: {lang: 'ko'}, createElement: tag => new Element(tag)};
    vm.runInNewContext(readFileSync(new URL('./openclaw-ui.js', import.meta.url), 'utf8'), {window, document});
    return {ui: window.DKSTOpenClawUI, root: new Element('div'), document, copied};
}
const initial = () => ({enabled: false, model: '', has_token: false, custom_endpoint: '', endpoint: 'http://192.168.1.4:2807/v1/chat/completions'});
const models = async () => [{id: 'alice-model', name: '[Default] Model A'}, {id: 'remote-model', name: '[Remote] Model B'}];
const button = (root, label) => elements(root, 'button').find(node => node.textContent === label);

test('account model selection, issuance and copy use the selected account; disabling wipes the token', async () => {
    const {ui, root, copied} = setup(); const calls = []; let stored = initial();
    await ui.mount(root, {userId: 'alice', api: {
        GetUserOpenClawSettings: async user => { assert.equal(user, 'alice'); return {...stored}; },
        GetOpenClawModels: async user => { assert.equal(user, 'alice'); return models(); },
        SetUserOpenClawSettings: async (...args) => { calls.push(['save', ...args]); stored = {...stored, enabled: args[1], model: args[2], custom_endpoint: args[3], endpoint: args[3] || initial().endpoint, has_token: args[1] && stored.has_token}; },
        IssueUserOpenClawToken: async user => { calls.push(['issue', user]); stored.has_token = true; return 'dkst_oc_alice_secret'; }
    }});
    const select = elements(root, 'select')[0]; select.value = 'alice-model'; await select.onchange();
    assert.deepEqual(calls[0], ['save', 'alice', false, 'alice-model', '']);
    await button(root, '토큰 발급').onclick();
    assert.deepEqual(calls.slice(-2), [['save', 'alice', true, 'alice-model', ''], ['issue', 'alice']]);
    const token = elements(root, 'input').find(input => input.type === 'password');
    assert.equal(token.value, 'dkst_oc_alice_secret');
    await button(root, '토큰 복사').onclick(); assert.deepEqual(copied, ['dkst_oc_alice_secret']);
    await button(root, '주소 복사').onclick(); assert.equal(copied[1], initial().endpoint);
    const toggle = elements(root, 'input').find(input => input.type === 'checkbox');
    toggle.checked = false; await toggle.onchange();
    assert.equal(token.value, ''); assert.equal(button(root, '토큰 복사').disabled, true);
    assert.deepEqual(calls.at(-1), ['save', 'alice', false, 'alice-model', '']);
});

test('an existing token is never redisplayed; cancelled regeneration keeps it valid; language changes preserve selection', async () => {
    const {ui, root, document} = setup(); let issued = 0;
    await ui.mount(root, {userId: 'alice', confirmRegenerate: async () => false, api: {
        GetUserOpenClawSettings: async () => ({...initial(), enabled: true, model: 'alice-model', has_token: true}),
        GetOpenClawModels: models,
        SetUserOpenClawSettings: async () => { throw Error('must not save on cancelled regeneration'); },
        IssueUserOpenClawToken: async () => { issued++; }
    }});
    assert.equal(elements(root, 'input').find(input => input.type === 'password').value, '');
    await button(root, '토큰 재발급').onclick(); assert.equal(issued, 0);
    document.documentElement.lang = 'en'; root.refreshOpenClawLanguage();
    assert.ok(button(root, 'Regenerate token')); assert.equal(elements(root, 'select')[0].value, 'alice-model');
});

test('account switch ignores late loading and late token responses and clears credential fields on close', async () => {
    const {ui, root} = setup(); let finishLoading;
    const old = ui.mount(root, {userId: 'alice', api: {GetUserOpenClawSettings: () => new Promise(resolve => finishLoading = resolve)}});
    const bob = {GetUserOpenClawSettings: async () => ({...initial(), model: 'remote-model'}), GetOpenClawModels: models};
    await ui.mount(root, {userId: 'bob', api: bob}); finishLoading({...initial(), model: 'alice-model'}); await old;
    assert.equal(elements(root, 'select')[0].value, 'remote-model');
    let finishIssuing;
    await ui.mount(root, {userId: 'alice', api: {
        GetUserOpenClawSettings: async () => ({...initial(), model: 'alice-model'}), GetOpenClawModels: models,
        SetUserOpenClawSettings: async () => {}, IssueUserOpenClawToken: () => new Promise(resolve => finishIssuing = resolve)
    }});
    const pending = button(root, '토큰 발급').onclick();
    await new Promise(resolve => setImmediate(resolve));
    await ui.mount(root, {userId: 'bob', api: bob}); finishIssuing('dkst_oc_alice_private'); await pending;
    assert.equal(elements(root, 'input').find(input => input.type === 'password').value, '');
    root.disposeOpenClaw(); assert.equal(root.children.length, 0); assert.equal(root.saveOpenClawSettings, null);
});

test('failed account setting changes restore the saved model and enabled state', async () => {
    const {ui, root} = setup();
    await ui.mount(root, {userId: 'alice', api: {
        GetUserOpenClawSettings: async () => ({...initial(), enabled: true, model: 'alice-model', has_token: true}),
        GetOpenClawModels: models, SetUserOpenClawSettings: async () => { throw Error('disk full'); }
    }});
    const select = elements(root, 'select')[0]; select.value = 'remote-model'; await select.onchange();
    assert.equal(select.value, 'alice-model'); assert.ok(elements(root, 'p').some(p => p.textContent === 'disk full'));
});


test('external server URL is editable, saved per account and used when copying; clearing restores automatic URL', async () => {
    const {ui, root, copied} = setup(); let stored = {...initial(), enabled: true, model: 'alice-model', has_token: true};
    const saved = [];
    await ui.mount(root, {userId: 'alice', api: {
        GetUserOpenClawSettings: async () => ({...stored}), GetOpenClawModels: models,
        SetUserOpenClawSettings: async (id, enabled, model, endpoint) => {
            saved.push([id, endpoint]);
            if (endpoint === 'invalid') throw Error('Invalid URL');
            stored = {...stored, enabled, model, custom_endpoint: endpoint, endpoint: endpoint || initial().endpoint};
        }
    }});
    const endpoint = elements(root, 'input').find(node => node.id === 'account-openclaw-endpoint');
    assert.equal(endpoint.readOnly, undefined); assert.equal(endpoint.value, ''); assert.equal(endpoint.placeholder, initial().endpoint);
    endpoint.value = 'https://chat.example.com/v1/chat/completions'; await endpoint.onchange();
    await button(root, '주소 복사').onclick();
    assert.equal(copied.at(-1), 'https://chat.example.com/v1/chat/completions'); assert.equal(saved.at(-1)[0], 'alice');
    assert.equal(stored.has_token, true);
    endpoint.value = 'invalid'; await endpoint.onchange();
    assert.equal(endpoint.value, 'https://chat.example.com/v1/chat/completions');
    endpoint.value = ''; await endpoint.onchange(); await button(root, '주소 복사').onclick();
    assert.equal(copied.at(-1), initial().endpoint); assert.equal(endpoint.value, '');
});
