# ForgeQueue Build Map

Branch: `new` (single commit, 14 files, about 450 lines).
Scope: structure, file paths, functions, and the exact work remaining. No implementation code.

---

## 1. What exists now

- **Go**
  - Length-prefixed framing: `Orchestrator/internal/ipc/frame.go`
  - Listener, accept, HELO: `Orchestrator/internal/ipc/conn.go`
  - Process spawn with its own process group: `Orchestrator/internal/worker/spawn.go`
  - Types: `Worker`, `Task`, `TaskResult`
- **Python**
  - Socket client, framing, HELO, task loop
  - Logger
  - Handler registry with one handler (`image.resize`)
- **Missing entirely:** Redis, worker lifecycle (start / run / restart), the pool, the Go entrypoint, webhooks, the Node API, `shared/`, Docker, tests.
- **README mismatch:** the README says `manager/`, `workers/`, `api/`, `shared/`. The repo has `Orchestrator/`, `Worker/`, `worker/` and no `api/` or `shared/`.

---

## 2. Bugs in existing code

These break the first run, so they come before new code.

1. **`Orchestrator/internal/worker/spawn.go`** builds one string `"path socket"` and passes it as a single argument, so Python looks for a script literally named `worker/main.py /tmp/x.sock`.
   Target: `exec.Command("python3", scriptPath, socketPath)` with two separate arguments.
2. **`worker/main.py`** sits in a lowercase folder, but `global_logger`, `handlers`, `ipc`, `response_generator` are in `Worker/`. Imports fail, and two folders differing only by case collide on macOS and Windows.
   Target: one folder, `Worker/main.py`.
3. **`Worker/global_logger.py`** never sets `self.taskId` in `__init__`, so the first log call raises `AttributeError` (and `main.py` logs before any task exists). The timestamp key is misspelled `tempstamp`.
   Target: `taskId` starts as `None`, key is `timestamp`, `taskId` is reset to `None` after each task.
4. **`Worker/handlers/image_resize.py`** builds the output path with `replace(".jpg", "_resized.jpg")`. For `.png`, `.jpeg`, `.JPG` the output path equals the input path, so the original image is overwritten.
   Target: `os.path.splitext`, write into the shared output folder.
5. **Status vocabulary disagrees.** The handler returns `done` / `fail`, `main.py` uses `failed` / `fail`, the failure result uses key `outputFile` while the handler uses `outputPath`, Go has `complete` / `stopped`.
   Target: one vocabulary (see section 3).
6. **`ipc/conn.go`** has the typo `AccpetWorker`, and `Accept` / `WaitForHelo` block forever if the Python process dies at startup.
7. **`ipc/frame.go`** allocates whatever length the peer sends, with no upper bound.
8. **`worker/main.py`**: `graceful_shutdown` closes a variable named `client` that is never assigned (the real socket is `sock`), so the signal handler closes nothing.
   Target: one socket variable, closed in a `finally` block of `main()`.

---

## 3. Contracts (must match in Go, Python, Node)

### 3.1 IPC between Go and Python (one Unix socket per worker)

- Frame = 4-byte big-endian length + payload. Maximum payload 16 MiB on both sides.
- Worker connects and sends one frame containing `HELO`.
- Orchestrator sends one JSON frame: `taskId`, `type`, `payload`, `createdAt`.
- Worker replies with one JSON frame: `taskId`, `status`, `result`, `error`.
  - `status` is `"done"` or `"failed"`.
  - On done: `result` is an object, `error` is null.
  - On failed: `result` is null, `error` is a string.
- One task in flight per worker. The orchestrator closing the socket is the shutdown signal.

### 3.2 Redis keys

| Key | Type | Meaning |
|---|---|---|
| `forge:queue:pending` | LIST | taskIds waiting |
| `forge:queue:processing` | LIST | taskIds currently claimed |
| `forge:queue:delayed` | ZSET | taskIds scored by retry time |
| `forge:queue:dead` | LIST | taskIds that exhausted retries |
| `forge:queue:webhooks` | LIST | taskIds awaiting webhook delivery |
| `forge:queue:webhooks:processing` | LIST | webhook deliveries currently claimed |
| `forge:queue:webhooks:delayed` | ZSET | webhook deliveries waiting for their next retry |
| `forge:job:<taskId>` | HASH | the job record |

Job hash fields: `taskId`, `type`, `payload` (JSON string), `status`, `attempts`, `maxAttempts`, `webhookUrl`, `createdAt`, `startedAt`, `finishedAt`, `result` (JSON string), `error`, `webhookStatus`, `webhookAttempts`.

Job status values: `queued`, `running`, `done`, `failed`, `dead`.

All Redis values are strings. Integers are stored as decimal strings. Timestamps are UTC ISO-8601 with a trailing `Z` (for example `2026-10-12T08:22:05Z`). `payload` and `result` are JSON text.

The API writes the hash first, then pushes the id to `pending`, so Go never claims an unknown id.

### 3.3 Retry policy

- Worker crash, timeout, or garbage response: retried, up to 3 attempts, delays 2s / 10s / 30s.
- Handler returns `failed` (bad input): not retried, job status `failed`.
- After the last attempt: job moves to `dead`.

---

## 4. Go orchestrator (all under `Orchestrator/`)

### `go.mod` (CHANGE)
- Add `github.com/redis/go-redis/v9`. Redis 6.2 or newer is needed for `BLMOVE`.

### `internal/config/config.go` (NEW)
- `type Config`: `RedisAddr`, `RedisPassword`, `WorkerCount`, `PythonBin`, `WorkerScript`, `SocketDir`, `SharedDir`, `TaskTimeout`, `StartTimeout`, `MaxAttempts`, `ShutdownGrace`, `JobTTL`, `WebhookSecret`, `WebhookTimeout`, `HealthAddr`, `LogLevel`. Defaults are listed in section 8.
- `func Load() (Config, error)` reads environment variables with defaults and validates (`WorkerCount >= 1`, script file exists, `SocketDir` plus the longest socket file name stays under the OS path limit).

### `internal/types/ipc.go` (CHANGE)
- Add constants `StatusDone = "done"` and `StatusFailed = "failed"` for `TaskResult.Status`.

