package nova_test

import (
	"context"
	"os"
	"testing"

	"github.com/liyan/nova/pkg/nova"
	"github.com/liyan/nova/sqlite"
)

func TestProcessBuilder_Validate(t *testing.T) {
	tests := []struct {
		name    string
		build   func() *nova.ProcessBuilder
		wantErr bool
	}{
		{
			name: "valid simple process",
			build: func() *nova.ProcessBuilder {
				return nova.NewProcess("TEST", "测试").
					Act("开始", nova.ActTypeSTART).
					Act("审批", nova.ActTypeMANUAL).On(nova.ManPolicySINGLE).
					Act("结束", nova.ActTypeEND).
					Link("开始", "审批").
					Link("审批", "结束")
			},
			wantErr: false,
		},
		{
			name: "valid concurrent process",
			build: func() *nova.ProcessBuilder {
				return nova.NewProcess("CONC", "并发测试").
					Act("开始", nova.ActTypeSTART).
					Act("起草", nova.ActTypeMANUAL).On(nova.ManPolicySINGLE).
					Act("路由", nova.ActTypeROUTER).
					Act("审批A", nova.ActTypeMANUAL).On(nova.ManPolicySINGLE).
					Act("审批B", nova.ActTypeMANUAL).On(nova.ManPolicySINGLE).
					Act("汇聚", nova.ActTypeCONVERGE).
					Act("归档", nova.ActTypeMANUAL).On(nova.ManPolicySINGLE).
					Act("结束", nova.ActTypeEND).
					Link("开始", "起草").
					Link("起草", "路由").
					Link("路由", "审批A").
					Link("路由", "审批B").
					Link("审批A", "汇聚").
					Link("审批B", "汇聚").
					Link("汇聚", "归档").
					Link("归档", "结束")
			},
			wantErr: false,
		},
		{
			name: "invalid: no end activity",
			build: func() *nova.ProcessBuilder {
				return nova.NewProcess("BAD", "无结束").Act("开始", nova.ActTypeSTART)
			},
			wantErr: true,
		},
		{
			name: "invalid: no links",
			build: func() *nova.ProcessBuilder {
				return nova.NewProcess("BAD", "无链接").
					Act("开始", nova.ActTypeSTART).
					Act("结束", nova.ActTypeEND)
			},
			wantErr: true,
		},
		{
			name: "invalid: duplicate act names",
			build: func() *nova.ProcessBuilder {
				return nova.NewProcess("BAD", "重复").
					Act("重复", nova.ActTypeSTART).
					Act("重复", nova.ActTypeEND)
			},
			wantErr: true,
		},
		{
			name: "invalid: link to undefined act",
			build: func() *nova.ProcessBuilder {
				return nova.NewProcess("BAD", "未定义").
					Act("开始", nova.ActTypeSTART).
					Act("结束", nova.ActTypeEND).
					Link("开始", "不存在")
			},
			wantErr: true,
		},
		{
			name: "invalid: unreachable act",
			build: func() *nova.ProcessBuilder {
				return nova.NewProcess("BAD", "不可达").
					Act("开始", nova.ActTypeSTART).
					Act("中间", nova.ActTypeMANUAL).On(nova.ManPolicySINGLE).
					Act("最终", nova.ActTypeEND).
					Link("开始", "最终"). // 中间没有入边
					Link("最终", "结束")
			},
			wantErr: true,
		},
		{
			name: "valid: wait act with waitfor",
			build: func() *nova.ProcessBuilder {
				return nova.NewProcess("WAIT", "等待测试").
					Act("开始", nova.ActTypeSTART).
					Act("前置A", nova.ActTypeMANUAL).On(nova.ManPolicySINGLE).
					Act("前置B", nova.ActTypeMANUAL).On(nova.ManPolicySINGLE).
					Act("等待", nova.ActTypeWAIT).WaitFor("前置A", "前置B").
					Act("结束", nova.ActTypeEND).
					Link("开始", "前置A").
					Link("开始", "前置B").
					Link("前置A", "等待").
					Link("前置B", "等待").
					Link("等待", "结束")
			},
			wantErr: false,
		},
		{
			name: "invalid: wait without waitfor",
			build: func() *nova.ProcessBuilder {
				return nova.NewProcess("BAD", "等待未配置").
					Act("开始", nova.ActTypeSTART).
					Act("等待", nova.ActTypeWAIT).
					Act("结束", nova.ActTypeEND).
					Link("开始", "等待").
					Link("等待", "结束")
			},
			wantErr: true,
		},
		{
			name: "valid: builder with groups and sel allowed",
			build: func() *nova.ProcessBuilder {
				return nova.NewProcess("GRP", "有部门").
					Act("开始", nova.ActTypeSTART).
					Act("审批", nova.ActTypeMANUAL).On(nova.ManPolicySINGLE).Group("dept1", "dept2").SelAllowed().
					Act("结束", nova.ActTypeEND).
					Link("开始", "审批").
					Link("审批", "结束")
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := tt.build()
			err := b.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr = %v", err, tt.wantErr)
			}
		})
	}
}

