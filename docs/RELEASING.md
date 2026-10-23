# 发布指南

插件：`io.dbx.apollo`，发布者：`zzhao5726`，需要 DBX 0.6.47+。源码和发布资产保留在 [zzhao5726/dbx-apollo](https://github.com/zzhao5726/dbx-apollo)。

1.0.0 已于 2026-10-23 经 [DBX Store PR #322](https://github.com/t8y2/dbx-store/pull/322) 完成签名并合并。本分支仅准备下一版本的名称与文案，不创建新 Release、不重发 1.0.0，也不提交新的商店审核 PR。

## 1. 源码身份

GitHub 属主和发布者均为 `zzhao5726`，Go 模块为 `github.com/zzhao5726/dbx-apollo`，提交者为 `zhao.zhao <zz19950726@gmail.com>`。插件 ID 保持 `io.dbx.apollo`。已发布 1.0.0 的商店名称为 Apollo Config Workbench / Apollo 配置工作台；下一正常版本使用 Apollo Release Workbench / Apollo 发布工作台。

首发代码保留一个初始提交，版本为 `1.0.0`，发布标签为 `v1.0.0`。标签和各平台安装包必须对应同一源码提交。后续正式版本递增版本号并新增提交，不改写已经签发的源码标签。

## 2. 发布 GitHub Release

只有获得新版本发布授权后，才统一递增 Manifest 与后端自报版本。在仓库 Releases 页面选择 **Draft a new release**，创建与新版本匹配的标签 `v<新版本号>`，标题填写 `Apollo Release Workbench <新版本号>`。发布工作流由 `release: published` 触发；仅推送分支或标签不会触发。

不要在已签发的 `v1.0.0` 标签上重新运行发布工作流，也不要将本分支的名称或内容回填到历史 Release。手动构建补跑仅面向尚未签发的新版本候选。

等待 Actions 的 `Release DBX plugin` 完成。工作流使用固定 CLI 0.1.9 和对应上游提交，生成五个平台：`darwin-arm64`、`darwin-x64`、`windows-x64`、`linux-x64`、`linux-arm64`。

Release 应包含每个平台的 `.dbxp`、`.artifact.json`，以及汇总的 `release-candidates.json`。检查下载 URL、版本、大小和 SHA-256 一致；各平台以 Actions 产物与公开下载核对结果为准。

候选包与 metadata 的文件名包含源码提交前 12 位，下载 URL、SHA-256 和大小共同固定审核对象。尚未签发的新版本候选内容变更时使用新文件名，不覆盖旧候选包，并同步更新该新版本的汇总清单和审核材料。重新提交前核对安装包与用户验收版本一致；已发布 1.0.0 的清单和资产保持原样。

这些是未签名审核候选包。GitHub Release 发布成功不等于已经进入 DBX 官方商店；候选包仅用于允许未签名包的本地开发安装。后续发布必须递增版本，不能覆盖已发布版本的资产。

### 已发布 1.0.0 的发布说明（历史记录）

- 应用、集群和命名空间管理，支持应用搜索与多环境切换。
- 配置增删改、搜索，以及 properties、YAML、JSON、XML、TXT 多格式编辑。
- 配置导入预览、同名配置跳过或覆盖、文件导出。
- 发布差异预览、普通与紧急发布、发布历史及回滚。
- 灰度规则管理、灰度发布、合并全量及放弃灰度。
- 支持 Token、账号密码、SSO Cookie 登录，并可选择 Apollo 1.x / 2.x 或 3.x 接口。
- 中文界面、深浅主题和操作异常详情查看。

## 3. 提交 DBX 官方商店

原审核 [PR #296](https://github.com/t8y2/dbx-store/pull/296) 已关闭，后续 [PR #322](https://github.com/t8y2/dbx-store/pull/322) 已完成 1.0.0 上架。用户明确允许发布新版本并提交 PR 后，再准备新的审核提交：

- 已存在的 `publishers/zzhao5726.json` 保持发布者归属，不重复新增。
- `candidates/io.dbx.apollo.json`：只提交新版本，包含固定源码提交、Apache-2.0 许可证、商店文案与各平台候选包 URL / SHA-256 / 大小。资产数据来自新 Release，不能预填不存在的跨平台产物。
- 从新版本 Manifest 显式同步 `name`、`description`、`localizations["zh-CN"].name`、`localizations["zh-CN"].description`；已有插件省略这些字段会保留旧商店文案。
- 中英文功能介绍沿用 README 的产品定位，保留接口模式、认证方式、权限与目标环境验收限制，不写成全版本或全认证已验收。

不手改 `plugins/io.dbx.apollo.json` 或 `catalog/index.json`，由商店签名流程生成。已发布 1.0.0 的版本号、时间、发布说明、产物 URL、SHA-256、大小及签名信息保持不变；新版本也不自行将 `verified` 设为 `true`。

遵循 [商店贡献指南](https://github.com/t8y2/dbx-store/blob/main/CONTRIBUTING.md)，按模板说明原生 Go 后端通过网络访问用户配置的 Apollo Portal，密钥由 DBX 连接密钥保存。不要提交完整源码副本、二进制或密钥，也不要自行填写官方签名或已认证状态。

维护者审核后在同一个 PR 中签名、发布正式安装包并生成目录，校验通过后合并。此后才能在 DBX 官方插件商店搜索、安装与更新。开发者无需生成或提供官方签名私钥。

后续可申请将仓库登记到 `dbx-store/automation/plugin-sources.json` 并启用 `autoUpdate`，由商店根据新 Release 自动创建候选 PR；审核、签名、合并仍由维护者完成。

依据：[DBX 插件开发与上架文档](https://github.com/t8y2/dbx/blob/main/docs/content/docs/plugin-development.cn.mdx)、[商店贡献指南](https://github.com/t8y2/dbx-store/blob/main/CONTRIBUTING.md)。

## 宿主权限说明

Manifest 显式声明 `permissions: []`。工作台通过 `dbxPlugin.invoke` 调用自己的 Go sidecar；原生 HTTP 请求访问当前连接配置的 Apollo，写操作仍受 Apollo 用户及 Token 范围约束。空 Host 权限不表示只读，也不表示原生进程被沙箱隔离。

导出通过 `dbxPlugin.saveFile` 将内容交给宿主，用户在原生另存为对话框中决定保存位置；插件没有任意路径文件系统入口。宿主的 [`host.saveFile` 分支](https://github.com/t8y2/dbx/blob/main/apps/desktop/src/lib/plugins/pluginHostBridge.ts) 不要求 `host.filesystem`；该权限用于 `openFilesystem`。因此不为配置写入或另存为额外申请不对应的 Host 权限。
