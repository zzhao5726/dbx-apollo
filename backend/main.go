package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	dbxpluginsdk "github.com/t8y2/dbx/plugins/sdk/go/dbx-plugin-sdk"
)

// apolloClient talks to one Apollo Portal via OpenAPI v1 and legacy Portal WebAPI.
type apolloClient struct {
	baseURL      string
	basePath     string // servlet context path, "" means "/"
	token        string
	userToken    bool
	env          string
	defaultAppID string
	operator     string
	apiLevel     string // modern | legacy (derived from server_version)
	http         *http.Client
	portalUser   string
	portalPass   string
	portalCookie string
	portalHTTP   *http.Client
}

func (c *apolloClient) do(method, path string, query map[string]string, body any) (json.RawMessage, *dbxpluginsdk.PluginError) {
	if c.portalConfigured() && c.apiLevel == "legacy" {
		return c.legacyPortalRequest(method, path, query, body)
	}
	return c.request(method, path, query, body, c.portalConfigured())
}

func (c *apolloClient) portalConfigured() bool {
	return c.portalCookie != "" || (c.portalUser != "" && c.portalPass != "")
}

func (c *apolloClient) portalDo(method, path string, body any) (json.RawMessage, *dbxpluginsdk.PluginError) {
	if !c.portalConfigured() {
		return nil, dbxpluginsdk.NewError(-32602, "此操作需要 Portal 用户鉴权，请在连接中选择账号密码或 SSO Cookie 登录；OpenAPI Token 不能用于此接口")
	}
	return c.request(method, path, nil, body, true)
}

func (c *apolloClient) authType() string {
	if c.userToken {
		return "USER_TOKEN"
	}
	if c.portalConfigured() {
		return "USER"
	}
	return "CONSUMER"
}

func (c *apolloClient) authInfo() (json.RawMessage, *dbxpluginsdk.PluginError) {
	if c.userToken {
		return c.do("GET", "/openapi/v1/user-tokens/current", nil, nil)
	}
	if c.portalConfigured() {
		return c.portalDo("GET", "/user", nil)
	}
	return json.RawMessage(`{"authType":"CONSUMER"}`), nil
}

func (c *apolloClient) currentAccount() (string, *dbxpluginsdk.PluginError) {
	raw, perr := c.authInfo()
	if perr != nil {
		return "", perr
	}
	var user struct {
		UserID string `json:"userId"`
	}
	err := json.Unmarshal(raw, &user)
	if err != nil || user.UserID == "" {
		message := "Apollo 未返回用户账号"
		if err != nil {
			message += "：" + err.Error()
		}
		perr := dbxpluginsdk.NewError(-32001, message)
		perr.Data = map[string]any{"body": string(raw)}
		return "", perr
	}
	return user.UserID, nil
}

// 路径分段由调用方转义；OpenAPI 和 Portal 用户鉴权互不混用。
func (c *apolloClient) request(method, path string, query map[string]string, body any, portal bool) (json.RawMessage, *dbxpluginsdk.PluginError) {
	u := c.baseURL + c.basePath + path
	if len(query) > 0 {
		q := url.Values{}
		for k, v := range query {
			if v != "" {
				q.Set(k, v)
			}
		}
		if enc := q.Encode(); enc != "" {
			u += "?" + enc
		}
	}
	var rdr io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, dbxpluginsdk.NewError(-32603, "encode request: "+err.Error())
		}
		rdr = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, u, rdr)
	if err != nil {
		return nil, dbxpluginsdk.NewError(-32603, "build request: "+err.Error())
	}
	client := c.http
	if portal {
		client = c.portalHTTP
		if c.portalCookie != "" {
			req.Header.Set("Cookie", c.portalCookie)
		} else {
			req.SetBasicAuth(c.portalUser, c.portalPass)
		}
	} else {
		req.Header.Set("Authorization", c.token)
	}
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, dbxpluginsdk.NewError(-32000, "apollo request failed: "+err.Error())
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		perr := dbxpluginsdk.NewError(-32000, "read apollo response: "+err.Error())
		perr.Data = map[string]any{"status": resp.StatusCode, "body": string(raw)}
		return nil, perr
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		perr := dbxpluginsdk.NewError(-32001, fmt.Sprintf("apollo %s %s (HTTP %d): %s", method, path, resp.StatusCode, string(raw)))
		if portal && (resp.StatusCode == http.StatusUnauthorized || (resp.StatusCode >= 300 && resp.StatusCode < 400)) {
			perr.Message = "Portal 未登录或登录已失效，请核对账号密码或更新登录 Cookie；" + perr.Message
		}
		perr.Data = map[string]any{"status": resp.StatusCode, "body": string(raw), "location": resp.Header.Get("Location")}
		return nil, perr
	}
	if len(bytes.TrimSpace(raw)) == 0 {
		return json.RawMessage("null"), nil
	}
	if !json.Valid(raw) {
		perr := dbxpluginsdk.NewError(-32001, "apollo returned invalid JSON")
		perr.Data = map[string]any{"status": resp.StatusCode, "body": string(raw)}
		return nil, perr
	}
	return json.RawMessage(raw), nil
}

