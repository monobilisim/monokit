---
session: ses_0586
updated: 2026-07-28T07:48:21.825Z
---

# Session Summary

## Goal
Implement a new `mongodbHealth` core tool in monokit (Go) that monitors MongoDB standalone and replicaset health, routing non-actionable conditions to Zulip-only alarms and app-impacting conditions to Zulip+Redmine issue creation — planning phase completed, now in direct implementation phase (no git commit/push).

## Constraints & Preferences
- User (Turkish): planning-first requested initially, then approved full plan and said "kendin yap ya da oracle'a yaptır fixer'ın limiti doldu" — implement directly, do not delegate to fixer (quota exhausted).
- User: "commit atmayalım" — do NOT run `git commit` or `git push`.
- Build as **core tool** (not plugin), `//go:build linux` only, like mysqlHealth/pgsqlHealth.
- Connection via single URI string, not discrete host/port/user fields.
- No `primary_switch_hook` shell-hook feature (user declined).
- Config lives inside shared `config/db.yml` as new `mongodb:` top-level key (user: "config de db.conf içinde olsun"), not a separate file.
- `oplog_window_critical_hours` = **1** (user explicitly set this, said apply rest as-is).
- Follow mysqlHealth/redisHealth/pritunlHealth code conventions exactly (see Critical Context).
- fix-1 (fixer) task session is stale/untrustworthy — despite Job Board showing "completed, reconciled", verified via `git status --short` (clean) and `ls mongodbHealth/` (not found) that NO files were actually written. Do not reuse or trust fix-1.

