package sqlite

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	_ "github.com/mattn/go-sqlite3"

	"github.com/liyan/nova"
)

// Store implements nova.Store backed by SQLite.
type Store struct {
	mu sync.Mutex
	db *sql.DB
}

// Config for SQLite store.
type Config struct {
	DSN string // e.g. "file:nova.db?cache=shared&_journal_mode=WAL"
}

// Open opens a new SQLite store, running migrations automatically.
func Open(cfg Config) (*Store, error) {
	if cfg.DSN == "" {
		cfg.DSN = "file:nova.db?cache=shared&_journal_mode=WAL&_busy_timeout=5000"
	}
	db, err := sql.Open("sqlite3", cfg.DSN)
	if err != nil {
		return nil, fmt.Errorf("sqlite open: %w", err)
	}
	db.SetMaxOpenConns(1) // SQLite concurrency limit
	db.SetConnMaxLifetime(0)

	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, fmt.Errorf("sqlite migrate: %w", err)
	}
	return s, nil
}

func (s *Store) Close() error { return s.db.Close() }

// Exec executes a raw SQL statement. Used for seeding/admin operations.
func (s *Store) Exec(query string, args ...any) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.db.Exec(query, args...)
	return err
}

// ─── UUID helper ───────────────────────────────────────────────────────────────

// newUUID generates a UUID v4-like hex string (no dashes, 32 hex chars).
func newUUID() string {
	var buf [16]byte
	if _, err := rand.Read(buf[:]); err != nil {
		// Fallback: timestamp-based
		return fmt.Sprintf("uuid-%d", time.Now().UnixNano())
	}
	// Set version 4 bits
	buf[6] = (buf[6] & 0x0f) | 0x40
	// Set variant bits
	buf[8] = (buf[8] & 0x3f) | 0x80
	return hex.EncodeToString(buf[:])
}

// ─── Migrations ───────────────────────────────────────────────────────────────