### `internal/types/job.go` (NEW)
- `type JobStatus string`: queued, running, done, failed, dead.
- `type Job`: `TaskId`, `Type`, `Payload`, `Status`, `Attempts`, `MaxAttempts`, `WebhookURL`, `CreatedAt`, `StartedAt`, `FinishedAt`, `Result`, `Error`.
- `func (j *Job) ToTask() Task` builds the IPC message.

### `internal/types/worker.go` (CHANGE)
- Worker status becomes `starting`, `free`, `busy`, `dead`. "complete" has no meaning for a long-lived process, and `Failed = "stopped"` mismatches its own name.
- Add fields `RestartCount`, `StartedAt`.
- Add methods `SetStatus(s)` and `GetStatus()` that take the mutex.

### `internal/ipc/frame.go` (CHANGE)
- Add `const MaxFrameSize = 16 MiB`. `ReadFrame` rejects any length above it.
- `WriteFrame` builds header + payload in one buffer and does a single `conn.Write`. It rejects payloads over the max.

### `internal/ipc/conn.go` (CHANGE)
- Rename `AccpetWorker` to `AcceptWorker` and give it a timeout (UnixListener deadline) so a dead worker cannot hang the accept.
- `WaitForHelo(conn, timeout)` sets a read deadline, reads, then clears the deadline.
- `CreateListener` also sets the socket file mode to `0600`.

### `internal/worker/spawn.go` (CHANGE)
- `SpawnWorker(pythonBin, scriptPath, socketPath string, env []string)` returns `(cmd, stdout io.ReadCloser, stderr io.ReadCloser, error)`. Pipes are created before `Start`.
- The child gets an explicit environment (`cmd.Env`) containing only `SHARED_DIR`, `LOG_LEVEL`, `PYTHONUNBUFFERED=1`, and `PATH`. It never receives the Redis password or the webhook secret.
- Add `KillGroup(cmd)`: `SIGKILL` to the negative pid, which kills the whole process group.
- Add `TermGroup(cmd)`: `SIGTERM`.
- Optional, Linux only: `Pdeathsig SIGKILL` so Python dies if the orchestrator dies.

### `internal/worker/worker.go` (NEW) — the core of the project
- Sentinel errors: `ErrTimeout`, `ErrWorkerDied`, `ErrBadResponse`.
- `func New(id, scriptPath, socketDir string) *types.Worker` sets `SocketPath` to `socketDir/worker-<id>.sock`.
- `func Start(w, scriptPath, startTimeout) error`, in order:
  1. `CreateListener`
  2. `SpawnWorker`
  3. start the log pipe goroutines
  4. `AcceptWorker` with timeout (a goroutine closes the listener if the process exits early)
  5. `WaitForHelo`
  6. close the listener, store `Conn`, set status `free`
  7. start the waiter goroutine, which calls `cmd.Wait()` only after both log pipes have drained
- `func RunTask(ctx, w, task, timeout) (*types.TaskResult, error)`:
  1. set status `busy` and `w.Job`
  2. marshal and `WriteFrame`
  3. set the read deadline to the timeout
  4. `ReadFrame` and unmarshal
  5. check the response `taskId` equals the task `taskId`
  6. clear the deadline, set status `free`
  - Timeout returns `ErrTimeout`, EOF returns `ErrWorkerDied`, bad JSON or wrong id returns `ErrBadResponse`.
- `func Stop(w, grace)`: close `Conn` (Python sees EOF), `TermGroup`, wait up to grace, then `KillGroup`, then remove the socket file.
- `func Restart(w, ...)`: `Stop` then `Start`.

### `internal/worker/logpipe.go` (NEW)
- `func PipeLogs(r io.Reader, workerID, stream string, log *slog.Logger)` reads lines with `bufio.Scanner` (enlarged buffer). Each line is parsed as the Python logger's JSON and re-emitted through `slog` with `workerID` and `taskId` attributes. Non-JSON lines are logged raw. stderr maps to the error level.

### `internal/queue/keys.go` (NEW)
- Key builders: `Pending()`, `Processing()`, `Delayed()`, `Dead()`, `Webhooks()`, `Job(id)`.

### `internal/queue/queue.go` (NEW)
- `func New(addr, password) (*Queue, error)` pings on creation.
- `Claim(ctx, wait) (*types.Job, error)`: `BLMOVE pending processing RIGHT LEFT` (producers use `LPUSH`, so this is FIFO), then `HGETALL`. If the hash is missing, the id is removed from processing and logged. Returns `nil, nil` on timeout. Exact commands for every function are in section 12.
- `MarkRunning(ctx, job)`: sets status and `startedAt`, increments attempts.
- `Complete(ctx, job, result)`: one transaction, `HSET` done + result + `finishedAt`, `EXPIRE` with `JobTTL`, then `LREM` from processing.
- `Fail(ctx, job, msg)`: same, with status `failed`.
- `Retry(ctx, job, delay)`: removes from processing, adds to delayed with score now + delay. If attempts have reached max, calls `Dead` instead.
- `Dead(ctx, job, reason)`: pushes to dead, sets status `dead`.
- `PromoteDelayed(ctx)`: runs every second as a Lua script (`ZRANGEBYSCORE` up to now, `LPUSH` to pending, `ZREM`).
- `Recover(ctx)`: at startup moves everything in processing back to pending (status `queued`). A job whose `attempts` already reached `maxAttempts` goes to `dead` instead, so a job that keeps crashing the orchestrator cannot loop forever. This assumes a single orchestrator instance. Running several needs a per-instance processing list with a lease.
- `EnqueueWebhook`, `ClaimWebhook`, `AckWebhook`, `ScheduleWebhookRetry`, `PromoteWebhookDelayed`, `SetWebhookResult` for the delivery loop (commands in section 12).

### `internal/pool/pool.go` (NEW)
- `type Pool`: workers, config, queue, logger.
- `New(cfg, q, log) *Pool`.
- `Start(ctx) error`: starts all workers in parallel, fails if any cannot start, then launches one run-loop goroutine per worker.
- `runLoop(ctx, w)` repeats:
  1. `Claim` a job
  2. `MarkRunning`
  3. `RunTask`
  4. result `done` → `Complete`; result `failed` → `Fail`; error `ErrTimeout` / `ErrWorkerDied` / `ErrBadResponse` → restart the worker with backoff (1s doubling to 30s, reset after a success) and `Retry`
  5. if the job has a `webhookUrl`, `EnqueueWebhook`
