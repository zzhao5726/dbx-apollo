package main

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	dbxpluginsdk "github.com/t8y2/dbx/plugins/sdk/go/dbx-plugin-sdk"
)

// 1.x / 2.x 的 OpenAPI 只接受 Token；Portal 登录使用其原生接口和数据结构。
func (c *apolloClient) legacyPortalRequest(method, path string, query map[string]string, body any) (json.RawMessage, *dbxpluginsdk.PluginError) {
	parts := strings.Split(strings.TrimPrefix(path, "/openapi/v1/"), "/")
	portalPath := "/" + strings.Join(parts, "/")
	if path == "/openapi/v1/apps/authorized" {
		return c.request(method, "/apps", nil, nil, true)
	}
	if len(parts) == 3 && parts[0] == "apps" && parts[2] == "envclusters" {
		return c.portalEnvClusters("/apps/" + parts[1] + "/navtree")
	}
	if len(parts) >= 4 && parts[0] == "envs" && parts[2] == "apps" {
		portalPath = "/apps/" + parts[3] + "/envs/" + parts[1]
		if len(parts) > 4 {
			portalPath += "/" + strings.Join(parts[4:], "/")
		}
	}
	if method == "POST" && len(parts) == 3 && parts[0] == "apps" && parts[2] == "appnamespaces" {
		values := body.(map[string]any)
		query = map[string]string{"appendNamespacePrefix": fmt.Sprint(values["appendNamespacePrefix"])}
	}
	// Portal 修改、删除配置使用真实 itemId，不能把配置 key 当作 id。
	if len(parts) == 10 && parts[8] == "items" && (method == "PUT" || method == "DELETE") {
		return c.portalWriteItem(method, portalPath[:strings.LastIndex(portalPath, "/items/")], parts[9], query, body)
	}
	if method == "POST" && len(parts) == 9 && parts[8] == "items" {
		portalPath = strings.TrimSuffix(portalPath, "items") + "item"
	}
	if method == "GET" && len(parts) == 9 && parts[8] == "lock" {
		raw, perr := c.portalDo("GET", portalPath+"-info", nil)
		if perr != nil {
			return nil, perr
		}
		var lock map[string]any
		if err := json.Unmarshal(raw, &lock); err != nil {
			return nil, portalDecodeError(raw, err)
		}
		if lock == nil {
			return raw, nil
		}
		lock["isLocked"] = strVal(lock, "lockOwner") != ""
		lock["lockedBy"] = lock["lockOwner"]
		return portalJSON(lock)
	}
	if method == "GET" && len(parts) == 10 && parts[8] == "releases" && parts[9] == "latest" {
		raw, perr := c.request(method, strings.TrimSuffix(portalPath, "latest")+"active", map[string]string{"page": "0", "size": "1"}, nil, true)
		if perr != nil {
			return nil, perr
		}
		var releases []json.RawMessage
		if err := json.Unmarshal(raw, &releases); err != nil {
			return nil, portalDecodeError(raw, err)
		}
		if len(releases) == 0 {
			return json.RawMessage("null"), nil
		}
		return releases[0], nil
	}
	raw, perr := c.request(method, portalPath, query, body, true)
	if perr != nil || method != "GET" {
		return raw, perr
	}
	if len(parts) == 7 && parts[6] == "namespaces" {
		var namespaces []json.RawMessage
		if err := json.Unmarshal(raw, &namespaces); err != nil {
			return nil, portalDecodeError(raw, err)
		}
		out := []map[string]any{}
		for _, namespace := range namespaces {
			ns, err := flattenPortalNamespace(namespace)
			if err != nil {
				return nil, portalDecodeError(raw, err)
			}
			out = append(out, ns)
		}
		return portalJSON(out)
	}
	if (len(parts) == 8 && parts[6] == "namespaces") || (len(parts) == 9 && parts[8] == "branches") {
		if string(raw) == "null" {
			return raw, nil
		}
		ns, err := flattenPortalNamespace(raw)
		if err != nil {
			return nil, portalDecodeError(raw, err)
		}
		return portalJSON(ns)
	}
	return raw, nil
}

