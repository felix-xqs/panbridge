# Roadmap

## Phase 0: foundations

- [x] Repository and Go module
- [x] Provider and file-entry model
- [x] Architecture and safety rules
- [x] Performance and cross-platform constraints
- [x] Live GitHub research and provider-capability review (2026-10-08)
- [ ] SQLite state schema
- [x] Temporary SQLite current-tree index for read-only preview
- [ ] Structured logging and error model

## Phase 0.5: feasibility and resource measurements

- [ ] Compare rclone bisync behavior with local prototype workloads
- [ ] Decide ownership of sync decisions and transport backend
- [ ] Validate optional OpenList WebDAV file-operation capabilities
- [ ] Verify Baidu authorization and Quark application/token availability
- [x] Reproducible ten-thousand/one-hundred-thousand-file fixture generator
- [x] One-hundred-thousand-file metadata scan and preview measurement entry points
- [x] macOS Intel 10k/100k metadata measurements and CPU-throttle comparison
- [ ] Measure idle, initial scan, small changes, large transfers, and multiple pairs
- [ ] Record PanBridge and optional bridge-process resource use separately

## Phase 1: local sync prototype

- [x] Local Provider prototype
- [ ] Symlink/reparse-point containment and special-file handling
- [x] Streaming local listing (256 entries per batch)
- [ ] Provider capabilities, checksum types, and remote paging
- [x] Recursive metadata scan with depth bounds and link skipping
- [ ] Global resource scheduler and cancellation during transfers
- [x] Shared cooperative CPU budget for CLI scan and preview
- [ ] One-way upload and download plans
- [x] Read-only initial-merge preview with bounded output
- [ ] Persistent sync baseline and three-way change comparison
- [ ] Conflict-copy policy
- [ ] Complete-scan checks, deletion thresholds, and recovery copies
- [ ] Restartable operation queue

## Phase 2: WebDAV and CLI

- [ ] WebDAV Provider
- [ ] Config file and credential loading
- [ ] Periodic scan and local file watcher
- [ ] Pause, resume, retry, and bandwidth limits

## Phase 3: desktop client

- [ ] Tray process and sync status
- [ ] Folder-pair management
- [ ] Conflict and error review
- [ ] macOS, Windows, and Linux packaging
- [ ] Native macOS Intel/Apple silicon and Windows runtime/performance tests
- [x] GitHub Actions native functional-check matrix (runs after repository publication)

## Phase 4: domestic drive adapters

- [ ] Baidu Netdisk adapter using supported APIs
- [ ] Quark adapter after application credentials and independent refresh are verified
- [ ] Capability matrix and provider-specific limitations