- `Snapshot()` returns worker statuses for the health endpoint.
- `Shutdown(ctx)`: stop claiming, wait for in-flight tasks until the grace deadline, then `Stop` every worker. A job still unfinished stays in processing, and `Recover` picks it up on the next start.

### `internal/webhook/sign.go` and `deliver.go` (NEW)
- `Sign(secret, timestamp, body)` returns `"sha256="` plus the hex HMAC-SHA256 of `<timestamp>.<body>`. Including the timestamp lets receivers reject replays (section 14).
- `Deliver(ctx, client, url, secret, body) error`: POST with `Content-Type`, `X-Forge-Signature`, `X-Forge-Task-Id`, `X-Forge-Timestamp`. Success is any 2xx. Redirects are not followed, timeout 10s, private and loopback addresses blocked at dial time.
- `RunDeliverer(ctx, q, cfg)`: claims a webhook entry, builds the body from the job hash, calls `Deliver`. On failure re-queues through `forge:queue:webhooks:delayed` with backoff (10s, 1m, 5m, 30m, 2h; six attempts in total), tracking `webhookAttempts`, and sets `webhookStatus` to `delivered` or `failed`. Runs on its own goroutines so slow customer endpoints never block image workers.

### `internal/health/server.go` (NEW)
- `Serve(ctx, addr, pool, queue)` exposes `/healthz` (always 200) and `/readyz` (all workers free or busy, and Redis ping OK) for Kubernetes probes.

### `internal/janitor/janitor.go` (NEW)
- `Run(ctx, cfg, q)` runs every 10 minutes.
- Deletes files in `SHARED_DIR/input` and `SHARED_DIR/output` older than `JobTTL`.
- Deletes input files older than 1 hour that have no job hash (left behind when the API crashed between saving the file and enqueueing).
- Finished job hashes expire on their own through `EXPIRE`, so the janitor only handles files.

### `cmd/orchestrator/main.go` (NEW)
1. Load config, set up the `slog` JSON logger.
2. Create the socket directory.
3. Connect Redis, call `Recover`.
4. Build the pool; start the `PromoteDelayed` ticker, the webhook deliverer, the health server, the janitor.
5. Create a context with `signal.NotifyContext` for SIGINT and SIGTERM.
6. Wait for `ctx.Done`, run `pool.Shutdown` with the grace period, exit.

---

## 5. Python worker (all under `Worker/`)

### `main.py` (MOVE from `worker/main.py`, restructure into functions)
- `parse_args()` reads the socket path.
- `connect_with_retry(path, attempts, delay)` unchanged apart from the folder move.
- `process_task(raw: bytes) -> dict` handles parsing, handler lookup, running, building the response. Never raises.
- `run_loop(sock)` is the read, process, write cycle. A clean EOF ends it with exit code 0.
- `main()` ties them together.
- Signal handling uses a `busy` flag. SIGTERM while idle exits immediately. SIGTERM while busy sets a flag so the loop exits after the current task. Python retries an interrupted `recv` after a handler returns (PEP 475), so a flag alone would not wake an idle worker.

### `ipc.py` (CHANGE)
- `read_frame` raises `ConnectionError` on EOF instead of returning `b""` (ambiguous with an empty frame).
- Reject lengths above 16 MiB.
- `_recv_exact` uses a `bytearray` with `recv_into` instead of repeated bytes concatenation.

### `response_generator.py` (CHANGE)
- Replace the single function with `success(taskId, result)` and `failure(taskId, message)`, returning exactly the contract shape.

### `global_logger.py` (CHANGE)
- The fixes in bug 3, and flush stderr as well.

### `handlers/__init__.py` (CHANGE)
- The skip-list names `"registry"` but the file is `register.py`. Correct the name.

### `handlers/register.py` (CHANGE)
- Rename `HANDERS` to `HANDLERS`.
- Handler contract: `handler(payload: dict, task_id: str) -> dict` returns the result on success and raises an exception on failure. `main.py` converts both into the contract response.

### `handlers/image_resize.py` (CHANGE)
- Validate `imagePath`, `width`, `height`. Width and height must be integers between 1 and 8192.
- Resolve the real path and require it to sit under `SHARED_DIR`, so a task cannot read arbitrary files.
- Output goes to `SHARED_DIR/output/<taskId>.<original extension>`.
- Convert RGBA to RGB when saving JPEG.
- Return `{outputPath, width, height, format, bytes}`.
- Full handling details (EXIF orientation, resampling, atomic write, bomb limits) are in section 16.

### `requirements.txt` (NEW)
- `Pillow`

### `tests/test_ipc.py` and `tests/test_image_resize.py` (NEW)

---

## 6. Node API (all under `api/`)

- `package.json` (NEW): `express`, `ioredis`, `multer`, `zod`, `pino`. Scripts: `start`, `dev`.
- `src/config.js` (NEW): `PORT`, `REDIS_URL`, `SHARED_DIR`, `MAX_UPLOAD_MB`, optional `API_KEY`.
- `src/keys.js` (NEW): the same key names as Go.
- `src/redis.js` (NEW): the client, plus `enqueueJob(job)`, `getJob(id)`, `ping()`.
  - `enqueueJob` uses one `MULTI`: `HSET` the job hash, then `LPUSH` the id to pending.
- `src/validate.js` (NEW): zod schema for the create body: `type` must be `image.resize`, `width` and `height` integers, optional `webhookUrl` that must be https and must not resolve to a private address.
- `src/upload.js` (NEW): multer disk storage writing to `SHARED_DIR/input/<taskId>.<ext>`, filtered to jpeg, png, webp, with a size limit.
- `src/routes/jobs.js` (NEW; request and response shapes in section 13):
  - `POST /jobs` takes multipart (file plus fields), generates the `taskId`, enqueues, returns 202 with `taskId`, `status`, and a status URL.
  - `GET /jobs/:id` returns the job state.
  - `GET /jobs/:id/result` streams the output file when status is `done`.
- `src/routes/health.js` (NEW): `/healthz` and `/readyz` (Redis ping).
- `src/middleware/errors.js` (NEW): 404 handler and central error handler.
- `src/middleware/auth.js` (NEW, optional): API key check.
- `src/server.js` (NEW): builds the app, mounts routes, listens, and on SIGTERM calls `server.close` then `redis.quit`.

---

## 7. Shared, Docker, tests, README

