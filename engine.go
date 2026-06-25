package nova

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Engine is the core workflow engine.
// It is the entry point for all workflow operations.
// Thread-safe: use one Engine per process definition.
type Engine struct {
	mu sync.Mutex

	pro      *Pro
	proVer   *ProVer
	acts     map[string]*Act // name -> act
	actsByID map[string]*Act
	links    []*Link

	// Outbound links indexed by prevActId
	outLinks map[string][]*Link

	// First activity
	firstAct *Act

	store Store

	// Optional components
	hookRouter *HookRouter
}

// EngineConfig configures an Engine instance.
type EngineConfig struct {
	Store      Store
	ProAlias   string
	ProVer     int // 0 = latest
	HookRouter *HookRouter
}

// NewEngine creates a new Engine for the given process definition.
func NewEngine(ctx context.Context, cfg EngineConfig) (*Engine, error) {
	pro, err := cfg.Store.GetProByAlias(ctx, cfg.ProAlias)
	if err != nil {
		return nil, fmt.Errorf("get pro: %w", err)
	}
	if pro == nil {
		return nil, fmt.Errorf("process not found: %s", cfg.ProAlias)
	}

	ver := cfg.ProVer
	if ver == 0 {
		ver = pro.Ver
	}

	proVer, err := cfg.Store.GetProVer(ctx, pro.ID, ver)
	if err != nil {
		return nil, fmt.Errorf("get pro ver: %w", err)
	}
	if proVer == nil {
		return nil, fmt.Errorf("process version not found: %s v%d", cfg.ProAlias, ver)
	}

	acts, err := cfg.Store.GetActsByProVer(ctx, proVer.ID)
	if err != nil {
		return nil, fmt.Errorf("get acts: %w", err)
	}
	links, err := cfg.Store.GetLinksByProVer(ctx, proVer.ID)
	if err != nil {
		return nil, fmt.Errorf("get links: %w", err)
	}

	// Build indices
	actsByName := make(map[string]*Act, len(acts))
	actsByID := make(map[string]*Act, len(acts))
	for _, a := range acts {
		actsByName[a.Name] = a
		actsByID[a.ID] = a
	}

	// Populate act references on links
	outLinks := make(map[string][]*Link)
	for _, l := range links {
		l.Act = actsByID[l.ActID]
		outLinks[l.PrevActID] = append(outLinks[l.PrevActID], l)
	}

	firstAct, err := cfg.Store.GetFirstAct(ctx, pro.ID, ver)
	if err != nil {
		return nil, fmt.Errorf("get first act: %w", err)
	}

	return &Engine{
		pro:        pro,
		proVer:     proVer,
		acts:       actsByName,
		actsByID:   actsByID,
		links:      links,
		outLinks:   outLinks,
		firstAct:   firstAct,
		store:      cfg.Store,
		hookRouter: cfg.HookRouter,
	}, nil
}

// ─── Public API ───────────────────────────────────────────────────────────────

// Pro returns the process definition.
func (e *Engine) Pro() *Pro { return e.pro }

// FirstAct returns the first activity of the process.
func (e *Engine) FirstAct() *Act { return e.firstAct }

// CreateEntity creates a new draft entity.
func (e *Engine) CreateEntity(ctx context.Context, code, title string, handlerUID, handlerName, handlerDept string, parentID string) (*Entity, error) {
	ent := &Entity{
		Code:      code,
		ProID:     e.pro.ID,
		ProVer:    e.proVer.Ver,
		Title:     title,
		State:     EntityStateDRAFT,
		DraftUID:  handlerUID,
		DraftName: handlerName,
		DraftDept: handlerDept,
		ParentID:  parentID,
	}
	if err := e.store.CreateEntity(ctx, ent); err != nil {
		return nil, fmt.Errorf("create entity: %w", err)
	}

	// Create first step
	step := &Step{
		EntityID: ent.ID,
		ActID:    e.firstAct.ID,
		ActName:  e.firstAct.Name,
	}
	if err := e.store.CreateStep(ctx, step); err != nil {
		return nil, fmt.Errorf("create step: %w", err)
	}

	// Create first task
	task := &Task{
		EntityID: ent.ID,
		StepID:   step.ID,
		ActID:    e.firstAct.ID,
		ActName:  e.firstAct.Name,
		ActTitle: e.firstAct.Title,
		State:    TaskStateTODO,
		StartAt:  timeNow(),
	}
	if err := e.store.CreateTask(ctx, task); err != nil {
		return nil, fmt.Errorf("create task: %w", err)
	}

	return ent, nil
}

