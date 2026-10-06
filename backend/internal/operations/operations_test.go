package operations

import (
	"errors"
	"testing"
	"time"
)

func TestTrackerCountsRecentServerFailuresAndWorkerHealth(t *testing.T) {
	tracker := NewTracker()
	now := time.Now().UTC()
	tracker.Request(500, now.Add(-6*time.Minute))
	tracker.Request(503, now)
	tracker.Register("email", now.Add(-time.Minute))
	tracker.Begin("email", now.Add(-time.Second))
	tracker.Finish("email", now, errors.New("smtp down"))
	tracker.Begin("email", now.Add(time.Second))
	tracker.Finish("email", now.Add(2*time.Second), errors.New("smtp down"))
	tracker.Begin("email", now.Add(3*time.Second))
	tracker.Finish("email", now.Add(4*time.Second), errors.New("smtp down"))

	count, workers, _ := tracker.snapshot()
	if count != 1 {
		t.Fatalf("recent API failures = %d, want 1", count)
	}
	if len(workers) != 1 || workers[0].Name != "email" || workers[0].ConsecutiveFailures != 3 || workers[0].Running {
		t.Fatalf("worker snapshot = %#v", workers)
	}
}