func hasStatus(err *dbxpluginsdk.PluginError, status int) bool {
	if err == nil {
		return false
	}
	data, _ := err.Data.(map[string]any)
	return data["status"] == status
}

func boolVal(values map[string]any, key string, fallback bool) bool {
	if v, ok := values[key].(bool); ok {
		return v
	}
	return fallback
}

func releaseBody(values map[string]any, operator string) map[string]any {
	title := strVal(values, "title")
	if title == "" {
		title = "dbx publish " + time.Now().Format("20060102-150405")
	}
	return map[string]any{
		"releaseTitle": title, "releaseComment": strVal(values, "comment"),
		"releasedBy": operator, "isEmergencyPublish": boolVal(values, "isEmergencyPublish", false),
	}
}

func itemBody(values map[string]any, operator string) (map[string]any, *dbxpluginsdk.PluginError) {
	body := map[string]any{
		"key": strVal(values, "key"), "value": strVal(values, "value"),
		"comment": strVal(values, "comment"), "dataChangeCreatedBy": operator,
		"dataChangeLastModifiedBy": operator,
	}
	if itemType, ok := values["type"]; ok && itemType != nil {
		n, valid := itemType.(float64)
		if !valid || n < 0 || n > 3 || n != float64(int(n)) {
			return nil, dbxpluginsdk.NewError(-32602, "配置类型必须为 0（字符串）、1（数字）、2（布尔）或 3（JSON）")
		}
		body["type"] = n
	}
	return body, nil
}

func esc(s string) string { return url.PathEscape(s) }

func strVal(m map[string]any, keys ...string) string {
	for _, k := range keys {
		v, ok := m[k]
		if !ok || v == nil {
			continue
		}
		switch t := v.(type) {
		case string:
			if t != "" {
				return t
			}
		case float64:
			return strconv.FormatFloat(t, 'f', -1, 64)
		case bool:
			return strconv.FormatBool(t)
		}
	}
	return ""
}

// buildClient hydrates an apolloClient from a DBX connection map.
// Common fields (host/port) live at top level; config-bound fields in
// external_config; tokens, passwords and cookies in connection_secrets.
func buildClient(conn map[string]any) (*apolloClient, *dbxpluginsdk.PluginError) {
	ext, _ := conn["external_config"].(map[string]any)
	secrets, _ := conn["connection_secrets"].(map[string]any)
	get := func(keys ...string) string {
		if v := strVal(conn, keys...); v != "" {
			return v
		}
		return strVal(ext, keys...)
	}
	host := get("host")
	if host == "" {
		host = "localhost"
	}
	port := get("port")
	if port == "" {
		port = "8070"
	}
	scheme := "http"
	if get("tls") == "true" {
		scheme = "https"
	}
	token := strings.TrimSpace(strVal(secrets, "token"))
	env := strings.ToUpper(get("env"))
	if env == "" {
		env = "DEV"
	}
	operator := get("operator")
	portalUser := strings.TrimSpace(get("portal_username"))
	portalPass := strVal(secrets, "portal_password")
	portalCookie := strings.TrimSpace(strVal(secrets, "portal_cookie"))
	auth := get("portal_auth")
	if auth == "" { // 历史连接默认使用原来必填的 Token，与表单默认值一致。
		auth = "none"
	}
	switch auth {
	case "none":
		portalUser, portalPass, portalCookie = "", "", ""
		if token == "" {
			return nil, dbxpluginsdk.NewError(-32602, "请填写 Token")
		}
	case "basic":
		token, portalCookie = "", ""
		if portalUser == "" || portalPass == "" {
			return nil, dbxpluginsdk.NewError(-32602, "请填写 Portal 登录账号和密码")
		}
	case "cookie":
		token, portalUser, portalPass = "", "", ""
		if portalCookie == "" {
			return nil, dbxpluginsdk.NewError(-32602, "请填写 Portal 登录 Cookie")
		}
	default:
		return nil, dbxpluginsdk.NewError(-32602, "不支持的登录方式")
	}
	// server_version drives API compatibility: only 3.x added new OpenAPI
	// endpoints (releases/active, /envs). 1.x and 2.x share the same Portal
	// OpenAPI surface, so they use the classic call patterns. Default to
	// legacy when unset so old saved connections keep working.
	apiLevel := "legacy"
	if get("server_version") == "3.x" {
		apiLevel = "modern"
	}
	userToken := strings.HasPrefix(strings.TrimPrefix(token, "Bearer "), "apollo_pat_")
	if userToken {
		if apiLevel != "modern" {
			return nil, dbxpluginsdk.NewError(-32602, "用户访问 Token 需要 Apollo 3.x，请选择对应接口兼容度")
		}
		token = "Bearer " + strings.TrimPrefix(token, "Bearer ")
	}
	// Normalize the servlet context path: "" or "/" means root.
	basePath := strings.TrimSpace(get("context_path"))
	if basePath == "" || basePath == "/" {
		basePath = ""
	} else {
		if !strings.HasPrefix(basePath, "/") {
			basePath = "/" + basePath
		}
		basePath = strings.TrimSuffix(basePath, "/")
	}
	return &apolloClient{
		baseURL:      fmt.Sprintf("%s://%s:%s", scheme, host, port),
		basePath:     basePath,
		token:        token,
		userToken:    userToken,
		env:          env,
		defaultAppID: get("app_id", "appId"),
		operator:     operator,
		apiLevel:     apiLevel,
		http:         &http.Client{Timeout: 30 * time.Second},
		portalUser:   portalUser,
		portalPass:   portalPass,
		portalCookie: portalCookie,
		portalHTTP: &http.Client{Timeout: 30 * time.Second, CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		}},
	}, nil
}

