# Apollo Release Workbench — DBX 插件

Apollo 发布工作台，在 DBX 中管理 Apollo 应用、集群、命名空间、配置和发布，需要 DBX **0.6.47+**。

本分支仅准备下一正常版本的名称和文案，尚未发布。当前已签名版本仍为 **Apollo Config Workbench / Apollo 配置工作台 1.0.0**，插件 ID 始终为 `io.dbx.apollo`。Manifest 版本暂不调整；新版本发布前再统一递增，不重发或覆盖 1.0.0。

## 产品定位

### 中文

以配置变更和发布操作为核心，提供配置编辑、导入预览、发布差异、灰度规则、全量合并及回滚。可手动选择 Apollo 1.x/2.x 或 3.x 接口模式，并使用开放平台 Token、3.x 用户访问 Token、账号密码或已有 SSO Cookie。可用操作取决于服务端版本、认证方式及权限；旧版管理写入和账号密码、LDAP、SSO 会话需在目标环境验证。SSO Cookie 仅复用已有会话，不包含完整的 SSO/OIDC 登录流程。

### English

A workbench focused on configuration changes and releases: edit configurations, preview imports and release differences, manage gray-release rules, merge to a full release, and roll back. Select the Apollo 1.x/2.x or 3.x API mode and use an Open Platform token, a 3.x user access token, username/password, or an existing SSO Cookie. Available operations depend on server version, authentication and permissions. Legacy management writes and password, LDAP and SSO sessions require target-environment validation. SSO Cookie authentication reuses an existing session; it does not provide a complete SSO/OIDC sign-in flow.

具体兼容限制见下方功能矩阵；历史实测及未验收范围见 [验证记录](docs/VALIDATION.md)，不将源码实现或接口模式等同于全版本、全认证及五平台全部验收。

## 连接与安装

连接目标为 **Apollo Portal**（通常是 8070），OpenAPI 不在 AdminService 上。

1. 新建 DBX「Apollo 发布工作台连接」，填写 Portal 地址和端口。**无需填写环境**，连接后从 Apollo 返回的应用、环境列表选择。
2. 按「接口兼容度」明确选择 `1.x / 2.x` 或 `3.x`，不自动检测版本。
3. 「登录方式」三选一：**Token、账号密码、SSO Cookie**。只显示并校验所选凭据；账号密码、Cookie 均无需 Token。
4. Token 支持「开放平台授权管理」的第三方令牌，以及 3.x 个人菜单「访问 Token」中的用户令牌。用户令牌直接粘贴 `apollo_pat_...` 即可，也接受已带 `Bearer ` 的值。
5. 「更多选项」包含变更操作人、默认应用和路径前缀。开放平台 Token 写入需填写 Apollo 已有账号；用户访问 Token 和 Portal 登录自动获取实际账号。默认应用和路径前缀均可留空。

本机已有 Go 和 DBX Plugin CLI 时执行：

```sh
dbx-plugin package .
```

