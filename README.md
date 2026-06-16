# cofiswarm-rag-worker

Cofiswarm component: `rag-worker`.

- Layout: [REPO-STANDARD-LAYOUT](https://github.com/keepdevops/cofiswarmdev/blob/main/docs/REPO-STANDARD-LAYOUT.md)
- Migration: [MIGRATION-SPRINTS](https://github.com/keepdevops/cofiswarmdev/blob/main/docs/MIGRATION-SPRINTS.md)

## FHS paths

| Path | Purpose |
|------|---------|
| `/etc/cofiswarm/rag-worker/` | config |
| `/var/lib/cofiswarm/rag-worker/` | state |
| `/var/log/cofiswarm/rag-worker/` | logs |

## Test

```bash
./test/scripts/assert-layout.sh rag-worker
```