const schema = `
PRAGMA foreign_keys = ON;
PRAGMA journal_mode = WAL;

CREATE TABLE IF NOT EXISTS process (
    id          TEXT PRIMARY KEY,
    seq         INTEGER NOT NULL DEFAULT 0,
    alias       TEXT NOT NULL UNIQUE,
    name        TEXT NOT NULL DEFAULT '',
    ver         INTEGER NOT NULL DEFAULT 1,
    channel     TEXT NOT NULL DEFAULT '',
    created_at  TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS pro_ver (
    id          TEXT PRIMARY KEY,
    seq         INTEGER NOT NULL DEFAULT 0,
    pro_id      TEXT NOT NULL REFERENCES process(id),
    ver         INTEGER NOT NULL DEFAULT 1,
    is_release  INTEGER NOT NULL DEFAULT 0,
    UNIQUE(pro_id, ver)
);

CREATE TABLE IF NOT EXISTS act (
    id              TEXT PRIMARY KEY,
    seq             INTEGER NOT NULL DEFAULT 0,
    pro_id          TEXT NOT NULL REFERENCES process(id),
    name            TEXT NOT NULL,
    title           TEXT NOT NULL DEFAULT '',
    act_type        INTEGER NOT NULL DEFAULT 1,
    act_ver         INTEGER NOT NULL DEFAULT 1,
    editable        INTEGER NOT NULL DEFAULT 1,
    force_opinion   INTEGER NOT NULL DEFAULT 0,
    wait_acts       TEXT NOT NULL DEFAULT '',
    created_at      TEXT NOT NULL DEFAULT (datetime('now')),
    UNIQUE(pro_id, name, act_ver)
);

CREATE TABLE IF NOT EXISTS act_link (
    id          TEXT PRIMARY KEY,
    seq         INTEGER NOT NULL DEFAULT 0,
    pro_ver_id  TEXT NOT NULL REFERENCES pro_ver(id),
    act_id      TEXT NOT NULL REFERENCES act(id),
    prev_act_id TEXT NOT NULL,
    title       TEXT NOT NULL DEFAULT '',
    link_type   INTEGER NOT NULL DEFAULT 1
);

CREATE TABLE IF NOT EXISTS man_rule (
    act_id      TEXT PRIMARY KEY REFERENCES act(id),
    base_on     INTEGER NOT NULL DEFAULT 0,
    policy      INTEGER NOT NULL DEFAULT 10,
    sel_allowed INTEGER NOT NULL DEFAULT 0,
    group_set   TEXT NOT NULL DEFAULT '',
    task_act    TEXT NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS entity (
    id          TEXT PRIMARY KEY,
    seq         INTEGER NOT NULL DEFAULT 0,
    code        TEXT NOT NULL UNIQUE,
    pro_id      TEXT NOT NULL,
    pro_ver     INTEGER NOT NULL DEFAULT 1,
    title       TEXT NOT NULL DEFAULT '',
    state       INTEGER NOT NULL DEFAULT 0,
    draft_uid   TEXT NOT NULL DEFAULT '',
    draft_name  TEXT NOT NULL DEFAULT '',
    draft_dept  TEXT NOT NULL DEFAULT '',
    parent_id   TEXT NOT NULL DEFAULT '',
    serial_num  TEXT NOT NULL DEFAULT '',
    send_at     TEXT,
    over_at     TEXT,
    created_at  TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at  TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS run_task (
    id          TEXT PRIMARY KEY,
    seq         INTEGER NOT NULL DEFAULT 0,
    entity_id   TEXT NOT NULL REFERENCES entity(id),
    step_id     INTEGER NOT NULL DEFAULT 0,
    act_id      TEXT NOT NULL,
    act_name    TEXT NOT NULL DEFAULT '',
    act_title   TEXT NOT NULL DEFAULT '',
    task_state  INTEGER NOT NULL DEFAULT 0,
    handlers    TEXT NOT NULL DEFAULT '',
    converge    INTEGER NOT NULL DEFAULT 0,
    waiting     TEXT NOT NULL DEFAULT '',
    start_at    TEXT,
    end_at      TEXT,
    created_at  TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at  TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS run_todo (
    id            TEXT PRIMARY KEY,
    seq           INTEGER NOT NULL DEFAULT 0,
    task_id       TEXT NOT NULL REFERENCES run_task(id),
    entity_id     TEXT NOT NULL DEFAULT '',
    act_id        TEXT NOT NULL DEFAULT '',
    pro_id        TEXT NOT NULL DEFAULT '',
    handler_uid   TEXT NOT NULL,
    handler_name  TEXT NOT NULL DEFAULT '',
    sender_uid    TEXT NOT NULL DEFAULT '',
    sender_name   TEXT NOT NULL DEFAULT '',
    act_title     TEXT NOT NULL DEFAULT '',
    entity_title  TEXT NOT NULL DEFAULT '',
    pro_name      TEXT NOT NULL DEFAULT '',
    arrive_at     TEXT,
    accept_at     TEXT,
    accepted      INTEGER NOT NULL DEFAULT 0,
    todo_key      TEXT NOT NULL DEFAULT '',
    created_at    TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS run_step (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    entity_id   TEXT NOT NULL REFERENCES entity(id),
    act_id      TEXT NOT NULL DEFAULT '',
    act_name    TEXT NOT NULL DEFAULT '',
    created_at  TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS step_log (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    entity_id       TEXT NOT NULL,
    step_id         INTEGER NOT NULL,
    cur_act_id      TEXT NOT NULL,
    dest_act_id     TEXT NOT NULL,
    dest_act_name   TEXT NOT NULL DEFAULT '',
    dest_act_title  TEXT NOT NULL DEFAULT '',
    dest_act_type   INTEGER NOT NULL DEFAULT 0,
    link_type       INTEGER NOT NULL DEFAULT 1,
    logged_at       TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS handle_log (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    entity_id     TEXT NOT NULL,
    step_id       INTEGER NOT NULL DEFAULT 0,
    task_id       TEXT NOT NULL DEFAULT '',
    act_id        TEXT NOT NULL DEFAULT '',
    act_title     TEXT NOT NULL DEFAULT '',
    handler_uid   TEXT NOT NULL DEFAULT '',
    handler_name  TEXT NOT NULL DEFAULT '',
    log           TEXT NOT NULL DEFAULT '',
    arrive_at     TEXT,
    accept_at     TEXT,
    finish_at     TEXT,
    client_type   INTEGER NOT NULL DEFAULT 0,
    insert_ip     TEXT NOT NULL DEFAULT '',
    created_at    TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS opinion (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    parent_id     INTEGER NOT NULL DEFAULT 0,
    entity_id     TEXT NOT NULL,
    act_id        TEXT NOT NULL DEFAULT '',
    task_id       TEXT NOT NULL DEFAULT '',
    handler_uid   TEXT NOT NULL DEFAULT '',
    handler_name  TEXT NOT NULL DEFAULT '',
    content       TEXT NOT NULL DEFAULT '',
    act_title     TEXT NOT NULL DEFAULT '',
    written_at    TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_task_entity ON run_task(entity_id);
CREATE INDEX IF NOT EXISTS idx_task_act ON run_task(act_id);
CREATE INDEX IF NOT EXISTS idx_todo_handler ON run_todo(handler_uid);
CREATE INDEX IF NOT EXISTS idx_todo_task ON run_todo(task_id);
CREATE INDEX IF NOT EXISTS idx_handle_log_entity ON handle_log(entity_id);
CREATE INDEX IF NOT EXISTS idx_opinion_entity ON opinion(entity_id);
CREATE INDEX IF NOT EXISTS idx_link_pro_ver ON act_link(pro_ver_id);
CREATE INDEX IF NOT EXISTS idx_act_pro ON act(pro_id);

CREATE TABLE IF NOT EXISTS users (
    id              TEXT PRIMARY KEY,
    seq             INTEGER NOT NULL DEFAULT 0,
    uid             TEXT NOT NULL UNIQUE,
    name            TEXT NOT NULL DEFAULT '',
    password_hash   TEXT NOT NULL DEFAULT '',
    email           TEXT NOT NULL DEFAULT '',
    phone           TEXT NOT NULL DEFAULT '',
    dept            TEXT NOT NULL DEFAULT '',
    state           INTEGER NOT NULL DEFAULT 0,
    created_at      TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at      TEXT NOT NULL DEFAULT (datetime('now'))
);
`

func (s *Store) migrate() error {
	if _, err := s.db.Exec(schema); err != nil {
		return err
	}
	return nil
}

// ─── Process Definitions ──────────────────────────────────────────────────────

func (s *Store) GetPro(ctx context.Context, id string) (*nova.Pro, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, alias, name, ver, channel FROM process WHERE id = ?`, id)
	p := &nova.Pro{}
	err := row.Scan(&p.ID, &p.Alias, &p.Name, &p.Ver, &p.Channel)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return p, err
}

func (s *Store) GetProByAlias(ctx context.Context, alias string) (*nova.Pro, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, alias, name, ver, channel FROM process WHERE alias = ?`, alias)
	p := &nova.Pro{}
	err := row.Scan(&p.ID, &p.Alias, &p.Name, &p.Ver, &p.Channel)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return p, err
}

func (s *Store) GetProVer(ctx context.Context, proID string, ver int) (*nova.ProVer, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, pro_id, ver, is_release FROM pro_ver WHERE pro_id = ? AND ver = ?`, proID, ver)
	pv := &nova.ProVer{}
	err := row.Scan(&pv.ID, &pv.ProID, &pv.Ver, &pv.IsRelease)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return pv, err
}

func (s *Store) GetActsByProVer(ctx context.Context, proVerID string) ([]*nova.Act, error) {
	// Get pro_id from pro_ver first, then query acts
	var proID string
	err := s.db.QueryRowContext(ctx, `SELECT pro_id FROM pro_ver WHERE id = ?`, proVerID).Scan(&proID)
	if err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, pro_id, name, title, act_type, editable, force_opinion, wait_acts
		 FROM act WHERE pro_id = ?`, proID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var acts []*nova.Act
	for rows.Next() {
		a := &nova.Act{}
		var waitActs string
		if err := rows.Scan(&a.ID, &a.ProID, &a.Name, &a.Title, &a.Type, &a.Editable, &a.ForceOpinion, &waitActs); err != nil {
			return nil, err
		}
		if waitActs != "" {
			a.WaitActs = split(waitActs)
		}
		acts = append(acts, a)
	}
	return acts, rows.Err()
}

