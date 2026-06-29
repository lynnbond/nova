package nova_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/liyan/nova/pkg/nova"
	"github.com/liyan/nova/sqlite"
)

// ─── Fixtures ─────────────────────────────────────────────────────────────────

func setupProcess(t *testing.T) (nova.Store, string) {
	t.Helper()

	dbPath := "/tmp/nova_test_" + t.Name() + ".db"
	_ = os.Remove(dbPath)

	st, err := sqlite.Open(sqlite.Config{
		DSN: "file:" + dbPath + "?cache=shared&_journal_mode=WAL&_busy_timeout=5000",
	})
	if err != nil {
		t.Fatalf("sqlite open: %v", err)
	}
	t.Cleanup(func() { st.Close(); os.Remove(dbPath) })

	return st, "TEST"
}

// ─── Compilation Test ─────────────────────────────────────────────────────────

func TestTypesCompile(t *testing.T) {
	_ = nova.ActTypeSTART
	_ = nova.ActTypeMANUAL
	_ = nova.ActTypeROUTER
	_ = nova.ActTypeCONVERGE
	_ = nova.ActTypeWAIT
	_ = nova.ActTypeTAIL
	_ = nova.ActTypeEND

	_ = nova.TaskStateINITIAL
	_ = nova.TaskStateCONVERGING
	_ = nova.TaskStateWAITING
	_ = nova.TaskStateTODO
	_ = nova.TaskStateOVER

	_ = nova.EntityStateDRAFT
	_ = nova.EntityStateSUBMIT
	_ = nova.EntityStateOVER

	_ = nova.ManPolicySINGLE
	_ = nova.ManPolicyCONCURRENT
	_ = nova.ManPolicyEXCLUSIVE

	_ = nova.LinkTypeFORWARD
	_ = nova.LinkTypeBACKWARD

	s := nova.SessionStateStart
	_ = nova.SessionStateHandle
	_ = nova.SessionStateView
	_ = nova.SessionStateDeleted
	_ = s
}

// ─── Seal Test ────────────────────────────────────────────────────────────────

func TestSealSignAndVerify(t *testing.T) {
	signer := nova.NewSealSigner(nova.SealConfig{
		SecretKey: []byte("test-secret-key-32bytes!"),
		TTL:       5 * time.Second,
	})

	token, err := signer.Sign("ENT-001", "user001", "42", []string{"审批"}, nil)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}

	seal, err := signer.Verify(token, "ENT-001", "user001", "42")
	if err != nil {
		t.Fatalf("verify: %v", err)
	}

	if seal.EntityCode != "ENT-001" {
		t.Errorf("expected ENT-001, got %s", seal.EntityCode)
	}
	if seal.HandlerUID != "user001" {
		t.Errorf("expected user001, got %s", seal.HandlerUID)
	}
	if seal.TaskID != "42" {
		t.Errorf("expected 42, got %s", seal.TaskID)
	}
	if len(seal.NextActNames) != 1 || seal.NextActNames[0] != "审批" {
		t.Errorf("expected [审批], got %v", seal.NextActNames)
	}
}

func TestSealExpiry(t *testing.T) {
	signer := nova.NewSealSigner(nova.SealConfig{
		SecretKey: []byte("test-key"),
		TTL:       1 * time.Nanosecond,
	})

	token, err := signer.Sign("ENT-001", "user001", "42", nil, nil)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}

	time.Sleep(10 * time.Millisecond)

	_, err = signer.Verify(token, "ENT-001", "user001", "42")
	if err == nil {
		t.Fatal("expected expiry error")
	}
	t.Logf("expected expiry: %v", err)
}

func TestSealTamper(t *testing.T) {
	signer := nova.NewSealSigner(nova.SealConfig{
		SecretKey: []byte("test-secret"),
	})

	token, err := signer.Sign("ENT-001", "user001", "42", nil, nil)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}

	_, err = signer.Verify(token, "ENT-001", "attacker", "42")
	if err == nil {
		t.Fatal("expected tamper error")
	}
	t.Logf("expected tamper: %v", err)

	_, err = signer.Verify(token, "ENT-999", "user001", "42")
	if err == nil {
		t.Fatal("expected tamper error")
	}
	t.Logf("expected tamper: %v", err)
}

