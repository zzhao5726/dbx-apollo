# 1.0.0 验证记录

日期：2026-10-19 至 2026-10-20。首发版本为 1.0.0；开发阶段验证、正式包安装及真实客户端结果分别记录。

## 编译与模拟验证

- Go 编译、静态检查、前端语法、Manifest 解析与差异格式检查通过；未新增测试类、未启动本地业务服务。
- 模拟交互覆盖 Apollo 1.x / 2.x 和 3.x 的配置增删改、导入预览、文本保存、发布差异、回滚、集群与命名空间管理、灰度规则 / 合并 / 放弃。
- 另验证部门与负责人选择、邮箱带出、管理员去重与移除、搜索失败重试、响应过期隔离、登录方式的显示与必填规则、中文详情、时间格式、深浅主题和窄窗口。
- 10000 个模拟应用覆盖搜索、虚拟列表、键盘选择和加载中切换；没有以模拟 DOM 的耗时作为真实渲染性能数据。

## 真实 Apollo 3.x 管理

环境：DBX 0.6.47，连接 `@me`，使用 Apollo 用户访问 Token。密钥未写入源码或本文。

- `GET /openapi/v1/user-tokens/current` 返回 200，自动获取所属账号及范围。登录信息显示中文字段与 `yyyy-MM-dd HH:mm:ss` 时间，不展示原始数据或密钥。
- 登录方式为 Token、账号密码、SSO Cookie 三选一；Cookie 方式隐藏 Token，未填写 Cookie 时测试、保存不可用。原生 / LDAP 登录与真实 SSO 会话未据此认定通过。
- 测试资源：应用 `dbx-pat-check-20261019`，DEV 集群 `dbx-verify`，私有 properties 命名空间 `dbx-check`。

| 操作 | 实际结果 |
|---|---|
| 部门与用户下拉 | 自动加载；选择 apollo 后自动带出邮箱 |
| 创建应用 | 成功，默认集群和 application 可读 |
| 创建集群 | 成功，DEV 集群数量由 1 变 2 |
| 创建命名空间定义并添加到当前集群 | 成功，当前集群出现 dbx-check |
| 删除命名空间实例并重新关联 | 成功，列表数量随删除与关联更新 |
| 删除应用级命名空间定义 | 成功，当前集群仅剩 application |
| 删除临时集群与应用 | 成功，回读确认临时资源不存在 |

定义、实例、集群和应用分别操作并回读；原有 `Aaaaa`、`demo`、`demo2` 保留，未修改其配置、发布或权限。

界面证据：[创建表单](screenshots/user-token-create-app.png)、[登录信息](screenshots/user-token-info.png)、[Cookie 表单](screenshots/connection-cookie-only.png)、[发布详情](screenshots/release-details-no-raw.png)。

## 1.0.0 安装与真实双副本灰度

2026-10-20（本机 Asia/Tokyo），DBX 已覆盖安装 1.0.0。包内 Manifest、UI、图标、平台入口及 checksums 与安装目录逐文件一致，后端自报版本也是 1.0.0。

使用 `@me` 的 Apollo 用户访问 Token，通过插件界面创建独立 `springboot-demo` 应用，部署两个真实 Spring Boot 客户端；Java 17、Spring Boot 3.5.16、Apollo Java Client 2.5.0。客户端通过 ConfigService 获取配置，不持有 Portal 管理 Token。

| 阶段 | 副本一 | 副本二 | 回读结果 |
|---|---|---|---|
| 创建应用、导入两项草稿并发布基线 #3 | baseline | baseline | 两副本 Ready，source=REMOTE |
| 按副本一 IP 保存规则并灰度发布 #4 | canary | baseline | 只命中指定副本，Spring 注入值与客户端 API 一致 |
| 合并并全量发布 #5 | canary | canary | 第二个副本动态收到变更 |
| 结束灰度、从 #5 回滚至 #3 | baseline | baseline | 两副本动态恢复，各收到两次值变化 |
| 将草稿恢复基线 | baseline | baseline | 发布差异为 0，无生效灰度分支 |

四个阶段的容器 ID 一致、重启次数均为 0，健康检查均为 UP；应用日志分别记录 baseline → canary → baseline。插件「使用实例」回读到两个实际 Pod IP。经用户确认结束灰度分支；随后按用户要求删除演示应用、两个副本、Deployment、Service 及专用命名空间，回读确认已清理。

本轮另建的 `dbx-release-check-20261020` 完成用户下拉、应用创建、配置写入、发布 #2、灰度分支 / 草稿 / 规则保存验证后，经用户确认删除；应用列表由 5 项回到 4 项，精确搜索该 AppId 为 0 项。原有应用和 `@ying` 数据未修改。

