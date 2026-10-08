# Architecture

## Components

PanBridge consists of three layers:

1. **Providers** expose local files and remote drives through one small interface.
2. **Sync engine** compares two provider trees, writes a durable operation queue, and applies operations with retries.
3. **Clients** provide a CLI first and a desktop client later. They share the engine and state database.

The provider boundary must not contain conflict policy, scheduling, or user interface behavior.

The engine uses bounded worker pools, metadata-first comparisons, and a durable operation queue. A local watcher is an optimization; periodic scans remain the recovery path when an operating system watcher drops or coalesces events.

## Synchronization rules

- A provider operation is idempotent or carries enough identity information for the engine to make retries safe.
- The current metadata index can be rebuilt by scanning. The last successful sync baseline, deletion history, and operation journal cannot be reconstructed from the two current trees. Missing history requires an explicit rebaseline flow with automatic deletion disabled.
- A change found on both sides is a conflict. The default policy creates a conflict copy rather than silently overwriting either file.
- A delete is inferred only against a known successful baseline and a complete, successful scan. Missing roots, authentication failures, and partial listings must never be interpreted as deletions. Keep tombstones and recovery copies; a concurrent modification becomes a conflict. Apply a deletion threshold before a large batch can proceed.
- File identity uses the provider object ID when available. Size and modification time are scan shortcuts; hashes or provider checksums verify content before destructive actions.
- Resource limits are explicit: transfer workers, hash workers, buffers, queue size, polling interval, and retry backoff all have bounded defaults.
- Update each path's baseline only after the relevant operations and verification succeed. A partial run must not advance the baseline for unprocessed paths.

## Provider contract before production sync

The current interface is a prototype. Before large-directory or cloud sync it needs:

- Paged or streaming listing, with explicit completion and cancellation semantics.
- Capability metadata for stable IDs, checksum algorithms, timestamp precision, case sensitivity, overwrite, conditional updates, range reads, and resumable uploads.
- Typed checksum values instead of comparing unspecified hash strings or arbitrary ETags.
- Expected object identity/version checks before replacement or deletion. A concurrent change must return a conflict rather than overwrite newer data.
- Resume handles and operation identity where the provider supports them. Move is not inherently idempotent; the engine must reconcile its result before retrying.

The local scanner uses a streaming listing contract and recognizes link/special-file entries without following them. The mutating provider still needs race-resistant symlink/reparse-point containment and cancellation during transfer; lexical path checks alone do not cover those cases.

The CLI preview stores current metadata in a temporary SQLite index with bounded cache and batch commits. It requires both scans to complete, rejects duplicate provider entries, and blocks descendants of unsupported entries or file/directory conflicts. It makes initial-merge suggestions only, without modifying either directory or inferring deletions. Size/time matches remain unverified content candidates. This index is not the durable historical baseline planned for the sync engine.

Scanning and planning share one cooperative process-CPU budget in the CLI. Feedback is based on process CPU time, including Go GC and SQLite. This approximate average is separate from the future transfer/queue scheduler and does not constitute an operating-system CPU cap.

## First implementation boundary

The first engine milestone will use two local directories behind the same Provider interface. This makes upload, download, rename, delete, restart recovery, and conflict behavior testable without depending on a network service.

WebDAV is the first remote adapter because its protocol is documented and can validate the remote-provider contract. OpenList can optionally bridge domestic drives, but backend capabilities and the combined resource footprint must be measured. QuarkOpen exists in OpenList, but its documentation explicitly says this is not a generally open interface; application credentials and independent token refresh remain unresolved for PanBridge.

rclone bisync is a comparison baseline and a candidate execution backend, not a decided dependency. A folder pair must have one owner of synchronization decisions: when bisync owns reconciliation, PanBridge must not concurrently apply its own mutation plan to that pair. The feasibility phase determines the boundary before implementation continues.

macOS and Windows are target platforms for full local-copy synchronization. File placeholders and on-demand downloads require separate operating-system integration. Native runtime testing is required in addition to cross-compilation.

See [references.md](references.md) for verified projects and [performance.md](performance.md) for resource defaults and the measurement plan.
