package nova

import "context"

// Store is the storage interface for the Nova engine.
// Implementations: SQLite (embedded), PostgreSQL (production).
// All methods must honor context cancellation.
type Store interface {
	// ── Process Definitions ──────────────────────────────────────────────

	GetPro(ctx context.Context, id string) (*Pro, error)
	GetProByAlias(ctx context.Context, alias string) (*Pro, error)
	GetProVer(ctx context.Context, proID string, ver int) (*ProVer, error)
	GetActsByProVer(ctx context.Context, proVerID string) ([]*Act, error)
	GetLinksByProVer(ctx context.Context, proVerID string) ([]*Link, error)
	GetManRule(ctx context.Context, actID string) (*ManRule, error)
	GetFirstAct(ctx context.Context, proID string, proVer int) (*Act, error)

	// ── Process Design / Management ──────────────────────────────────────

	ListPros(ctx context.Context) ([]*Pro, error)
	DeletePro(ctx context.Context, id string) error
	DeleteActsByProVer(ctx context.Context, proVerID string) error
	DeleteLinksByProVer(ctx context.Context, proVerID string) error
	DeleteManRulesByAct(ctx context.Context, actID string) error
	UpdatePro(ctx context.Context, p *Pro) error
	ListProVers(ctx context.Context, proID string) ([]*ProVer, error)

	// ── Entity ──────────────────────────────────────────────────────────

	CreateEntity(ctx context.Context, e *Entity) error
	GetEntity(ctx context.Context, id string) (*Entity, error)
	GetEntityByCode(ctx context.Context, code string) (*Entity, error)
	UpdateEntityState(ctx context.Context, id string, state EntityState) error
	UpdateEntityTitle(ctx context.Context, id string, title string) error
	UpdateEntitySerial(ctx context.Context, id string, serial string) error

	// ── Task ────────────────────────────────────────────────────────────

	CreateTask(ctx context.Context, t *Task) error
	GetTask(ctx context.Context, id string) (*Task, error)
	GetTasksByEntity(ctx context.Context, entityID string) ([]*Task, error)
	GetActTasks(ctx context.Context, entityID string, actID string) ([]*Task, error)
	UpdateTaskState(ctx context.Context, id string, state TaskState) error
	UpdateTaskConverge(ctx context.Context, id string, converge int64) error
	UpdateTaskWaiting(ctx context.Context, id string, waiting string) error
	MoveTaskToHistory(ctx context.Context, t *Task) error // called when task completes

	// ── Todo ────────────────────────────────────────────────────────────

	CreateTodo(ctx context.Context, t *Todo) error
	GetEntityTodos(ctx context.Context, entityID string) ([]*Todo, error)
	GetUserTodos(ctx context.Context, handlerUID string) ([]*Todo, error)
	AcceptTodo(ctx context.Context, id string) error
	DeleteTodo(ctx context.Context, id string) error

	// ── Step / Log ──────────────────────────────────────────────────────

	CreateStep(ctx context.Context, s *Step) error
	CreateStepLog(ctx context.Context, l *StepLog) error
	CreateHandleLog(ctx context.Context, l *HandleLog) error
	UpdateHandleLogFinish(ctx context.Context, taskID string, at int64) error

	// ── Opinion ─────────────────────────────────────────────────────────

	CreateOpinion(ctx context.Context, o *Opinion) error
	GetEntityOpinions(ctx context.Context, entityID string) ([]*Opinion, error)

	// ── Children (sub-process) ──────────────────────────────────────────

	GetChildEntities(ctx context.Context, parentID string) ([]*Entity, error)

	// ── Sn / Counter ────────────────────────────────────────────────────

	NextSerial(ctx context.Context, proAlias string) (string, error)

	// ── User ───────────────────────────────────────────────────────────

	ListUsers(ctx context.Context) ([]*User, error)
	GetUser(ctx context.Context, id string) (*User, error)
	GetUserByUID(ctx context.Context, uid string) (*User, error)
	CreateUser(ctx context.Context, u *User) error
	UpdateUser(ctx context.Context, u *User) error
	DeleteUser(ctx context.Context, id string) error
	SetPassword(ctx context.Context, userID string, hash string) error

	// ── Notification ────────────────────────────────────────────────────

	CreateNotification(ctx context.Context, n *Notification) error
	GetUserNotifications(ctx context.Context, uid string, limit, offset int) ([]*Notification, int, error)
	GetUnreadNotificationCount(ctx context.Context, uid string) (int, error)
	MarkNotificationRead(ctx context.Context, id string) error
	MarkAllNotificationsRead(ctx context.Context, uid string) error

	// ── Close ───────────────────────────────────────────────────────────

	Close() error
}

// TxStore extends Store with transactional support.
type TxStore interface {
	Store
	BeginTx(ctx context.Context) (TxStore, error)
	Commit() error
	Rollback() error
}
