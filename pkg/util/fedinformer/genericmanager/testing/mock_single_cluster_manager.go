// Code generated by MockGen. DO NOT EDIT.
// Source: single_cluster_manager.go

// Package testing is a generated GoMock package.
package testing

import (
	context "context"
	reflect "reflect"
	time "time"

	gomock "github.com/golang/mock/gomock"
	schema "k8s.io/apimachinery/pkg/runtime/schema"
	dynamic "k8s.io/client-go/dynamic"
	cache "k8s.io/client-go/tools/cache"
)

// MockSingleClusterInformerManager is a mock of SingleClusterInformerManager interface.
type MockSingleClusterInformerManager struct {
	ctrl     *gomock.Controller
	recorder *MockSingleClusterInformerManagerMockRecorder
}

// MockSingleClusterInformerManagerMockRecorder is the mock recorder for MockSingleClusterInformerManager.
type MockSingleClusterInformerManagerMockRecorder struct {
	mock *MockSingleClusterInformerManager
}

// NewMockSingleClusterInformerManager creates a new mock instance.
func NewMockSingleClusterInformerManager(ctrl *gomock.Controller) *MockSingleClusterInformerManager {
	mock := &MockSingleClusterInformerManager{ctrl: ctrl}
	mock.recorder = &MockSingleClusterInformerManagerMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockSingleClusterInformerManager) EXPECT() *MockSingleClusterInformerManagerMockRecorder {
	return m.recorder
}

// Context mocks base method.
func (m *MockSingleClusterInformerManager) Context() context.Context {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Context")
	ret0, _ := ret[0].(context.Context)
	return ret0
}

// Context indicates an expected call of Context.
func (mr *MockSingleClusterInformerManagerMockRecorder) Context() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Context", reflect.TypeOf((*MockSingleClusterInformerManager)(nil).Context))
}

// ForResource mocks base method.
func (m *MockSingleClusterInformerManager) ForResource(resource schema.GroupVersionResource, handler cache.ResourceEventHandler) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "ForResource", resource, handler)
}

// ForResource indicates an expected call of ForResource.
func (mr *MockSingleClusterInformerManagerMockRecorder) ForResource(resource, handler interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ForResource", reflect.TypeOf((*MockSingleClusterInformerManager)(nil).ForResource), resource, handler)
}

// GetClient mocks base method.
func (m *MockSingleClusterInformerManager) GetClient() dynamic.Interface {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetClient")
	ret0, _ := ret[0].(dynamic.Interface)
	return ret0
}

// GetClient indicates an expected call of GetClient.
func (mr *MockSingleClusterInformerManagerMockRecorder) GetClient() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetClient", reflect.TypeOf((*MockSingleClusterInformerManager)(nil).GetClient))
}

// IsHandlerExist mocks base method.
func (m *MockSingleClusterInformerManager) IsHandlerExist(resource schema.GroupVersionResource, handler cache.ResourceEventHandler) bool {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "IsHandlerExist", resource, handler)
	ret0, _ := ret[0].(bool)
	return ret0
}

// IsHandlerExist indicates an expected call of IsHandlerExist.
func (mr *MockSingleClusterInformerManagerMockRecorder) IsHandlerExist(resource, handler interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "IsHandlerExist", reflect.TypeOf((*MockSingleClusterInformerManager)(nil).IsHandlerExist), resource, handler)
}

// IsInformerSynced mocks base method.
func (m *MockSingleClusterInformerManager) IsInformerSynced(resource schema.GroupVersionResource) bool {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "IsInformerSynced", resource)
	ret0, _ := ret[0].(bool)
	return ret0
}

// IsInformerSynced indicates an expected call of IsInformerSynced.
func (mr *MockSingleClusterInformerManagerMockRecorder) IsInformerSynced(resource interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "IsInformerSynced", reflect.TypeOf((*MockSingleClusterInformerManager)(nil).IsInformerSynced), resource)
}

// Lister mocks base method.
func (m *MockSingleClusterInformerManager) Lister(resource schema.GroupVersionResource) cache.GenericLister {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Lister", resource)
	ret0, _ := ret[0].(cache.GenericLister)
	return ret0
}

// Lister indicates an expected call of Lister.
func (mr *MockSingleClusterInformerManagerMockRecorder) Lister(resource interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Lister", reflect.TypeOf((*MockSingleClusterInformerManager)(nil).Lister), resource)
}

// Start mocks base method.
func (m *MockSingleClusterInformerManager) Start() {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "Start")
}

// Start indicates an expected call of Start.
func (mr *MockSingleClusterInformerManagerMockRecorder) Start() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Start", reflect.TypeOf((*MockSingleClusterInformerManager)(nil).Start))
}

// Stop mocks base method.
func (m *MockSingleClusterInformerManager) Stop() {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "Stop")
}

// Stop indicates an expected call of Stop.
func (mr *MockSingleClusterInformerManagerMockRecorder) Stop() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Stop", reflect.TypeOf((*MockSingleClusterInformerManager)(nil).Stop))
}

// WaitForCacheSync mocks base method.
func (m *MockSingleClusterInformerManager) WaitForCacheSync() map[schema.GroupVersionResource]bool {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "WaitForCacheSync")
	ret0, _ := ret[0].(map[schema.GroupVersionResource]bool)
	return ret0
}

// WaitForCacheSync indicates an expected call of WaitForCacheSync.
func (mr *MockSingleClusterInformerManagerMockRecorder) WaitForCacheSync() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "WaitForCacheSync", reflect.TypeOf((*MockSingleClusterInformerManager)(nil).WaitForCacheSync))
}

// WaitForCacheSyncWithTimeout mocks base method.
func (m *MockSingleClusterInformerManager) WaitForCacheSyncWithTimeout(cacheSyncTimeout time.Duration) map[schema.GroupVersionResource]bool {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "WaitForCacheSyncWithTimeout", cacheSyncTimeout)
	ret0, _ := ret[0].(map[schema.GroupVersionResource]bool)
	return ret0
}

// WaitForCacheSyncWithTimeout indicates an expected call of WaitForCacheSyncWithTimeout.
func (mr *MockSingleClusterInformerManagerMockRecorder) WaitForCacheSyncWithTimeout(cacheSyncTimeout interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "WaitForCacheSyncWithTimeout", reflect.TypeOf((*MockSingleClusterInformerManager)(nil).WaitForCacheSyncWithTimeout), cacheSyncTimeout)
}
