package workmemory

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"ariadne/internal/securestore"
)

const milvusVectorField = "vector"

type embeddingHit struct {
	EntryID string
	Score   float64
}

type milvusRESTVectorStore struct {
	HTTPClient *http.Client
}

type milvusAPIResponse struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

func newMilvusRESTVectorStore() milvusRESTVectorStore {
	return milvusRESTVectorStore{HTTPClient: &http.Client{Timeout: 30 * time.Second}}
}

func (m milvusRESTVectorStore) Refresh(ctx context.Context, policy EmbeddingPolicy, namespace string, records []embeddingRecord) (int, error) {
	if len(records) == 0 {
		return 0, nil
	}
	dimension := len(records[0].Vector)
	if dimension == 0 {
		return 0, fmt.Errorf("Milvus 向量维度为空")
	}
	for _, record := range records {
		if len(record.Vector) != dimension {
			return 0, fmt.Errorf("Milvus 向量维度不一致: %s", record.EntryID)
		}
	}
	if err := m.ensureCollection(ctx, policy, dimension); err != nil {
		return 0, err
	}
	if err := m.deleteNamespace(ctx, policy, namespace); err != nil {
		return 0, err
	}
	indexed := 0
	for start := 0; start < len(records); start += 128 {
		end := start + 128
		if end > len(records) {
			end = len(records)
		}
		count, err := m.upsertBatch(ctx, policy, namespace, records[start:end])
		if err != nil {
			return indexed, err
		}
		indexed += count
	}
	if err := m.loadCollection(ctx, policy); err != nil {
		return indexed, err
	}
	return indexed, nil
}

func (m milvusRESTVectorStore) Search(ctx context.Context, policy EmbeddingPolicy, namespace string, vector []float64, limit int) ([]embeddingHit, error) {
	if len(vector) == 0 {
		return nil, fmt.Errorf("Milvus 查询向量为空")
	}
	if limit <= 0 {
		limit = 20
	}
	if err := m.loadCollection(ctx, policy); err != nil {
		return nil, err
	}
	payload := map[string]any{
		"collectionName": policy.VectorCollection,
		"data":           [][]float64{vector},
		"annsField":      milvusVectorField,
		"filter":         fmt.Sprintf("namespace == %q", namespace),
		"limit":          limit,
		"outputFields":   []string{"entry_id"},
		"searchParams": map[string]any{
			"metricType": "COSINE",
			"params":     map[string]any{},
		},
	}
	var rows []struct {
		EntryID  string  `json:"entry_id"`
		Distance float64 `json:"distance"`
	}
	if err := m.post(ctx, policy, "/v2/vectordb/entities/search", payload, &rows); err != nil {
		return nil, err
	}
	hits := make([]embeddingHit, 0, len(rows))
	for _, row := range rows {
		if strings.TrimSpace(row.EntryID) == "" {
			continue
		}
		hits = append(hits, embeddingHit{EntryID: row.EntryID, Score: row.Distance})
	}
	return hits, nil
}

func (m milvusRESTVectorStore) ensureCollection(ctx context.Context, policy EmbeddingPolicy, dimension int) error {
	exists, err := m.hasCollection(ctx, policy)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}
	payload := map[string]any{
		"collectionName": policy.VectorCollection,
		"schema": map[string]any{
			"autoID":             false,
			"enableDynamicField": false,
			"fields": []map[string]any{
				{
					"fieldName":         "id",
					"dataType":          "VarChar",
					"isPrimary":         true,
					"elementTypeParams": map[string]string{"max_length": "256"},
				},
				{
					"fieldName":         "entry_id",
					"dataType":          "VarChar",
					"elementTypeParams": map[string]string{"max_length": "256"},
				},
				{
					"fieldName":         "namespace",
					"dataType":          "VarChar",
					"elementTypeParams": map[string]string{"max_length": "128"},
				},
				{"fieldName": "indexed_at", "dataType": "Int64"},
				{
					"fieldName":         milvusVectorField,
					"dataType":          "FloatVector",
					"elementTypeParams": map[string]string{"dim": fmt.Sprintf("%d", dimension)},
				},
			},
		},
		"indexParams": []map[string]any{
			{
				"fieldName":  milvusVectorField,
				"indexName":  "vector_index",
				"metricType": "COSINE",
				"params":     map[string]string{"index_type": "AUTOINDEX"},
			},
		},
	}
	if err := m.post(ctx, policy, "/v2/vectordb/collections/create", payload, nil); err != nil {
		return err
	}
	return m.loadCollection(ctx, policy)
}

