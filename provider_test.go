// Copyright 2018 GRAIL, Inc. All rights reserved.
// Use of this source code is governed by the Apache 2.0
// license that can be found in the LICENSE file.
package infra

import (
	"reflect"
	"testing"
)

func TestProvider(t *testing.T) {
	type x int

	typ := reflect.TypeOf(x(0))
	p := provider{"", typ}
	if err := p.Typecheck(); err != nil {
		t.Fatal(err)
	}
	inst := p.New(Config{}, "")
	if got, want := inst.val.Type(), typ; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
	if got, want := inst.val.Int(), int64(0); got != want {
		t.Errorf("got %v, want %v", got, want)
	}

	typ = reflect.TypeOf(new(x))
	p = provider{"", typ}
	if err := p.Typecheck(); err != nil {
		t.Fatal(err)
	}
	inst = p.New(Config{}, "")
	if inst.val.Pointer() == uintptr(0) {
		t.Error("instantiated nil pointer")
	}
	if got, want := inst.val.Elem().Int(), int64(0); got != want {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestProviderValidName(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Unexpected panic: %v", r)
		}
	}()
	Register("test", new(Schema))
	Register("ec2cluster", new(Schema))
}

func TestProviderInvalidNameSpecialCharacters(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("Expected panic")
		}
	}()
	Register("232*772", new(Schema))
}

func TestProviderInvalidNameCaps(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("Expected panic")
		}
	}()
	Register("CAPS", new(Schema))
}
