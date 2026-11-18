// Package importer 提供 Postman Collection (v2.0 / v2.1) 与 OpenAPI 数据导入解析。
package importer

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"rddpostman/internal/domain"
)

// postmanCollectionSchema 对应 Postman 导出的 collection.json 顶层结构。
type postmanCollectionSchema struct {
	Info struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		Schema      string `json:"schema"`
	} `json:"info"`
	Item     []postmanItem `json:"item"`
	Auth     *postmanAuth  `json:"auth"`
	Variable []postmanKV   `json:"variable"`
}

type postmanItem struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Item        []postmanItem   `json:"item,omitempty"` // 递归子目录
	Request     *postmanRequest `json:"request,omitempty"`
}

type postmanRequest struct {
	Method string          `json:"method"`
	Header []postmanKV     `json:"header"`
	Body   *postmanBody    `json:"body"`
	URL    json.RawMessage `json:"url"` // 可以是 string 或 object
	Auth   *postmanAuth    `json:"auth"`
}

type postmanBody struct {
	Mode       string        `json:"mode"`
	Raw        string        `json:"raw"`
	URLEncoded []postmanKV   `json:"urlencoded"`
	FormData   []postmanKV   `json:"formdata"`
	Options    *postmanOpt   `json:"options"`
}

type postmanOpt struct {
	Raw struct {
		Language string `json:"language"`
	} `json:"raw"`
}

type postmanAuth struct {
	Type   string                   `json:"type"`
	Bearer []postmanKV              `json:"bearer"`
	Basic  []postmanKV              `json:"basic"`
	APIKey []postmanKV              `json:"apikey"`
}

type postmanKV struct {
	Key      string `json:"key"`
	Value    string `json:"value"`
	Type     string `json:"type"`
	Disabled bool   `json:"disabled"`
	Desc     string `json:"description"`
}