## Progress
### Done
- [x] Full repo research completed and verified via direct Read/Glob/Grep (exp-1 explorer's findings were discarded as fabricated/hallucinated — do not reuse exp-1).
- [x] All architectural decisions approved by user (build model, config location, URI-only connection, no shell hook, oplog threshold=1).
- [x] Zulip/Redmine routing table finalized per condition (12 conditions mapped to service keys + alarm-only vs alarm+redmine).
- [x] Package layout for `mongodbHealth/` approved (main.go, types.go, client.go, standalone.go, replicaset.go, ui.go, optional alarm.go).
- [x] **Edited `common/db/main.go`**: added new `Mongodb` struct (Uri, Connect_timeout_seconds, Alarm.Enabled, Limits.Connections_percent/Cache_percent, Replicaset.{Min_secondaries, Lag_warn_seconds, Lag_critical_seconds, Oplog_window_warn_hours, Oplog_window_critical_hours, Primary_election_grace_seconds}) and added `Mongodb Mongodb` field to `DbHealth` struct. **CONFIRMED APPLIED.**
- [x] **Edited `config/db.yml`**: appended `mongodb:` block (uri, connect_timeout_seconds, alarm.enabled, limits, replicaset with oplog_window_critical_hours=1) right after existing `mysql:` block. **CONFIRMED APPLIED** — file now has 3 top-level keys: postgres, mysql, mongodb.
- [x] Read and confirmed all remaining reference file contents needed for implementation: `mysqlHealth/main.go` (full), `mysqlHealth/mysql.go` (full, incl. CheckReceiveQueue/CheckFlowControl dual-alerting precedent), `mysqlHealth/ui.go` (full, incl. NewMySQLHealthData/RenderCompact/RenderAll pattern), `redisHealth/shared_types.go`, `redisHealth/main.go`, `pritunlHealth/main.go`+`types.go`+`go.mod` (mongo-driver v2 usage), `common/healthdb` API, `linux-cmds.go`/`nonlinux-cmds.go`/`main.go` CLI registration wiring, `common/plugins.go` KnownPlugins list, README.md structure.
- [x] Confirmed root `go.mod` (module github.com/monobilisim/monokit, go 1.24.3) does NOT yet have `go.mongodb.org/mongo-driver/v2` dependency.

### In Progress
- [ ] Nothing actively mid-edit right now — about to start writing `mongodbHealth/*.go` files.

### Blocked
- (none)

## Key Decisions
- **Core tool, not plugin**: Mongo monitoring should always be available like mysqlHealth/pgsqlHealth; `//go:build linux` tag only, register via `client.WrapperGetServiceStatus("mongodbHealth")` (matching mysqlHealth/redisHealth convention, NOT pritunlHealth's direct `common.WrapperGetServiceStatus` variant).
- **Config in shared db.yml**: avoids fragmenting config files; extends existing `common.DbHealth` struct pattern already shared by mysql/postgres.
- **URI-only connection string**: simpler, matches how mongo URIs naturally encode replicaSet/auth/hosts.
- **No shell hook for primary switch**: user declined added complexity; primary change is purely an informational Zulip alarm via `common.Alarm(...)`, no state-tracked CheckDown/CheckUp needed (mirrors mysqlHealth's `CheckDB()` informational broadcast pattern), with `healthdb.GetJSON/PutJSON("mongodbHealth","primary",...)` used only to detect the change (not to gate alarm dedup).
- **Alarm+Redmine dual-call pattern for actionable conditions**: exact precedent from `mysqlHealth/mysql.go` `CheckFlowControl()` (calls both `common.AlarmCheckDown` AND `issues.CheckDown`) vs `CheckReceiveQueue()` (calls only `common.AlarmCheckDown`) — this is the established codebase pattern for the user's Zulip-only vs Zulip+Redmine split.
- **fix-1 session discarded**: Background Job Board is unreliable here; verified empty git diff proves no files were written despite "completed" status.

## Next Steps
1. Run `go get go.mongodb.org/mongo-driver/v2@v2.2.2` in root module to add the dependency.
2. Create `mongodbHealth/types.go` — HealthData struct (Service/Connection/Standalone/ReplicaSet nested sections, MemberInfo, LastChecked), modeled on `redisHealth/shared_types.go` + `mysqlHealth/ui.go` struct style.
3. Create `mongodbHealth/client.go` — `connectMongo(uri string, timeoutSeconds int)` using `go.mongodb.org/mongo-driver/v2/mongo`, `.../mongo/options`, `.../mongo/readpref`, `.../bson` (v2 driver: `mongo.Connect(clientOptions)` takes no context arg; see pritunlHealth pattern in Critical Context).
4. Create `mongodbHealth/standalone.go` — serverStatus-based checks (connections %, WiredTiger cache %, ticket exhaustion) using CheckReceiveQueue/CheckFlowControl dual-alerting templates per routing table.
5. Create `mongodbHealth/replicaset.go` — replSetGetStatus + oplog window computation; primary/secondary/lag/oplog checks per routing table; primary-change detection via `healthdb.GetJSON/PutJSON("mongodbHealth","primary",...)`.
6. Create `mongodbHealth/ui.go` — HealthData rendering via `common.DisplayBox`/`SectionTitle`/`StatusListItem`/`SimpleStatusListItem`, modeled exactly on `mysqlHealth/ui.go`'s `NewXHealthData()`/`RenderCompact()`/`RenderAll()` pattern.
7. Create `mongodbHealth/main.go` — `init()`+`RegisterComponent`, `DetectMongoDB()` (checks `common.ConfExists("db")` AND `DbHealthConfig.Mongodb.Uri != ""` before real ping), `Main(cmd, args)` following mysqlHealth/main.go flow exactly (ScriptName, TmpDir, common.Init, ConfInit("db",...), WrapperGetServiceStatus, connect, dispatch standalone vs replicaset checks, RenderAll, no PostHostHealth call needed to match mysqlHealth convention).
8. Edit `linux-cmds.go`: add import `"github.com/monobilisim/monokit/mongodbHealth"` + `func MongodbCommandAdd()` (Use:"mongodbHealth", Short:"MongoDB Health", Run: mongodbHealth.Main).
9. Edit `nonlinux-cmds.go`: add no-op stub `func MongodbCommandAdd() {}`.
10. Edit `main.go`: add `MongodbCommandAdd()` call near `PgsqlCommandAdd()`.
11. Edit `common/plugins.go`: add `"mongodbHealth"` to `KnownPlugins` list.
12. Edit `README.md`: add `## Tools` bullet entry for mongodbHealth near mysqlHealth/pgsqlHealth, matching format (see Critical Context for exact template).
13. Run `go build ./...` and `go vet ./mongodbHealth/...` to verify compilation.
14. Do NOT run `git commit` or `git push`.

## Critical Context

**Approved `config/db.yml` mongodb block (APPLIED):**
```yaml
mongodb:
  uri: "mongodb://user:pass@localhost:27017/?replicaSet=rs0"
  connect_timeout_seconds: 5
  alarm:
    enabled: true
  limits:
    connections_percent: 80
    cache_percent: 95
  replicaset:
    min_secondaries: 1
    lag_warn_seconds: 10
    lag_critical_seconds: 60
    oplog_window_warn_hours: 24
    oplog_window_critical_hours: 1
    primary_election_grace_seconds: 30
```

**Approved `common/db/main.go` Mongodb struct (APPLIED):**
```go
type Mongodb struct {
    Uri                     string
    Connect_timeout_seconds int
    Alarm struct{ Enabled bool }
    Limits struct {
        Connections_percent int
        Cache_percent       int
    }
    Replicaset struct {
        Min_secondaries                int
        Lag_warn_seconds               int
        Lag_critical_seconds           int
        Oplog_window_warn_hours        float64
        Oplog_window_critical_hours    float64
        Primary_election_grace_seconds int
    }
}
// DbHealth struct now: DbHealth{ Mysql Mysql; Postgres Postgres; Mongodb Mongodb }
```

**Zulip/Redmine routing table (service key → Zulip/Redmine):**
| Condition | service key | Zulip | Redmine |
|---|---|---|---|
| Standalone/replica connection down | `mongodb-connection` | yes | yes |
| WiredTiger ticket exhaustion | `mongodb-ticket-exhaustion` | yes | yes |
| Primary changed (informational) | `mongodb-primary-change` | yes | no |
| No primary elected within grace period | `mongodb-primary-absent` | yes | yes |
| Secondary down but >= min_secondaries remain | `mongodb-secondary-unhealthy` | yes | no |
| Healthy secondary count < min_secondaries | `mongodb-secondary-quorum` | yes | yes |
| Lag warn (between warn/critical) | `mongodb-lag-warn` | yes | no |
| Lag > critical | `mongodb-lag-critical` | yes | yes |
| Oplog window warn | `mongodb-oplog-warn` | yes | no |
| Oplog window < critical (1h) | `mongodb-oplog-critical` | yes | yes |
| Connections % > limit | `mongodb-connections-limit` | yes | no |
| Cache % > limit | `mongodb-cache-limit` | yes | no |

**Code templates confirmed (exact, from mysqlHealth/mysql.go):**
```go
// Zulip-only precedent (CheckReceiveQueue)
if count > limit {
    msg := fmt.Sprintf("...")
    // Only send alarm (webhook), no Redmine issue ... as requested
    common.AlarmCheckDown("receive queue", msg, false, "", "")
} else {
    common.AlarmCheckUp("receive queue", fmt.Sprintf("... OK"), false)
}

// Zulip+Redmine precedent (CheckFlowControl)
if paused > threshold {
    msg := fmt.Sprintf("...")
    msgTr := fmt.Sprintf("...")
    subject := fmt.Sprintf("%s için ...", common.Config.Identifier)
    common.AlarmCheckDown("flow control", msg, false, "", "")
    issues.CheckDown("flow-control", subject, msgTr, false, 0)
} else {
    common.AlarmCheckUp("flow control", "Flow Control OK", false)
    issues.CheckUp("flow-control", msgTr)
}
```
Import: `issues "github.com/monobilisim/monokit/common/redmine/issues"`.

**Primary-change informational alarm pattern:** use `common.Alarm(message, "", "", false)` directly (no CheckDown/CheckUp state), message format `"[" + ScriptName + " - " + Config.Identifier + "] [:info:] Primary changed from X to Y"`. State tracking for detecting the change itself: `healthdb.PutJSON("mongodbHealth", "primary", primaryHostString, nil, time.Now())` / `healthdb.GetJSON("mongodbHealth", "primary")` (package `common/healthdb`, functions `PutJSON(module, key, json string, nextCheckAt *time.Time, cachedAt time.Time) error`, `GetJSON(module, key string) (json string, cachedAt time.Time, nextCheckAt *time.Time, found bool, err error)`, `Delete(module, key string) error`).

**mongo-driver v2 connect pattern (from pritunlHealth, root module needs `go get go.mongodb.org/mongo-driver/v2@v2.2.2`):**
```go
import (
    "go.mongodb.org/mongo-driver/v2/bson"
    "go.mongodb.org/mongo-driver/v2/mongo"
    "go.mongodb.org/mongo-driver/v2/mongo/options"
    "go.mongodb.org/mongo-driver/v2/mongo/readpref"
)
clientOptions := options.Client().ApplyURI(uri)
client, err := mongo.Connect(clientOptions) // v2: no context arg
defer func() {
    ctxD, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    client.Disconnect(ctxD)
}()
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()
err = client.Ping(ctx, readpref.Primary())
```

**mysqlHealth/main.go Main() flow to mirror exactly:**
```go
func Main(cmd *cobra.Command, args []string) {
    version := "x.y.z"
    common.ScriptName = "mongodbHealth"
    common.TmpDir = common.TmpDir + "mongodbHealth"
    common.Init()
    common.ConfInit("db", &DbHealthConfig)
    healthData = NewMongoHealthData()
    healthData.Version = version
    client.WrapperGetServiceStatus("mongodbHealth")
    // connect; on failure: common.AlarmCheckDown("ping", errMsg, false, "", ""); set healthData conn fields; print RenderAll(); return
    // on success: common.AlarmCheckUp("ping", ..., false); run check funcs sequentially
    fmt.Println(healthData.RenderAll())
}
```
`DetectMongoDB()` mirrors `DetectMySQL()`: check `common.ConfExists("db")`, load local `var detectConf db.DbHealth; common.ConfInit("db", &detectConf)`, check `detectConf.Mongodb.Uri != ""`, attempt real connect/ping, return true only on success.

**CLI registration wiring (linux-cmds.go / nonlinux-cmds.go / main.go):**
```go
// linux-cmds.go
func MongodbCommandAdd() {
    var mongodbHealthCmd = &cobra.Command{
        Use:   "mongodbHealth",
        Short: "MongoDB Health",
        Run:   mongodbHealth.Main,
    }
    RootCmd.AddCommand(mongodbHealthCmd)
}
// nonlinux-cmds.go
func MongodbCommandAdd() {}
// main.go — add call near PgsqlCommandAdd():
PgsqlCommandAdd()
MongodbCommandAdd()
```

**common/plugins.go KnownPlugins** (add `"mongodbHealth"` to this slice — it's a general component-name registry mixing core tools and true plugins, not plugin-exclusive):
```go
var KnownPlugins = []string{"k8sHealth", "mysqlHealth", "pgsqlHealth", "redisHealth", "zimbraHealth", "traefikHealth", "rmqHealth", "pritunlHealth", "wppconnectHealth", "pmgHealth", "esHealth", "postalHealth"}
```

**README.md Tools entry template to mimic** (mysqlHealth's exact confirmed entry, insert mongodbHealth entry near mysqlHealth/pgsqlHealth):
```
- mysqlHealth
  - Checks MySQL health, including read and write operations.
  - Supports Galera Cluster monitoring (Receive Queue and Flow Control).
  - Sends alarm notifications to a Slack webhook.
  - Opens Redmine issues for Galera Flow Control issues.
  - Config: `/etc/mono/db.yaml`
```

**mysqlHealth/ui.go rendering helpers (`common/display.go`):** `DisplayBox(title, content string) string`, `SectionTitle(title string) string`, `StatusListItem(label, statusPrefix, limits, current string, isSuccess bool) string`, `SimpleStatusListItem(label, expectedState string, isSuccess bool) string`. Pattern: `NewXHealthData() *XHealthData` constructor → `RenderCompact() string` (builds via strings.Builder using the above helpers, sections separated by `"\n\n"+SectionTitle(...)`) → `RenderAll() string` (`title := "monokit mongodbHealth"` + version, `content := RenderCompact()`, `return common.DisplayBox(title, content)`).

**Background Job Board note:** `fix-1`/`exp-1` sessions both marked "completed, reconciled" but exp-1's findings were fabricated (discarded) and fix-1 wrote no files despite claim (discarded). Do not reuse either for this task — proceed with direct implementation only.
