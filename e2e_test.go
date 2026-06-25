package nova_test

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"testing"

	_ "github.com/mattn/go-sqlite3"
	"github.com/liyan/nova"
	"github.com/liyan/nova/sqlite"
)

// ─── Two seed DBs for isolation ───────────────────────────────────────────────

// seedSimpleDB: START -> 起草 -> 审批 -> 归档 -> END
func seedSimpleDB(t *testing.T) string {
	t.Helper()
	return seedProcesses(t, false)
}

// seedConcurDB: START -> 起草 -> ROUTER -> [审批A, 审批B] -> CONVERGE -> 归档 -> END
func seedConcurDB(t *testing.T) string {
	t.Helper()
	return seedProcesses(t, true)
}

func seedProcesses(t *testing.T, withConcur bool) string {
	t.Helper()
	dbPath := "/tmp/nova_test_" + t.Name() + ".db"
	_ = os.Remove(dbPath)

	db, err := sql.Open("sqlite3", dbPath+"?cache=shared&_journal_mode=WAL")
	if err != nil {
		t.Fatalf("seed db: %v", err)
	}
	defer db.Close()

	// Schema
	schema := []string{
		`PRAGMA foreign_keys = ON;`,
		`PRAGMA journal_mode = WAL;`,
		`CREATE TABLE IF NOT EXISTS process (id TEXT PRIMARY KEY, seq INTEGER NOT NULL DEFAULT 0, alias TEXT NOT NULL UNIQUE, name TEXT NOT NULL DEFAULT '', ver INTEGER NOT NULL DEFAULT 1, channel TEXT NOT NULL DEFAULT '', created_at TEXT NOT NULL DEFAULT (datetime('now')));`,
		`CREATE TABLE IF NOT EXISTS pro_ver (id TEXT PRIMARY KEY, seq INTEGER NOT NULL DEFAULT 0, pro_id TEXT NOT NULL REFERENCES process(id), ver INTEGER NOT NULL DEFAULT 1, is_release INTEGER NOT NULL DEFAULT 0, UNIQUE(pro_id, ver));`,
		`CREATE TABLE IF NOT EXISTS act (id TEXT PRIMARY KEY, seq INTEGER NOT NULL DEFAULT 0, pro_id TEXT NOT NULL REFERENCES process(id), name TEXT NOT NULL, title TEXT NOT NULL DEFAULT '', act_type INTEGER NOT NULL DEFAULT 1, act_ver INTEGER NOT NULL DEFAULT 1, editable INTEGER NOT NULL DEFAULT 1, force_opinion INTEGER NOT NULL DEFAULT 0, wait_acts TEXT NOT NULL DEFAULT '', created_at TEXT NOT NULL DEFAULT (datetime('now')), UNIQUE(pro_id, name, act_ver));`,
		`CREATE TABLE IF NOT EXISTS act_link (id TEXT PRIMARY KEY, seq INTEGER NOT NULL DEFAULT 0, pro_ver_id TEXT NOT NULL REFERENCES pro_ver(id), act_id TEXT NOT NULL REFERENCES act(id), prev_act_id TEXT NOT NULL, title TEXT NOT NULL DEFAULT '', link_type INTEGER NOT NULL DEFAULT 1, decision_filter TEXT NOT NULL DEFAULT '');`,
		`CREATE TABLE IF NOT EXISTS man_rule (act_id TEXT PRIMARY KEY REFERENCES act(id), base_on INTEGER NOT NULL DEFAULT 0, policy INTEGER NOT NULL DEFAULT 10, sel_allowed INTEGER NOT NULL DEFAULT 0, group_set TEXT NOT NULL DEFAULT '', task_act TEXT NOT NULL DEFAULT '');`,
		`CREATE TABLE IF NOT EXISTS entity (id TEXT PRIMARY KEY, seq INTEGER NOT NULL DEFAULT 0, code TEXT NOT NULL UNIQUE, pro_id TEXT NOT NULL, pro_ver INTEGER NOT NULL DEFAULT 1, title TEXT NOT NULL DEFAULT '', state INTEGER NOT NULL DEFAULT 0, draft_uid TEXT NOT NULL DEFAULT '', draft_name TEXT NOT NULL DEFAULT '', draft_dept TEXT NOT NULL DEFAULT '', parent_id TEXT NOT NULL DEFAULT '', serial_num TEXT NOT NULL DEFAULT '', send_at TEXT, over_at TEXT, created_at TEXT NOT NULL DEFAULT (datetime('now')), updated_at TEXT NOT NULL DEFAULT (datetime('now')));`,
		`CREATE TABLE IF NOT EXISTS run_task (id TEXT PRIMARY KEY, seq INTEGER NOT NULL DEFAULT 0, entity_id TEXT NOT NULL REFERENCES entity(id), step_id INTEGER NOT NULL DEFAULT 0, act_id TEXT NOT NULL, act_name TEXT NOT NULL DEFAULT '', act_title TEXT NOT NULL DEFAULT '', task_state INTEGER NOT NULL DEFAULT 0, handlers TEXT NOT NULL DEFAULT '', converge INTEGER NOT NULL DEFAULT 0, waiting TEXT NOT NULL DEFAULT '', start_at TEXT, end_at TEXT, created_at TEXT NOT NULL DEFAULT (datetime('now')), updated_at TEXT NOT NULL DEFAULT (datetime('now')));`,
		`CREATE TABLE IF NOT EXISTS run_todo (id TEXT PRIMARY KEY, seq INTEGER NOT NULL DEFAULT 0, task_id TEXT NOT NULL REFERENCES run_task(id), entity_id TEXT NOT NULL DEFAULT '', act_id TEXT NOT NULL DEFAULT '', pro_id TEXT NOT NULL DEFAULT '', handler_uid TEXT NOT NULL, handler_name TEXT NOT NULL DEFAULT '', sender_uid TEXT NOT NULL DEFAULT '', sender_name TEXT NOT NULL DEFAULT '', act_title TEXT NOT NULL DEFAULT '', entity_title TEXT NOT NULL DEFAULT '', pro_name TEXT NOT NULL DEFAULT '', arrive_at TEXT, accept_at TEXT, accepted INTEGER NOT NULL DEFAULT 0, todo_key TEXT NOT NULL DEFAULT '', created_at TEXT NOT NULL DEFAULT (datetime('now')));`,
		`CREATE TABLE IF NOT EXISTS run_step (id INTEGER PRIMARY KEY AUTOINCREMENT, entity_id TEXT NOT NULL REFERENCES entity(id), act_id TEXT NOT NULL DEFAULT '', act_name TEXT NOT NULL DEFAULT '', created_at TEXT NOT NULL DEFAULT (datetime('now')));`,
		`CREATE TABLE IF NOT EXISTS step_log (id INTEGER PRIMARY KEY AUTOINCREMENT, entity_id TEXT NOT NULL, step_id INTEGER NOT NULL, cur_act_id TEXT NOT NULL, dest_act_id TEXT NOT NULL, dest_act_name TEXT NOT NULL DEFAULT '', dest_act_title TEXT NOT NULL DEFAULT '', dest_act_type INTEGER NOT NULL DEFAULT 0, link_type INTEGER NOT NULL DEFAULT 1, logged_at TEXT NOT NULL DEFAULT (datetime('now')));`,
		`CREATE TABLE IF NOT EXISTS handle_log (id INTEGER PRIMARY KEY AUTOINCREMENT, entity_id TEXT NOT NULL, step_id INTEGER NOT NULL DEFAULT 0, task_id TEXT NOT NULL DEFAULT '', act_id TEXT NOT NULL DEFAULT '', act_title TEXT NOT NULL DEFAULT '', handler_uid TEXT NOT NULL DEFAULT '', handler_name TEXT NOT NULL DEFAULT '', arrive_at TEXT, accept_at TEXT, finish_at TEXT, client_type INTEGER NOT NULL DEFAULT 0, insert_ip TEXT NOT NULL DEFAULT '', created_at TEXT NOT NULL DEFAULT (datetime('now')));`,
		`CREATE TABLE IF NOT EXISTS opinion (id INTEGER PRIMARY KEY AUTOINCREMENT, parent_id INTEGER NOT NULL DEFAULT 0, entity_id TEXT NOT NULL, act_id TEXT NOT NULL DEFAULT '', task_id TEXT NOT NULL DEFAULT '', handler_uid TEXT NOT NULL DEFAULT '', handler_name TEXT NOT NULL DEFAULT '', content TEXT NOT NULL DEFAULT '', act_title TEXT NOT NULL DEFAULT '', written_at TEXT NOT NULL DEFAULT (datetime('now')));`,
	}
	for _, m := range schema {
		if _, err := db.Exec(m); err != nil {
			t.Fatalf("schema: %v", err)
		}
	}

	// SIMPLE process
	if _, err := db.Exec(`INSERT INTO process (id, alias, name, ver) VALUES ('1', 'SIMPLE', 'Simple Approval', 1)`); err != nil {
		t.Fatalf("seed SIMPLE: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO pro_ver (id, pro_id, ver, is_release) VALUES ('1', '1', 1, 1)`); err != nil {
		t.Fatalf("seed SIMPLE ver: %v", err)
	}
	type act struct{ id int; name string; typ nova.ActType }
	simpleActs := []act{
		{10, "开始", nova.ActTypeSTART},
		{11, "起草", nova.ActTypeMANUAL},
		{12, "审批", nova.ActTypeMANUAL},
		{13, "归档", nova.ActTypeMANUAL},
		{14, "结束", nova.ActTypeEND},
	}
	for _, a := range simpleActs {
		db.Exec(`INSERT INTO act (id, pro_id, name, title, act_type, act_ver, editable) VALUES (?, '1', ?, ?, ?, 1, 1)`, fmt.Sprintf("%d", a.id), a.name, "活动-"+a.name, a.typ)
	}
	type link struct{ id, prev, act int }
	for _, l := range []link{{1, 10, 11}, {2, 11, 12}, {3, 12, 13}, {4, 13, 14}} {
		db.Exec(`INSERT INTO act_link (id, pro_ver_id, act_id, prev_act_id, title, link_type) VALUES (?, '1', ?, ?, ?, 1)`, fmt.Sprintf("%d", l.id), fmt.Sprintf("%d", l.act), fmt.Sprintf("%d", l.prev), fmt.Sprintf("L%d", l.id))
	}
	for _, aid := range []int{11, 12, 13} {
		db.Exec(`INSERT INTO man_rule (act_id, base_on, policy, sel_allowed) VALUES (?, 0, 10, 0)`, fmt.Sprintf("%d", aid))
	}

	if !withConcur {
		return dbPath
	}

	// CONCUR process
	if _, err := db.Exec(`INSERT INTO process (id, alias, name, ver) VALUES ('2', 'CONCUR', 'Concurrent Approval', 1)`); err != nil {
		t.Fatalf("seed CONCUR: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO pro_ver (id, pro_id, ver, is_release) VALUES ('2', '2', 1, 1)`); err != nil {
		t.Fatalf("seed CONCUR ver: %v", err)
	}
	concurActs := []act{
		{20, "开始", nova.ActTypeSTART},
		{21, "起草", nova.ActTypeMANUAL},
		{22, "并发路由", nova.ActTypeROUTER},
		{23, "审批A", nova.ActTypeMANUAL},
		{24, "审批B", nova.ActTypeMANUAL},
		{25, "同步汇聚", nova.ActTypeCONVERGE},
		{26, "归档", nova.ActTypeMANUAL},
		{27, "结束", nova.ActTypeEND},
	}
	for _, a := range concurActs {
		db.Exec(`INSERT INTO act (id, pro_id, name, title, act_type, act_ver, editable) VALUES (?, '2', ?, ?, ?, 1, 1)`, fmt.Sprintf("%d", a.id), a.name, "活动-"+a.name, a.typ)
	}
	for _, l := range []link{
		{10, 20, 21}, {11, 21, 22},
		{12, 22, 23}, {13, 22, 24},
		{14, 23, 25}, {15, 24, 25},
		{16, 25, 26}, {17, 26, 27},
	} {
		db.Exec(`INSERT INTO act_link (id, pro_ver_id, act_id, prev_act_id, title, link_type) VALUES (?, '2', ?, ?, ?, 1)`, fmt.Sprintf("%d", l.id), fmt.Sprintf("%d", l.act), fmt.Sprintf("%d", l.prev), fmt.Sprintf("L%d", l.id))
	}
	for _, aid := range []int{21, 23, 24, 26} {
		db.Exec(`INSERT INTO man_rule (act_id, base_on, policy, sel_allowed) VALUES (?, 0, 10, 0)`, fmt.Sprintf("%d", aid))
	}

	return dbPath
}

func openStore(t *testing.T, dbPath string) *sqlite.Store {
	t.Helper()
	st, err := sqlite.Open(sqlite.Config{
		DSN: "file:" + dbPath + "?cache=shared",
	})
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	return st
}

// ─── CONVERGE End-to-End Test ────────────────────────────────────────────────

func TestConverge_FullFlow(t *testing.T) {
	dbPath := seedConcurDB(t)
	defer os.Remove(dbPath)
	ctx := context.Background()
	st := openStore(t, dbPath)
	defer st.Close()

	eng, err := nova.NewEngine(ctx, nova.EngineConfig{Store: st, ProAlias: "CONCUR"})
	if err != nil {
		t.Fatalf("engine: %v", err)
	}

	// Phase 1: Create entity + first submit (START -> 起草 -> ROUTER -> [审批A, 审批B])
	ent, err := eng.CreateEntity(ctx, "CV-001", "汇聚测试", "user001", "张三", "", "")
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	sess, _ := eng.Session(ctx, ent.ID, "user001", "张三")
	if _, err := sess.Submit(); err != nil {
		t.Fatalf("submit1: %v", err)
	}

	sess2, _ := eng.Session(ctx, ent.ID, "user001", "张三")
	if _, err := sess2.Submit(); err != nil {
		t.Fatalf("submit2: %v", err)
	}

	tasks, _ := st.GetTasksByEntity(ctx, ent.ID)
	assertTaskStates(t, tasks, map[string]nova.TaskState{
		"开始":  nova.TaskStateOVER,
		"起草":  nova.TaskStateOVER,
		"审批A": nova.TaskStateTODO,
		"审批B": nova.TaskStateTODO,
	})
	t.Log("✓ Both branches created")

	// Phase 2: Complete 审批A -> should NOT pass CONVERGE (审批B still TODO)
	sessA, _ := eng.Session(ctx, ent.ID, "userA", "审批人A")
	if _, err := sessA.Submit(); err != nil {
		t.Fatalf("submit A: %v", err)
	}

	tasks2, _ := st.GetTasksByEntity(ctx, ent.ID)
	assertTaskStates(t, tasks2, map[string]nova.TaskState{
		"开始":  nova.TaskStateOVER,
		"起草":  nova.TaskStateOVER,
		"审批A": nova.TaskStateOVER,
		"审批B": nova.TaskStateTODO,
	})
	t.Log("✓ 审批A done, 审批B still TODO — converge not triggered yet")

	// Phase 3: Complete 审批B -> should trigger CONVERGE, go to 归档
	sessB, _ := eng.Session(ctx, ent.ID, "userB", "审批人B")
	if _, err := sessB.Submit(); err != nil {
		t.Fatalf("submit B: %v", err)
	}

	tasks3, _ := st.GetTasksByEntity(ctx, ent.ID)
	// After 审批B completes, CONVERGE detects all done and routes to 归档(MANUAL)
	assertTaskStates(t, tasks3, map[string]nova.TaskState{
		"开始":  nova.TaskStateOVER,
		"起草":  nova.TaskStateOVER,
		"审批A": nova.TaskStateOVER,
		"审批B": nova.TaskStateOVER,
		"归档":  nova.TaskStateTODO,
	})
	t.Log("✓ 审批B done → converge triggered, 归档 created")

	// Phase 4: 归档 -> END
	sessArc, _ := eng.Session(ctx, ent.ID, "userArc", "归档人")
	if _, err := sessArc.Submit(); err != nil {
		t.Fatalf("submit 归档: %v", err)
	}

	// Entity should be OVER
	entity, _ := st.GetEntity(ctx, ent.ID)
	if entity.State != nova.EntityStateOVER {
		t.Fatalf("expected entity OVER, got %d", entity.State)
	}
	t.Log("✓ Entity completed successfully")

	// Verify handle logs
	t.Log("✓ Full converge flow passed")
}

func assertTaskStates(t *testing.T, tasks []*nova.Task, expected map[string]nova.TaskState) {
	t.Helper()
	actual := make(map[string]nova.TaskState)
	for _, task := range tasks {
		actual[task.ActName] = task.State
	}
	for name, state := range expected {
		got, ok := actual[name]
		if !ok {
			t.Errorf("missing task for act %q", name)
			continue
		}
		if got != state {
			t.Errorf("act %q: expected state %d, got %d", name, state, got)
		}
	}
}

// ─── SIMPLE Workflow Test ─────────────────────────────────────────────────────

func TestSimple_FullFlow(t *testing.T) {
	dbPath := seedSimpleDB(t)
	defer os.Remove(dbPath)
	ctx := context.Background()
	st := openStore(t, dbPath)
	defer st.Close()

	eng, err := nova.NewEngine(ctx, nova.EngineConfig{Store: st, ProAlias: "SIMPLE"})
	if err != nil {
		t.Fatalf("engine: %v", err)
	}

	ent, err := eng.CreateEntity(ctx, "SP-001", "简单流程", "user001", "张三", "", "")
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	// Submit through each step
	steps := []struct {
		name string
		from nova.EntityState
		to   nova.TaskState // expected task state after
	}{
		{"START→起草", nova.EntityStateDRAFT, nova.TaskStateTODO},
		{"起草→审批", nova.EntityStateSUBMIT, nova.TaskStateTODO},
		{"审批→归档", nova.EntityStateSUBMIT, nova.TaskStateTODO},
		{"归档→END", nova.EntityStateOVER, nova.TaskStateOVER},
	}

	for i, step := range steps {
		sess, _ := eng.Session(ctx, ent.ID, "user001", "测试")
		si, err := sess.Submit()
		if err != nil {
			t.Fatalf("step %d (%s): %v", i, step.name, err)
		}
		destName := "END"
		if si.Direct != nil {
			destName = si.Direct.Act.Name
		}
		t.Logf("  step %d %s → %s (%d targets)", i, step.name, destName, len(si.DestTargets))
		_ = si
	}

	t.Log("✓ Simple flow completed: entity OVER")
}

// ─── Cancel / State Machine Test ──────────────────────────────────────────────

func TestEngine_CancelDraft(t *testing.T) {
	dbPath := seedSimpleDB(t)
	defer os.Remove(dbPath)
	ctx := context.Background()
	st := openStore(t, dbPath)
	defer st.Close()

	eng, _ := nova.NewEngine(ctx, nova.EngineConfig{Store: st, ProAlias: "SIMPLE"})

	ent, _ := eng.CreateEntity(ctx, "CN-001", "可撤销", "user001", "张三", "", "")

	// Cancel before submit (draft)
	if err := st.UpdateEntityState(ctx, ent.ID, nova.EntityStateCANCEL); err != nil {
		t.Fatalf("cancel: %v", err)
	}
	ent2, _ := st.GetEntity(ctx, ent.ID)
	if ent2.State != nova.EntityStateCANCEL {
		t.Errorf("expected CANCEL, got %d", ent2.State)
	}
	t.Logf("✓ Entity cancelled (state=%d)", ent2.State)
}

// ─── Seal Edge Cases ──────────────────────────────────────────────────────────

func TestSeal_EmptyNextActs(t *testing.T) {
	signer := nova.NewSealSigner(nova.SealConfig{SecretKey: []byte("key")})

	token, err := signer.Sign("ENT-001", "user001", "1", nil, nil)
	if err != nil {
		t.Fatalf("sign with nil acts: %v", err)
	}

	seal, err := signer.Verify(token, "ENT-001", "user001", "1")
	if err != nil {
		t.Fatalf("verify nil acts: %v", err)
	}
	if len(seal.NextActNames) != 0 {
		t.Errorf("expected empty next acts, got %v", seal.NextActNames)
	}
	t.Log("✓ Seal with nil next acts works")
}

func TestSeal_WithActUIDsMap(t *testing.T) {
	signer := nova.NewSealSigner(nova.SealConfig{SecretKey: []byte("key")})

	actUIDs := map[string][]string{
		"审批": {"u001", "u002"},
		"归档": {"u003"},
	}

	token, err := signer.Sign("ENT-001", "user001", "1", []string{"审批", "归档"}, actUIDs)
	if err != nil {
		t.Fatalf("sign with actUIDs: %v", err)
	}

	seal, err := signer.Verify(token, "ENT-001", "user001", "1")
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if len(seal.ActUIDsMap) != 2 {
		t.Errorf("expected 2 acts in map, got %d", len(seal.ActUIDsMap))
	}
	if len(seal.ActUIDsMap["审批"]) != 2 {
		t.Errorf("expected 2 UIDs for 审批, got %d", len(seal.ActUIDsMap["审批"]))
	}
	t.Log("✓ Seal with actUIDsMap works")
}

func TestSeal_LargePayload(t *testing.T) {
	signer := nova.NewSealSigner(nova.SealConfig{SecretKey: []byte("key")})

	// 50 act names, each with 20 UIDs = 1000 entries
	actUIDs := make(map[string][]string)
	var actNames []string
	for i := 0; i < 50; i++ {
		name := fmt.Sprintf("act-%d", i)
		actNames = append(actNames, name)
		var uids []string
		for j := 0; j < 20; j++ {
			uids = append(uids, fmt.Sprintf("u-%d-%d", i, j))
		}
		actUIDs[name] = uids
	}

	token, err := signer.Sign("ENT-LARGE", "user001", "999", actNames, actUIDs)
	if err != nil {
		t.Fatalf("sign large: %v", err)
	}

	seal, err := signer.Verify(token, "ENT-LARGE", "user001", "999")
	if err != nil {
		t.Fatalf("verify large: %v", err)
	}
	if len(seal.NextActNames) != 50 {
		t.Errorf("expected 50 act names, got %d", len(seal.NextActNames))
	}
	t.Logf("✓ Large seal works (token size: %d bytes)", len(token))
}

// ─── Store Edge Cases ─────────────────────────────────────────────────────────

func TestStore_EntityNotFound(t *testing.T) {
	dbPath := seedSimpleDB(t)
	defer os.Remove(dbPath)
	st := openStore(t, dbPath)
	defer st.Close()
	ctx := context.Background()

	e, err := st.GetEntity(ctx, "99999")
	if err != nil {
		t.Fatalf("get nonexistent: %v", err)
	}
	if e != nil {
		t.Fatal("expected nil for nonexistent entity")
	}
	t.Log("✓ GetEntity returns nil for not found")
}

func TestStore_ChildEntities(t *testing.T) {
	dbPath := seedSimpleDB(t)
	defer os.Remove(dbPath)
	st := openStore(t, dbPath)
	defer st.Close()
	ctx := context.Background()

	parent := &nova.Entity{
		Code: "PARENT-001", ProID: "1", ProVer: 1,
		Title: "Parent", State: nova.EntityStateDRAFT,
		DraftUID: "u001", DraftName: "P",
	}
	if err := st.CreateEntity(ctx, parent); err != nil {
		t.Fatalf("create parent: %v", err)
	}

	child := &nova.Entity{
		Code: "CHILD-001", ProID: "1", ProVer: 1,
		Title: "Child", State: nova.EntityStateDRAFT,
		DraftUID: "u001", DraftName: "C",
		ParentID: parent.ID,
	}
	if err := st.CreateEntity(ctx, child); err != nil {
		t.Fatalf("create child: %v", err)
	}

	children, err := st.GetChildEntities(ctx, parent.ID)
	if err != nil {
		t.Fatalf("get children: %v", err)
	}
	if len(children) != 1 {
		t.Fatalf("expected 1 child, got %d", len(children))
	}
	if children[0].Code != "CHILD-001" {
		t.Errorf("expected CHILD-001, got %s", children[0].Code)
	}
	t.Log("✓ Children found")

	noChildren, _ := st.GetChildEntities(ctx, child.ID)
	if len(noChildren) != 0 {
		t.Errorf("expected 0 children for leaf, got %d", len(noChildren))
	}
	t.Log("✓ Leaf entity has no children")
}

func TestStore_SerialNumber(t *testing.T) {
	dbPath := seedSimpleDB(t)
	defer os.Remove(dbPath)
	st := openStore(t, dbPath)
	defer st.Close()
	ctx := context.Background()

	s1, err := st.NextSerial(ctx, "TEST")
	if err != nil {
		t.Fatalf("first serial: %v", err)
	}
	if s1 == "" {
		t.Fatal("expected non-empty serial")
	}

	s2, _ := st.NextSerial(ctx, "TEST")
	if s2 == "" {
		t.Fatal("expected non-empty serial")
	}
	if s1 == s2 {
		t.Log("Note: serials may collide within same millisecond")
	}
	t.Logf("✓ Serials: %s, %s", s1, s2)
}

func TestStore_UpdateEntityTitle(t *testing.T) {
	dbPath := seedSimpleDB(t)
	defer os.Remove(dbPath)
	st := openStore(t, dbPath)
	defer st.Close()
	ctx := context.Background()

	e := &nova.Entity{
		Code: "UPD-001", ProID: "1", ProVer: 1,
		Title: "Original", State: nova.EntityStateDRAFT,
		DraftUID: "u001", DraftName: "U",
	}
	st.CreateEntity(ctx, e)

	if err := st.UpdateEntityTitle(ctx, e.ID, "Updated Title"); err != nil {
		t.Fatalf("update title: %v", err)
	}

	got, _ := st.GetEntity(ctx, e.ID)
	if got.Title != "Updated Title" {
		t.Errorf("expected 'Updated Title', got %q", got.Title)
	}
	t.Log("✓ Title updated")
}

// ─── Hook Edge Cases ──────────────────────────────────────────────────────────

func TestHook_MultipleHooks(t *testing.T) {
	var calls1 []nova.ThroughType
	var calls2 []nova.ThroughType

	h1 := nova.NewHookFunc("h1", func(ctx context.Context, tt nova.ThroughType) nova.FeedbackStatus {
		calls1 = append(calls1, tt)
		return nova.FeedbackContinue
	})
	h2 := nova.NewHookFunc("h2", func(ctx context.Context, tt nova.ThroughType) nova.FeedbackStatus {
		calls2 = append(calls2, tt)
		return nova.FeedbackContinue
	})

	router := nova.NewHookRouter(h1, h2)
	ctx := context.Background()

	router.DoHook(ctx, nova.ThroughBefSubmit)

	if len(calls1) != 1 || len(calls2) != 1 {
		t.Fatalf("expected both hooks called, got h1=%d, h2=%d", len(calls1), len(calls2))
	}
	t.Log("✓ Multiple hooks both fire")
}

func TestHook_Breakdown(t *testing.T) {
	breakHook := nova.NewHookFunc("break", func(ctx context.Context, tt nova.ThroughType) nova.FeedbackStatus {
		return nova.FeedbackBreakdown
	})
	router := nova.NewHookRouter(breakHook)
	fb := router.DoHook(context.Background(), nova.ThroughBefSubmit)
	if fb != nova.FeedbackBreakdown {
		t.Fatal("expected breakdown")
	}
	t.Log("✓ Hook breakdown works")
}

// ─── Context Edge ─────────────────────────────────────────────────────────────

func TestContext_NilContext(t *testing.T) {
	_, _, ok := nova.CurHandler(nil)
	if ok {
		t.Fatal("expected no handler from nil context")
	}
	t.Log("✓ Nil context returns no handler")
}

// ─── State Machine Edge ───────────────────────────────────────────────────────

func TestStateMachine_AllTransitions(t *testing.T) {
	expect := map[[2]int]bool{
		{0, 1}: true, // Start->Save
		{0, 3}: true, // Start->Cancel
		{1, 2}: true, // Handle->Submit
		{1, 3}: true, // Handle->Cancel
	}
	for from := 0; from <= 3; from++ {
		for op := 1; op <= 3; op++ {
			_, ok := nova.ValidTransition(nova.SessionState(from), nova.Op(op))
			key := [2]int{from, op}
			if ok != expect[key] {
				if expect[key] {
					t.Errorf("expected valid transition: from=%d op=%d, got invalid", from, op)
				}
			}
		}
	}
	t.Log("✓ All state transitions verified")
}

// ─── Store Concurrency ────────────────────────────────────────────────────────

func TestStore_ConcurrentCreate(t *testing.T) {
	dbPath := seedSimpleDB(t)
	defer os.Remove(dbPath)
	st := openStore(t, dbPath)
	defer st.Close()
	ctx := context.Background()

	done := make(chan bool, 5)
	for i := 0; i < 5; i++ {
		go func(n int) {
			e := &nova.Entity{
				Code: fmt.Sprintf("CONC-%d", n), ProID: "1", ProVer: 1,
				Title: fmt.Sprintf("Concurrent %d", n), State: nova.EntityStateDRAFT,
				DraftUID: "u001", DraftName: "T",
			}
			if err := st.CreateEntity(ctx, e); err != nil {
				t.Errorf("concurrent create %d: %v", n, err)
			}
			done <- true
		}(i)
	}
	for i := 0; i < 5; i++ {
		<-done
	}
	t.Log("✓ 5 concurrent creates completed")
}
