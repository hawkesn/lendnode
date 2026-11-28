package moonlighter

import (
	"context"
	"errors"
	"time"
)

// Pod is a unit running on the node; DaemonSet pods are never evicted.
type Pod struct {
	Namespace, Name string
	DaemonSet       bool
}

// NodeState is what the cluster API reports about this node.
type NodeState struct {
	Unschedulable bool
	Ours          bool // the cordon was made by this agent
	Since         time.Time
}

// Cluster is the part of the cluster API the agent needs.
type Cluster interface {
	Node(ctx context.Context) (NodeState, error)
	Cordon(ctx context.Context, now time.Time) error
	Uncordon(ctx context.Context) error
	Pods(ctx context.Context) ([]Pod, error)
	Evict(ctx context.Context, p Pod) error
}

// Agent lends the node while idle and takes it back when the owner returns.
type Agent struct {
	Cluster       Cluster
	LowerPriority func(ctx context.Context) error // optional
}

// Tick lends the node back when Decide says idle, reclaims it otherwise.
func (a Agent) Tick(ctx context.Context, s Settings, sig Signals, now time.Time) error {
	if ok, _ := Decide(s, sig, now); ok {
		return a.Lend(ctx)
	}
	return a.Reclaim(ctx, s, now)
}

// Lend uncordons the node, but only a cordon this agent made itself.
func (a Agent) Lend(ctx context.Context) error {
	st, err := a.Cluster.Node(ctx)
	if err != nil {
		return err
	}
	if st.Unschedulable && !st.Ours || !st.Unschedulable && st.Since.IsZero() {
		return nil
	}
	return a.Cluster.Uncordon(ctx)
}

// Reclaim cordons before anything else, then applies the leave mode.
func (a Agent) Reclaim(ctx context.Context, s Settings, now time.Time) error {
	st, err := a.Cluster.Node(ctx)
	if err != nil {
		return err
	}
	if !st.Unschedulable || st.Since.IsZero() {
		if err := a.Cluster.Cordon(ctx, now); err != nil {
			return err
		}
		st.Since = now
	}
	switch s.LeaveMode {
	case LeaveSlow, LeaveNow:
		if a.LowerPriority != nil {
			if err := a.LowerPriority(ctx); err != nil {
				return err
			}
		}
		if s.LeaveMode == LeaveSlow {
			return nil
		}
	case LeaveFinish:
		if now.Sub(st.Since) < s.MaxWait {
			return nil
		}
	default:
		return nil
	}
	pods, err := a.Cluster.Pods(ctx)
	if err != nil {
		return err
	}
	var errs []error
	for _, p := range pods {
		if p.DaemonSet {
			continue
		}
		if err := a.Cluster.Evict(ctx, p); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}
