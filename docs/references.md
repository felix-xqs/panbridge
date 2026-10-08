# 同类项目调研

核查日期：2026-10-08。结论来自实际读取的 GitHub README、仓库元数据、发布附件、部分驱动源码及项目文档。尚未用真实网盘账号运行这些项目，也没有测量它们的 CPU、内存或速度；README 的性能描述仅作为设计参考。

## 结论

最值得组合参考的是 **rclone 的传输和 bisync、Syncthing 的事件检测与同步安全、OpenList 的国内网盘适配、OpenList Desktop 的跨平台集成**。

PanBridge 的目标是用户选择一个本地目录后，自动上传和下载、保留双端冲突、断网后继续工作。网盘挂载、分享链接转存、单向追加下载分别有不同的数据一致性要求。

Go + SQLite + Provider 的主线可以保留，但应先验证网盘能力和资源占用，再实现完整双向同步。OpenList WebDAV 可作为可选接入桥梁；是否把 rclone 作为执行后端，要通过技术验证决定。

## 重点项目

| 项目 | 已核查的能力 | macOS / Windows | 对 PanBridge 的价值与边界 |
| --- | --- | --- | --- |
| [rclone/rclone](https://github.com/rclone/rclone) | 多存储后端、流式传输、限速、并发控制、bisync 双向同步 | 支持两者 | 优先研究成熟传输与双向同步；当前上游后端列表没有百度和夸克，可通过 WebDAV 桥接 |
| [syncthing/syncthing](https://github.com/syncthing/syncthing) | 持续同步、文件监听、定期扫描、冲突副本、临时文件替换 | 发布包覆盖 macOS Intel/Apple 芯片和 Windows | 研究事件检测、变更索引和数据安全；它使用设备间协议，不能直接代替网盘适配器 |
| [OpenListTeam/OpenList](https://github.com/OpenListTeam/OpenList) | 百度、夸克等存储驱动，WebDAV 文件操作 | 服务可跨平台部署 | 国内网盘接入参考；各驱动的覆盖、凭证刷新、写入和代理行为仍需验证 |
| [OpenListTeam/OpenList-Desktop](https://github.com/OpenListTeam/OpenList-Desktop) | 桌面管理 OpenList，通过 rclone 挂载 WebDAV | 发布附件包含 macOS Intel/Apple 芯片和 Windows x64/ARM64 | 参考 Tauri、进程生命周期和打包；README 核心工作流是挂载 |
| [qjfoidnh/BaiduPCS-Go](https://github.com/qjfoidnh/BaiduPCS-Go) | 百度命令行、上传下载、同名上传策略、凭证登录 | 发布附件覆盖两者 | 参考百度传输和故障处理；Cookie/BDUSS 登录不能当成我们的官方 OAuth 方案 |
| [nextcloud/desktop](https://github.com/nextcloud/desktop) | Nextcloud 服务端与桌面目录同步 | README 明确支持两者 | 参考桌面同步体验、状态和错误处理；服务端协议与国内网盘不同 |

本次核查时，重点仓库均未标记 archived。稳定发布附件可见 rclone v1.75.1、Syncthing v2.1.6、OpenList Desktop v0.9.1、BaiduPCS-Go v4.0.2。发布和活跃状态会变化。

## 更接近国内需求的项目

### quark-timed-sync

[Shadowfax-YJ/quark-timed-sync](https://github.com/Shadowfax-YJ/quark-timed-sync) 已采用“桌面客户端 + 内置 OpenList + rclone”的组合。README 描述支持 Windows、macOS Universal 和 Linux，有托盘、暂停、日志、自动更新与打包测试。

默认下载本地尚不存在的文件，保留已有内容；另有依赖云端修订记录的可选更新机制。它证明整合已有工具的路线有人实现，但实际资源占用、账号可用性仍需验证。它没有提供 PanBridge 所需的通用本地上传/双向删除闭环。

### quark-cli

[yuansui486/quark-cli](https://github.com/yuansui486/quark-cli) 是 Rust 夸克客户端，README 说明有授权、上传、断点续传和 manifest 增量下载。发布平台说明为 Windows 和 Linux，本次未确认 macOS 支持。

可参考其约 256 KiB 单下载缓冲区、任务检查点和顺序上传分片。这些是文档给出的实现描述，尚非本次性能实测。

其 download sync 会用云端版本替换本地修改，并保留云端已删除文件的本地副本，因此不能直接复用为我们的双向冲突和删除规则。

### Sync Vault

[abcamus/obsidian-sync-vault-ce](https://github.com/abcamus/obsidian-sync-vault-ce) 的 README 声明支持百度、夸克等多网盘、双向同步、删除恢复和虚拟文件访问。它面向 Obsidian vault，可作为多云同步和按需读取的补充参考；未验证独立目录同步、平台兼容性或运行性能。

### 历史项目及产品参考

- [wishinlife/SyncY](https://github.com/wishinlife/SyncY)：百度同步，面向 Linux、路由器和 NAS，最后推送时间为 2020 年。可参考历史设计，不能据此假定当前 API 仍能用。
- [mqflive/syncpan-release](https://github.com/mqflive/syncpan-release)：多网盘桌面产品，主要分享转存、追更和跨盘操作。发布仓库没有明确开源许可证，不能当作可直接复用的开源引擎。
- [haiwen/seafile-client](https://github.com/haiwen/seafile-client)：专用服务端的桌面客户端，可作补充参考；核心同步还依赖其他 Seafile 仓库。

上一轮的 FreeFileSync/FreeFileSync 链接这次未能验证，不再作为已确认的 GitHub 仓库推荐。

## 百度与夸克的接入限制

OpenList 的存在不意味着所有后端具有相同的 WebDAV 语义。

- 百度驱动区分官方与非官方下载接口，可使用在线服务刷新令牌。PanBridge 应先验证自己的官方授权、下载权限和账号限速。
- OpenList 已有 quark_open 驱动，使用 AppID、SignKey、access token 和 refresh token。但 [夸克文档](https://doc.oplist.org/guide/drivers/quark) 明确说明该 “Open” 并非真正意义上的开放接口，尚不能认定 PanBridge 可以申请官方接入资质。
- 本次读取的 quark_open 源码中，本地刷新令牌尚未实现，默认依赖在线刷新服务；quark_uc 使用 Cookie，TV 驱动仅提供访问和下载。
- 夸克配置包含 OnlyProxy、NoOverwriteUpload，上传还可能先缓存文件并计算 MD5/SHA1。桥接时必须测量 OpenList 的磁盘、CPU、内存和分片并发，仅限制 PanBridge worker 不足以限制整个组合。

WebDAV 验证至少覆盖大目录、校验和含义、同名覆盖、移动、删除、Range 下载、时间精度、凭证过期与部分失败。能力不足的后端应明确降级或拒绝对应操作。

源码证据：

- [rclone 后端目录](https://github.com/rclone/rclone/tree/master/backend)
- [OpenList 驱动](https://github.com/OpenListTeam/OpenList/tree/main/drivers)
- [QuarkOpen 配置](https://github.com/OpenListTeam/OpenList/blob/main/drivers/quark_open/meta.go)
- [QuarkOpen 令牌刷新](https://github.com/OpenListTeam/OpenList/blob/main/drivers/quark_open/util.go)
- [QuarkOpen 上传](https://github.com/OpenListTeam/OpenList/blob/main/drivers/quark_open/driver.go)
- [百度文档](https://doc.oplist.org/guide/drivers/baidu)

## 性能与一致性机制

[Syncthing 同步机制](https://docs.syncthing.net/users/syncing.html) 描述：监听配合定期扫描，默认完整扫描每小时一次并随机错开，先比较元数据，检测到变化才重新哈希；监听默认积累十秒，删除延迟更长。PanBridge 不必照搬，但原来的 500 ms 合并、15 分钟完整扫描对大目录偏激进，应调整并测量。

[Syncthing 配置](https://docs.syncthing.net/users/config.html) 提供低优先级和跨目录 I/O 并发控制。多个目录的总负载需要全进程调度，不能每个 Provider 各自增加 worker。

[rclone 选项](https://rclone.org/docs/) 分别限制文件传输、检查、缓冲内存、单文件多线程和 API 频率。fast-list 用内存换请求，不能默认认为它总是更快、更省资源。

[rclone bisync](https://rclone.org/bisync/) 使用上次目录快照识别增改删，并提供访问检查、删除阈值、冲突处理、锁和中断恢复。resync 是重建基线，不能每次调用或在历史丢失后静默执行。

代码许可证不同：rclone 为 MIT，Syncthing 为 MPL-2.0，OpenList 为 AGPL-3.0，OpenList Desktop 为 GPL-3.0，BaiduPCS-Go 为 Apache-2.0。通过独立进程/API 连接与复制源代码是不同复用方式，选择依赖时应按实际许可证处理。

## 推进建议

1. 保留 Go 项目，现有 Provider 定位为原型，补分页/流式列举、校验和算法与能力描述。
2. 用两个临时本地目录比较 rclone bisync 与自研增量扫描的行为和负载，先不决定内嵌整套 rclone。
3. 加可选 OpenList WebDAV 端点，验证百度/夸克真实文件操作，分别测量各进程和总体负载。
4. 优先实现持久化基线、冲突保留、删除保护和资源调度，再建设桌面界面。
5. macOS 和 Windows 都作为首发目标，先做完整本地副本的目录同步。iCloud 式按需占位文件另需 macOS File Provider / Windows Cloud Files 集成，单靠 Go 交叉编译不能完成。

实验方案见 [performance.md](performance.md)，后续任务见 [roadmap.md](roadmap.md)。
