// Package marq exposes the repo-root skills/ tree (an Agent Skills pack) to the
// binary. It lives at the root because go:embed can't reach parent directories,
// and skills/ must stay at the root for `npx skills` and plugin discovery.
package marq

import "embed"

//go:embed skills
var Skills embed.FS
