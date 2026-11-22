package updateapi

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

// TestNewer 验证版本比较：只比三段数字，带 v 或不带都行，预发布后缀忽略，解析不了一律按「不新」。
func TestNewer(t *testing.T) {
	cases := []struct {
		latest, current string
		want            bool
	}{
		{"v1.2.4", "v1.2.3", true},
		{"v1.3.0", "v1.2.9", true},
		{"v2.0.0", "v1.99.99", true},
		{"v1.2.3", "v1.2.3", false},
		{"v1.2.3", "v1.2.4", false},
		{"v1.10.0", "v1.9.0", true},
		{"1.2.4", "v1.2.3", true},
		{"v1.2.4-rc1", "v1.2.3+abc", true},
		{"v1.2", "v1.2.3", false},
		{"latest", "v1.2.3", false},
		{"v1.2.3", "dev", false},
		{"v1.2.3", "", false},
		{"v-1.2.3", "v1.2.2", false},
	}
	for _, c := range cases {
		if got := newer(c.latest, c.current); got != c.want {
			t.Errorf("newer(%q, %q) = %v，期望 %v", c.latest, c.current, got, c.want)
		}
	}
}

// fakeGitHub 是一个假的 GitHub：release 信息、包和校验文件都由它提供，并统计 release 接口被请求的次数。
type fakeGitHub struct {
	*httptest.Server
	releaseHits atomic.Int32
	noAssets    atomic.Bool       // 为真时 release 里没有任何文件，模拟发布工作流还没传完包
	status      int               // release 接口的状态码，0 表示 200
	files       map[string][]byte // 路径 → 内容
	tag         string
	goos        string
}

// newFakeGitHub 起一个假 GitHub，发布 tag 版本，包的平台是 goos/amd64，包里的程序内容是 binary。
func newFakeGitHub(t *testing.T, tag, goos, binary string) *fakeGitHub {
	t.Helper()
	g := &fakeGitHub{tag: tag, goos: goos, files: map[string][]byte{}}
	// 新版发布包把文件放在同名的目录里，测试也用这种布局；旧版平铺包另由 TestExtractBinary 覆盖
	name := fmt.Sprintf("slate_%s_%s_amd64.tar.gz", tag, goos)
	bin := "slate"
	pack := tarGz(t, strings.TrimSuffix(name, ".tar.gz")+"/"+bin, binary)
	if goos == "windows" {
		name = fmt.Sprintf("slate_%s_windows_amd64.zip", tag)
		bin = "slate.exe"
		pack = zipOf(t, strings.TrimSuffix(name, ".zip")+"/"+bin, binary)
	}
	sum := sha256.Sum256(pack)
	g.files["/dl/"+tag+"/"+name] = pack
	g.files["/dl/"+tag+"/checksums.txt"] = []byte(hex.EncodeToString(sum[:]) + "  " + name + "\n")
	mux := http.NewServeMux()
	// 只提供 github.com 网页端点，没有 /repos/... 的 API：代码要是还去访问 API，会得到 404
	mux.HandleFunc("/"+repo+"/releases/latest", func(w http.ResponseWriter, r *http.Request) {
		g.releaseHits.Add(1)
		if g.status != 0 {
			w.WriteHeader(g.status)
			return
		}
		http.Redirect(w, r, "http://"+g.Listener.Addr().String()+"/"+repo+"/releases/tag/"+tag, http.StatusFound)
	})
	mux.HandleFunc("/"+repo+"/releases.atom", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprintf(w, `<?xml version="1.0" encoding="UTF-8"?><feed xmlns="http://www.w3.org/2005/Atom">`+
			`<entry><link rel="alternate" type="text/html" href="https://github.com/%s/releases/tag/%s"/>`+
			`<title>%s</title><content type="html">&lt;p&gt;更新说明&lt;/p&gt;

&lt;p&gt;第二段 &amp;amp; 符号&lt;/p&gt;</content></entry>`+
			`<entry><updated>2000-01-01T00:00:00Z</updated><link rel="alternate" type="text/html" href="https://github.com/%s/releases/tag/v0.0.1"/>`+
			`<title>v0.0.1</title><content>No content.</content></entry></feed>`, repo, tag, tag, repo)
	})
	mux.HandleFunc("/dl/", func(w http.ResponseWriter, r *http.Request) {
		data, ok := g.files[r.URL.Path]
		if !ok || g.noAssets.Load() {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write(data)
	})
	g.Server = httptest.NewServer(mux)
	t.Cleanup(g.Close)
	return g
}

