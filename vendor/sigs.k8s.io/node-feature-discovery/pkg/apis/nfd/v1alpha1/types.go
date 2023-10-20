/*
Copyright 2021 The Kubernetes Authors.

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

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NodeFeatureList contains a list of NodeFeature objects.
// +kubebuilder:object:root=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type NodeFeatureList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []NodeFeature `json:"items"`
}

// NodeFeature resource holds the features discovered for one node in the
// cluster.
// +kubebuilder:object:root=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +genclient
type NodeFeature struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec NodeFeatureSpec `json:"spec"`
}

// NodeFeatureSpec describes a NodeFeature object.
type NodeFeatureSpec struct {
	// Features is the full "raw" features data that has been discovered.
	// +optional
	Features Features `json:"features"`
	// Labels is the set of node labels that are requested to be created.
	// +optional
	Labels map[string]string `json:"labels"`
}

// Features is the collection of all discovered features.
//
// +protobuf=true
type Features struct {
	// Flags contains all the flag-type features of the node.
	// +optional
	Flags map[string]FlagFeatureSet `json:"flags" protobuf:"bytes,1,rep,name=flags"`
	// Attributes contains all the attribute-type features of the node.
	// +optional
	Attributes map[string]AttributeFeatureSet `json:"attributes" protobuf:"bytes,2,rep,name=vattributes"`
	// Instances contains all the instance-type features of the node.
	// +optional
	Instances map[string]InstanceFeatureSet `json:"instances" protobuf:"bytes,3,rep,name=instances"`
}

// FlagFeatureSet is a set of simple features only containing names without values.
//
// +protobuf=true
type FlagFeatureSet struct {
	Elements map[string]Nil `json:"elements" protobuf:"bytes,1,rep,name=elements"`
}

// AttributeFeatureSet is a set of features having string value.
//
// +protobuf=true
type AttributeFeatureSet struct {
	Elements map[string]string `json:"elements" protobuf:"bytes,1,rep,name=elements"`
}

// InstanceFeatureSet is a set of features each of which is an instance having multiple attributes.
//
// +protobuf=true
type InstanceFeatureSet struct {
	Elements []InstanceFeature `json:"elements" protobuf:"bytes,1,rep,name=elements"`
}

// InstanceFeature represents one instance of a complex features, e.g. a device.
//
// +protobuf=true
type InstanceFeature struct {
	Attributes map[string]string `json:"attributes" protobuf:"bytes,1,rep,name=attributes"`
}

// Nil is a dummy empty struct for protobuf compatibility
//
// +protobuf=true
type Nil struct{}

// NodeFeatureRuleList contains a list of NodeFeatureRule objects.
// +kubebuilder:object:root=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type NodeFeatureRuleList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []NodeFeatureRule `json:"items"`
}

// NodeFeatureRule resource specifies a configuration for feature-based
// customization of node objects, such as node labeling.
// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Cluster,shortName=nfr
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +genclient
// +genclient:nonNamespaced
type NodeFeatureRule struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec NodeFeatureRuleSpec `json:"spec"`
}

// NodeFeatureRuleSpec describes a NodeFeatureRule.
type NodeFeatureRuleSpec struct {
	// Rules is a list of node customization rules.
	Rules []Rule `json:"rules"`
}

// Rule defines a rule for node customization such as labeling.
type Rule struct {
	// Name of the rule.
	Name string `json:"name"`

	// Labels to create if the rule matches.
	// +optional
	Labels map[string]string `json:"labels"`

	// LabelsTemplate specifies a template to expand for dynamically generating
	// multiple labels. Data (after template expansion) must be keys with an
	// optional value (<key>[=<value>]) separated by newlines.
	// +optional
	LabelsTemplate string `json:"labelsTemplate"`

	// Vars is the variables to store if the rule matches. Variables do not
	// directly inflict any changes in the node object. However, they can be
	// referenced from other rules enabling more complex rule hierarchies,
	// without exposing intermediary output values as labels.
	// +optional
	Vars map[string]string `json:"vars"`

	// VarsTemplate specifies a template to expand for dynamically generating
	// multiple variables. Data (after template expansion) must be keys with an
	// optional value (<key>[=<value>]) separated by newlines.
	// +optional
	VarsTemplate string `json:"varsTemplate"`

	// Taints to create if the rule matches.
	// +optional
	Taints []corev1.Taint `json:"taints,omitempty"`

	// ExtendedResources to create if the rule matches.
	// +optional
	ExtendedResources map[string]string `json:"extendedResources"`

	// MatchFeatures specifies a set of matcher terms all of which must match.
	// +optional
	MatchFeatures FeatureMatcher `json:"matchFeatures"`

	// MatchAny specifies a list of matchers one of which must match.
	// +optional
	MatchAny []MatchAnyElem `json:"matchAny"`

	// private helpers/cache for handling golang templates
	labelsTemplate *templateHelper `json:"-"`
	varsTemplate   *templateHelper `json:"-"`
}

// MatchAnyElem specifies one sub-matcher of MatchAny.
type MatchAnyElem struct {
	// MatchFeatures specifies a set of matcher terms all of which must match.
	MatchFeatures FeatureMatcher `json:"matchFeatures"`
}

// FeatureMatcher specifies a set of feature matcher terms (i.e. per-feature
// matchers), all of which must match.
type FeatureMatcher []FeatureMatcherTerm

// FeatureMatcherTerm defines requirements against one feature set. All
// requirements (specified as MatchExpressions) are evaluated against each
// element in the feature set.
type FeatureMatcherTerm struct {
	Feature          string             `json:"feature"`
	MatchExpressions MatchExpressionSet `json:"matchExpressions"`
}

// MatchExpressionSet contains a set of MatchExpressions, each of which is
// evaluated against a set of input values.
type MatchExpressionSet map[string]*MatchExpression

// Expressions is a helper type to work around issues with k8s deepcopy-gen

// MatchExpression specifies an expression to evaluate against a set of input
// values. It contains an operator that is applied when matching the input and
// an array of values that the operator evaluates the input against.
//
// NB: CreateMatchExpression or MustCreateMatchExpression() should be used for
// creating new instances.
//
// NB: Validate() must be called if Op or Value fields are modified or if a new
// instance is created from scratch without using the helper functions.
type MatchExpression struct {
	// Op is the operator to be applied.
	Op MatchOp `json:"op"`

	// Value is the list of values that the operand evaluates the input
	// against. Value should be empty if the operator is Exists, DoesNotExist,
	// IsTrue or IsFalse. Value should contain exactly one element if the
	// operator is Gt or Lt and exactly two elements if the operator is GtLt.
	// In other cases Value should contain at least one element.
	// +optional
	Value MatchValue `json:"value,omitempty"`

	// valueRe caches compiled regexps for "InRegexp" operator
	valueRe valueRegexpCache `json:"-"`
}

// MatchOp is the match operator that is applied on values when evaluating a
// MatchExpression.
// +kubebuilder:validation:Enum="In";"NotIn";"InRegexp";"Exists";"DoesNotExist";"Gt";"Lt";"GtLt";"IsTrue";"IsFalse"
type MatchOp string

// MatchValue is the list of values associated with a MatchExpression.
type MatchValue []string

const (
	// MatchAny returns always true.
	MatchAny MatchOp = ""
	// MatchIn returns true if any of the values stored in the expression is
	// equal to the input.
	MatchIn MatchOp = "In"
	// MatchNotIn returns true if none of the values in the expression are
	// equal to the input.
	MatchNotIn MatchOp = "NotIn"
	// MatchInRegexp treats values of the expression as regular expressions and
	// returns true if any of them matches the input.
	MatchInRegexp MatchOp = "InRegexp"
	// MatchExists returns true if the input is valid. The expression must not
	// have any values.
	MatchExists MatchOp = "Exists"
	// MatchDoesNotExist returns true if the input is not valid. The expression
	// must not have any values.
	MatchDoesNotExist MatchOp = "DoesNotExist"
	// MatchGt returns true if the input is greater than the value of the
	// expression (number of values in the expression must be exactly one).
	// Both the input and value must be integer numbers, otherwise an error is
	// returned.
	MatchGt MatchOp = "Gt"
	// MatchLt returns true if the input is less  than the value of the
	// expression (number of values in the expression must be exactly one).
	// Both the input and value must be integer numbers, otherwise an error is
	// returned.
	MatchLt MatchOp = "Lt"
	// MatchGtLt returns true if the input is between two values, i.e. greater
	// than the first value and less than the second value of the expression
	// (number of values in the expression must be exactly two). Both the input
	// and values must be integer numbers, otherwise an error is returned.
	MatchGtLt MatchOp = "GtLt"
	// MatchIsTrue returns true if the input holds the value "true". The
	// expression must not have any values.
	MatchIsTrue MatchOp = "IsTrue"
	// MatchIsFalse returns true if the input holds the value "false". The
	// expression must not have any values.
	MatchIsFalse MatchOp = "IsFalse"
)

const (
	// RuleBackrefDomain is the special feature domain for backreferencing
	// output of preceding rules.
	RuleBackrefDomain = "rule"
	// RuleBackrefFeature is the special feature name for backreferencing
	// output of preceding rules.
	RuleBackrefFeature = "matched"
)
