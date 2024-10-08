// Copyright 2019 The Cockroach Authors.
//
// Use of this software is governed by the CockroachDB Software License
// included in the /LICENSE file.

package keysutils

import (
	"fmt"
	"testing"

	"github.com/cockroachdb/cockroach/pkg/keys"
	"github.com/cockroachdb/cockroach/pkg/roachpb"
	"github.com/cockroachdb/cockroach/pkg/testutils"
	"github.com/cockroachdb/cockroach/pkg/util/encoding"
)

func TestPrettyScanner(t *testing.T) {
	tests := []struct {
		prettyKey    string
		expKey       func(roachpb.TenantID) roachpb.Key
		expRemainder string
	}{
		{
			prettyKey: "/Table/t1",
			expKey: func(tenantID roachpb.TenantID) roachpb.Key {
				return keys.MakeSQLCodec(tenantID).TablePrefix(50)
			},
		},
		{
			prettyKey: "/Table/t1/pk",
			expKey: func(tenantID roachpb.TenantID) roachpb.Key {
				return keys.MakeSQLCodec(tenantID).IndexPrefix(50, 1)
			},
		},
		{
			prettyKey: "/Table/t1/pk/1/2/3",
			expKey: func(tenantID roachpb.TenantID) roachpb.Key {
				k := keys.MakeSQLCodec(tenantID).IndexPrefix(50, 1)
				k = encoding.EncodeVarintAscending(k, 1)
				k = encoding.EncodeVarintAscending(k, 2)
				k = encoding.EncodeVarintAscending(k, 3)
				return k
			},
		},
		{
			prettyKey:    "/Table/t1/pk/1/2/3/foo",
			expKey:       nil,
			expRemainder: "/foo",
		},
		{
			prettyKey: "/Table/t1/idx1/1/2/3",
			expKey: func(tenantID roachpb.TenantID) roachpb.Key {
				k := keys.MakeSQLCodec(tenantID).IndexPrefix(50, 5)
				k = encoding.EncodeVarintAscending(k, 1)
				k = encoding.EncodeVarintAscending(k, 2)
				k = encoding.EncodeVarintAscending(k, 3)
				return k
			},
		},
	}

	tableToID := map[string]int{"t1": 50}
	idxToID := map[string]int{"t1.idx1": 5}
	for _, tenantID := range []roachpb.TenantID{roachpb.SystemTenantID, roachpb.MustMakeTenantID(42)} {
		scanner := MakePrettyScannerForNamedTables(tenantID, tableToID, idxToID)
		for _, test := range tests {
			prettyKey := test.prettyKey
			if tenantID != roachpb.SystemTenantID {
				prettyKey = fmt.Sprintf("/Tenant/%s%s", tenantID, prettyKey)
			}
			t.Run(prettyKey, func(t *testing.T) {
				k, err := scanner.Scan(prettyKey)
				if err != nil {
					if test.expRemainder != "" {
						if testutils.IsError(err, fmt.Sprintf("can't parse\"%s\"", test.expRemainder)) {
							t.Fatalf("expected remainder: %s, got err: %s", test.expRemainder, err)
						}
					} else {
						t.Fatal(err)
					}
				}
				if test.expRemainder != "" && err == nil {
					t.Fatalf("expected a remainder but got none: %s", test.expRemainder)
				}
				if test.expKey == nil {
					if k != nil {
						t.Fatalf("unexpected key returned: %s", k)
					}
					return
				}
				expKey := test.expKey(tenantID)
				if !k.Equal(expKey) {
					t.Fatalf("expected: %+v, got %+v", []byte(expKey), []byte(k))
				}
			})
		}
	}
}