- `shared/input/.gitkeep` and `shared/output/.gitkeep` (NEW). The `.gitignore` already refers to them and they do not exist.
- `Orchestrator/Dockerfile` (NEW): stage one builds the Go binary. Stage two is `python:3.12-slim` with Pillow, the binary, and `Worker/` copied in, since the orchestrator spawns Python.
- `api/Dockerfile` (NEW): node 22 slim.
- `docker-compose.yml` (NEW): redis 7 with appendonly on, orchestrator, api, and one named volume mounted at `/shared` in both. Includes healthchecks, `init: true` on the orchestrator (reaps orphaned child processes), and non-root users with write access to the volume.
- `k8s/` (NEW, optional, last): Deployments for api and orchestrator (replicas 1 for the orchestrator), Redis, and shared storage. Sharing a disk between pods needs ReadWriteMany storage. A real deployment would swap the shared folder for object storage.
- `README.md` (CHANGE): real folder structure, the IPC contract, the Redis schema, how to run.
- Orchestrator tests (NEW):
  - `internal/ipc/frame_test.go` using `net.Pipe`
  - `internal/worker/worker_test.go` using a fake Python worker in `testdata/`
  - `internal/queue/queue_test.go` using miniredis
- `scripts/e2e.sh` (NEW): compose up, POST an image, poll until done, check the output file, check the webhook arrived.

---

## 8. Configuration reference

| Variable | Used by | Default | Meaning |
|---|---|---|---|
| `REDIS_ADDR` | Go | `localhost:6379` | Redis host:port |
| `REDIS_PASSWORD` | Go | empty | Redis password |
| `REDIS_URL` | Node | `redis://localhost:6379` | Redis connection URL |
| `WORKER_COUNT` | Go | number of CPUs | Python worker processes. Resizing is CPU-bound, so more than the CPU count adds no throughput |
| `PYTHON_BIN` | Go | `python3` | Interpreter used to start workers |
| `WORKER_SCRIPT` | Go | `../Worker/main.py` (Docker: `/app/Worker/main.py`) | Worker entry script |
| `SOCKET_DIR` | Go | `/tmp/forgequeue` | Folder for per-worker socket files. The full socket path must stay under 104 bytes (macOS) or 108 bytes (Linux) |
| `SHARED_DIR` | Go, Python, Node | `./shared` (Docker: `/shared`) | Root holding `input/` and `output/`. Go passes it to workers through the child environment |
| `TASK_TIMEOUT` | Go | `60s` | Maximum time for one task before the worker is killed |
| `START_TIMEOUT` | Go | `10s` | Maximum time from spawn to HELO |
| `MAX_ATTEMPTS` | Node (writes), Go (fallback) | `3` | Attempts per job before it is dead. The API writes it into each job hash; Go uses it only when the field is missing |
| `SHUTDOWN_GRACE` | Go | `30s` | Time allowed for in-flight tasks after SIGTERM |
| `JOB_TTL` | Go | `168h` | How long finished job hashes, input files, and output files are kept |
| `WEBHOOK_SECRET` | Go | none | HMAC key for webhook signatures. Required when any job has a webhook |
| `WEBHOOK_TIMEOUT` | Go | `10s` | Per-delivery HTTP timeout |
| `HEALTH_ADDR` | Go | `:9090` | Health and metrics listener |
| `LOG_LEVEL` | Go, Python | `info` | `debug`, `info`, `warn`, `error` |
| `PORT` | Node | `3000` | API port |
| `MAX_UPLOAD_MB` | Node | `10` | Upload size limit |
| `API_KEY` | Node | none | When set, requests need the header `X-API-Key` |

`config.Load` fails fast, with a message naming the variable, on any invalid value.

---

## 9. Lifecycles and flows

### 9.1 Happy path, step by step

1. Client sends `POST /jobs` with an image.
2. API generates `taskId` (UUID v4), saves the upload as `SHARED_DIR/input/<taskId>.<ext>`, validates the fields.
3. API runs one `MULTI`: `HSET forge:job:<taskId> ...`, then `LPUSH forge:queue:pending <taskId>`. It responds 202.
4. An idle worker's run-loop in Go is blocked in `BLMOVE` on pending. It receives the id, which now sits in processing.
5. Go reads the hash, sets `running`, increments `attempts`, and builds the task frame with `payload.imagePath` pointing at the input file.
6. Python reads the frame, runs `image.resize`, writes `SHARED_DIR/output/<taskId>.<ext>`, and replies with a `done` frame.
7. Go runs one `MULTI`: set `done`, result, `finishedAt`, expiry, and remove the id from processing.
8. If `webhookUrl` is set, Go pushes the id to the webhooks list and the deliverer POSTs the signed payload.
9. Client polls `GET /jobs/:id`, then downloads `GET /jobs/:id/result`.

### 9.2 Job state machine

```
queued   --Claim + MarkRunning-->                               running
running  --worker replied "done"-->                             done
running  --worker replied "failed"-->                           failed   (no retry)
running  --crash / timeout / bad response, attempts < max-->    queued   (via delayed, then pending)
running  --crash / timeout / bad response, attempts >= max-->   dead
running  --orchestrator restart (Recover), attempts < max-->    queued
running  --orchestrator restart (Recover), attempts >= max-->   dead
```

`done`, `failed`, `dead` are terminal. Their job hashes expire after `JOB_TTL`.

### 9.3 Worker state machine

```
starting --HELO received-->                      free
free     --task assigned-->                      busy
busy     --result received-->                    free
busy     --timeout / EOF / bad response-->       dead
free     --process exited while idle-->          dead
dead     --restart with backoff-->               starting
any      --Stop-->                               removed
```

### 9.4 Where a job id lives in Redis

```
pending    --BLMOVE-->                 processing
processing --LREM on done / failed-->  (hash only)
processing --Retry-->                  delayed
delayed    --PromoteDelayed-->         pending
processing --Dead-->                   dead
```

Invariant: a taskId is in exactly one of pending, processing, delayed, dead, or in none of them (terminal `done` / `failed`). Moves between structures happen inside a single command, a `MULTI`, or a Lua script, never as two separate commands.

---

## 10. Failure matrix