// Session opens a new Session bound to an existing entity.
// This is the primary API for interacting with a running entity.
func (e *Engine) Session(ctx context.Context, entityID string, handlerUID, handlerName string) (*Session, error) {
	ent, err := e.store.GetEntity(ctx, entityID)
	if err != nil {
		return nil, err
	}
	if ent == nil {
		return nil, fmt.Errorf("entity not found: %s", entityID)
	}

	handlerCtx := WithHandler(ctx, handlerUID, handlerName)
	sess := &Session{
		eng:    e,
		ctx:    handlerCtx,
		entity: ent,
		store:  e.store,
	}

	// Determine initial state from entity
	switch ent.State {
	case EntityStateDRAFT:
		sess.state = SessionStateHandle // draft entity is ready to handle
	case EntityStateSUBMIT, EntityStateBACK:
		sess.state = SessionStateHandle // in-process
	case EntityStateOVER, EntityStateCANCEL:
		sess.state = SessionStateView // completed
	default:
		sess.state = SessionStateStart // new
	}

	return sess, nil
}

// Session provides the stateful workflow interaction API.
// Modeled after the Nova4 java session pattern:
//
//	sess.Save(formData)
//	sess.Submit()
//	sess.SubmitWith(nil)          // with form data
//	sess.PreSubmit()              // validate without committing
type Session struct {
	eng        *Engine
	ctx        context.Context
	entity     *Entity
	store      Store

	state      SessionState
	submitInfo *SubmitInfo
}

// Entity returns the current entity.
func (s *Session) Entity() *Entity { return s.entity }

// State returns the current session state.
func (s *Session) State() SessionState { return s.state }

// Save persists the entity.
// If this is a new entity (StateStart), transitions to HandleState.
// If already in HandleState, updates the entity.
func (s *Session) Save() error {
	switch s.state {
	case SessionStateStart:
		return s.doStart()
	case SessionStateHandle:
		return s.doSave()
	default:
		return fmt.Errorf("cannot save in state %d", s.state)
	}
}

// PreSubmit validates the submission without committing.
func (s *Session) PreSubmit() (*SubmitInfo, error) {
	if s.state != SessionStateHandle {
		return nil, fmt.Errorf("cannot pre-submit in state %d", s.state)
	}
	return s.doSubmit(false)
}

// Submit submits the entity to the next activity.
func (s *Session) Submit() (*SubmitInfo, error) {
	if s.state != SessionStateHandle {
		return nil, fmt.Errorf("cannot submit in state %d", s.state)
	}
	return s.doSubmit(true)
}

// ─── Internal: State Machine Transitions ──────────────────────────────────────

func (s *Session) doStart() error {
	next, ok := ValidTransition(SessionStateStart, OpSave)
	if !ok {
		return fmt.Errorf("invalid transition: start -> save")
	}
	s.state = next
	return nil
}

func (s *Session) doSave() error {
	// Entity is already persisted; just confirm state
	return nil
}

func (s *Session) doSubmit(isTrue bool) (*SubmitInfo, error) {
	// Fire BEF_SUBMIT hook
	if s.eng.hookRouter != nil {
		feedback := s.eng.hookRouter.DoHook(s.ctx, ThroughBefSubmit)
		if feedback == FeedbackAbandon {
			return nil, fmt.Errorf("submission abandoned by hook")
		}
	}

	stater := &submitEngine{
		session: s,
		store:   s.store,
		pro:     s.eng.pro,
		acts:    s.eng.acts,
		actsByID: s.eng.actsByID,
		outLinks: s.eng.outLinks,
		submitInfo: &SubmitInfo{},
		hookRouter: s.eng.hookRouter,
	}

	si, err := stater.execute(isTrue)
	if err != nil {
		return nil, err
	}
	s.submitInfo = si

	if isTrue {
		// Transition state
		next, ok := ValidTransition(SessionStateHandle, OpSubmit)
		if !ok {
			return nil, fmt.Errorf("invalid transition: handle -> submit")
		}
		s.state = next
	}

	return si, nil
}

// ─── Submit Engine (internal, stateless) ──────────────────────────────────────

