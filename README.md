# SIS — Password Strength Analyzer (Checkpoint 1)

A small synchronous, in-memory Go application. A **task** is a unit of work:
*analyze the strength of one password*.

| Part        | In this project                                                          |
|-------------|--------------------------------------------------------------------------|
| Input       | a password                                                               |
| Processing  | the password is checked against weighted rules (length, case, digits, symbols, repeats, common passwords) |
| Result      | a score (0–100), a level (weak / medium / strong / very strong) and the list of unmet requirements |

No external services, databases or third-party packages are used.

## Run and test

Requires Go 1.27.

```bash
go run ./cmd/sis      # demo: creates, processes and prints several tasks

gofmt -l .            # must print nothing
go vet ./...
go test ./...
```

## Task lifecycle

```
              Start()                Complete(result)
 Pending ───────────────▶ Processing ─────────────────▶ Completed
                              │
                              │ Fail(reason)
                              ▼
                           Failed
```

| From         | Allowed next state     | Meaning                                          |
|--------------|------------------------|--------------------------------------------------|
| `Pending`    | `Processing`           | task was created, processing starts              |
| `Processing` | `Completed`, `Failed`  | processing finished successfully / with an error |
| `Completed`  | — (terminal)           | the result is stored on the task                 |
| `Failed`     | — (terminal)           | the failure reason is stored on the task         |

Every other transition is rejected with an error wrapping
`task.ErrInvalidTransition` (details in `task.TransitionError`). For example,
processing a `Completed` or `Failed` task again is an invalid operation, and a
rejected operation never changes the task.

The `Processing` state is saved to the store *before* the processor runs, so
the stored status always matches what is really happening.

## Validation

Input is validated when a task is created; invalid input is rejected with an
error and nothing is stored (no ID is consumed either).

- empty password → `task.ErrEmptyPassword`
- longer than 128 characters → `task.ErrPasswordTooLong` (guards against huge payloads)

A password of spaces is *not* invalid: it is a legal (if terrible) password and
simply receives a low score.

## Scoring

The default rules add up to 100 points:

| Rule                                          | Points |
|-----------------------------------------------|--------|
| at least 8 characters                         | 20     |
| at least 12 characters                        | 15     |
| a lowercase letter                            | 10     |
| an uppercase letter                           | 10     |
| a digit                                       | 10     |
| a symbol or punctuation mark                  | 15     |
| no character repeated 3+ times in a row       | 10     |
| not a commonly used password (**blocking**)   | 10     |

Score = earned points / total points × 100. If a *blocking* rule fails (a
common password such as `password`), the score is capped at 20, so a popular
password can never look good just because it is long.

Levels: `< 40` weak, `< 75` medium, `< 95` strong, otherwise very strong.

## Project structure

```
cmd/sis/main.go            demo program
internal/task/             domain model: Task, Input, Result, Status, errors
internal/password/         processing logic: Rule interface, rules, Analyzer
internal/store/            in-memory storage (MemoryStore)
internal/service/          application logic: create / get / list / process
```

Dependencies point one way: `service` → `task`; `password` → `task`;
`store` → `task`. Nothing depends on `service` except `main`.

## Design decisions

**Structs and methods.** `Task`, `Input`, `Result`, `Analyzer`, `MemoryStore`
and `Service` are structs with behaviour attached.

**Value vs pointer receivers.**
- Methods that *change* state (`Task.Start`, `Complete`, `Fail`, the store, the
  service) use pointer receivers, so the change is visible to the caller.
- Read-only methods on small types (`Status.String`, `Input.Validate`,
  `Task.String`, `Task.Clone`) use value receivers.
- `Task` is stored and passed **by value**. Because it contains a `*Result`,
  a plain copy would share the result; `Task.Clone` makes a deep copy, and the
  store clones on `Add`, `Get`, `Update` and `List`. As a result a caller can
  never modify stored data except through `Update`.
- `Result` is a pointer on `Task` so that "not processed yet" (`nil`) is
  distinguishable from "processed with a zero score".

**Interfaces.**
- `password.Rule` — one requirement. New rules are added by implementing it;
  the `Analyzer` does not change (the tests include a custom rule).
- `service.Processor` — anything that turns an `Input` into a `Result`;
  `*password.Analyzer` satisfies it. Tests use a fake to simulate failures.
- `service.Store` — anything that stores tasks; `*store.MemoryStore` satisfies it.
- Interfaces are declared in the package that *uses* them, and kept small.

**Slices and maps.** `MemoryStore` keeps tasks in a `map[string]task.Task` for
O(1) lookup by ID and a `[]string` with insertion order, because map iteration
order is random and `List` should be stable. Allowed transitions are a
`map[Status][]Status`.

**Error handling.** Expected errors are returned, never `panic`ked. Sentinel
errors (`ErrNotFound`, `ErrEmptyPassword`, ...) are wrapped with context via
`fmt.Errorf("...: %w", err)` and checked with `errors.Is`. `TransitionError` is
a custom error type that carries the states and is read with `errors.As`.

**Passwords are not leaked.** `Input.String` returns `[redacted]`, so
printing a task never reveals the password.

**Ready to evolve.** IDs and storage sit behind small boundaries, so later
checkpoints can extend them without rewriting the domain model. Nothing from
later checkpoints (concurrency, HTTP) is implemented yet; `MemoryStore` and
`Service` are intentionally **not** safe for concurrent use.
