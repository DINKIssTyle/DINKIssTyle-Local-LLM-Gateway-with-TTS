# Bundled skills

This directory is reserved for read-only skills shipped with the application.
Each enabled skill will live in its own directory and contain a `SKILL.md`.

The server validates these skills, selects request-relevant instructions, and
injects them at the chat prompt boundary. User skills belong in the writable
`skills/user` directory shown in the server window and use a separate `user:`
namespace.
