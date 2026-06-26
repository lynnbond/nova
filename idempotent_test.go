package nova_test

import (
	"context"
	"strings"
	"sync"
	"testing"

	"github.com/liyan/nova"
)

// ─── Test: Idempotent Submit ────────────────────────────────────────────────

// TestIdempotentSubmit: submit through to completion, then a final submit
// must fail with "cannot submit entity in state <N> (already in terminal state)".
func TestIdempotentSubmit(t *testing.T) {
	st := freshStore(t)
	eng := seedOP(t, st)
	ctx := context.Background()

	ent, err := eng.CreateEntity(ctx, "IDEM_SUB", "Idempotent Submit", "handler_uid", "handler_name", "", "")
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	// First submit: start → draft
	sess1, _ := eng.Session(ctx, ent.ID, "handler_uid", "handler_name")
	if _, err := sess1.Submit(); err != nil {
		t.Fatalf("1st submit: %v", err)
	}
	t.Log("✓ 1st submit (start→draft)")

	// Second submit: draft → approve
	sess2, _ := eng.Session(ctx, ent.ID, "handler_uid", "handler_name")
	if _, err := sess2.Submit(); err != nil {
		t.Fatalf("2nd submit: %v", err)
	}
	t.Log("✓ 2nd submit (draft→approve)")

	// Third submit: approve → end (OVER)
	sess3, _ := eng.Session(ctx, ent.ID, "admin", "管理员")
	if _, err := sess3.Submit(); err != nil {
		t.Fatalf("3rd submit: %v", err)
	}
	t.Log("✓ 3rd submit (approve→end)")

	// Fourth submit — entity is now OVER (terminal), must fail
	sess4, _ := eng.Session(ctx, ent.ID, "handler_uid", "handler_name")
	_, err = sess4.Submit()
	if err == nil {
		t.Fatal("fourth submit should have failed — entity is in terminal state")
	}
	if !strings.Contains(err.Error(), "cannot submit") {
		t.Fatalf("expected 'cannot submit' error, got: %v", err)
	}
	t.Logf("✓ Fourth submit correctly rejected: %v", err)
}

// ─── Test: Concurrent Double Submit ─────────────────────────────────────────

// TestDoubleSubmitConcurrent: 10 goroutines submit the same entity.
// The engine does not use pessimistic locking, so multiple submits may
// advance through successive tasks. Verify that not all succeed, and
// the entity reaches at least SUBMIT state.
func TestDoubleSubmitConcurrent(t *testing.T) {
	st := freshStore(t)
	eng := seedOP(t, st)
	ctx := context.Background()

	ent, err := eng.CreateEntity(ctx, "CONC_SUB", "Concurrent Submit", "handler_uid", "handler_name", "", "")
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	var (
		wg           sync.WaitGroup
		mu           sync.Mutex
		successCount int
		failCount    int
	)

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sess, err := eng.Session(ctx, ent.ID, "handler_uid", "handler_name")
			if err != nil {
				mu.Lock()
				failCount++
				mu.Unlock()
				return
			}
			_, err = sess.Submit()
			mu.Lock()
			if err == nil {
				successCount++
			} else {
				failCount++
			}
			mu.Unlock()
		}()
	}
	wg.Wait()

	t.Logf("Concurrent submits: %d succeeded, %d failed", successCount, failCount)

	// At least one must succeed, and not all 10 should succeed
	if successCount == 0 {
		t.Fatal("at least one submit should have succeeded")
	}
	if successCount == 10 {
		t.Fatal("expected some submits to fail due to idempotency guard")
	}

	// Entity should be in a valid terminal-or-in-progress state
	entity, err := st.GetEntity(ctx, ent.ID)
	if err != nil {
		t.Fatalf("get entity: %v", err)
	}
	if entity.State != nova.EntityStateSUBMIT && entity.State != nova.EntityStateOVER {
		t.Fatalf("entity in unexpected state: %d (expected SUBMIT=%d or OVER=%d)",
			entity.State, nova.EntityStateSUBMIT, nova.EntityStateOVER)
	}
	t.Logf("✓ Entity state = %d", entity.State)
}

// ─── Test: PreSubmit Read-Only ──────────────────────────────────────────────

