package lock

import (
	"sync"
	"testing"
)

func TestLocalGetLockReturnsOneMutexAndSerializesCallers(t *testing.T) {
	const workers = 32
	local := NewLocal()
	mutexes := make(chan *sync.Mutex, workers)
	var wait sync.WaitGroup
	wait.Add(workers)
	counter := 0

	for range workers {
		go func() {
			defer wait.Done()
			mutex := local.GetLock("key")
			mutexes <- mutex
			mutex.Lock()
			counter++
			mutex.Unlock()
		}()
	}
	wait.Wait()
	close(mutexes)

	var first *sync.Mutex
	for mutex := range mutexes {
		if first == nil {
			first = mutex
			continue
		}
		if mutex != first {
			t.Fatal("GetLock returned different mutexes for the same key")
		}
	}
	if counter != workers {
		t.Fatalf("serialized counter = %d, want %d", counter, workers)
	}
}

func TestLocalLockSerializesCallers(t *testing.T) {
	const workers = 32
	local := NewLocal()
	var wait sync.WaitGroup
	wait.Add(workers)
	counter := 0

	for range workers {
		go func() {
			defer wait.Done()
			local.Lock("key")
			counter++
			local.UnLock("key")
		}()
	}
	wait.Wait()

	if counter != workers {
		t.Fatalf("serialized counter = %d, want %d", counter, workers)
	}
}

func TestSyncMapLoadOrStoreSelectsOneValue(t *testing.T) {
	const workers = 32
	var values sync.Map
	results := make(chan int, workers)
	created := make(chan bool, workers)
	var wait sync.WaitGroup
	wait.Add(workers)

	for value := 1; value <= workers; value++ {
		go func(candidate int) {
			defer wait.Done()
			actual, loaded := values.LoadOrStore("key", candidate)
			results <- actual.(int)
			created <- !loaded
		}(value)
	}
	wait.Wait()
	close(results)
	close(created)

	expected := 0
	for result := range results {
		if expected == 0 {
			expected = result
		}
		if result != expected {
			t.Fatalf("LoadOrStore returned %d after selecting %d", result, expected)
		}
	}
	creators := 0
	for wasCreated := range created {
		if wasCreated {
			creators++
		}
	}
	if creators != 1 {
		t.Fatalf("LoadOrStore creators = %d, want 1", creators)
	}
}
