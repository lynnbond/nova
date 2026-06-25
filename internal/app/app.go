// Package app wires the entire Nova self-contained application.
package app

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"embed"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"mime"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/liyan/nova"
	"github.com/liyan/nova/sqlite"
)

//go:embed webdist/*
var webFS embed.FS

// Default JWT secret (dev only — override via Config.JWTSecret).
const defaultJWTSecret = "nova-dev-secret-do-not-use-in-production"

// Config configures the Nova application.
type Config struct {
	DBPath    string
	JWTSecret string
}

// App is the self-contained Nova workflow application.
type App struct {
	mu         sync.Mutex
	cfg        Config
	st         *sqlite.Store
	mux        *http.ServeMux
	srv        *http.Server
	engines    map[string]*nova.Engine // alias → engine
	engByProID map[string]*nova.Engine // pro ID → engine
	proAliases []string               // ordered list of known process aliases
	jwtSecret  []byte
}

func New(cfg Config) (*App, error) {
	st, err := sqlite.Open(sqlite.Config{
		DSN: "file:" + cfg.DBPath + "?cache=shared&_journal_mode=WAL&_busy_timeout=5000",
	})
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}

	// Migrate existing DBs to add password_hash column
	_ = st.MigrateAddPasswordColumn()

	jwtSecret := cfg.JWTSecret
	if jwtSecret == "" {
		jwtSecret = defaultJWTSecret
	}

	a := &App{
		cfg:        cfg,
		st:         st,
		mux:        http.NewServeMux(),
		jwtSecret:  []byte(jwtSecret),
		engines:    make(map[string]*nova.Engine),
		engByProID: make(map[string]*nova.Engine),
	}
	if err := a.seed(); err != nil {
		return nil, fmt.Errorf("seed: %w", err)
	}

	// Build engines for all seeded processes
	ctx := context.Background()
	allPros, err := st.ListPros(ctx)
	if err != nil {
		return nil, fmt.Errorf("list pros: %w", err)
	}
	for _, pro := range allPros {
		eng, err := nova.NewEngine(ctx, nova.EngineConfig{Store: st, ProAlias: pro.Alias})
		if err != nil {
			return nil, fmt.Errorf("create engine for %s: %w", pro.Alias, err)
		}
		a.engines[pro.Alias] = eng
		a.engByProID[eng.Pro().ID] = eng
	}

	a.registerRoutes()
	return a, nil
}

func (a *App) Store() nova.Store { return a.st }

// engineForEntity finds the correct engine for an entity by looking up
// the entity's ProID, finding the corresponding Pro, and returning the engine.
func (a *App) engineForEntity(ctx context.Context, entityID string) (*nova.Engine, error) {
	ent, err := a.st.GetEntity(ctx, entityID)
	if err != nil {
		return nil, fmt.Errorf("get entity: %w", err)
	}
	if ent == nil {
		return nil, fmt.Errorf("entity not found")
	}
	eng, ok := a.engByProID[ent.ProID]
	if ok {
		return eng, nil
	}
	// Fallback: look up pro by ID
	pro, err := a.st.GetPro(ctx, ent.ProID)
	if err != nil {
		return nil, fmt.Errorf("get pro: %w", err)
	}
	if pro == nil {
		return nil, fmt.Errorf("process not found for entity")
	}
	eng, ok = a.engines[pro.Alias]
	if !ok {
		return nil, fmt.Errorf("engine not found for process %s", pro.Alias)
	}
	a.engByProID[ent.ProID] = eng
	return eng, nil
}

func (a *App) Serve(addr string) error {
	a.srv = &http.Server{Addr: addr, Handler: a.mux}
	log.Printf("Nova server listening on %s", addr)
	return a.srv.ListenAndServe()
}

func (a *App) Close() {
	if a.srv != nil {
		_ = a.srv.Shutdown(context.Background())
	}
	_ = a.st.Close()
}

// ─── Seed ──────────────────────────────────────────────────────────────────────

func (a *App) seed() error {
	ctx := context.Background()

	// ── OP 流程（基础单线审批） ──
	proOP, _ := a.st.GetProByAlias(ctx, "OP")
	if proOP == nil {
		log.Println("Seeding OP workflow...")
		_, err := nova.NewProcess("OP", "OP申请单").
			Act("start", nova.ActTypeSTART).
			Act("draft", nova.ActTypeMANUAL).Title("起草申请").On(nova.ManPolicySINGLE).
			Act("approve", nova.ActTypeMANUAL).Title("OP审批").On(nova.ManPolicySINGLE).
			Act("end", nova.ActTypeEND).
			Link("start", "draft").
			Link("draft", "approve").
			Link("approve", "end").
			Store(ctx, a.st)
		if err != nil {
			return fmt.Errorf("OP process builder store: %w", err)
		}
		log.Printf("✅ OP workflow seeded")
	}
	a.proAliases = append(a.proAliases, "OP")

	// ── DEMO 流程（并发 + 汇聚 + 等待，全特性展示） ──
	// 流程图：
	//   start → draft
	//     → router (ROUTER 并行分发)
	//         ├── concur_a (MANUAL CONCURRENT 会签A: 张三+李四)
	//         ├── concur_b (MANUAL CONCURRENT 会签B: 张三+李四)
	//         └── single_c (MANUAL SINGLE 单签C: 王五)
	//     → converge (CONVERGE 汇聚: 等三条分支全到)
	//     → wait_item (WAIT 等待确认: WaitFor single_c)
	//     → fin_signing (MANUAL SINGLE 终审: admin)
	//     → end
	proDEMO, _ := a.st.GetProByAlias(ctx, "DEMO")
	if proDEMO == nil {
		log.Println("Seeding DEMO workflow...")
		_, err := nova.NewProcess("DEMO", "综合审批Demo").
			Act("start", nova.ActTypeSTART).
			Act("draft", nova.ActTypeMANUAL).Title("起草申请").On(nova.ManPolicySINGLE).
			Act("router1", nova.ActTypeROUTER).
			Act("concur_a", nova.ActTypeMANUAL).Title("会签A").On(nova.ManPolicyCONCURRENT).
			Act("concur_b", nova.ActTypeMANUAL).Title("会签B").On(nova.ManPolicyCONCURRENT).
			Act("single_c", nova.ActTypeMANUAL).Title("单签C").On(nova.ManPolicySINGLE).
			Act("converge1", nova.ActTypeCONVERGE).
			Act("wait_item", nova.ActTypeWAIT).Title("等待外部确认").WaitFor("single_c").
			Act("fin_signing", nova.ActTypeMANUAL).Title("终审签字").On(nova.ManPolicySINGLE).
			Act("end", nova.ActTypeEND).
			Link("start", "draft").
			Link("draft", "router1").
			Link("router1", "concur_a").
			Link("router1", "concur_b").
			Link("router1", "single_c").
			Link("concur_a", "converge1").
			Link("concur_b", "converge1").
			Link("single_c", "converge1").
			Link("converge1", "wait_item").
			Link("wait_item", "fin_signing").
			Link("fin_signing", "end").
			Store(ctx, a.st)
		if err != nil {
			return fmt.Errorf("DEMO process builder store: %w", err)
		}
		log.Printf("✅ DEMO workflow seeded (ROUTER+CONVERGE+WAIT)")
	}
	a.proAliases = append(a.proAliases, "DEMO")

	// Seed demo users
	ctxBag := context.Background()
	count, _ := a.st.ListUsers(ctxBag)
	if len(count) == 0 {
		users := []*nova.User{
			{UID: "admin", Name: "管理员", Dept: "管理部"},
			{UID: "zhangsan", Name: "张三", Dept: "审批部"},
			{UID: "lisi", Name: "李四", Dept: "审批部"},
			{UID: "wangwu", Name: "王五", Dept: "业务部"},
		}
		for _, u := range users {
			if err := a.st.CreateUser(context.Background(), u); err != nil {
				log.Printf("⚠️  seed user %s: %v", u.UID, err)
			} else {
				log.Printf("✅ Seeded user %s (%s)", u.UID, u.Name)
			}
		}
	}

	// Set default password "123456" for users that don't have one yet
	allUsers, _ := a.st.ListUsers(context.Background())
	for _, u := range allUsers {
		if u.PasswordHash == "" {
			hash, err := bcrypt.GenerateFromPassword([]byte("123456"), bcrypt.DefaultCost)
			if err != nil {
				log.Printf("⚠️  hash password for %s: %v", u.UID, err)
				continue
			}
			if err := a.st.SetPassword(context.Background(), u.ID, string(hash)); err != nil {
				log.Printf("⚠️  set password for %s: %v", u.UID, err)
			} else {
				log.Printf("🔑 Set default password for %s", u.UID)
			}
		}
	}

	return nil
}

