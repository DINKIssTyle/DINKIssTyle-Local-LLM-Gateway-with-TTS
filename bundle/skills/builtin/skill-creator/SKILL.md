---
name: skill-creator
description: Plan, confirm with the user, and scaffold a new user skill in the local user skills directory. Use for 스킬 만들기, 스킬 제작, 새 스킬 추가, 스킬 수정, or create skill requests.
requires-tools: [list_skills, create_user_skill]
triggers: [스킬 만들, 스킬 제작, 새 스킬, 스킬 추가, 스킬 생성, 스킬 수정, create skill, new skill, make a skill, skill-creator]
---

# 스킬 제작

사용자 스킬을 **계획 수립 → 사용자 승인 → 표준 스캐폴딩 구현** 2단계로 제작한다. **스킬 ID(`builtin:skill-creator`)는 도구가 아니다.** 실제 도구는 `list_skills`, `read_skill`, `create_user_skill`이다.

## 절대 규칙

- **사용자 승인 전에는 절대 `create_user_skill`을 호출하지 않는다.** 1턴에서 계획 제시와 승인을 요청하고 응답을 마친다.
- 내장 스킬(`builtin:*`)은 수정하지 않는다. 모든 생성·수정은 사용자 스킬 폴더(`skills/user/<skill-id>/`, `user:<skill-id>`)에만 수행한다.

## 1단계: 계획 수립 (생성 금지)

1. `list_skills`를 `{}`로 호출해 기존 스킬 ID 충돌과 `available_tools`의 실제 도구명을 확인한다. 기존 `user:*` 스킬 수정이면 `read_skill`로 먼저 읽는다.
2. 핵심 요구사항이 모호하면 1~2개만 짧게 묻고, 명확하면 바로 계획서를 제시한다:
   - 스킬 ID (`user:<skill-id>`, 소문자·숫자·하이픈) 및 표시 이름 (`display_name`)
   - 목적·트리거 (`description`, `triggers`) 및 필요 도구 (`requires-tools`, 실제 도구명만)
   - 핵심 동작 절차 요약 및 생성 파일 (`SKILL.md`, `agents/openai.yaml`, 선택 `references/*.md`)
   - 기존 사용자 스킬 덮어쓰기 여부
3. **"이 계획대로 생성할까요?"** 로 승인을 요청하고 도구 생성 없이 종료한다.

## 2단계: 승인 후 구현

1. 사용자가 명시적으로 승인한 다음 턴에만 `create_user_skill`을 `user_approved: true`로 호출한다(기존 스킬 수정 승인 시 `overwrite: true`). 수정 요청이면 계획을 고쳐 재승인받는다.
2. `instructions`에는 YAML frontmatter(`---`) 없이 순수 Markdown 본문만 넣는다. 본문에는 실제 도구명만 쓰고, 실패 시 거짓 성공을 금지하며, 간결하게 작성한다(상세 규격: [skill-format.md](references/skill-format.md)).
3. 생성 후 네임스페이스(`user:<skill-id>`), 생성 파일, 호출 방법(`/` 메뉴 또는 `$user:<skill-id>`, 재시작 불필요)과 경고를 안내한다.
