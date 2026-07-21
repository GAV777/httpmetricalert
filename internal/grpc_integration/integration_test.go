package grpc_integration

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	grpcserver "github.com/GAV777/httpmetricalert/internal/grpc"
	grpcclient "github.com/GAV777/httpmetricalert/internal/grpcclient"
	"github.com/GAV777/httpmetricalert/internal/model"
	"github.com/GAV777/httpmetricalert/internal/storage"
	"google.golang.org/grpc"
)

// findFreePort находит свободный порт
func findFreePort(t *testing.T) int {
	t.Helper()
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to find free port: %v", err)
	}
	port := lis.Addr().(*net.TCPAddr).Port
	lis.Close()
	return port
}

// startGRPCServer запускает gRPC-сервер на указанном адресе
func startGRPCServer(t *testing.T, addr string, store storage.MetricsStorage, subnet string) *grpc.Server {
	t.Helper()

	svc := grpcserver.NewMetricsServiceForTest(store, subnet)

	lis, err := net.Listen("tcp", addr)
	if err != nil {
		t.Fatalf("Failed to listen on %s: %v", addr, err)
	}

	srv := grpc.NewServer(
		grpc.UnaryInterceptor(svc.UnaryServerInterceptor()),
	)
	svc.Register(srv)

	go func() {
		if err := srv.Serve(lis); err != nil {
			// Сервер остановлен — это ожидаемо
			return
		}
	}()

	// Ждём, чтобы сервер успел стартовать
	time.Sleep(50 * time.Millisecond)

	t.Cleanup(func() {
		srv.GracefulStop()
	})

	return srv
}

