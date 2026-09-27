// Package pipeline schedules kitchen DAGs without I/O or heap scratch storage.
package pipeline

import "errors"

const MaxSteps = 64
const (
	Hands       uint8 = 1
	Board       uint8 = 2
	MainStove   uint8 = 4
	SecondStove uint8 = 8
	RiceCooker  uint8 = 16
	Oven        uint8 = 32
)

type Step struct {
	Duration     int
	Dependencies uint64
	Resources    uint8
}
type Slot struct {
	Start    int `json:"start"`
	End      int `json:"end"`
	Earliest int `json:"earliest"`
	Latest   int `json:"latest"`
}
type Result struct {
	Slots                               [MaxSteps]Slot
	Count, Duration, CriticalLowerBound int
	DeadlineMet                         bool
}

// Schedule uses critical-tail priority list scheduling. Earliest is a DAG-only
// bound; Latest includes the selected resource order. No global optimality claim.
func Schedule(steps []Step, deadline int) (Result, error) {
	var out Result
	n := len(steps)
	if n == 0 || n > 64 || deadline < 0 || deadline > 86400 {
		return out, errors.New("invalid pipeline bounds")
	}
	var mask uint64 = ^uint64(0)
	if n < 64 {
		mask = (uint64(1) << n) - 1
	}
	var order, tails [64]int
	var done uint64
	for i, s := range steps {
		if s.Duration <= 0 || s.Duration > 28800 || s.Dependencies & ^mask != 0 || s.Dependencies&(uint64(1)<<i) != 0 || s.Resources == 0 || s.Resources & ^uint8(63) != 0 {
			return out, errors.New("invalid step")
		}
	}
	for pos := 0; pos < n; pos++ {
		found := -1
		for i, s := range steps {
			if done&(uint64(1)<<i) == 0 && s.Dependencies & ^done == 0 {
				found = i
				break
			}
		}
		if found < 0 {
			return out, errors.New("cyclic dependencies")
		}
		order[pos] = found
		done |= uint64(1) << found
		for j := 0; j < n; j++ {
			if steps[found].Dependencies&(uint64(1)<<j) != 0 {
				out.Slots[found].Earliest = max(out.Slots[found].Earliest, out.Slots[j].Earliest+steps[j].Duration)
			}
		}
		out.CriticalLowerBound = max(out.CriticalLowerBound, out.Slots[found].Earliest+steps[found].Duration)
	}
	for p := n - 1; p >= 0; p-- {
		i := order[p]
		tails[i] = steps[i].Duration
		for j, s := range steps {
			if s.Dependencies&(uint64(1)<<i) != 0 {
				tails[i] = max(tails[i], steps[i].Duration+tails[j])
			}
		}
	}
	var placed uint64
	var resourceEnd [6]int
	var previous [6]int
	for i := range previous {
		previous[i] = -1
	}
	var predecessors [64]uint64
	for pos := 0; pos < n; pos++ {
		best, start := -1, 0
		for i, s := range steps {
			if placed&(uint64(1)<<i) != 0 || s.Dependencies & ^placed != 0 {
				continue
			}
			at := 0
			for j := 0; j < n; j++ {
				if s.Dependencies&(uint64(1)<<j) != 0 {
					at = max(at, out.Slots[j].End)
				}
			}
			for r := 0; r < 6; r++ {
				if s.Resources&(1<<r) != 0 {
					at = max(at, resourceEnd[r])
				}
			}
			if best < 0 || at < start || (at == start && tails[i] > tails[best]) {
				best, start = i, at
			}
		}
		order[pos] = best
		s := steps[best]
		out.Slots[best].Start = start
		out.Slots[best].End = start + s.Duration
		predecessors[best] = s.Dependencies
		for r := 0; r < 6; r++ {
			if s.Resources&(1<<r) != 0 {
				if previous[r] >= 0 {
					predecessors[best] |= uint64(1) << previous[r]
				}
				previous[r] = best
				resourceEnd[r] = out.Slots[best].End
			}
		}
		placed |= uint64(1) << best
		out.Duration = max(out.Duration, out.Slots[best].End)
	}
	finish := max(deadline, out.Duration)
	for i := range steps {
		out.Slots[i].Latest = finish - steps[i].Duration
	}
	for p := n - 1; p >= 0; p-- {
		i := order[p]
		for j := 0; j < n; j++ {
			if predecessors[i]&(uint64(1)<<j) != 0 {
				out.Slots[j].Latest = min(out.Slots[j].Latest, out.Slots[i].Latest-steps[j].Duration)
			}
		}
	}
	out.Count = n
	out.DeadlineMet = deadline >= out.Duration
	return out, nil
}
