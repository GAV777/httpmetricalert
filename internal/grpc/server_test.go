package grpc

import (
	"context"
	"net"
	"testing"

	"github.com/GAV777/httpmetricalert/internal/model"
	"github.com/GAV777/httpmetricalert/internal/proto"
	"github.com/GAV777/httpmetricalert/internal/storage"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

const bufSize = 1024 * 1024

// testEnv — окружение для одного теста (сервер, listener, клиент)
type testEnv struct {
	srv    *grpc.Server
	lis    *bufconn.Listener
	client proto.MetricsClient
	store  *storage.MemStorage
}

// setupTestEnv создаёт изолированное тестовое окружение
func setupTestEnv(t *testing.T, subnet string) *testEnv {
	t.Helper()

	store := storage.NewMemStorage()
	svc := NewMetricsServiceForTest(store, subnet)

	lis := bufconn.Listen(bufSize)
	srv := grpc.NewServer(
		grpc.UnaryInterceptor(svc.UnaryServerInterceptor()),
	)
	svc.Register(srv)

	go func() {
		if err := srv.Serve(lis); err != nil {
			// Сервер остановлен в cleanup — это ожидаемо
			return
		}
	}()

	dialer := func(context.Context, string) (net.Conn, error) {
		return lis.Dial()
	}

	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithContextDialer(dialer),
	)
	if err != nil {
		t.Fatalf("Failed to dial bufnet: %v", err)
	}

	env := &testEnv{
		srv:    srv,
		lis:    lis,
		client: proto.NewMetricsClient(conn),
		store:  store,
	}

	t.Cleanup(func() {
		conn.Close()
		srv.Stop()
	})

	return env
}

func TestMetricsService_UpdateMetrics_Gauge(t *testing.T) {
	t.Parallel()

	env := setupTestEnv(t, "")

	value := 42.5
	resp, err := env.client.UpdateMetrics(context.Background(), &proto.UpdateMetricsRequest{
		Metrics: []*proto.Metric{
			{Id: "test_gauge", Type: proto.Metric_GAUGE, Value: value},
		},
	})
	if err != nil {
		t.Fatalf("UpdateMetrics returned error: %v", err)
	}
	if resp == nil {
		t.Fatal("UpdateMetrics returned nil response")
	}

	got, ok := env.store.GetGauge("test_gauge")
	if !ok {
		t.Fatal("GetGauge returned false")
	}
	if got != value {
		t.Errorf("GetGauge = %v, want %v", got, value)
	}
}

func TestMetricsService_UpdateMetrics_Counter(t *testing.T) {
	t.Parallel()

	env := setupTestEnv(t, "")

	delta := int64(100)
	resp, err := env.client.UpdateMetrics(context.Background(), &proto.UpdateMetricsRequest{
		Metrics: []*proto.Metric{
			{Id: "test_counter", Type: proto.Metric_COUNTER, Delta: delta},
		},
	})
	if err != nil {
		t.Fatalf("UpdateMetrics returned error: %v", err)
	}
	if resp == nil {
		t.Fatal("UpdateMetrics returned nil response")
	}

	got, ok := env.store.GetCounter("test_counter")
	if !ok {
		t.Fatal("GetCounter returned false")
	}
	if got != delta {
		t.Errorf("GetCounter = %v, want %v", got, delta)
	}
}

func TestMetricsService_UpdateMetrics_Batch(t *testing.T) {
	t.Parallel()

	env := setupTestEnv(t, "")

	gaugeVal := 3.14
	counterDelta := int64(50)

	resp, err := env.client.UpdateMetrics(context.Background(), &proto.UpdateMetricsRequest{
		Metrics: []*proto.Metric{
			{Id: "batch_gauge", Type: proto.Metric_GAUGE, Value: gaugeVal},
			{Id: "batch_counter", Type: proto.Metric_COUNTER, Delta: counterDelta},
			{Id: "batch_gauge2", Type: proto.Metric_GAUGE, Value: 2.71},
		},
	})
	if err != nil {
		t.Fatalf("UpdateMetrics returned error: %v", err)
	}
	if resp == nil {
		t.Fatal("UpdateMetrics returned nil response")
	}

	// Проверяем все метрики
	gv, ok := env.store.GetGauge("batch_gauge")
	if !ok || gv != gaugeVal {
		t.Errorf("batch_gauge = %v, want %v", gv, gaugeVal)
	}

	cv, ok := env.store.GetCounter("batch_counter")
	if !ok || cv != counterDelta {
		t.Errorf("batch_counter = %v, want %v", cv, counterDelta)
	}

	gv2, ok := env.store.GetGauge("batch_gauge2")
	if !ok || gv2 != 2.71 {
		t.Errorf("batch_gauge2 = %v, want 2.71", gv2)
	}
}

