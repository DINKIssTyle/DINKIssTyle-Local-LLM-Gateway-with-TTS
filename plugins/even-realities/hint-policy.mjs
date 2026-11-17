export const HINT_LABELS={key:'정보',term:'용어',verify:'확인',schedule:'일정',recap:'요약'};

// Legacy recap labels remain readable in history; automatic selection is information-only.
export const INFORMATION_HINT_KINDS=['key','term','verify','schedule'];
export function isInformationHintPlan(plan){return !!plan && INFORMATION_HINT_KINDS.includes(plan.kind || 'key') && ['internal','web','memory'].includes(plan.source);}
