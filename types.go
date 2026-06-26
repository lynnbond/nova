// Package nova is a pure-Go workflow engine for OA/enterprise approval processes.
// It is a rebirth of the Nova4 engine (originally by LiYan @ Sogou/Pandora),
// preserving the proven state-machine + concurrency model while shedding all legacy baggage.
package nova

import (
	"context"
	"time"
)

// ─── Enums ────────────────────────────────────────────────────────────────────

// ActType represents the type of a workflow activity.
type ActType int8

const (
	ActTypeSTART    ActType = 0   // Start activity
	ActTypeMANUAL   ActType = 1   // Human-interaction activity
	ActTypeROUTER   ActType = 3   // Concurrent routing (fan-out)
	ActTypeCONVERGE ActType = 4   // Synchronous convergence (fan-in)
	ActTypeWAIT     ActType = 5   // Async wait for other activities
	ActTypeTAIL     ActType = 6   // Branch termination
	ActTypeEND      ActType = 100 // Process termination
)

func (a ActType) String() string {
	switch a {
	case ActTypeSTART:
		return "START"
	case ActTypeMANUAL:
		return "MANUAL"
	case ActTypeROUTER:
		return "ROUTER"
	case ActTypeCONVERGE:
		return "CONVERGE"
	case ActTypeWAIT:
		return "WAIT"
	case ActTypeTAIL:
		return "TAIL"
	case ActTypeEND:
		return "END"
	default:
		return "UNKNOWN"
	}
}

// TaskState represents the processing state of a task.
type TaskState int8

const (
	TaskStateINITIAL    TaskState = 0   // Initial
	TaskStateCONVERGING TaskState = 1   // Waiting for convergence
	TaskStateWAITING    TaskState = 2   // Async waiting
	TaskStateREADY      TaskState = 3   // Ready to create todo
	TaskStateTODO       TaskState = 4   // Has pending todo
	TaskStatePROCESSING TaskState = 5   // Being handled
	TaskStateOVER       TaskState = 100 // Completed
)

func (t TaskState) String() string {
	switch t {
	case TaskStateINITIAL:
		return "INITIAL"
	case TaskStateCONVERGING:
		return "CONVERGING"
	case TaskStateWAITING:
		return "WAITING"
	case TaskStateREADY:
		return "READY"
	case TaskStateTODO:
		return "TODO"
	case TaskStatePROCESSING:
		return "PROCESSING"
	case TaskStateOVER:
		return "OVER"
	default:
		return "UNKNOWN"
	}
}

// EntityState represents the lifecycle state of an entity.
type EntityState int8

const (
	EntityStateDRAFT  EntityState = 0   // Unsaved draft
	EntityStateSUBMIT EntityState = 1   // Submitted (in process)
	EntityStateBACK   EntityState = 50  // Returned
	EntityStateCANCEL EntityState = 90  // Cancelled
	EntityStateOVER   EntityState = 100 // Completed
)

// ManPolicy defines how handlers are assigned for a manual activity.
type ManPolicy int8

const (
	ManPolicySINGLE     ManPolicy = 10  // Single person
	ManPolicyEXCLUSIVE  ManPolicy = 21  // Multiple, exclusive
	ManPolicyCONCURRENT ManPolicy = 22  // Multiple, concurrent
)

// LinkType defines the direction of an activity link.
type LinkType int8

const (
	LinkTypeFORWARD  LinkType = 1 // Forward
	LinkTypeBACKWARD LinkType = 2 // Backward
)

// HandlerBase defines the base for handler resolution.
type HandlerBase int8

const (
	HandlerBaseChannel HandlerBase = 0
	HandlerBaseDept    HandlerBase = 1
	HandlerBaseTeam    HandlerBase = 2
	HandlerBaseRole    HandlerBase = 3
	HandlerBaseTask    HandlerBase = 4
)

// ─── User ──────────────────────────────────────────────────────────────────────

