# Service

A service holds application logic: use cases that span several domains, the
queue, email, or config. It answers "what should happen across entities?"
rather than "what is true about this entity?". It is not a pass-through
wrapper around a single domain method.

## Files

### services/<name>.go
```go
package services

import (
	"context"

	"app/config"
	"app/domains"

	"github.com/mbvlabs/andurel/pkg/storage"
)

type Identity struct {
	users  domains.Users
	tokens domains.Tokens
	queue  storage.InsertQueue
	pepper string
}

func NewIdentity(
	users domains.Users,
	tokens domains.Tokens,
	queue storage.InsertQueue,
	authCfg config.Auth,
	mailCfg config.Mail,
) Identity {
	return Identity{users: users, tokens: tokens, queue: queue, pepper: authCfg.Pepper}
}

func (i Identity) AuthenticateUser(ctx context.Context, data LoginData) (domains.User, error) {
	// coordinate user + token + email; the only place these domains meet
	return domains.User{}, nil
}
```

## Rules

- A service may depend on several domains, the queue, email, and config.
- A service never imports `domains/internal/queries`; it calls domain methods.
- Put use cases here, not validation of a single entity (that is the domain).
- Keep constructors explicit (`NewIdentity(...)`); the runtime wires them.
- Transactions that span domains are started here.

## Checklist

- [ ] Constructor lists every dependency explicitly.
- [ ] No import of `domains/internal/queries`.
- [ ] The service adds orchestration rather than forwarding one call.
- [ ] Wired once in `internal/runtime`.
