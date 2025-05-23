// Copyright 2021 The Cockroach Authors.
//
// Use of this software is governed by the CockroachDB Software License
// included in the /LICENSE file.

package randgen

import (
	"context"
	"math/rand"
	"sort"

	clustersettings "github.com/cockroachdb/cockroach/pkg/settings/cluster"
	"github.com/cockroachdb/cockroach/pkg/sql/catalog/colinfo"
	"github.com/cockroachdb/cockroach/pkg/sql/oidext"
	"github.com/cockroachdb/cockroach/pkg/sql/rowenc/valueside"
	"github.com/cockroachdb/cockroach/pkg/sql/types"
	"github.com/lib/pq/oid"
)

var (
	// SeedTypes includes the following types that form the basis of randomly
	// generated types:
	//   - All scalar types, except UNKNOWN, ANY, TRIGGER, REGNAMESPACE, and
	//     FLOAT4
	//   - ARRAY of ANY and TUPLE of ANY, where the ANY will be replaced with
	//     one of the legal array element types in RandType
	//   - OIDVECTOR and INT2VECTOR types
	SeedTypes []*types.T

	// arrayContentsTypes contains all of the types that are valid to store within
	// an array.
	arrayContentsTypes []*types.T
	collationLocales   = [...]string{"da_DK", "de_DE", "en_US"}
)

func init() {
	for _, typ := range types.OidToType {
		switch typ.Oid() {
		case oid.T_regnamespace:
			// Temporarily don't include this.
			// TODO(msirek): Remove this exclusion once
			// https://github.com/cockroachdb/cockroach/issues/55791 is fixed.
		case oid.T_unknown, oid.T_anyelement, oid.T_any, oid.T_trigger:
			// Don't include these.
		case oid.T_float4:
			// Don't include FLOAT4 due to known bugs that cause test failures.
			// See #73743 and #48613.
		case oidext.T_jsonpath:
			// TODO(#22513): Temporarily don't include Jsonpath
		case oid.T_anyarray, oid.T_oidvector, oid.T_int2vector:
			// Include these.
			SeedTypes = append(SeedTypes, typ)
		default:
			// Only include scalar types.
			if typ.Family() != types.ArrayFamily {
				SeedTypes = append(SeedTypes, typ)
			}
		}
	}

	for _, typ := range types.OidToType {
		if IsAllowedForArray(typ) {
			arrayContentsTypes = append(arrayContentsTypes, typ)
		}
	}

	// Add a collated string separately (since it shares the oid with the STRING
	// type and, thus, wasn't included above).
	collatedStringType := types.MakeCollatedString(types.String, "en" /* locale */)
	SeedTypes = append(SeedTypes, collatedStringType)
	if IsAllowedForArray(collatedStringType) {
		arrayContentsTypes = append(arrayContentsTypes, collatedStringType)
	}

	// Sort these so randomly chosen indexes always point to the same element.
	sort.Slice(SeedTypes, func(i, j int) bool {
		return SeedTypes[i].String() < SeedTypes[j].String()
	})
	sort.Slice(arrayContentsTypes, func(i, j int) bool {
		return arrayContentsTypes[i].String() < arrayContentsTypes[j].String()
	})
}

// IsAllowedForArray returns true iff the passed in type can be a valid ArrayContents()
func IsAllowedForArray(typ *types.T) bool {
	// Don't include un-encodable types.
	encTyp, err := valueside.DatumTypeToArrayElementEncodingType(typ)
	if err != nil || encTyp == 0 {
		return false
	}

	// Don't include reg types, since parser currently doesn't allow them to
	// be declared as array element types.
	if typ.Family() == types.OidFamily && typ.Oid() != oid.T_oid {
		return false
	}

	return true
}

// RandType returns a random type value.
func RandType(rng *rand.Rand) *types.T {
	return RandTypeFromSlice(rng, SeedTypes)
}

// RandArrayContentsType returns a random type that's guaranteed to be valid to
// use as the contents of an array.
func RandArrayContentsType(rng *rand.Rand) *types.T {
	return RandTypeFromSlice(rng, arrayContentsTypes)
}