func (s *Store) GetLinksByProVer(ctx context.Context, proVerID string) ([]*nova.Link, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, pro_ver_id, act_id, prev_act_id, title, link_type
		 FROM act_link WHERE pro_ver_id = ?`, proVerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var links []*nova.Link
	for rows.Next() {
		l := &nova.Link{}
		if err := rows.Scan(&l.ID, &l.ProVerID, &l.ActID, &l.PrevActID, &l.Title, &l.Type); err != nil {
			return nil, err
		}
		// Act will be populated by caller
		links = append(links, l)
	}
	return links, rows.Err()
}

func (s *Store) GetManRule(ctx context.Context, actID string) (*nova.ManRule, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT act_id, base_on, policy, sel_allowed, group_set, task_act FROM man_rule WHERE act_id = ?`, actID)
	r := &nova.ManRule{}
	var grp, ta string
	err := row.Scan(&r.ActID, &r.BaseOn, &r.Policy, &r.SelAllowed, &grp, &ta)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if grp != "" {
		r.GroupSet = split(grp)
	}
	r.TaskAct = ta
	return r, nil
}

func (s *Store) GetFirstAct(ctx context.Context, proID string, proVer int) (*nova.Act, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, pro_id, name, title, act_type, editable, force_opinion, wait_acts
		 FROM act WHERE pro_id = ? AND act_type = 0 ORDER BY seq LIMIT 1`, proID)
	a := &nova.Act{}
	var waitActs string
	err := row.Scan(&a.ID, &a.ProID, &a.Name, &a.Title, &a.Type, &a.Editable, &a.ForceOpinion, &waitActs)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if waitActs != "" {
		a.WaitActs = split(waitActs)
	}
	return a, nil
}

// ─── Entity ───────────────────────────────────────────────────────────────────

func (s *Store) CreateEntity(ctx context.Context, e *nova.Entity) error {
	now := time.Now().UTC().Format(time.RFC3339)
	e.ID = newUUID()
	err := s.db.QueryRowContext(ctx,
		`SELECT COALESCE((SELECT MAX(seq) FROM entity), 0) + 1`).Scan(&e.Seq)
	if err != nil {
		return fmt.Errorf("generate seq: %w", err)
	}
	_, err = s.db.ExecContext(ctx,
		`INSERT INTO entity (id, seq, code, pro_id, pro_ver, title, state, draft_uid, draft_name, draft_dept, parent_id, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		e.ID, e.Seq, e.Code, e.ProID, e.ProVer, e.Title, e.State, e.DraftUID, e.DraftName, e.DraftDept, e.ParentID, now, now)
	if err != nil {
		return err
	}
	e.CreatedAt, _ = time.Parse(time.RFC3339, now)
	e.UpdatedAt = e.CreatedAt
	return nil
}

func (s *Store) GetEntity(ctx context.Context, id string) (*nova.Entity, error) {
	return s.scanEntity(s.db.QueryRowContext(ctx,
		`SELECT id, code, pro_id, pro_ver, title, state, draft_uid, draft_name, draft_dept,
		        parent_id, serial_num, send_at, over_at, created_at, updated_at
		 FROM entity WHERE id = ?`, id))
}

func (s *Store) GetEntityByCode(ctx context.Context, code string) (*nova.Entity, error) {
	return s.scanEntity(s.db.QueryRowContext(ctx,
		`SELECT id, code, pro_id, pro_ver, title, state, draft_uid, draft_name, draft_dept,
		        parent_id, serial_num, send_at, over_at, created_at, updated_at
		 FROM entity WHERE code = ?`, code))
}

func (s *Store) scanEntity(row interface{ Scan(dest ...any) error }) (*nova.Entity, error) {
	e := &nova.Entity{}
	var sendAt, overAt, createdAt, updatedAt sql.NullString
	err := row.Scan(&e.ID, &e.Code, &e.ProID, &e.ProVer, &e.Title, &e.State,
		&e.DraftUID, &e.DraftName, &e.DraftDept,
		&e.ParentID, &e.SerialNum, &sendAt, &overAt, &createdAt, &updatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	e.SendAt = parseTime(sendAt)
	e.OverAt = parseTime(overAt)
	e.CreatedAt, _ = time.Parse(time.RFC3339, createdAt.String)
	e.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt.String)
	return e, nil
}

func (s *Store) UpdateEntityState(ctx context.Context, id string, state nova.EntityState) error {
	now := time.Now().UTC().Format(time.RFC3339)
	var err error
	if state == nova.EntityStateOVER {
		_, err = s.db.ExecContext(ctx, `UPDATE entity SET state = ?, over_at = ?, updated_at = ? WHERE id = ?`, state, now, now, id)
	} else {
		_, err = s.db.ExecContext(ctx, `UPDATE entity SET state = ?, updated_at = ? WHERE id = ?`, state, now, id)
	}
	return err
}

func (s *Store) UpdateEntityTitle(ctx context.Context, id string, title string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := s.db.ExecContext(ctx, `UPDATE entity SET title = ?, updated_at = ? WHERE id = ?`, title, now, id)
	return err
}

func (s *Store) UpdateEntitySerial(ctx context.Context, id string, serial string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := s.db.ExecContext(ctx, `UPDATE entity SET serial_num = ?, updated_at = ? WHERE id = ?`, serial, now, id)
	return err
}

// ─── Task ─────────────────────────────────────────────────────────────────────

