package refcount

import (
	"errors"
	"testing"
)

type refTester struct {
	released bool
}

func (r *refTester) release() error {
	r.released = true
	return nil
}
func TestRelease(t *testing.T) {
	var counter ReferenceCountable
	tester := &refTester{}
	instance, release, err := counter.Get(tester.create)
	if err != nil {
		t.Fatal("Error in Get")
	}
	if instance != tester {
		t.Fatal("unexpected instance")
	}
	instance, _, _ = counter.Get(tester.create)

	// make sure thre create worked.
	if instance != tester {
		t.Fatal("unexpected instance")
	}

	// make sure the object is not yet released, only ref count decremented
	if err := release(); err != nil {
		t.Fatal("Error in release")
	}
	if tester.released {
		t.Fatal("Released before time")
	}

	// make sure object is now released
	if err := release(); err != nil {
		t.Fatal("Error in release")
	}
	if !tester.released {
		t.Fatal("instance wasn't released")
	}

	if err := release(); err == nil {
		t.Fatal("should return error on releasing when refcount is zero")
	}
}

func TestReleaseOnError(t *testing.T) {
	var counter ReferenceCountable
	tester := &refTester{}
	instance, release, err := counter.Get(tester.createWithError)
	if err == nil {
		t.Fatal("should return error")
	}
	if release != nil {
		t.Fatal("release func should be nil in case creation failed")
	}
	if instance != nil {
		t.Fatal("instance should be nil")
	}
	if !tester.released {
		t.Fatal("instance wasn't released")
	}
}

func (r *refTester) create() (interface{}, ReleaseFunc, error) {
	return r, r.release, nil
}

func (r *refTester) createWithError() (interface{}, ReleaseFunc, error) {
	return nil, r.release, errors.New("create error")
}