// ─── Hook Test ────────────────────────────────────────────────────────────────

func TestHookRouter(t *testing.T) {
	var hookCalls []nova.ThroughType

	hook := nova.NewHookFunc("test-hook", func(ctx context.Context, tt nova.ThroughType) nova.FeedbackStatus {
		hookCalls = append(hookCalls, tt)
		return nova.FeedbackContinue
	})

	router := nova.NewHookRouter(hook)

	ctx := context.Background()
	fb := router.DoHook(ctx, nova.ThroughBefSubmit)
	if fb != nova.FeedbackContinue {
		t.Fatalf("expected continue, got %d", fb)
	}

	fb = router.DoHook(ctx, nova.ThroughBefChooseLinks)
	if fb != nova.FeedbackContinue {
		t.Fatalf("expected continue, got %d", fb)
	}

	if len(hookCalls) != 2 {
		t.Fatalf("expected 2 hook calls, got %d", len(hookCalls))
	}
	if hookCalls[0] != nova.ThroughBefSubmit {
		t.Errorf("expected ThroughBefSubmit(310), got %d", hookCalls[0])
	}
}

func TestHookAbandon(t *testing.T) {
	abortHook := nova.NewHookFunc("abort", func(ctx context.Context, tt nova.ThroughType) nova.FeedbackStatus {
		return nova.FeedbackAbandon
	})

	router := nova.NewHookRouter(abortHook)

	fb := router.DoHook(context.Background(), nova.ThroughBefSubmit)
	if fb != nova.FeedbackAbandon {
		t.Fatal("expected abandon")
	}
}

// ─── State Machine Test ───────────────────────────────────────────────────────

func TestStateMachineValidTransition(t *testing.T) {
	tests := []struct {
		from   nova.SessionState
		op     nova.Op
		expect nova.SessionState
		valid  bool
	}{
		{nova.SessionStateStart, nova.OpSave, nova.SessionStateHandle, true},
		{nova.SessionStateStart, nova.OpCancel, nova.SessionStateDeleted, true},
		{nova.SessionStateHandle, nova.OpSubmit, nova.SessionStateView, true},
		{nova.SessionStateHandle, nova.OpCancel, nova.SessionStateDeleted, true},
		{nova.SessionStateView, nova.OpSubmit, 0, false},
		{nova.SessionStateDeleted, nova.OpSubmit, 0, false},
		{nova.SessionStateStart, nova.OpSubmit, 0, false},
	}

	for _, tt := range tests {
		next, ok := nova.ValidTransition(tt.from, tt.op)
		if ok != tt.valid {
			t.Errorf("ValidTransition(%d,%d): got ok=%v, want %v", tt.from, tt.op, ok, tt.valid)
		}
		if ok && next != tt.expect {
			t.Errorf("ValidTransition(%d,%d): got next=%d, want %d", tt.from, tt.op, next, tt.expect)
		}
	}
}

// ─── Store Test ───────────────────────────────────────────────────────────────

func TestSQLiteStoreCreateEntity(t *testing.T) {
	st, _ := setupProcess(t)
	ctx := context.Background()

	e := &nova.Entity{
		Code:      "TEST-001",
		ProID:     "1",
		ProVer:    1,
		Title:     "测试工单",
		State:     nova.EntityStateDRAFT,
		DraftUID:  "user001",
		DraftName: "测试用户",
	}
	if err := st.CreateEntity(ctx, e); err != nil {
		t.Fatalf("create entity: %v", err)
	}
	if e.ID == "" {
		t.Fatal("expected non-zero ID")
	}

	got, err := st.GetEntity(ctx, e.ID)
	if err != nil {
		t.Fatalf("get entity: %v", err)
	}
	if got.Title != "测试工单" {
		t.Errorf("title: got %q, want 测试工单", got.Title)
	}
	if got.State != nova.EntityStateDRAFT {
		t.Errorf("state: got %d, want DRAFT", got.State)
	}
}

