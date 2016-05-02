package worker

// schedule queue for jobs

import (
	"sync"
	"time"

	"github.com/ryszard/goskiplist/skiplist"
)

type scheduleQueue struct {
	sync.Mutex
	list *skiplist.SkipList
	jobs map[string]bool
}

func timeLessThan(l, r interface{}) bool {
	tl := l.(time.Time)
	tr := r.(time.Time)
	return tl.Before(tr)
}

func newScheduleQueue() *scheduleQueue {
	queue := new(scheduleQueue)
	queue.list = skiplist.NewCustomMap(timeLessThan)
	queue.jobs = make(map[string]bool)
	return queue
}

func (q *scheduleQueue) AddJob(schedTime time.Time, job *mirrorJob) {
	q.Lock()
	defer q.Unlock()
	if _, ok := q.jobs[job.Name()]; ok {
		logger.Warningf("Job %s already scheduled, removing the existing one", job.Name())
		q.unsafeRemove(job.Name())
	}
	q.jobs[job.Name()] = true
	q.list.Set(schedTime, job)
	logger.Debugf("Added job %s @ %v", job.Name(), schedTime)
}

// pop out the first job if it's time to run it
func (q *scheduleQueue) Pop() *mirrorJob {
	q.Lock()
	defer q.Unlock()

	first := q.list.SeekToFirst()
	if first == nil {
		return nil
	}
	defer first.Close()

	t := first.Key().(time.Time)
	if t.Before(time.Now()) {
		job := first.Value().(*mirrorJob)
		q.list.Delete(first.Key())
		delete(q.jobs, job.Name())
		logger.Debug("Popped out job %s @%v", job.Name(), t)
		return job
	}
	return nil
}

// remove job
func (q *scheduleQueue) Remove(name string) bool {
	q.Lock()
	defer q.Unlock()
	return q.unsafeRemove(name)
}

// remove job
func (q *scheduleQueue) unsafeRemove(name string) bool {
	cur := q.list.Iterator()
	defer cur.Close()

	for cur.Next() {
		cj := cur.Value().(*mirrorJob)
		if cj.Name() == name {
			q.list.Delete(cur.Key())
			delete(q.jobs, name)
			return true
		}
	}
	return false
}
