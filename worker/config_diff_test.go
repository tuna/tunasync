package worker

import (
	"sort"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestConfigDiff(t *testing.T) {
	Convey("When old and new configs are equal", t, func() {
		oldList := []mirrorConfig{
			{Name: "debian"},
			{Name: "debian-security"},
			{Name: "fedora"},
			{Name: "archlinux"},
			{Name: "AOSP"},
			{Name: "ubuntu"},
		}
		newList := make([]mirrorConfig, len(oldList))
		copy(newList, oldList)

		difference := diffMirrorConfig(oldList, newList)
		So(len(difference), ShouldEqual, 0)
	})
	Convey("When old config is empty", t, func() {
		newList := []mirrorConfig{
			{Name: "debian"},
			{Name: "debian-security"},
			{Name: "fedora"},
			{Name: "archlinux"},
			{Name: "AOSP"},
			{Name: "ubuntu"},
		}
		oldList := make([]mirrorConfig, 0)

		difference := diffMirrorConfig(oldList, newList)
		So(len(difference), ShouldEqual, len(newList))
	})
	Convey("When new config is empty", t, func() {
		oldList := []mirrorConfig{
			{Name: "debian"},
			{Name: "debian-security"},
			{Name: "fedora"},
			{Name: "archlinux"},
			{Name: "AOSP"},
			{Name: "ubuntu"},
		}
		newList := make([]mirrorConfig, 0)

		difference := diffMirrorConfig(oldList, newList)
		So(len(difference), ShouldEqual, len(oldList))
	})
	Convey("When giving two config lists with different names", t, func() {
		oldList := []mirrorConfig{
			{Name: "debian"},
			{Name: "debian-security"},
			{Name: "fedora"},
			{Name: "archlinux"},
			{Name: "AOSP", Env: map[string]string{"REPO": "/usr/bin/repo"}},
			{Name: "ubuntu"},
		}
		newList := []mirrorConfig{
			{Name: "debian"},
			{Name: "debian-cd"},
			{Name: "archlinuxcn"},
			{Name: "AOSP", Env: map[string]string{"REPO": "/usr/local/bin/aosp-repo"}},
			{Name: "ubuntu-ports"},
		}

		difference := diffMirrorConfig(oldList, newList)

		sort.Sort(sortableMirrorList(oldList))
		emptyList := []mirrorConfig{}

		for _, o := range oldList {
			keep := true
			for _, op := range difference {
				if (op.diffOp == diffDelete || op.diffOp == diffModify) &&
					op.mirCfg.Name == o.Name {

					keep = false
					break
				}
			}
			if keep {
				emptyList = append(emptyList, o)
			}
		}

		for _, op := range difference {
			if op.diffOp == diffAdd || op.diffOp == diffModify {
				emptyList = append(emptyList, op.mirCfg)
			}
		}
		sort.Sort(sortableMirrorList(emptyList))
		sort.Sort(sortableMirrorList(newList))
		So(emptyList, ShouldResemble, newList)

	})
}