func (c *apolloClient) portalEnvClusters(path string) (json.RawMessage, *dbxpluginsdk.PluginError) {
	raw, perr := c.portalDo("GET", path, nil)
	if perr != nil {
		return nil, perr
	}
	var response struct {
		Entities []struct {
			Code int `json:"code"`
			Body struct {
				Env      string `json:"env"`
				Clusters []struct {
					Name string `json:"name"`
				} `json:"clusters"`
			} `json:"body"`
		} `json:"entities"`
	}
	if err := json.Unmarshal(raw, &response); err != nil {
		return nil, portalDecodeError(raw, err)
	}
	if response.Entities == nil {
		return nil, portalDecodeError(raw, fmt.Errorf("Portal 未返回环境和集群列表"))
	}
	out := []map[string]any{}
	for _, entity := range response.Entities {
		if entity.Code != 200 {
			return nil, portalDecodeError(raw, fmt.Errorf("Portal 加载环境失败 (code %d)", entity.Code))
		}
		names := []string{}
		for _, cluster := range entity.Body.Clusters {
			names = append(names, cluster.Name)
		}
		out = append(out, map[string]any{"env": entity.Body.Env, "clusters": names})
	}
	return portalJSON(out)
}

func flattenPortalNamespace(raw json.RawMessage) (map[string]any, error) {
	var namespace map[string]any
	if err := json.Unmarshal(raw, &namespace); err != nil {
		return nil, err
	}
	base, ok := namespace["baseInfo"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("Portal 未返回命名空间 baseInfo")
	}
	for key, value := range namespace {
		if key != "baseInfo" && key != "items" {
			base[key] = value
		}
	}
	items := []any{}
	if entries, ok := namespace["items"].([]any); ok {
		for _, entry := range entries {
			item, ok := entry.(map[string]any)
			if !ok || item["item"] == nil {
				return nil, fmt.Errorf("Portal 未返回配置项 item")
			}
			if !boolVal(item, "isDeleted", false) {
				items = append(items, item["item"])
			}
		}
	}
	base["items"] = items
	return base, nil
}

func (c *apolloClient) portalWriteItem(method, namespacePath, escapedKey string, query map[string]string, body any) (json.RawMessage, *dbxpluginsdk.PluginError) {
	key, err := url.PathUnescape(escapedKey)
	if err != nil {
		return nil, dbxpluginsdk.NewError(-32602, err.Error())
	}
	raw, perr := c.portalDo("GET", namespacePath+"/items", nil)
	if perr != nil {
		return nil, perr
	}
	var items []map[string]any
	if err := json.Unmarshal(raw, &items); err != nil {
		return nil, portalDecodeError(raw, err)
	}
	for _, item := range items {
		if strVal(item, "key") != key {
			continue
		}
		if method == "DELETE" {
			return c.portalDo("DELETE", namespacePath+"/items/"+esc(strVal(item, "id")), nil)
		}
		for field, value := range body.(map[string]any) {
			item[field] = value
		}
		return c.portalDo("PUT", namespacePath+"/item", item)
	}
	if method == "PUT" && query["createIfNotExists"] == "true" {
		return c.portalDo("POST", namespacePath+"/item", body)
	}
	return nil, dbxpluginsdk.NewError(-32001, "配置不存在："+key)
}

func portalDecodeError(raw json.RawMessage, err error) *dbxpluginsdk.PluginError {
	perr := dbxpluginsdk.NewError(-32001, "parse Portal response: "+err.Error())
	perr.Data = map[string]any{"body": string(raw)}
	return perr
}

func portalJSON(value any) (json.RawMessage, *dbxpluginsdk.PluginError) {
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, dbxpluginsdk.NewError(-32603, err.Error())
	}
	return raw, nil
}
