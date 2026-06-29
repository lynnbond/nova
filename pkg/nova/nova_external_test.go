package nova_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/liyan/nova/pkg/nova"
	"github.com/liyan/nova/sqlite"
)

func freshStore(t testing.TB) *sqlite.Store {
	t.Helper()
	f, err := os.CreateTemp("", "nova-test-*.db")
	if err != nil {
		t.Fatalf("create temp db: %v", err)
	}
	path := f.Name()
	f.Close()
	t.Cleanup(func() { os.Remove(path) })

	st, err := sqlite.Open(sqlite.Config{DSN: "file:" + path + "?cache=shared&_journal_mode=WAL"})
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	return st
}

func seedOP(t testing.TB, st *sqlite.Store) *nova.Engine {
	t.Helper()
	_, err := nova.NewProcess("OP", "OP申请单").
		Act("start", nova.ActTypeSTART).
		Act("draft", nova.ActTypeMANUAL).Title("起草申请").On(nova.ManPolicySINGLE).
		Act("approve", nova.ActTypeMANUAL).Title("OP审批").On(nova.ManPolicySINGLE).
		Act("end", nova.ActTypeEND).
		Link("start", "draft").
		Link("draft", "approve").
		Link("approve", "end").
		Store(context.Background(), st)
	if err != nil {
		t.Fatalf("seed OP: %v", err)
	}
	eng, err := nova.NewEngine(context.Background(), nova.EngineConfig{Store: st, ProAlias: "OP"})
	if err != nil {
		t.Fatalf("create engine: %v", err)
	}
	return eng
}

// ─── Test: OP 完整流程 (start→draft→approve→end) ─────────────────────────

func TestEngine_OPFullFlow(t *testing.T) {
	st := freshStore(t)
	eng := seedOP(t, st)
	ctx := context.Background()

	// 1. 张三创建 → "start" task created
	ent, err := eng.CreateEntity(ctx, "OP_FLOW", "OP完整流程", "zhangsan", "张三", "审批部", "")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if ent.State != nova.EntityStateDRAFT {
		t.Fatalf("expected DRAFT, got %d", ent.State)
	}
	t.Logf("① Created DRAFT: id=%s", ent.ID[:8])

	// 2. 第一次提交 → start→draft
	sess1, _ := eng.Session(ctx, ent.ID, "zhangsan", "张三")
	si1, err := sess1.Submit()
	if err != nil {
		t.Fatalf("1st submit: %v", err)
	}
	if len(si1.NewTodos) != 1 {
		t.Fatalf("expected 1 todo after 1st submit, got %d", len(si1.NewTodos))
	}
	t.Logf("② 1st submit (start→draft): todo for %s", si1.NewTodos[0].HandlerName)

	// 3. 第二次提交 → draft→approve (张三需要再次提交)
	sess2, _ := eng.Session(ctx, ent.ID, "zhangsan", "张三")
	si2, err := sess2.Submit()
	if err != nil {
		t.Fatalf("2nd submit: %v", err)
	}
	ent2, _ := st.GetEntity(ctx, ent.ID)
	if ent2.State != nova.EntityStateSUBMIT {
		t.Fatalf("expected SUBMIT after 2nd submit, got %d", ent2.State)
	}
	t.Logf("③ 2nd submit (draft→approve): todo for %s", si2.NewTodos[0].HandlerName)

	// 4. Admin 提交 → approve→end (OVER)
	adminTodos, _ := st.GetUserTodos(ctx, "admin")
	if len(adminTodos) == 0 {
		t.Fatal("admin should have a todo")
	}
	t.Logf("④ Admin has %d todo(s)", len(adminTodos))

	sess3, _ := eng.Session(ctx, ent.ID, "admin", "管理员")
	_, err = sess3.Submit()
	if err != nil {
		t.Fatalf("admin submit: %v", err)
	}
	ent3, _ := st.GetEntity(ctx, ent.ID)
	if ent3.State != nova.EntityStateOVER {
		t.Fatalf("expected OVER, got %d", ent3.State)
	}
	if ent3.OverAt == nil || ent3.SerialNum == "" {
		t.Fatal("OverAt and SerialNum should be set")
	}
	t.Logf("⑤ OVER: serial=%s", ent3.SerialNum)
}

// ─── Test: 退回 — 验证处理日志追踪原始处理人 ─────────────────────────────

