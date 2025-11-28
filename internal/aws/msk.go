package aws

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/kafka"
	"github.com/aws/aws-sdk-go-v2/service/kafka/types"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	kafkago "github.com/segmentio/kafka-go"
	"github.com/segmentio/kafka-go/sasl/scram"
)

type MSKCluster struct {
	Name                 string
	Arn                  string
	State                string
	KafkaVersion         string
	BrokerNodes          int32
	CreationTime         string
	EncryptionIn         string
	Monitoring           string
	ClientAuth           string
	BootstrapBrokers     string
	BootstrapBrokersSASL string
	BootstrapBrokersTLS  string
	SecretArns           []string
	BrokerNodeDetails    []BrokerNode
	Topics               []KafkaTopic
}

type KafkaTopic struct {
	Name       string
	Partitions int
	Replicas   int
}

type BrokerNode struct {
	ID           string
	ClientSubnet string
	Endpoint     string
}

func (c *Client) ListMSKClusters(ctx context.Context) ([]MSKCluster, error) {
	paginator := kafka.NewListClustersPaginator(c.MSK, &kafka.ListClustersInput{
		MaxResults: aws.Int32(100),
	})

	var clusters []MSKCluster
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			// Check for endpoint resolution errors (MSK not available in region)
			var nf *aws.EndpointNotFoundError
			if errors.As(err, &nf) {
				// Return empty list instead of error - MSK is not available in this region
				return []MSKCluster{}, nil
			}
			// Check specifically for ResolveEndpointV2 errors
			errStr := err.Error()
			if strings.Contains(errStr, "operation error Kafka: ListClusters") && strings.Contains(errStr, "ResolveEndpointV2") {
				// Return empty list instead of error - MSK endpoint resolution failed
				return []MSKCluster{}, nil
			}
			return nil, fmt.Errorf("failed to list MSK clusters: %w", err)
		}

		for _, info := range page.ClusterInfoList {
			cluster := MSKCluster{
				Name:  aws.ToString(info.ClusterName),
				Arn:   aws.ToString(info.ClusterArn),
				State: string(info.State),
				KafkaVersion: func() string {
					if info.CurrentBrokerSoftwareInfo != nil {
						return aws.ToString(info.CurrentBrokerSoftwareInfo.KafkaVersion)
					}
					return ""
				}(),
				BrokerNodes: aws.ToInt32(info.NumberOfBrokerNodes),
			}

			if info.CreationTime != nil {
				cluster.CreationTime = info.CreationTime.Format(c.dateFmt)
			}

			if info.EncryptionInfo != nil && info.EncryptionInfo.EncryptionAtRest != nil {
				cluster.EncryptionIn = aws.ToString(info.EncryptionInfo.EncryptionAtRest.DataVolumeKMSKeyId)
			}

			if info.ClientAuthentication != nil {
				cluster.ClientAuth = summarizeClientAuth(info.ClientAuthentication)
			}

			if info.OpenMonitoring != nil && info.OpenMonitoring.Prometheus != nil {
				cluster.Monitoring = summarizeMonitoring(info.OpenMonitoring.Prometheus)
			}

			// Fetch bootstrap brokers
			clusterArn := aws.ToString(info.ClusterArn)
			if brokers, err := c.getBootstrapBrokers(ctx, clusterArn); err == nil {
				cluster.BootstrapBrokers = brokers.Plain
				cluster.BootstrapBrokersSASL = brokers.SASL
				cluster.BootstrapBrokersTLS = brokers.TLS
			}

			// Fetch SCRAM secrets
			if secrets, err := c.listScramSecrets(ctx, clusterArn); err == nil {
				cluster.SecretArns = secrets
			}

			// Fetch broker node details
			if nodes, err := c.listBrokerNodes(ctx, clusterArn); err == nil {
				cluster.BrokerNodeDetails = nodes
			}

			clusters = append(clusters, cluster)
		}
	}

	return clusters, nil
}

type bootstrapBrokers struct {
	Plain string
	SASL  string
	TLS   string
}

func (c *Client) getBootstrapBrokers(ctx context.Context, clusterArn string) (*bootstrapBrokers, error) {
	output, err := c.MSK.GetBootstrapBrokers(ctx, &kafka.GetBootstrapBrokersInput{
		ClusterArn: aws.String(clusterArn),
	})
	if err != nil {
		return nil, err
	}

	return &bootstrapBrokers{
		Plain: aws.ToString(output.BootstrapBrokerString),
		SASL:  aws.ToString(output.BootstrapBrokerStringSaslScram),
		TLS:   aws.ToString(output.BootstrapBrokerStringTls),
	}, nil
}

func (c *Client) listScramSecrets(ctx context.Context, clusterArn string) ([]string, error) {
	output, err := c.MSK.ListScramSecrets(ctx, &kafka.ListScramSecretsInput{
		ClusterArn: aws.String(clusterArn),
		MaxResults: aws.Int32(100),
	})
	if err != nil {
		return nil, err
	}

	return output.SecretArnList, nil
}

