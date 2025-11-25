package aws

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

type EC2Instance struct {
	ID             string
	Name           string
	State          string
	Type           string
	PrivateIP      string
	PublicIP       string
	VpcID          string
	SubnetID       string
	KeyName        string
	LaunchTime     string
	SecurityGroups []string
	Tags           map[string]string
}

func (c *Client) ListEC2Instances(ctx context.Context, filters []types.Filter) ([]EC2Instance, error) {
	input := &ec2.DescribeInstancesInput{}
	if len(filters) > 0 {
		input.Filters = filters
	}

	var instances []EC2Instance
	paginator := ec2.NewDescribeInstancesPaginator(c.EC2, input)

	for paginator.HasMorePages() {
		output, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to describe instances: %w", err)
		}

		for _, reservation := range output.Reservations {
			for _, inst := range reservation.Instances {
				instance := EC2Instance{
					ID:        aws.ToString(inst.InstanceId),
					State:     string(inst.State.Name),
					Type:      string(inst.InstanceType),
					PrivateIP: aws.ToString(inst.PrivateIpAddress),
					PublicIP:  aws.ToString(inst.PublicIpAddress),
					VpcID:     aws.ToString(inst.VpcId),
					SubnetID:  aws.ToString(inst.SubnetId),
					KeyName:   aws.ToString(inst.KeyName),
					Tags:      make(map[string]string),
				}

				if inst.LaunchTime != nil {
					instance.LaunchTime = inst.LaunchTime.Format(c.dateFmt)
				}

				for _, sg := range inst.SecurityGroups {
					instance.SecurityGroups = append(instance.SecurityGroups, aws.ToString(sg.GroupId))
				}

				for _, tag := range inst.Tags {
					key := aws.ToString(tag.Key)
					value := aws.ToString(tag.Value)
					instance.Tags[key] = value
					if key == "Name" {
						instance.Name = value
					}
				}

				if instance.Name == "" {
					instance.Name = instance.ID
				}

				instances = append(instances, instance)
			}
		}
	}

	return instances, nil
}

func (inst *EC2Instance) StateColor() string {
	switch inst.State {
	case "running":
		return "green"
	case "stopped":
		return "red"
	case "pending", "stopping":
		return "yellow"
	default:
		return "gray"
	}
}

func (inst *EC2Instance) DisplayLine() string {
	ip := inst.PrivateIP
	if inst.PublicIP != "" {
		ip = inst.PublicIP
	}
	return fmt.Sprintf("%-20s %-12s %-15s %-15s",
		truncate(inst.Name, 20),
		inst.State,
		inst.Type,
		ip)
}

func (inst *EC2Instance) DetailLines() []string {
	lines := []string{
		fmt.Sprintf("Instance ID:     %s", inst.ID),
		fmt.Sprintf("Name:            %s", inst.Name),
		fmt.Sprintf("State:           %s", inst.State),
		fmt.Sprintf("Instance Type:   %s", inst.Type),
		fmt.Sprintf("Private IP:      %s", inst.PrivateIP),
		fmt.Sprintf("Public IP:       %s", inst.PublicIP),
		fmt.Sprintf("VPC ID:          %s", inst.VpcID),
		fmt.Sprintf("Subnet ID:       %s", inst.SubnetID),
		fmt.Sprintf("Key Name:        %s", inst.KeyName),
		fmt.Sprintf("Launch Time:     %s", inst.LaunchTime),
		fmt.Sprintf("Security Groups: %s", strings.Join(inst.SecurityGroups, ", ")),
		"",
		"Tags:",
	}
	for k, v := range inst.Tags {
		lines = append(lines, fmt.Sprintf("  %s: %s", k, v))
	}
	return lines
}

func truncate(s string, length int) string {
	if len(s) <= length {
		return s
	}
	return s[:length-3] + "..."
}
