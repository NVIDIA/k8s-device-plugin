package aws

import (
	"fmt"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go/service/route53"
	"github.com/gogo/protobuf/proto"
	"github.com/stretchr/testify/require"
)

// GetRoute53Record returns a Route 53 Record
func GetRoute53Record(t *testing.T, hostedZoneID, recordName, recordType, awsRegion string) *route53.ResourceRecordSet {
	r, err := GetRoute53RecordE(t, hostedZoneID, recordName, recordType, awsRegion)
	require.NoError(t, err)

	return r
}

// GetRoute53RecordE returns a Route 53 Record
func GetRoute53RecordE(t *testing.T, hostedZoneID, recordName, recordType, awsRegion string) (record *route53.ResourceRecordSet, err error) {
	route53Client, err := NewRoute53ClientE(t, awsRegion)
	if err != nil {
		return nil, err
	}

	o, err := route53Client.ListResourceRecordSets(&route53.ListResourceRecordSetsInput{
		HostedZoneId:    &hostedZoneID,
		StartRecordName: &recordName,
		StartRecordType: &recordType,
		MaxItems:        proto.String("1"),
	})
	if err != nil {
		return
	}
	for _, record = range o.ResourceRecordSets {
		if strings.EqualFold(recordName+".", *record.Name) {
			break
		}
		record = nil
	}
	if record == nil {
		err = fmt.Errorf("record not found")
	}
	return
}

// NewRoute53ClientE creates a route 53 client.
func NewRoute53Client(t *testing.T, region string) *route53.Route53 {
	c, err := NewRoute53ClientE(t, region)
	require.NoError(t, err)

	return c
}

// NewRoute53ClientE creates a route 53 client.
func NewRoute53ClientE(t *testing.T, region string) (*route53.Route53, error) {
	sess, err := NewAuthenticatedSession(region)
	if err != nil {
		return nil, err
	}

	return route53.New(sess), nil
}
