package worker

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestContext(t *testing.T) {
	Convey("Context should work", t, func() {

		ctx := NewContext()
		So(ctx, ShouldNotBeNil)
		So(ctx.parent, ShouldBeNil)

		ctx.Set("logdir1", "logdir_value_1")
		ctx.Set("logdir2", "logdir_value_2")
		logdir, ok := ctx.Get("logdir1")
		So(ok, ShouldBeTrue)
		So(logdir, ShouldEqual, "logdir_value_1")

		Convey("When entering a new context", func() {
			ctx = ctx.Enter()
			logdir, ok = ctx.Get("logdir1")
			So(ok, ShouldBeTrue)
			So(logdir, ShouldEqual, "logdir_value_1")

			ctx.Set("logdir1", "new_value_1")

			logdir, ok = ctx.Get("logdir1")
			So(ok, ShouldBeTrue)
			So(logdir, ShouldEqual, "new_value_1")

			logdir, ok = ctx.Get("logdir2")
			So(ok, ShouldBeTrue)
			So(logdir, ShouldEqual, "logdir_value_2")

			Convey("When accesing invalid key", func() {
				logdir, ok = ctx.Get("invalid_key")
				So(ok, ShouldBeFalse)
				So(logdir, ShouldBeNil)
			})

			Convey("When exiting the new context", func() {
				ctx, err := ctx.Exit()
				So(err, ShouldBeNil)

				logdir, ok = ctx.Get("logdir1")
				So(ok, ShouldBeTrue)
				So(logdir, ShouldEqual, "logdir_value_1")

				logdir, ok = ctx.Get("logdir2")
				So(ok, ShouldBeTrue)
				So(logdir, ShouldEqual, "logdir_value_2")

				Convey("When exiting from top bottom context", func() {
					ctx, err := ctx.Exit()
					So(err, ShouldNotBeNil)
					So(ctx, ShouldBeNil)
				})
			})
		})
	})
}
