package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
)

type ECSCluster struct {
	Name                         string
	Arn                          string
	Status                       string
	RunningTasksCount            int32
	PendingTasksCount            int32
	ActiveServicesCount          int32
	RegisteredContainerInstances int32
	Services                     []ECSService
}

type ECSService struct {
	Name           string
	Status         string
	DesiredCount   int32
	RunningCount   int32
	PendingCount   int32
	TaskDefinition string
	LaunchType     string
}

func (c *Client) ListECSClusters(ctx context.Context) ([]ECSCluster, error) {
	var clusters []ECSCluster
	paginator := ecs.NewListClustersPaginator(c.ECS, &ecs.ListClustersInput{
		MaxResults: aws.Int32(100),
	})

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list ECS clusters: %w", err)
		}

		if len(page.ClusterArns) == 0 {
			continue
		}

		descOutput, err := c.ECS.DescribeClusters(ctx, &ecs.DescribeClustersInput{
			Clusters: page.ClusterArns,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to describe ECS clusters: %w", err)
		}

		for _, cluster := range descOutput.Clusters {
			ecsCluster := ECSCluster{
				Name:                         aws.ToString(cluster.ClusterName),
				Arn:                          aws.ToString(cluster.ClusterArn),
				Status:                       aws.ToString(cluster.Status),
				RunningTasksCount:            cluster.RunningTasksCount,
				PendingTasksCount:            cluster.PendingTasksCount,
				ActiveServicesCount:          cluster.ActiveServicesCount,
				RegisteredContainerInstances: cluster.RegisteredContainerInstancesCount,
			}

			// Fetch services for this cluster
			services, _ := c.listECSServices(ctx, ecsCluster.Arn)
			ecsCluster.Services = services

			clusters = append(clusters, ecsCluster)
		}
	}

	return clusters, nil
}

func (c *Client) listECSServices(ctx context.Context, clusterArn string) ([]ECSService, error) {
	var services []ECSService
	paginator := ecs.NewListServicesPaginator(c.ECS, &ecs.ListServicesInput{
		Cluster:    aws.String(clusterArn),
		MaxResults: aws.Int32(100),
	})

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}

		if len(page.ServiceArns) == 0 {
			continue
		}

		descOutput, err := c.ECS.DescribeServices(ctx, &ecs.DescribeServicesInput{
			Cluster:  aws.String(clusterArn),
			Services: page.ServiceArns,
		})
		if err != nil {
			return nil, err
		}

		for _, svc := range descOutput.Services {
			service := ECSService{
				Name:           aws.ToString(svc.ServiceName),
				Status:         aws.ToString(svc.Status),
				DesiredCount:   svc.DesiredCount,
				RunningCount:   svc.RunningCount,
				PendingCount:   svc.PendingCount,
				TaskDefinition: aws.ToString(svc.TaskDefinition),
				LaunchType:     string(svc.LaunchType),
			}
			services = append(services, service)
		}
	}

	return services, nil
}

func (cluster *ECSCluster) StatusColor() string {
	switch cluster.Status {
	case "ACTIVE":
		return "green"
	case "PROVISIONING":
		return "yellow"
	case "DEPROVISIONING", "FAILED", "INACTIVE":
		return "red"
	default:
		return "gray"
	}
}

func (cluster *ECSCluster) DisplayLine() string {
	return fmt.Sprintf("%-30s %-10s %d tasks, %d services",
		truncate(cluster.Name, 30),
		cluster.Status,
		cluster.RunningTasksCount,
		cluster.ActiveServicesCount)
}

func (cluster *ECSCluster) DetailLines() []string {
	lines := []string{
		fmt.Sprintf("Cluster Name:       %s", cluster.Name),
		fmt.Sprintf("Status:             %s", cluster.Status),
		fmt.Sprintf("Running Tasks:      %d", cluster.RunningTasksCount),
		fmt.Sprintf("Pending Tasks:      %d", cluster.PendingTasksCount),
		fmt.Sprintf("Active Services:    %d", cluster.ActiveServicesCount),
		fmt.Sprintf("Container Instances: %d", cluster.RegisteredContainerInstances),
		"",
		"Services:",
	}

	for _, svc := range cluster.Services {
		lines = append(lines,
			fmt.Sprintf("  %s:", svc.Name),
			fmt.Sprintf("    Status: %s, Launch: %s", svc.Status, svc.LaunchType),
			fmt.Sprintf("    Tasks: %d/%d (running/desired)", svc.RunningCount, svc.DesiredCount),
		)
	}

	return lines
}