func (s *Store) CreateTask(ctx context.Context, t *nova.Task) error {
	now := time.Now().UTC().Format(time.RFC3339)
	t.ID = newUUID()
	err := s.db.QueryRowContext(ctx,
		`SELECT COALESCE((SELECT MAX(seq) FROM run_task), 0) + 1`).Scan(&t.Seq)
	if err != nil {
		return fmt.Errorf("generate seq: %w", err)
	}
	_, err = s.db.ExecContext(ctx,
		`INSERT INTO run_task (id, seq, entity_id, step_id, act_id, act_name, act_title, task_state, handlers, converge, waiting, start_at, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		t.ID, t.Seq, t.EntityID, t.StepID, t.ActID, t.ActName, t.ActTitle, t.State, t.Handlers, t.Converge, t.Waiting, formatTime(t.StartAt), now, now)
	if err != nil {
		return err
	}
	t.CreatedAt, _ = time.Parse(time.RFC3339, now)
	t.UpdatedAt = t.CreatedAt
	return nil
}

func (s *Store) GetTask(ctx context.Context, id string) (*nova.Task, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, entity_id, step_id, act_id, act_name, act_title, task_state, handlers,
		        converge, waiting, start_at, end_at, created_at, updated_at
		 FROM run_task WHERE id = ?`, id)
	return s.scanTask(row)
}

func (s *Store) GetTasksByEntity(ctx context.Context, entityID string) ([]*nova.Task, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, entity_id, step_id, act_id, act_name, act_title, task_state, handlers,
		        converge, waiting, start_at, end_at, created_at, updated_at
		 FROM run_task WHERE entity_id = ?`, entityID)
	if err != nil {
		return nil, err
	}
	return s.scanTasks(rows)
}

func (s *Store) GetActTasks(ctx context.Context, entityID string, actID string) ([]*nova.Task, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, entity_id, step_id, act_id, act_name, act_title, task_state, handlers,
		        converge, waiting, start_at, end_at, created_at, updated_at
		 FROM run_task WHERE entity_id = ? AND act_id = ?`, entityID, actID)
	if err != nil {
		return nil, err
	}
	return s.scanTasks(rows)
}

func (s *Store) scanTasks(rows *sql.Rows) ([]*nova.Task, error) {
	defer rows.Close()
	var tasks []*nova.Task
	for rows.Next() {
		t, err := s.scanTask(rows)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, t)
	}
	return tasks, rows.Err()
}

func (s *Store) scanTask(row interface{ Scan(dest ...any) error }) (*nova.Task, error) {
	t := &nova.Task{}
	var startAt, endAt, createdAt, updatedAt sql.NullString
	err := row.Scan(&t.ID, &t.EntityID, &t.StepID, &t.ActID, &t.ActName, &t.ActTitle, &t.State, &t.Handlers,
		&t.Converge, &t.Waiting, &startAt, &endAt, &createdAt, &updatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	t.StartAt = parseTime(startAt)
	t.EndAt = parseTime(endAt)
	t.CreatedAt, _ = time.Parse(time.RFC3339, createdAt.String)
	t.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt.String)
	return t, nil
}

func (s *Store) UpdateTaskState(ctx context.Context, id string, state nova.TaskState) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := s.db.ExecContext(ctx, `UPDATE run_task SET task_state = ?, updated_at = ? WHERE id = ?`, state, now, id)
	return err
}

func (s *Store) UpdateTaskConverge(ctx context.Context, id string, converge int64) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := s.db.ExecContext(ctx, `UPDATE run_task SET converge = ?, updated_at = ? WHERE id = ?`, converge, now, id)
	return err
}

func (s *Store) UpdateTaskWaiting(ctx context.Context, id string, waiting string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := s.db.ExecContext(ctx, `UPDATE run_task SET waiting = ?, updated_at = ? WHERE id = ?`, waiting, now, id)
	return err
}

func (s *Store) MoveTaskToHistory(ctx context.Context, t *nova.Task) error {
	// SQLite: just mark as OVER, soft-delete
	return s.UpdateTaskState(ctx, t.ID, nova.TaskStateOVER)
}

// ─── Todo ─────────────────────────────────────────────────────────────────────

func (s *Store) CreateTodo(ctx context.Context, t *nova.Todo) error {
	now := time.Now().UTC().Format(time.RFC3339)
	t.ID = newUUID()
	err := s.db.QueryRowContext(ctx,
		`SELECT COALESCE((SELECT MAX(seq) FROM run_todo), 0) + 1`).Scan(&t.Seq)
	if err != nil {
		return fmt.Errorf("generate seq: %w", err)
	}
	_, err = s.db.ExecContext(ctx,
		`INSERT INTO run_todo (id, seq, task_id, entity_id, act_id, pro_id, handler_uid, handler_name,
		                      sender_uid, sender_name, act_title, entity_title, pro_name,
		                      arrive_at, todo_key, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		t.ID, t.Seq, t.TaskID, t.EntityID, t.ActID, t.ProID, t.HandlerUID, t.HandlerName,
		t.SenderUID, t.SenderName, t.ActTitle, t.EntityTitle, t.ProName,
		formatTime(t.ArriveAt), t.TodoKey, now)
	if err != nil {
		return err
	}
	return nil
}

func (s *Store) GetEntityTodos(ctx context.Context, entityID string) ([]*nova.Todo, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, task_id, entity_id, act_id, pro_id, handler_uid, handler_name,
		        sender_uid, sender_name, act_title, entity_title, pro_name,
		        arrive_at, accept_at, accepted, todo_key, created_at
		 FROM run_todo WHERE entity_id = ?`, entityID)
	if err != nil {
		return nil, err
	}
	return s.scanTodos(rows)
}

