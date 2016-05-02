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
}

func timeLessThan(l, r interface{}) bool {
	tl := l.(time.Time)
	tr := r.(time.Time)
	return tl.Before(tr)
}

func newScheduleQueue() *scheduleQueue {
	queue := new(scheduleQueue)
	queue.list = skiplist.NewCustomMap(timeLessThan)
	return queue
}

func (q *scheduleQueue) AddJob(schedTime time.Time, job *mirrorJob) {
	q.Lock()
	defer q.Unlock()
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
	// logger.Debug("First job should run @%v", t)
	if t.Before(time.Now()) {
		job := first.Value().(*mirrorJob)
		q.list.Delete(first.Key())
		return job
	}
	return nil
}

// remove job
func (q *scheduleQueue) Remove(name string) bool {
	q.Lock()
	defer q.Unlock()

	cur := q.list.Iterator()
	defer cur.Close()

	for cur.Next() {
		cj := cur.Value().(*mirrorJob)
		if cj.Name() == name {
			q.list.Delete(cur.Key())
			return true
		}
	}
	return false
}