type submitEngine struct {
	session    *Session
	store      Store
	pro        *Pro
	acts       map[string]*Act
	actsByID   map[string]*Act
	outLinks   map[string][]*Link
	submitInfo *SubmitInfo
	hookRouter *HookRouter
}

func (se *submitEngine) execute(isTrue bool) (*SubmitInfo, error) {
	entity := se.session.entity
	ctx := se.session.ctx

	// Get current tasks for this entity
	tasks, err := se.store.GetTasksByEntity(ctx, entity.ID)
	if err != nil {
		return nil, fmt.Errorf("get tasks: %w", err)
	}

	// Find current (active) task
	curTask := findActiveTask(tasks)
	if curTask == nil {
		return nil, fmt.Errorf("no active task found")
	}

	// Complete the current task
	curTask.State = TaskStateOVER
	now := time.Now().UTC()
	curTask.EndAt = &now
	if err := se.store.UpdateTaskState(ctx, curTask.ID, TaskStateOVER); err != nil {
		return nil, fmt.Errorf("finish task: %w", err)
	}

	// Update handle log
	if err := se.store.UpdateHandleLogFinish(ctx, curTask.ID, now.Unix()); err != nil {
		return nil, fmt.Errorf("update handle log: %w", err)
	}

	se.submitInfo.DoneTask = curTask

	// Check if this is the first submission (draft -> submit)
	if entity.State == EntityStateDRAFT && isTrue {
		if err := se.store.UpdateEntityState(ctx, entity.ID, EntityStateSUBMIT); err != nil {
			return nil, fmt.Errorf("update entity state: %w", err)
		}
		entity.State = EntityStateSUBMIT
		serial, err := se.store.NextSerial(ctx, se.pro.Alias)
		if err != nil {
			return nil, fmt.Errorf("next serial: %w", err)
		}
		entity.SerialNum = serial
		if err := se.store.UpdateEntitySerial(ctx, entity.ID, serial); err != nil {
			return nil, fmt.Errorf("update serial: %w", err)
		}
	}

	// Find next activities
	return se.gotoNext(ctx, entity, curTask)
}

func (se *submitEngine) gotoNext(ctx context.Context, entity *Entity, curTask *Task) (*SubmitInfo, error) {
	nextList := se.outLinks[curTask.ActID]
	if len(nextList) == 0 {
		return nil, fmt.Errorf("no next activity configured for act %s", curTask.ActID)
	}

	// Fire BEF_CHOOSE_NEXT_LINKS hook
	if se.hookRouter != nil {
		fb := se.hookRouter.DoHook(ctx, ThroughBefChooseLinks)
		if fb == FeedbackAbandon {
			return nil, fmt.Errorf("next-link selection abandoned by hook")
		}
	}

	// If multiple choices, use the first (caller can override via hook)
	next := nextList[0]

	return se.doSwitch(ctx, entity, curTask, next)
}

func (se *submitEngine) doSwitch(ctx context.Context, entity *Entity, curTask *Task, link *Link) (*SubmitInfo, error) {
	switch link.Act.Type {
	case ActTypeMANUAL:
		return se.gotoManualAct(ctx, entity, curTask, link)
	case ActTypeROUTER:
		return se.gotoRouterAct(ctx, entity, curTask, link)
	case ActTypeCONVERGE:
		return se.gotoConvergeAct(ctx, entity, curTask, link)
	case ActTypeWAIT:
		return se.gotoWaitingAct(ctx, entity, curTask, link)
	case ActTypeTAIL:
		return se.gotoTailAct(ctx, entity, curTask, link)
	case ActTypeEND:
		return se.gotoEndAct(ctx, entity, curTask, link)
	default:
		return nil, fmt.Errorf("unsupported act type: %s", link.Act.Type)
	}
}

// ─── Activity Type Handlers ───────────────────────────────────────────────────

