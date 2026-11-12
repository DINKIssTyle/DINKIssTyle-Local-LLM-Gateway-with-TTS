function firstString(...values) {
  return values.find(value => typeof value === 'string' && value.trim())?.trim() || '';
}

// LM Studio's native API uses `key`; OpenAI-compatible APIs use `id`.
export function normalizeModels(payload) {
  const source = Array.isArray(payload) ? payload
    : Array.isArray(payload?.data) ? payload.data
    : Array.isArray(payload?.models) ? payload.models : [];
  const seen = new Set();
  return source.flatMap(model => {
    const id = typeof model === 'string' ? model.trim()
      : firstString(model?.id, model?.key, model?.model, model?.name);
    if (!id || id === '[object Object]' || seen.has(id)) return [];
    seen.add(id);
    const label = firstString(model?.display_name, model?.displayName, model?.name, id);
    return [{ id, label }];
  });
}