| Failure | Detected by | Action | Job ends as |
|---|---|---|---|
| Python exits before HELO (import error, bad path) | Waiter goroutine sees the exit, or `AcceptWorker` / `WaitForHelo` times out | `Start` returns an error that includes the tail of the worker's stderr. The pool retries with backoff. After 5 consecutive start failures `/readyz` reports not ready | Unaffected (no job claimed yet) |
| Python exits while idle | Waiter goroutine | Mark `dead`, restart with backoff | Unaffected |
| Python exits mid-task | `ReadFrame` returns EOF, `ErrWorkerDied` | Restart worker, `Retry` job | `queued` again, or `dead` after max attempts |
| Task exceeds `TASK_TIMEOUT` | Read deadline, `ErrTimeout` | `KillGroup` (not TERM, since Pillow may be stuck in C code), restart worker, `Retry` | Same as above |
| Worker returns invalid JSON, wrong `taskId`, or an oversize frame | Unmarshal, id check, or `MaxFrameSize`, `ErrBadResponse` | Restart worker (stream state is unknown), `Retry` | Same as above |
| Handler rejects input (not an image, bad dimensions, path outside `SHARED_DIR`) | Worker replies `failed` | `Fail`. Worker stays free | `failed`, no retry |
| Disk full or write error on output | Handler raises `OSError`, worker replies `failed` | `Fail` | `failed` |
| Redis unreachable while claiming | `Claim` returns an error | Worker stays free, loop sleeps with backoff 1s to 30s, `/readyz` not ready | Unaffected |
| Redis unreachable while completing | `Complete` returns an error | Retry the write 5 times with short backoff. If it still fails, leave the id in processing and log loudly | Re-run after the next `Recover` (at-least-once). Output name is `<taskId>.<ext>`, so a re-run overwrites the same file |
| Orchestrator killed (SIGKILL, OOM) | Restart, then `Recover` | Workers died with it (`Pdeathsig`). Ids in processing go back to pending, or to dead if attempts are exhausted | `queued` or `dead` |
| API crashes after saving the file, before enqueue | Janitor | Input file with no job hash, older than 1 hour, is deleted | No job exists |
| Webhook endpoint down, slow, or answers non-2xx | `Deliver` returns an error | Retry schedule (section 14). After the last attempt `webhookStatus` is `failed` | Job status unchanged |
| Socket path too long | `config.Load` | Exit at startup with a message naming `SOCKET_DIR` | Not applicable |

---

## 11. Concurrency and ownership rules (Go)

- **One owner per worker.** Each `types.Worker` is owned by exactly one run-loop goroutine. Only that goroutine calls `RunTask` or `Restart` or touches `Conn`. Other goroutines (health server, `Snapshot`) read status only through `GetStatus()` under the read lock.
- **No I/O under the lock.** `RunTask` takes the mutex to set `busy` and `Job`, releases it, does the socket I/O, then takes it again to record the outcome.
- **Exit channel.** The waiter goroutine owns `cmd.Wait()` and sends the exit error on a buffered channel (capacity 1). The owner selects on that channel and on the task read, so an idle crash is noticed without polling.
- **Log pipes.** Each pipe has one reader goroutine that ends on EOF. A `sync.WaitGroup` tracks both. The waiter calls `cmd.Wait()` only after the group is done.
- **Deadlines versus context.** `Claim` uses the shutdown context, so it stops immediately. `RunTask` uses its own deadline rather than the context, so SIGTERM lets a running task finish. After the grace period the pool calls `KillGroup` on whatever is still running.
- **Single cleanup.** `Stop` is guarded by `sync.Once` so the process kill and socket removal run exactly once.
- **Shutdown order.** Cancel the claim context, wait for run-loops (until grace), stop workers, stop the webhook deliverer, stop the janitor and health server, close Redis.
- **Race detector.** `go test -race ./...` is part of every phase's done criteria.

---

## 12. Redis command reference

| Function | Redis commands | Notes |
|---|---|---|
| `Claim` | `BLMOVE forge:queue:pending forge:queue:processing RIGHT LEFT <wait>`, then `HGETALL forge:job:<id>` | Producers use `LPUSH`, so `RIGHT` takes the oldest id (FIFO). Use a 5 second wait in a loop so the shutdown context is checked. If the hash is missing, `LREM` the id and log it |
| `MarkRunning` | `HSET` status=running startedAt=now, `HINCRBY` attempts 1 | One pipeline |
| `Complete` | `MULTI`: `HSET` status=done result=json finishedAt=now; `EXPIRE` job `JOB_TTL`; `LREM processing 1 <id>`; `EXEC` | Atomic |
| `Fail` | Same as `Complete` with status=failed and error=message | Atomic |
| `Retry` | `MULTI`: `HSET` status=queued error=message; `LREM processing 1 <id>`; `ZADD delayed <now+delay as unix seconds> <id>`; `EXEC` | If attempts >= maxAttempts, call `Dead` instead |
| `Dead` | `MULTI`: `HSET` status=dead error finishedAt; `EXPIRE`; `LREM processing 1 <id>`; `LPUSH dead <id>`; `EXEC` | Atomic |
| `PromoteDelayed` | Lua script: `ZRANGEBYSCORE delayed -inf <now> LIMIT 0 100`, then for each id `ZREM delayed id` and `LPUSH pending id` | One call per second, atomic |
| `Recover` | `LRANGE processing 0 -1`. For each id read attempts and maxAttempts. Exhausted: `Dead`. Otherwise `LREM processing 1 id`, `LPUSH pending id`, `HSET` status=queued | Startup only |
| `EnqueueWebhook` | `HSET` webhookStatus=pending webhookAttempts=0, then `LPUSH webhooks id` | |
| `ClaimWebhook` | `BLMOVE webhooks webhooks:processing RIGHT LEFT <wait>` | |
| `AckWebhook` | `LREM webhooks:processing 1 id`, `HSET` webhookStatus=delivered | |
| `ScheduleWebhookRetry` | `ZADD webhooks:delayed <time> id`, `LREM webhooks:processing 1 id`, `HINCRBY` webhookAttempts 1 | A second promote script moves due ids back to `webhooks` |
| API `enqueueJob` | `MULTI`: `HSET` job fields; `LPUSH pending id`; `EXEC` | Hash first, id second |

- `BLMOVE` needs Redis 6.2 or newer. In go-redis v9 it is `rdb.BLMove(ctx, src, dst, "RIGHT", "LEFT", timeout)`.
- Run Redis with `appendonly yes` and `appendfsync everysec`. A hard crash can lose about one second of writes. At-least-once processing and `Recover` cover that window.
- `EXPIRE` is set only on terminal states, never on `queued` or `running` hashes.

