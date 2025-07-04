// Code generated by MockGen. DO NOT EDIT.
// Source: github.com/cockroachdb/cockroach/pkg/kv/kvpb (interfaces: RPCInternalClient,RPCInternal_MuxRangeFeedClient)

// Package kvpbmock is a generated GoMock package.
package kvpbmock

import (
	context "context"
	reflect "reflect"

	kvpb "github.com/cockroachdb/cockroach/pkg/kv/kvpb"
	roachpb "github.com/cockroachdb/cockroach/pkg/roachpb"
	gomock "github.com/golang/mock/gomock"
)

// MockRPCInternalClient is a mock of RPCInternalClient interface.
type MockRPCInternalClient struct {
	ctrl     *gomock.Controller
	recorder *MockRPCInternalClientMockRecorder
}

// MockRPCInternalClientMockRecorder is the mock recorder for MockRPCInternalClient.
type MockRPCInternalClientMockRecorder struct {
	mock *MockRPCInternalClient
}

// NewMockRPCInternalClient creates a new mock instance.
func NewMockRPCInternalClient(ctrl *gomock.Controller) *MockRPCInternalClient {
	mock := &MockRPCInternalClient{ctrl: ctrl}
	mock.recorder = &MockRPCInternalClientMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockRPCInternalClient) EXPECT() *MockRPCInternalClientMockRecorder {
	return m.recorder
}

