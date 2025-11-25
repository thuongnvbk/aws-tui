package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/eks"
)

type EKSCluster struct {
	Name            string
	Status          string
	Version         string
	Endpoint        string
	RoleArn         string
	VpcID           string
	PlatformVersion string
	CreatedAt       string
	NodeGroups      []EKSNodeGroup
	Tags            map[string]string
}

type EKSNodeGroup struct {
	Name         string
	Status       string
	InstanceType string
	DesiredSize  int32
	MinSize      int32
	MaxSize      int32
	AmiType      string
}

func (c *Client) ListEKSClusters(ctx context.Context) ([]EKSCluster, error) {
	var clusters []EKSCluster
	paginator := eks.NewListClustersPaginator(c.EKS, &eks.ListClustersInput{})

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list EKS clusters: %w", err)
		}

		for _, clusterName := range page.Clusters {
			descOutput, err := c.EKS.DescribeCluster(ctx, &eks.DescribeClusterInput{
				Name: aws.String(clusterName),
			})
			if err != nil {
				continue
			}

			cluster := descOutput.Cluster
			eksCluster := EKSCluster{
				Name:            aws.ToString(cluster.Name),
				Status:          string(cluster.Status),
				Version:         aws.ToString(cluster.Version),
				Endpoint:        aws.ToString(cluster.Endpoint),
				RoleArn:         aws.ToString(cluster.RoleArn),
				PlatformVersion: aws.ToString(cluster.PlatformVersion),
				Tags:            cluster.Tags,
			}

			if cluster.ResourcesVpcConfig != nil {
				eksCluster.VpcID = aws.ToString(cluster.ResourcesVpcConfig.VpcId)
			}

			if cluster.CreatedAt != nil {
				eksCluster.CreatedAt = cluster.CreatedAt.Format(c.dateFmt)
			}

			// Fetch node groups
			nodeGroups, _ := c.listNodeGroups(ctx, clusterName)
			eksCluster.NodeGroups = nodeGroups

			clusters = append(clusters, eksCluster)
		}
	}

	return clusters, nil
}

func (c *Client) listNodeGroups(ctx context.Context, clusterName string) ([]EKSNodeGroup, error) {
	var nodeGroups []EKSNodeGroup
	paginator := eks.NewListNodegroupsPaginator(c.EKS, &eks.ListNodegroupsInput{
		ClusterName: aws.String(clusterName),
	})

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}

		for _, ngName := range page.Nodegroups {
			descOutput, err := c.EKS.DescribeNodegroup(ctx, &eks.DescribeNodegroupInput{
				ClusterName:   aws.String(clusterName),
				NodegroupName: aws.String(ngName),
			})
			if err != nil {
				continue
			}

			ng := descOutput.Nodegroup
			nodeGroup := EKSNodeGroup{
				Name:    aws.ToString(ng.NodegroupName),
				Status:  string(ng.Status),
				AmiType: string(ng.AmiType),
			}

			if ng.ScalingConfig != nil {
				nodeGroup.DesiredSize = aws.ToInt32(ng.ScalingConfig.DesiredSize)
				nodeGroup.MinSize = aws.ToInt32(ng.ScalingConfig.MinSize)
				nodeGroup.MaxSize = aws.ToInt32(ng.ScalingConfig.MaxSize)
			}

			if len(ng.InstanceTypes) > 0 {
				nodeGroup.InstanceType = ng.InstanceTypes[0]
			}

			nodeGroups = append(nodeGroups, nodeGroup)
		}
	}

	return nodeGroups, nil
}

func (cluster *EKSCluster) StatusColor() string {
	switch cluster.Status {
	case "ACTIVE":
		return "green"
	case "CREATING", "UPDATING":
		return "yellow"
	case "DELETING", "FAILED":
		return "red"
	default:
		return "gray"
	}
}

func (cluster *EKSCluster) DisplayLine() string {
	ngCount := len(cluster.NodeGroups)
	return fmt.Sprintf("%-30s %-10s %-8s %d node groups",
		truncate(cluster.Name, 30),
		cluster.Status,
		cluster.Version,
		ngCount)
}

func (cluster *EKSCluster) DetailLines() []string {
	lines := []string{
		fmt.Sprintf("Cluster Name:      %s", cluster.Name),
		fmt.Sprintf("Status:            %s", cluster.Status),
		fmt.Sprintf("Version:           %s", cluster.Version),
		fmt.Sprintf("Platform Version:  %s", cluster.PlatformVersion),
		fmt.Sprintf("Endpoint:          %s", cluster.Endpoint),
		fmt.Sprintf("VPC ID:            %s", cluster.VpcID),
		fmt.Sprintf("Role ARN:          %s", cluster.RoleArn),
		fmt.Sprintf("Created At:        %s", cluster.CreatedAt),
		"",
		"Node Groups:",
	}

	for _, ng := range cluster.NodeGroups {
		lines = append(lines,
			fmt.Sprintf("  %s:", ng.Name),
			fmt.Sprintf("    Status: %s, Type: %s", ng.Status, ng.InstanceType),
			fmt.Sprintf("    Scaling: %d/%d/%d (min/desired/max)", ng.MinSize, ng.DesiredSize, ng.MaxSize),
		)
	}

	if len(cluster.Tags) > 0 {
		lines = append(lines, "", "Tags:")
		for k, v := range cluster.Tags {
			lines = append(lines, fmt.Sprintf("  %s: %s", k, v))
		}
	}

	return lines
}
