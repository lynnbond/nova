package nova

import (
	"context"
	"fmt"
)

// processStore defines the storage operations needed by ProcessBuilder.
type processStore interface {
	CreateProcess(ctx context.Context, p *Pro) error
	CreateProVer(ctx context.Context, pv *ProVer) error
	CreateAct(ctx context.Context, a *Act) error
	CreateLink(ctx context.Context, l *Link) error
	CreateManRule(ctx context.Context, r *ManRule) error
}

// ProcessBuilder provides a chainable API for defining workflow processes.
type ProcessBuilder struct {
	alias string
	name  string
	acts  []*actDef
	links []*linkDef
	cur   *actDef
}

type actDef struct {
	name         string
	title        string
	typ          ActType
	rule         *ManRuleDef
	waitActs     []string
	forceOpinion bool
}

// ManRuleDef configures handler assignment for a manual activity.
type ManRuleDef struct {
	Policy     ManPolicy
	Groups     []string
	SelAllowed bool
}

type linkDef struct {
	from           string
	to             string
	decisionFilter string
}

// NewProcess starts building a new process definition.
func NewProcess(alias, name string) *ProcessBuilder {
	return &ProcessBuilder{alias: alias, name: name}
}

// Act adds an activity to the process definition.
func (b *ProcessBuilder) Act(name string, typ ActType) *ProcessBuilder {
	a := &actDef{
		name:  name,
		title: name,
		typ:   typ,
	}
	if typ == ActTypeSTART || typ == ActTypeEND {
		a.title = "活动-" + name
	}
	b.acts = append(b.acts, a)
	b.cur = a
	return b
}

// Title sets the display title for the current activity.
func (b *ProcessBuilder) Title(title string) *ProcessBuilder {
	if b.cur != nil {
		b.cur.title = title
	}
	return b
}

// On configures the handler rule for a MANUAL activity.
func (b *ProcessBuilder) On(policy ManPolicy) *ProcessBuilder {
	if b.cur != nil {
		b.cur.rule = &ManRuleDef{Policy: policy}
	}
	return b
}

// Group sets candidate handler groups (dept/team/role IDs) for the current rule.
func (b *ProcessBuilder) Group(groups ...string) *ProcessBuilder {
	if b.cur != nil && b.cur.rule != nil {
		b.cur.rule.Groups = groups
	}
	return b
}

// SelAllowed allows handlers to self-select from the candidate pool.
func (b *ProcessBuilder) SelAllowed() *ProcessBuilder {
	if b.cur != nil && b.cur.rule != nil {
		b.cur.rule.SelAllowed = true
	}
	return b
}

// ForceOpinion marks the current activity as requiring a mandatory opinion on submit.
func (b *ProcessBuilder) ForceOpinion() *ProcessBuilder {
	if b.cur != nil {
		b.cur.forceOpinion = true
	}
	return b
}

// WaitFor configures which activities a WAIT activity should wait for.
func (b *ProcessBuilder) WaitFor(actNames ...string) *ProcessBuilder {
	if b.cur != nil {
		b.cur.waitActs = actNames
	}
	return b
}

// Link adds a directed edge from one activity to another.
func (b *ProcessBuilder) Link(from, to string) *ProcessBuilder {
	b.links = append(b.links, &linkDef{from: from, to: to})
	return b
}

// Decision sets a decision filter label on the most recently added link.
// When set, this link is only taken when the submitter chooses a matching decision.
func (b *ProcessBuilder) Decision(label string) *ProcessBuilder {
	if len(b.links) == 0 {
		return b
	}
	b.links[len(b.links)-1].decisionFilter = label
	return b
}

