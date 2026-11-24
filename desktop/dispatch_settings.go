package desktop

import (
	"context"
	"crypto/rand"
	"encoding/json/v2"
	"errors"
	"fmt"
	"maps"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"sync"

	"slate-kanban/internal/core/kanban"
)

type desktopDispatch struct {
	core  dispatchCore
	host  dispatchHost
	text  func() desktopText
	path  string
	file  string
	actor kanban.Actor

	control sync.Mutex
	closed  bool
	cancel  context.CancelFunc
	done    chan struct{}
	mu      sync.Mutex
	config  dispatchConfig
	status  map[int64]engineerStatus
	failure string
}

type dispatchState struct {
	Config    dispatchConfig           `json:"config"`
	Hosts     []availableHost          `json:"hosts"`
	Status    map[int64]engineerStatus `json:"status"`
	Failure   string                   `json:"failure,omitempty"`
	Engineers []*kanban.Engineer       `json:"engineers"`
	Template  bool                     `json:"template"`
}

func newDesktopDispatch(core dispatchCore, dir string, text func() desktopText) *desktopDispatch {
	d := &desktopDispatch{
		core: core, host: localDispatchHost, text: text,
		path: "/.slate-desktop/" + rand.Text(), file: filepath.Join(dir, "desktop-dispatch.json"),
		status: make(map[int64]engineerStatus),
	}
	raw, err := os.ReadFile(d.file)
	if errors.Is(err, os.ErrNotExist) {
		return d
	}
	if err != nil || json.Unmarshal(raw, &d.config, json.RejectUnknownMembers(true)) != nil {
		d.config = dispatchConfig{}
		d.failure = "settings"
		return d
	}
	if d.config.Enabled {
		snapshot, err := d.validate(context.Background(), d.config)
		if err != nil {
			d.failure = "startup"
		} else {
			d.start(snapshot)
		}
	}
	return d
}

// dispatchSettingsError 是保存派发设置时可以告诉用户的失败原因。
// code 是稳定的英文标识，前端据此显示对应语言的具体提示；message 只给日志和排查用。
type dispatchSettingsError struct{ code, message string }

func (e *dispatchSettingsError) Error() string { return e.message }

// dispatchSettingsCode 取出错误对应的原因标识，没有归类的错误返回空串。
func dispatchSettingsCode(err error) string {
	if se, ok := errors.AsType[*dispatchSettingsError](err); ok {
		return se.code
	}
	if errors.Is(err, errDispatchTemplate) {
		return "template"
	}
	if errors.Is(err, errHostUnavailable) {
		return "host"
	}
	return ""
}

// validate 在启用前核对看板、模板、项目路径和宿主，让不可用的配置在保存时就失败，
// 而不是等到真正派发才发现。
func (d *desktopDispatch) validate(ctx context.Context, cfg dispatchConfig) (*kanban.Snapshot, error) {
	if cfg.BoardID <= 0 {
		return nil, &dispatchSettingsError{"board", "board is required"}
	}
	snapshot, err := d.core.BoardSnapshot(ctx, cfg.BoardID)
	if err != nil {
		return nil, &dispatchSettingsError{"board", "board is unavailable"}
	}
	if _, err := templateFlow(snapshot.Columns); err != nil {
		return nil, err
	}
	if !cfg.Enabled {
		return snapshot, nil
	}
	if len(cfg.Hosts) == 0 {
		return nil, &dispatchSettingsError{"host", "enabled dispatch needs at least one host"}
	}
	if !filepath.IsAbs(snapshot.ProjectPath) {
		return nil, &dispatchSettingsError{"project", "enabled dispatch needs an absolute Git project path"}
	}
	if _, err := os.Stat(snapshot.ProjectPath); err != nil {
		return nil, &dispatchSettingsError{"project", "board project path is unavailable"}
	}
	ids := make(map[int64]bool)
	for _, engineer := range snapshot.Engineers {
		ids[engineer.ID] = true
	}
	for id, host := range cfg.Hosts {
		if !ids[id] {
			return nil, &dispatchSettingsError{"engineer", "engineer is no longer on the board"}
		}
		if _, err := lookupDesktopHost(host); err != nil {
			return nil, err
		}
	}
	return snapshot, nil
}

