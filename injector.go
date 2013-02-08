package webrouter

import (
	"net/http"
	"sort"
)

type injector struct {
	name     string
	follower string
	priority int
	h        http.Handler
}

type injectors []injector

func (itrs injectors) Len() int {
	return len(itrs)
}

func (itrs injectors) Swap(i, j int) {
	itrs[i], itrs[j] = itrs[j], itrs[i]
}

type byInjectorPriority struct {
	injectors
}

func (bip byInjectorPriority) Less(i, j int) bool {
	return bip.injectors[i].priority > bip.injectors[j].priority
}

type byInjectorFollower struct {
	injectors
}

func (bifn byInjectorFollower) Less(i, j int) bool {
	jp := j - 1
	if jp < 0 {
		jp = j
	}

	return bifn.injectors[j].follower != "" && bifn.injectors[j].follower != bifn.injectors[jp].name
}

func hasSameInjector(itrs []injector, name string) bool {
	for _, itr := range itrs {
		if itr.name == name {
			return true
		}
	}

	return false
}

//1. priority order: the higher the value the more before
//2. move to follower's after
func sortInjector(itrs []injector) []injector {
	sort.Sort(byInjectorPriority{itrs})
	sort.Sort(byInjectorFollower{itrs})

	return itrs
}