// Validate checks the process definition for completeness.
func (b *ProcessBuilder) Validate() error {
	if b.alias == "" {
		return fmt.Errorf("process alias is required")
	}
	if len(b.acts) == 0 {
		return fmt.Errorf("process must have at least one activity")
	}

	actMap := make(map[string]*actDef, len(b.acts))
	hasStart := false
	hasEnd := false
	for _, a := range b.acts {
		if a.name == "" {
			return fmt.Errorf("activity name must not be empty")
		}
		if _, dup := actMap[a.name]; dup {
			return fmt.Errorf("duplicate activity name: %q", a.name)
		}
		actMap[a.name] = a
		switch a.typ {
		case ActTypeSTART:
			if hasStart {
				return fmt.Errorf("duplicate START activity")
			}
			hasStart = true
		case ActTypeEND:
			hasEnd = true
		case ActTypeWAIT:
			if len(a.waitActs) == 0 {
				return fmt.Errorf("WAIT activity %q must have WaitFor(...) configured", a.name)
			}
		}
	}
	if !hasEnd {
		return fmt.Errorf("process must have an END activity")
	}
	if len(b.links) == 0 {
		return fmt.Errorf("process must have at least one link")
	}
	for _, l := range b.links {
		if _, ok := actMap[l.from]; !ok {
			return fmt.Errorf("link from undefined activity %q", l.from)
		}
		if _, ok := actMap[l.to]; !ok {
			return fmt.Errorf("link to undefined activity %q", l.to)
		}
	}

	// Every non-START must be reachable
	incoming := map[string]int{}
	for _, l := range b.links {
		incoming[l.to]++
	}
	for _, a := range b.acts {
		if a.typ != ActTypeSTART && incoming[a.name] == 0 {
			return fmt.Errorf("activity %q has no incoming link", a.name)
		}
	}
	return nil
}

// Store persists the process definition.
func (b *ProcessBuilder) Store(ctx context.Context, s Store) (*Pro, error) {
	if err := b.Validate(); err != nil {
		return nil, fmt.Errorf("validate: %w", err)
	}
	ps, ok := s.(processStore)
	if !ok {
		return nil, fmt.Errorf("store does not support process creation (missing CreateProcess)")
	}

	// Create process
	p := &Pro{Alias: b.alias, Name: b.name, Ver: 1}
	if err := ps.CreateProcess(ctx, p); err != nil {
		return nil, fmt.Errorf("create process: %w", err)
	}

	// Create version
	pv := &ProVer{ProID: p.ID, Ver: 1, IsRelease: true}
	if err := ps.CreateProVer(ctx, pv); err != nil {
		return nil, fmt.Errorf("create version: %w", err)
	}

	// Create activities
	nameToID := make(map[string]string, len(b.acts))
	for _, a := range b.acts {
		act := &Act{
			ProID:        p.ID,
			Name:         a.name,
			Title:        a.title,
			Type:         a.typ,
			Ver:          1,
			Editable:     a.typ == ActTypeMANUAL,
			ForceOpinion: a.forceOpinion,
		}
		if len(a.waitActs) > 0 {
			act.WaitActs = a.waitActs
		}
		if err := ps.CreateAct(ctx, act); err != nil {
			return nil, fmt.Errorf("create act %q: %w", a.name, err)
		}
		nameToID[a.name] = act.ID
	}

	// Create links
	for i, l := range b.links {
		link := &Link{
			ProVerID:       pv.ID,
			ActID:          nameToID[l.to],
			PrevActID:      nameToID[l.from],
			Title:          fmt.Sprintf("L%d", i+1),
			Type:           LinkTypeFORWARD,
			DecisionFilter: l.decisionFilter,
		}
		if err := ps.CreateLink(ctx, link); err != nil {
			return nil, fmt.Errorf("create link %s→%s: %w", l.from, l.to, err)
		}
	}

	// Create rules
	for _, a := range b.acts {
		if a.rule != nil {
			r := &ManRule{
				ActID:      nameToID[a.name],
				BaseOn:     HandlerBaseChannel,
				Policy:     a.rule.Policy,
				SelAllowed: a.rule.SelAllowed,
				GroupSet:   a.rule.Groups,
			}
			if err := ps.CreateManRule(ctx, r); err != nil {
				return nil, fmt.Errorf("create rule for %q: %w", a.name, err)
			}
		}
	}

	return p, nil
}