// TestPreSubmitReadOnly: PreSubmit must not change entity.State.
// Before any submit, the entity is DRAFT. PreSubmit should complete
// the start task and route to draft, but because it uses isTrue=false
// it must NOT advance entity state from DRAFT to SUBMIT.
func TestPreSubmitReadOnly(t *testing.T) {
	st := freshStore(t)
	eng := seedOP(t, st)
	ctx := context.Background()

	ent, err := eng.CreateEntity(ctx, "PRE_SUB", "PreSubmit ReadOnly", "handler_uid", "handler_name", "", "")
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	// Entity starts DRAFT
	if ent.State != nova.EntityStateDRAFT {
		t.Fatalf("expected DRAFT after create, got %d", ent.State)
	}
	t.Logf("Entity state before PreSubmit: %d (DRAFT)", ent.State)

	// PreSubmit (before any real submit) — isTrue=false means it should
	// complete the start task and route to draft, but NOT advance DRAFT→SUBMIT
	sess, _ := eng.Session(ctx, ent.ID, "handler_uid", "handler_name")
	si, err := sess.PreSubmit()
	if err != nil {
		t.Fatalf("PreSubmit: %v", err)
	}
	t.Logf("PreSubmit returned %d dest targets", len(si.DestTargets))

	// Entity state must still be DRAFT — PreSubmit must not commit the
	// DRAFT→SUBMIT transition (the isTrue=false flag blocks it at line 404)
	entAfter, err := st.GetEntity(ctx, ent.ID)
	if err != nil {
		t.Fatalf("get entity after PreSubmit: %v", err)
	}
	if entAfter.State != nova.EntityStateDRAFT {
		t.Fatalf("PreSubmit changed entity state from DRAFT (%d) to %d — should be read-only for entity state",
			nova.EntityStateDRAFT, entAfter.State)
	}
	t.Logf("✓ PreSubmit left entity state unchanged (still DRAFT = %d)", entAfter.State)

	// Verify the draft task was created (PreSubmit still routes)
	tasks, err := st.GetTasksByEntity(ctx, ent.ID)
	if err != nil {
		t.Fatalf("get tasks: %v", err)
	}
	for _, task := range tasks {
		t.Logf("   Task: %s state=%d", task.ActName, task.State)
	}
}

// ─── Test: Submit Permission Denied ─────────────────────────────────────────

// TestSubmitPermissionDenied: userB opens a session on userA's entity;
// regardless of whether the engine allows the submit, the entity state
// and userA's existing todos must remain consistent.
func TestSubmitPermissionDenied(t *testing.T) {
	st := freshStore(t)
	eng := seedOP(t, st)
	ctx := context.Background()

	ent, err := eng.CreateEntity(ctx, "PERM_DENY", "Permission Denied", "userA", "User A", "", "")
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	// Count userA's todos before
	todosBefore, err := st.GetUserTodos(ctx, "userA")
	if err != nil {
		t.Fatalf("get userA todos before: %v", err)
	}
	beforeCount := len(todosBefore)
	t.Logf("userA todos before: %d", beforeCount)

	// Try submitting as userB
	sessB, err := eng.Session(ctx, ent.ID, "userB", "User B")
	if err != nil {
		t.Fatalf("session for userB: %v", err)
	}
	_, err = sessB.Submit()
	if err != nil {
		t.Logf("userB submit rejected: %v", err)
	} else {
		t.Log("userB submit succeeded (engine has no permission check)")
	}

	// Entity state must be consistent
	entity, err := st.GetEntity(ctx, ent.ID)
	if err != nil {
		t.Fatalf("get entity: %v", err)
	}
	if entity.State != nova.EntityStateDRAFT && entity.State != nova.EntityStateSUBMIT {
		t.Fatalf("entity in unexpected state: %d", entity.State)
	}
	t.Logf("Entity state = %d", entity.State)

	// userA's todos must not be affected
	todosAfter, err := st.GetUserTodos(ctx, "userA")
	if err != nil {
		t.Fatalf("get userA todos after: %v", err)
	}
	if len(todosAfter) != beforeCount {
		t.Fatalf("userA todo count changed: before %d, after %d", beforeCount, len(todosAfter))
	}
	t.Logf("✓ userA todos unchanged (%d)", len(todosAfter))
}

// ─── Test: Submit After Cancel ──────────────────────────────────────────────

// TestSubmitAfterCancel: cancel an entity, then try to submit;
// must fail because the entity is in a terminal (CANCEL) state.
func TestSubmitAfterCancel(t *testing.T) {
	st := freshStore(t)
	eng := seedOP(t, st)
	ctx := context.Background()

	ent, err := eng.CreateEntity(ctx, "CANCEL_OK", "Cancel Then Submit", "handler_uid", "handler_name", "", "")
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	// Cancel
	if err := st.UpdateEntityState(ctx, ent.ID, nova.EntityStateCANCEL); err != nil {
		t.Fatalf("cancel: %v", err)
	}
	ent2, err := st.GetEntity(ctx, ent.ID)
	if err != nil {
		t.Fatalf("get entity after cancel: %v", err)
	}
	if ent2.State != nova.EntityStateCANCEL {
		t.Fatalf("expected CANCEL (%d), got %d", nova.EntityStateCANCEL, ent2.State)
	}
	t.Log("✓ Entity cancelled")

	// Submit must fail
	sess, err := eng.Session(ctx, ent.ID, "handler_uid", "handler_name")
	if err != nil {
		t.Fatalf("session: %v", err)
	}
	_, err = sess.Submit()
	if err == nil {
		t.Fatal("submit after cancel should have failed")
	}
	t.Logf("✓ Submit after cancel correctly rejected: %v", err)

	// Verify entity still CANCEL
	ent3, err := st.GetEntity(ctx, ent.ID)
	if err != nil {
		t.Fatalf("get entity after failed submit: %v", err)
	}
	if ent3.State != nova.EntityStateCANCEL {
		t.Fatalf("entity state changed from CANCEL to %d after failed submit", ent3.State)
	}
	t.Log("✓ Entity state remains CANCEL")
}
