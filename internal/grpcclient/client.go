// Package grpcclient provides a gRPC client for sending metrics to the server.
package grpcclient

import (
	"context"
	"fmt"
	"time"

	"github.com/GAV777/httpmetricalert/internal/model"
	"github.com/GAV777/httpmetricalert/internal/proto"
	"github.com/GAV777/httpmetricalert/pkg/netutil"
	"github.com/GAV777/httpmetricalert/pkg/retry"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
)

// Client wraps a gRPC connection and provides methods to send metrics.
type Client struct {
	conn    *grpc.ClientConn
	client  proto.MetricsClient
	agentIP string
}

// NewClient creates a new gRPC client connected to the given address.
func NewClient(address, agentIP string) (*Client, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cfg := retry.DefaultConfig()
	var conn *grpc.ClientConn
	var dialErr error

	if err := retry.Do(ctx, cfg, func() error {
		conn, dialErr = grpc.NewClient(address, grpc.WithTransportCredentials(insecure.NewCredentials()))
		return dialErr
	}); err != nil {
		return nil, fmt.Errorf("grpc connect to %s: %w", address, err)
	}

	return &Client{
		conn:    conn,
		client:  proto.NewMetricsClient(conn),
		agentIP: agentIP,
	}, nil
}

// SendBatch sends a batch of metrics to the server via gRPC.
func (c *Client) SendBatch(ctx context.Context, metrics []model.Metrics) error {
	if len(metrics) == 0 {
		return nil
	}

	protoMetrics := make([]*proto.Metric, 0, len(metrics))
	for _, m := range metrics {
		pm := &proto.Metric{
			Id: m.ID,
		}

		switch m.MType {
		case model.Gauge:
			pm.Type = proto.Metric_GAUGE
			if m.Value != nil {
				pm.Value = *m.Value
			}
		case model.Counter:
			pm.Type = proto.Metric_COUNTER
			if m.Delta != nil {
				pm.Delta = *m.Delta
			}
		}

		protoMetrics = append(protoMetrics, pm)
	}

	req := &proto.UpdateMetricsRequest{
		Metrics: protoMetrics,
	}

	// Добавляем IP-адрес в метаданные
	ctxWithMD := ctx
	if c.agentIP != "" {
		ctxWithMD = metadata.NewOutgoingContext(ctx, metadata.Pairs("x-real-ip", c.agentIP))
	}

	cfg := retry.DefaultConfig()
	return retry.Do(ctxWithMD, cfg, func() error {
		_, err := c.client.UpdateMetrics(ctxWithMD, req)
		if err != nil {
			return fmt.Errorf("update metrics: %w", err)
		}
		return nil
	})
}

// Close closes the gRPC connection.
func (c *Client) Close() error {
	return c.conn.Close()
}

// GetLocalIP returns the local IP address of the host.
func GetLocalIP() string {
	return netutil.LocalIP()
}
