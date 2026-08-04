const $ = (id) => document.getElementById(id);

const elements = {
  endpoint: $('endpoint'), model: $('model'), modelList: $('modelList'), apiKey: $('apiKey'),
  workspace: $('workspace'), mode: $('mode'), allowWrites: $('allowWrites'),
  allowCommands: $('allowCommands'), testPrompt: $('testPrompt'), analysisTask: $('analysisTask'), maxSteps: $('maxSteps'),
  run: $('run'), loadModels: $('loadModels'), status: $('statusBadge'),
  resultPanel: $('resultPanel'), failure: $('failure'), answer: $('answer'),
  trace: $('trace'), runMeta: $('runMeta'), changedSection: $('changedSection'),
  changedFiles: $('changedFiles'),
  testSection: $('testSection'), testMeta: $('testMeta'), testAnswer: $('testAnswer'), testTrace: $('testTrace'),
};

function setStatus(text, state) {
  elements.status.textContent = text;
  elements.status.className = `badge ${state}`;
}

function setBusy(busy) {
  elements.run.disabled = busy;
  elements.loadModels.disabled = busy;
  elements.run.textContent = busy ? '실행 중…' : '테스트베드 실행';
}

function saveSettings() {
  const settings = {
    endpoint: elements.endpoint.value,
    model: elements.model.value,
    workspace: elements.workspace.value,
    maxSteps: elements.maxSteps.value,
  };
  localStorage.setItem('localLLMCodeTestbed', JSON.stringify(settings));
}

function loadSavedSettings() {
  try { return JSON.parse(localStorage.getItem('localLLMCodeTestbed') || '{}'); }
  catch { return {}; }
}

async function loadConfig() {
  const response = await fetch('/api/config');
  const config = await response.json();
  const saved = loadSavedSettings();
  elements.endpoint.value = saved.endpoint || config.endpoint;
  elements.model.value = saved.model || '';
  elements.workspace.value = saved.workspace || config.workspace;
  elements.maxSteps.value = saved.maxSteps || config.max_steps;
}

async function loadModels() {
  setStatus('모델 조회', 'running');
  try {
    const response = await fetch(`/api/models?endpoint=${encodeURIComponent(elements.endpoint.value.trim())}`, {
      headers: elements.apiKey.value ? { 'X-Testbed-API-Key': elements.apiKey.value } : {},
    });
    const payload = await response.json();
    if (!response.ok) throw new Error(payload.error || `HTTP ${response.status}`);
    elements.modelList.replaceChildren();
    for (const model of payload.models || []) {
      const option = document.createElement('option');
      option.value = model;
      elements.modelList.append(option);
    }
    if (!elements.model.value && payload.models?.length) elements.model.value = payload.models[0];
    setStatus(`${payload.models?.length || 0}개 모델`, 'done');
    saveSettings();
  } catch (error) {
    setStatus('조회 실패', 'error');
    window.alert(`모델 조회 실패: ${error.message}`);
  }
}

function renderResult(result) {
  elements.resultPanel.classList.remove('hidden');
  elements.answer.textContent = result.answer || '(최종 답변 없음)';
  elements.runMeta.textContent = `${result.rounds || 0} 라운드 · ${result.duration_ms || 0}ms`;
  elements.failure.textContent = result.failure || '';
  elements.failure.classList.toggle('hidden', !result.failure);
  renderConversationTest(result.test_report);
  elements.changedFiles.replaceChildren();
  for (const path of result.changed_files || []) {
    const item = document.createElement('li');
    item.textContent = path;
    elements.changedFiles.append(item);
  }
  elements.changedSection.classList.toggle('hidden', !(result.changed_files || []).length);
  elements.trace.replaceChildren();
  for (const step of result.trace || []) {
    const details = document.createElement('details');
    if (step.error) details.classList.add('error');
    const summary = document.createElement('summary');
    summary.textContent = `${step.error ? '실패' : '완료'} · ${step.tool} · ${step.duration_ms || 0}ms`;
    const body = document.createElement('div');
    body.className = 'trace-body';
    const meta = document.createElement('div');
    meta.className = 'trace-meta';
    meta.textContent = `라운드 ${step.round}${step.recovered_text ? ' · 텍스트 호출 복구' : ''}`;
    const content = document.createElement('pre');
    const args = JSON.stringify(step.arguments || {}, null, 2);
    content.textContent = `Arguments\n${args}\n\n${step.error ? `Error\n${step.error}\n\n` : ''}Result\n${step.result || '(출력 없음)'}`;
    body.append(meta, content);
    details.append(summary, body);
    elements.trace.append(details);
  }
  if (!(result.trace || []).length) {
    const empty = document.createElement('p');
    empty.className = 'meta';
    empty.textContent = '도구 호출이 없습니다.';
    elements.trace.append(empty);
  }
  elements.resultPanel.scrollIntoView({ behavior: 'smooth', block: 'start' });
}