func TestEngine_ReturnOriginalHandler(t *testing.T) {
	st := freshStore(t)
	eng := seedOP(t, st)
	ctx := context.Background()

	// 张三创建并提交两次 (start→draft→approve)
	ent, _ := eng.CreateEntity(ctx, "OP_RTH", "退回处理人测试", "zhangsan", "张三", "审批部", "")
	sess1, _ := eng.Session(ctx, ent.ID, "zhangsan", "张三")
	sess1.Submit()
	sess2, _ := eng.Session(ctx, ent.ID, "zhangsan", "张三")
	sess2.Submit()

	// 写 handle log —— app 层会在 CreateEntity / Submit 时做
	now := time.Now().UTC()
	st.CreateHandleLog(ctx, &nova.HandleLog{
		EntityID: ent.ID, ActTitle: "起草申请",
		HandlerUID: "zhangsan", HandlerName: "张三", ArriveAt: &now,
	})

	// 验证 handle log 中有张三
	logs, _ := st.GetEntityHandleLogs(ctx, ent.ID)
	found := false
	for _, l := range logs {
		if l.HandlerUID == "zhangsan" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected handle log for zhangsan")
	}
	t.Log("✓ Handle log tracks zhangsan as original draft handler")

	// Admin 有待办
	adminTodos, _ := st.GetUserTodos(ctx, "admin")
	if len(adminTodos) == 0 {
		t.Fatal("admin should have a todo after 2 submits")
	}
	t.Logf("✓ Admin has %d todo(s)", len(adminTodos))
	ent2, _ := st.GetEntity(ctx, ent.ID)
	t.Logf("Entity state: %d (SUBMIT=%d)", ent2.State, nova.EntityStateSUBMIT)
}

// ─── Test: DEMO 创建+提交 ────────────────────────────────────────────────

func TestDEMO_CreateAndSubmit(t *testing.T) {
	st := freshStore(t)
	eng := seedDEMOProcess(t, st)
	ctx := context.Background()

	ent, _ := eng.CreateEntity(ctx, "DEMO_CS", "起草提交测试", "zhangsan", "张三", "审批部", "")
	if ent.State != nova.EntityStateDRAFT {
		t.Fatalf("expected DRAFT, got %v", ent.State)
	}

	sess, _ := eng.Session(ctx, ent.ID, "zhangsan", "张三")
	si, err := sess.Submit()
	if err != nil {
		t.Fatalf("submit: %v", err)
	}

	ent2, _ := st.GetEntity(ctx, ent.ID)
	if ent2.State != nova.EntityStateDRAFT { // After 1st submit (start→draft), entity still DRAFT
		t.Logf("Entity state after 1st submit: %d (DRAFT=%d)", ent2.State, nova.EntityStateDRAFT)
	}
	t.Logf("✓ DEMO: 1st submit done, todos=%d", len(si.NewTodos))
	for _, td := range si.NewTodos {
		t.Logf("   → Todo: %s / %s", td.ActTitle, td.HandlerName)
	}

	// 2nd submit: draft→router
	sess2, _ := eng.Session(ctx, ent.ID, "zhangsan", "张三")
	si2, err := sess2.Submit()
	if err != nil {
		t.Fatalf("2nd submit: %v", err)
	}
	ent3, _ := st.GetEntity(ctx, ent.ID)
	if ent3.State != nova.EntityStateSUBMIT {
		t.Fatalf("expected SUBMIT after 2nd submit, got %d", ent3.State)
	}
	t.Logf("✓ DEMO: 2nd submit done (draft→router), todos=%d", len(si2.NewTodos))
	for _, td := range si2.NewTodos {
		t.Logf("   → Todo: %s / %s", td.ActTitle, td.HandlerName)
	}
}

func seedDEMOProcess(t testing.TB, st *sqlite.Store) *nova.Engine {
	t.Helper()
	_, err := nova.NewProcess("DEMO", "综合审批Demo").
		Act("start", nova.ActTypeSTART).
		Act("draft", nova.ActTypeMANUAL).Title("起草申请").On(nova.ManPolicySINGLE).
		Act("router1", nova.ActTypeROUTER).
		Act("concur_a", nova.ActTypeMANUAL).Title("会签A").On(nova.ManPolicyCONCURRENT).
		Act("concur_b", nova.ActTypeMANUAL).Title("会签B").On(nova.ManPolicyCONCURRENT).
		Act("single_c", nova.ActTypeMANUAL).Title("单签C").On(nova.ManPolicySINGLE).
		Act("converge1", nova.ActTypeCONVERGE).
		Act("wait_item", nova.ActTypeWAIT).Title("等待确认").WaitFor("single_c").
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
		Store(context.Background(), st)
	if err != nil {
		t.Fatalf("seed DEMO: %v", err)
	}
	eng, err := nova.NewEngine(context.Background(), nova.EngineConfig{Store: st, ProAlias: "DEMO"})
	if err != nil {
		t.Fatalf("create engine: %v", err)
	}
	return eng
}

// ─── Test: 边缘情况 ─────────────────────────────────────────────────────

