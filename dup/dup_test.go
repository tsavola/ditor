package dup_test

import (
	"encoding/gob"
	"math"
	"reflect"
	"testing"

	dup "."
)

type object struct {
	Head [4]byte
	Data []byte
}

type state struct {
	X int
	Y []string
	Z struct {
		A bool
		B float64
		C struct{}
		D *object
		E *object
		F interface{}
		G interface{}
	}
}

func init() {
	gob.Register(new(object))
}

func Test(t *testing.T) {
	s1 := new(state)

	d1 := dup.Duper{
		State:   s1,
		SockDir: ".",
		Name:    "dup",
	}

	if err := d1.Export(); err != nil {
		t.Fatal(err)
	}
	defer d1.Close()

	if d1.Name != "dup" {
		t.Fatalf("unexpected d1.Name: %s", d1.Name)
	}

	d1.Lock()

	s1.X = 42
	s1.Y = []string{"foo", "bar", "baz"}
	s1.Z.A = true
	s1.Z.B = math.Pi
	s1.Z.D = &object{
		Head: [4]byte{1, 2, 3, 4},
		Data: []byte("hello world"),
	}
	s1.Z.F = s1.Z.D

	d1.Unlock()

	s2 := new(state)

	d2 := dup.Duper{
		State:   s2,
		SockDir: ".",
	}

	oldName, err := d2.Import("dup")
	if err != nil {
		t.Fatal(err)
	}

	if oldName != "" {
		t.Fatalf("unexpected oldName: %s", oldName)
	}

	if d2.Name != "dup" {
		t.Fatalf("unexpected d2.Name: %s", d2.Name)
	}

	if !reflect.DeepEqual(s1, s2) {
		t.Fatalf("not equal: %#v", s2)
	}
}
