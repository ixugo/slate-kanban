package desktop

import (
	"bufio"
	"encoding/json/v2"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"sync"
	"testing"

	"slate-kanban/internal/app"
	"slate-kanban/internal/core/kanban"
)

const processDataDir = "SLATE_DESKTOP_TEST_DATA_DIR"

type serviceReady struct {
	URL    string
	Recent []kanban.RecentCard
}

func TestDesktopServiceRestartsWithSameData(t *testing.T) {
	dataDir := t.TempDir()
	first, stop := startTestProcess(t, dataDir)
	if data := request(t, first.URL, http.MethodGet, "/health", ""); !strings.Contains(string(data), "v1.2.3") {
		t.Fatalf("桌面健康检查未返回构建版本: %s", data)
	}
	boardData := request(t, first.URL, http.MethodPost, "/api/v1/boards",
		`{"name":"Desktop board","slug":"DESK","template":"simple"}`)
	var board struct {
		ID int64 `json:"id"`
	}
	if err := json.Unmarshal(boardData, &board); err != nil || board.ID <= 0 {
		t.Fatalf("创建看板结果无效: %s (%v)", boardData, err)
	}
	createPath := fmt.Sprintf("/api/v1/boards/%d/cards", board.ID)
	var card struct {
		ID int64 `json:"id"`
	}
	data := request(t, first.URL, http.MethodPost, createPath, `{"title":"Persistent card"}`)
	if err := json.Unmarshal(data, &card); err != nil || card.ID <= 0 {
		t.Fatalf("创建任务结果无效: %s (%v)", data, err)
	}
	request(t, first.URL, http.MethodPost, createPath, `{"title":"Newest card"}`)
	request(t, first.URL, http.MethodPost, fmt.Sprintf("/api/v1/cards/%d/comments", card.ID), `{"body":"最近的处理"}`)
	if err := stop(); err != nil {
		t.Fatal(err)
	}

	second, _ := startTestProcess(t, dataDir)
	data = request(t, second.URL, http.MethodGet, fmt.Sprintf("/api/v1/cards/%d", card.ID), "")
	var restored struct {
		Title string `json:"title"`
	}
	if err := json.Unmarshal(data, &restored); err != nil || restored.Title != "Persistent card" {
		t.Fatalf("重启后任务不存在: %s (%v)", data, err)
	}
	if recent := second.Recent; len(recent) != 2 || recent[1].Title != "Newest card" ||
		recent[0].ID != card.ID || recent[0].BoardID != board.ID {
		t.Fatalf("重启后最近任务的内容或顺序错误: %+v", recent)
	}

}

// 跟随桌面进程的真实边界启动服务，避免在同一测试进程重复注册全局指标。
func TestDesktopServiceProcess(t *testing.T) {
	dataDir := os.Getenv(processDataDir)
	if dataDir == "" {
		return
	}
	build := app.BuildInfo{Version: "v1.2.3", Branch: "desktop-test", Commit: "123456"}
	cfg, closeConfig, err := loadConfig(build.Version)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(closeConfig)
	cfg.Server.HTTP.Host = "127.0.0.1"
	cfg.Server.HTTP.Port = 0
	s, err := startService(cfg, build, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := s.close(); err != nil {
			t.Error(err)
		}
	})
	recent, err := s.recentCards(t.Context(), 8)
	if err != nil {
		t.Fatal(err)
	}
	payload, err := json.Marshal(serviceReady{URL: s.url, Recent: recent})
	if err != nil {
		t.Fatal(err)
	}
	fmt.Printf("SLATE_READY %s\n", payload)
	if _, err := io.Copy(io.Discard, os.Stdin); err != nil {
		t.Fatal(err)
	}
}

func startTestProcess(t *testing.T, dataDir string) (serviceReady, func() error) {
	t.Helper()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(executable, "-test.run=^TestDesktopServiceProcess$")
	cmd.Env = append(os.Environ(), "HOME="+dataDir, "USERPROFILE="+dataDir, processDataDir+"="+dataDir)
	cmd.Stderr = os.Stderr
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	stop := sync.OnceValue(func() error {
		_ = stdin.Close()
		return cmd.Wait()
	})
	t.Cleanup(func() {
		if err := stop(); err != nil {
			t.Error(err)
		}
	})
	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		value, found := strings.CutPrefix(scanner.Text(), "SLATE_READY ")
		if !found {
			continue
		}
		var ready serviceReady
		if err := json.Unmarshal([]byte(value), &ready); err != nil {
			t.Fatal(err)
		}
		return ready, stop
	}
	t.Fatalf("桌面进程未就绪: %v", scanner.Err())
	return serviceReady{}, stop
}

func request(t *testing.T, origin, method, path, body string) []byte {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), method, origin+path, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	data, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusOK {
		t.Fatalf("%s %s: %s: %s", method, path, res.Status, data)
	}
	return data
}