如需安装打包工具：`npm install --global @dbx-app/plugin-cli`。
在 DBX 插件中心允许安装未签名的开发包，安装 `dist/` 中对应平台的 `.dbxp`。
已发布 1.0.0 的历史未签名候选包见 [Release 下载清单](https://github.com/zzhao5726/dbx-apollo/releases/download/v1.0.0/release-candidates.json)，不包含本分支文案变更；文件名包含源码提交前 12 位，Apple Silicon Mac 对应 `darwin-arm64` 包。正式签名版本通过 DBX 插件商店安装。

正式发布与 DBX 官方商店上架步骤见 [发布指南](docs/RELEASING.md)。

### 登录与权限

- **开放平台 Token**：使用原始 Token 鉴权，应用授权、创建应用和管理用户权限由开放平台分别配置。
- **用户访问 Token（3.x）**：自动添加一次 `Bearer `，通过 `/openapi/v1/user-tokens/current` 读取所属账号作为操作人。「登录信息」以中文字段展示名称、有效期、限流和操作 / 资源范围。实际权限是所属用户权限与 Token 范围的交集；删除应用、集群需要超级管理员账号及 `system:admin` 范围，无需另填 Cookie。
- **账号密码**：原生 / LDAP 使用 HTTP Basic，仅发送所选账号密码。
- **SSO Cookie**：填写完整 Cookie 请求头值，不含 `Cookie:` 前缀；仅发送 Cookie，过期后更新。

测试、连接和后续操作统一使用所选身份，隐藏字段中的旧密钥不参与请求。没有登录方式字段的历史连接默认使用 Token；原来同时配置 Portal 登录的连接需重新选择账号密码或 Cookie。密钥仅交给后端，不传入工作台。

3.x 的 Portal 用户可访问 OpenAPI。1.x / 2.x 的 OpenAPI 只接受 Token，因此选择账号密码或 Cookie 时，应用、导航、配置、集群、命名空间、发布与灰度改走 Portal WebAPI，并转换其返回结构。Token 连接仍保留旧版 OpenAPI 基础能力；旧版命名空间关联、删除和集群删除需切换为 Portal 登录。

连接测试校验当前身份及应用列表；Portal 登录读取 `/user`，用户访问 Token 读取自身身份接口，操作人均使用实际账号。工作台「登录信息」展示所用身份。请求失败不会自动更换身份重试，完整原始错误保留。

鉴权依据：[1.9 Token 过滤器](https://github.com/apolloconfig/apollo/blob/v1.9.0/apollo-portal/src/main/java/com/ctrip/framework/apollo/openapi/filter/ConsumerAuthenticationFilter.java)、[3.0 用户身份支持](https://github.com/apolloconfig/apollo/blob/v3.0.0/apollo-portal/src/main/java/com/ctrip/framework/apollo/openapi/filter/ConsumerAuthenticationFilter.java)。

用户 Token 依据：[Bearer 鉴权](https://github.com/apolloconfig/apollo/blob/v3.0.0/apollo-portal/src/main/java/com/ctrip/framework/apollo/portal/filter/UserTokenAuthenticationFilter.java)、[身份与范围接口](https://github.com/apolloconfig/apollo/blob/v3.0.0/apollo-portal/src/main/java/com/ctrip/framework/apollo/openapi/v1/controller/UserTokenOpenApiController.java)、[用户权限与 Token 范围交集](https://github.com/apolloconfig/apollo/blob/v3.0.0/apollo-portal/src/main/java/com/ctrip/framework/apollo/portal/component/UserTokenPermissionValidator.java)。

## 工作台与应用表单

- 顶部切换应用和环境，左侧选择集群、命名空间，右侧查看配置、差异、发布记录、灰度和实例。
- 连接页使用 DBX 原生表单，插件控制字段顺序、联动和说明。环境输入已移除，登录凭据按方式展开，操作人及其他可选设置收进「更多选项」；原生表单的边框、间距和外层弹窗由 DBX 渲染。
- 工作台统一表格、表单、弹窗和危险操作样式，适配窄窗口，并跟随系统深浅主题。
- 配置导出通过 DBX「另存为」选择文件位置；成功后显示完整保存路径，取消不提示导出成功。导出内容为当前命名空间已保存的草稿。
- 集群、命名空间、应用、发布、实例和 Portal 用户详情使用中文字段和配置表格展示。操作失败在当前弹窗中提示，页面顶部不显示异常横幅；「错误详情」默认收起，保留完整 JSON、原始消息、堆栈和原因链。
- 3.x 创建、编辑应用时，从 Apollo 查询部门列表，选择后提交对应部门编号与名称。
- 打开应用表单即加载 Apollo 用户，负责人直接下拉选择并带出邮箱，额外管理员下拉添加；账号或姓名筛选为可选项，无需先点搜索。AppId、应用名称仍由用户指定。
- 用户查询失败时保留表单输入和完整错误，可原地重新加载。查询需要的权限不因界面选择而改变。
- 删除应用需要超级管理员身份。配置 Portal 登录后使用该账号调用 Portal 删除接口；仅 Token 时使用 OpenAPI，第三方应用 Token 即使允许创建应用和管理用户也不能删除应用，需使用具备相应权限的用户访问令牌或配置 Portal 登录。权限失败保留原始错误，不自动切换鉴权重试。

详情、配置列表、发布记录及实例中的时间统一显示为 `yyyy-MM-dd HH:mm:ss`，保留服务端时间的日期和时分秒，不转换时区。移除详情页重复的接口限制和默认资源说明，删除入口及后端继续保护默认资源。

界面预览使用模拟数据：[工作台](docs/screenshots/workbench-light.png)、[深色主题](docs/screenshots/workbench-dark.png)、[创建应用](docs/screenshots/create-app.png)、[命名空间详情](docs/screenshots/namespace-details.png)。连接字段结构预览：[基础设置](docs/screenshots/connection-default.png)、[账号密码](docs/screenshots/connection-basic.png)。

## 功能与版本兼容

| 功能 | Apollo 1.x / 2.x | Apollo 3.x |
|---|---|---|
| 已授权应用列表、应用信息 | 支持 | 支持 |
| 新建、编辑、删除应用 | Portal 操作 | 支持 |
| 部门查询 | Portal 用户接口 | OpenAPI |
| 用户查询与筛选 | Portal 用户接口 | Portal 用户接口，或具有管理用户权限的 OpenAPI Token |
| 多环境切换 | 从应用 `envclusters` 获取 | 环境列表 + `envclusters` |
| 集群列表、详情、创建 | **支持** | 支持 |
| 删除集群 | 支持，需 Portal 超级管理员 | 支持，禁止删除 default |
| 创建命名空间 | **支持**，配置 Portal 鉴权后按需添加到当前集群 | 支持，按需添加到当前集群 |
| properties / yaml / yml / json / xml / txt | **支持** | 支持 |
| 公共 / 私有、部门前缀、说明 | **创建时支持** | 创建时支持 |
| 关联公共命名空间、补建已有定义 | 支持，需 Portal 鉴权 | 支持 |
| 检测并补齐当前集群缺失命名空间 | 使用「关联 / 补建」逐个添加 | 支持 |
| 命名空间详情、编辑锁信息 | 支持 | 支持 |
| 删除当前集群命名空间 / 应用级定义 | 支持，需 Portal 鉴权及对应权限 | 支持，输入名称确认 |
| 配置增删改、搜索 | 支持 | 支持，含字符串 / 数字 / 布尔 / JSON 类型 |
| 文本格式整块编辑 | 支持，固定使用 `content` 配置项 | 支持 |
| 导入、导出、变更预览 | 支持 | 支持 |
| 发布前差异、普通 / 紧急发布 | 支持 | 支持 |
| 当前生效版本及配置详情 | 支持 | 支持 |
| 完整有效发布历史、分页 | Portal 查看 | 支持 |
| 撤销当前发布，恢复上一有效版本 | 支持 | 支持 |
| 回滚到选定历史版本 | Portal 操作 | 支持 |
| 灰度分支、规则增删、灰度发布、全量合并、放弃灰度 | 支持 | 支持 |
| 使用实例、客户端 AppId 精确筛选、分页 | Portal 查看 | 支持 |

版本约束同时由界面和后端检查。旧版打开工作台不会调用 3.x 发布历史接口，也不会将接口失败显示成空配置。

### 命名空间创建范围

- Apollo 的经典创建接口建立的是 **应用级 AppNamespace 定义**，由服务端同步到各环境的 default 集群；创建不是仅作用于当前环境。
- 新建文本命名空间时，扩展名由 Apollo 添加。输入 `datasource` 并选择 yaml 会得到 `datasource.yaml`；输入已有 `.yaml` 后缀也不会重复。
- 公共命名空间默认添加部门前缀，可在创建表单取消。
- 旧版未配置 Portal 鉴权时，创建定义后界面切换到 default；配置后会检查并按需添加到当前集群。3.x 同样按需添加。
- 如果应用级定义创建成功、补建失败，会保留原始错误并提示从「关联 / 补建」重试，不会自动删除已创建的定义。
- 旧版关联候选来自公共定义和当前环境 default 集群；其他已有定义可以直接输入完整名称。关联后回读目标命名空间确认结果，避免旧版接口吞掉创建异常后返回 200 而误报成功。
- 所有版本的资源删除均需输入名称确认，并保护 default 集群、application 命名空间。旧版删除应用级定义后，各环境清理由服务端事件处理，响应成功不表示所有环境清理已完成。
- 名称、格式和公开属性创建后没有对应 OpenAPI 修改操作；界面展示详情和实际支持的操作。

### 配置与发布

- properties 配置按 Key 编辑；文件格式使用完整文本编辑器，JSON / XML 保存前检查语法。
- 导出的是 **当前草稿**。properties 导出 JSON，文本格式导出原始文本。
- properties 导入支持插件导出的 JSON、`[{"key":"k","value":"v","comment":"说明"}]`，或 `{"k":"v"}`；值必须是字符串。3.x 支持配置项 `type`：0 字符串、1 数字、2 布尔、3 JSON。
- 导入默认跳过已有 Key，可先预览并勾选覆盖。导入不删除文件之外的配置，也不自动发布。
- 导入逐项执行。某项失败会停止，保留已完成的操作和完整错误，显示完成数量；重试时重新读取现有配置。
- 「待发布差异」比较草稿与最新生效发布，展示新增、修改、删除及前后值。
- 回滚传入的是 **当前生效 releaseId**，3.x 指定目标时另传 `toReleaseId`。草稿不会因回滚自动发布。
- 灰度「添加规则」直接增加表单行并保留未保存内容；支持删除规则、清空规则。
- 全量合并填写发布标题和备注，可选择合并后保留分支。「放弃灰度」删除灰度分支及其规则。

## 实现与接口

- `manifest.json`：连接字段。Token、Portal 密码和 Cookie 为 secret，只发给 Go sidecar。
- `backend/main.go`：OpenAPI / Portal WebAPI 客户端、版本约束、RPC 转发；保留 HTTP 状态和完整原始响应体。登录方式互斥，Portal 请求不跟随登录重定向。
- `ui/index.html`：单文件工作台，通过 `window.dbxPlugin.invoke` 调用后端，不直接访问 Portal。
- `third_party/dbx-plugin-sdk`：内置 Go SDK；无需额外下载 DBX 仓库。

| RPC | Apollo OpenAPI |
|---|---|
| `apollo/apps.list` | `GET /openapi/v1/apps/authorized` |
| `apollo/app.get` | `GET /openapi/v1/apps?appIds=...`（兼容旧版） |
| `apollo/app.create/update/delete` | `POST /apps`、`PUT/DELETE /apps/{appId}`（3.x） |
| `apollo/organizations.list` | `GET /organizations`（3.x）；旧版使用 Portal `/organizations` |
| `apollo/users.search` | 未配置 Portal 凭据时：`GET /users?keyword=&offset=0&limit=20`（3.x）；配置后或旧版使用 Portal `/users` |
| `apollo/envclusters`、`apollo/clusters.list` | `GET /apps/{appId}/envclusters` |
| `apollo/cluster.get/create/delete` | `GET/DELETE /envs/{env}/apps/{appId}/clusters/{cluster}`、`POST .../clusters`；旧版 DELETE 见下表 |
| `apollo/namespace.create` | `POST /apps/{appId}/appnamespaces` |
| `apollo/appnamespaces.list` | `GET /appnamespaces` 或 `/apps/{appId}/appnamespaces`（3.x）；旧版见下表 |
| `apollo/namespace.attach` | `POST /namespaces`（3.x，数组请求体）；旧版见下表 |
| `apollo/namespaces.missing/repair` | `GET/POST /apps/{appId}/envs/{env}/clusters/{cluster}/missing-namespaces`（3.x） |
| `apollo/namespace.delete` | `DELETE /apps/{appId}/envs/{env}/clusters/{cluster}/namespaces/{ns}`（3.x）；旧版见下表 |
| `apollo/appnamespace.delete` | `DELETE /apps/{appId}/appnamespaces/{ns}`（3.x）；旧版见下表 |
| `apollo/namespace.get/lock` | `GET .../namespaces/{ns}`、`GET .../namespaces/{ns}/lock` |
| `apollo/item.create/update/delete` | `POST .../items`、`PUT/DELETE .../items/{key}` |
| `apollo/release.latest/publish` | `GET .../releases/latest`、`POST .../releases` |
| `apollo/releases.history` | `GET .../releases/active?page&size`（3.x） |
| `apollo/release.rollback` | `PUT /envs/{env}/releases/{currentId}/rollback?operator=&toReleaseId=`（toReleaseId 仅 3.x） |
| `apollo/branch.get/create/delete` | `GET/POST .../branches`、`DELETE .../branches/{branch}` |
| `apollo/branch.rules.get/set` | `GET/PUT .../branches/{branch}/rules` |
| `apollo/branch.release/merge` | `POST .../branches/{branch}/releases`、`POST .../merge?deleteBranch=`；均有发布请求体 |
| `apollo/branch.abandon` | `DELETE .../branches/{branch}`；与删除分支语义一致 |
| `apollo/instances.list` | `GET /envs/{env}/instances/by-namespace`（3.x） |

表中省略的统一前缀为 `/openapi/v1`。`GET .../clusters` 集群列表接口并不存在，Token 连接与 3.x 使用 `envclusters`；旧版 Portal 登录使用 `/apps/{appId}/navtree`。
`gray-del-releases` 用于发布灰度删除 Key，不能用来放弃整个灰度分支。

### 旧版替代接口

以下为 Portal WebAPI 完整路径（另加连接的「路径前缀」），不加 `/openapi/v1`。旧版 Portal 登录使用下列接口，3.x 资源管理继续使用 OpenAPI（删除应用使用 Portal `/apps/{appId}`）。不因接口报错自动切换鉴权。

| 操作 | Portal WebAPI |
|---|---|
| 当前登录用户 | `GET /user` |
| 应用列表 / 环境集群 | `GET /apps`、`GET /apps/{appId}/navtree` |
| 集群读取 / 创建 | `GET /apps/{appId}/envs/{env}/clusters/{cluster}`、`POST /apps/{appId}/envs/{env}/clusters` |
| 命名空间读取 / 创建定义 | `GET /apps/{appId}/envs/{env}/clusters/{cluster}/namespaces[/{ns}]`、`POST /apps/{appId}/appnamespaces` |
| 配置读取 / 创建 / 更新 / 删除 | `GET .../namespaces/{ns}/items`、`POST/PUT .../namespaces/{ns}/item`、`DELETE .../namespaces/{ns}/items/{itemId}` |
| 最新发布 / 发布 / 回滚 | `GET .../namespaces/{ns}/releases/active?page=0&size=1`、`POST .../namespaces/{ns}/releases`、`PUT /envs/{env}/releases/{releaseId}/rollback` |
| 灰度 | `.../namespaces/{ns}/branches` 及规则、发布、合并子路径 |
| 部门列表 | `GET /organizations` |
| 搜索用户 | `GET /users?keyword=...&offset=0&limit=20` |
| 公共定义候选 | `GET /appnamespaces/public`；另读取 default 集群私有定义候选；Portal 登录使用 WebAPI |
| 关联 / 补建 | `POST /apps/{appId}/namespaces`，请求体 `[{"env":"DEV","namespace":{"appId":"demo","clusterName":"zone-a","namespaceName":"TEST.common"}}]` |
| 关联结果回读 | `GET /apps/{appId}/envs/{env}/clusters/{cluster}/namespaces/{ns}`，核对 `baseInfo` 的应用、集群和名称 |
| 删除当前集群命名空间 | `DELETE /apps/{appId}/envs/{env}/clusters/{cluster}/namespaces/{ns}` |
| 删除应用级定义 | `DELETE /apps/{appId}/appnamespaces/{ns}` |
| 删除集群 | `DELETE /apps/{appId}/envs/{env}/clusters/{cluster}` |

上述替代接口和鉴权已核对官方 1.9.0、2.0.0 源码。更早的 1.x、定制 SSO 或代理鉴权需要在目标环境确认。401、登录重定向、403、服务端约束错误和完整响应体均保留展示。

旧版依据：[NamespaceController](https://github.com/apolloconfig/apollo/blob/v1.9.0/apollo-portal/src/main/java/com/ctrip/framework/apollo/portal/controller/NamespaceController.java)、[ClusterController](https://github.com/apolloconfig/apollo/blob/v1.9.0/apollo-portal/src/main/java/com/ctrip/framework/apollo/portal/controller/ClusterController.java)、[AuthConfiguration](https://github.com/apolloconfig/apollo/blob/v1.9.0/apollo-portal/src/main/java/com/ctrip/framework/apollo/portal/spi/configuration/AuthConfiguration.java)。

接口依据：[Apollo 官方开放平台文档](https://github.com/apolloconfig/apollo/wiki/Apollo%E5%BC%80%E6%94%BE%E5%B9%B3%E5%8F%B0)、
[Apollo 1.9 源码](https://github.com/apolloconfig/apollo/tree/v1.9.0/apollo-portal/src/main/java/com/ctrip/framework/apollo/openapi/v1/controller)、
[Apollo 2.0 源码](https://github.com/apolloconfig/apollo/tree/v2.0.0/apollo-portal/src/main/java/com/ctrip/framework/apollo/openapi/v1/controller)、
[Apollo 3.0 源码](https://github.com/apolloconfig/apollo/tree/v3.0.0/apollo-portal/src/main/java/com/ctrip/framework/apollo/openapi/v1/controller)、
[官方 OpenAPI 规范](https://github.com/apolloconfig/apollo-openapi/blob/main/apollo-openapi.yaml)。

## 验证范围

首发版本为 **1.0.0**，验证结果汇总如下：

- Go 编译、静态检查、前端语法、连接字段联动和打包校验通过。
- 模拟交互覆盖 Apollo 1.x / 2.x 与 3.x 的配置、集群、命名空间、发布和灰度流程，以及失败重试、响应过期隔离和窄窗口布局；模拟结果不能替代目标环境鉴权验证。
- 开发阶段已在 DBX 0.6.47 和真实 Apollo 3.x 上验证用户访问 Token、用户与部门下拉、临时应用 / 集群 / 命名空间的创建、关联及删除，临时资源已清理。
- 真实 2340 个应用的连接已验证下拉多项、滚动、AppId / 中文名称搜索、清空和切换；未测量渲染耗时。
- 原生另存为、保存路径、取消导出及导入本地校验提示已实测；只读验证未修改现有 Apollo 配置。
- 1.0.0 已实际安装 DBX，使用 `@me` 在独立 Spring Boot 双副本上验证基线、单副本 IP 灰度、合并全量及回滚；配置热更新生效，两个副本均无重启。
- 真实 1.x / 2.x Portal 管理写入、LDAP 和 SSO 会话尚未验收。其他平台的构建结果与本机运行验收分别记录。

详见 [验证记录](docs/VALIDATION.md)。
