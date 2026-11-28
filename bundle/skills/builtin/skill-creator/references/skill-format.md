# User Skill Standard Scaffolding & Format

All user skills live under `<AppDataDir>/skills/user/<skill-id>/` and are exposed under the `user:<skill-id>` namespace.

## Standard Directory Layout

```text
<skill-id>/
├── SKILL.md              # Required: YAML frontmatter + Markdown instructions (<= 64 KB)
├── agents/
│   └── openai.yaml       # Required: UI display metadata (<= 16 KB)
└── references/           # Optional: supporting Markdown/text/JSON/YAML docs (<= 64 KB each)
    └── <name>.md
```

## Validation Rules

- `skill_id`: lowercase ASCII letters (`a-z`), digits (`0-9`), and single hyphens (`-`); max 64 characters; no leading/trailing `-`, no `--`.
- `display_name`: required, max 80 characters.
- `SKILL.md` allowed frontmatter keys: `id`, `name`, `description`, `version`, `permissions`, `platforms`, `license`, `allowed-tools`, `metadata`, `requires-tools`, `triggers`, `discovery`, `priority`.
- `discovery`: `on-trigger` (default) or `always`.
- `requires-tools`: list of real registered tool names from `list_skills` (`available_tools`).
- `references/`: plain filenames only (no path separators or hidden files) with `.md`, `.txt`, `.json`, `.yaml`, or `.yml` extensions.