// ParsePostmanCollection 解析 Postman v2.0/v2.1 导出的 JSON 数据，转换为当前系统 domain.Collection。
func ParsePostmanCollection(data []byte) (*domain.Collection, error) {
	var pm postmanCollectionSchema
	if err := json.Unmarshal(data, &pm); err != nil {
		return nil, fmt.Errorf("解析 Postman Collection JSON 失败: %w", err)
	}

	name := strings.TrimSpace(pm.Info.Name)
	if name == "" {
		name = "导入的集合"
	}

	now := time.Now()
	coll := &domain.Collection{
		Schema:      domain.SchemaCollection,
		Version:     domain.Version,
		ID:          domain.NewID(),
		Name:        name,
		Description: pm.Info.Description,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	// 转换集合级变量
	for _, v := range pm.Variable {
		if strings.TrimSpace(v.Key) != "" {
			coll.Variables = append(coll.Variables, domain.KV{
				ID:      domain.NewID(),
				Key:     v.Key,
				Value:   v.Value,
				Desc:    v.Desc,
				Enabled: !v.Disabled,
			})
		}
	}

	// 转换集合级 Auth
	if pm.Auth != nil {
		coll.Auth = convertAuth(pm.Auth)
	}

	// 递归转换子节点
	coll.Items = convertItems(pm.Item)

	return coll, nil
}

func convertItems(items []postmanItem) []*domain.Node {
	var nodes []*domain.Node
	for idx, it := range items {
		now := time.Now()
		node := &domain.Node{
			Version:     domain.Version,
			ID:          domain.NewID(),
			Name:        it.Name,
			Description: it.Description,
			Order:       int64((idx + 1) * 100),
			CreatedAt:   now,
			UpdatedAt:   now,
		}

		if len(it.Item) > 0 || it.Request == nil {
			// 目录节点
			node.Schema = domain.SchemaFolder
			node.Kind = domain.KindFolder
			node.Children = convertItems(it.Item)
		} else {
			// 请求节点
			node.Schema = domain.SchemaRequest
			node.Kind = domain.KindRequest
			node.Request = convertRequest(it.Request)
			if it.Request.Auth != nil {
				node.Auth = convertAuth(it.Request.Auth)
			}
		}
		nodes = append(nodes, node)
	}
	return nodes
}

func convertRequest(pr *postmanRequest) *domain.Request {
	if pr == nil {
		return nil
	}
	now := time.Now()
	req := &domain.Request{
		Method:    strings.ToUpper(strings.TrimSpace(pr.Method)),
		CreatedAt: now,
		UpdatedAt: now,
		Options: domain.Options{
			TimeoutMS: 10000,
		},
	}
	if req.Method == "" {
		req.Method = "GET"
	}

	// 解析 URL
	req.URL, req.Query, req.PathVariables = parsePostmanURL(pr.URL)

	// 解析 Headers
	for _, h := range pr.Header {
		if strings.TrimSpace(h.Key) != "" {
			req.Headers = append(req.Headers, domain.KV{
				ID:      domain.NewID(),
				Key:     h.Key,
				Value:   h.Value,
				Desc:    h.Desc,
				Enabled: !h.Disabled,
			})
		}
	}

	// 解析 Body
	if pr.Body != nil {
		switch strings.ToLower(pr.Body.Mode) {
		case "raw":
			req.Body.Mode = domain.BodyRaw
			req.Body.Raw = pr.Body.Raw
			if pr.Body.Options != nil {
				req.Body.Language = pr.Body.Options.Raw.Language
			}
		case "urlencoded":
			req.Body.Mode = domain.BodyURLEncoded
			for _, f := range pr.Body.URLEncoded {
				req.Body.Fields = append(req.Body.Fields, domain.KV{
					ID:      domain.NewID(),
					Key:     f.Key,
					Value:   f.Value,
					Desc:    f.Desc,
					Enabled: !f.Disabled,
				})
			}
		case "formdata":
			req.Body.Mode = domain.BodyFormData
			for _, f := range pr.Body.FormData {
				req.Body.Fields = append(req.Body.Fields, domain.KV{
					ID:      domain.NewID(),
					Key:     f.Key,
					Value:   f.Value,
					Type:    f.Type,
					Desc:    f.Desc,
					Enabled: !f.Disabled,
				})
			}
		}
	}

	return req
}

func parsePostmanURL(rawURL json.RawMessage) (uStr string, queries []domain.KV, pathVars []domain.KV) {
	if len(rawURL) == 0 {
		return "", nil, nil
	}

	// 尝试作为字符串反序列化
	var str string
	if err := json.Unmarshal(rawURL, &str); err == nil {
		return str, nil, nil
	}

	// 否则作为 Postman URL Object
	var obj struct {
		Raw      string      `json:"raw"`
		Query    []postmanKV `json:"query"`
		Variable []postmanKV `json:"variable"`
	}
	if err := json.Unmarshal(rawURL, &obj); err == nil {
		uStr = obj.Raw
		for _, q := range obj.Query {
			if strings.TrimSpace(q.Key) != "" {
				queries = append(queries, domain.KV{
					ID:      domain.NewID(),
					Key:     q.Key,
					Value:   q.Value,
					Desc:    q.Desc,
					Enabled: !q.Disabled,
				})
			}
		}
		for _, v := range obj.Variable {
			if strings.TrimSpace(v.Key) != "" {
				pathVars = append(pathVars, domain.KV{
					ID:      domain.NewID(),
					Key:     v.Key,
					Value:   v.Value,
					Desc:    v.Desc,
					Enabled: !v.Disabled,
				})
			}
		}
	}
	return uStr, queries, pathVars
}

func convertAuth(pa *postmanAuth) domain.Auth {
	if pa == nil {
		return domain.Auth{Type: domain.AuthInherit}
	}
	auth := domain.Auth{}
	switch strings.ToLower(pa.Type) {
	case "bearer":
		auth.Type = domain.AuthBearer
		for _, kv := range pa.Bearer {
			auth.Params = append(auth.Params, domain.KV{Key: kv.Key, Value: kv.Value})
		}
	case "basic":
		auth.Type = domain.AuthBasic
		for _, kv := range pa.Basic {
			auth.Params = append(auth.Params, domain.KV{Key: kv.Key, Value: kv.Value})
		}
	case "apikey":
		auth.Type = domain.AuthAPIKey
		for _, kv := range pa.APIKey {
			auth.Params = append(auth.Params, domain.KV{Key: kv.Key, Value: kv.Value})
		}
	default:
		auth.Type = domain.AuthInherit
	}
	return auth
}

// ExportPostmanCollection 将当前的 domain.Collection 序列化为 Postman Collection v2.1 格式的 JSON 字节。
func ExportPostmanCollection(coll *domain.Collection) ([]byte, error) {
	if coll == nil {
		return nil, fmt.Errorf("集合为空")
	}

	var pm postmanCollectionSchema
	pm.Info.Name = coll.Name
	pm.Info.Description = coll.Description
	pm.Info.Schema = "https://schema.getpostman.com/json/collection/v2.1.0/collection.json"

	for _, v := range coll.Variables {
		pm.Variable = append(pm.Variable, postmanKV{
			Key:      v.Key,
			Value:    v.Value,
			Desc:     v.Desc,
			Disabled: !v.Enabled,
		})
	}

	if coll.Auth.Type != "" && coll.Auth.Type != domain.AuthInherit && coll.Auth.Type != domain.AuthNone {
		pm.Auth = exportAuth(coll.Auth)
	}

	pm.Item = exportItems(coll.Items)

	return json.MarshalIndent(pm, "", "  ")
}

func exportItems(nodes []*domain.Node) []postmanItem {
	var items []postmanItem
	for _, n := range nodes {
		if n == nil {
			continue
		}
		item := postmanItem{
			Name:        n.Name,
			Description: n.Description,
		}

		if n.Kind == domain.KindFolder {
			item.Item = exportItems(n.Children)
		} else if n.Kind == domain.KindRequest && n.Request != nil {
			req := n.Request
			pReq := &postmanRequest{
				Method: req.Method,
			}

			// URL
			rawURLBytes, _ := json.Marshal(req.URL)
			pReq.URL = rawURLBytes

			// Headers
			for _, h := range req.Headers {
				pReq.Header = append(pReq.Header, postmanKV{
					Key:      h.Key,
					Value:    h.Value,
					Desc:     h.Desc,
					Disabled: !h.Enabled,
				})
			}

			// Body
			if req.Body.Mode != "" && req.Body.Mode != domain.BodyNone {
				pBody := &postmanBody{Mode: string(req.Body.Mode)}
				switch req.Body.Mode {
				case domain.BodyRaw:
					pBody.Raw = req.Body.Raw
				case domain.BodyURLEncoded:
					for _, f := range req.Body.Fields {
						pBody.URLEncoded = append(pBody.URLEncoded, postmanKV{
							Key:      f.Key,
							Value:    f.Value,
							Disabled: !f.Enabled,
						})
					}
				case domain.BodyFormData:
					for _, f := range req.Body.Fields {
						pBody.FormData = append(pBody.FormData, postmanKV{
							Key:      f.Key,
							Value:    f.Value,
							Type:     f.Type,
							Disabled: !f.Enabled,
						})
					}
				}
				pReq.Body = pBody
			}

			// Auth
			if n.Auth.Type != "" && n.Auth.Type != domain.AuthInherit && n.Auth.Type != domain.AuthNone {
				pReq.Auth = exportAuth(n.Auth)
			}

			item.Request = pReq
		}

		items = append(items, item)
	}
	return items
}

func exportAuth(auth domain.Auth) *postmanAuth {
	pa := &postmanAuth{Type: string(auth.Type)}
	var kvs []postmanKV
	for _, p := range auth.Params {
		kvs = append(kvs, postmanKV{Key: p.Key, Value: p.Value})
	}
	switch auth.Type {
	case domain.AuthBearer:
		pa.Bearer = kvs
	case domain.AuthBasic:
		pa.Basic = kvs
	case domain.AuthAPIKey:
		pa.APIKey = kvs
	}
	return pa
}

// PostmanEnvironmentSchema 对应 Postman 导出的环境配置文件结构。
type PostmanEnvironmentSchema struct {
	ID     string              `json:"id,omitempty"`
	Name   string              `json:"name"`
	Values []PostmanEnvValue   `json:"values"`
	Scope  string              `json:"_postman_variable_scope"`
}

// PostmanEnvValue 对应环境配置中的单项键值对。
type PostmanEnvValue struct {
	Key     string `json:"key"`
	Value   string `json:"value"`
	Type    string `json:"type,omitempty"`
	Enabled bool   `json:"enabled"`
}

// ParsePostmanEnvironment 解析 Postman Environment JSON 格式数据。
func ParsePostmanEnvironment(data []byte) (*domain.Environment, error) {
	var pm PostmanEnvironmentSchema
	if err := json.Unmarshal(data, &pm); err != nil {
		return nil, fmt.Errorf("解析 Postman 环境 JSON 失败: %w", err)
	}

	name := strings.TrimSpace(pm.Name)
	if name == "" {
		name = "导入的环境"
	}

	now := time.Now()
	env := &domain.Environment{
		Schema:    domain.SchemaEnvironment,
		Version:   domain.Version,
		ID:        domain.NewID(),
		Name:      name,
		CreatedAt: now,
		UpdatedAt: now,
	}

	for _, v := range pm.Values {
		if strings.TrimSpace(v.Key) != "" {
			env.Vars = append(env.Vars, domain.KV{
				ID:      domain.NewID(),
				Key:     v.Key,
				Value:   v.Value,
				Enabled: v.Enabled,
				Secret:  v.Type == "secret",
			})
		}
	}

	return env, nil
}

// ExportPostmanEnvironment 将 domain.Environment 导出为 Postman 兼容的环境配置 JSON。
func ExportPostmanEnvironment(env *domain.Environment) ([]byte, error) {
	if env == nil {
		return nil, fmt.Errorf("环境为空")
	}

	pm := PostmanEnvironmentSchema{
		ID:    string(env.ID),
		Name:  env.Name,
		Scope: "environment",
	}

	for _, v := range env.Vars {
		vType := "default"
		if v.Secret {
			vType = "secret"
		}
		pm.Values = append(pm.Values, PostmanEnvValue{
			Key:     v.Key,
			Value:   v.Value,
			Type:    vType,
			Enabled: v.Enabled,
		})
	}

	return json.MarshalIndent(pm, "", "  ")
}