func TestSQLiteStoreTasks(t *testing.T) {
	st, _ := setupProcess(t)
	ctx := context.Background()

	e := &nova.Entity{
		Code:      "TEST-002",
		ProID:     "1",
		ProVer:    1,
		Title:     "Test Two",
		DraftUID:  "user001",
		DraftName: "测试用户",
	}
	if err := st.CreateEntity(ctx, e); err != nil {
		t.Fatalf("create entity: %v", err)
	}

	task := &nova.Task{
		EntityID: e.ID,
		StepID:   1,
		ActID:    "1",
		ActName:  "开始",
		ActTitle: "开始活动",
		State:    nova.TaskStateTODO,
	}
	if err := st.CreateTask(ctx, task); err != nil {
		t.Fatalf("create task: %v", err)
	}
	if task.ID == "" {
		t.Fatal("expected non-zero task ID")
	}

	tasks, err := st.GetTasksByEntity(ctx, e.ID)
	if err != nil {
		t.Fatalf("get tasks: %v", err)
	}
	if len(tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(tasks))
	}

	if err := st.UpdateTaskState(ctx, task.ID, nova.TaskStateOVER); err != nil {
		t.Fatalf("update task state: %v", err)
	}
	updated, err := st.GetTask(ctx, task.ID)
	if err != nil {
		t.Fatalf("get updated task: %v", err)
	}
	if updated.State != nova.TaskStateOVER {
		t.Errorf("expected OVER, got %d", updated.State)
	}
}

func TestSQLiteStoreTodos(t *testing.T) {
	st, _ := setupProcess(t)
	ctx := context.Background()

	e := &nova.Entity{
		Code:      "TEST-003",
		ProID:     "1",
		ProVer:    1,
		Title:     "Test Three",
		DraftUID:  "user001",
		DraftName: "用户",
	}
	if err := st.CreateEntity(ctx, e); err != nil {
		t.Fatalf("create entity: %v", err)
	}

	task := &nova.Task{
		EntityID: e.ID,
		ActID:    "2",
		ActName:  "审批",
		State:    nova.TaskStateTODO,
	}
	if err := st.CreateTask(ctx, task); err != nil {
		t.Fatalf("create task: %v", err)
	}

	todo := &nova.Todo{
		TaskID:      task.ID,
		EntityID:    e.ID,
		ActID:       "2",
		ProID:       "1",
		HandlerUID:  "user002",
		HandlerName: "审批人",
		ActTitle:    "审批",
		EntityTitle: e.Title,
	}
	if err := st.CreateTodo(ctx, todo); err != nil {
		t.Fatalf("create todo: %v", err)
	}

	// Query user's todos
	todos, err := st.GetUserTodos(ctx, "user002")
	if err != nil {
		t.Fatalf("get user todos: %v", err)
	}
	if len(todos) != 1 {
		t.Fatalf("expected 1 todo, got %d", len(todos))
	}

	// Accept
	if err := st.AcceptTodo(ctx, todo.ID); err != nil {
		t.Fatalf("accept: %v", err)
	}
	accepted, err := st.GetUserTodos(ctx, "user002")
	if err != nil {
		t.Fatalf("get after accept: %v", err)
	}
	if len(accepted) > 0 && !accepted[0].Accepted {
		t.Error("expected accepted=true")
	}
}

func TestSQLiteStoreOpinions(t *testing.T) {
	st, _ := setupProcess(t)
	ctx := context.Background()

	e := &nova.Entity{
		Code:      "TEST-004",
		ProID:     "1",
		ProVer:    1,
		Title:     "Test Opinion",
		DraftUID:  "user001",
		DraftName: "用户",
	}
	if err := st.CreateEntity(ctx, e); err != nil {
		t.Fatalf("create entity: %v", err)
	}

	o := &nova.Opinion{
		EntityID:    e.ID,
		ActID:       "2",
		HandlerUID:  "user001",
		HandlerName: "用户",
		Content:     "通过，同意。",
		ActTitle:    "审批",
	}
	if err := st.CreateOpinion(ctx, o); err != nil {
		t.Fatalf("create opinion: %v", err)
	}

	opinions, err := st.GetEntityOpinions(ctx, e.ID)
	if err != nil {
		t.Fatalf("get opinions: %v", err)
	}
	if len(opinions) != 1 {
		t.Fatalf("expected 1 opinion, got %d", len(opinions))
	}
	if opinions[0].Content != "通过，同意。" {
		t.Errorf("content: got %q, want 通过", opinions[0].Content)
	}
}