func TestIntegration_GaugeRoundTrip(t *testing.T) {
	t.Parallel()

	port := findFreePort(t)
	addr := fmt.Sprintf("127.0.0.1:%d", port)

	store := storage.NewMemStorage()
	startGRPCServer(t, addr, store, "")

	client, err := grpcclient.NewClient(addr, "127.0.0.1")
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	gaugeVal := 12345.67
	batch := []model.Metrics{
		{ID: "integration_gauge", MType: model.Gauge, Value: &gaugeVal},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.SendBatch(ctx, batch); err != nil {
		t.Fatalf("SendBatch returned error: %v", err)
	}

	got, ok := store.GetGauge("integration_gauge")
	if !ok {
		t.Fatal("GetGauge returned false")
	}
	if got != gaugeVal {
		t.Errorf("GetGauge = %v, want %v", got, gaugeVal)
	}
}

func TestIntegration_CounterRoundTrip(t *testing.T) {
	t.Parallel()

	port := findFreePort(t)
	addr := fmt.Sprintf("127.0.0.1:%d", port)

	store := storage.NewMemStorage()
	startGRPCServer(t, addr, store, "")

	client, err := grpcclient.NewClient(addr, "127.0.0.1")
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	delta := int64(42)
	batch := []model.Metrics{
		{ID: "integration_counter", MType: model.Counter, Delta: &delta},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.SendBatch(ctx, batch); err != nil {
		t.Fatalf("SendBatch returned error: %v", err)
	}

	got, ok := store.GetCounter("integration_counter")
	if !ok {
		t.Fatal("GetCounter returned false")
	}
	if got != delta {
		t.Errorf("GetCounter = %v, want %v", got, delta)
	}
}

func TestIntegration_MixedBatchRoundTrip(t *testing.T) {
	t.Parallel()

	port := findFreePort(t)
	addr := fmt.Sprintf("127.0.0.1:%d", port)

	store := storage.NewMemStorage()
	startGRPCServer(t, addr, store, "")

	client, err := grpcclient.NewClient(addr, "127.0.0.1")
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	gaugeVal := 99.9
	counterDelta := int64(100)
	batch := []model.Metrics{
		{ID: "mixed_gauge", MType: model.Gauge, Value: &gaugeVal},
		{ID: "mixed_counter", MType: model.Counter, Delta: &counterDelta},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.SendBatch(ctx, batch); err != nil {
		t.Fatalf("SendBatch returned error: %v", err)
	}

	gv, ok := store.GetGauge("mixed_gauge")
	if !ok || gv != gaugeVal {
		t.Errorf("mixed_gauge = %v, want %v", gv, gaugeVal)
	}

	cv, ok := store.GetCounter("mixed_counter")
	if !ok || cv != counterDelta {
		t.Errorf("mixed_counter = %v, want %v", cv, counterDelta)
	}
}

func TestIntegration_TrustedSubnet(t *testing.T) {
	t.Parallel()

	port := findFreePort(t)
	addr := fmt.Sprintf("127.0.0.1:%d", port)

	store := storage.NewMemStorage()
	// Подсеть 127.0.0.0/8 включает localhost
	startGRPCServer(t, addr, store, "127.0.0.0/8")

	// IP клиента входит в подсеть
	client, err := grpcclient.NewClient(addr, "127.0.0.1")
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	gaugeVal := 1.0
	batch := []model.Metrics{
		{ID: "trusted_integration", MType: model.Gauge, Value: &gaugeVal},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.SendBatch(ctx, batch); err != nil {
		t.Fatalf("SendBatch with trusted IP returned error: %v", err)
	}

	got, ok := store.GetGauge("trusted_integration")
	if !ok || got != gaugeVal {
		t.Errorf("trusted_integration = %v, want %v", got, gaugeVal)
	}
}

func TestIntegration_UntrustedSubnet(t *testing.T) {
	t.Parallel()

	port := findFreePort(t)
	addr := fmt.Sprintf("127.0.0.1:%d", port)

	store := storage.NewMemStorage()
	// Подсеть 10.0.0.0/8 НЕ включает 127.0.0.1
	startGRPCServer(t, addr, store, "10.0.0.0/8")

	// IP клиента НЕ входит в подсеть
	client, err := grpcclient.NewClient(addr, "127.0.0.1")
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	gaugeVal := 1.0
	batch := []model.Metrics{
		{ID: "untrusted_integration", MType: model.Gauge, Value: &gaugeVal},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = client.SendBatch(ctx, batch)
	if err == nil {
		t.Fatal("SendBatch with untrusted IP did not return error")
	}
}

func TestIntegration_MultipleBatches(t *testing.T) {
	t.Parallel()

	port := findFreePort(t)
	addr := fmt.Sprintf("127.0.0.1:%d", port)

	store := storage.NewMemStorage()
	startGRPCServer(t, addr, store, "")

	client, err := grpcclient.NewClient(addr, "127.0.0.1")
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Отправляем несколько батчей
	for i := 0; i < 5; i++ {
		delta := int64(10)
		batch := []model.Metrics{
			{ID: "accumulating_counter", MType: model.Counter, Delta: &delta},
		}

		if err := client.SendBatch(ctx, batch); err != nil {
			t.Fatalf("SendBatch %d returned error: %v", i, err)
		}
	}

	// Счётчик должен накопиться (5 * 10 = 50)
	got, ok := store.GetCounter("accumulating_counter")
	if !ok {
		t.Fatal("GetCounter returned false")
	}
	if got != 50 {
		t.Errorf("accumulating_counter = %v, want 50", got)
	}
}

func TestIntegration_LargeBatch(t *testing.T) {
	t.Parallel()

	port := findFreePort(t)
	addr := fmt.Sprintf("127.0.0.1:%d", port)

	store := storage.NewMemStorage()
	startGRPCServer(t, addr, store, "")

	client, err := grpcclient.NewClient(addr, "127.0.0.1")
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	// Создаём батч из 500 метрик
	batch := make([]model.Metrics, 500)
	for i := 0; i < 500; i++ {
		val := float64(i)
		name := fmt.Sprintf("large_gauge_%d", i)
		batch[i] = model.Metrics{
			ID:    name,
			MType: model.Gauge,
			Value: &val,
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := client.SendBatch(ctx, batch); err != nil {
		t.Fatalf("SendBatch with 500 metrics returned error: %v", err)
	}

	// Проверяем несколько случайных метрик
	for _, i := range []int{0, 249, 499} {
		name := fmt.Sprintf("large_gauge_%d", i)
		got, ok := store.GetGauge(name)
		if !ok {
			t.Errorf("%s not found", name)
			continue
		}
		if got != float64(i) {
			t.Errorf("%s = %v, want %v", name, got, float64(i))
		}
	}
}