func (s *Store) GetUserTodos(ctx context.Context, handlerUID string) ([]*nova.Todo, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, task_id, entity_id, act_id, pro_id, handler_uid, handler_name,
		        sender_uid, sender_name, act_title, entity_title, pro_name,
		        arrive_at, accept_at, accepted, todo_key, created_at
		 FROM run_todo WHERE handler_uid = ? ORDER BY created_at DESC`, handlerUID)
	if err != nil {
		return nil, err
	}
	return s.scanTodos(rows)
}

func (s *Store) scanTodos(rows *sql.Rows) ([]*nova.Todo, error) {
	defer rows.Close()
	var todos []*nova.Todo
	for rows.Next() {
		t := &nova.Todo{}
		var arriveAt, acceptAt, createdAt sql.NullString
		if err := rows.Scan(&t.ID, &t.TaskID, &t.EntityID, &t.ActID, &t.ProID,
			&t.HandlerUID, &t.HandlerName, &t.SenderUID, &t.SenderName,
			&t.ActTitle, &t.EntityTitle, &t.ProName,
			&arriveAt, &acceptAt, &t.Accepted, &t.TodoKey, &createdAt); err != nil {
			return nil, err
		}
		t.ArriveAt = parseTime(arriveAt)
		t.AcceptAt = parseTime(acceptAt)
		todos = append(todos, t)
	}
	return todos, rows.Err()
}

func (s *Store) AcceptTodo(ctx context.Context, id string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := s.db.ExecContext(ctx, `UPDATE run_todo SET accepted = 1, accept_at = ? WHERE id = ?`, now, id)
	return err
}

func (s *Store) DeleteTodo(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM run_todo WHERE id = ?`, id)
	return err
}

// ─── Step / Log ───────────────────────────────────────────────────────────────

func (s *Store) CreateStep(ctx context.Context, st *nova.Step) error {
	now := time.Now().UTC().Format(time.RFC3339)
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO run_step (entity_id, act_id, act_name, created_at) VALUES (?, ?, ?, ?)`,
		st.EntityID, st.ActID, st.ActName, now)
	if err != nil {
		return err
	}
	id, _ := res.LastInsertId()
	st.ID = id
	st.CreatedAt, _ = time.Parse(time.RFC3339, now)
	return nil
}

func (s *Store) CreateStepLog(ctx context.Context, l *nova.StepLog) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO step_log (entity_id, step_id, cur_act_id, dest_act_id, dest_act_name,
		                      dest_act_title, dest_act_type, link_type, logged_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, datetime('now'))`,
		l.EntityID, l.StepID, l.CurActID, l.DestActID, l.DestActName, l.DestActTitle, l.DestActType, l.LinkType)
	return err
}

func (s *Store) CreateHandleLog(ctx context.Context, l *nova.HandleLog) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO handle_log (entity_id, step_id, task_id, act_id, act_title, handler_uid,
		                        handler_name, log, arrive_at, insert_ip, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, datetime('now'))`,
		l.EntityID, l.StepID, l.TaskID, l.ActID, l.ActTitle, l.HandlerUID, l.HandlerName,
		l.Content, formatTime(l.ArriveAt), l.InsertIP)
	return err
}

func (s *Store) UpdateHandleLogFinish(ctx context.Context, taskID string, at int64) error {
	ts := time.Unix(at, 0).UTC().Format(time.RFC3339)
	_, err := s.db.ExecContext(ctx, `UPDATE handle_log SET finish_at = ? WHERE task_id = ?`, ts, taskID)
	return err
}

func (s *Store) UpdateHandleLogContent(ctx context.Context, taskID string, content string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE handle_log SET log = ? WHERE task_id = ?`, content, taskID)
	return err
}

// ─── Opinion ──────────────────────────────────────────────────────────────────

func (s *Store) CreateOpinion(ctx context.Context, o *nova.Opinion) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO opinion (parent_id, entity_id, act_id, task_id, handler_uid, handler_name,
		                      content, act_title, written_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, datetime('now'))`,
		o.ParentID, o.EntityID, o.ActID, o.TaskID, o.HandlerUID, o.HandlerName, o.Content, o.ActTitle)
	return err
}

func (s *Store) GetEntityOpinions(ctx context.Context, entityID string) ([]*nova.Opinion, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, parent_id, entity_id, act_id, task_id, handler_uid, handler_name,
		        content, act_title, written_at
		 FROM opinion WHERE entity_id = ? ORDER BY written_at`, entityID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ops []*nova.Opinion
	for rows.Next() {
		o := &nova.Opinion{}
		var writtenAt sql.NullString
		if err := rows.Scan(&o.ID, &o.ParentID, &o.EntityID, &o.ActID, &o.TaskID,
			&o.HandlerUID, &o.HandlerName, &o.Content, &o.ActTitle, &writtenAt); err != nil {
			return nil, err
		}
		if writtenAt.Valid {
			o.WrittenAt, _ = time.Parse(time.RFC3339, writtenAt.String)
		}
		ops = append(ops, o)
	}
	return ops, rows.Err()
}

// ─── Children ─────────────────────────────────────────────────────────────────