---

## 13. API reference

### POST /jobs

Request: `multipart/form-data`.

| Field | Required | Notes |
|---|---|---|
| `file` | yes | jpeg, png, or webp, up to `MAX_UPLOAD_MB` |
| `type` | yes | `image.resize` |
| `width` | yes | integer 1 to 8192 |
| `height` | yes | integer 1 to 8192 |
| `webhookUrl` | no | https URL, must not resolve to a private address |

```
curl -X POST http://localhost:3000/jobs \
  -H "X-API-Key: $API_KEY" \
  -F file=@photo.jpg -F type=image.resize -F width=800 -F height=600 \
  -F webhookUrl=https://example.com/hooks/forge
```

Response `202`:

```json
{
  "taskId": "7c9e6679-7425-40de-944b-e07fc1f90ae7",
  "status": "queued",
  "statusPath": "/jobs/7c9e6679-7425-40de-944b-e07fc1f90ae7"
}
```

The client never supplies `imagePath`. The API builds the payload itself from the saved upload.

### GET /jobs/:id

Response `200`:

```json
{
  "taskId": "7c9e6679-7425-40de-944b-e07fc1f90ae7",
  "type": "image.resize",
  "status": "done",
  "attempts": 1,
  "createdAt": "2026-10-12T08:22:05Z",
  "startedAt": "2026-10-12T08:22:06Z",
  "finishedAt": "2026-10-12T08:22:07Z",
  "result": { "width": 800, "height": 600, "format": "jpeg", "bytes": 48213 },
  "resultPath": "/jobs/7c9e6679-7425-40de-944b-e07fc1f90ae7/result",
  "error": null,
  "webhookStatus": "delivered"
}
```

The API strips the server-side `outputPath` from `result`. It reads that value internally to stream the file.

### GET /jobs/:id/result

- `200`: the image, with `Content-Type` taken from the extension.
- `404`: unknown id.
- `409`: job exists but is not `done` yet (body includes the current `status`).
- `410`: job is `done` but the output file has expired.

### GET /healthz and GET /readyz

`/healthz` always returns 200. `/readyz` returns 200 only when the Redis ping succeeds.

### Error shape and codes

```json
{ "error": { "code": "VALIDATION_ERROR", "message": "width must be an integer between 1 and 8192", "details": [] } }
```

| HTTP | `code` |
|---|---|
| 400 | `VALIDATION_ERROR` |
| 401 | `UNAUTHORIZED` |
| 404 | `NOT_FOUND` |
| 409 | `JOB_NOT_DONE` |
| 410 | `RESULT_EXPIRED` |
| 413 | `PAYLOAD_TOO_LARGE` |
| 415 | `UNSUPPORTED_MEDIA_TYPE` |
| 429 | `RATE_LIMITED` (only if rate limiting is added) |
| 503 | `SERVICE_UNAVAILABLE` (Redis down) |

Upload handling order matters. Multer cannot see form fields that arrive after the file, so the route validates fields after the upload finishes and deletes the saved file if validation fails.

---

## 14. Webhook reference

Event names: `job.done`, `job.failed`, `job.dead`.

Body:

```json
{
  "event": "job.done",
  "taskId": "7c9e6679-7425-40de-944b-e07fc1f90ae7",
  "status": "done",
  "attempts": 1,
  "createdAt": "2026-10-12T08:22:05Z",
  "finishedAt": "2026-10-12T08:22:07Z",
  "result": { "width": 800, "height": 600, "format": "jpeg", "bytes": 48213 },
  "resultPath": "/jobs/7c9e6679-7425-40de-944b-e07fc1f90ae7/result",
  "error": null
}
```

Headers: `Content-Type: application/json`, `X-Forge-Task-Id`, `X-Forge-Event`, `X-Forge-Timestamp` (unix seconds), `X-Forge-Signature` (`sha256=<hex>`).

Signature: HMAC-SHA256 with `WEBHOOK_SECRET` over the string `<timestamp>.<raw body>`.

Receiver verification:

1. Read `X-Forge-Timestamp`. Reject if it differs from the current time by more than 300 seconds.
2. Compute the HMAC over `<timestamp>.<raw request bytes>`, before any JSON parsing.
3. Compare with `X-Forge-Signature` using a constant-time comparison.
4. Answer 2xx quickly and process the event asynchronously.
5. Deduplicate on `taskId` plus `event`.

Delivery semantics:

- At-least-once. The same event can arrive twice.
- Success is any 2xx. Everything else, including timeouts and connection errors, is a failure.
- Schedule: immediate attempt, then +10s, +1m, +5m, +30m, +2h. Six attempts in total.
- Redirects are not followed.
- A webhook failure never changes the job's own status. `webhookStatus` tracks it separately.
- v1 uses one global secret. A per-job secret is listed under extensions.

---

## 15. Security and hardening checklist

- **Path traversal.** The API never accepts a path from the client. Python resolves `imagePath` with `os.path.realpath` and requires it to sit under `SHARED_DIR/input`. Output paths are built by the worker, never taken from the payload.
- **SSRF on webhooks.** Validate the URL at the API (https only, no credentials in the URL). At delivery time, resolve DNS and block loopback, private, link-local (including `169.254.169.254`), and unspecified addresses. Enforce this in a `net.Dialer.Control` hook so a DNS rebind between check and connect is also caught. Do not follow redirects.
- **Uploads.** Enforce the size limit. Check magic bytes (for example with the `file-type` package), not only the declared mimetype. Cap dimensions in the validator.
- **Decompression bombs.** Set `Image.MAX_IMAGE_PIXELS = 50_000_000` in Python and treat `DecompressionBombError` as a `failed` result.
- **Socket permissions.** Socket directory mode `0700`, socket file mode `0600`.
- **Child environment.** The orchestrator builds the worker's environment explicitly. Workers never receive the Redis password or the webhook secret.
- **Containers.** Run as a non-root user. Make sure both the API and orchestrator containers can write to the shared volume.
- **Resource limits.** Set container memory limits. Optionally call `resource.setrlimit(RLIMIT_AS, ...)` in the worker so one huge image cannot take the machine down.
- **Log hygiene.** Log the host of a webhook URL, not the full URL (it may carry a token). Never log the API key or the secret.
- **API key.** Compare with a constant-time function.

---

## 16. Image handling details (Python)

