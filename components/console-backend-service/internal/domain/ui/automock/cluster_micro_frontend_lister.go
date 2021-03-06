// Code generated by mockery v1.0.0. DO NOT EDIT.

package automock

import mock "github.com/stretchr/testify/mock"

import v1alpha1 "github.com/kyma-project/kyma/common/microfrontend-client/pkg/apis/ui/v1alpha1"

// clusterMicroFrontendLister is an autogenerated mock type for the clusterMicroFrontendLister type
type clusterMicroFrontendLister struct {
	mock.Mock
}

// List provides a mock function with given fields:
func (_m *clusterMicroFrontendLister) List() ([]*v1alpha1.ClusterMicroFrontend, error) {
	ret := _m.Called()

	var r0 []*v1alpha1.ClusterMicroFrontend
	if rf, ok := ret.Get(0).(func() []*v1alpha1.ClusterMicroFrontend); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]*v1alpha1.ClusterMicroFrontend)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func() error); ok {
		r1 = rf()
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}