func (s *Store) GetChildEntities(ctx context.Context, parentID string) ([]*nova.Entity, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, code, pro_id, pro_ver, title, state, draft_uid, draft_name, draft_dept,
		        parent_id, serial_num, send_at, over_at, created_at, updated_at
		 FROM entity WHERE parent_id = ?`, parentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ents []*nova.Entity
	for rows.Next() {
		e, err := s.scanEntity(rows)
		if err != nil {
			return nil, err
		}
		ents = append(ents, e)
	}
	return ents, rows.Err()
}

// ─── Serial Number ────────────────────────────────────────────────────────────

func (s *Store) NextSerial(ctx context.Context, proAlias string) (string, error) {
	// Simple: alias + timestamp + random
	now := time.Now()
	return fmt.Sprintf("%s-%s-%04d", proAlias, now.Format("20060102"), now.UnixMilli()%10000), nil
}

// ─── ProcessBuilder Store Methods ─────────────────────────────────────────────

func (s *Store) CreateManRule(ctx context.Context, r *nova.ManRule) error {
	grp := joinJoin(r.GroupSet)
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO man_rule (act_id, base_on, policy, sel_allowed, group_set, task_act)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		r.ActID, r.BaseOn, r.Policy, boolToInt(r.SelAllowed), grp, r.TaskAct)
	return err
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func (s *Store) CreateProcess(ctx context.Context, p *nova.Pro) error {
	p.ID = newUUID()
	err := s.db.QueryRowContext(ctx,
		`SELECT COALESCE((SELECT MAX(seq) FROM process), 0) + 1`).Scan(&p.Seq)
	if err != nil {
		return fmt.Errorf("generate seq: %w", err)
	}
	_, err = s.db.ExecContext(ctx,
		`INSERT INTO process (id, seq, alias, name, ver) VALUES (?, ?, ?, ?, ?)`,
		p.ID, p.Seq, p.Alias, p.Name, p.Ver)
	return err
}

func (s *Store) CreateProVer(ctx context.Context, pv *nova.ProVer) error {
	pv.ID = newUUID()
	err := s.db.QueryRowContext(ctx,
		`SELECT COALESCE((SELECT MAX(seq) FROM pro_ver), 0) + 1`).Scan(&pv.Seq)
	if err != nil {
		return fmt.Errorf("generate seq: %w", err)
	}
	release := 0
	if pv.IsRelease {
		release = 1
	}
	_, err = s.db.ExecContext(ctx,
		`INSERT INTO pro_ver (id, seq, pro_id, ver, is_release) VALUES (?, ?, ?, ?, ?)`,
		pv.ID, pv.Seq, pv.ProID, pv.Ver, release)
	return err
}

func (s *Store) CreateAct(ctx context.Context, a *nova.Act) error {
	a.ID = newUUID()
	err := s.db.QueryRowContext(ctx,
		`SELECT COALESCE((SELECT MAX(seq) FROM act), 0) + 1`).Scan(&a.Seq)
	if err != nil {
		return fmt.Errorf("generate seq: %w", err)
	}
	editable := 0
	if a.Editable {
		editable = 1
	}
	force := 0
	if a.ForceOpinion {
		force = 1
	}
	waiActs := joinJoin(a.WaitActs)
	_, err = s.db.ExecContext(ctx,
		`INSERT INTO act (id, seq, pro_id, name, title, act_type, act_ver, editable, force_opinion, wait_acts)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		a.ID, a.Seq, a.ProID, a.Name, a.Title, a.Type, a.Ver, editable, force, waiActs)
	return err
}
// CreateLink creates a new act_link record.
func (s *Store) CreateLink(ctx context.Context, l *nova.Link) error {
	// act_link id is TEXT PRIMARY KEY in schema, but Link.ID is int64 in types.go
	// (cannot modify types.go). We store the sequential integer as a text string,
	// which go-sqlite3 can scan back into int64.
	var nextID int64
	err := s.db.QueryRowContext(ctx,
		`SELECT COALESCE((SELECT MAX(CAST(id AS INTEGER)) FROM act_link), 0) + 1`).Scan(&nextID)
	if err != nil {
		return fmt.Errorf("generate id: %w", err)
	}
	_, err = s.db.ExecContext(ctx,
		`INSERT INTO act_link (id, pro_ver_id, act_id, prev_act_id, title, link_type)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		fmt.Sprintf("%d", nextID), l.ProVerID, l.ActID, l.PrevActID, l.Title, l.Type)
	if err != nil {
		return err
	}
	l.ID = nextID
	return nil
}

// ─── Process Design / Management ──────────────────────────────────────────────

func (s *Store) ListPros(ctx context.Context) ([]*nova.Pro, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, seq, alias, name, ver, channel FROM process ORDER BY seq`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var pros []*nova.Pro
	for rows.Next() {
		p := &nova.Pro{}
		if err := rows.Scan(&p.ID, &p.Seq, &p.Alias, &p.Name, &p.Ver, &p.Channel); err != nil {
			return nil, err
		}
		pros = append(pros, p)
	}
	if pros == nil {
		pros = []*nova.Pro{}
	}
	return pros, rows.Err()
}

func (s *Store) DeletePro(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Delete man_rules for acts under this process
	_, _ = s.db.ExecContext(ctx,
		`DELETE FROM man_rule WHERE act_id IN (SELECT id FROM act WHERE pro_id = ?)`, id)

	// Delete act_links for all pro_vers under this process
	_, _ = s.db.ExecContext(ctx,
		`DELETE FROM act_link WHERE pro_ver_id IN (SELECT id FROM pro_ver WHERE pro_id = ?)`, id)

	// Delete acts
	_, _ = s.db.ExecContext(ctx, `DELETE FROM act WHERE pro_id = ?`, id)

	// Delete pro_vers
	_, _ = s.db.ExecContext(ctx, `DELETE FROM pro_ver WHERE pro_id = ?`, id)

	// Delete process
	_, err := s.db.ExecContext(ctx, `DELETE FROM process WHERE id = ?`, id)
	return err
}

func (s *Store) DeleteActsByProVer(ctx context.Context, proVerID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Get pro_id from pro_ver
	var proID string
	err := s.db.QueryRowContext(ctx, `SELECT pro_id FROM pro_ver WHERE id = ?`, proVerID).Scan(&proID)
	if err != nil {
		return err
	}

	// Delete man_rules for acts under this process
	_, _ = s.db.ExecContext(ctx,
		`DELETE FROM man_rule WHERE act_id IN (SELECT id FROM act WHERE pro_id = ?)`, proID)

	// Delete acts
	_, err = s.db.ExecContext(ctx, `DELETE FROM act WHERE pro_id = ?`, proID)
	return err
}

func (s *Store) DeleteLinksByProVer(ctx context.Context, proVerID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.db.ExecContext(ctx, `DELETE FROM act_link WHERE pro_ver_id = ?`, proVerID)
	return err
}

func (s *Store) DeleteManRulesByAct(ctx context.Context, actID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.db.ExecContext(ctx, `DELETE FROM man_rule WHERE act_id = ?`, actID)
	return err
}

func (s *Store) UpdatePro(ctx context.Context, p *nova.Pro) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.db.ExecContext(ctx,
		`UPDATE process SET name = ?, ver = ?, channel = ? WHERE id = ?`,
		p.Name, p.Ver, p.Channel, p.ID)
	return err
}

