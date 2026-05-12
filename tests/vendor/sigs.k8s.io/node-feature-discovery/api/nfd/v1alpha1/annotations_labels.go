/*
Copyright 2022 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

const (
	// FeatureLabelNs is the (default) namespace for feature labels.
	FeatureLabelNs = "feature.node.kubernetes.io"

	// FeatureLabelSubNsSuffix is the suffix for allowed feature label sub-namespaces.
	FeatureLabelSubNsSuffix = "." + FeatureLabelNs

	// ProfileLabelNs is the namespace for profile labels.
	ProfileLabelNs = "profile.node.kubernetes.io"

	// ProfileLabelSubNsSuffix is the suffix for allowed profile label sub-namespaces.
	ProfileLabelSubNsSuffix = "." + ProfileLabelNs

	// TaintNs is the k8s.io namespace that can be used for NFD-managed taints.
	TaintNs = "feature.node.kubernetes.io"

	// TaintSubNsSuffix is the suffix for allowed sub-namespaces for NFD-managed taints.
	TaintSubNsSuffix = "." + TaintNs

	// AnnotationNs namespace for all NFD-related annotations.
	AnnotationNs = "nfd.node.kubernetes.io"

	// ExtendedResourceNs is the namespace for extended resources.
	ExtendedResourceNs = "feature.node.kubernetes.io"

	// ExtendedResourceSubNsSuffix is the suffix for allowed extended resources sub-namespaces.
	ExtendedResourceSubNsSuffix = "." + ExtendedResourceNs

	// ExtendedResourceAnnotation is the annotation that holds all extended resources managed by NFD.
	ExtendedResourceAnnotation = AnnotationNs + "/extended-resources"

	// FeatureLabelsAnnotation is the annotation that holds all feature labels managed by NFD.
	FeatureLabelsAnnotation = AnnotationNs + "/feature-labels"

	// MasterVersionAnnotation is the annotation that holds the version of nfd-master running on the node
	// DEPRECATED: will not be used in NFD v0.15 or later.
	MasterVersionAnnotation = AnnotationNs + "/master.version"

	// WorkerVersionAnnotation is the annotation that holds the version of nfd-worker running on the node
	WorkerVersionAnnotation = AnnotationNs + "/worker.version"

	// NodeTaintsAnnotation is the annotation that holds the taints that nfd-master set on the node
	NodeTaintsAnnotation = AnnotationNs + "/taints"

	// FeatureAnnotationsTrackingAnnotation is the annotation that holds all feature annotations that nfd-master set on the node
	FeatureAnnotationsTrackingAnnotation = AnnotationNs + "/feature-annotations"

	// NodeFeatureObjNodeNameLabel is the label that specifies which node the
	// NodeFeature object is targeting. Creators of NodeFeature objects must
	// set this label and consumers of the objects are supposed to use the
	// label for filtering features designated for a certain node.
	NodeFeatureObjNodeNameLabel = "nfd.node.kubernetes.io/node-name"

	// FeatureAnnotationNs is the (default) namespace for feature annotations.
	FeatureAnnotationNs = "feature.node.kubernetes.io"

	// FeatureAnnotationSubNsSuffix is the suffix for allowed feature annotation sub-namespaces.
	FeatureAnnotationSubNsSuffix = "." + FeatureAnnotationNs

	// FeatureAnnotationValueSizeLimit is the maximum allowed length for the value of a feature annotation.
	FeatureAnnotationValueSizeLimit = 1 << 10
)
