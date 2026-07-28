// marq — a universal cyber assistant.
// Copyright (C) 2026 marq contributors
//
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License as published by the Free
// Software Foundation, either version 3 of the License, or (at your option) any
// later version. It is distributed WITHOUT ANY WARRANTY. See the LICENSE file or
// <https://www.gnu.org/licenses/> for details.

// Package pi embeds the host-side integration files so `marq shim` / `marq
// prompt` can emit them from the binary. Without this, driving marq with a
// local model needs a git clone alongside the pulled image — and a shim cloned
// from main can then be talking to an older image than it was written for.
// Embedding pins the shim and prompts to the binary they drive.
package pi

import _ "embed"

// Shim is the pi/marq host script: `docker exec`s into a long-lived container.
//
//go:embed marq
var Shim string

// SystemPrompt replaces Pi's default agent framing with marq's scope-first one.
// Without it a small model narrates instead of driving the tools.
//
//go:embed SYSTEM.md
var SystemPrompt string

// Skill is the portable markdown skill: calling convention, tool discovery,
// scope rules and a few-shot for small models.
//
//go:embed SKILL.md
var Skill string
