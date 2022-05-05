package metric

import (
	"fmt"
	"strings"

	"github.com/segmentio/go-camelcase"
	v1 "k8s.io/api/core/v1"

	"github.com/newrelic/nri-kubernetes/v3/src/definition"
)

type (
	resourceType string
	resourceUnit string
)

const (
	resourceTypeAllocatable resourceType = "allocatable"
	resourceTypeCapacity    resourceType = "capacity"
	resourceUnitBytes       resourceUnit = "Bytes"
	resourceUnitCores       resourceUnit = "Cores"
)

// oneAttributePerResource transforms a map of resources to
// FetchedValues type, which will be converted later to one attribute
// per resource.
//
// The attribute names will be prefixed with the given resourceType.
func oneAttributePerResource(rawResources definition.FetchedValue, r resourceType) (definition.FetchedValue, error) {
	resources, ok := rawResources.(v1.ResourceList)
	if !ok {
		return rawResources, fmt.Errorf("creating resource %s attributes", r)
	}

	modified := make(definition.FetchedValues, len(resources))
	for resourceName, resourceQuantity := range resources {
		n := camelcase.Camelcase(string(r) + strings.Title(addResourceUnit(resourceName)))

		switch resourceName {
		case v1.ResourceCPU:
			// AsApproximateFloat64 is used to avoid round up CPU cores metrics which are reported in cores to New Relic.
			modified[n] = resourceQuantity.AsApproximateFloat64()
		default:
			modified[n] = resourceQuantity.Value()
		}
	}

	return modified, nil
}

// addResourceUnit adds the resource unit as a suffix.
func addResourceUnit(resource v1.ResourceName) string {
	switch resource {
	case v1.ResourceEphemeralStorage:
		return string(resource) + string(resourceUnitBytes)
	case v1.ResourceMemory:
		return string(resource) + string(resourceUnitBytes)
	case v1.ResourceCPU:
		return string(resource) + string(resourceUnitCores)
	case v1.ResourceStorage:
		return string(resource) + string(resourceUnitBytes)
	}
	return string(resource)
}

// OneAttributePerAllocatable transforms a map of resources to
// FetchedValues type, which will be converted later to one attribute
// per allocatable resource.
//
// The attribute names will be prefixed with `allocatable.`.
func OneAttributePerAllocatable(rawResources definition.FetchedValue) (definition.FetchedValue, error) {
	return oneAttributePerResource(rawResources, resourceTypeAllocatable)
}

// OneAttributePerCapacity transforms a map of resources to
// FetchedValues type, which will be converted later to one attribute
// per capacity resource.
//
// The attribute names will be prefixed with `capacity.`.
func OneAttributePerCapacity(rawResources definition.FetchedValue) (definition.FetchedValue, error) {
	return oneAttributePerResource(rawResources, resourceTypeCapacity)
}