// tarGz 生成只含一个文件的 tar.gz。
func tarGz(t *testing.T, name, content string) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o755, Size: int64(len(content)), Typeflag: tar.TypeReg}); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write([]byte(content)); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// zipOf 生成只含一个文件的 zip。
func zipOf(t *testing.T, name, content string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.Create(name)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write([]byte(content)); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// TestExtractBinary 验证两种包布局都能取出程序：新版放在 slate_<tag>_<系统>_<架构>/ 目录里，
// 旧版平铺在包根目录；已经发出去的旧包仍要能自动更新。
func TestExtractBinary(t *testing.T) {
	cases := []struct {
		name string
		pack []byte
		bin  string
	}{
		{"tar.gz 目录包", tarGz(t, "slate_v1.2.0_linux_amd64/slate", "NEW"), "slate"},
		{"tar.gz 平铺旧包", tarGz(t, "slate", "NEW"), "slate"},
		{"zip 目录包", zipOf(t, "slate_v1.2.0_windows_amd64/slate.exe", "NEW"), "slate.exe"},
		{"zip 平铺旧包", zipOf(t, "slate.exe", "NEW"), "slate.exe"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p := filepath.Join(t.TempDir(), "pack")
			if err := os.WriteFile(p, c.pack, 0o644); err != nil {
				t.Fatal(err)
			}
			got, err := extractBinary(p, c.bin, t.TempDir())
			if err != nil {
				t.Fatalf("取出程序失败: %v", err)
			}
			data, err := os.ReadFile(got)
			if err != nil || string(data) != "NEW" {
				t.Fatalf("程序内容不对: %q %v", data, err)
			}
		})
	}
}

// newAPI 用假 GitHub 和一个临时目录里的「程序」创建接口，返回接口和程序路径。
func newAPI(t *testing.T, g *fakeGitHub, current string, mod func(*Options)) (API, string) {
	t.Helper()
	exe := filepath.Join(t.TempDir(), "slate")
	if g.goos == "windows" {
		exe += ".exe"
	}
	if err := os.WriteFile(exe, []byte("old"), 0o755); err != nil {
		t.Fatal(err)
	}
	opt := Options{
		Current: current, WebBase: g.URL, DownloadPrefix: g.URL + "/dl/",
		ExePath: exe, GOOS: g.goos, GOARCH: "amd64",
	}
	if mod != nil {
		mod(&opt)
	}
	return New(opt), exe
}

// do 通过真实路由发请求，返回状态码和响应体。
func do(a API, method string) (int, []byte) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	Register(r.Group("/api/v1"), a)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(method, "/api/v1/update", http.NoBody))
	return rec.Code, rec.Body.Bytes()
}

// infoOf 取 GET /update 的数据部分。
func infoOf(t *testing.T, a API) Info {
	t.Helper()
	code, body := do(a, http.MethodGet)
	if code != http.StatusOK {
		t.Fatalf("GET /update 应 200: %d %s", code, body)
	}
	var out Info
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatalf("解析响应: %v %s", err, body)
	}
	return out
}