func TestMetricsService_UpdateMetrics_EmptyRequest(t *testing.T) {
	t.Parallel()

	env := setupTestEnv(t, "")

	resp, err := env.client.UpdateMetrics(context.Background(), &proto.UpdateMetricsRequest{})
	if err != nil {
		t.Fatalf("UpdateMetrics with empty request returned error: %v", err)
	}
	if resp == nil {
		t.Fatal("UpdateMetrics returned nil response")
	}
}

func TestTrustedSubnetInterceptor_AllowsTrustedIP(t *testing.T) {
	t.Parallel()

	env := setupTestEnv(t, "192.168.1.0/24")

	// Добавляем IP в метаданные вручную
	md := metadata.New(map[string]string{"x-real-ip": "192.168.1.100"})
	ctx := metadata.NewOutgoingContext(context.Background(), md)

	gaugeVal := 1.0
	_, err := env.client.UpdateMetrics(ctx, &proto.UpdateMetricsRequest{
		Metrics: []*proto.Metric{
			{Id: "trusted_gauge", Type: proto.Metric_GAUGE, Value: gaugeVal},
		},
	})
	if err != nil {
		t.Fatalf("UpdateMetrics with trusted IP returned error: %v", err)
	}

	got, ok := env.store.GetGauge("trusted_gauge")
	if !ok || got != gaugeVal {
		t.Errorf("trusted_gauge = %v, want %v", got, gaugeVal)
	}
}

func TestTrustedSubnetInterceptor_RejectsUntrustedIP(t *testing.T) {
	t.Parallel()

	env := setupTestEnv(t, "192.168.1.0/24")

	md := metadata.New(map[string]string{"x-real-ip": "10.0.0.1"})
	ctx := metadata.NewOutgoingContext(context.Background(), md)

	gaugeVal := 1.0
	_, err := env.client.UpdateMetrics(ctx, &proto.UpdateMetricsRequest{
		Metrics: []*proto.Metric{
			{Id: "untrusted_gauge", Type: proto.Metric_GAUGE, Value: gaugeVal},
		},
	})
	if err == nil {
		t.Fatal("UpdateMetrics with untrusted IP did not return error")
	}

	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("Error is not a gRPC status error: %v", err)
	}
	if st.Code() != codes.PermissionDenied {
		t.Errorf("Expected code PermissionDenied, got %v", st.Code())
	}
}

func TestTrustedSubnetInterceptor_NoSubnetConfigured(t *testing.T) {
	t.Parallel()

	// Без подсети — должно работать даже без IP-метаданных
	env := setupTestEnv(t, "")

	gaugeVal := 1.0
	_, err := env.client.UpdateMetrics(context.Background(), &proto.UpdateMetricsRequest{
		Metrics: []*proto.Metric{
			{Id: "no_subnet_gauge", Type: proto.Metric_GAUGE, Value: gaugeVal},
		},
	})
	if err != nil {
		t.Fatalf("UpdateMetrics without subnet returned error: %v", err)
	}

	got, ok := env.store.GetGauge("no_subnet_gauge")
	if !ok || got != gaugeVal {
		t.Errorf("no_subnet_gauge = %v, want %v", got, gaugeVal)
	}
}

func TestTrustedSubnetInterceptor_MissingIPHeader(t *testing.T) {
	t.Parallel()

	env := setupTestEnv(t, "10.0.0.0/8")

	// Нет x-real-ip — должен быть отказ
	gaugeVal := 1.0
	_, err := env.client.UpdateMetrics(context.Background(), &proto.UpdateMetricsRequest{
		Metrics: []*proto.Metric{
			{Id: "missing_ip_gauge", Type: proto.Metric_GAUGE, Value: gaugeVal},
		},
	})
	if err == nil {
		t.Fatal("UpdateMetrics without x-real-ip did not return error")
	}

	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("Error is not a gRPC status error: %v", err)
	}
	if st.Code() != codes.PermissionDenied {
		t.Errorf("Expected code PermissionDenied, got %v", st.Code())
	}
}