func (s *Store) ListProVers(ctx context.Context, proID string) ([]*nova.ProVer, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, seq, pro_id, ver, is_release FROM pro_ver WHERE pro_id = ? ORDER BY ver`, proID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var pvs []*nova.ProVer
	for rows.Next() {
		pv := &nova.ProVer{}
		var release int
		if err := rows.Scan(&pv.ID, &pv.Seq, &pv.ProID, &pv.Ver, &release); err != nil {
			return nil, err
		}
		pv.IsRelease = release != 0
		pvs = append(pvs, pv)
	}
	if pvs == nil {
		pvs = []*nova.ProVer{}
	}
	return pvs, rows.Err()
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

func split(s string) []string {
	if s == "" {
		return nil
	}
	parts := make([]string, 0, 4)
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == ',' {
			parts = append(parts, s[start:i])
			start = i + 1
		}
	}
	parts = append(parts, s[start:])
	return parts
}

func joinJoin(parts []string) string {
	if len(parts) == 0 {
		return ""
	}
	b := make([]byte, 0, len(parts)*16)
	for i, p := range parts {
		if i > 0 {
			b = append(b, ',')
		}
		b = append(b, p...)
	}
	return string(b)
}

func formatTime(t *time.Time) *string {
	if t == nil {
		return nil
	}
	s := t.UTC().Format(time.RFC3339)
	return &s
}

func parseTime(ns sql.NullString) *time.Time {
	if !ns.Valid || ns.String == "" {
		return nil
	}
	t, err := time.Parse(time.RFC3339, ns.String)
	if err != nil {
		return nil
	}
	return &t
}

// ─── User CRUD ────────────────────────────────────────────────────────────────

func (s *Store) ListUsers(ctx context.Context) ([]*nova.User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rows, err := s.db.QueryContext(ctx, `SELECT id, uid, name, password_hash, email, phone, dept, state, seq,
		COALESCE(created_at,''), COALESCE(updated_at,'') FROM users ORDER BY seq`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var users []*nova.User
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	if users == nil {
		users = []*nova.User{}
	}
	return users, nil
}

func (s *Store) GetUser(ctx context.Context, id string) (*nova.User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	row := s.db.QueryRowContext(ctx, `SELECT id, uid, name, password_hash, email, phone, dept, state, seq,
		COALESCE(created_at,''), COALESCE(updated_at,'') FROM users WHERE id = ?`, id)
	u, err := scanUser(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return u, err
}

func (s *Store) GetUserByUID(ctx context.Context, uid string) (*nova.User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	row := s.db.QueryRowContext(ctx, `SELECT id, uid, name, password_hash, email, phone, dept, state, seq,
		COALESCE(created_at,''), COALESCE(updated_at,'') FROM users WHERE uid = ?`, uid)
	u, err := scanUser(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return u, err
}

func (s *Store) CreateUser(ctx context.Context, u *nova.User) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	u.ID = newUUID()
	now := time.Now().UTC().Format("2006-01-02 15:04:05")
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO users (id, uid, name, password_hash, email, phone, dept, state, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		u.ID, u.UID, u.Name, u.PasswordHash, u.Email, u.Phone, u.Dept, u.State, now, now)
	if err != nil {
		return err
	}
	// Set seq
	var seq int64
	s.db.QueryRowContext(ctx, `SELECT COALESCE(MAX(seq), 0) + 1 FROM users`).Scan(&seq)
	u.Seq = seq
	s.db.ExecContext(ctx, `UPDATE users SET seq = ? WHERE id = ?`, seq, u.ID)
	if t, err := time.Parse("2006-01-02 15:04:05", now); err == nil {
		u.CreatedAt = t
		u.UpdatedAt = t
	}
	return nil
}

func (s *Store) UpdateUser(ctx context.Context, u *nova.User) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().UTC().Format("2006-01-02 15:04:05")
	_, err := s.db.ExecContext(ctx,
		`UPDATE users SET uid=?, name=?, email=?, phone=?, dept=?, state=?, updated_at=? WHERE id=?`,
		u.UID, u.Name, u.Email, u.Phone, u.Dept, u.State, now, u.ID)
	return err
}

func (s *Store) DeleteUser(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.db.ExecContext(ctx, `DELETE FROM users WHERE id = ?`, id)
	return err
}

func (s *Store) SetPassword(ctx context.Context, userID string, hash string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.db.ExecContext(ctx, `UPDATE users SET password_hash = ? WHERE id = ?`, hash, userID)
	return err
}

func (s *Store) MigrateAddPasswordColumn() error {
	// Try adding password_hash column if it doesn't exist (for existing DBs)
	_, err := s.db.Exec(`ALTER TABLE users ADD COLUMN password_hash TEXT NOT NULL DEFAULT ''`)
	if err != nil {
		// Column already exists — ignore error
	}
	return nil
}

func scanUser(row interface{ Scan(...any) error }) (*nova.User, error) {
	var u nova.User
	var createdAt, updatedAt string
	if err := row.Scan(&u.ID, &u.UID, &u.Name, &u.PasswordHash, &u.Email, &u.Phone, &u.Dept, &u.State, &u.Seq,
		&createdAt, &updatedAt); err != nil {
		return nil, err
	}
	if t, err := time.Parse("2006-01-02 15:04:05", createdAt); err == nil {
		u.CreatedAt = t
	}
	if t, err := time.Parse("2006-01-02 15:04:05", updatedAt); err == nil {
		u.UpdatedAt = t
	}
	return &u, nil
}

// ─── Extended Query Methods for Phase 1 ──────────────────────────────────────

// GetAct retrieves a single activity by ID.
func (s *Store) GetAct(ctx context.Context, id string) (*nova.Act, error) {
	row := s.db.QueryRowContext(ctx, `SELECT id, pro_id, name, title, act_type, act_ver,
		editable, force_opinion, wait_acts FROM act WHERE id = ?`, id)
	var a nova.Act
	var waitActs string
	var editable, forceOpinion int
	if err := row.Scan(&a.ID, &a.ProID, &a.Name, &a.Title, &a.Type, &a.Ver,
		&editable, &forceOpinion, &waitActs); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	a.Editable = editable != 0
	a.ForceOpinion = forceOpinion != 0
	a.WaitActs = split(waitActs)
	return &a, nil
}

// GetLinksByAct returns all forward links where prev_act_id = actID.
func (s *Store) GetLinksByAct(ctx context.Context, actID string) ([]*nova.Link, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, pro_ver_id, act_id, prev_act_id,
		title, link_type FROM act_link WHERE prev_act_id = ?`, actID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var links []*nova.Link
	for rows.Next() {
		var l nova.Link
		if err := rows.Scan(&l.ID, &l.ProVerID, &l.ActID, &l.PrevActID, &l.Title, &l.Type); err != nil {
			return nil, err
		}
		links = append(links, &l)
	}
	return links, nil
}

