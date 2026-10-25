# Architecture & Technical Approach: Wallet Transfer Service

## 1. Overview & Core Philosophy

The goal is to design a high-integrity financial wallet engine handling deposits and inter-wallet transfers. In monetary systems, eventual consistency is usually the wrong tradeoff. Our core requirements are:

* **Strong transactional integrity**: Transfer operations must be atomic (either debits and credits commit together or fail cleanly).
* **Double-entry bookkeeping**: Never mutate isolated balances without an immutable trail of debits and credits.
* **Zero double-spend / overdraft**: Concurrency cannot permit race conditions or negative balances.
* **Predictable idempotency**: Safe client retries that return the original response without executing the transaction twice.
---

## 2. Technology Choices & Justification

### Go

Standard Go concurrency primitives, interfaces, and testing packages are well suited for building resilient financial services. Standard dependencies will be kept lean (`net/http` or lightweight routing via `chi`).

### SQLite vs. PostgreSQL (Why SQLite here?)

While PostgreSQL is standard for high-throughput distributed systems with row-level pessimistic locking (`SELECT ... FOR UPDATE`), **SQLite was chosen intentionally for this assignment**:

1. **Frictionless Local & CI Evaluation**: Reviewers can clone the repo and immediately run `go test ./...` without booting Docker containers, managing network ports, or setting up external schemas.
2. **Pure-Go Driver (`modernc.org/sqlite`)**: Avoids CGo build dependencies, ensuring tests compile cleanly on any OS (macOS, Linux, Windows).
3. **Strict ACID Guarantees**: Under WAL mode, SQLite provides serializable transaction guarantees and ACID compliance.
4. **Handling SQLite's Lock Model**: SQLite operates with database/file locks rather than Postgres-style row locks. We embrace this intentionally rather than fighting it:
* Run in `PRAGMA journal_mode = WAL;` to enable concurrent readers alongside a writer.
* Configure `PRAGMA busy_timeout = 5000;` to prevent immediate `SQLITE_BUSY` contention during write spikes.
* Execute balance validation and mutations within `BEGIN IMMEDIATE` transactions, securing the reserved write lock immediately at the transaction boundary to eliminate lock escalation deadlocks.

---

## 3. Data Model & Double-Entry Ledger Design

### Tables

* **`accounts`**: `id` (UUID), `user_id`, `currency`, `balance` (stored in integer minor units/cents), `created_at`, with `CHECK (balance >= 0)` as database-enforced overdraft protection.
* **`transfers`**: `id` (UUID), `source_account_id`, `destination_account_id`, `amount` (`> 0`), `currency`, `status` (`PENDING`, `COMPLETED`, `FAILED`), `created_at`.
* **`ledger_entries`**: `id` (UUID), `transfer_id` (foreign key), `account_id` (foreign key), `amount` (`> 0`), `direction` (`DEBIT` or `CREDIT`), `created_at`.
* **`idempotency_records`**: `key` (PK), `request_hash` (SHA-256), `status` (`IN_PROGRESS`, `RESOLVED`), `response_code`, `response_body`, `created_at`.

### Design Rule

Balances are updated in lockstep with the corresponding immutable debits and credits inside the same atomic transaction. A background or periodic check can always verify that:

$$\sum (\text{Credits}) - \sum (\text{Debits}) == \text{Current Balance}$$

---

## 4. Concurrency & Deadlock Prevention

1. **Transaction Boundary & Reservation**: All mutating operations begin with `BEGIN IMMEDIATE`. This ensures only one write transaction proceeds at a time, completely eliminating race conditions around balance reads and writes.
2. **Deterministic Account Ordering**: Even though SQLite locks the file, our application layer sorts the involved account IDs lexicographically (`min(ID)` then `max(ID)`) prior to balance verification and updates. This ensures clean software habits and consistency if swapped for row-level locked databases later.
3. **Database Constraints as Fallback**: The database check constraint `CHECK (balance >= 0)` ensures that even if application logic were compromised, an overdraft can never physically write to disk.

---

## 5. Caching Strategies

In financial ledgers, blind read-caching balances can lead to dirty reads and double spending. We will apply caching strategically:

1. **Idempotency Response Caching (Persistent Read-Through)**:
    * The primary cache in this system lives in the `idempotency_records` table.
    * When an identical request is submitted with the same `Idempotency-Key` and matching payload hash, the service skips transaction execution and directly serves the cached HTTP status code and response payload.
    * Stored in the database so that server restarts do not clear the idempotency cache.
2. **Account Metadata / Read Cache (In-Memory with Write Invalidation)**:
    * Non-volatile account attributes (e.g. verifying currency matching, account existence, and user ownership) can be cached in an in-memory TTL/LRU structure (or `sync.Map`).
    * **Balance queries**: Balances are *never* cached during write-critical transfer evaluation; they are always evaluated directly inside the active write transaction. For read-only balance lookups (`GET /accounts/:id`), reads run concurrently against the WAL file without blocking writers.

---

## 6. Idempotency Flow

Retrying requests due to dropped network connections must never result in duplicate debits:

1. Client supplies an `Idempotency-Key` header.
2. Compute `hash = SHA256(request_body)`.
3. Check `idempotency_records`:
    * **Record found & status is `RESOLVED`**:
        * If hash matches: Return cached response immediately (`200/201`).
        * If hash differs: Reject with `422 Unprocessable Entity` (key reuse with differing payload).
    * **Record found & status is `IN_PROGRESS`**:
        * Reject with `409 Conflict` (request currently in flight).
    * **Record not found**:
        * Insert record with `IN_PROGRESS` status.
        * Execute transfer and double-entry postings in a single transaction.
        * Update record to `RESOLVED` with the HTTP status and body payload, then commit.

---

## 7. Implementation Plan

* **Milestone 1**: Domain structs, database schema, SQLite connection setup (WAL mode, timeouts).
* **Milestone 2**: Repository layer with transactional double-entry posting and idempotency persistence.
* **Milestone 3**: Service layer business logic (currency checks, balance validation).
* **Milestone 4**: HTTP routing, request validation, and middleware mapping.
* **Milestone 5**: Integration tests:
    * Concurrent race tests on a single account to prove balances never drop below zero.
    * Bidirectional concurrent transfers between two accounts to verify absence of deadlock.
    * Concurrent idempotency tests verifying that duplicate requests result in exactly one execution.