## 大量应用导航

开发包在 DBX 安装后逐文件核对一致。真实连接 `@ying`，接口兼容度选择 1.x / 2.x，返回 2340 个应用；本节仅验证读取与导航。

| 检查 | 实际结果 |
|---|---|
| 展开与滚动 | 同时显示 7 个候选，可继续滚动；共 2340 个应用 |
| AppId 搜索 | xcrm.site.platform 命中 1 项 |
| 中文名称搜索 | 电销web命中对应应用 |
| 无匹配与清空 | 无匹配显示 0 / 2340，清空恢复全部候选 |
| 键盘与鼠标切换 | 当前应用和配置路径同步更新，完成后恢复原选择 |

未在真实连接测量性能耗时，也未验收快速连续切换或加载中切换；未修改 Apollo 数据。

## 导出与错误提示

开发阶段安装包 SHA-256：`f94f4ca0e6e8512b52ef90b6e7d63e7ce95cc5b51c3ba578fb9427da7f6eb191`。逐文件核对安装内容一致；更新前临时关闭 @ying，安装后重连，其他数据库标签保留，未重启 DBX。

- 导出弹出原生另存为窗口，取消不提示成功；保存后显示完整路径。
- 本地导出文件存在、JSON 可解析，17985 字节，范围为 `DEV / kf_win_oa / default / application`，共 7 项；未输出配置值。
- 导入窗口输入 `{"bad":1}`，仅点击「预览变更」触发本地校验：消息在当前窗口内显示，输入保留，页面顶部无异常横幅。
- 「错误详情」默认折叠，展开后保留原始消息与完整堆栈。随后取消导入，未点击「导入草稿」，未修改 Apollo。

## 修复依据与原始错误

用户查询显式保留 `keyword=`；第三方 Token 的应用授权、管理用户、创建应用分别校验。删除应用要求超级管理员，用户访问 Token 还需对应范围。不会在 401 / 403 后自动切换身份。

首次用户查询因遗漏 keyword 返回：

```json
{
  "name": "Error",
  "message": "apollo GET /openapi/v1/users (HTTP 500): {\"exception\":\"org.springframework.web.bind.MissingServletRequestParameterException\",\"message\":\"Required request parameter \\u0027keyword\\u0027 for method parameter type String is not present\",\"status\":500,\"timestamp\":\"2026-10-19T09:59:04.13023417\"}",
  "stack": "@about:srcdoc:331:52"
}
```

补齐 keyword 后，曾返回管理用户权限错误；随后授权连接重新验证下拉成功，未将该次 403 直接归因为当前权限状态：

```json
{
  "name": "Error",
  "message": "apollo GET /openapi/v1/users?keyword=&limit=20&offset=0 (HTTP 403): {\"exception\":\"org.springframework.security.access.AccessDeniedException\",\"message\":\"Manage users permission is required\",\"status\":403,\"timestamp\":\"2026-10-19T10:00:17.706200834\"}",
  "stack": "@about:srcdoc:331:52"
}
```

命名空间删除曾误用读取接口的路径顺序；修复为 `/openapi/v1/apps/{appId}/envs/{env}/clusters/{clusterName}/namespaces/{namespaceName}` 后实际删除通过：

```json
{
  "name": "Error",
  "message": "apollo DELETE /openapi/v1/envs/DEV/apps/dbx-pat-check-20261019/clusters/dbx-verify/namespaces/dbx-check (HTTP 400): {\"exception\":\"org.springframework.web.HttpRequestMethodNotSupportedException\",\"message\":\"Request method \\u0027DELETE\\u0027 is not supported\",\"status\":400,\"timestamp\":\"2026-10-19T11:56:43.689256753\"}",
  "stack": "@about:srcdoc:331:52"
}
```

## 正式包与验收边界

- 上述实测使用的历史 1.0.0 Apple Silicon 候选包：5481971 字节，SHA-256 `ef6fdda3a5d690d534c87869d879a4411c2ed946f61a387f094536daf1aa3bfc`。该包版本、UI、平台入口、下载 URL、大小和 metadata 核对通过。
- 该历史包已实际安装并完成上述真实双副本验证。当前候选包以 Release 的 `release-candidates.json` 为准，重新打包后核对各平台包内源码提交、内容与校验值；构建检查不等同于新包已安装运行，本机结果不扩大为所有平台验收。
- 旧版 Portal 路由完成官方源码核对、编译与模拟验证；真实 1.x / 2.x 管理写入、LDAP 和 SSO 会话仍待目标环境验收。