// 旧版没有列出 AppNamespace 定义的统一接口，合并公共定义和 default 中的已有实例。
func (c *apolloClient) legacyAppNamespaces(env, appID string, all bool) (any, *dbxpluginsdk.PluginError) {
	raw, perr := c.portalDo("GET", "/appnamespaces/public", nil)
	if perr != nil {
		return nil, perr
	}
	var public []map[string]any
	if err := json.Unmarshal(raw, &public); err != nil {
		perr := dbxpluginsdk.NewError(-32001, "parse public namespaces: "+err.Error())
		perr.Data = map[string]any{"body": string(raw)}
		return nil, perr
	}
	out := []map[string]any{}
	seen := map[string]bool{}
	for _, ns := range public {
		name := strVal(ns, "name")
		if !all && strVal(ns, "appId") != appID {
			continue
		}
		ns["isPublic"] = true
		out = append(out, ns)
		seen[name] = true
	}
	raw, perr = c.do("GET", "/openapi/v1/envs/"+esc(env)+"/apps/"+esc(appID)+"/clusters/default/namespaces", nil, nil)
	if perr != nil {
		return nil, perr
	}
	var namespaces []map[string]any
	if err := json.Unmarshal(raw, &namespaces); err != nil {
		perr := dbxpluginsdk.NewError(-32001, "parse default namespaces: "+err.Error())
		perr.Data = map[string]any{"body": string(raw)}
		return nil, perr
	}
	for _, ns := range namespaces {
		name := strVal(ns, "namespaceName")
		if name == "" || seen[name] || boolVal(ns, "isPublic", false) {
			continue
		}
		out = append(out, map[string]any{"appId": appID, "name": name, "format": ns["format"], "isPublic": false, "comment": ns["comment"]})
		seen[name] = true
	}
	return out, nil
}

func (c *apolloClient) legacyAttach(env, appID, cluster, namespace string) (any, *dbxpluginsdk.PluginError) {
	path := "/apps/" + esc(appID) + "/envs/" + esc(env) + "/clusters/" + esc(cluster) + "/namespaces/" + esc(namespace)
	raw, perr := c.portalDo("POST", "/apps/"+esc(appID)+"/namespaces", []map[string]any{
		{"env": env, "namespace": map[string]string{"appId": appID, "clusterName": cluster, "namespaceName": namespace}},
	})
	if perr != nil {
		return nil, perr
	}
	// 旧版 Controller 会捕获并记录创建异常，HTTP 200 之后仍需回读目标实例。
	observed, perr := c.portalDo("GET", path, nil)
	if perr != nil {
		err := dbxpluginsdk.NewError(-32001, "Portal 已返回关联请求，但未能确认目标命名空间存在："+perr.Message)
		err.Data = map[string]any{"operationResponse": string(raw), "verificationError": perr}
		return nil, err
	}
	var dto struct {
		BaseInfo struct {
			AppID     string `json:"appId"`
			Cluster   string `json:"clusterName"`
			Namespace string `json:"namespaceName"`
		} `json:"baseInfo"`
	}
	parseErr := json.Unmarshal(observed, &dto)
	if parseErr != nil || dto.BaseInfo.AppID != appID || dto.BaseInfo.Cluster != cluster || dto.BaseInfo.Namespace != namespace {
		err := dbxpluginsdk.NewError(-32001, "Portal 返回后未确认关联成功，请检查服务端创建错误和命名空间定义是否已同步到目标环境")
		err.Data = map[string]any{"operationResponse": string(raw), "verificationResponse": string(observed)}
		if parseErr != nil {
			err.Message += ": " + parseErr.Error()
		}
		return nil, err
	}
	return map[string]any{"success": true, "verified": true}, nil
}

type plugin struct {
	mutex   sync.Mutex
	clients map[string]*apolloClient
}

func (p *plugin) clientFor(values map[string]any) (*apolloClient, *dbxpluginsdk.PluginError) {
	id, _ := values["connectionId"].(string)
	if id == "" {
		return nil, dbxpluginsdk.NewError(-32602, "missing connectionId")
	}
	p.mutex.Lock()
	defer p.mutex.Unlock()
	c, ok := p.clients[id]
	if !ok {
		return nil, dbxpluginsdk.NewError(-32002, "connection not open, reconnect first")
	}
	return c, nil
}

// resolve fills env/appId/operator defaults from the client.
func resolve(c *apolloClient, values map[string]any) (env, appID, operator string) {
	env = strVal(values, "env")
	if env == "" {
		env = c.env
	}
	appID = strVal(values, "appId", "app_id")
	if appID == "" {
		appID = c.defaultAppID
	}
	operator = strVal(values, "operator")
	if operator == "" {
		operator = c.operator
	}
	return strings.ToUpper(env), appID, operator
}