// User represents a platform user (handler).
type User struct {
	Seq          int64     `json:"seq"`          // sequential number (for ordering)
	ID           string    `json:"id"`           // UUID, portable primary key
	UID          string    `json:"uid"`          // login name, unique (e.g. "zhangsan")
	Name         string    `json:"name"`         // display name
	PasswordHash string    `json:"-"`            // bcrypt hash, never exposed
	Email        string    `json:"email"`
	Phone        string    `json:"phone"`
	Dept         string    `json:"dept"`         // department
	State        int       `json:"state"`        // 0=active, 1=disabled
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// ─── Runtime Objects ──────────────────────────────────────────────────────────

// Pro is a runtime process definition.
type Pro struct {
	Seq     int64  // sequential number (for ordering)
	ID      string // UUID, portable primary key
	Alias   string // 4-char uppercase alias
	Name    string
	Ver     int    // current online version
	Channel string // channel class name (business hook ID)
}

// Act is a runtime activity node within a process.
type Act struct {
	Seq          int64  // sequential number (for ordering)
	ID           string // UUID, portable primary key
	ProID        string
	Name         string   // unique within process
	Title        string
	Type         ActType
	Ver          int
	Editable     bool
	ForceOpinion bool
	WaitActs     []string // activity names to wait for (WAIT type)

	// Runtime-only (not persisted in act table):
	WaitingList []*Act // injected during routing for propagating wait requirements
}

// Link represents a directed edge between activities.
type Link struct {
	Seq       int64  // sequential number (for ordering)
	ID        int64  // SQLite rowid, not portable
	ProVerID  string
	ActID     string  // stored in DB
	Act       *Act    // populated at runtime
	PrevActID string
	Title     string
	Type      LinkType

	// DecisionFilter restricts this link to submissions with a matching decision label.
	// Empty string means this link is the default (used when no decision is specified).
	DecisionFilter string
}

// ManRule defines handler configuration for a MANUAL activity.
type ManRule struct {
	ActID      string
	BaseOn     HandlerBase
	Policy     ManPolicy
	SelAllowed bool     // whether handler can self-select
	GroupSet   []string // department/team/role IDs
	TaskAct    string   // task-based act name
}

// ProVer represents a versioned process snapshot.
type ProVer struct {
	Seq       int64  // sequential number (for ordering)
	ID        string // UUID, portable primary key
	ProID     string
	Ver       int
	IsRelease bool
}

// ─── Entity / Task / Todo ────────────────────────────────────────────────────

// Entity is the core business document flowing through the process.
type Entity struct {
	Seq       int64  // sequential number (for ordering)
	ID        string // UUID, portable primary key
	Code      string // unique identifier
	ProID     string
	ProVer    int
	Title     string
	State     EntityState

	DraftUID  string
	DraftName string
	DraftDept string

	ParentID  string // "" = no parent
	SerialNum string

	SendAt  *time.Time
	OverAt  *time.Time

	CreatedAt time.Time
	UpdatedAt time.Time
}

// Task represents a single activity instance within an entity's process execution.
type Task struct {
	Seq      int64  // sequential number (for ordering)
	ID       string // UUID, portable primary key
	EntityID string
	StepID   int64
	ActID    string
	ActName  string
	ActTitle string
	State    TaskState

	Handlers string // display-only, comma-separated handler names

	// Concurrency control:
	Converge int64 // act seq to converge from (for CONVERGE state)
	Waiting  string // comma-separated act names to wait for (for WAITING state)

	StartAt *time.Time
	EndAt   *time.Time

	CreatedAt time.Time
	UpdatedAt time.Time
}

// Todo is a handler's task assignment.
type Todo struct {
	Seq     int64  `json:"seq"`     // sequential number
	ID      string `json:"id"`       // UUID
	TaskID  string `json:"task_id"`  // UUID
	EntityID string `json:"entity_id"` // UUID
	ActID   string `json:"act_id"`   // UUID
	ProID   string `json:"pro_id"`   // UUID

	HandlerUID   string `json:"handler_uid"`
	HandlerName  string `json:"handler_name"`
	SenderUID    string `json:"sender_uid"`
	SenderName   string `json:"sender_name"`

	ActTitle     string     `json:"act_title"`
	EntityTitle  string     `json:"entity_title"`
	ProName      string     `json:"pro_name"`

	ArriveAt     *time.Time `json:"arrive_at"`
	AcceptAt     *time.Time `json:"accept_at"`
	Accepted     bool       `json:"accepted"`
	TodoKey      string     `json:"todo_key"`
}

// Step records the entity's movement between activities.
type Step struct {
	ID        int64
	EntityID  string
	ActID     string
	ActName   string
	CreatedAt time.Time
}

// StepLog records each transition in detail.
type StepLog struct {
	ID           int64
	EntityID     string
	StepID       int64
	CurActID     string
	DestActID    string
	DestActName  string
	DestActTitle string
	DestActType  ActType
	LinkType     LinkType
	LoggedAt     time.Time
}

// HandleLog records who did what and when.
type HandleLog struct {
	ID          int64
	EntityID    string
	StepID      int64
	TaskID      string
	ActID       string
	ActTitle    string
	HandlerUID  string
	HandlerName string
	Content     string
	ArriveAt    *time.Time
	AcceptAt    *time.Time
	FinishAt    *time.Time
	ClientType  int
	InsertIP    string
}

// Opinion is a comment posted on an entity during processing.
type Opinion struct {
	ID          int64
	ParentID    int64 // threaded comments
	EntityID    string
	ActID       string
	TaskID      string
	HandlerUID  string
	HandlerName string
	Content     string
	ActTitle    string
	WrittenAt   time.Time
}

// ─── Workflow-level State Machine ─────────────────────────────────────────────

// Op represents an operation that triggers a state transition.
type Op int8

const (
	OpSave   Op = 1
	OpSubmit Op = 2
	OpCancel Op = 3
)

// SessionState represents the current state in the entity lifecycle state machine.
type SessionState int8

const (
	SessionStateStart   SessionState = 0
	SessionStateHandle  SessionState = 1
	SessionStateView    SessionState = 2
	SessionStateDeleted SessionState = 3
)

// transitionTable defines the state machine.
// [current][op] -> next
var transitionTable = map[SessionState]map[Op]SessionState{
	SessionStateStart: {
		OpSave:   SessionStateHandle,
		OpCancel: SessionStateDeleted,
	},
	SessionStateHandle: {
		OpSubmit: SessionStateView,
		OpCancel: SessionStateDeleted,
	},
}

func ValidTransition(from SessionState, op Op) (SessionState, bool) {
	ops, ok := transitionTable[from]
	if !ok {
		return 0, false
	}
	next, ok := ops[op]
	return next, ok
}

// ─── Context key types ────────────────────────────────────────────────────────

type contextKey string

const (
	ctxKeyHandler = contextKey("nova:handler")
	ctxKeyClient  = contextKey("nova:client")
)

// WithHandler embeds the current handler into context.
func WithHandler(ctx context.Context, uid, name string) context.Context {
	return context.WithValue(ctx, ctxKeyHandler, [2]string{uid, name})
}

// CurHandler extracts the current handler from context.
func CurHandler(ctx context.Context) (uid, name string, ok bool) {
	if ctx == nil {
		return "", "", false
	}
	v := ctx.Value(ctxKeyHandler)
	if v == nil {
		return "", "", false
	}
	pair := v.([2]string)
	return pair[0], pair[1], true
}

// ─── Feedback / Status ────────────────────────────────────────────────────────

// FeedbackStatus is returned by hooks to influence the submission flow.
type FeedbackStatus int8

const (
	FeedbackContinue   FeedbackStatus = 0 // Continue normally
	FeedbackAbandon    FeedbackStatus = 1 // Abandon this submission
	FeedbackBreakdown  FeedbackStatus = 2 // Break the submission with error
)

// ─── SubmitInfo ───────────────────────────────────────────────────────────────

// SubmitInfo carries the result of a submit operation.
type SubmitInfo struct {
	Direct       *Link
	DestTargets  []TargetInfo
	NewTodos     []*Todo
	DoneTask     *Task
	Message      string
}

// TargetInfo describes a destination activity and its assigned handlers.
type TargetInfo struct {
	Link     *Link
	Rule     *ManRule
	Handlers []HandlerRef
}

// HandlerRef identifies a handler.
type HandlerRef struct {
	UID   string
	Name  string
	Dept  string
}

// ─── Notification ─────────────────────────────────────────────────────────

// Notification represents a notification event for a user.
type Notification struct {
	ID        string     `json:"id"`
	EntityID  string     `json:"entity_id"`
	TaskID    string     `json:"task_id,omitempty"`
	TodoID    string     `json:"todo_id,omitempty"`
	NotifType string     `json:"notif_type"` // todo_created, entity_returned, entity_completed, entity_submitted
	Title     string     `json:"title"`
	Content   string     `json:"content"`
	TargetUID string     `json:"target_uid"`
	Read      bool       `json:"read"`
	CreatedAt time.Time  `json:"created_at"`
	ReadAt    *time.Time `json:"read_at,omitempty"`
}