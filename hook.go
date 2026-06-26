package nova

import "context"

// ─── Hook System ──────────────────────────────────────────────────────────────

// ThroughType defines the point in the submission lifecycle where a hook fires.
type ThroughType int

const (
	ThroughBefSubmit        ThroughType = 310 // Before submit logic starts
	ThroughBefChooseLinks   ThroughType = 320 // Before choosing next links
	ThroughBefChooseHandler ThroughType = 330 // Before choosing handler
	ThroughBefSubmitCommit  ThroughType = 340 // Before submit transaction commit
	ThroughAftSubmitCommit  ThroughType = 350 // After submit transaction commit
	ThroughAftSubmitSuccess ThroughType = 352 // After successful submit commit
	ThroughAftSubmitFail    ThroughType = 354 // After failed submit commit
)

// Hook defines a callback that fires at a specific point in the lifecycle.
type Hook interface {
	// ID returns a unique identifier for this hook.
	ID() string
	// Handle is called at the specified through-point.
	// Return FeedbackAbandon to abort, FeedbackBreakdown to error, FeedbackContinue to proceed.
	Handle(ctx context.Context, through ThroughType) FeedbackStatus
}

// HookFunc is a function-based Hook adapter.
type HookFunc struct {
	id string
	fn func(ctx context.Context, through ThroughType) FeedbackStatus
}

func NewHookFunc(id string, fn func(ctx context.Context, through ThroughType) FeedbackStatus) *HookFunc {
	return &HookFunc{id: id, fn: fn}
}

func (h *HookFunc) ID() string                                               { return h.id }
func (h *HookFunc) Handle(ctx context.Context, t ThroughType) FeedbackStatus { return h.fn(ctx, t) }

// HookRouter manages hook registration and dispatch.
type HookRouter struct {
	hooks []Hook
}

// NewHookRouter creates a new HookRouter.
func NewHookRouter(hooks ...Hook) *HookRouter {
	return &HookRouter{hooks: hooks}
}

// Register adds a hook to the router.
func (r *HookRouter) Register(hook Hook) {
	r.hooks = append(r.hooks, hook)
}

// DoHook fires all registered hooks at the given through-point.
// Returns FeedbackAbandon if any hook aborts, FeedbackBreakdown if any breaks.
// Returns FeedbackContinue if all hooks return continue.
func (r *HookRouter) DoHook(ctx context.Context, through ThroughType) FeedbackStatus {
	for _, h := range r.hooks {
		fb := h.Handle(ctx, through)
		switch fb {
		case FeedbackAbandon:
			return fb
		case FeedbackBreakdown:
			return fb
		}
	}
	return FeedbackContinue
}