// TestCheck 验证查询：有新版本时给出说明和可更新；同版本没有更新；GitHub 失败不报错且短时间内不重复请求；
// 结果会缓存，开发版本和容器里不查询或不可更新。
func TestCheck(t *testing.T) {
	t.Run("有新版本", func(t *testing.T) {
		g := newFakeGitHub(t, "v1.2.0", "linux", "new")
		a, _ := newAPI(t, g, "v1.1.0", nil)
		got := infoOf(t, a)
		if !got.HasUpdate || !got.CanApply || got.Latest != "v1.2.0" || got.Notes != "更新说明\n\n第二段 & 符号" || got.Current != "v1.1.0" {
			t.Fatalf("信息不对: %+v", got)
		}
		infoOf(t, a)
		if n := g.releaseHits.Load(); n != 1 {
			t.Fatalf("10 秒内应该走缓存，实际请求 %d 次", n)
		}
		a.u.cachedAt = time.Now().Add(-11 * time.Second)
		infoOf(t, a)
		if n := g.releaseHits.Load(); n != 2 {
			t.Fatalf("缓存过期后应重新请求，实际请求 %d 次", n)
		}
	})
	t.Run("已是最新", func(t *testing.T) {
		g := newFakeGitHub(t, "v1.2.0", "linux", "new")
		a, _ := newAPI(t, g, "v1.2.0", nil)
		if got := infoOf(t, a); got.HasUpdate || got.CanApply {
			t.Fatalf("同版本不应提示: %+v", got)
		}
	})
	t.Run("GitHub 失败不报错且不重复请求", func(t *testing.T) {
		g := newFakeGitHub(t, "v1.2.0", "linux", "new")
		g.status = http.StatusForbidden
		a, _ := newAPI(t, g, "v1.1.0", nil)
		if got := infoOf(t, a); got.HasUpdate || got.Latest != "" {
			t.Fatalf("失败时应按没有更新返回: %+v", got)
		}
		infoOf(t, a)
		if n := g.releaseHits.Load(); n != 1 {
			t.Fatalf("失败后短时间内不应再请求，实际 %d 次", n)
		}
	})
	t.Run("包还没传完不提示也不能更新；包一传完就提示，和发布多久无关", func(t *testing.T) {
		g := newFakeGitHub(t, "v1.2.0", "linux", "NEW")
		g.noAssets.Store(true)
		a, exe := newAPI(t, g, "v1.1.0", nil)
		got := infoOf(t, a)
		if got.HasUpdate || got.CanApply || got.Latest != "v1.2.0" || got.CheckState != CheckStateOK {
			t.Fatalf("包没传完不该提示更新: %+v", got)
		}
		if code, _ := do(a, http.MethodPost); code == http.StatusOK {
			t.Fatal("包没传完时不该能更新")
		}
		if data, _ := os.ReadFile(exe); string(data) != "old" {
			t.Fatalf("被拒绝时不能动现有程序，实际 %q", data)
		}
		g.noAssets.Store(false)
		a.u.cachedAt = time.Now().Add(-11 * time.Second)
		if got := infoOf(t, a); !got.HasUpdate || !got.CanApply {
			t.Fatalf("包传完后应立即提示并可更新: %+v", got)
		}
	})
	t.Run("check_state 区分成功与 GitHub 失败", func(t *testing.T) {
		g := newFakeGitHub(t, "v1.2.0", "linux", "new")
		a, _ := newAPI(t, g, "v1.1.0", nil)
		if got := infoOf(t, a); got.CheckState != CheckStateOK {
			t.Fatalf("成功查询应为 ok: %+v", got)
		}
		g403 := newFakeGitHub(t, "v1.2.0", "linux", "new")
		g403.status = http.StatusForbidden
		a403, _ := newAPI(t, g403, "v1.1.0", nil)
		if got := infoOf(t, a403); got.CheckState != CheckStateUnreachable {
			t.Fatalf("GitHub 失败应为 unreachable: %+v", got)
		}
	})
	t.Run("release 还没传完包时只短暂缓存，包传完后马上能更新", func(t *testing.T) {
		g := newFakeGitHub(t, "v1.2.0", "linux", "new")
		g.noAssets.Store(true)
		a, _ := newAPI(t, g, "v1.1.0", nil)
		if got := infoOf(t, a); got.HasUpdate || got.CanApply {
			t.Fatalf("没有包时不应提示更新: %+v", got)
		}
		infoOf(t, a)
		if n := g.releaseHits.Load(); n != 1 {
			t.Fatalf("短缓存期内不应重复请求，实际 %d 次", n)
		}
		g.noAssets.Store(false)
		a.u.cachedAt = time.Now().Add(-11 * time.Second)
		if got := infoOf(t, a); !got.HasUpdate || !got.CanApply {
			t.Fatalf("短缓存过期后应读到新上传的包: %+v", got)
		}
	})
	t.Run("容器里只提示不能更新", func(t *testing.T) {
		g := newFakeGitHub(t, "v1.2.0", "linux", "new")
		a, _ := newAPI(t, g, "v1.1.0", func(o *Options) { o.Container = true })
		if got := infoOf(t, a); !got.HasUpdate || got.CanApply {
			t.Fatalf("容器里应有更新但不能立即更新: %+v", got)
		}
	})
}

