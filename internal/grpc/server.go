// Package grpc provides gRPC server implementation for metrics service.
package grpc

import (
	"context"
	"net"
	"net/netip"
	"strings"

	"github.com/GAV777/httpmetricalert/internal/model"
	"github.com/GAV777/httpmetricalert/internal/proto"
	"github.com/GAV777/httpmetricalert/internal/storage"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// MetricsService implements the gRPC Metrics service.
type MetricsService struct {
	proto.UnimplementedMetricsServer
	store  storage.MetricsStorage
	subnet *net.IPNet
}

// NewMetricsService creates a new MetricsService.
func NewMetricsService(store storage.MetricsStorage, trustedSubnet string) *MetricsService {
	svc := &MetricsService{store: store}

	if trustedSubnet != "" {
		_, ipNet, err := net.ParseCIDR(trustedSubnet)
		if err != nil {
			// Логирование ошибки парсинга подсети
			// Сервер продолжит работу без проверки подсети
		} else {
			svc.subnet = ipNet
		}
	}

	return svc
}

// NewMetricsServiceForTest creates a MetricsService with explicit subnet for testing.
func NewMetricsServiceForTest(store storage.MetricsStorage, subnet string) *MetricsService {
	svc := &MetricsService{store: store}

	if subnet != "" {
		_, ipNet, err := net.ParseCIDR(subnet)
		if err != nil {
			// Игнорируем ошибку в тестах
		} else {
			svc.subnet = ipNet
		}
	}

	return svc
}

// UpdateMetrics implements the UpdateMetrics RPC.
func (s *MetricsService) UpdateMetrics(ctx context.Context, req *proto.UpdateMetricsRequest) (*proto.UpdateMetricsResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is nil")
	}

	if len(req.Metrics) == 0 {
		return &proto.UpdateMetricsResponse{}, nil
	}

	// Конвертируем proto-метрики в модельные
	metrics := make([]model.Metrics, 0, len(req.Metrics))
	for _, m := range req.Metrics {
		metric := model.Metrics{
			ID:    m.Id,
			MType: mTypeToString(m.Type),
		}

		switch m.Type {
		case proto.Metric_GAUGE:
			v := m.Value
			metric.Value = &v
		case proto.Metric_COUNTER:
			d := m.Delta
			metric.Delta = &d
		}

		metrics = append(metrics, metric)
	}

	// Сохраняем батч
	if err := s.store.UpdateBatch(metrics); err != nil {
		return nil, status.Error(codes.Internal, "failed to update metrics: "+err.Error())
	}

	return &proto.UpdateMetricsResponse{}, nil
}

// UnaryServerInterceptor returns a gRPC unary interceptor that validates the client IP.
func (s *MetricsService) UnaryServerInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		if s.subnet == nil {
			// Подсеть не настроена — пропускаем проверку
			return handler(ctx, req)
		}

		// Извлекаем IP из метаданных
		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			return nil, status.Error(codes.PermissionDenied, "missing metadata")
		}

		// Ищем IP в ключах x-real-ip, x-forwarded-for
		ipStr := firstHeaderValue(md, "x-real-ip", "x-forwarded-for")
		if ipStr == "" {
			return nil, status.Error(codes.PermissionDenied, "missing x-real-ip header")
		}

		// Парсим IP
		ip, err := netip.ParseAddr(ipStr)
		if err != nil {
			return nil, status.Error(codes.PermissionDenied, "invalid x-real-ip: "+err.Error())
		}

		// Проверяем принадлежность подсети
		if !s.subnet.Contains(ip.AsSlice()) {
			return nil, status.Error(codes.PermissionDenied, "ip "+ipStr+" not in trusted subnet")
		}

		return handler(ctx, req)
	}
}

// Register registers the service with the given gRPC server.
func (s *MetricsService) Register(gs *grpc.Server) {
	proto.RegisterMetricsServer(gs, s)
}

// mTypeToString converts proto MType to model type string.
func mTypeToString(t proto.Metric_MType) string {
	switch t {
	case proto.Metric_GAUGE:
		return model.Gauge
	case proto.Metric_COUNTER:
		return model.Counter
	default:
		return ""
	}
}

// firstHeaderValue returns the first value for the given header keys.
func firstHeaderValue(md metadata.MD, keys ...string) string {
	for _, key := range keys {
		if vals := md.Get(key); len(vals) > 0 {
			return strings.TrimSpace(vals[0])
		}
	}
	return ""
}
