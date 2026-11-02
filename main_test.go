package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

// newTestServer 起一个临时库的完整 mux，测的是 handler 而不是网络栈。
func newTestServer(t *testing.T) http.Handler {
	t.Helper()
	db, err := openDB(filepath.Join(t.TempDir(), "plan.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	mux := http.NewServeMux()
	mux.HandleFunc("/api/plan", func(w http.ResponseWriter, r *http.Request) { planHandler(w, r, db, defaultPlanID) })
	mux.HandleFunc("/api/plans", func(w http.ResponseWriter, r *http.Request) { plansHandler(w, r, db) })
	mux.HandleFunc("/api/plans/", func(w http.ResponseWriter, r *http.Request) {
		id := strings.TrimPrefix(r.URL.Path, "/api/plans/")
		if !validPlanID(id) {
			http.Error(w, "bad id", http.StatusBadRequest)
			return
		}
		planHandler(w, r, db, id)
	})
	mux.HandleFunc("/api/items", func(w http.ResponseWriter, r *http.Request) { itemsHandler(w, r, db) })
	return mount(mux, normalizeBase("/floorplan"))
}

func do(t *testing.T, h http.Handler, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

func TestPlanRoundTrip(t *testing.T) {
	h := newTestServer(t)

	if w := do(t, h, "GET", "/floorplan/api/plan", ""); w.Code != http.StatusNoContent {
		t.Fatalf("空库应返回 204，实际 %d", w.Code)
	}
	if w := do(t, h, "PUT", "/floorplan/api/plan", `{"style":"chinese"}`); w.Code != http.StatusNoContent {
		t.Fatalf("PUT 应返回 204，实际 %d", w.Code)
	}
	w := do(t, h, "GET", "/floorplan/api/plan", "")
	if w.Code != http.StatusOK || strings.TrimSpace(w.Body.String()) != `{"style":"chinese"}` {
		t.Fatalf("回读不一致: %d %q", w.Code, w.Body.String())
	}
	if w := do(t, h, "PUT", "/floorplan/api/plan", `{oops`); w.Code != http.StatusBadRequest {
		t.Fatalf("非法 JSON 应 400，实际 %d", w.Code)
	}
}

func TestItemsReplace(t *testing.T) {
	h := newTestServer(t)
	body := `[{"category":"地面","name":"木地板","qty":32.2,"price":268},{"name":"乳胶漆","unit":"L","qty":18}]`
	if w := do(t, h, "PUT", "/floorplan/api/items", body); w.Code != http.StatusNoContent {
		t.Fatalf("PUT 应返回 204，实际 %d", w.Code)
	}
	// 第二次整表替换，行数必须收敛而不是叠加
	if w := do(t, h, "PUT", "/floorplan/api/items", `[{"name":"踢脚线","qty":48}]`); w.Code != http.StatusNoContent {
		t.Fatalf("PUT 应返回 204，实际 %d", w.Code)
	}
	var got []item
	if err := json.Unmarshal(do(t, h, "GET", "/floorplan/api/items", "").Body.Bytes(), &got); err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if len(got) != 1 || got[0].Name != "踢脚线" {
		t.Fatalf("整表替换失败: %+v", got)
	}
}

func TestSubPathRouting(t *testing.T) {
	h := newTestServer(t)
	if w := do(t, h, "GET", "/floorplan", ""); w.Code != http.StatusMovedPermanently || w.Header().Get("location") != "/floorplan/" {
		t.Fatalf("/floorplan 应 301 到 /floorplan/，实际 %d %q", w.Code, w.Header().Get("location"))
	}
	if w := do(t, h, "GET", "/api/plan", ""); w.Code != http.StatusNotFound {
		t.Fatalf("前缀外的路径应 404，实际 %d", w.Code)
	}
	if w := do(t, h, "GET", "/floorplan/api/plan", ""); w.Code != http.StatusNoContent {
		t.Fatalf("前缀内应可达，实际 %d", w.Code)
	}
}

// 多方案：列表 / 复制 / 按 id 读写 / 重命名 / 删除 / 非法 id
func TestPlansCRUD(t *testing.T) {
	h := newTestServer(t)

	if w := do(t, h, "PUT", "/floorplan/api/plan", `{"style":"modern"}`); w.Code != http.StatusNoContent {
		t.Fatalf("默认方案写入失败: %d", w.Code)
	}
	// 列表里只有默认方案
	var list []planMeta
	json.Unmarshal(do(t, h, "GET", "/floorplan/api/plans", "").Body.Bytes(), &list)
	if len(list) != 1 || list[0].ID != defaultPlanID {
		t.Fatalf("初始列表不对: %+v", list)
	}

	// 复制成 plan-b
	if w := do(t, h, "POST", "/floorplan/api/plans?from=default&id=plan-b&name=方案B", ""); w.Code != http.StatusOK {
		t.Fatalf("复制失败: %d %s", w.Code, w.Body.String())
	}
	if w := do(t, h, "GET", "/floorplan/api/plans/plan-b", ""); w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"modern"`) {
		t.Fatalf("复制出来的方案读不到: %d", w.Code)
	}

	// 改 B 不影响 A
	do(t, h, "PUT", "/floorplan/api/plans/plan-b?name=方案B", `{"style":"chinese"}`)
	if got := do(t, h, "GET", "/floorplan/api/plan", "").Body.String(); !strings.Contains(got, "modern") {
		t.Fatalf("改 B 污染了默认方案: %s", got)
	}
	if got := do(t, h, "GET", "/floorplan/api/plans/plan-b", "").Body.String(); !strings.Contains(got, "chinese") {
		t.Fatalf("B 没存住: %s", got)
	}

	// 重命名（PUT 带 name）
	do(t, h, "PUT", "/floorplan/api/plans/plan-b?name=客厅改造", `{"style":"chinese"}`)
	json.Unmarshal(do(t, h, "GET", "/floorplan/api/plans", "").Body.Bytes(), &list)
	found := false
	for _, m := range list {
		if m.ID == "plan-b" && m.Name == "客厅改造" {
			found = true
		}
	}
	if !found {
		t.Fatalf("重命名没生效: %+v", list)
	}

	// 非法 id 与删除默认方案都要拦住
	if w := do(t, h, "GET", "/floorplan/api/plans/bad..id", ""); w.Code != http.StatusBadRequest {
		t.Fatalf("非法 id 应该 400，实际 %d", w.Code)
	}
	if w := do(t, h, "DELETE", "/floorplan/api/plans/default", ""); w.Code != http.StatusBadRequest {
		t.Fatalf("默认方案不该能删，实际 %d", w.Code)
	}
	if w := do(t, h, "DELETE", "/floorplan/api/plans/plan-b", ""); w.Code != http.StatusNoContent {
		t.Fatalf("删除失败: %d", w.Code)
	}
	json.Unmarshal(do(t, h, "GET", "/floorplan/api/plans", "").Body.Bytes(), &list)
	if len(list) != 1 {
		t.Fatalf("删除后应只剩默认方案: %+v", list)
	}
}