- Open with `Image.open`, then apply `ImageOps.exif_transpose(img)` so phone photos keep their orientation.
- Resampling filter: `Image.Resampling.LANCZOS`.
- Output format follows the input extension: `.jpg` / `.jpeg` to JPEG, `.png` to PNG, `.webp` to WebP. Always pass `format=` explicitly to `save`.
- JPEG cannot store alpha. Convert RGBA, LA, and palette images to RGB before saving as JPEG.
- Atomic write: save to `<final path>.tmp`, then `os.replace` to the final name. `GET /jobs/:id/result` never serves a half-written file.
- Output name is `<taskId>.<ext>`. A retried or re-run job overwrites the same file, so duplicates are harmless.
- Resize semantics: the current code stretches the image to exactly `width` x `height`. Decision for v1: keep stretch as the default. An optional payload field `fit` can add `contain` (fit inside, keep aspect ratio) and `cover` (fill and center-crop) using `ImageOps.contain` and `ImageOps.fit`.
- Error messages sent back contain no filesystem paths. Map `FileNotFoundError`, `UnidentifiedImageError`, `ValueError`, and `OSError` to short messages such as "input file not found", "unsupported or corrupt image", "invalid dimensions", "could not write output".
- Result object: `{outputPath, width, height, format, bytes}`.

---

## 17. Observability

**Go log fields** (`slog` JSON): `time`, `level`, `msg`, `component` (`pool`, `worker`, `queue`, `webhook`, `janitor`), `workerId`, `taskId`, `attempt`, `durationMs`, `err`.

**Python log fields** (existing, with the typo fixed): `timestamp`, `taskId`, `internal`, `critical`, `module`, `message`.

**Correlation.** `taskId` appears in every log line across the API, the orchestrator, and the workers, so one job can be followed with a single search.

**Metrics** (optional, Prometheus text format on `HEALTH_ADDR` at `/metrics`):

| Metric | Type | Labels |
|---|---|---|
| `forge_jobs_total` | counter | `status` |
| `forge_job_duration_seconds` | histogram | `type` |
| `forge_queue_depth` | gauge | `queue` (from `LLEN` and `ZCARD`) |
| `forge_worker_state` | gauge | `state` |
| `forge_worker_restarts_total` | counter | `reason` (`crash`, `timeout`, `bad_response`) |
| `forge_webhook_deliveries_total` | counter | `result` |

---

## 18. Test plan

**Go, `internal/ipc`**
- Round trip of a normal payload.
- Zero-length payload.
- Header above `MaxFrameSize` is rejected without allocating.
- Truncated header and truncated payload both return an error.
- Concurrent writers do not interleave frames (single `Write`).

**Go, `internal/worker`** (uses `testdata/fake_worker.py`, which takes a mode from an environment variable: `echo`, `sleep`, `crash`, `garbage`, `wrong_id`, `exit_early`)
- `Start` succeeds and reaches `free`.
- A script that exits immediately makes `Start` return within `START_TIMEOUT`, not hang.
- `RunTask` success round trip.
- Timeout kills the whole process group. Test with a worker that spawns a grandchild `sleep` and assert the grandchild is gone.
- Crash mid-task returns `ErrWorkerDied`, and a following `Restart` plus `RunTask` works.
- `garbage` and `wrong_id` return `ErrBadResponse`.
- `Stop` leaves no zombie process and no socket file.
- Worker stdout and stderr lines reach the logger.

**Go, `internal/queue`** (a real Redis through testcontainers or compose is the safest choice. `miniredis` only works if its version covers `BLMOVE` and Lua scripting)
- FIFO order across several claims.
- `Claim` timeout returns `nil, nil`.
- `Complete` removes the id from processing and sets the expiry.
- `Retry` places the id in delayed, and `PromoteDelayed` moves it after the delay.
- Exhausted attempts go to dead.
- `Recover` requeues, and sends exhausted jobs to dead.
- Orphan id (no hash) is dropped from processing.

**Python**
- `read_frame` with data arriving in small chunks.
- EOF raises `ConnectionError`.
- Frame above the cap is rejected.
- `process_task` with an unknown type returns `failed`.
- Malformed JSON returns `failed` with `taskId` null.
- Resize keeps PNG as PNG and leaves the original untouched.
- A path outside `SHARED_DIR` is rejected.
- Invalid dimensions are rejected.
- RGBA to JPEG works.

**Node API** (supertest)
- Validation failures return the documented codes.
- A valid upload creates the hash and the pending entry.
- Unknown id returns 404.
- Result before completion returns 409.
- A validation failure removes the uploaded file.

**Webhook**
- Signature test vector: fixed secret, timestamp, and body produce a known hex string.
- `httptest` server that answers 500 and then 200 produces the retry schedule.
- SSRF targets (`127.0.0.1`, `10.0.0.1`, `169.254.169.254`) are refused.

**End to end** (`scripts/e2e.sh` plus manual chaos checks)
- Happy path.
- `kill -9` a worker mid-task: the job still completes with `attempts` 2.
- `kill -9` the orchestrator mid-task, restart it: the job completes, and no Python processes were left behind.
- Webhook receiver down, then up: delivery succeeds on a retry.
- Restart Redis during load: the orchestrator recovers without losing jobs.

---

## 19. Pitfalls checklist

- `cmd.Wait()` must run only after every read from `StdoutPipe` and `StderrPipe` has finished. `Wait` closes the pipes.
- After `ErrTimeout`, the socket stream is out of sync, because a late reply could still arrive. Always restart the worker. Never reuse the connection.
- `syscall.Kill(-pid, sig)` signals the whole group only because `Setpgid: true` makes the child's pgid equal its pid.
- Unix socket paths have a hard length limit (104 bytes on macOS, 108 on Linux). Long Docker or Kubernetes mount paths can exceed it.
- Python block-buffers stdout when it is a pipe. Set `PYTHONUNBUFFERED=1` in the child environment so third-party prints and the logger both appear immediately.
- Always clear a connection deadline after a successful read. A leftover deadline fails the next call.
- `HGETALL` returns strings only. Parse integers and timestamps explicitly, and keep `payload` and `result` as `json.RawMessage`.
- All timestamps are UTC ISO-8601 with a trailing `Z`, in all three languages.
- JSON numbers from Node must be integers for Python (`800`, not `800.0`).
- `BLMOVE` with timeout `0` blocks forever. Use a finite wait in a loop.
- Go as PID 1 in a container does not reap orphaned grandchildren. Use `init: true` in compose, or `tini`.
- Kubernetes `terminationGracePeriodSeconds` must exceed `SHUTDOWN_GRACE` plus a margin, or the kubelet kills the pod mid-shutdown.
- Keep the orchestrator at one replica until `Recover` uses per-instance processing lists. Otherwise one instance's restart steals another instance's running jobs.
- Two folders whose names differ only by case (`worker/` and `Worker/`) break checkouts on macOS and Windows.