// Batch mocks base method.
func (m *MockRPCInternalClient) Batch(arg0 context.Context, arg1 *kvpb.BatchRequest) (*kvpb.BatchResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Batch", arg0, arg1)
	ret0, _ := ret[0].(*kvpb.BatchResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Batch indicates an expected call of Batch.
func (mr *MockRPCInternalClientMockRecorder) Batch(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Batch", reflect.TypeOf((*MockRPCInternalClient)(nil).Batch), arg0, arg1)
}

// BatchStream mocks base method.
func (m *MockRPCInternalClient) BatchStream(arg0 context.Context) (kvpb.RPCInternal_BatchStreamClient, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "BatchStream", arg0)
	ret0, _ := ret[0].(kvpb.RPCInternal_BatchStreamClient)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// BatchStream indicates an expected call of BatchStream.
func (mr *MockRPCInternalClientMockRecorder) BatchStream(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "BatchStream", reflect.TypeOf((*MockRPCInternalClient)(nil).BatchStream), arg0)
}

// GetAllSystemSpanConfigsThatApply mocks base method.
func (m *MockRPCInternalClient) GetAllSystemSpanConfigsThatApply(arg0 context.Context, arg1 *roachpb.GetAllSystemSpanConfigsThatApplyRequest) (*roachpb.GetAllSystemSpanConfigsThatApplyResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetAllSystemSpanConfigsThatApply", arg0, arg1)
	ret0, _ := ret[0].(*roachpb.GetAllSystemSpanConfigsThatApplyResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetAllSystemSpanConfigsThatApply indicates an expected call of GetAllSystemSpanConfigsThatApply.
func (mr *MockRPCInternalClientMockRecorder) GetAllSystemSpanConfigsThatApply(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetAllSystemSpanConfigsThatApply", reflect.TypeOf((*MockRPCInternalClient)(nil).GetAllSystemSpanConfigsThatApply), arg0, arg1)
}

// GetRangeDescriptors mocks base method.
func (m *MockRPCInternalClient) GetRangeDescriptors(arg0 context.Context, arg1 *kvpb.GetRangeDescriptorsRequest) (kvpb.RPCInternal_GetRangeDescriptorsClient, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetRangeDescriptors", arg0, arg1)
	ret0, _ := ret[0].(kvpb.RPCInternal_GetRangeDescriptorsClient)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetRangeDescriptors indicates an expected call of GetRangeDescriptors.
func (mr *MockRPCInternalClientMockRecorder) GetRangeDescriptors(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetRangeDescriptors", reflect.TypeOf((*MockRPCInternalClient)(nil).GetRangeDescriptors), arg0, arg1)
}

// GetSpanConfigs mocks base method.
func (m *MockRPCInternalClient) GetSpanConfigs(arg0 context.Context, arg1 *roachpb.GetSpanConfigsRequest) (*roachpb.GetSpanConfigsResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetSpanConfigs", arg0, arg1)
	ret0, _ := ret[0].(*roachpb.GetSpanConfigsResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetSpanConfigs indicates an expected call of GetSpanConfigs.
func (mr *MockRPCInternalClientMockRecorder) GetSpanConfigs(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetSpanConfigs", reflect.TypeOf((*MockRPCInternalClient)(nil).GetSpanConfigs), arg0, arg1)
}

// GossipSubscription mocks base method.
func (m *MockRPCInternalClient) GossipSubscription(arg0 context.Context, arg1 *kvpb.GossipSubscriptionRequest) (kvpb.RPCInternal_GossipSubscriptionClient, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GossipSubscription", arg0, arg1)
	ret0, _ := ret[0].(kvpb.RPCInternal_GossipSubscriptionClient)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GossipSubscription indicates an expected call of GossipSubscription.
func (mr *MockRPCInternalClientMockRecorder) GossipSubscription(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GossipSubscription", reflect.TypeOf((*MockRPCInternalClient)(nil).GossipSubscription), arg0, arg1)
}

// Join mocks base method.
func (m *MockRPCInternalClient) Join(arg0 context.Context, arg1 *kvpb.JoinNodeRequest) (*kvpb.JoinNodeResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Join", arg0, arg1)
	ret0, _ := ret[0].(*kvpb.JoinNodeResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Join indicates an expected call of Join.
func (mr *MockRPCInternalClientMockRecorder) Join(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Join", reflect.TypeOf((*MockRPCInternalClient)(nil).Join), arg0, arg1)
}

// MuxRangeFeed mocks base method.
func (m *MockRPCInternalClient) MuxRangeFeed(arg0 context.Context) (kvpb.RPCInternal_MuxRangeFeedClient, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "MuxRangeFeed", arg0)
	ret0, _ := ret[0].(kvpb.RPCInternal_MuxRangeFeedClient)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// MuxRangeFeed indicates an expected call of MuxRangeFeed.
func (mr *MockRPCInternalClientMockRecorder) MuxRangeFeed(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "MuxRangeFeed", reflect.TypeOf((*MockRPCInternalClient)(nil).MuxRangeFeed), arg0)
}

// RangeLookup mocks base method.
func (m *MockRPCInternalClient) RangeLookup(arg0 context.Context, arg1 *kvpb.RangeLookupRequest) (*kvpb.RangeLookupResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "RangeLookup", arg0, arg1)
	ret0, _ := ret[0].(*kvpb.RangeLookupResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// RangeLookup indicates an expected call of RangeLookup.
func (mr *MockRPCInternalClientMockRecorder) RangeLookup(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "RangeLookup", reflect.TypeOf((*MockRPCInternalClient)(nil).RangeLookup), arg0, arg1)
}

// ResetQuorum mocks base method.
func (m *MockRPCInternalClient) ResetQuorum(arg0 context.Context, arg1 *kvpb.ResetQuorumRequest) (*kvpb.ResetQuorumResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ResetQuorum", arg0, arg1)
	ret0, _ := ret[0].(*kvpb.ResetQuorumResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ResetQuorum indicates an expected call of ResetQuorum.
func (mr *MockRPCInternalClientMockRecorder) ResetQuorum(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ResetQuorum", reflect.TypeOf((*MockRPCInternalClient)(nil).ResetQuorum), arg0, arg1)
}

// SpanConfigConformance mocks base method.
func (m *MockRPCInternalClient) SpanConfigConformance(arg0 context.Context, arg1 *roachpb.SpanConfigConformanceRequest) (*roachpb.SpanConfigConformanceResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SpanConfigConformance", arg0, arg1)
	ret0, _ := ret[0].(*roachpb.SpanConfigConformanceResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// SpanConfigConformance indicates an expected call of SpanConfigConformance.
func (mr *MockRPCInternalClientMockRecorder) SpanConfigConformance(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SpanConfigConformance", reflect.TypeOf((*MockRPCInternalClient)(nil).SpanConfigConformance), arg0, arg1)
}

// TenantSettings mocks base method.
func (m *MockRPCInternalClient) TenantSettings(arg0 context.Context, arg1 *kvpb.TenantSettingsRequest) (kvpb.RPCInternal_TenantSettingsClient, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "TenantSettings", arg0, arg1)
	ret0, _ := ret[0].(kvpb.RPCInternal_TenantSettingsClient)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// TenantSettings indicates an expected call of TenantSettings.
func (mr *MockRPCInternalClientMockRecorder) TenantSettings(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "TenantSettings", reflect.TypeOf((*MockRPCInternalClient)(nil).TenantSettings), arg0, arg1)
}

// TokenBucket mocks base method.
func (m *MockRPCInternalClient) TokenBucket(arg0 context.Context, arg1 *kvpb.TokenBucketRequest) (*kvpb.TokenBucketResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "TokenBucket", arg0, arg1)
	ret0, _ := ret[0].(*kvpb.TokenBucketResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// TokenBucket indicates an expected call of TokenBucket.
func (mr *MockRPCInternalClientMockRecorder) TokenBucket(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "TokenBucket", reflect.TypeOf((*MockRPCInternalClient)(nil).TokenBucket), arg0, arg1)
}

// UpdateSpanConfigs mocks base method.
func (m *MockRPCInternalClient) UpdateSpanConfigs(arg0 context.Context, arg1 *roachpb.UpdateSpanConfigsRequest) (*roachpb.UpdateSpanConfigsResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "UpdateSpanConfigs", arg0, arg1)
	ret0, _ := ret[0].(*roachpb.UpdateSpanConfigsResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// UpdateSpanConfigs indicates an expected call of UpdateSpanConfigs.
func (mr *MockRPCInternalClientMockRecorder) UpdateSpanConfigs(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "UpdateSpanConfigs", reflect.TypeOf((*MockRPCInternalClient)(nil).UpdateSpanConfigs), arg0, arg1)
}

// MockRPCInternal_MuxRangeFeedClient is a mock of RPCInternal_MuxRangeFeedClient interface.
type MockRPCInternal_MuxRangeFeedClient struct {
	ctrl     *gomock.Controller
	recorder *MockRPCInternal_MuxRangeFeedClientMockRecorder
}

// MockRPCInternal_MuxRangeFeedClientMockRecorder is the mock recorder for MockRPCInternal_MuxRangeFeedClient.
type MockRPCInternal_MuxRangeFeedClientMockRecorder struct {
	mock *MockRPCInternal_MuxRangeFeedClient
}

// NewMockRPCInternal_MuxRangeFeedClient creates a new mock instance.
func NewMockRPCInternal_MuxRangeFeedClient(ctrl *gomock.Controller) *MockRPCInternal_MuxRangeFeedClient {
	mock := &MockRPCInternal_MuxRangeFeedClient{ctrl: ctrl}
	mock.recorder = &MockRPCInternal_MuxRangeFeedClientMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockRPCInternal_MuxRangeFeedClient) EXPECT() *MockRPCInternal_MuxRangeFeedClientMockRecorder {
	return m.recorder
}

// CloseSend mocks base method.
func (m *MockRPCInternal_MuxRangeFeedClient) CloseSend() error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CloseSend")
	ret0, _ := ret[0].(error)
	return ret0
}

// CloseSend indicates an expected call of CloseSend.
func (mr *MockRPCInternal_MuxRangeFeedClientMockRecorder) CloseSend() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CloseSend", reflect.TypeOf((*MockRPCInternal_MuxRangeFeedClient)(nil).CloseSend))
}

// Context mocks base method.
func (m *MockRPCInternal_MuxRangeFeedClient) Context() context.Context {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Context")
	ret0, _ := ret[0].(context.Context)
	return ret0
}

// Context indicates an expected call of Context.
func (mr *MockRPCInternal_MuxRangeFeedClientMockRecorder) Context() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Context", reflect.TypeOf((*MockRPCInternal_MuxRangeFeedClient)(nil).Context))
}

// Recv mocks base method.
func (m *MockRPCInternal_MuxRangeFeedClient) Recv() (*kvpb.MuxRangeFeedEvent, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Recv")
	ret0, _ := ret[0].(*kvpb.MuxRangeFeedEvent)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Recv indicates an expected call of Recv.
func (mr *MockRPCInternal_MuxRangeFeedClientMockRecorder) Recv() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Recv", reflect.TypeOf((*MockRPCInternal_MuxRangeFeedClient)(nil).Recv))
}

// Send mocks base method.
func (m *MockRPCInternal_MuxRangeFeedClient) Send(arg0 *kvpb.RangeFeedRequest) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Send", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// Send indicates an expected call of Send.
func (mr *MockRPCInternal_MuxRangeFeedClientMockRecorder) Send(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Send", reflect.TypeOf((*MockRPCInternal_MuxRangeFeedClient)(nil).Send), arg0)
}
