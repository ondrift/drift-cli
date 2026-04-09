# drift-cli

Command-line interface for the Drift platform. Manages accounts, slices, and deployments from the terminal.

## Install

```bash
go install ./...
```

The binary is named `drift`.

## Commands

### Account

```
drift account signup              Sign up for a Drift account
drift account login               Log in and store session token
drift account usage               View current usage and limits
drift account upgrade <tier>      Upgrade account tier
```

### Slice lifecycle

```
drift slice create                          Open the configurator in a browser; type the name in the form
drift slice create <name>                   Same, but pre-fills the form with the given name
drift slice create <name> --headless [-t hacker]
                                            Create a slice with a named tier (no browser, for CI/scripts)
drift slice resize <name>                   Open the configurator to resize an existing slice
drift slice list                            List slices (* marks active)
drift slice use <name>                      Set active slice
drift slice delete <name>                   Delete a slice (with confirmation)
```

### Deploy

```
drift deploy drift.yaml           Deploy from a manifest (requires active slice)
drift plan drift.yaml             Preview what a deploy would do
```

### Atomic functions

```
drift atomic new <name>           Scaffold a new function
drift atomic deploy <dir>         Deploy a function to the active slice
drift atomic run <dir>            Run a function locally for development
```

### Backbone

```
drift backbone secret set|get|delete|list    Manage secrets
drift backbone blob put|get|delete|list      Manage blobs
drift backbone queue push|pop|peek|len       Manage queues
drift backbone lock acquire|release|renew    Manage distributed locks
drift backbone nosql write|read|drop         Manage NoSQL collections
drift backbone cache set|get|del|exists      Manage cache entries
```

### Canvas

```
drift canvas deploy <dir>         Deploy a static site
```

## Session

Login credentials and the active slice are stored in `~/.drift/session.json`. The active slice is preserved across logins.

All authenticated requests include:
- `Authorization: Bearer <jwt>` — user identity
- `X-Slice: <active-slice>` — target slice

## Talks to

- **drift-core API** — All commands hit the API gateway at the configured Drift API URL
