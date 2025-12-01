package aws

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// S3 Bucket
type S3Bucket struct {
	Name         string
	CreationDate string
	Region       string
	Versioning   bool
	Encryption   bool
}

func (c *Client) ListS3Buckets(ctx context.Context) ([]S3Bucket, error) {
	output, err := c.S3.ListBuckets(ctx, &s3.ListBucketsInput{})
	if err != nil {
		return nil, fmt.Errorf("failed to list S3 buckets: %w", err)
	}

	var buckets []S3Bucket
	for _, b := range output.Buckets {
		bucket := S3Bucket{
			Name: aws.ToString(b.Name),
		}
		if b.CreationDate != nil {
			bucket.CreationDate = b.CreationDate.Format(c.dateFmt)
		}
		buckets = append(buckets, bucket)
	}

	return buckets, nil
}

func (bucket *S3Bucket) DisplayLine() string {
	return fmt.Sprintf("%-50s %s", truncate(bucket.Name, 50), bucket.CreationDate)
}

func (bucket *S3Bucket) DetailLines() []string {
	return []string{
		fmt.Sprintf("Bucket Name:   %s", bucket.Name),
		fmt.Sprintf("Created:       %s", bucket.CreationDate),
	}
}

// RDS Instance
type RDSInstance struct {
	Identifier       string
	Engine           string
	EngineVersion    string
	Status           string
	InstanceClass    string
	Endpoint         string
	Port             int32
	MultiAZ          bool
	StorageType      string
	AllocatedStorage int32
	VpcID            string
	AvailabilityZone string
}

func (c *Client) ListRDSInstances(ctx context.Context) ([]RDSInstance, error) {
	paginator := rds.NewDescribeDBInstancesPaginator(c.RDS, &rds.DescribeDBInstancesInput{})

	var instances []RDSInstance
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to describe RDS instances: %w", err)
		}

		for _, db := range page.DBInstances {
			instance := RDSInstance{
				Identifier:       aws.ToString(db.DBInstanceIdentifier),
				Engine:           aws.ToString(db.Engine),
				EngineVersion:    aws.ToString(db.EngineVersion),
				Status:           aws.ToString(db.DBInstanceStatus),
				InstanceClass:    aws.ToString(db.DBInstanceClass),
				MultiAZ:          aws.ToBool(db.MultiAZ),
				StorageType:      aws.ToString(db.StorageType),
				AllocatedStorage: aws.ToInt32(db.AllocatedStorage),
				AvailabilityZone: aws.ToString(db.AvailabilityZone),
			}

			if db.Endpoint != nil {
				instance.Endpoint = aws.ToString(db.Endpoint.Address)
				instance.Port = aws.ToInt32(db.Endpoint.Port)
			}

			if db.DBSubnetGroup != nil {
				instance.VpcID = aws.ToString(db.DBSubnetGroup.VpcId)
			}

			instances = append(instances, instance)
		}
	}

	return instances, nil
}

func (inst *RDSInstance) StatusColor() string {
	switch inst.Status {
	case "available":
		return "green"
	case "creating", "modifying", "backing-up":
		return "yellow"
	case "stopped", "stopping", "deleting", "failed":
		return "red"
	default:
		return "gray"
	}
}

func (inst *RDSInstance) DisplayLine() string {
	return fmt.Sprintf("%-30s %-12s %-15s %-15s",
		truncate(inst.Identifier, 30),
		inst.Status,
		inst.Engine,
		inst.InstanceClass)
}

func (inst *RDSInstance) DetailLines() []string {
	multiAZ := "No"
	if inst.MultiAZ {
		multiAZ = "Yes"
	}
	return []string{
		fmt.Sprintf("Identifier:      %s", inst.Identifier),
		fmt.Sprintf("Status:          %s", inst.Status),
		fmt.Sprintf("Engine:          %s %s", inst.Engine, inst.EngineVersion),
		fmt.Sprintf("Instance Class:  %s", inst.InstanceClass),
		fmt.Sprintf("Endpoint:        %s:%d", inst.Endpoint, inst.Port),
		fmt.Sprintf("Storage:         %d GB (%s)", inst.AllocatedStorage, inst.StorageType),
		fmt.Sprintf("Multi-AZ:        %s", multiAZ),
		fmt.Sprintf("Availability Zone: %s", inst.AvailabilityZone),
		fmt.Sprintf("VPC ID:          %s", inst.VpcID),
	}
}

// Lambda Function
type LambdaFunction struct {
	Name         string
	Runtime      string
	Handler      string
	CodeSize     int64
	MemorySize   int32
	Timeout      int32
	LastModified string
	Description  string
	State        string
	Role         string
}

func (c *Client) ListLambdaFunctions(ctx context.Context) ([]LambdaFunction, error) {
	var functions []LambdaFunction
	var marker *string

	for {
		output, err := c.Lambda.ListFunctions(ctx, &lambda.ListFunctionsInput{
			Marker: marker,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to list Lambda functions: %w", err)
		}

		for _, fn := range output.Functions {
			function := LambdaFunction{
				Name:         aws.ToString(fn.FunctionName),
				Runtime:      string(fn.Runtime),
				Handler:      aws.ToString(fn.Handler),
				CodeSize:     fn.CodeSize,
				MemorySize:   aws.ToInt32(fn.MemorySize),
				Timeout:      aws.ToInt32(fn.Timeout),
				LastModified: aws.ToString(fn.LastModified),
				Description:  aws.ToString(fn.Description),
				State:        string(fn.State),
				Role:         aws.ToString(fn.Role),
			}
			functions = append(functions, function)
		}

		if output.NextMarker == nil {
			break
		}
		marker = output.NextMarker
	}

	return functions, nil
}

func (fn *LambdaFunction) StateColor() string {
	switch fn.State {
	case "Active":
		return "green"
	case "Pending":
		return "yellow"
	case "Inactive", "Failed":
		return "red"
	default:
		return "gray"
	}
}

func (fn *LambdaFunction) DisplayLine() string {
	return fmt.Sprintf("%-40s %-15s %-10s %dMB",
		truncate(fn.Name, 40),
		fn.Runtime,
		fn.State,
		fn.MemorySize)
}

func (fn *LambdaFunction) DetailLines() []string {
	lines := []string{
		fmt.Sprintf("Function Name: %s", fn.Name),
		fmt.Sprintf("State:         %s", fn.State),
		fmt.Sprintf("Runtime:       %s", fn.Runtime),
		fmt.Sprintf("Handler:       %s", fn.Handler),
		fmt.Sprintf("Memory:        %d MB", fn.MemorySize),
		fmt.Sprintf("Timeout:       %d seconds", fn.Timeout),
		fmt.Sprintf("Code Size:     %.2f KB", float64(fn.CodeSize)/1024),
		fmt.Sprintf("Last Modified: %s", fn.LastModified),
		fmt.Sprintf("Role:          %s", truncateRole(fn.Role)),
	}
	if fn.Description != "" {
		lines = append(lines, fmt.Sprintf("Description:   %s", fn.Description))
	}
	return lines
}

func truncateRole(arn string) string {
	parts := strings.Split(arn, "/")
	if len(parts) > 1 {
		return parts[len(parts)-1]
	}
	return arn
}
