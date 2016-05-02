package worker

import (
	"fmt"
	"reflect"
	"sort"
)

// Find difference of mirror config, this is important for hot reloading config file
// NOTICE: only the [[mirrors]] section is supported

// make []mirrorConfig sortable
type sortableMirrorList []mirrorConfig

func (l sortableMirrorList) Len() int           { return len(l) }
func (l sortableMirrorList) Swap(i, j int)      { l[i], l[j] = l[j], l[i] }
func (l sortableMirrorList) Less(i, j int) bool { return l[i].Name < l[j].Name }

const (
	diffDelete uint8 = iota
	diffAdd
	diffModify
)

// a unit of mirror config difference
type mirrorCfgTrans struct {
	diffOp uint8
	mirCfg mirrorConfig
}

func (t mirrorCfgTrans) String() string {
	var op string
	if t.diffOp == diffDelete {
		op = "Del"
	} else {
		op = "Add"
	}
	return fmt.Sprintf("{%s, %s}", op, t.mirCfg.Name)
}

// diffMirrorConfig finds the difference between the oldList and the newList
// it returns a series of operations that if these operations are applied to
// oldList, a newList equavuilance can be obtained.
func diffMirrorConfig(oldList, newList []mirrorConfig) []mirrorCfgTrans {
	operations := []mirrorCfgTrans{}

	oList := make([]mirrorConfig, len(oldList))
	nList := make([]mirrorConfig, len(newList))
	copy(oList, oldList)
	copy(nList, newList)

	// first ensure oldList and newList are sorted
	sort.Sort(sortableMirrorList(oList))
	sort.Sort(sortableMirrorList(nList))

	// insert a tail node to both lists
	// as the maximum node
	lastOld, lastNew := oList[len(oList)-1], nList[len(nList)-1]
	maxName := lastOld.Name
	if lastNew.Name > lastOld.Name {
		maxName = lastNew.Name
	}
	Nil := mirrorConfig{Name: "~" + maxName}
	if Nil.Name <= maxName {
		panic("Nil.Name should be larger than maxName")
	}
	oList, nList = append(oList, Nil), append(nList, Nil)

	// iterate over both lists to find the difference
	for i, j := 0, 0; i < len(oList) && j < len(nList); {
		o, n := oList[i], nList[j]
		if n.Name < o.Name {
			operations = append(operations, mirrorCfgTrans{diffAdd, n})
			j++
		} else if o.Name < n.Name {
			operations = append(operations, mirrorCfgTrans{diffDelete, o})
			i++
		} else {
			if !reflect.DeepEqual(o, n) {
				operations = append(operations, mirrorCfgTrans{diffModify, n})
			}
			i++
			j++
		}
	}

	return operations
}
