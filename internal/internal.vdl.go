// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file was auto-generated by the vanadium vdl tool.
// Package: internal

package internal

import (
	"time"
	"v.io/v23/vdl"
	vdltime "v.io/v23/vdlroot/time"
)

var _ = __VDLInit() // Must be first; see __VDLInit comments for details.

//////////////////////////////////////////////////
// Type definitions

type PeerInfo struct {
	Sent        bool
	FatalError  bool
	NextAttempt time.Time
	NumAttempts int32
}

func (PeerInfo) __VDLReflect(struct {
	Name string `vdl:"messenger/internal.PeerInfo"`
}) {
}

func (x PeerInfo) VDLIsZero() bool {
	if x.Sent {
		return false
	}
	if x.FatalError {
		return false
	}
	if !x.NextAttempt.IsZero() {
		return false
	}
	if x.NumAttempts != 0 {
		return false
	}
	return true
}

func (x PeerInfo) VDLWrite(enc vdl.Encoder) error {
	if err := enc.StartValue(__VDLType_struct_1); err != nil {
		return err
	}
	if x.Sent {
		if err := enc.NextFieldValueBool(0, vdl.BoolType, x.Sent); err != nil {
			return err
		}
	}
	if x.FatalError {
		if err := enc.NextFieldValueBool(1, vdl.BoolType, x.FatalError); err != nil {
			return err
		}
	}
	if !x.NextAttempt.IsZero() {
		if err := enc.NextField(2); err != nil {
			return err
		}
		var wire vdltime.Time
		if err := vdltime.TimeFromNative(&wire, x.NextAttempt); err != nil {
			return err
		}
		if err := wire.VDLWrite(enc); err != nil {
			return err
		}
	}
	if x.NumAttempts != 0 {
		if err := enc.NextFieldValueInt(3, vdl.Int32Type, int64(x.NumAttempts)); err != nil {
			return err
		}
	}
	if err := enc.NextField(-1); err != nil {
		return err
	}
	return enc.FinishValue()
}

func (x *PeerInfo) VDLRead(dec vdl.Decoder) error {
	*x = PeerInfo{}
	if err := dec.StartValue(__VDLType_struct_1); err != nil {
		return err
	}
	decType := dec.Type()
	for {
		index, err := dec.NextField()
		switch {
		case err != nil:
			return err
		case index == -1:
			return dec.FinishValue()
		}
		if decType != __VDLType_struct_1 {
			index = __VDLType_struct_1.FieldIndexByName(decType.Field(index).Name)
			if index == -1 {
				if err := dec.SkipValue(); err != nil {
					return err
				}
				continue
			}
		}
		switch index {
		case 0:
			switch value, err := dec.ReadValueBool(); {
			case err != nil:
				return err
			default:
				x.Sent = value
			}
		case 1:
			switch value, err := dec.ReadValueBool(); {
			case err != nil:
				return err
			default:
				x.FatalError = value
			}
		case 2:
			var wire vdltime.Time
			if err := wire.VDLRead(dec); err != nil {
				return err
			}
			if err := vdltime.TimeToNative(wire, &x.NextAttempt); err != nil {
				return err
			}
		case 3:
			switch value, err := dec.ReadValueInt(32); {
			case err != nil:
				return err
			default:
				x.NumAttempts = int32(value)
			}
		}
	}
}

// Hold type definitions in package-level variables, for better performance.
var (
	__VDLType_struct_1 *vdl.Type
	__VDLType_struct_2 *vdl.Type
)

var __VDLInitCalled bool

// __VDLInit performs vdl initialization.  It is safe to call multiple times.
// If you have an init ordering issue, just insert the following line verbatim
// into your source files in this package, right after the "package foo" clause:
//
//    var _ = __VDLInit()
//
// The purpose of this function is to ensure that vdl initialization occurs in
// the right order, and very early in the init sequence.  In particular, vdl
// registration and package variable initialization needs to occur before
// functions like vdl.TypeOf will work properly.
//
// This function returns a dummy value, so that it can be used to initialize the
// first var in the file, to take advantage of Go's defined init order.
func __VDLInit() struct{} {
	if __VDLInitCalled {
		return struct{}{}
	}
	__VDLInitCalled = true

	// Register types.
	vdl.Register((*PeerInfo)(nil))

	// Initialize type definitions.
	__VDLType_struct_1 = vdl.TypeOf((*PeerInfo)(nil)).Elem()
	__VDLType_struct_2 = vdl.TypeOf((*vdltime.Time)(nil)).Elem()

	return struct{}{}
}
