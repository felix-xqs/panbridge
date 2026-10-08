# 本地扫描与预览验证

当前里程碑提供 scan 和 dry-run，验证大目录元数据处理、计划生成和资源占用。它尚未上传或下载文件，也没有后台监听、历史基线或持续双向同步。

## 可复现测量

先构建可执行文件，排除 Go 编译成本：

```bash
go build -o ./bin/panbridge ./cmd/panbridge

python3 scripts/make_fixture.py \
  --output /tmp/panbridge-fixture \
  --files 100000 --layout flat --changes 10 --large-gib 5

python3 scripts/measure.py \
  --binary ./bin/panbridge \
  --fixture /tmp/panbridge-fixture \
  --output /tmp/panbridge-measurement.json
```

Windows 下使用新建的测试目录，例如 `C:\Temp\panbridge-fixture`，可执行文件改为 `bin\panbridge.exe`。这些测量脚本仅依赖 Python 标准库。测试数据必须写到新的或空的目录，脚本拒绝覆盖已有文件。

fixture 在左右目录各创建指定数量的空文件，左侧十个文件写入少量内容；flat 模式将所有文件放在同一目录，nested 模式每千个文件分组。large-gib 通过 truncate 创建额外的大文件，是否实际稀疏取决于操作系统与文件系统，需要预留相应空间；本次 macOS 样本为稀疏文件，仅验证元数据处理，不用于评估真实传输速度或磁盘吞吐。

CPU 默认目标是单个逻辑核心的 25%，每批根据实际进程 CPU 时间调整暂停，扫描、索引与计划阶段共享预算。用 measure.py 的 cpu-budget 参数可调整，例如：

```bash
python3 scripts/measure.py \
  --binary ./bin/panbridge --fixture /tmp/panbridge-fixture \
  --cpu-budget 0 --scan-pause 0 \
  --output /tmp/panbridge-throughput.json
```

这会关闭协作式 CPU 节流和固定批次暂停，用于比较吞吐。CPU 预算是平均目标，采样、批次大小、垃圾回收与系统调度都会影响瞬时占用；它不是操作系统硬配额。

## 输出与语义

- scan 返回文件、目录、链接/特殊项数量、文件大小总和及资源统计，不读取文件内容。
- dry-run 用临时 SQLite 索引比较两次完整扫描，每 512 项提交一批，目录枚举每批 256 项，最多递归 128 层。
- 左右独有文件列为初次合并建议；同名元数据差异需要同步历史或内容校验，文件/目录同名冲突的后代全部阻止规划。
- 普通符号链接与不支持的特殊文件不被遍历；`.panbridge`、`.panbridge-upload-*`、`.panbridge-index-*` 是保留的内部命名空间，扫描忽略它们。
- 所有扫描必须完成。中断、错误、重复分页条目或过深目录会使预览失败，不输出成功汇总。
- 大小和时间相同只算 metadata_matches，不能证明文件内容相同；预览不推断删除，也不执行任何修改。
- JSON 预览采用逐行记录，最后一行是完整汇总；默认只输出前 50 条计划，limit 为 0 时只输出汇总。

当前初次预览尚未处理所有跨平台大小写/Unicode 名称冲突，不应把输出当成可直接执行的生产同步计划。正式执行前还需文件版本检查、名称策略以及可恢复的操作日志。

## 指标口径

CPU 由进程用户态和内核态耗时除以测量区间计算，涵盖 Go GC 和 SQLite 的线程；100% 表示占用一个逻辑核心，允许超过 100%。它不是整台电脑的 CPU 百分比。

process_peak_rss_bytes 来自操作系统的进程生命周期峰值，包含启动后的内存；heap_bytes 是结束时 Go 堆的快照，两者不能互相替代。Unix 和 Windows 使用各自的系统统计接口；跨平台构建通过不代表 Windows 运行测量已完成。

elapsed_seconds 覆盖扫描或临时索引/预览操作，不包括 Go 编译、测试数据生成和最终临时目录清理。单次样本受缓存与机器负载影响。

## 第一轮结果

实测环境为 macOS 13.3.1、Intel x86_64、12 个逻辑核心、Go 1.23.3；每侧十万个空文件、十处差异和一个 5 GiB 稀疏文件。结果为单次样本，缓存条件和机器负载并未严格控制。

| 操作 | 固定 5 ms 暂停版本 | CPU 目标 25% 版本 |
| --- | --- | --- |
| 扫描一侧 | 3.44 s / CPU 73.3% / 峰值 RSS 10.8 MiB | 14.70 s / CPU 25.0% / 峰值 RSS 10.4 MiB |
| 扫描两侧并预览 | 18.48 s / CPU 120.1% / 峰值 RSS 33.8 MiB | 102.96 s / CPU 25.0% / 峰值 RSS 34.7 MiB |

两次预览均识别十处待确认差异，其余 99991 项为未经内容校验的元数据匹配。节流降低了平均 CPU，同时增加耗时；这些数据不证明后台空闲时已经达到 1% CPU 目标。

原始记录：[加入 CPU 反馈前](measurements/2026-10-08-macos-intel-100k-before-cpu-budget.json)、[当前默认值](measurements/2026-10-08-macos-intel-100k.json)。

[一万文件补充样本](measurements/2026-10-08-macos-intel-10k.json) 的扫描耗时 1.12 s，双目录预览耗时 11.09 s；预览 CPU 为单核口径 25.0%，峰值 RSS 18.3 MiB。与十万文件的 34.7 MiB 对比，这个规模区间的内存没有按文件数量成比例增加；更大的目录和不同深度仍需分别验证。

这些样本只能证明当前一次性元数据扫描和比较的行为。稀疏大文件未被读取，十处差异仍通过完整扫描发现；哈希、真实传输、局部监听、后台空闲占用和真实网盘错误恢复属于后续测试。

## 接下来的实现顺序

1. 验证 WebDAV 和可选 OpenList 的覆盖、改名、校验和、Range 与账号限流；与 rclone bisync 做行为对照后确定执行后端。
2. 建立持久化历史基线，区分新建、修改、删除与首次接入；保留冲突和删除恢复信息。
3. 加入本地监听与局部扫描，验证修改十个文件时的 stat 范围、哈希次数及后台空闲负载。
4. 实现有界传输队列、暂停与恢复，再加入桌面托盘和目录配置。
