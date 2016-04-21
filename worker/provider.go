// mirror provider is the wrapper of mirror jobs

package worker

// a mirrorProvider instance
type mirrorProvider interface {
	// run mirror job
	Run()
	// terminate mirror job
	Terminate()
	// get context
	Context()
}
