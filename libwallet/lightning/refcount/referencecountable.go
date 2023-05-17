package refcount

import (
	"errors"
	"sync"
)

// CreateFunc is the function that creates the actual struct
type CreateFunc func() (s interface{}, cleanup ReleaseFunc, err error)

// ReleaseFunc should be called by the client when he stopps to use the service
type ReleaseFunc func() error

// ReferenceCountable is a wrapper that handles the reference counting of an instance
type ReferenceCountable struct {
	mu       sync.Mutex
	instance interface{}
	refCount int32
	create   CreateFunc
	release  ReleaseFunc
}

// Get receives a factory function and returns the instance if exists or creates a new one.
// In addition it returns ReleaseFunc to be used when the owner no longer needs the instance.
func (r *ReferenceCountable) Get(create CreateFunc) (sr interface{}, rf ReleaseFunc, err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.refCount == 0 {
		r.instance, r.release, err = create()
		if err != nil {
			if r.release != nil {
				r.release()
			}
			return nil, nil, err
		}
	}
	r.refCount++
	return r.instance, r.Release, nil
}

// Release should be called when the owner of the instance no longer uses it and would like
// to release resources.
func (r *ReferenceCountable) Release() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.refCount == 0 {
		return errors.New("can't release, refcount is zero")
	}

	r.refCount--
	if r.refCount == 0 {
		return r.release()
	}
	return nil
}