func (c *Client) listBrokerNodes(ctx context.Context, clusterArn string) ([]BrokerNode, error) {
	output, err := c.MSK.ListNodes(ctx, &kafka.ListNodesInput{
		ClusterArn: aws.String(clusterArn),
		MaxResults: aws.Int32(100),
	})
	if err != nil {
		return nil, err
	}

	var nodes []BrokerNode
	for _, nodeInfo := range output.NodeInfoList {
		if nodeInfo.BrokerNodeInfo != nil {
			broker := nodeInfo.BrokerNodeInfo
			node := BrokerNode{
				ID: fmt.Sprintf("%.0f", aws.ToFloat64(broker.BrokerId)),
			}

			if broker.ClientSubnet != nil {
				node.ClientSubnet = aws.ToString(broker.ClientSubnet)
			}

			if broker.Endpoints != nil && len(broker.Endpoints) > 0 {
				node.Endpoint = broker.Endpoints[0]
			}

			nodes = append(nodes, node)
		}
	}

	return nodes, nil
}

func summarizeClientAuth(auth *types.ClientAuthentication) string {
	var modes []string
	if auth.Sasl != nil {
		if auth.Sasl.Iam != nil && auth.Sasl.Iam.Enabled != nil && aws.ToBool(auth.Sasl.Iam.Enabled) {
			modes = append(modes, "IAM")
		}
		if auth.Sasl.Scram != nil && auth.Sasl.Scram.Enabled != nil && aws.ToBool(auth.Sasl.Scram.Enabled) {
			modes = append(modes, "SCRAM")
		}
	}
	if auth.Tls != nil && auth.Tls.CertificateAuthorityArnList != nil && len(auth.Tls.CertificateAuthorityArnList) > 0 {
		modes = append(modes, "TLS")
	}
	if auth.Unauthenticated != nil && auth.Unauthenticated.Enabled != nil && aws.ToBool(auth.Unauthenticated.Enabled) {
		modes = append(modes, "Unauth")
	}
	return strings.Join(modes, ",")
}

func summarizeMonitoring(prom *types.Prometheus) string {
	modes := []string{}
	if prom.JmxExporter != nil && prom.JmxExporter.EnabledInBroker != nil && aws.ToBool(prom.JmxExporter.EnabledInBroker) {
		modes = append(modes, "JMX")
	}
	if prom.NodeExporter != nil && prom.NodeExporter.EnabledInBroker != nil && aws.ToBool(prom.NodeExporter.EnabledInBroker) {
		modes = append(modes, "NodeExporter")
	}
	return strings.Join(modes, ",")
}

func (cluster *MSKCluster) StatusColor() string {
	switch cluster.State {
	case "ACTIVE":
		return "green"
	case "CREATING", "UPDATING", "HEALING":
		return "yellow"
	case "DELETING", "FAILED":
		return "red"
	default:
		return "gray"
	}
}

func (cluster *MSKCluster) DisplayLine() string {
	return fmt.Sprintf("%-26s %-10s v%-6s %2d brokers",
		truncate(cluster.Name, 26),
		cluster.State,
		cluster.KafkaVersion,
		cluster.BrokerNodes)
}

func (cluster *MSKCluster) DetailLines() []string {
	lines := []string{
		fmt.Sprintf("Cluster Name:    %s", cluster.Name),
		fmt.Sprintf("State:           %s", cluster.State),
		fmt.Sprintf("Kafka Version:   %s", cluster.KafkaVersion),
		fmt.Sprintf("Brokers:         %d", cluster.BrokerNodes),
		fmt.Sprintf("Created:         %s", cluster.CreationTime),
		fmt.Sprintf("Client Auth:     %s", cluster.ClientAuth),
		fmt.Sprintf("Monitoring:      %s", cluster.Monitoring),
		fmt.Sprintf("KMS Key:         %s", cluster.EncryptionIn),
		"",
		"=== Connection Information ===",
	}

	// Bootstrap brokers
	if cluster.BootstrapBrokersSASL != "" {
		lines = append(lines, fmt.Sprintf("Bootstrap (SASL): %s", cluster.BootstrapBrokersSASL))
	}
	if cluster.BootstrapBrokersTLS != "" {
		lines = append(lines, fmt.Sprintf("Bootstrap (TLS):  %s", cluster.BootstrapBrokersTLS))
	}
	if cluster.BootstrapBrokers != "" {
		lines = append(lines, fmt.Sprintf("Bootstrap (Plain): %s", cluster.BootstrapBrokers))
	}

	// SASL SCRAM Secrets
	if len(cluster.SecretArns) > 0 {
		lines = append(lines, "", "=== SASL SCRAM Secrets ===")
		for i, secretArn := range cluster.SecretArns {
			// Extract secret name from ARN
			secretName := extractSecretName(secretArn)
			lines = append(lines, fmt.Sprintf("Secret %d: %s", i+1, secretName))
			lines = append(lines, fmt.Sprintf("  ARN: %s", secretArn))
		}
	}

	// Broker node details
	if len(cluster.BrokerNodeDetails) > 0 {
		lines = append(lines, "", "=== Broker Nodes ===")
		for _, node := range cluster.BrokerNodeDetails {
			lines = append(lines, fmt.Sprintf("Broker ID %s:", node.ID))
			if node.Endpoint != "" {
				lines = append(lines, fmt.Sprintf("  Endpoint: %s", node.Endpoint))
			}
			if node.ClientSubnet != "" {
				lines = append(lines, fmt.Sprintf("  Subnet:   %s", node.ClientSubnet))
			}
		}
	}

	// Kafka topics
	if len(cluster.Topics) > 0 {
		lines = append(lines, "", fmt.Sprintf("=== Kafka Topics (%d) ===", len(cluster.Topics)))
		for _, topic := range cluster.Topics {
			lines = append(lines, fmt.Sprintf("%-40s  %2d partitions  %d replicas",
				truncate(topic.Name, 40),
				topic.Partitions,
				topic.Replicas))
		}
	} else {
		lines = append(lines, "", "=== Kafka Topics ===")
		lines = append(lines, "Press 't' to fetch topics (requires SASL credentials)")
	}

	lines = append(lines, "", "=== ARN ===", cluster.Arn)

	return lines
}

