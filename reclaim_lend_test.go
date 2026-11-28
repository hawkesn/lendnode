package moonlighter

import (
	"context"
	"testing"
)

// A node another actor already cordoned, with no reclaim start recorded,
// still gets cordoned by the agent so a finish wait counts from now on.
func TestReclaimCordonsForeignNodeWithoutSince(t *testing.T) {
	f := &fakeCluster{st: NodeState{Unschedulable: true}, pods: []Pod{{"ci", "job-a", false}}}
	if err := nodeAgent(f).Reclaim(context.Background(), nodeSettings(LeaveNow), t0); err != nil {
		t.Fatal(err)
	}
	if len(f.calls) == 0 || f.calls[0] != "cordon" {
		t.Fatalf("calls = %v, want cordon first", f.calls)
	}
}

// An own cordon is uncordoned even before a start was recorded on it.
func TestLendUncordonsOwnCordonWithoutSince(t *testing.T) {
	f := &fakeCluster{st: NodeState{Unschedulable: true, Ours: true}}
	if err := nodeAgent(f).Lend(context.Background()); err != nil || len(f.calls) != 1 || f.calls[0] != "uncordon" {
		t.Fatalf("calls = %v, err %v, want one uncordon", f.calls, err)
	}
}
