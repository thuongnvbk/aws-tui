package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	"github.com/aws/aws-sdk-go-v2/service/kafka"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
)

type Client struct {
	cfg            aws.Config
	Profile        string
	Region         string
	dateFmt        string
	EC2            *ec2.Client
	EKS            *eks.Client
	ECS            *ecs.Client
	S3             *s3.Client
	RDS            *rds.Client
	Lambda         *lambda.Client
	MSK            *kafka.Client
	SecretsManager *secretsmanager.Client
}

func NewClient(ctx context.Context, profile, region, dateFmt string) (*Client, error) {
	opts := []func(*awsconfig.LoadOptions) error{
		awsconfig.WithRegion(region),
	}

	if profile != "" && profile != "default" {
		opts = append(opts, awsconfig.WithSharedConfigProfile(profile))
	}

	cfg, err := awsconfig.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	if dateFmt == "" {
		dateFmt = "2006-01-02 15:04:05"
	}

	return &Client{
		cfg:            cfg,
		Profile:        profile,
		Region:         region,
		dateFmt:        dateFmt,
		EC2:            ec2.NewFromConfig(cfg),
		EKS:            eks.NewFromConfig(cfg),
		ECS:            ecs.NewFromConfig(cfg),
		S3:             s3.NewFromConfig(cfg),
		RDS:            rds.NewFromConfig(cfg),
		Lambda:         lambda.NewFromConfig(cfg),
		MSK:            kafka.NewFromConfig(cfg),
		SecretsManager: secretsmanager.NewFromConfig(cfg),
	}, nil
}

func (c *Client) SwitchProfile(ctx context.Context, profile, region string) error {
	newClient, err := NewClient(ctx, profile, region, c.dateFmt)
	if err != nil {
		return err
	}
	*c = *newClient
	return nil
}

func (c *Client) SwitchRegion(ctx context.Context, region string) error {
	return c.SwitchProfile(ctx, c.Profile, region)
}

func (c *Client) DateFormat() string {
	return c.dateFmt
}