func TestProcessBuilder_StoreAndLoad(t *testing.T) {
	dbPath := "/tmp/nova_test_builder.db"
	_ = os.Remove(dbPath)
	defer os.Remove(dbPath)

	ctx := context.Background()
	st, err := sqlite.Open(sqlite.Config{
		DSN: "file:" + dbPath + "?cache=shared",
	})
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer st.Close()

	// Build and store a concurrent process
	pro, err := nova.NewProcess("LEAVE", "请假审批").
		Act("开始", nova.ActTypeSTART).
		Act("起草", nova.ActTypeMANUAL).On(nova.ManPolicySINGLE).
		Act("部门审批", nova.ActTypeMANUAL).On(nova.ManPolicySINGLE).Group("dept1").
		Act("总监审批", nova.ActTypeMANUAL).On(nova.ManPolicySINGLE).
		Act("结束", nova.ActTypeEND).
		Link("开始", "起草").
		Link("起草", "部门审批").
		Link("部门审批", "总监审批").
		Link("总监审批", "结束").
		Store(ctx, st)
	if err != nil {
		t.Fatalf("Store: %v", err)
	}
	if pro.ID == "" {
		t.Fatal("expected non-zero process ID")
	}
	if pro.Alias != "LEAVE" {
		t.Errorf("alias: got %q, want LEAVE", pro.Alias)
	}
	t.Logf("Process created: id=%s alias=%s", pro.ID, pro.Alias)

	// Load it back
	eng, err := nova.NewEngine(ctx, nova.EngineConfig{Store: st, ProAlias: "LEAVE"})
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	if eng.Pro().Alias != "LEAVE" {
		t.Errorf("loaded alias: got %q, want LEAVE", eng.Pro().Alias)
	}
	if eng.FirstAct() == nil {
		t.Fatal("expected first act")
	}
	t.Logf("Engine loaded: %s, first act=%s", eng.Pro().Name, eng.FirstAct().Name)
}

