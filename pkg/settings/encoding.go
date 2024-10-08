// Copyright 2022 The Cockroach Authors.
//
// Use of this software is governed by the CockroachDB Software License
// included in the /LICENSE file.

package settings

import (
	"github.com/cockroachdb/redact"
	"github.com/cockroachdb/redact/interfaces"
)

// String is part of fmt.Stringer.
func (v EncodedValue) String() string {
	return redact.Sprint(v).StripMarkers()
}

// SafeFormat is part of redact.SafeFormatter.
func (v EncodedValue) SafeFormat(s interfaces.SafePrinter, verb rune) {
	s.Printf("%q (%s)", v.Value, redact.SafeString(v.Type))
}

var _ redact.SafeFormatter = EncodedValue{}
