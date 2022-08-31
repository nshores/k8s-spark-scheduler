// Code generated by lister-gen. DO NOT EDIT.

package v1beta1

import (
	v1beta1 "github.com/palantir/k8s-spark-scheduler-lib/pkg/apis/sparkscheduler/v1beta1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"
)

// ResourceReservationLister helps list ResourceReservations.
// All objects returned here must be treated as read-only.
type ResourceReservationLister interface {
	// List lists all ResourceReservations in the indexer.
	// Objects returned here must be treated as read-only.
	List(selector labels.Selector) (ret []*v1beta1.ResourceReservation, err error)
	// ResourceReservations returns an object that can list and get ResourceReservations.
	ResourceReservations(namespace string) ResourceReservationNamespaceLister
	ResourceReservationListerExpansion
}

// resourceReservationLister implements the ResourceReservationLister interface.
type resourceReservationLister struct {
	indexer cache.Indexer
}

// NewResourceReservationLister returns a new ResourceReservationLister.
func NewResourceReservationLister(indexer cache.Indexer) ResourceReservationLister {
	return &resourceReservationLister{indexer: indexer}
}

// List lists all ResourceReservations in the indexer.
func (s *resourceReservationLister) List(selector labels.Selector) (ret []*v1beta1.ResourceReservation, err error) {
	err = cache.ListAll(s.indexer, selector, func(m interface{}) {
		ret = append(ret, m.(*v1beta1.ResourceReservation))
	})
	return ret, err
}

// ResourceReservations returns an object that can list and get ResourceReservations.
func (s *resourceReservationLister) ResourceReservations(namespace string) ResourceReservationNamespaceLister {
	return resourceReservationNamespaceLister{indexer: s.indexer, namespace: namespace}
}

// ResourceReservationNamespaceLister helps list and get ResourceReservations.
// All objects returned here must be treated as read-only.
type ResourceReservationNamespaceLister interface {
	// List lists all ResourceReservations in the indexer for a given namespace.
	// Objects returned here must be treated as read-only.
	List(selector labels.Selector) (ret []*v1beta1.ResourceReservation, err error)
	// Get retrieves the ResourceReservation from the indexer for a given namespace and name.
	// Objects returned here must be treated as read-only.
	Get(name string) (*v1beta1.ResourceReservation, error)
	ResourceReservationNamespaceListerExpansion
}

// resourceReservationNamespaceLister implements the ResourceReservationNamespaceLister
// interface.
type resourceReservationNamespaceLister struct {
	indexer   cache.Indexer
	namespace string
}

// List lists all ResourceReservations in the indexer for a given namespace.
func (s resourceReservationNamespaceLister) List(selector labels.Selector) (ret []*v1beta1.ResourceReservation, err error) {
	err = cache.ListAllByNamespace(s.indexer, s.namespace, selector, func(m interface{}) {
		ret = append(ret, m.(*v1beta1.ResourceReservation))
	})
	return ret, err
}

// Get retrieves the ResourceReservation from the indexer for a given namespace and name.
func (s resourceReservationNamespaceLister) Get(name string) (*v1beta1.ResourceReservation, error) {
	obj, exists, err := s.indexer.GetByKey(s.namespace + "/" + name)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, errors.NewNotFound(v1beta1.Resource("resourcereservation"), name)
	}
	return obj.(*v1beta1.ResourceReservation), nil
}