// RandTypeFromSlice returns a random type from the input slice of types.
func RandTypeFromSlice(rng *rand.Rand, typs []*types.T) *types.T {
	typ := typs[rng.Intn(len(typs))]
	switch typ.Family() {
	case types.BitFamily:
		return types.MakeBit(int32(rng.Intn(50)))
	case types.CollatedStringFamily:
		return types.MakeCollatedString(types.String, *RandCollationLocale(rng))
	case types.ArrayFamily:
		if typ.ArrayContents().Family() == types.AnyFamily {
			inner := RandArrayContentsType(rng)
			if inner.Family() == types.CollatedStringFamily {
				// TODO(justin): change this when collated arrays are supported.
				inner = types.String
			}
			return types.MakeArray(inner)
		}
		if typ.ArrayContents().Family() == types.TupleFamily {
			return types.MakeArray(RandTupleFromSlice(rng, typs))
		}
	case types.TupleFamily:
		// In 50% of cases generate a new tuple type based on the given slice;
		// in other 50% just use the provided tuple type (if it's not a wildcard
		// type).
		if rng.Intn(2) == 0 && !typ.Identical(types.AnyTuple) {
			return typ
		}
		return RandTupleFromSlice(rng, typs)
	}
	return typ
}

// RandTupleFromSlice returns a random tuple which has field chosen randomly
// from the input slice of types.
func RandTupleFromSlice(rng *rand.Rand, typs []*types.T) *types.T {
	// Generate tuples between 0 and 4 datums in length
	len := rng.Intn(5)
	contents := make([]*types.T, len)
	for i := range contents {
		contents[i] = RandTypeFromSlice(rng, typs)
	}
	return types.MakeTuple(contents)
}

// RandColumnType returns a random type that is a legal column type (e.g. no
// nested arrays or tuples).
func RandColumnType(rng *rand.Rand) *types.T {
	for {
		typ := RandType(rng)
		if IsLegalColumnType(typ) {
			return typ
		}
	}
}

// IsLegalColumnType returns true if the given type can be
// given to a column in a user-created table.
func IsLegalColumnType(typ *types.T) bool {
	switch typ.Oid() {
	case oid.T_int2vector, oid.T_oidvector:
		// OIDVECTOR and INT2VECTOR are not valid column types for
		// user-created tables.
		return false
	case oid.T_regproc, oid.T_regprocedure:
		// REGPROC and REGPROCEDURE columns hit an edge case. Customers are very
		// unlikely to use these types of columns, so disabling their generation
		// is low risk.
		// TODO(#95641): Remove this once we correctly handle this edge case.
		return false
	case oidext.T_jsonpath, oidext.T__jsonpath:
		// Jsonpath and Jsonpath[] columns are not supported yet. Customers are very
		// unlikely to use these types of columns, so disabling their generation
		// is low risk.
		return false
	}
	ctx := context.Background()
	st := clustersettings.MakeTestingClusterSettings()
	return colinfo.ValidateColumnDefType(ctx, st, typ) == nil
}

// RandArrayType generates a random array type.
func RandArrayType(rng *rand.Rand) *types.T {
	ctx := context.Background()
	st := clustersettings.MakeTestingClusterSettings()
	for {
		typ := RandColumnType(rng)
		resTyp := types.MakeArray(typ)
		if err := colinfo.ValidateColumnDefType(ctx, st, resTyp); err == nil {
			return resTyp
		}
	}
}

// RandColumnTypes returns a slice of numCols random types. These types must be
// legal table column types.
func RandColumnTypes(rng *rand.Rand, numCols int) []*types.T {
	types := make([]*types.T, numCols)
	for i := range types {
		types[i] = RandColumnType(rng)
	}
	return types
}

// RandSortingType returns a column type which can be key-encoded.
func RandSortingType(rng *rand.Rand) *types.T {
	typ := RandType(rng)
	for colinfo.MustBeValueEncoded(typ) || typ.Family() == types.VoidFamily {
		typ = RandType(rng)
	}
	return typ
}

// RandSortingTypes returns a slice of numCols random ColumnType values
// which are key-encodable.
func RandSortingTypes(rng *rand.Rand, numCols int) []*types.T {
	types := make([]*types.T, numCols)
	for i := range types {
		types[i] = RandSortingType(rng)
	}
	return types
}

// RandCollationLocale returns a random element of collationLocales.
func RandCollationLocale(rng *rand.Rand) *string {
	return &collationLocales[rng.Intn(len(collationLocales))]
}
