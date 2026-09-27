package pipeline

import "testing"

func TestPassiveCookingReleasesHands(t *testing.T) {
	steps := []Step{{60, 0, Hands | Board}, {2100, 1, MainStove}, {120, 0, Hands | Board}, {180, 4, Hands | SecondStove}}
	result, err := Schedule(steps, 2400)
	if err != nil {
		t.Fatal(err)
	}
	if result.Duration != 2160 || result.Slots[2].Start != 60 || result.Slots[3].End > result.Slots[1].End {
		t.Fatalf("not parallel: %+v", result)
	}
	for i, s := range steps {
		for j := 0; j < i; j++ {
			a, b := result.Slots[i], result.Slots[j]
			if s.Resources&steps[j].Resources != 0 && a.Start < b.End && b.Start < a.End {
				t.Fatal("resource overlap")
			}
		}
		if result.Slots[i].Latest < result.Slots[i].Start {
			t.Fatal("invalid latest start")
		}
	}
	short, _ := Schedule(steps, 100)
	if short.DeadlineMet {
		t.Fatal("impossible deadline accepted")
	}
}
func TestRejectInvalidDAG(t *testing.T) {
	for _, steps := range [][]Step{{{1, 2, Hands}, {1, 1, Hands}}, {{1, 1, Hands}}, {{0, 0, Hands}}, {{1, 0, 128}}} {
		if _, err := Schedule(steps, 10); err == nil {
			t.Fatal("invalid DAG accepted")
		}
	}
}
func TestKernelZeroAlloc(t *testing.T) {
	steps := []Step{{60, 0, Hands | Board}, {600, 1, MainStove}, {120, 0, Hands | Board}}
	if alloc := testing.AllocsPerRun(100, func() { _, _ = Schedule(steps, 1000) }); alloc != 0 {
		t.Fatalf("allocations %v", alloc)
	}
}
func BenchmarkSchedule(b *testing.B) {
	steps := []Step{{60, 0, Hands | Board}, {2100, 1, MainStove}, {120, 0, Hands | Board}, {180, 4, Hands | SecondStove}}
	b.ReportAllocs()
	for b.Loop() {
		_, _ = Schedule(steps, 2400)
	}
}