func (se *submitEngine) gotoManualAct(ctx context.Context, entity *Entity, curTask *Task, link *Link) (*SubmitInfo, error) {
	rule, err := se.store.GetManRule(ctx, link.Act.ID)
	if err != nil {
		return nil, fmt.Errorf("get man rule: %w", err)
	}
	if rule == nil {
		return nil, fmt.Errorf("no manual rule for act %s", link.Act.ID)
	}

	// Fire BEF_CHOOSE_HANDLER hook
	if se.hookRouter != nil {
		fb := se.hookRouter.DoHook(ctx, ThroughBefChooseHandler)
		if fb == FeedbackAbandon {
			return nil, fmt.Errorf("handler selection abandoned by hook")
		}
	}

	// Create step
	step := &Step{
		EntityID: entity.ID,
		ActID:    link.Act.ID,
		ActName:  link.Act.Name,
	}
	if err := se.store.CreateStep(ctx, step); err != nil {
		return nil, fmt.Errorf("create step: %w", err)
	}

	// Determine handlers
	handlerSet := defaultHandlerSet(rule, link.Act.Name)

	switch rule.Policy {
	case ManPolicyCONCURRENT:
		// Multi-person concurrent: create one task per handler
		for _, h := range handlerSet {
			task := &Task{
				EntityID: entity.ID,
				StepID:   step.ID,
				ActID:    link.Act.ID,
				ActName:  link.Act.Name,
				ActTitle: link.Act.Title,
				State:    TaskStateTODO,
				Handlers: h.Name,
				StartAt:  timeNow(),
			}
			if err := se.store.CreateTask(ctx, task); err != nil {
				return nil, fmt.Errorf("create concurrent task: %w", err)
			}
			// Create todo
			todo := &Todo{
				TaskID:      task.ID,
				EntityID:    entity.ID,
				ActID:       link.Act.ID,
				ProID:       se.pro.ID,
				HandlerUID:  h.UID,
				HandlerName: h.Name,
				ActTitle:    link.Act.Title,
				EntityTitle: entity.Title,
				ProName:     se.pro.Name,
				ArriveAt:    timeNow(),
			}
			if err := se.store.CreateTodo(ctx, todo); err != nil {
				return nil, fmt.Errorf("create concurrent todo: %w", err)
			}
			se.submitInfo.NewTodos = append(se.submitInfo.NewTodos, todo)
		}
	case ManPolicySINGLE, ManPolicyEXCLUSIVE:
		// Single handler: one task assigned to first handler
		h := handlerSet[0]
		task := &Task{
			EntityID: entity.ID,
			StepID:   step.ID,
			ActID:    link.Act.ID,
			ActName:  link.Act.Name,
			ActTitle: link.Act.Title,
			State:    TaskStateTODO,
			Handlers: h.Name,
			StartAt:  timeNow(),
		}
		if err := se.store.CreateTask(ctx, task); err != nil {
			return nil, fmt.Errorf("create task: %w", err)
		}
		todo := &Todo{
			TaskID:      task.ID,
			EntityID:    entity.ID,
			ActID:       link.Act.ID,
			ProID:       se.pro.ID,
			HandlerUID:  h.UID,
			HandlerName: h.Name,
			ActTitle:    link.Act.Title,
			EntityTitle: entity.Title,
			ProName:     se.pro.Name,
			ArriveAt:    timeNow(),
		}
		if err := se.store.CreateTodo(ctx, todo); err != nil {
			return nil, fmt.Errorf("create todo: %w", err)
		}
		se.submitInfo.NewTodos = append(se.submitInfo.NewTodos, todo)
	}

	// Create step log
	stepLog := &StepLog{
		EntityID:     entity.ID,
		StepID:       step.ID,
		CurActID:     curTask.ActID,
		DestActID:    link.Act.ID,
		DestActName:  link.Act.Name,
		DestActTitle: link.Act.Title,
		DestActType:  link.Act.Type,
		LinkType:     link.Type,
	}
	if err := se.store.CreateStepLog(ctx, stepLog); err != nil {
		return nil, fmt.Errorf("create step log: %w", err)
	}

	se.submitInfo.Direct = link
	se.submitInfo.DestTargets = append(se.submitInfo.DestTargets, TargetInfo{
		Link: link, Rule: rule, Handlers: handlerSet,
	})

	return se.submitInfo, nil
}