func extractSecretName(arn string) string {
	// Extract the secret name from ARN format: arn:aws:secretsmanager:region:account:secret:name-XXXXX
	parts := strings.Split(arn, ":")
	if len(parts) >= 7 {
		return parts[6]
	}
	return arn
}

// MSKCredentials represents the SASL/SCRAM credentials stored in Secrets Manager
type MSKCredentials struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// GetMSKCredentials retrieves SASL/SCRAM credentials from AWS Secrets Manager
func (c *Client) GetMSKCredentials(ctx context.Context, secretArn string) (*MSKCredentials, error) {
	output, err := c.SecretsManager.GetSecretValue(ctx, &secretsmanager.GetSecretValueInput{
		SecretId: aws.String(secretArn),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve secret: %w", err)
	}

	if output.SecretString == nil {
		return nil, fmt.Errorf("secret string is empty")
	}

	var creds MSKCredentials
	if err := json.Unmarshal([]byte(*output.SecretString), &creds); err != nil {
		return nil, fmt.Errorf("failed to parse secret JSON: %w", err)
	}

	return &creds, nil
}

// ListTopics connects to the Kafka cluster and retrieves topic metadata
// Note: This requires proper authentication credentials
func (c *Client) ListTopics(ctx context.Context, cluster *MSKCluster, username, password string) ([]KafkaTopic, error) {
	if cluster.BootstrapBrokersSASL == "" {
		return nil, fmt.Errorf("no bootstrap brokers available")
	}

	// Parse bootstrap brokers
	brokers := strings.Split(cluster.BootstrapBrokersSASL, ",")
	if len(brokers) == 0 {
		return nil, fmt.Errorf("no bootstrap brokers found")
	}

	// Create SASL SCRAM mechanism if credentials provided
	var dialer *kafkago.Dialer
	if username != "" && password != "" {
		mechanism, err := scram.Mechanism(scram.SHA512, username, password)
		if err != nil {
			return nil, fmt.Errorf("failed to create SCRAM mechanism: %w", err)
		}

		dialer = &kafkago.Dialer{
			Timeout:       10 * time.Second,
			DualStack:     true,
			SASLMechanism: mechanism,
			TLS:           &tls.Config{MinVersion: tls.VersionTLS12},
		}
	} else {
		dialer = &kafkago.Dialer{
			Timeout:   10 * time.Second,
			DualStack: true,
			TLS:       &tls.Config{MinVersion: tls.VersionTLS12},
		}
	}

	// Connect to the first broker to get metadata
	conn, err := dialer.DialContext(ctx, "tcp", brokers[0])
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Kafka: %w", err)
	}
	defer conn.Close()

	// Set deadline for metadata fetch
	if err := conn.SetDeadline(time.Now().Add(10 * time.Second)); err != nil {
		return nil, fmt.Errorf("failed to set deadline: %w", err)
	}

	// Fetch partitions metadata
	partitions, err := conn.ReadPartitions()
	if err != nil {
		return nil, fmt.Errorf("failed to read partitions: %w", err)
	}

	// Group partitions by topic
	topicMap := make(map[string][]kafkago.Partition)
	for _, partition := range partitions {
		topicMap[partition.Topic] = append(topicMap[partition.Topic], partition)
	}

	// Build topic list
	var topics []KafkaTopic
	for topicName, parts := range topicMap {
		topic := KafkaTopic{
			Name:       topicName,
			Partitions: len(parts),
		}
		// Get replica count from first partition
		if len(parts) > 0 {
			topic.Replicas = len(parts[0].Replicas)
		}
		topics = append(topics, topic)
	}

	// Sort topics alphabetically
	sortTopics(topics)

	return topics, nil
}

func sortTopics(topics []KafkaTopic) {
	sort.Slice(topics, func(i, j int) bool {
		return topics[i].Name < topics[j].Name
	})
}