// GetEntityStepLogs returns step logs for an entity.
func (s *Store) GetEntityStepLogs(ctx context.Context, entityID string) ([]*nova.StepLog, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, entity_id, step_id, cur_act_id,
		dest_act_id, dest_act_name, dest_act_title, dest_act_type, link_type, logged_at
		FROM step_log WHERE entity_id = ? ORDER BY logged_at`, entityID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var logs []*nova.StepLog
	for rows.Next() {
		var l nova.StepLog
		var loggedAt string
		if err := rows.Scan(&l.ID, &l.EntityID, &l.StepID, &l.CurActID,
			&l.DestActID, &l.DestActName, &l.DestActTitle, &l.DestActType,
			&l.LinkType, &loggedAt); err != nil {
			return nil, err
		}
		if parsed, err := time.Parse("2006-01-02 15:04:05", loggedAt); err == nil {
			l.LoggedAt = parsed
		}
		logs = append(logs, &l)
	}
	return logs, nil
}

// GetEntityHandleLogs returns handle logs for an entity.
func (s *Store) GetEntityHandleLogs(ctx context.Context, entityID string) ([]*nova.HandleLog, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, entity_id, step_id, task_id, act_id,
		act_title, handler_uid, handler_name, log, arrive_at, accept_at, finish_at, client_type, insert_ip
		FROM handle_log WHERE entity_id = ? ORDER BY COALESCE(finish_at, arrive_at)`, entityID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var logs []*nova.HandleLog
	for rows.Next() {
		var l nova.HandleLog
		var arriveStr, acceptStr, finishStr sql.NullString
		if err := rows.Scan(&l.ID, &l.EntityID, &l.StepID, &l.TaskID, &l.ActID,
			&l.ActTitle, &l.HandlerUID, &l.HandlerName, &l.Content,
			&arriveStr, &acceptStr, &finishStr, &l.ClientType, &l.InsertIP); err != nil {
			return nil, err
		}
		if arriveStr.Valid {
			if t, err := time.Parse("2006-01-02 15:04:05", arriveStr.String); err == nil {
				l.ArriveAt = &t
			}
		}
		if acceptStr.Valid {
			if t, err := time.Parse("2006-01-02 15:04:05", acceptStr.String); err == nil {
				l.AcceptAt = &t
			}
		}
		if finishStr.Valid {
			if t, err := time.Parse("2006-01-02 15:04:05", finishStr.String); err == nil {
				l.FinishAt = &t
			}
		}
		logs = append(logs, &l)
	}
	return logs, nil
}

// ListEntities lists all entities, optionally filtered by state.
func (s *Store) ListEntities(ctx context.Context, stateFilter *int) ([]*nova.Entity, error) {
	query := `SELECT id, code, pro_id, pro_ver, title, state,
		draft_uid, draft_name, draft_dept, parent_id, serial_num,
		send_at, over_at, created_at, updated_at FROM entity`
	var args []any
	if stateFilter != nil {
		query += " WHERE state = ?"
		args = append(args, *stateFilter)
	}
	query += " ORDER BY seq DESC"

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entities []*nova.Entity
	for rows.Next() {
		e, err := scanEntity(rows)
		if err != nil {
			return nil, err
		}
		entities = append(entities, e)
	}
	return entities, nil
}

// GetUserDoneEntities returns entities that the specified handler has completed.
func (s *Store) GetUserDoneEntities(ctx context.Context, handlerUID string) ([]*nova.Entity, error) {
	// Join through handle_log to find entities the user acted on
	rows, err := s.db.QueryContext(ctx, `SELECT DISTINCT e.id, e.code, e.pro_id, e.pro_ver,
		e.title, e.state, e.draft_uid, e.draft_name, e.draft_dept,
		e.parent_id, e.serial_num, e.send_at, e.over_at, e.created_at, e.updated_at
		FROM entity e
		INNER JOIN handle_log hl ON hl.entity_id = e.id AND hl.handler_uid = ?
		WHERE e.state >= 1
		ORDER BY COALESCE(e.over_at, e.send_at, e.created_at) DESC`, handlerUID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entities []*nova.Entity
	for rows.Next() {
		e, err := scanEntity(rows)
		if err != nil {
			return nil, err
		}
		entities = append(entities, e)
	}
	return entities, nil
}

// scanEntity scans an entity row from a standard entity query.
// Assumes columns: id, code, pro_id, pro_ver, title, state,
// draft_uid, draft_name, draft_dept, parent_id, serial_num,
// send_at, over_at, created_at, updated_at
func scanEntity(row interface{ Scan(...any) error }) (*nova.Entity, error) {
	var e nova.Entity
	var sendAt, overAt, createdAt, updatedAt sql.NullString
	var serialNum, draftDept sql.NullString

	if err := row.Scan(&e.ID, &e.Code, &e.ProID, &e.ProVer, &e.Title, &e.State,
		&e.DraftUID, &e.DraftName, &draftDept,
		&e.ParentID, &serialNum,
		&sendAt, &overAt, &createdAt, &updatedAt); err != nil {
		return nil, err
	}

	e.DraftDept = draftDept.String
	if serialNum.Valid {
		e.SerialNum = serialNum.String
	}
	if t, err := time.Parse("2006-01-02 15:04:05", sendAt.String); err == nil {
		e.SendAt = &t
	}
	if t, err := time.Parse("2006-01-02 15:04:05", overAt.String); err == nil {
		e.OverAt = &t
	}
	if t, err := time.Parse("2006-01-02 15:04:05", createdAt.String); err == nil {
		e.CreatedAt = t
	}
	if t, err := time.Parse("2006-01-02 15:04:05", updatedAt.String); err == nil {
		e.UpdatedAt = t
	}

	return &e, nil
}