// ─── Routes ───────────────────────────────────────────────────────────────────

func (a *App) registerRoutes() {
	// Public endpoints
	a.mux.HandleFunc("POST /api/v1/login", a.handleLogin)

	// Protected API routes (require JWT)
	a.mux.Handle("GET /api/v1/processes", a.authMiddleware(http.HandlerFunc(a.handleListProcesses)))
	a.mux.Handle("GET /api/v1/processes/{alias}/design", a.authMiddleware(http.HandlerFunc(a.handleLoadDesign)))
	a.mux.Handle("POST /api/v1/processes/{alias}/design", a.authMiddleware(http.HandlerFunc(a.handleSaveDesign)))
	a.mux.Handle("POST /api/v1/processes/{alias}/publish", a.authMiddleware(http.HandlerFunc(a.handlePublish)))
	a.mux.Handle("DELETE /api/v1/processes/{alias}", a.authMiddleware(http.HandlerFunc(a.handleDeleteProcess)))
	a.mux.Handle("GET /api/v1/entities", a.authMiddleware(http.HandlerFunc(a.handleListEntities)))
	a.mux.Handle("POST /api/v1/entities", a.authMiddleware(http.HandlerFunc(a.handleCreateEntity)))
	a.mux.Handle("GET /api/v1/entities/{id}", a.authMiddleware(http.HandlerFunc(a.handleGetEntity)))
	a.mux.Handle("POST /api/v1/entities/{id}/submit", a.authMiddleware(http.HandlerFunc(a.handleSubmitEntity)))
	a.mux.Handle("GET /api/v1/todos", a.authMiddleware(http.HandlerFunc(a.handleListTodos)))
		a.mux.Handle("POST /api/v1/todos/{id}/accept", a.authMiddleware(http.HandlerFunc(a.handleAcceptTodo)))
		a.mux.Handle("GET /api/v1/outbox", a.authMiddleware(http.HandlerFunc(a.handleListOutbox)))
		a.mux.Handle("POST /api/v1/entities/{id}/return", a.authMiddleware(http.HandlerFunc(a.handleReturnEntity)))
		a.mux.Handle("GET /api/v1/entities/{id}/opinions", a.authMiddleware(http.HandlerFunc(a.handleGetOpinions)))
	a.mux.Handle("POST /api/v1/entities/{id}/opinions", a.authMiddleware(http.HandlerFunc(a.handleAddOpinion)))
	a.mux.Handle("GET /api/v1/entities/{id}/logs", a.authMiddleware(http.HandlerFunc(a.handleEntityLogs)))
	a.mux.Handle("GET /api/v1/users", a.authMiddleware(http.HandlerFunc(a.handleListUsers)))
	a.mux.Handle("POST /api/v1/users", a.authMiddleware(http.HandlerFunc(a.handleCreateUser)))
	a.mux.Handle("GET /api/v1/users/{id}", a.authMiddleware(http.HandlerFunc(a.handleGetUser)))
	a.mux.Handle("PUT /api/v1/users/{id}", a.authMiddleware(http.HandlerFunc(a.handleUpdateUser)))
	a.mux.Handle("DELETE /api/v1/users/{id}", a.authMiddleware(http.HandlerFunc(a.handleDeleteUser)))
	a.mux.Handle("GET /api/v1/me", a.authMiddleware(http.HandlerFunc(a.handleMe)))

	// SPA fallback (no auth)
	a.mux.HandleFunc("/", a.spaHandler)
}

func (a *App) spaHandler(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/api/") {
		http.NotFound(w, r)
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/")
	if path == "" {
		path = "index.html"
	}
	data, err := webFS.ReadFile("webdist/" + path)
	if err != nil {
		data, _ = webFS.ReadFile("webdist/index.html")
	}
	if data != nil {
		ext := filepath.Ext(path)
		if mimeType := mime.TypeByExtension(ext); mimeType != "" {
			w.Header().Set("Content-Type", mimeType)
		}
		w.Write(data)
	}
}

// ─── Types ────────────────────────────────────────────────────────────────────

type entityItem struct {
	ID        string     `json:"id"`
	Seq       int64      `json:"seq"`
	Code      string     `json:"code"`
	ProName   string     `json:"pro_name"`
	Title     string     `json:"title"`
	State     int        `json:"state"`
	StateText string     `json:"state_text"`
	SerialNum string     `json:"serial_num"`
	CurAct    string     `json:"cur_act,omitempty"`
	DraftName string     `json:"draft_name"`
	SendAt    *time.Time `json:"send_at,omitempty"`
	OverAt    *time.Time `json:"over_at,omitempty"`
	CreatedAt string     `json:"created_at"`
}

