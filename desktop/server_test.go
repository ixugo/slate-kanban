package desktop

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"

	"slate-kanban/internal/app"
	"slate-kanban/internal/conf"
	"slate-kanban/internal/core/kanban"
)

const (
	processDataDir    = "SLATE_DESKTOP_TEST_DATA_DIR"
	openedBrowserFile = "opened-browser-url"
)

type serviceReady struct {
	URL              string
	Port             int
	BrowserPath      string
	ConnectionScript string
	Recent           []kanban.RecentCard
}

func TestDesktopServiceRestartsWithSameData(t *testing.T) {
	dataDir := t.TempDir()
	configDir := filepath.Join(dataDir, conf.DefaultDirName)
	if err := os.Mkdir(configDir, 0o700); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(configDir, "config.toml")
	cfg := conf.DefaultConfig(configDir)
	cfg.Server.HTTP.Host = "127.0.0.1"
	cfg.Server.HTTP.Port = 0
	if err := conf.WriteConfig(cfg, configPath); err != nil {
		t.Fatal(err)
	}
	first, stop := startTestProcess(t, dataDir)
	checkDesktopListeningPort(t, first)
	checkDesktopBrowser(t, dataDir, first)
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
	if _, err := os.Stat(filepath.Join(configDir, "data.db")); err != nil {
		t.Fatalf("默认 SQLite 文件不存在: %v", err)
	}
	address := strings.TrimPrefix(first.URL, "http://")
	_, port, err := net.SplitHostPort(address)
	if err != nil {
		t.Fatal(err)
	}
	cfg.Server.HTTP.Port, err = strconv.Atoi(port)
	if err != nil {
		t.Fatal(err)
	}
	if err := conf.WriteConfig(cfg, configPath); err != nil {
		t.Fatal(err)
	}
	originalConfig, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}

	for _, test := range []struct {
		name     string
		occupied bool
	}{
		{name: "configured port"},
		{name: "occupied port", occupied: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			if test.occupied {
				listener, err := net.Listen("tcp", address)
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() {
					if err := listener.Close(); err != nil {
						t.Error(err)
					}
				})
			}
			second, _ := startTestProcess(t, dataDir)
			checkDesktopListeningPort(t, second)
			checkDesktopBrowser(t, dataDir, second)
			if (second.URL != first.URL) != test.occupied {
				t.Fatalf("桌面访问地址错误: 配置 %q, 实际 %q", first.URL, second.URL)
			}
			data := request(t, second.URL, http.MethodGet, fmt.Sprintf("/api/v1/cards/%d", card.ID), "")
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
			if current, err := os.ReadFile(configPath); err != nil || !bytes.Equal(current, originalConfig) {
				t.Fatalf("桌面服务修改了原配置: %v", err)
			}
		})
	}
}

func checkDesktopListeningPort(t *testing.T, ready serviceReady) {
	t.Helper()
	origin, err := url.Parse(ready.URL)
	if err != nil {
		t.Fatal(err)
	}
	if ready.Port <= 0 || strconv.Itoa(ready.Port) != origin.Port() {
		t.Fatalf("设置显示的监听端口 %d 与访问地址 %q 不符", ready.Port, ready.URL)
	}
}

func checkDesktopBrowser(t *testing.T, dataDir string, ready serviceReady) {
	t.Helper()
	opened := filepath.Join(dataDir, openedBrowserFile)
	for _, tt := range []struct {
		name, method, origin, header string
		want                         int
	}{
		{name: "wrong method", method: http.MethodGet, origin: ready.URL, header: "browser", want: http.StatusNotFound},
		{name: "foreign origin", method: http.MethodPost, origin: "https://example.org", header: "browser", want: http.StatusNotFound},
		{name: "no origin", method: http.MethodPost, header: "browser", want: http.StatusNotFound},
		{name: "no desktop header", method: http.MethodPost, origin: ready.URL, want: http.StatusNotFound},
		{name: "desktop click", method: http.MethodPost, origin: ready.URL, header: "browser", want: http.StatusNoContent},
	} {
		t.Run(tt.name, func(t *testing.T) {
			// Even a supplied URL cannot override the listening instance's URL.
			req, err := http.NewRequestWithContext(t.Context(), tt.method, ready.URL+ready.BrowserPath+"?url=https://example.org", nil)
			if err != nil {
				t.Fatal(err)
			}
			req.Header.Set("Origin", tt.origin)
			req.Header.Set("X-Slate-Desktop", tt.header)
			res, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			defer res.Body.Close()
			if res.StatusCode != tt.want {
				t.Fatalf("browser request status = %d, want %d", res.StatusCode, tt.want)
			}
			actual, err := os.ReadFile(opened)
			if tt.want == http.StatusNoContent {
				if err != nil || string(actual) != ready.URL {
					t.Fatalf("opened %q, want %q (%v)", actual, ready.URL, err)
				}
				if err := os.Remove(opened); err != nil {
					t.Fatal(err)
				}
			} else if !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("rejected request set the target to %q (%v)", actual, err)
			}
		})
	}
}

func TestDesktopStartupRejectsInvalidPort(t *testing.T) {
	cfg := conf.DefaultConfig(t.TempDir())
	cfg.Server.HTTP.Host = "127.0.0.1"
	cfg.Server.HTTP.Port = -1
	s, err := startService(cfg, app.BuildInfo{}, nil, nil)
	if err == nil {
		t.Cleanup(func() {
			if err := s.close(); err != nil {
				t.Error(err)
			}
		})
		t.Fatal("端口配置无效时桌面服务仍启动")
	}
	if addrErr, ok := errors.AsType[*net.AddrError](err); !ok || addrErr.Addr != "-1" {
		t.Fatalf("未返回原始的端口配置错误: %v", err)
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
	s, err := startService(cfg, build, nil, func(_ context.Context, target string) error {
		return os.WriteFile(filepath.Join(dataDir, openedBrowserFile), []byte(target), 0o600)
	})
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
	payload, err := json.Marshal(serviceReady{
		URL: s.url, Port: s.connection.port, BrowserPath: s.connection.openPath,
		ConnectionScript: s.connection.script(s.url), Recent: recent,
	})
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
			t.Log(scanner.Text())
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