func (d *desktopDispatch) apply(ctx context.Context, cfg dispatchConfig) error {
	d.control.Lock()
	defer d.control.Unlock()
	if d.closed {
		return errors.New("desktop dispatch is shutting down")
	}
	d.mu.Lock()
	same := cfg.BoardID == d.config.BoardID && cfg.Enabled == d.config.Enabled && maps.Equal(cfg.Hosts, d.config.Hosts) && d.failure == ""
	d.mu.Unlock()
	if same {
		return nil
	}
	var snapshot *kanban.Snapshot
	if cfg.Enabled {
		var err error
		snapshot, err = d.validate(ctx, cfg)
		if err != nil {
			return err
		}
	}
	if err := saveDispatchConfig(d.file, cfg); err != nil {
		return &dispatchSettingsError{"save", "could not save desktop settings"}
	}
	d.stop()
	d.mu.Lock()
	d.config = cfg
	d.status = make(map[int64]engineerStatus)
	d.failure = ""
	d.mu.Unlock()
	if cfg.Enabled {
		d.start(snapshot)
	}
	return nil
}

func (d *desktopDispatch) start(snapshot *kanban.Snapshot) {
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	d.cancel, d.done = cancel, done
	run := &dispatchRun{
		core: d.core, config: d.config, host: d.host, text: d.text,
		sessions: filepath.Join(filepath.Dir(d.file), "desktop-dispatch-sessions"),
		update: func(id int64, status engineerStatus) {
			d.mu.Lock()
			d.status[id] = status
			d.mu.Unlock()
		},
	}
	go func() {
		defer close(done)
		run.run(ctx, snapshot)
		if ctx.Err() == nil {
			d.mu.Lock()
			d.failure = "stopped"
			d.mu.Unlock()
		}
	}()
}

func (d *desktopDispatch) stop() {
	if d.cancel != nil {
		d.cancel()
		<-d.done
		d.cancel, d.done = nil, nil
	}
}

func (d *desktopDispatch) close() {
	d.control.Lock()
	defer d.control.Unlock()
	d.closed = true
	d.stop()
}

func saveDispatchConfig(file string, cfg dispatchConfig) error {
	raw, err := json.Marshal(cfg)
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(file), ".dispatch-settings-")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	_, err = tmp.Write(raw)
	if err = errors.Join(err, tmp.Close()); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), file)
}

func (d *desktopDispatch) state(ctx context.Context, boardID int64) dispatchState {
	d.mu.Lock()
	state := dispatchState{Config: d.config, Failure: d.failure, Status: make(map[int64]engineerStatus), Engineers: []*kanban.Engineer{}}
	for id, status := range d.status {
		state.Status[id] = status
	}
	d.mu.Unlock()
	for _, host := range desktopHosts {
		_, err := lookupDesktopHost(host.id)
		state.Hosts = append(state.Hosts, availableHost{host.id, host.name, err == nil})
	}
	if boardID == 0 {
		boardID = state.Config.BoardID
	}
	if boardID > 0 {
		if snapshot, err := d.core.BoardSnapshot(ctx, boardID); err == nil {
			state.Engineers = snapshot.Engineers
			_, err := templateFlow(snapshot.Columns)
			state.Template = err == nil
		}
	}
	return state
}

func (d *desktopDispatch) script(origin string) string {
	return fmt.Sprintf(`(() => {
		if (location.origin !== %q) return;
		window.slateDesktopDispatch = %q;
		window.dispatchEvent(new Event("slate-desktop-dispatch"));
	})()`, origin, d.path)
}

func (d *desktopDispatch) wrap(origin string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.URL.Path != d.path {
			next.ServeHTTP(w, req)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Type", "application/json")
		// A bare website does not receive the capability path. Also reject
		// cross-origin requests, including writes from embedded sites.
		if req.Header.Get("X-Slate-Desktop") != "dispatch" ||
			(req.Method != http.MethodGet && req.Header.Get("Origin") != origin) ||
			(req.Header.Get("Origin") != "" && req.Header.Get("Origin") != origin) {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}
		switch req.Method {
		case http.MethodGet:
			boardID, _ := strconv.ParseInt(req.URL.Query().Get("board_id"), 10, 64)
			_ = json.MarshalWrite(w, d.state(req.Context(), boardID))
		case http.MethodPut:
			var cfg dispatchConfig
			if err := json.UnmarshalRead(http.MaxBytesReader(w, req.Body, 32<<10), &cfg, json.RejectUnknownMembers(true)); err != nil {
				http.Error(w, "Invalid settings", http.StatusBadRequest)
				return
			}
			if err := d.apply(req.Context(), cfg); err != nil {
				w.WriteHeader(http.StatusBadRequest)
				_ = json.MarshalWrite(w, map[string]string{"error": err.Error(), "code": dispatchSettingsCode(err)})
				return
			}
			_ = json.MarshalWrite(w, d.state(req.Context(), cfg.BoardID))
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})
}