func (se *submitEngine) gotoRouterAct(ctx context.Context, entity *Entity, curTask *Task, link *Link) (*SubmitInfo, error) {
	nextList := se.outLinks[link.ActID]
	if len(nextList) == 0 {
		return nil, fmt.Errorf("router act %s has no outgoing links", link.ActID)
	}

	for _, nextLink := range nextList {
		// Propagate waiting list
		nextLink.Act.WaitingList = append(nextLink.Act.WaitingList, link.Act.WaitingList...)

		// Record step for the router transition
		step := &Step{
			EntityID: entity.ID,
			ActID:    link.Act.ID,
			ActName:  link.Act.Name,
		}
		if err := se.store.CreateStep(ctx, step); err != nil {
			return nil, fmt.Errorf("create router step: %w", err)
		}

		createStepLog(se.store, entity.ID, step.ID,
			curTask.ActID, nextLink.Act.ID, nextLink.Act.Name,
			nextLink.Act.Title, nextLink.Act.Type, nextLink.Type)

		// Recurse for each branch
		if _, err := se.doSwitch(ctx, entity, curTask, nextLink); err != nil {
			return nil, fmt.Errorf("router branch %s: %w", nextLink.Act.Name, err)
		}
	}

	return se.submitInfo, nil
}

func (se *submitEngine) gotoConvergeAct(ctx context.Context, entity *Entity, curTask *Task, link *Link) (*SubmitInfo, error) {
	// Check if all parent activities' tasks are done
	tasks, err := se.store.GetTasksByEntity(ctx, entity.ID)
	if err != nil {
		return nil, fmt.Errorf("get tasks for converge: %w", err)
	}

	allDone := isAllPrevDone(tasks, link.Act.ID, se.outLinks)
	if !allDone {
		// Create a CONVERGING task to track progress
		task := &Task{
			EntityID: entity.ID,
			ActID:    link.Act.ID,
			ActName:  link.Act.Name,
			ActTitle: link.Act.Title,
			State:    TaskStateCONVERGING,
			Converge: se.actsByID[curTask.ActID].Seq,
		}
		if err := se.store.CreateTask(ctx, task); err != nil {
			return nil, fmt.Errorf("create converge task: %w", err)
		}
		return se.submitInfo, nil
	}

	// All done — find the next links FROM the converge activity itself,
	// NOT from curTask (curTask belongs to a completed branch).
	convergeNextList := se.outLinks[link.Act.ID]
	if len(convergeNextList) == 0 {
		return nil, fmt.Errorf("converge act %s has no outgoing links", link.Act.ID)
	}

	// Fire BEF_CHOOSE_NEXT_LINKS hook before proceeding
	if se.hookRouter != nil {
		fb := se.hookRouter.DoHook(ctx, ThroughBefChooseLinks)
		if fb == FeedbackAbandon {
			return nil, fmt.Errorf("next-link selection abandoned by hook")
		}
	}

	next := convergeNextList[0]
	return se.doSwitch(ctx, entity, curTask, next)
}

func (se *submitEngine) gotoWaitingAct(ctx context.Context, entity *Entity, curTask *Task, link *Link) (*SubmitInfo, error) {
	waitActs := link.Act.WaitActs
	if len(waitActs) == 0 {
		return nil, fmt.Errorf("wait act %s has no wait_acts configured", link.Act.Name)
	}

	tasks, err := se.store.GetTasksByEntity(ctx, entity.ID)
	if err != nil {
		return nil, fmt.Errorf("get tasks for wait: %w", err)
	}

	// Check if all waited activities are done
	allWaitedDone := true
	for _, waitName := range waitActs {
		waitedAct, ok := se.acts[waitName]
		if !ok {
			continue // act not found in this process
		}
		if !isActAllDone(tasks, waitedAct.ID) {
			allWaitedDone = false
			break
		}
	}

	// Move past wait to next activity
	nextList := se.outLinks[link.ActID]
	if len(nextList) == 0 {
		return nil, fmt.Errorf("wait act %s has no outgoing links", link.ActID)
	}
	nextLink := nextList[0]

	if !allWaitedDone {
		// Need to wait: create a WAITING task
		waitingStr := join(waitActs)
		task := &Task{
			EntityID: entity.ID,
			ActID:    link.Act.ID,
			ActName:  link.Act.Name,
			ActTitle: link.Act.Title,
			State:    TaskStateWAITING,
			Waiting:  waitingStr,
		}
		if err := se.store.CreateTask(ctx, task); err != nil {
			return nil, fmt.Errorf("create waiting task: %w", err)
		}

		// Also go to the next link directly
		step := &Step{
			EntityID: entity.ID,
			ActID:    link.Act.ID,
			ActName:  link.Act.Name,
		}
		if err := se.store.CreateStep(ctx, step); err != nil {
			return nil, fmt.Errorf("create wait step: %w", err)
		}
		nextLink.Act.WaitingList = append(nextLink.Act.WaitingList, link.Act.WaitingList...)
		return se.doSwitch(ctx, entity, curTask, nextLink)
	}

	// All waited acts done, proceed directly
	step := &Step{
		EntityID: entity.ID,
		ActID:    link.Act.ID,
		ActName:  link.Act.Name,
	}
	if err := se.store.CreateStep(ctx, step); err != nil {
		return nil, fmt.Errorf("create wait step: %w", err)
	}
	nextLink.Act.WaitingList = append(nextLink.Act.WaitingList, link.Act.WaitingList...)
	return se.doSwitch(ctx, entity, curTask, nextLink)
}

