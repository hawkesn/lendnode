package moonlighter

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"
)

type fakeCluster struct {
	st       NodeState
	pods     []Pod
	calls    []string
	nodeErr  error
	evictErr map[string]error
}

func (f *fakeCluster) Node(context.Context) (NodeState, error) { return f.st, f.nodeErr }
func (f *fakeCluster) Cordon(_ context.Context, now time.Time) error {
	f.calls = append(f.calls, "cordon")
	if !f.st.Unschedulable {
		f.st.Unschedulable, f.st.Ours = true, true
	}
	f.st.Since = now
	return nil
}
func (f *fakeCluster) Uncordon(context.Context) error {
	f.calls = append(f.calls, "uncordon")
	f.st = NodeState{}
	return nil
}
func (f *fakeCluster) Pods(context.Context) ([]Pod, error) { return f.pods, nil }
func (f *fakeCluster) Evict(_ context.Context, p Pod) error {
	f.calls = append(f.calls, "evict "+p.Name)
	return f.evictErr[p.Name]
}

func nodeAgent(f *fakeCluster) Agent {
	return Agent{Cluster: f, LowerPriority: func(context.Context) error { f.calls = append(f.calls, "lower"); return nil }}
}

var t0 = time.Date(2026, 11, 28, 23, 0, 0, 0, time.UTC)

func nodeSettings(m LeaveMode) Settings {
	return Settings{Enabled: true, Hours: []Window{{0, 23*60 + 59}}, IdleThreshold: time.Minute, MaxWait: 5 * time.Minute, LeaveMode: m}
}

func countCalls(f *fakeCluster, prefix string) (n int) {
	for _, c := range f.calls {
		if strings.HasPrefix(c, prefix) {
			n++
		}
	}
	return n
}

func TestNodeGameSignalCordonsFirst(t *testing.T) {
	f := &fakeCluster{pods: []Pod{{"ci", "job-a", false}}}
	err := nodeAgent(f).Tick(context.Background(), nodeSettings(LeaveNow), Signals{Idle: time.Hour, OwnerInput: true}, t0)
	if err != nil || len(f.calls) == 0 || f.calls[0] != "cordon" {
		t.Fatalf("calls = %v, err %v, want cordon first", f.calls, err)
	}
}

func TestNodeFinishWaitsForMaxWait(t *testing.T) {
	f := &fakeCluster{pods: []Pod{{"ci", "job-a", false}}}
	a, s := nodeAgent(f), nodeSettings(LeaveFinish)
	for _, c := range []struct {
		at    time.Duration
		evict int
	}{{0, 0}, {4 * time.Minute, 0}, {5 * time.Minute, 1}} {
		if err := a.Reclaim(context.Background(), s, t0.Add(c.at)); err != nil {
			t.Fatal(err)
		}
		if got := countCalls(f, "evict"); got != c.evict {
			t.Fatalf("after %v: %d evictions, want %d (calls %v)", c.at, got, c.evict, f.calls)
		}
	}
	if countCalls(f, "cordon") != 1 || f.calls[0] != "cordon" {
		t.Fatalf("calls = %v", f.calls)
	}
}

func TestNodeSlowLowersPriorityAndNeverEvicts(t *testing.T) {
	f := &fakeCluster{pods: []Pod{{"ci", "job-a", false}}}
	if err := nodeAgent(f).Reclaim(context.Background(), nodeSettings(LeaveSlow), t0.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if want := []string{"cordon", "lower"}; !slices.Equal(f.calls, want) {
		t.Fatalf("calls = %v, want %v", f.calls, want)
	}
}

func TestNodeNowOrdersCordonLowerEvict(t *testing.T) {
	f := &fakeCluster{pods: []Pod{{"ci", "job-a", false}, {"kube-system", "ds-x", true}}}
	if err := nodeAgent(f).Reclaim(context.Background(), nodeSettings(LeaveNow), t0); err != nil {
		t.Fatal(err)
	}
	if want := []string{"cordon", "lower", "evict job-a"}; !slices.Equal(f.calls, want) {
		t.Fatalf("calls = %v, want %v", f.calls, want)
	}
}

func TestNodeLendUncordonsOnlyItsOwnCordon(t *testing.T) {
	idle := Signals{Idle: time.Hour}
	foreign := &fakeCluster{st: NodeState{Unschedulable: true}}
	if err := nodeAgent(foreign).Tick(context.Background(), nodeSettings(LeaveFinish), idle, t0); err != nil || countCalls(foreign, "uncordon") != 0 {
		t.Fatalf("foreign cordon: calls %v, err %v, want no uncordon", foreign.calls, err)
	}
	ours := &fakeCluster{st: NodeState{Unschedulable: true, Ours: true, Since: t0}}
	if err := nodeAgent(ours).Tick(context.Background(), nodeSettings(LeaveFinish), idle, t0); err != nil || countCalls(ours, "uncordon") != 1 {
		t.Fatalf("own cordon: calls %v, err %v, want one uncordon", ours.calls, err)
	}
	// A foreign cordon stays after a reclaim began on it and the owner leaves again.
	f := &fakeCluster{st: NodeState{Unschedulable: true}}
	a := nodeAgent(f)
	if err := a.Reclaim(context.Background(), nodeSettings(LeaveSlow), t0); err != nil {
		t.Fatal(err)
	}
	if err := a.Lend(context.Background()); err != nil || countCalls(f, "uncordon") != 0 {
		t.Fatalf("calls %v, err %v, want the foreign cordon kept", f.calls, err)
	}
}

func TestNodeRestartMidReclaimContinuesFromNodeState(t *testing.T) {
	f := &fakeCluster{st: NodeState{Unschedulable: true, Ours: true, Since: t0}, pods: []Pod{{"ci", "job-a", false}}}
	if err := nodeAgent(f).Reclaim(context.Background(), nodeSettings(LeaveFinish), t0.Add(6*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if want := []string{"evict job-a"}; !slices.Equal(f.calls, want) {
		t.Fatalf("calls = %v, want %v", f.calls, want)
	}
}

func TestNodeFailuresNeverUncordon(t *testing.T) {
	refused := errors.New("429 too many requests")
	f := &fakeCluster{pods: []Pod{{"ci", "a", false}, {"ci", "b", false}}, evictErr: map[string]error{"a": refused}}
	err := nodeAgent(f).Reclaim(context.Background(), nodeSettings(LeaveNow), t0)
	if !errors.Is(err, refused) || countCalls(f, "uncordon") != 0 || countCalls(f, "evict b") != 1 {
		t.Fatalf("calls = %v, err %v, want the refusal returned, b still tried, no uncordon", f.calls, err)
	}
	down := &fakeCluster{nodeErr: errors.New("api unreachable")}
	if err := nodeAgent(down).Reclaim(context.Background(), nodeSettings(LeaveNow), t0); err == nil || len(down.calls) != 0 {
		t.Fatalf("calls = %v, err %v, want the error and no action", down.calls, err)
	}
}

func TestNodeUnknownLeaveModeNeverEvicts(t *testing.T) {
	f := &fakeCluster{pods: []Pod{{"ci", "job-a", false}}}
	if err := nodeAgent(f).Reclaim(context.Background(), Settings{}, t0.Add(24*time.Hour)); err != nil || countCalls(f, "evict") != 0 {
		t.Fatalf("calls = %v, err %v, want no eviction", f.calls, err)
	}
}