type entityDetail struct {
	entityItem
	Tasks    []taskItem    `json:"tasks"`
	Todos    []todoItem    `json:"todos"`
	NextActs []nextActItem `json:"next_acts,omitempty"`
	Logs     []logItem     `json:"logs"`
	Opinions []opinionItem `json:"opinions"`
}

type taskItem struct {
	ID        string `json:"id"`
	Seq       int64  `json:"seq"`
	ActName   string `json:"act_name"`
	ActTitle  string `json:"act_title"`
	State     int    `json:"state"`
	StateText string `json:"state_text"`
	Handlers  string `json:"handlers"`
}

type todoItem struct {
	ID          string `json:"id"`
	Seq         int64  `json:"seq"`
	TaskID      string `json:"task_id"`
	EntityID    string `json:"entity_id"`
	ActID       string `json:"act_id"`
	ActTitle    string `json:"act_title"`
	EntityTitle string `json:"entity_title"`
	ProName     string `json:"pro_name"`
	HandlerUID  string `json:"handler_uid"`
	HandlerName string `json:"handler_name"`
	Accepted    bool   `json:"accepted"`
}

type nextActItem struct {
	ActID    string `json:"act_id"`
	ActName  string `json:"act_name"`
	ActTitle string `json:"act_title"`
	LinkID   int64  `json:"link_id"`
	LinkType int    `json:"link_type"`
}

type logItem struct {
	ID          int64  `json:"id"`
	ActTitle    string `json:"act_title"`
	HandlerName string `json:"handler_name"`
	Content     string `json:"content"`
	ArriveAt    string `json:"arrive_at"`
	FinishAt    string `json:"finish_at"`
}

type opinionItem struct {
	ID          int64  `json:"id"`
	ActTitle    string `json:"act_title"`
	HandlerName string `json:"handler_name"`
	Content     string `json:"content"`
	WrittenAt   string `json:"written_at"`
}

func fmtState(s nova.EntityState) string {
	switch s {
	case nova.EntityStateDRAFT:  return "草稿"
	case nova.EntityStateSUBMIT:  return "处理中"
	case nova.EntityStateBACK:    return "已退回"
	case nova.EntityStateCANCEL:  return "已撤销"
	case nova.EntityStateOVER:    return "已完结"
	default: return "未知"
	}
}

func fmtTaskState(s nova.TaskState) string {
	switch s {
	case nova.TaskStateTODO:       return "待办"
	case nova.TaskStatePROCESSING: return "处理中"
	case nova.TaskStateOVER:       return "已完成"
	case nova.TaskStateCONVERGING: return "汇聚中"
	case nova.TaskStateWAITING:    return "等待中"
	case nova.TaskStateREADY:      return "就绪"
	case nova.TaskStateINITIAL:    return "初始"
	default: return "未知"
	}
}

// ─── Handlers ─────────────────────────────────────────────────────────────────

func (a *App) handleListProcesses(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	pros, err := a.st.ListPros(ctx)
	if err != nil {
		errJSON(w, http.StatusInternalServerError, err.Error())
		return
	}
	var procs []map[string]any
	for _, pro := range pros {
		procs = append(procs, map[string]any{
			"id": pro.ID, "seq": pro.Seq, "alias": pro.Alias, "name": pro.Name, "ver": pro.Ver,
		})
	}
	if procs == nil {
		procs = []map[string]any{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"processes": procs})
}

// ─── Process Design Handlers ─────────────────────────────────────────────────

// designAct is the JSON representation of an activity in design mode.
type designAct struct {
	Name     string `json:"name"`
	Title    string `json:"title"`
	Type     int    `json:"type"`
	Policy   int    `json:"policy,omitempty"`
	Editable bool   `json:"editable"`
}

// designLink is the JSON representation of a link in design mode.
type designLink struct {
	From string `json:"from"`
	To   string `json:"to"`
}