func TestTrustedSubnetInterceptor_InvalidIP(t *testing.T) {
	t.Parallel()

	env := setupTestEnv(t, "10.0.0.0/8")

	md := metadata.New(map[string]string{"x-real-ip": "not-an-ip"})
	ctx := metadata.NewOutgoingContext(context.Background(), md)

	gaugeVal := 1.0
	_, err := env.client.UpdateMetrics(ctx, &proto.UpdateMetricsRequest{
		Metrics: []*proto.Metric{
			{Id: "invalid_ip_gauge", Type: proto.Metric_GAUGE, Value: gaugeVal},
		},
	})
	if err == nil {
		t.Fatal("UpdateMetrics with invalid IP did not return error")
	}

	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("Error is not a gRPC status error: %v", err)
	}
	if st.Code() != codes.PermissionDenied {
		t.Errorf("Expected code PermissionDenied, got %v", st.Code())
	}
}

func TestTrustedSubnetInterceptor_XForwardedFor(t *testing.T) {
	t.Parallel()

	env := setupTestEnv(t, "172.16.0.0/12")

	// x-forwarded-for как fallback
	md := metadata.New(map[string]string{"x-forwarded-for": "172.16.5.10"})
	ctx := metadata.NewOutgoingContext(context.Background(), md)

	gaugeVal := 1.0
	_, err := env.client.UpdateMetrics(ctx, &proto.UpdateMetricsRequest{
		Metrics: []*proto.Metric{
			{Id: "xff_gauge", Type: proto.Metric_GAUGE, Value: gaugeVal},
		},
	})
	if err != nil {
		t.Fatalf("UpdateMetrics with x-forwarded-for returned error: %v", err)
	}

	got, ok := env.store.GetGauge("xff_gauge")
	if !ok || got != gaugeVal {
		t.Errorf("xff_gauge = %v, want %v", got, gaugeVal)
	}
}

func TestMetricsService_UpdateMetrics_MixedBatchWithCounters(t *testing.T) {
	t.Parallel()

	env := setupTestEnv(t, "")

	// Несколько метрик, включая повторные счётчики (должны суммироваться)
	delta1 := int64(10)
	delta2 := int64(20)
	gaugeVal := 100.0

	resp, err := env.client.UpdateMetrics(context.Background(), &proto.UpdateMetricsRequest{
		Metrics: []*proto.Metric{
			{Id: "multi_counter", Type: proto.Metric_COUNTER, Delta: delta1},
			{Id: "multi_counter", Type: proto.Metric_COUNTER, Delta: delta2},
			{Id: "single_gauge", Type: proto.Metric_GAUGE, Value: gaugeVal},
		},
	})
	if err != nil {
		t.Fatalf("UpdateMetrics returned error: %v", err)
	}
	if resp == nil {
		t.Fatal("UpdateMetrics returned nil response")
	}

	// Счётчики суммируются в хранилище
	cv, ok := env.store.GetCounter("multi_counter")
	if !ok {
		t.Fatal("GetCounter returned false")
	}
	if cv != 30 {
		t.Errorf("multi_counter = %v, want 30", cv)
	}

	gv, ok := env.store.GetGauge("single_gauge")
	if !ok || gv != gaugeVal {
		t.Errorf("single_gauge = %v, want %v", gv, gaugeVal)
	}
}

// Тест на соответствие proto.Metric -> model.Metrics
func TestMTypeToString(t *testing.T) {
	t.Parallel()

	tests := []struct {
		mtype proto.Metric_MType
		want  string
	}{
		{proto.Metric_GAUGE, model.Gauge},
		{proto.Metric_COUNTER, model.Counter},
		{proto.Metric_MType(99), ""},
	}

	for _, tt := range tests {
		got := mTypeToString(tt.mtype)
		if got != tt.want {
			t.Errorf("mTypeToString(%v) = %q, want %q", tt.mtype, got, tt.want)
		}
	}
}