function renderConversationTest(report) {
  elements.testSection.classList.toggle('hidden', !report);
  if (!report) return;
  const skills = (report.selected_skills || []).join(', ') || '선택된 스킬 없음';
  elements.testMeta.textContent = `${report.llm_rounds || 0} 라운드 · ${report.duration_ms || 0}ms · ${skills}`;
  elements.testAnswer.textContent = report.final_answer || report.failure || '(응답 없음)';
  elements.testTrace.replaceChildren();
  for (const step of report.tool_trace || []) {
    const details = document.createElement('details');
    if (step.error) details.classList.add('error');
    const summary = document.createElement('summary');
    summary.textContent = `${step.error ? '실패' : '완료'} · ${step.name}`;
    const body = document.createElement('div');
    body.className = 'trace-body';
    const content = document.createElement('pre');
    content.textContent = `Arguments\n${JSON.stringify(step.arguments || {}, null, 2)}\n\n${step.error ? `Error\n${step.error}\n\n` : ''}Result\n${step.result_preview || '(출력 없음)'}${step.sources?.length ? `\n\nSources\n${step.sources.join('\n')}` : ''}`;
    body.append(content);
    details.append(summary, body);
    elements.testTrace.append(details);
  }
}

async function run() {
  const payload = {
    endpoint: elements.endpoint.value.trim(),
    api_key: elements.apiKey.value,
    model: elements.model.value.trim(),
    workspace: elements.workspace.value.trim(),
    test_prompt: elements.testPrompt.value.trim(),
    analysis_task: elements.analysisTask.value.trim(),
    mode: elements.mode.value,
    allow_writes: elements.allowWrites.checked,
    allow_commands: elements.allowCommands.checked,
    max_steps: Number(elements.maxSteps.value || 8),
  };
  if (!payload.endpoint || !payload.model || !payload.workspace || !payload.test_prompt) {
    window.alert('엔드포인트, 모델, 작업 폴더, 실제 대화 테스트 질의를 모두 입력하세요.');
    return;
  }
  if (payload.allow_writes && !window.confirm('로컬 LLM이 작업 폴더의 파일을 수정하도록 허용할까요?')) return;
  saveSettings();
  setBusy(true);
  setStatus('에이전트 실행', 'running');
  elements.resultPanel.classList.add('hidden');
  try {
    const response = await fetch('/api/run', {
      method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(payload),
    });
    const result = await response.json();
    if (!response.ok && !result.failure) throw new Error(result.error || `HTTP ${response.status}`);
    renderResult(result);
    setStatus(result.failure ? '일부 실패' : '완료', result.failure ? 'error' : 'done');
  } catch (error) {
    setStatus('실행 실패', 'error');
    renderResult({ failure: error.message, answer: '', trace: [], rounds: 0, duration_ms: 0 });
  } finally {
    setBusy(false);
  }
}

elements.mode.addEventListener('change', () => {
  const canWrite = elements.mode.value === 'fix';
  elements.allowWrites.disabled = !canWrite;
  if (!canWrite) elements.allowWrites.checked = false;
});
elements.loadModels.addEventListener('click', loadModels);
elements.run.addEventListener('click', run);
loadConfig().catch((error) => {
  setStatus('초기화 실패', 'error');
  window.alert(error.message);
});