func (a *App) handleLoadDesign(w http.ResponseWriter, r *http.Request) {
	alias := r.PathValue("alias")
	if alias == "" {
		errJSON(w, http.StatusBadRequest, "alias required")
		return
	}

	ctx := r.Context()
	pro, err := a.st.GetProByAlias(ctx, alias)
	if err != nil {
		errJSON(w, http.StatusInternalServerError, err.Error())
		return
	}
	if pro == nil {
		errJSON(w, http.StatusNotFound, "process not found: "+alias)
		return
	}

	// Get ProVer for current version
	pv, err := a.st.GetProVer(ctx, pro.ID, pro.Ver)
	if err != nil {
		errJSON(w, http.StatusInternalServerError, err.Error())
		return
	}
	if pv == nil {
		errJSON(w, http.StatusNotFound, "process version not found")
		return
	}

	// Get acts
	acts, err := a.st.GetActsByProVer(ctx, pv.ID)
	if err != nil {
		errJSON(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Get links
	links, err := a.st.GetLinksByProVer(ctx, pv.ID)
	if err != nil {
		errJSON(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Get man_rules for each act
	manRules := make(map[string]*nova.ManRule)
	for _, act := range acts {
		rule, _ := a.st.GetManRule(ctx, act.ID)
		if rule != nil {
			manRules[act.Name] = rule
		}
	}

	// Build response: map act IDs to names
	actIDToName := make(map[string]string, len(acts))
	for _, act := range acts {
		actIDToName[act.ID] = act.Name
	}

	var actItems []map[string]any
	for _, act := range acts {
		item := map[string]any{
			"name":  act.Name,
			"title": act.Title,
			"type":  int(act.Type),
		}
		if rule, ok := manRules[act.Name]; ok {
			item["policy"] = int(rule.Policy)
		}
		actItems = append(actItems, item)
	}

	var linkItems []map[string]any
	for _, link := range links {
		fromName := actIDToName[link.PrevActID]
		if fromName == "" {
			fromName = link.PrevActID
		}
		toName := actIDToName[link.ActID]
		if toName == "" {
			toName = link.ActID
		}
		linkItems = append(linkItems, map[string]any{
			"from": fromName,
			"to":   toName,
		})
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"process": map[string]any{
			"id":    pro.ID,
			"alias": pro.Alias,
			"name":  pro.Name,
			"ver":   pro.Ver,
		},
		"acts":  actItems,
		"links": linkItems,
		"man_rules": map[string]any{
			// Flatten: activity_name -> policy
		},
	})
}

func (a *App) handleSaveDesign(w http.ResponseWriter, r *http.Request) {
	alias := r.PathValue("alias")
	if alias == "" {
		errJSON(w, http.StatusBadRequest, "alias required")
		return
	}

	var req struct {
		Name  string       `json:"name"`
		Acts  []designAct  `json:"acts"`
		Links []designLink `json:"links"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		errJSON(w, http.StatusBadRequest, "invalid json")
		return
	}
	if req.Name == "" {
		errJSON(w, http.StatusBadRequest, "name required")
		return
	}
	if len(req.Acts) == 0 {
		errJSON(w, http.StatusBadRequest, "at least one act required")
		return
	}

	ctx := r.Context()
	pro, ver, err := a.saveDesign(ctx, alias, req.Name, req.Acts, req.Links)
	if err != nil {
		errJSON(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"process": map[string]any{
			"id": pro.ID, "alias": pro.Alias, "name": pro.Name, "ver": pro.Ver,
		},
		"ver": ver,
	})
}

func (a *App) handlePublish(w http.ResponseWriter, r *http.Request) {
	alias := r.PathValue("alias")
	if alias == "" {
		errJSON(w, http.StatusBadRequest, "alias required")
		return
	}

	ctx := r.Context()
	pro, err := a.st.GetProByAlias(ctx, alias)
	if err != nil {
		errJSON(w, http.StatusInternalServerError, err.Error())
		return
	}
	if pro == nil {
		errJSON(w, http.StatusNotFound, "process not found: "+alias)
		return
	}

	// Get the current ProVer (draft) and mark it as released
	pv, err := a.st.GetProVer(ctx, pro.ID, pro.Ver)
	if err != nil {
		errJSON(w, http.StatusInternalServerError, err.Error())
		return
	}
	if pv == nil {
		errJSON(w, http.StatusNotFound, "version not found")
		return
	}

	// Update ProVer is_release to 1
	// Since there's no UpdateProVer method, we UPDATE via raw SQL on the store
	if err := a.st.Exec(`UPDATE pro_ver SET is_release = 1 WHERE id = ?`, pv.ID); err != nil {
		errJSON(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Rebuild engine for this process
	eng, err := nova.NewEngine(ctx, nova.EngineConfig{Store: a.st, ProAlias: alias})
	if err != nil {
		errJSON(w, http.StatusInternalServerError, err.Error())
		return
	}
	a.engines[alias] = eng
	a.engByProID[eng.Pro().ID] = eng

	writeJSON(w, http.StatusOK, map[string]any{
		"status": "published",
		"ver":    pro.Ver,
	})
}

func (a *App) handleDeleteProcess(w http.ResponseWriter, r *http.Request) {
	alias := r.PathValue("alias")
	if alias == "" {
		errJSON(w, http.StatusBadRequest, "alias required")
		return
	}

	ctx := r.Context()
	pro, err := a.st.GetProByAlias(ctx, alias)
	if err != nil {
		errJSON(w, http.StatusInternalServerError, err.Error())
		return
	}
	if pro == nil {
		errJSON(w, http.StatusNotFound, "process not found: "+alias)
		return
	}

	// Remove engine
	delete(a.engines, alias)
	delete(a.engByProID, pro.ID)

	if err := a.st.DeletePro(ctx, pro.ID); err != nil {
		errJSON(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// saveDesign persists a process design. If the process does not exist, it creates
// a new one. If it exists, it creates a new draft version with incremented ver.
func (a *App) saveDesign(ctx context.Context, alias, name string, acts []designAct, links []designLink) (*nova.Pro, int, error) {
	pro, _ := a.st.GetProByAlias(ctx, alias)
	if pro == nil {
		// New process: use ProcessBuilder to create
		builder := nova.NewProcess(alias, name)
		for _, act := range acts {
			at := nova.ActType(act.Type)
			builder.Act(act.Name, at)
			if act.Title != "" {
				builder.Title(act.Title)
			}
			if act.Policy > 0 {
				builder.On(nova.ManPolicy(act.Policy))
			}
		}
		for _, link := range links {
			builder.Link(link.From, link.To)
		}
		p, err := builder.Store(ctx, a.st)
		if err != nil {
			return nil, 0, fmt.Errorf("create process: %w", err)
		}
		return p, 1, nil
	}

	// Existing process: create new draft version
	nextVer := pro.Ver + 1

	// Validate (reuse builder.Validate logic)
	b := nova.NewProcess(alias, name)
	for _, act := range acts {
		at := nova.ActType(act.Type)
		b.Act(act.Name, at)
		if act.Title != "" {
			b.Title(act.Title)
		}
		if act.Policy > 0 {
			b.On(nova.ManPolicy(act.Policy))
		}
	}
	for _, link := range links {
		b.Link(link.From, link.To)
	}
	if err := b.Validate(); err != nil {
		return nil, 0, fmt.Errorf("validate: %w", err)
	}

	// Get existing pro_ver to know the pro_ver ID for this version
	oldPv, err := a.st.GetProVer(ctx, pro.ID, pro.Ver)
	if err != nil {
		return nil, 0, fmt.Errorf("get current ver: %w", err)
	}

	// Delete old acts and links
	if oldPv != nil {
		if err := a.st.DeleteLinksByProVer(ctx, oldPv.ID); err != nil {
			return nil, 0, fmt.Errorf("delete old links: %w", err)
		}
		if err := a.st.DeleteActsByProVer(ctx, oldPv.ID); err != nil {
			return nil, 0, fmt.Errorf("delete old acts: %w", err)
		}
	}

	// Create new ProVer (draft)
	newPv := &nova.ProVer{ProID: pro.ID, Ver: nextVer, IsRelease: false}
	if err := a.st.CreateProVer(ctx, newPv); err != nil {
		return nil, 0, fmt.Errorf("create version: %w", err)
	}

	// Create activities
	nameToID := make(map[string]string, len(acts))
	for _, act := range acts {
		aAct := &nova.Act{
			ProID: pro.ID,
			Name:  act.Name,
			Title: act.Title,
			Type:  nova.ActType(act.Type),
			Ver:   nextVer,
		}
		if act.Type == int(nova.ActTypeMANUAL) {
			aAct.Editable = true
		}
		if err := a.st.CreateAct(ctx, aAct); err != nil {
			return nil, 0, fmt.Errorf("create act %q: %w", act.Name, err)
		}
		nameToID[act.Name] = aAct.ID
	}

	// Create links
	for i, link := range links {
		l := &nova.Link{
			ProVerID:  newPv.ID,
			ActID:     nameToID[link.To],
			PrevActID: nameToID[link.From],
			Title:     fmt.Sprintf("L%d", i+1),
			Type:      nova.LinkTypeFORWARD,
		}
		if err := a.st.CreateLink(ctx, l); err != nil {
			return nil, 0, fmt.Errorf("create link %s→%s: %w", link.From, link.To, err)
		}
	}

	// Create man rules
	for _, act := range acts {
		if act.Policy > 0 {
			r := &nova.ManRule{
				ActID:      nameToID[act.Name],
				BaseOn:     nova.HandlerBaseChannel,
				Policy:     nova.ManPolicy(act.Policy),
				SelAllowed: false,
			}
			if err := a.st.CreateManRule(ctx, r); err != nil {
				return nil, 0, fmt.Errorf("create rule for %q: %w", act.Name, err)
			}
		}
	}

	// Update pro.Ver
	pro.Name = name
	pro.Ver = nextVer
	if err := a.st.UpdatePro(ctx, pro); err != nil {
		return nil, 0, fmt.Errorf("update pro: %w", err)
	}

	return pro, nextVer, nil
}

func (a *App) handleListEntities(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Parse optional state filter
	stateStr := r.URL.Query().Get("state")
	var stateFilter *int
	var sv int
	if _, err := fmt.Sscanf(stateStr, "%d", &sv); err == nil && sv >= 0 {
		stateFilter = &sv
	}

	entities, err := a.st.ListEntities(ctx, stateFilter)
	if err != nil {
		errJSON(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Build proID → proName map for all referenced pros
	proNameByID := make(map[string]string)
	for _, e := range entities {
		if _, ok := proNameByID[e.ProID]; !ok {
			pro, _ := a.st.GetPro(ctx, e.ProID)
			if pro != nil {
				proNameByID[e.ProID] = pro.Name
			} else {
				proNameByID[e.ProID] = ""
			}
		}
	}

	var list []entityItem
	for _, e := range entities {
		// Find current active task title
		curAct := ""
		tasks, _ := a.st.GetTasksByEntity(ctx, e.ID)
		for _, t := range tasks {
			if t.State != nova.TaskStateOVER {
				curAct = t.ActTitle
				break
			}
		}
		list = append(list, entityItem{
			ID: e.ID, Seq: e.Seq, Code: e.Code, ProName: proNameByID[e.ProID],
			Title: e.Title, State: int(e.State),
			StateText: fmtState(e.State), SerialNum: e.SerialNum,
			CurAct: curAct, DraftName: e.DraftName,
			SendAt: e.SendAt, OverAt: e.OverAt,
			CreatedAt: e.CreatedAt.Format("2006-01-02 15:04"),
		})
	}
	if list == nil { list = []entityItem{} }
	writeJSON(w, http.StatusOK, map[string]any{"entities": list})
}

func (a *App) handleCreateEntity(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ProAlias string `json:"pro_alias"`
		Title    string `json:"title"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		errJSON(w, http.StatusBadRequest, "invalid json")
		return
	}
	if req.ProAlias == "" || req.Title == "" {
		errJSON(w, http.StatusBadRequest, "pro_alias and title required")
		return
	}

	eng, ok := a.engines[req.ProAlias]
	if !ok {
		errJSON(w, http.StatusBadRequest, "unknown process alias: "+req.ProAlias)
		return
	}

	handlerUID, handlerName, _ := nova.CurHandler(r.Context())
	if handlerUID == "" {
		handlerUID = "unknown"
		handlerName = "未知用户"
	}

	ctx := r.Context()
	code := fmt.Sprintf("%s_%d", req.ProAlias, time.Now().Unix())
	ent, err := eng.CreateEntity(ctx, code, req.Title, handlerUID, handlerName, "", "")
	if err != nil {
		errJSON(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Create handle log
	now := time.Now().UTC()
	_ = a.st.CreateHandleLog(ctx, &nova.HandleLog{
		EntityID: ent.ID, ActTitle: "起草申请",
		HandlerUID: handlerUID, HandlerName: handlerName,
		ArriveAt: &now,
	})

	writeJSON(w, http.StatusCreated, map[string]any{
		"entity": map[string]any{
			"id": ent.ID, "seq": ent.Seq, "code": ent.Code, "title": ent.Title,
			"state": int(ent.State), "state_text": fmtState(ent.State),
		},
	})
}

func (a *App) handleGetEntity(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		errJSON(w, http.StatusBadRequest, "invalid id")
		return
	}

	ctx := r.Context()
	ent, err := a.st.GetEntity(ctx, id)
	if err != nil || ent == nil {
		errJSON(w, http.StatusNotFound, "entity not found")
		return
	}

	pro, _ := a.st.GetPro(ctx, ent.ProID)
	proName := ""
	if pro != nil { proName = pro.Name }

	// Tasks
	tasks, _ := a.st.GetTasksByEntity(ctx, id)
	var taskItems []taskItem
	var curAct string
	for _, t := range tasks {
		taskItems = append(taskItems, taskItem{
			ID: t.ID, Seq: t.Seq, ActName: t.ActName, ActTitle: t.ActTitle,
			State: int(t.State), StateText: fmtTaskState(t.State), Handlers: t.Handlers,
		})
		if t.State != nova.TaskStateOVER { curAct = t.ActTitle }
	}
	if taskItems == nil { taskItems = []taskItem{} }

	// Todos
	todos, _ := a.st.GetEntityTodos(ctx, id)
	var todoItems []todoItem
	for _, td := range todos {
		todoItems = append(todoItems, todoItem{
			ID: td.ID, Seq: td.Seq, TaskID: td.TaskID, EntityID: td.EntityID, ActID: td.ActID,
			ActTitle: td.ActTitle, EntityTitle: td.EntityTitle, ProName: td.ProName,
			HandlerUID: td.HandlerUID, HandlerName: td.HandlerName, Accepted: td.Accepted,
		})
	}
	if todoItems == nil { todoItems = []todoItem{} }

	// Next actions
	var nextActs []nextActItem
	for _, t := range tasks {
		if t.State != nova.TaskStateOVER {
			links, _ := a.st.GetLinksByAct(ctx, t.ActID)
			for _, l := range links {
				nextAct, _ := a.st.GetAct(ctx, l.ActID)
				if nextAct != nil && nextAct.Type != nova.ActTypeSTART {
					nextActs = append(nextActs, nextActItem{
						ActID: nextAct.ID, ActName: nextAct.Name, ActTitle: nextAct.Title,
						LinkID: l.ID, LinkType: int(l.Type),
					})
				}
			}
		}
	}
	if nextActs == nil { nextActs = []nextActItem{} }

	// Handle logs
	handleLogs, _ := a.st.GetEntityHandleLogs(ctx, id)
	var logItems []logItem
	for _, l := range handleLogs {
		logItems = append(logItems, logItem{
			ID: l.ID, ActTitle: l.ActTitle, HandlerName: l.HandlerName,
			Content: l.Content, ArriveAt: fmtTime(l.ArriveAt), FinishAt: fmtTime(l.FinishAt),
		})
	}
	if logItems == nil { logItems = []logItem{} }

	// Opinions
	opinions, _ := a.st.GetEntityOpinions(ctx, id)
	var opinionItems []opinionItem
	for _, o := range opinions {
		opinionItems = append(opinionItems, opinionItem{
			ID: o.ID, ActTitle: o.ActTitle, HandlerName: o.HandlerName,
			Content: o.Content, WrittenAt: o.WrittenAt.Format("2006-01-02 15:04"),
		})
	}
	if opinionItems == nil { opinionItems = []opinionItem{} }

	detail := entityDetail{
		entityItem: entityItem{
			ID: ent.ID, Seq: ent.Seq, Code: ent.Code, ProName: proName,
			Title: ent.Title, State: int(ent.State),
			StateText: fmtState(ent.State), SerialNum: ent.SerialNum,
			CurAct: curAct, DraftName: ent.DraftName,
			SendAt: ent.SendAt, OverAt: ent.OverAt,
			CreatedAt: ent.CreatedAt.Format("2006-01-02 15:04"),
		},
		Tasks: taskItems, Todos: todoItems,
		NextActs: nextActs, Logs: logItems,
		Opinions: opinionItems,
	}

	writeJSON(w, http.StatusOK, map[string]any{"entity": detail})
}

func (a *App) handleSubmitEntity(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		errJSON(w, http.StatusBadRequest, "invalid id")
		return
	}

	handlerUID, handlerName, _ := nova.CurHandler(r.Context())
	if handlerUID == "" {
		handlerUID = "unknown"
		handlerName = "未知用户"
	}
	var opinionContent string
	// Accept optional handler override and opinion from body
	var req struct {
		HandlerUID     string `json:"handler_uid,omitempty"`
		HandlerName    string `json:"handler_name,omitempty"`
		OpinionContent string `json:"opinion_content,omitempty"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	if req.HandlerUID != "" {
		handlerUID = req.HandlerUID
		handlerName = req.HandlerName
	}
	if req.OpinionContent != "" {
		opinionContent = req.OpinionContent
	}

	ctx := r.Context()
	eng, err := a.engineForEntity(ctx, id)
	if err != nil {
		errJSON(w, http.StatusBadRequest, err.Error())
		return
	}
	sess, err := eng.Session(ctx, id, handlerUID, handlerName)
	if err != nil {
		errJSON(w, http.StatusInternalServerError, err.Error())
		return
	}

	si, err := sess.Submit()
	if err != nil {
		errJSON(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Now().UTC()

	// Save opinion if provided
	if opinionContent != "" {
		// Find current act title from the done task
		opinionActTitle := ""
		if si.DoneTask != nil {
			opinionActTitle = si.DoneTask.ActTitle
		}
		_ = a.st.CreateOpinion(ctx, &nova.Opinion{
			EntityID:    id,
			ActID:       "",
			TaskID:      "",
			HandlerUID:  handlerUID,
			HandlerName: handlerName,
			Content:     opinionContent,
			ActTitle:    opinionActTitle,
		})
	}

	// Update handle log finish with opinion
	if si.DoneTask != nil {
		_ = a.st.UpdateHandleLogFinish(ctx, si.DoneTask.ID, now.Unix())
		_ = a.st.UpdateHandleLogContent(ctx, si.DoneTask.ID, opinionContent)
	}

	// Create handle log for the new task
	if len(si.NewTodos) > 0 {
		for _, td := range si.NewTodos {
			_ = a.st.CreateHandleLog(ctx, &nova.HandleLog{
				EntityID:    id,
				TaskID:      td.TaskID,
				ActTitle:    td.ActTitle,
				HandlerUID:  td.HandlerUID,
				HandlerName: td.HandlerName,
				Content:     opinionContent,
				ArriveAt:    &now,
			})
		}
	}

	resp := map[string]any{
		"status":    "ok",
		"new_todos": len(si.NewTodos),
	}
	if sess.State() == nova.SessionStateView {
		resp["entity_over"] = true
	}
	writeJSON(w, http.StatusOK, resp)
}

func (a *App) handleListTodos(w http.ResponseWriter, r *http.Request) {
	handlerUID := r.URL.Query().Get("handler")
	if handlerUID == "" {
		uid, _, _ := nova.CurHandler(r.Context())
		handlerUID = uid
	}
	if handlerUID == "" { handlerUID = "unknown" }

	todos, err := a.st.GetUserTodos(r.Context(), handlerUID)
	if err != nil || todos == nil {
		todos = []*nova.Todo{}
	}

	writeJSON(w, http.StatusOK, map[string]any{"todos": todos})
}

func (a *App) handleAcceptTodo(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		errJSON(w, http.StatusBadRequest, "invalid id")
		return
	}
	if err := a.st.AcceptTodo(r.Context(), id); err != nil {
		errJSON(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (a *App) handleListOutbox(w http.ResponseWriter, r *http.Request) {
	handlerUID := r.URL.Query().Get("handler")
	if handlerUID == "" {
		uid, _, _ := nova.CurHandler(r.Context())
		handlerUID = uid
	}
	if handlerUID == "" { handlerUID = "unknown" }

	entities, err := a.st.GetUserDoneEntities(r.Context(), handlerUID)
	if err != nil || entities == nil {
		entities = []*nova.Entity{}
	}

	// Build proID → proName map for all referenced pros
	proNameByID := make(map[string]string)
	for _, e := range entities {
		if _, ok := proNameByID[e.ProID]; !ok {
			pro, _ := a.st.GetPro(r.Context(), e.ProID)
			if pro != nil {
				proNameByID[e.ProID] = pro.Name
			} else {
				proNameByID[e.ProID] = ""
			}
		}
	}

	var list []entityItem
	for _, e := range entities {
		list = append(list, entityItem{
			ID: e.ID, Seq: e.Seq, Code: e.Code, ProName: proNameByID[e.ProID],
			Title: e.Title, State: int(e.State),
			StateText: fmtState(e.State), SerialNum: e.SerialNum,
			DraftName: e.DraftName, SendAt: e.SendAt, OverAt: e.OverAt,
			CreatedAt: e.CreatedAt.Format("2006-01-02 15:04"),
		})
	}
	if list == nil { list = []entityItem{} }
	writeJSON(w, http.StatusOK, map[string]any{"outbox": list})
}

func (a *App) handleEntityLogs(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		errJSON(w, http.StatusBadRequest, "invalid id")
		return
	}

	logs, err := a.st.GetEntityHandleLogs(r.Context(), id)
	if err != nil || logs == nil { logs = []*nova.HandleLog{} }

	var items []logItem
	for _, l := range logs {
		items = append(items, logItem{
			ID: l.ID, ActTitle: l.ActTitle, HandlerName: l.HandlerName,
			Content: l.Content, ArriveAt: fmtTime(l.ArriveAt), FinishAt: fmtTime(l.FinishAt),
		})
	}
	if items == nil { items = []logItem{} }
	writeJSON(w, http.StatusOK, map[string]any{"logs": items})
}

// ─── User Handlers ──────────────────────────────────────────────────────────

func (a *App) handleListUsers(w http.ResponseWriter, r *http.Request) {
	users, err := a.st.ListUsers(r.Context())
	if err != nil {
		errJSON(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"users": users})
}

func (a *App) handleCreateUser(w http.ResponseWriter, r *http.Request) {
	var req struct {
		UID   string `json:"uid"`
		Name  string `json:"name"`
		Email string `json:"email"`
		Phone string `json:"phone"`
		Dept  string `json:"dept"`
		State int    `json:"state"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		errJSON(w, http.StatusBadRequest, "invalid json")
		return
	}
	if req.UID == "" || req.Name == "" {
		errJSON(w, http.StatusBadRequest, "uid and name required")
		return
	}
	u := &nova.User{UID: req.UID, Name: req.Name, Email: req.Email, Phone: req.Phone, Dept: req.Dept, State: req.State}
	if err := a.st.CreateUser(r.Context(), u); err != nil {
		errJSON(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"user": u})
}

func (a *App) handleGetUser(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		errJSON(w, http.StatusBadRequest, "invalid id")
		return
	}
	u, err := a.st.GetUser(r.Context(), id)
	if err != nil || u == nil {
		errJSON(w, http.StatusNotFound, "user not found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"user": u})
}

func (a *App) handleUpdateUser(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		errJSON(w, http.StatusBadRequest, "invalid id")
		return
	}
	var req struct {
		UID   string `json:"uid"`
		Name  string `json:"name"`
		Email string `json:"email"`
		Phone string `json:"phone"`
		Dept  string `json:"dept"`
		State int    `json:"state"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		errJSON(w, http.StatusBadRequest, "invalid json")
		return
	}
	u := &nova.User{ID: id, UID: req.UID, Name: req.Name, Email: req.Email, Phone: req.Phone, Dept: req.Dept, State: req.State}
	if err := a.st.UpdateUser(r.Context(), u); err != nil {
		errJSON(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"user": u})
}

func (a *App) handleDeleteUser(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		errJSON(w, http.StatusBadRequest, "invalid id")
		return
	}
	if err := a.st.DeleteUser(r.Context(), id); err != nil {
		errJSON(w, http.StatusInternalServerError, err.Error())
		return
	}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	}

	// ─── Return Handler ─────────────────────────────────────────────────────

	func (a *App) handleReturnEntity(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if id == "" {
			errJSON(w, http.StatusBadRequest, "invalid id")
			return
		}
		var req struct {
			TargetActName  string `json:"target_act_name"`
			OpinionContent string `json:"opinion_content"`
		}
		json.NewDecoder(r.Body).Decode(&req)
		if req.TargetActName == "" {
			errJSON(w, http.StatusBadRequest, "target_act_name required")
			return
		}
		curUID, curName, _ := nova.CurHandler(r.Context())
		if curUID == "" {
			curUID = "unknown"
			curName = "未知用户"
		}
		ctx := r.Context()

		ent, err := a.st.GetEntity(ctx, id)
		if err != nil || ent == nil {
			errJSON(w, http.StatusNotFound, "entity not found")
			return
		}
		if ent.State != nova.EntityStateSUBMIT && ent.State != nova.EntityStateBACK {
			errJSON(w, http.StatusBadRequest, "entity not in process")
			return
		}

		tasks, err := a.st.GetTasksByEntity(ctx, id)
		if err != nil {
			errJSON(w, http.StatusInternalServerError, err.Error())
			return
		}
		var curTask *nova.Task
		for _, t := range tasks {
			if t.State == nova.TaskStateTODO || t.State == nova.TaskStatePROCESSING {
				curTask = t
				break
			}
		}
		if curTask == nil {
			errJSON(w, http.StatusBadRequest, "no active task")
			return
		}

		// Find target act in process definition
		pro, err := a.st.GetPro(ctx, ent.ProID)
		if err != nil || pro == nil {
			errJSON(w, http.StatusNotFound, "process not found")
			return
		}
		proVer, err := a.st.GetProVer(ctx, pro.ID, pro.Ver)
		if err != nil || proVer == nil {
			errJSON(w, http.StatusNotFound, "process ver not found")
			return
		}
		acts, err := a.st.GetActsByProVer(ctx, proVer.ID)
		if err != nil {
			errJSON(w, http.StatusInternalServerError, err.Error())
			return
		}
		var targetAct *nova.Act
		for _, a2 := range acts {
			if a2.Name == req.TargetActName {
				targetAct = a2
				break
			}
		}
		if targetAct == nil {
			errJSON(w, http.StatusBadRequest, "target act not found")
			return
		}

		// Find original handler of the target act from handle logs
		targetUID, targetName := curUID, curName // fallback to current user
		logs, _ := a.st.GetEntityHandleLogs(ctx, id)
		for i := len(logs) - 1; i >= 0; i-- {
			if logs[i].ActTitle == targetAct.Title && logs[i].HandlerUID != "" {
				targetUID = logs[i].HandlerUID
				targetName = logs[i].HandlerName
				break
			}
		}
		// If target is draft act, use the draft creator
		if targetUID == curUID && ent.DraftUID != "" && targetAct.Name == "draft" {
			targetUID = ent.DraftUID
			targetName = ent.DraftName
		}

		now := time.Now().UTC()
		curTask.State = nova.TaskStateOVER
		curTask.EndAt = &now
		a.st.UpdateTaskState(ctx, curTask.ID, nova.TaskStateOVER)

		step := &nova.Step{EntityID: id, ActID: targetAct.ID, ActName: targetAct.Name}
		a.st.CreateStep(ctx, step)

		newTask := &nova.Task{
			EntityID: id, StepID: step.ID, ActID: targetAct.ID,
			ActName: targetAct.Name, ActTitle: targetAct.Title,
			State: nova.TaskStateTODO, Handlers: targetName, StartAt: &now,
		}
		a.st.CreateTask(ctx, newTask)

		a.st.CreateTodo(ctx, &nova.Todo{
			TaskID: newTask.ID, EntityID: id, ActID: targetAct.ID, ProID: pro.ID,
			HandlerUID: targetUID, HandlerName: targetName,
			ActTitle: targetAct.Title, EntityTitle: ent.Title, ProName: pro.Name, ArriveAt: &now,
		})

		a.st.UpdateEntityState(ctx, id, nova.EntityStateBACK)
		a.st.CreateHandleLog(ctx, &nova.HandleLog{
			EntityID: id, TaskID: curTask.ID, ActTitle: curTask.ActTitle,
			HandlerUID: curUID, HandlerName: curName,
			Content: "退回至【" + targetAct.Title + "】", ArriveAt: &now, FinishAt: &now,
		})
		a.st.CreateHandleLog(ctx, &nova.HandleLog{
			EntityID: id, TaskID: newTask.ID, ActTitle: targetAct.Title,
			HandlerUID: targetUID, HandlerName: targetName,
			Content: "被退回至此", ArriveAt: &now,
		})

		if req.OpinionContent != "" {
			a.st.CreateOpinion(ctx, &nova.Opinion{
				EntityID: id, HandlerUID: curUID, HandlerName: curName,
				Content: "退回: " + req.OpinionContent, ActTitle: curTask.ActTitle, WrittenAt: now,
			})
		}

		writeJSON(w, http.StatusOK, map[string]any{"status": "returned", "target_act": req.TargetActName, "new_task_id": newTask.ID, "entity_state": "已退回"})
	}

	// ─── Opinion Handlers ──────────────────────────────────────────────────────

func (a *App) handleGetOpinions(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		errJSON(w, http.StatusBadRequest, "invalid id")
		return
	}

	opinions, err := a.st.GetEntityOpinions(r.Context(), id)
	if err != nil || opinions == nil { opinions = []*nova.Opinion{} }

	var items []opinionItem
	for _, o := range opinions {
		items = append(items, opinionItem{
			ID: o.ID, ActTitle: o.ActTitle, HandlerName: o.HandlerName,
			Content: o.Content, WrittenAt: o.WrittenAt.Format("2006-01-02 15:04"),
		})
	}
	if items == nil { items = []opinionItem{} }
	writeJSON(w, http.StatusOK, map[string]any{"opinions": items})
}

func (a *App) handleAddOpinion(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		errJSON(w, http.StatusBadRequest, "invalid id")
		return
	}

	var req struct {
		Content string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		errJSON(w, http.StatusBadRequest, "invalid json")
		return
	}
	if req.Content == "" {
		errJSON(w, http.StatusBadRequest, "content required")
		return
	}

	ctx := r.Context()
	ent, err := a.st.GetEntity(ctx, id)
	if err != nil || ent == nil {
		errJSON(w, http.StatusNotFound, "entity not found")
		return
	}

	// Find current task's act title
	actTitle := ""
	tasks, _ := a.st.GetTasksByEntity(ctx, id)
	for _, t := range tasks {
		if t.State != nova.TaskStateOVER {
			actTitle = t.ActTitle
			break
		}
	}

	handlerUID, handlerName, _ := nova.CurHandler(r.Context())
	if handlerUID == "" {
		handlerUID = "unknown"
		handlerName = "未知用户"
	}

	_ = a.st.CreateOpinion(ctx, &nova.Opinion{
		EntityID:    id,
		HandlerUID:  handlerUID,
		HandlerName: handlerName,
		Content:     req.Content,
		ActTitle:    actTitle,
	})

	writeJSON(w, http.StatusCreated, map[string]string{"status": "ok"})
}

// ─── JWT / Auth ─────────────────────────────────────────────────────────────

// jwtClaims represents the payload of a Nova JWT token.
type jwtClaims struct {
	UserID  string `json:"uid"`
	Seq     int64  `json:"seq"`
	Name    string `json:"name"`
	Dept    string `json:"dept"`
	Expires int64  `json:"exp"`
}

func (a *App) generateToken(user *nova.User) string {
	claims := jwtClaims{
		UserID:  user.UID,
		Seq:     user.Seq,
		Name:    user.Name,
		Dept:    user.Dept,
		Expires: time.Now().Add(72 * time.Hour).Unix(),
	}
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256","typ":"JWT"}`))
	payload := base64.RawURLEncoding.EncodeToString(mustMarshal(claims))
	sig := hmacSHA256(a.jwtSecret, header+"."+payload)
	return header + "." + payload + "." + sig
}

func (a *App) verifyToken(token string) *jwtClaims {
	parts := strings.SplitN(token, ".", 3)
	if len(parts) != 3 {
		return nil
	}
	expected := hmacSHA256(a.jwtSecret, parts[0]+"."+parts[1])
	if parts[2] != expected {
		return nil
	}
	data, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil
	}
	var claims jwtClaims
	if err := json.Unmarshal(data, &claims); err != nil {
		return nil
	}
	if time.Now().Unix() > claims.Expires {
		return nil
	}
	return &claims
}

func hmacSHA256(secret []byte, data string) string {
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(data))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func mustMarshal(v any) []byte {
	data, _ := json.Marshal(v)
	return data
}

// authMiddleware extracts the JWT from the Authorization header and
// injects the user info into the request context.
func (a *App) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth == "" || !strings.HasPrefix(auth, "Bearer ") {
			errJSON(w, http.StatusUnauthorized, "missing or invalid token")
			return
		}
		claims := a.verifyToken(strings.TrimPrefix(auth, "Bearer "))
		if claims == nil {
			errJSON(w, http.StatusUnauthorized, "invalid or expired token")
			return
		}
		ctx := context.WithValue(r.Context(), ctxKeyAuth, claims)
		ctx = nova.WithHandler(ctx, claims.UserID, claims.Name)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

type contextAuthKey string

const ctxKeyAuth = contextAuthKey("nova:auth")

// currentUser extracts the authenticated user's claims from context.
func currentUser(ctx context.Context) *jwtClaims {
	v := ctx.Value(ctxKeyAuth)
	if v == nil {
		return nil
	}
	return v.(*jwtClaims)
}

// ─── Login Handler ──────────────────────────────────────────────────────────

func (a *App) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		UID      string `json:"uid"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		errJSON(w, http.StatusBadRequest, "invalid json")
		return
	}
	if req.UID == "" || req.Password == "" {
		errJSON(w, http.StatusBadRequest, "uid and password required")
		return
	}

	user, err := a.st.GetUserByUID(r.Context(), req.UID)
	if err != nil || user == nil {
		errJSON(w, http.StatusUnauthorized, "invalid credentials")
		return
	}
	if user.State != 0 {
		errJSON(w, http.StatusForbidden, "account disabled")
		return
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		errJSON(w, http.StatusUnauthorized, "invalid credentials")
		return
	}

	token := a.generateToken(user)
	writeJSON(w, http.StatusOK, map[string]any{
		"token": token,
		"user": map[string]any{
			"uid":  user.UID,
			"name": user.Name,
			"dept": user.Dept,
		},
	})
}

// ─── Me Handler ─────────────────────────────────────────────────────────────

func (a *App) handleMe(w http.ResponseWriter, r *http.Request) {
	claims := currentUser(r.Context())
	if claims == nil {
		errJSON(w, http.StatusUnauthorized, "not authenticated")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"uid":  claims.UserID,
		"name": claims.Name,
		"dept": claims.Dept,
		"seq":  claims.Seq,
	})
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func errJSON(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func fmtTime(t *time.Time) string {
	if t == nil { return "" }
	return t.Format("2006-01-02 15:04")
}