---

## 20. Optional extensions and design decisions

**Decisions made in this map** (each can be reversed later):

- Job ids are generated by the API, not by the client.
- Handler failures are never retried. Worker-level failures are retried up to 3 attempts.
- Single orchestrator instance in v1.
- Shared volume for files in v1, object storage later.
- Resize stretches by default, with an optional `fit` field.
- One global webhook secret.
- Delivery is at-least-once for both jobs and webhooks.

**Extensions after v1:**

- More handlers: `image.thumbnail`, `image.convert`, `image.grayscale`, `image.watermark`.
- Priorities: separate pending lists (`high`, `normal`, `low`). `BLMOVE` takes one source, so `Claim` would try each in order with a short wait.
- Multiple orchestrators: per-instance processing lists with a lease and heartbeat.
- `Idempotency-Key` header on `POST /jobs`.
- Rate limiting per API key.
- Object storage (S3 or MinIO) instead of the shared volume.
- `DELETE /jobs/:id`: remove a queued job, or kill the worker group for a running one.
- Heartbeat frames (`PING` / `PONG`) so hung workers are detected sooner than `TASK_TIMEOUT`.
- Per-job webhook secrets.
- Prometheus and Grafana dashboards on top of section 17.

---

## 21. Target folder layout

```
ForgeQueue/
├── README.md
├── .gitignore
├── docker-compose.yml
├── Orchestrator/
│   ├── Dockerfile
│   ├── go.mod
│   ├── go.sum
│   ├── cmd/orchestrator/main.go
│   └── internal/
│       ├── config/config.go
│       ├── health/server.go
│       ├── janitor/janitor.go
│       ├── ipc/
│       │   ├── conn.go
│       │   ├── frame.go
│       │   └── frame_test.go
│       ├── pool/pool.go
│       ├── queue/
│       │   ├── keys.go
│       │   ├── queue.go
│       │   └── queue_test.go
│       ├── types/
│       │   ├── ipc.go
│       │   ├── job.go
│       │   └── worker.go
│       ├── webhook/
│       │   ├── sign.go
│       │   └── deliver.go
│       └── worker/
│           ├── spawn.go
│           ├── worker.go
│           ├── logpipe.go
│           ├── worker_test.go
│           └── testdata/fake_worker.py
├── Worker/
│   ├── main.py
│   ├── ipc.py
│   ├── response_generator.py
│   ├── global_logger.py
│   ├── requirements.txt
│   ├── handlers/
│   │   ├── __init__.py
│   │   ├── register.py
│   │   └── image_resize.py
│   └── tests/
│       ├── test_ipc.py
│       └── test_image_resize.py
├── api/
│   ├── Dockerfile
│   ├── package.json
│   └── src/
│       ├── server.js
│       ├── config.js
│       ├── keys.js
│       ├── redis.js
│       ├── validate.js
│       ├── upload.js
│       ├── routes/
│       │   ├── jobs.js
│       │   └── health.js
│       └── middleware/
│           ├── errors.js
│           └── auth.js
├── shared/
│   ├── input/.gitkeep
│   └── output/.gitkeep
├── k8s/
└── scripts/e2e.sh
```

---

## 22. Build order (hardest first)

| Phase | Work | Done when |
|---|---|---|
| 1. Worker lifecycle | Bugs 1, 6, 7; types changes; `spawn.go`; `logpipe.go`; `worker.go` | A Go test starts a real Python worker, runs a task, kills it mid-task, sees `ErrWorkerDied`, restarts, and runs another task |
| 2. Python side | Bugs 2 to 5 and 8; restructure `main.py`; finish the handler contract | The Phase 1 test passes with the real `image.resize` handler and a real image |
| 3. Redis queue | `keys.go`, `queue.go` | Tests show claim, complete, retry, dead, and recover each leave the right state in Redis |
| 4. Wiring | `config.go`, `pool.go`, `cmd/orchestrator/main.go` | A hand-made job (`HSET` + `LPUSH` from `redis-cli`) gets processed and the output file appears |
| 5. Node API | Everything in section 6 | `POST /jobs` then `GET /jobs/:id` shows queued, running, done without touching `redis-cli` |
| 6. Webhooks | `webhook/` package and the deliverer loop | A local receiver gets a signed POST, and a failing receiver gets retries |
| 7. Shutdown and health | Graceful shutdown in Go and Node, health server | SIGTERM during a running task lets it finish, then exits cleanly |
| 8. Packaging | Dockerfiles, compose, `e2e.sh`, README, `k8s/` if time remains | `docker compose up` followed by `e2e.sh` passes |

Note: frame size, status words, and Redis field names appear in all three languages. A change to any of them has to be made in all three places.

---

## 23. Definition of done

- `docker compose up` from a clean clone starts Redis, the orchestrator, and the API. `/readyz` returns 200 on both services.
- Posting an image returns 202. Polling shows `done`. `GET /jobs/:id/result` returns an image with exactly the requested dimensions.
- Bad input (not an image, oversized dimensions, a path outside `SHARED_DIR`) ends as `failed` with a clear message, and the worker is still alive afterwards.
- `kill -9` on a worker mid-task: the job still completes, with `attempts` equal to 2.
- A task that exceeds `TASK_TIMEOUT`: the worker and its child processes are killed, the job is retried, and it ends as `dead` after 3 attempts.
- `kill -9` on the orchestrator mid-task, then restart: the job completes, and no orphan Python processes remain.
- SIGTERM while tasks are running: in-flight tasks finish and the process exits with code 0 within `SHUTDOWN_GRACE`.
- A webhook receiver gets a correctly signed POST. With the receiver down, retries show up in `webhookAttempts`, and the job's own status is unchanged.
- `go test -race ./...`, `pytest`, `npm test`, and `scripts/e2e.sh` all pass.
- The README documents how to run, the contracts in section 3, and the environment variables in section 8.