func (se *submitEngine) gotoTailAct(ctx context.Context, entity *Entity, curTask *Task, link *Link) (*SubmitInfo, error) {
	se.submitInfo.DestTargets = append(se.submitInfo.DestTargets, TargetInfo{Link: link})
	return se.submitInfo, nil
}

func (se *submitEngine) gotoEndAct(ctx context.Context, entity *Entity, curTask *Task, link *Link) (*SubmitInfo, error) {
	now := time.Now().UTC()

	// Update entity state
	if err := se.store.UpdateEntityState(ctx, entity.ID, EntityStateOVER); err != nil {
		return nil, fmt.Errorf("end entity: %w", err)
	}
	entity.State = EntityStateOVER
	entity.OverAt = &now

	se.submitInfo.DestTargets = append(se.submitInfo.DestTargets, TargetInfo{Link: link})

	// Clean up todos
	todos, err := se.store.GetEntityTodos(ctx, entity.ID)
	if err == nil {
		for _, todo := range todos {
			_ = se.store.DeleteTodo(ctx, todo.ID)
		}
	}

	return se.submitInfo, nil
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

func findActiveTask(tasks []*Task) *Task {
	for _, t := range tasks {
		if t.State == TaskStateTODO || t.State == TaskStatePROCESSING {
			return t
		}
	}
	return nil
}

// isAllPrevDone checks whether all tasks for activities that precede
// the converge activity (via prev links) have been completed.
func isAllPrevDone(tasks []*Task, convergeActID string, outLinks map[string][]*Link) bool {
	// Collect all prev act IDs that link to converge's parents
	prevActIDs := make(map[string]bool)
	for _, links := range outLinks {
		for _, l := range links {
			if l.ActID == convergeActID {
				prevActIDs[l.PrevActID] = true
			}
		}
	}

	for _, t := range tasks {
		if prevActIDs[t.ActID] && t.State != TaskStateOVER {
			return false
		}
	}
	return true
}

// isActAllDone checks whether all tasks for a given activity are done.
func isActAllDone(tasks []*Task, actID string) bool {
	hasAct := false
	for _, t := range tasks {
		if t.ActID == actID {
			hasAct = true
			if t.State != TaskStateOVER {
				return false
			}
		}
	}
	return hasAct
}

func defaultHandlerSet(rule *ManRule, actName string) []HandlerRef {
	// Map activity names to real demo users
	// For production, use HookRouter (ThroughBefChooseHandler) or DB lookup
	switch actName {
	case "draft", "起草申请":
		return []HandlerRef{{UID: "zhangsan", Name: "张三"}}
	case "concur_a", "会签A", "concur_b", "会签B":
		return []HandlerRef{
			{UID: "zhangsan", Name: "张三"},
			{UID: "lisi", Name: "李四"},
		}
	case "single_c", "单签C":
		return []HandlerRef{{UID: "wangwu", Name: "王五"}}
	case "approve", "OP审批", "fin_signing", "终审签字":
		return []HandlerRef{{UID: "admin", Name: "管理员"}}
	default:
		return []HandlerRef{{UID: "admin", Name: "管理员"}}
	}
}

func createStepLog(store Store, entityID string, stepID int64, curActID, destActID string, destName, destTitle string, destType ActType, linkType LinkType) {
	_ = store.CreateStepLog(context.Background(), &StepLog{
		EntityID: entityID, StepID: stepID,
		CurActID: curActID, DestActID: destActID,
		DestActName: destName, DestActTitle: destTitle,
		DestActType: destType, LinkType: linkType,
	})
}

func timeNow() *time.Time {
	t := time.Now().UTC()
	return &t
}

func join(parts []string) string {
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
