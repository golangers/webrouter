package webrouter

import (
	"net/http"
	"sort"
)

type releasor struct {
	name   string
	leader string
	lag    int
	h      http.Handler
}

type releasors []releasor

func (rsrs releasors) Len() int {
	return len(rsrs)
}

func (rsrs releasors) Swap(i, j int) {
	rsrs[i], rsrs[j] = rsrs[j], rsrs[i]
}

type byReleasorLag struct {
	releasors
}

func (brp byReleasorLag) Less(i, j int) bool {
	return brp.releasors[i].lag < brp.releasors[j].lag
}

type byReleasorLeader struct {
	releasors
}

func (brfn byReleasorLeader) Less(i, j int) bool {
	lrs := brfn.releasors.Len()
	in := i + 1
	if in == lrs {
		in = i
	}

	return brfn.releasors[i].leader != "" && brfn.releasors[i].leader != brfn.releasors[in].name
}

func hasSameReleasor(rsrs []releasor, name string) bool {
	for _, rsr := range rsrs {
		if rsr.name == name {
			return true
		}
	}

	return false
}

//1. lag order: the higher the value the more after
//2. move leader to leader's before
func sortReleasor(rsrs []releasor) []releasor {
	sort.Sort(byReleasorLag{rsrs})
	sort.Sort(byReleasorLeader{rsrs})

	return rsrs
}