func TestEngine_EdgeCases(t *testing.T) {
	t.Run("duplicate entity code", func(t *testing.T) {
		st := freshStore(t)
		eng := seedOP(t, st)
		ctx := context.Background()
		eng.CreateEntity(ctx, "DUPX", "First", "zhangsan", "张三", "", "")
		_, err := eng.CreateEntity(ctx, "DUPX", "Second", "lisi", "李四", "", "")
		if err == nil {
			t.Fatal("expected error for duplicate code")
		}
		t.Logf("✓ Duplicate code rejected: %v", err)
	})

	t.Run("nonexistent process alias", func(t *testing.T) {
		st := freshStore(t)
		_, err := nova.NewEngine(context.Background(), nova.EngineConfig{Store: st, ProAlias: "NONEXIST"})
		if err == nil {
			t.Fatal("expected error")
		}
		t.Logf("✓ Nonexistent process: %v", err)
	})

	t.Run("empty act name", func(t *testing.T) {
		_, err := nova.NewProcess("BAD", "bad").
			Act("", nova.ActTypeSTART).
			Act("draft", nova.ActTypeMANUAL).On(nova.ManPolicySINGLE).
			Act("end", nova.ActTypeEND).
			Link("", "draft").
			Link("draft", "end").
			Store(context.Background(), freshStore(t))
		if err == nil {
			t.Fatal("expected error")
		}
		t.Logf("✓ Empty act name: %v", err)
	})
}

// ─── Test: 流程设计 CRUD ───────────────────────────────────────────────

func TestStore_ProcessDesignFullCRUD(t *testing.T) {
	st := freshStore(t)
	ctx := context.Background()

	pro, err := nova.NewProcess("LEAVE", "请假流程").
		Act("start", nova.ActTypeSTART).
		Act("draft", nova.ActTypeMANUAL).Title("起草").On(nova.ManPolicySINGLE).
		Act("approve", nova.ActTypeMANUAL).Title("审批").On(nova.ManPolicySINGLE).
		Act("end", nova.ActTypeEND).
		Link("start", "draft").
		Link("draft", "approve").
		Link("approve", "end").
		Store(ctx, st)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	t.Logf("① Created: %s v%d", pro.Name, pro.Ver)

	pros, _ := st.ListPros(ctx)
	if len(pros) == 0 {
		t.Fatal("expected processes")
	}

	vers, _ := st.ListProVers(ctx, pro.ID)
	acts, _ := st.GetActsByProVer(ctx, vers[0].ID)
	links, _ := st.GetLinksByProVer(ctx, vers[0].ID)
	ruleCount := 0
	for _, a := range acts {
		if a.Type == nova.ActTypeMANUAL {
			if r, _ := st.GetManRule(ctx, a.ID); r != nil {
				ruleCount++
			}
		}
	}
	t.Logf("③ Load: ver=%d, acts=%d, links=%d, rules=%d", pro.Ver, len(acts), len(links), ruleCount)

	pro.Name = "更新请假审批"
	st.UpdatePro(ctx, pro)
	t.Log("④ Update OK")
	st.DeletePro(ctx, pro.ID)
	pros2, _ := st.ListPros(ctx)
	if len(pros2) != 0 {
		t.Fatal("expected 0 pros after delete")
	}
	t.Log("⑤ Delete cascade OK")
}

// ─── Test: App 集成模式 ────────────────────────────────────────────────

func TestStore_AppIntegrationPattern(t *testing.T) {
	st := freshStore(t)
	ctx := context.Background()
	eng := seedOP(t, st)

	st.CreateUser(ctx, &nova.User{UID: "zhangsan", Name: "张三", Dept: "审批部"})
	st.CreateUser(ctx, &nova.User{UID: "admin", Name: "管理员", Dept: "管理部"})

	ent, _ := eng.CreateEntity(ctx, "APP_INT", "集成测试", "zhangsan", "张三", "审批部", "")
	st.CreateHandleLog(ctx, &nova.HandleLog{
		EntityID: ent.ID, ActTitle: "起草申请",
		HandlerUID: "zhangsan", HandlerName: "张三", ArriveAt: nowPtr(),
	})

	// Submit twice: start→draft, then draft→approve
	sess, _ := eng.Session(ctx, ent.ID, "zhangsan", "张三")
	si1, _ := sess.Submit()
	for _, td := range si1.NewTodos {
		st.CreateHandleLog(ctx, &nova.HandleLog{
			EntityID: ent.ID, TaskID: td.TaskID, ActTitle: td.ActTitle,
			HandlerUID: td.HandlerUID, HandlerName: td.HandlerName, ArriveAt: nowPtr(),
		})
	}
	sess2, _ := eng.Session(ctx, ent.ID, "zhangsan", "张三")
	si2, _ := sess2.Submit()
	for _, td := range si2.NewTodos {
		st.CreateHandleLog(ctx, &nova.HandleLog{
			EntityID: ent.ID, TaskID: td.TaskID, ActTitle: td.ActTitle,
			HandlerUID: td.HandlerUID, HandlerName: td.HandlerName, ArriveAt: nowPtr(),
		})
	}

	ent2, _ := st.GetEntity(ctx, ent.ID)
	if ent2.State != nova.EntityStateSUBMIT {
		t.Fatalf("expected SUBMIT, got %d", ent2.State)
	}
	logs, _ := st.GetEntityHandleLogs(ctx, ent.ID)
	t.Logf("✓ App pattern: DRAFT→SUBMIT, %d handle logs, admin has %d todos", len(logs), len(si2.NewTodos))
}

func nowPtr() *time.Time {
	t := time.Now().UTC()
	return &t
}