// namespaceItems lists the config items of a namespace by reading the
// namespace DTO (its `items` field). The dedicated GET .../items list
// endpoint is missing from the Portal OpenAPI before 3.0, so this works
// on every version. cluster may be a gray branch name.
func (c *apolloClient) namespaceItems(env, appID, cluster, namespace string) (json.RawMessage, *dbxpluginsdk.PluginError) {
	path := "/openapi/v1/envs/" + esc(env) + "/apps/" + esc(appID) +
		"/clusters/" + esc(cluster) + "/namespaces/" + esc(namespace)
	raw, perr := c.do("GET", path, map[string]string{"fillItemDetail": "true"}, nil)
	if perr != nil {
		return nil, perr
	}
	var dto struct {
		Items []json.RawMessage `json:"items"`
	}
	if err := json.Unmarshal(raw, &dto); err != nil {
		return nil, dbxpluginsdk.NewError(-32001, "parse namespace items: "+err.Error())
	}
	if dto.Items == nil {
		return json.RawMessage("[]"), nil
	}
	out, err := json.Marshal(dto.Items)
	if err != nil {
		return nil, dbxpluginsdk.NewError(-32603, err.Error())
	}
	return json.RawMessage(out), nil
}

// pick runs newCall or oldCall depending on the Apollo server version:
// only 3.x has the newer OpenAPI endpoints (releases/active, /envs).
func (c *apolloClient) pick(
	newCall func() (json.RawMessage, *dbxpluginsdk.PluginError),
	oldCall func() (json.RawMessage, *dbxpluginsdk.PluginError),
) (json.RawMessage, *dbxpluginsdk.PluginError) {
	if c.apiLevel == "legacy" {
		return oldCall()
	}
	return newCall()
}