func (m milvusRESTVectorStore) hasCollection(ctx context.Context, policy EmbeddingPolicy) (bool, error) {
	payload := map[string]any{}
	var names []string
	if err := m.post(ctx, policy, "/v2/vectordb/collections/list", payload, &names); err != nil {
		return false, err
	}
	for _, name := range names {
		if name == policy.VectorCollection {
			return true, nil
		}
	}
	return false, nil
}

func (m milvusRESTVectorStore) loadCollection(ctx context.Context, policy EmbeddingPolicy) error {
	return m.post(ctx, policy, "/v2/vectordb/collections/load", map[string]any{
		"collectionName": policy.VectorCollection,
	}, nil)
}

func (m milvusRESTVectorStore) deleteNamespace(ctx context.Context, policy EmbeddingPolicy, namespace string) error {
	return m.post(ctx, policy, "/v2/vectordb/entities/delete", map[string]any{
		"collectionName": policy.VectorCollection,
		"filter":         fmt.Sprintf("namespace == %q", namespace),
	}, nil)
}

func (m milvusRESTVectorStore) upsertBatch(ctx context.Context, policy EmbeddingPolicy, namespace string, records []embeddingRecord) (int, error) {
	rows := make([]map[string]any, 0, len(records))
	for _, record := range records {
		rows = append(rows, map[string]any{
			"id":         milvusRecordID(namespace, record.EntryID),
			"entry_id":   record.EntryID,
			"namespace":  namespace,
			"indexed_at": record.IndexedAt,
			"vector":     record.Vector,
		})
	}
	payload := map[string]any{
		"collectionName": policy.VectorCollection,
		"data":           rows,
	}
	var result struct {
		UpsertCount int `json:"upsertCount"`
	}
	if err := m.post(ctx, policy, "/v2/vectordb/entities/upsert", payload, &result); err != nil {
		return 0, err
	}
	if result.UpsertCount == 0 {
		return len(records), nil
	}
	return result.UpsertCount, nil
}

func (m milvusRESTVectorStore) post(ctx context.Context, policy EmbeddingPolicy, path string, payload any, data any) error {
	endpoint, err := milvusEndpoint(policy.VectorStoreURI)
	if err != nil {
		return err
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint+path, bytes.NewReader(raw))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if token := milvusToken(); token != "" {
		if strings.HasPrefix(strings.ToLower(token), "bearer ") {
			req.Header.Set("Authorization", token)
		} else {
			req.Header.Set("Authorization", "Bearer "+token)
		}
	}
	client := m.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return err
	}
	if resp.StatusCode >= 400 {
		return fmt.Errorf("Milvus REST HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var api milvusAPIResponse
	if err := json.Unmarshal(body, &api); err != nil {
		return fmt.Errorf("Milvus REST 响应解析失败: %w", err)
	}
	if api.Code != 0 {
		if strings.TrimSpace(api.Message) == "" {
			api.Message = "unknown Milvus error"
		}
		return fmt.Errorf("Milvus REST code %d: %s", api.Code, api.Message)
	}
	if data != nil && len(api.Data) > 0 && string(api.Data) != "null" {
		if err := json.Unmarshal(api.Data, data); err != nil {
			return fmt.Errorf("Milvus REST data 解析失败: %w", err)
		}
	}
	return nil
}

func milvusEndpoint(raw string) (string, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return "", fmt.Errorf("Milvus URI 未配置")
	}
	if strings.HasPrefix(strings.ToLower(value), "milvus://") {
		value = "http://" + value[len("milvus://"):]
	}
	if !strings.Contains(value, "://") {
		value = "http://" + value
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.Host == "" {
		return "", fmt.Errorf("Milvus URI 无效: %s", raw)
	}
	return strings.TrimRight(parsed.String(), "/"), nil
}

func milvusToken() string {
	for _, key := range []string{"ARIADNE_MILVUS_TOKEN", "MILVUS__TOKEN", "MILVUS_TOKEN"} {
		if value := strings.TrimSpace(os.Getenv(key)); value != "" {
			return value
		}
	}
	if value, ok, err := securestore.Read(securestore.TargetMilvusToken); err == nil && ok && strings.TrimSpace(value) != "" {
		return strings.TrimSpace(value)
	}
	return ""
}

func milvusRecordID(namespace string, entryID string) string {
	id := namespace + "::" + strings.TrimSpace(entryID)
	if len(id) <= 256 {
		return id
	}
	return namespace + "::" + shortHash(entryID)
}
