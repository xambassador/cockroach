// Copyright 2022 The Cockroach Authors.
//
// Use of this software is governed by the CockroachDB Software License
// included in the /LICENSE file.

package contention

import (
	"context"

	"github.com/cockroachdb/cockroach/pkg/server/serverpb"
	"github.com/cockroachdb/cockroach/pkg/sql/appstatspb"
	"github.com/cockroachdb/cockroach/pkg/sql/contentionpb"
	"github.com/cockroachdb/cockroach/pkg/util/uuid"
)

type fakeStatusServer struct {
	data          map[uuid.UUID]appstatspb.TransactionFingerprintID
	injectedError error
}

func newFakeStatusServer() *fakeStatusServer {
	return &fakeStatusServer{
		data:          make(map[uuid.UUID]appstatspb.TransactionFingerprintID),
		injectedError: nil,
	}
}

func (f *fakeStatusServer) txnIDResolution(
	_ context.Context, req *serverpb.TxnIDResolutionRequest,
) (*serverpb.TxnIDResolutionResponse, error) {
	if f.injectedError != nil {
		return nil, f.injectedError
	}

	resp := &serverpb.TxnIDResolutionResponse{
		ResolvedTxnIDs: make([]contentionpb.ResolvedTxnID, 0),
	}

	for _, txnID := range req.TxnIDs {
		if txnFingerprintID, ok := f.data[txnID]; ok {
			resp.ResolvedTxnIDs = append(resp.ResolvedTxnIDs, contentionpb.ResolvedTxnID{
				TxnID:            txnID,
				TxnFingerprintID: txnFingerprintID,
			})
		}
	}

	return resp, nil
}

func (f *fakeStatusServer) setTxnIDEntry(
	txnID uuid.UUID, txnFingerprintID appstatspb.TransactionFingerprintID,
) {
	f.data[txnID] = txnFingerprintID
}

type fakeStatusServerCluster map[string]*fakeStatusServer

func newFakeStatusServerCluster() fakeStatusServerCluster {
	return make(fakeStatusServerCluster)
}

func (f fakeStatusServerCluster) getStatusServer(coordinatorID string) *fakeStatusServer {
	statusServer, ok := f[coordinatorID]
	if !ok {
		statusServer = newFakeStatusServer()
		f[coordinatorID] = statusServer
	}
	return statusServer
}

func (f fakeStatusServerCluster) txnIDResolution(
	ctx context.Context, req *serverpb.TxnIDResolutionRequest,
) (*serverpb.TxnIDResolutionResponse, error) {
	return f.getStatusServer(req.CoordinatorID).txnIDResolution(ctx, req)
}

func (f fakeStatusServerCluster) setTxnIDEntry(
	coordinatorNodeID string, txnID uuid.UUID, txnFingerprintID appstatspb.TransactionFingerprintID,
) {
	f.getStatusServer(coordinatorNodeID).setTxnIDEntry(txnID, txnFingerprintID)
}

func (f fakeStatusServerCluster) setStatusServerError(coordinatorNodeID string, err error) {
	f.getStatusServer(coordinatorNodeID).injectedError = err
}

func (f fakeStatusServerCluster) clear() {
	for k := range f {
		delete(f, k)
	}
}

func (f fakeStatusServerCluster) clearErrors() {
	for _, statusServer := range f {
		statusServer.injectedError = nil
	}
}