// TestApply 验证更新：类 Unix 和 windows 都能下载、校验、替换程序，替换后不再提示更新；
// 校验不过、非官方、容器、已是最新都被拒绝，并且不改动现有程序、不留下临时文件。
func TestApply(t *testing.T) {
	for _, goos := range []string{"linux", "windows"} {
		t.Run(goos+"替换成功", func(t *testing.T) {
			g := newFakeGitHub(t, "v1.2.0", goos, "NEW-BINARY")
			a, exe := newAPI(t, g, "v1.1.0", nil)
			code, body := do(a, http.MethodPost)
			if code != http.StatusOK || !strings.Contains(string(body), "v1.2.0") {
				t.Fatalf("更新应成功: %d %s", code, body)
			}
			if got, _ := os.ReadFile(exe); string(got) != "NEW-BINARY" {
				t.Fatalf("程序内容应被替换，实际 %q", got)
			}
			assertNoTemp(t, filepath.Dir(exe), goos == "windows")
			if got := infoOf(t, a); got.HasUpdate || got.CanApply {
				t.Fatalf("更新后不应再提示: %+v", got)
			}
		})
	}
	t.Run("替换成功后请求重启，响应带 restarting", func(t *testing.T) {
		g := newFakeGitHub(t, "v1.2.0", "linux", "NEW")
		var calls []string
		a, exe := newAPI(t, g, "v1.1.0", func(o *Options) {
			o.Restart = func(path string) bool { calls = append(calls, path); return true }
		})
		code, body := do(a, http.MethodPost)
		if code != http.StatusOK || !strings.Contains(string(body), `"restarting":true`) {
			t.Fatalf("响应应带 restarting=true: %d %s", code, body)
		}
		want, _ := filepath.EvalSymlinks(exe)
		if len(calls) != 1 || calls[0] != want {
			t.Fatalf("应带着程序路径请求一次重启，期望 %q，实际 %v", want, calls)
		}
	})
	t.Run("环境不支持重启时 restarting 为假", func(t *testing.T) {
		g := newFakeGitHub(t, "v1.2.0", "linux", "NEW")
		for name, restart := range map[string]func(string) bool{
			"没有重启函数": nil,
			"重启函数拒绝": func(string) bool { return false },
		} {
			a, _ := newAPI(t, g, "v1.1.0", func(o *Options) { o.Restart = restart })
			code, body := do(a, http.MethodPost)
			if code != http.StatusOK || !strings.Contains(string(body), `"restarting":false`) {
				t.Errorf("%s：更新应成功且 restarting=false: %d %s", name, code, body)
			}
		}
	})
	t.Run("更新失败不请求重启", func(t *testing.T) {
		g := newFakeGitHub(t, "v1.2.0", "linux", "NEW")
		g.files["/dl/v1.2.0/checksums.txt"] = []byte(strings.Repeat("0", 64) + "  slate_v1.2.0_linux_amd64.tar.gz\n")
		called := false
		a, _ := newAPI(t, g, "v1.1.0", func(o *Options) { o.Restart = func(string) bool { called = true; return true } })
		if code, _ := do(a, http.MethodPost); code == http.StatusOK {
			t.Fatal("校验和不对应该失败")
		}
		if called {
			t.Fatal("更新失败时不该请求重启")
		}
	})
	t.Run("校验和不对", func(t *testing.T) {
		g := newFakeGitHub(t, "v1.2.0", "linux", "NEW")
		g.files["/dl/v1.2.0/checksums.txt"] = []byte(strings.Repeat("0", 64) + "  slate_v1.2.0_linux_amd64.tar.gz\n")
		a, exe := newAPI(t, g, "v1.1.0", nil)
		if code, _ := do(a, http.MethodPost); code == http.StatusOK {
			t.Fatal("校验和不对应该失败")
		}
		if got, _ := os.ReadFile(exe); string(got) != "old" {
			t.Fatalf("失败时不能动现有程序，实际 %q", got)
		}
		assertNoTemp(t, filepath.Dir(exe), false)
	})
	t.Run("被拒绝的情形", func(t *testing.T) {
		for name, mod := range map[string]func(*Options){
			"容器":  func(o *Options) { o.Container = true },
			"已最新": func(o *Options) { o.Current = "v1.2.0" },
		} {
			g := newFakeGitHub(t, "v1.2.0", "linux", "NEW")
			a, exe := newAPI(t, g, "v1.1.0", mod)
			if code, _ := do(a, http.MethodPost); code == http.StatusOK {
				t.Errorf("%s 应该被拒绝", name)
			}
			if got, _ := os.ReadFile(exe); string(got) != "old" {
				t.Errorf("%s：被拒绝时不能动现有程序", name)
			}
		}
	})
}

// assertNoTemp 验证目录里没有残留的下载和解压临时文件；windows 允许留下 .old。
func assertNoTemp(t *testing.T, dir string, allowOld bool) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		n := e.Name()
		if strings.HasPrefix(n, ".slate-") || (!allowOld && strings.HasSuffix(n, ".old")) {
			t.Errorf("残留了临时文件 %s", n)
		}
	}
}

// TestChecksumFor 验证解析 sha256sum 文本：能找到指定文件，认带星号的二进制模式，找不到返回 false。
func TestChecksumFor(t *testing.T) {
	sums := []byte("aaa  one.tar.gz\nbbb *two.zip\n\nbad line\n")
	for name, want := range map[string]string{"one.tar.gz": "aaa", "two.zip": "bbb"} {
		if got, ok := checksumFor(sums, name); !ok || got != want {
			t.Errorf("%s: 得到 %q %v", name, got, ok)
		}
	}
	if _, ok := checksumFor(sums, "three.zip"); ok {
		t.Error("不存在的文件不应找到")
	}
}