func (p *plugin) Handle(
	_ dbxpluginsdk.RequestContext,
	method string,
	params json.RawMessage,
	_ *dbxpluginsdk.Emitter,
) (any, *dbxpluginsdk.PluginError) {
	var values map[string]any
	if err := json.Unmarshal(params, &values); err != nil {
		return nil, dbxpluginsdk.NewError(-32602, "Invalid request parameters")
	}
	switch method {
	case "connection/test":
		conn, _ := values["connection"].(map[string]any)
		c, perr := buildClient(conn)
		if perr != nil {
			return nil, perr
		}
		if c.portalConfigured() || c.userToken {
			if _, perr := c.currentAccount(); perr != nil {
				return nil, perr
			}
		}
		raw, perr := c.do("GET", "/openapi/v1/apps/authorized", nil, nil)
		if perr != nil {
			return nil, perr
		}
		var apps []any
		_ = json.Unmarshal(raw, &apps)
		return map[string]any{
			"success": true,
			"message": fmt.Sprintf("连接成功，可访问 %d 个应用", len(apps)),
		}, nil

	case "connection/connect":
		conn, _ := values["connection"].(map[string]any)
		id, _ := conn["id"].(string)
		if id == "" {
			return nil, dbxpluginsdk.NewError(-32602, "Missing connection id")
		}
		c, perr := buildClient(conn)
		if perr != nil {
			return nil, perr
		}
		if c.portalConfigured() || c.userToken {
			c.operator, perr = c.currentAccount()
			if perr != nil {
				return nil, perr
			}
		}
		if _, perr := c.do("GET", "/openapi/v1/apps/authorized", nil, nil); perr != nil {
			return nil, perr
		}
		p.mutex.Lock()
		p.clients[id] = c
		p.mutex.Unlock()
		return map[string]any{"success": true}, nil

	case "connection/disconnect":
		conn, _ := values["connection"].(map[string]any)
		id, _ := conn["id"].(string)
		p.mutex.Lock()
		delete(p.clients, id)
		p.mutex.Unlock()
		return map[string]any{"success": true}, nil

	case "dbx-apollo/ping":
		return map[string]any{"ok": true, "plugin": "io.dbx.apollo"}, nil
	}

	// ---- Apollo domain RPCs ----
	c, perr := p.clientFor(values)
	if perr != nil {
		return nil, perr
	}
	env, appID, operator := resolve(c, values)
	cluster := strVal(values, "cluster")
	if cluster == "" {
		cluster = "default"
	}
	namespace := strVal(values, "namespace")
	nsBase := func() string {
		return "/openapi/v1/envs/" + esc(env) + "/apps/" + esc(appID) +
			"/clusters/" + esc(cluster) + "/namespaces/" + esc(namespace)
	}
	clusterBase := "/openapi/v1/envs/" + esc(env) + "/apps/" + esc(appID) + "/clusters"
	// 此处仅限制尚未接入旧版替代接口的功能。
	switch method {
	case "apollo/app.create", "apollo/app.update", "apollo/app.delete",
		"apollo/namespaces.missing", "apollo/namespaces.repair", "apollo/instances.list":
		if c.apiLevel != "modern" {
			return nil, dbxpluginsdk.NewError(-32602, "此操作需要 Apollo 3.x，请在 Portal 中操作")
		}
	}
	needsOperator := false
	switch method {
	case "apollo/app.create", "apollo/app.update", "apollo/cluster.create", "apollo/namespace.create",
		"apollo/namespaces.repair", "apollo/item.create", "apollo/item.update", "apollo/item.delete",
		"apollo/release.publish", "apollo/release.rollback", "apollo/branch.create", "apollo/branch.delete",
		"apollo/branch.rules.set", "apollo/branch.release", "apollo/branch.merge", "apollo/branch.abandon":
		needsOperator = true
	case "apollo/app.delete":
		needsOperator = !c.portalConfigured()
	case "apollo/cluster.delete", "apollo/namespace.attach", "apollo/namespace.delete", "apollo/appnamespace.delete":
		needsOperator = c.apiLevel == "modern"
	}
	if needsOperator && operator == "" {
		return nil, dbxpluginsdk.NewError(-32602, "请在「更多选项」填写已有 Apollo 变更操作人；用户访问 Token、账号密码或 Cookie 登录后可自动获取账号")
	}
	if method != "apollo/connection.info" && method != "apollo/auth.info" && method != "apollo/portal.user" && method != "apollo/organizations.list" && method != "apollo/users.search" && method != "apollo/envs.list" && method != "apollo/apps.list" && method != "apollo/app.create" && appID == "" {
		return nil, dbxpluginsdk.NewError(-32602, "missing appId")
	}

	switch method {
	case "apollo/connection.info":
		return map[string]any{
			"baseUrl": c.baseURL, "basePath": c.basePath, "env": c.env,
			"defaultAppId": c.defaultAppID, "operator": c.operator,
			"apiLevel":         c.apiLevel,
			"portalConfigured": c.portalConfigured(),
			"authType":         c.authType(),
		}, nil

	case "apollo/auth.info":
		return c.authInfo()

	case "apollo/portal.user":
		return c.portalDo("GET", "/user", nil)

	case "apollo/organizations.list":
		if c.apiLevel == "legacy" {
			return c.portalDo("GET", "/organizations", nil)
		}
		return c.do("GET", "/openapi/v1/organizations", nil, nil)

	case "apollo/users.search":
		keyword := strings.TrimSpace(strVal(values, "keyword"))
		// 空关键词用于加载下拉候选，也必须发送 keyword=。
		query := url.Values{"keyword": {keyword}, "offset": {"0"}, "limit": {"20"}}
		if c.apiLevel == "legacy" || c.portalConfigured() {
			return c.portalDo("GET", "/users?"+query.Encode(), nil)
		}
		return c.do("GET", "/openapi/v1/users?"+query.Encode(), nil, nil)

	case "apollo/envs.list":
		return c.pick(
			func() (json.RawMessage, *dbxpluginsdk.PluginError) {
				return c.do("GET", "/openapi/v1/envs", nil, nil)
			},
			func() (json.RawMessage, *dbxpluginsdk.PluginError) {
				raw, err := json.Marshal([]string{c.env})
				if err != nil {
					return nil, dbxpluginsdk.NewError(-32603, err.Error())
				}
				return json.RawMessage(raw), nil
			},
		)

	case "apollo/apps.list":
		return c.do("GET", "/openapi/v1/apps/authorized", nil, nil)

	case "apollo/app.create", "apollo/app.update":
		app, ok := values["app"].(map[string]any)
		if !ok || strVal(app, "appId") == "" || strVal(app, "name") == "" || strVal(app, "ownerName") == "" || strVal(app, "orgId") == "" || strVal(app, "orgName") == "" {
			return nil, dbxpluginsdk.NewError(-32602, "appId、name、ownerName、orgId、orgName 不能为空")
		}
		if method == "apollo/app.update" {
			if strVal(app, "appId") != appID {
				return nil, dbxpluginsdk.NewError(-32602, "appId in path and payload must match")
			}
			return c.do("PUT", "/openapi/v1/apps/"+esc(appID), map[string]string{"operator": operator}, app)
		}
		app["dataChangeCreatedBy"] = operator
		return c.do("POST", "/openapi/v1/apps", nil, map[string]any{
			"app": app, "admins": values["admins"], "assignAppRoleToSelf": boolVal(values, "assignAppRoleToSelf", true),
		})

	case "apollo/app.delete":
		if c.portalConfigured() {
			return c.portalDo("DELETE", "/apps/"+esc(appID), nil)
		}
		raw, perr := c.do("DELETE", "/openapi/v1/apps/"+esc(appID), map[string]string{"operator": operator}, nil)
		if hasStatus(perr, http.StatusForbidden) {
			message := "删除应用需要超级管理员身份。第三方应用 Token 不支持此操作；可使用超级管理员的用户访问 Token（包含 system:admin 范围）、账号密码或 Cookie。"
			if c.userToken {
				message = "删除应用需要 Token 所属账号是超级管理员，且 Token 包含 system:admin 并允许目标应用；创建应用、管理用户范围不能代替。"
			}
			perr.Message = message + "原始错误：" + perr.Message
		}
		return raw, perr

	case "apollo/app.get":
		raw, perr := c.do("GET", "/openapi/v1/apps", map[string]string{"appIds": appID}, nil)
		if perr != nil {
			return nil, perr
		}
		var apps []json.RawMessage
		if err := json.Unmarshal(raw, &apps); err != nil {
			return nil, dbxpluginsdk.NewError(-32001, "parse apps: "+err.Error())
		}
		if len(apps) == 0 {
			return nil, dbxpluginsdk.NewError(-32001, "app not found: "+appID)
		}
		return apps[0], nil

	case "apollo/envclusters":
		if appID == "" {
			return nil, dbxpluginsdk.NewError(-32602, "missing appId")
		}
		return c.do("GET", "/openapi/v1/apps/"+esc(appID)+"/envclusters", nil, nil)

	case "apollo/cluster.get":
		return c.do("GET", clusterBase+"/"+esc(cluster), nil, nil)

	case "apollo/cluster.create":
		name := strings.TrimSpace(strVal(values, "name"))
		if name == "" {
			return nil, dbxpluginsdk.NewError(-32602, "missing cluster name")
		}
		return c.do("POST", clusterBase, nil, map[string]any{
			"name": name, "appId": appID, "dataChangeCreatedBy": operator,
		})

	case "apollo/cluster.delete":
		if cluster == "default" {
			return nil, dbxpluginsdk.NewError(-32602, "不能删除 default 集群")
		}
		if c.apiLevel == "legacy" {
			return c.portalDo("DELETE", "/apps/"+esc(appID)+"/envs/"+esc(env)+"/clusters/"+esc(cluster), nil)
		}
		return c.do("DELETE", clusterBase+"/"+esc(cluster), map[string]string{"operator": operator}, nil)

	case "apollo/namespace.create":
		name := strings.TrimSpace(strVal(values, "name"))
		format := strVal(values, "format")
		if format == "" {
			format = "properties"
		}
		switch format {
		case "properties", "xml", "json", "yaml", "yml", "txt":
		default:
			return nil, dbxpluginsdk.NewError(-32602, "unsupported namespace format: "+format)
		}
		if name == "" {
			return nil, dbxpluginsdk.NewError(-32602, "missing namespace name")
		}
		// Apollo appends the file extension and optional organization prefix.
		if format != "properties" {
			name = strings.TrimSuffix(name, "."+format)
		}
		if name == "" {
			return nil, dbxpluginsdk.NewError(-32602, "missing namespace name")
		}
		return c.do("POST", "/openapi/v1/apps/"+esc(appID)+"/appnamespaces", nil, map[string]any{
			"name": name, "appId": appID, "format": format,
			"isPublic":              boolVal(values, "isPublic", false),
			"appendNamespacePrefix": boolVal(values, "appendNamespacePrefix", true),
			"comment":               strVal(values, "comment"), "dataChangeCreatedBy": operator,
		})

	case "apollo/appnamespaces.list":
		if c.apiLevel == "legacy" {
			return c.legacyAppNamespaces(env, appID, boolVal(values, "all", false))
		}
		path := "/openapi/v1/apps/" + esc(appID) + "/appnamespaces"
		if boolVal(values, "all", false) {
			path = "/openapi/v1/appnamespaces"
		}
		return c.do("GET", path, nil, nil)

	case "apollo/namespace.attach":
		if namespace == "" {
			return nil, dbxpluginsdk.NewError(-32602, "missing namespace")
		}
		if c.apiLevel == "legacy" {
			return c.legacyAttach(env, appID, cluster, namespace)
		}
		return c.do("POST", "/openapi/v1/namespaces", map[string]string{"operator": operator}, []map[string]string{
			{"appId": appID, "env": env, "clusterName": cluster, "appNamespaceName": namespace},
		})

	case "apollo/namespace.delete", "apollo/appnamespace.delete":
		if namespace == "" || namespace == "application" {
			return nil, dbxpluginsdk.NewError(-32602, "请选择非 application 命名空间")
		}
		if c.apiLevel == "legacy" {
			path := "/apps/" + esc(appID) + "/envs/" + esc(env) + "/clusters/" + esc(cluster) + "/namespaces/" + esc(namespace)
			if method == "apollo/appnamespace.delete" {
				path = "/apps/" + esc(appID) + "/appnamespaces/" + esc(namespace)
			}
			return c.portalDo("DELETE", path, nil)
		}
		path := "/openapi/v1/apps/" + esc(appID) + "/envs/" + esc(env) + "/clusters/" + esc(cluster) + "/namespaces/" + esc(namespace)
		if method == "apollo/appnamespace.delete" {
			path = "/openapi/v1/apps/" + esc(appID) + "/appnamespaces/" + esc(namespace)
		}
		return c.do("DELETE", path, map[string]string{"operator": operator}, nil)

	case "apollo/namespaces.missing", "apollo/namespaces.repair":
		verb := "GET"
		if method == "apollo/namespaces.repair" {
			verb = "POST"
		}
		return c.do(verb, "/openapi/v1/apps/"+esc(appID)+"/envs/"+esc(env)+"/clusters/"+esc(cluster)+"/missing-namespaces", map[string]string{"operator": operator}, nil)

	case "apollo/clusters.list":
		if appID == "" {
			return nil, dbxpluginsdk.NewError(-32602, "missing appId")
		}
		// GET .../apps/{appId}/clusters (list) never existed in the Portal
		// OpenAPI on any version, so always use the classic envclusters call.
		raw, perr := c.do("GET", "/openapi/v1/apps/"+esc(appID)+"/envclusters", nil, nil)
		if perr != nil {
			return nil, perr
		}
		var envClusters []struct {
			Env      string   `json:"env"`
			Clusters []string `json:"clusters"`
		}
		if err := json.Unmarshal(raw, &envClusters); err != nil {
			return nil, dbxpluginsdk.NewError(-32001, "parse envclusters: "+err.Error())
		}
		out := []map[string]any{}
		for _, ec := range envClusters {
			if !strings.EqualFold(ec.Env, env) {
				continue
			}
			for _, name := range ec.Clusters {
				out = append(out, map[string]any{"name": name})
			}
		}
		raw2, err := json.Marshal(out)
		if err != nil {
			return nil, dbxpluginsdk.NewError(-32603, err.Error())
		}
		return json.RawMessage(raw2), nil

	case "apollo/namespaces.list":
		if appID == "" {
			return nil, dbxpluginsdk.NewError(-32602, "missing appId")
		}
		return c.do("GET", "/openapi/v1/envs/"+esc(env)+"/apps/"+esc(appID)+
			"/clusters/"+esc(cluster)+"/namespaces", nil, nil)

	case "apollo/namespace.get":
		if namespace == "" {
			return nil, dbxpluginsdk.NewError(-32602, "missing namespace")
		}
		return c.do("GET", nsBase(), map[string]string{"fillItemDetail": "true"}, nil)

	case "apollo/namespace.lock":
		if namespace == "" {
			return nil, dbxpluginsdk.NewError(-32602, "missing namespace")
		}
		return c.do("GET", nsBase()+"/lock", nil, nil)

	case "apollo/instances.list":
		if namespace == "" {
			return nil, dbxpluginsdk.NewError(-32602, "missing namespace")
		}
		page, size := strVal(values, "page"), strVal(values, "size")
		if page == "" {
			page = "0"
		}
		if size == "" {
			size = "20"
		}
		return c.do("GET", "/openapi/v1/envs/"+esc(env)+"/instances/by-namespace", map[string]string{
			"appId": appID, "clusterName": cluster, "namespaceName": namespace,
			"instanceAppId": strVal(values, "instanceAppId"), "page": page, "size": size,
		}, nil)

	case "apollo/items.list":
		if namespace == "" {
			return nil, dbxpluginsdk.NewError(-32602, "missing namespace")
		}
		// GET .../namespaces/{ns}/items (list) is not in the Portal OpenAPI
		// before 3.0, so read items off the namespace DTO instead. Works on
		// every version, including gray branches (cluster = branch name).
		return c.namespaceItems(env, appID, cluster, namespace)

	case "apollo/item.create":
		key := strVal(values, "key")
		if key == "" || namespace == "" {
			return nil, dbxpluginsdk.NewError(-32602, "missing key or namespace")
		}
		body, perr := itemBody(values, operator)
		if perr != nil {
			return nil, perr
		}
		return c.do("POST", nsBase()+"/items", nil, body)

	case "apollo/item.update":
		key := strVal(values, "key")
		if key == "" || namespace == "" {
			return nil, dbxpluginsdk.NewError(-32602, "missing key or namespace")
		}
		body, perr := itemBody(values, operator)
		if perr != nil {
			return nil, perr
		}
		return c.do("PUT", nsBase()+"/items/"+esc(key), map[string]string{
			"createIfNotExists": strconv.FormatBool(boolVal(values, "createIfNotExists", false)),
		}, body)

	case "apollo/item.delete":
		key := strVal(values, "key")
		if key == "" || namespace == "" {
			return nil, dbxpluginsdk.NewError(-32602, "missing key or namespace")
		}
		return c.do("DELETE", nsBase()+"/items/"+esc(key),
			map[string]string{"operator": operator}, nil)

	case "apollo/release.publish":
		if namespace == "" {
			return nil, dbxpluginsdk.NewError(-32602, "missing namespace")
		}
		return c.do("POST", nsBase()+"/releases",
			map[string]string{"operator": operator}, releaseBody(values, operator))

	case "apollo/releases.history":
		if namespace == "" {
			return nil, dbxpluginsdk.NewError(-32602, "missing namespace")
		}
		page := strVal(values, "page")
		if page == "" {
			page = "0"
		}
		size := strVal(values, "size")
		if size == "" {
			size = "20"
		}
		return c.pick(
			func() (json.RawMessage, *dbxpluginsdk.PluginError) {
				// releases/active only exists on 3.x.
				return c.do("GET", nsBase()+"/releases/active",
					map[string]string{"page": page, "size": size}, nil)
			},
			func() (json.RawMessage, *dbxpluginsdk.PluginError) {
				return nil, dbxpluginsdk.NewError(-32001,
					"发布历史接口需要 Apollo 3.x，请在 Portal 页面查看发布历史")
			},
		)

	case "apollo/release.latest":
		if namespace == "" {
			return nil, dbxpluginsdk.NewError(-32602, "missing namespace")
		}
		raw, perr := c.do("GET", nsBase()+"/releases/latest", nil, nil)
		if hasStatus(perr, http.StatusNotFound) {
			return nil, nil
		}
		return raw, perr

	case "apollo/release.rollback":
		releaseID := strVal(values, "releaseId")
		if releaseID == "" {
			return nil, dbxpluginsdk.NewError(-32602, "missing releaseId")
		}
		q := map[string]string{"operator": operator}
		if v := strVal(values, "toReleaseId"); v != "" {
			if c.apiLevel != "modern" {
				return nil, dbxpluginsdk.NewError(-32602, "回滚到指定版本需要 Apollo 3.x")
			}
			if v == releaseID {
				return nil, dbxpluginsdk.NewError(-32602, "当前发布与回滚目标不能相同")
			}
			q["toReleaseId"] = v
		}
		return c.do("PUT", "/openapi/v1/envs/"+esc(env)+"/releases/"+esc(releaseID)+"/rollback", q, nil)

	case "apollo/branch.get":
		if namespace == "" {
			return nil, dbxpluginsdk.NewError(-32602, "missing namespace")
		}
		// GET .../branches returns the single gray-branch namespace DTO
		// (a namespace has at most one gray branch).
		raw, perr := c.do("GET", nsBase()+"/branches", nil, nil)
		if perr != nil {
			if hasStatus(perr, http.StatusNotFound) {
				return map[string]any{"exists": false}, nil
			}
			return nil, perr
		}
		var dto map[string]any
		if err := json.Unmarshal(raw, &dto); err != nil {
			return nil, dbxpluginsdk.NewError(-32001, "parse branch: "+err.Error())
		}
		if len(dto) == 0 {
			return map[string]any{"exists": false}, nil
		}
		dto["exists"] = true
		if _, ok := dto["branchName"]; !ok {
			if cn, ok := dto["clusterName"].(string); ok && cn != "" {
				dto["branchName"] = cn
			}
		}
		return dto, nil

	case "apollo/branch.create":
		if namespace == "" {
			return nil, dbxpluginsdk.NewError(-32602, "missing namespace")
		}
		return c.do("POST", nsBase()+"/branches",
			map[string]string{"operator": operator}, nil)

	case "apollo/branch.delete":
		branch := strVal(values, "branch")
		if branch == "" || namespace == "" {
			return nil, dbxpluginsdk.NewError(-32602, "missing branch or namespace")
		}
		return c.do("DELETE", nsBase()+"/branches/"+esc(branch),
			map[string]string{"operator": operator}, nil)

	case "apollo/branch.items":
		branch := strVal(values, "branch")
		if branch == "" || namespace == "" {
			return nil, dbxpluginsdk.NewError(-32602, "missing branch or namespace")
		}
		// A gray branch is a special cluster; read its items via the namespace.
		return c.namespaceItems(env, appID, branch, namespace)

	case "apollo/branch.rules.get":
		branch := strVal(values, "branch")
		if branch == "" || namespace == "" {
			return nil, dbxpluginsdk.NewError(-32602, "missing branch or namespace")
		}
		return c.do("GET", nsBase()+"/branches/"+esc(branch)+"/rules", nil, nil)

	case "apollo/branch.rules.set":
		branch := strVal(values, "branch")
		if branch == "" || namespace == "" {
			return nil, dbxpluginsdk.NewError(-32602, "missing branch or namespace")
		}
		var rules any
		if r, ok := values["rules"]; ok {
			rules = r
		} else {
			rules = map[string]any{"ruleItems": values["ruleItems"]}
		}
		return c.do("PUT", nsBase()+"/branches/"+esc(branch)+"/rules",
			map[string]string{"operator": operator}, rules)

	case "apollo/branch.release":
		branch := strVal(values, "branch")
		if branch == "" || namespace == "" {
			return nil, dbxpluginsdk.NewError(-32602, "missing branch or namespace")
		}
		return c.do("POST", nsBase()+"/branches/"+esc(branch)+"/releases",
			map[string]string{"operator": operator}, releaseBody(values, operator))

	case "apollo/branch.merge":
		branch := strVal(values, "branch")
		if branch == "" || namespace == "" {
			return nil, dbxpluginsdk.NewError(-32602, "missing branch or namespace")
		}
		return c.do("POST", nsBase()+"/branches/"+esc(branch)+"/merge",
			map[string]string{"operator": operator, "deleteBranch": strconv.FormatBool(boolVal(values, "deleteBranch", true))}, releaseBody(values, operator))

	case "apollo/branch.abandon":
		branch := strVal(values, "branch")
		if branch == "" || namespace == "" {
			return nil, dbxpluginsdk.NewError(-32602, "missing branch or namespace")
		}
		// gray-del-releases publishes deleted keys; abandoning uses branch deletion.
		return c.do("DELETE", nsBase()+"/branches/"+esc(branch),
			map[string]string{"operator": operator}, nil)
	}

	return nil, dbxpluginsdk.MethodNotFound(method)
}

func main() {
	metadata := dbxpluginsdk.Metadata{
		ID:           "io.dbx.apollo",
		Version:      "1.0.0",
		Capabilities: []string{"connections"},
	}
	server := dbxpluginsdk.NewServer(metadata, &plugin{clients: map[string]*apolloClient{}})
	if err := server.Serve(); err != nil {
		panic(err)
	}
}