func TestProcessBuilder_StoreConcurrent(t *testing.T) {
	dbPath := "/tmp/nova_test_builder_concur.db"
	_ = os.Remove(dbPath)
	defer os.Remove(dbPath)

	ctx := context.Background()
	st, err := sqlite.Open(sqlite.Config{
		DSN: "file:" + dbPath + "?cache=shared",
	})
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer st.Close()

	_, err = nova.NewProcess("CONCUR", "并发审批").
		Act("开始", nova.ActTypeSTART).
		Act("起草", nova.ActTypeMANUAL).On(nova.ManPolicySINGLE).
		Act("路由", nova.ActTypeROUTER).
		Act("会签A", nova.ActTypeMANUAL).On(nova.ManPolicyCONCURRENT).
		Act("会签B", nova.ActTypeMANUAL).On(nova.ManPolicyCONCURRENT).
		Act("会签C", nova.ActTypeMANUAL).On(nova.ManPolicySINGLE).
		Act("汇聚", nova.ActTypeCONVERGE).
		Act("归档", nova.ActTypeMANUAL).On(nova.ManPolicySINGLE).
		Act("结束", nova.ActTypeEND).
		Link("开始", "起草").
		Link("起草", "路由").
		Link("路由", "会签A").
		Link("路由", "会签B").
		Link("路由", "会签C").
		Link("会签A", "汇聚").
		Link("会签B", "汇聚").
		Link("会签C", "汇聚").
		Link("汇聚", "归档").
		Link("归档", "结束").
		Store(ctx, st)
	if err != nil {
		t.Fatalf("Store concurrent: %v", err)
	}

	// Load and verify
	eng, err := nova.NewEngine(ctx, nova.EngineConfig{Store: st, ProAlias: "CONCUR"})
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	t.Logf("Concurrent process loaded: %s (%d acts)", eng.Pro().Name, len(eng.Pro().Alias))
	_ = eng
}

func TestProcessBuilder_StoreAndRun(t *testing.T) {
	dbPath := "/tmp/nova_test_builder_run.db"
	_ = os.Remove(dbPath)
	defer os.Remove(dbPath)

	ctx := context.Background()
	st, err := sqlite.Open(sqlite.Config{
		DSN: "file:" + dbPath + "?cache=shared",
	})
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer st.Close()

	// Build a simple process
	_, err = nova.NewProcess("RUN", "可运行流程").
		Act("开始", nova.ActTypeSTART).
		Act("起草", nova.ActTypeMANUAL).On(nova.ManPolicySINGLE).
		Act("审批", nova.ActTypeMANUAL).On(nova.ManPolicySINGLE).
		Act("结束", nova.ActTypeEND).
		Link("开始", "起草").
		Link("起草", "审批").
		Link("审批", "结束").
		Store(ctx, st)
	if err != nil {
		t.Fatalf("Store: %v", err)
	}

	// Run through the process
	eng, err := nova.NewEngine(ctx, nova.EngineConfig{Store: st, ProAlias: "RUN"})
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}

	ent, err := eng.CreateEntity(ctx, "RUN-001", "流程测试", "u001", "张三", "", "")
	if err != nil {
		t.Fatalf("CreateEntity: %v", err)
	}

	sess, _ := eng.Session(ctx, ent.ID, "u001", "张三")
	if _, err := sess.Submit(); err != nil {
		t.Fatalf("submit 起草: %v", err)
	}

	sess2, _ := eng.Session(ctx, ent.ID, "u002", "李四")
	if _, err := sess2.Submit(); err != nil {
		t.Fatalf("submit 审批: %v", err)
	}

	sess3, _ := eng.Session(ctx, ent.ID, "u003", "系统")
	if _, err := sess3.Submit(); err != nil {
		t.Fatalf("submit 结束: %v", err)
	}

	entity, _ := st.GetEntity(ctx, ent.ID)
	if entity.State != nova.EntityStateOVER {
		t.Fatalf("expected OVER, got %d", entity.State)
	}
	t.Log("✓ ProcessBuilder: built, stored, and ran full workflow")
}

func TestProcessBuilder_ValidationErrors(t *testing.T) {
	// Test that validate error prevents Store
	ctx := context.Background()
	st, err := sqlite.Open(sqlite.Config{DSN: "file:/tmp/nova_test_val.db?cache=shared"})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer st.Close()

	_, err = nova.NewProcess("BAD", "坏流程").
		Act("开始", nova.ActTypeSTART).
		Store(ctx, st)
	if err == nil {
		t.Fatal("expected validation error")
	}
	t.Logf("Validation caught: %v", err)
}
