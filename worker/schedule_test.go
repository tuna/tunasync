package worker

import (
	"testing"
	"time"

	. "github.com/smartystreets/goconvey/convey"
)

func TestSchedule(t *testing.T) {

	Convey("MirrorJobSchedule should work", t, func(ctx C) {
		schedule := newScheduleQueue()

		Convey("When poping on empty schedule", func() {
			job := schedule.Pop()
			So(job, ShouldBeNil)
		})

		Convey("When adding some jobs", func() {
			c := cmdConfig{
				name: "schedule_test",
			}
			provider, _ := newCmdProvider(c)
			job := newMirrorJob(provider)
			sched := time.Now().Add(1 * time.Second)

			schedule.AddJob(sched, job)
			So(schedule.Pop(), ShouldBeNil)
			time.Sleep(1200 * time.Millisecond)
			So(schedule.Pop(), ShouldEqual, job)

		})
		Convey("When adding one job twice", func() {
			c := cmdConfig{
				name: "schedule_test",
			}
			provider, _ := newCmdProvider(c)
			job := newMirrorJob(provider)
			sched := time.Now().Add(1 * time.Second)

			schedule.AddJob(sched, job)
			schedule.AddJob(sched.Add(1*time.Second), job)

			So(schedule.Pop(), ShouldBeNil)
			time.Sleep(1200 * time.Millisecond)
			So(schedule.Pop(), ShouldBeNil)
			time.Sleep(1200 * time.Millisecond)
			So(schedule.Pop(), ShouldEqual, job)

		})
		Convey("When removing jobs", func() {
			c := cmdConfig{
				name: "schedule_test",
			}
			provider, _ := newCmdProvider(c)
			job := newMirrorJob(provider)
			sched := time.Now().Add(1 * time.Second)

			schedule.AddJob(sched, job)
			So(schedule.Remove("something"), ShouldBeFalse)
			So(schedule.Remove("schedule_test"), ShouldBeTrue)
			time.Sleep(1200 * time.Millisecond)
			So(schedule.Pop(), ShouldBeNil)
		})

	})
}
