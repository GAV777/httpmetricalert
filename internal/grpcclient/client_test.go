package grpcclient

import (
	"context"
	"net"
	"testing"

	grpcserver "github.com/GAV777/httpmetricalert/internal/grpc"
	"github.com/GAV777/httpmetricalert/internal/model"
	"github.com/GAV777/httpmetricalert/internal/proto"
	"github.com/GAV777/httpmetricalert/internal/storage"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/test/bufconn"
)

const bufSize = 1024 * 1024

// setupTestServer создаёт тестовый gRPC-сервер с хранилищем и возвращает клиент
func setupTestServer(t *testing.T, subnet string) (store *storage.MemStorage, client *Client) {
	t.Helper()

	store = storage.NewMemStorage()
	svc := grpcserver.NewMetricsServiceForTest(store, subnet)

	lis := bufconn.Listen(bufSize)
	s := grpc.NewServer(
		grpc.UnaryInterceptor(svc.UnaryServerInterceptor()),
	)
	svc.Register(s)

	go func() {
		if err := s.Serve(lis); err != nil {
			t.Errorf("server failed: %v", err)
		}
	}()

	t.Cleanup(func() {
		s.Stop()
	})

	dialer := func(context.Context, string) (net.Conn, error) {
		return lis.Dial()
	}

	conn, err := grpc.NewClient("passthrough:///bufnet", grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithContextDialer(dialer))
	if err != nil {
		t.Fatalf("Failed to dial bufnet: %v", err)
	}
	t.Cleanup(func() { conn.Close() })

	client = &Client{
		conn:    conn,
		client:  proto.NewMetricsClient(conn),
		agentIP: "192.168.1.50",
	}

	return store, client
}

func TestClient_SendBatch_Gauges(t *testing.T) {
	t.Parallel()

	store, client := setupTestServer(t, "")

	gaugeVal1 := 42.5
	gaugeVal2 := 100.0
	batch := []model.Metrics{
		{ID: "gauge1", MType: model.Gauge, Value: &gaugeVal1},
		{ID: "gauge2", MType: model.Gauge, Value: &gaugeVal2},
	}

	err := client.SendBatch(context.Background(), batch)
	if err != nil {
		t.Fatalf("SendBatch returned error: %v", err)
	}

	gv1, ok := store.GetGauge("gauge1")
	if !ok || gv1 != gaugeVal1 {
		t.Errorf("gauge1 = %v, want %v", gv1, gaugeVal1)
	}

	gv2, ok := store.GetGauge("gauge2")
	if !ok || gv2 != gaugeVal2 {
		t.Errorf("gauge2 = %v, want %v", gv2, gaugeVal2)
	}
}

func TestClient_SendBatch_Counters(t *testing.T) {
	t.Parallel()

	store, client := setupTestServer(t, "")

	delta1 := int64(10)
	delta2 := int64(20)
	batch := []model.Metrics{
		{ID: "counter1", MType: model.Counter, Delta: &delta1},
		{ID: "counter2", MType: model.Counter, Delta: &delta2},
	}

	err := client.SendBatch(context.Background(), batch)
	if err != nil {
		t.Fatalf("SendBatch returned error: %v", err)
	}

	cv1, ok := store.GetCounter("counter1")
	if !ok || cv1 != delta1 {
		t.Errorf("counter1 = %v, want %v", cv1, delta1)
	}

	cv2, ok := store.GetCounter("counter2")
	if !ok || cv2 != delta2 {
		t.Errorf("counter2 = %v, want %v", cv2, delta2)
	}
}

func TestClient_SendBatch_Mixed(t *testing.T) {
	t.Parallel()

	store, client := setupTestServer(t, "")

	gaugeVal := 3.14
	counterDelta := int64(50)
	batch := []model.Metrics{
		{ID: "mixed_gauge", MType: model.Gauge, Value: &gaugeVal},
		{ID: "mixed_counter", MType: model.Counter, Delta: &counterDelta},
	}

	err := client.SendBatch(context.Background(), batch)
	if err != nil {
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

func TestClient_SendBatch_Empty(t *testing.T) {
	t.Parallel()

	_, client := setupTestServer(t, "")

	err := client.SendBatch(context.Background(), []model.Metrics{})
	if err != nil {
		t.Fatalf("SendBatch with empty batch returned error: %v", err)
	}
}

func TestClient_SendBatch_NilValues(t *testing.T) {
	t.Parallel()

	store, client := setupTestServer(t, "")

	// Метрики с nil-значениями должны отправляться (конвертируются в proto с нулями)
	batch := []model.Metrics{
		{ID: "nil_gauge", MType: model.Gauge, Value: nil},
		{ID: "nil_counter", MType: model.Counter, Delta: nil},
	}

	err := client.SendBatch(context.Background(), batch)
	if err != nil {
		t.Fatalf("SendBatch with nil values returned error: %v", err)
	}

	// На сервере nil-значения сохраняются как 0
	gv, ok := store.GetGauge("nil_gauge")
	if !ok {
		t.Error("nil_gauge should exist")
	} else if gv != 0 {
		t.Errorf("nil_gauge = %v, want 0", gv)
	}

	cv, ok := store.GetCounter("nil_counter")
	if !ok {
		t.Error("nil_counter should exist")
	} else if cv != 0 {
		t.Errorf("nil_counter = %v, want 0", cv)
	}
}

func TestClient_SendBatch_WithTrustedSubnet(t *testing.T) {
	t.Parallel()

	store, client := setupTestServer(t, "192.168.1.0/24")

	// IP клиента "192.168.1.50" входит в подсеть
	gaugeVal := 99.9
	batch := []model.Metrics{
		{ID: "trusted_gauge", MType: model.Gauge, Value: &gaugeVal},
	}

	err := client.SendBatch(context.Background(), batch)
	if err != nil {
		t.Fatalf("SendBatch with trusted IP returned error: %v", err)
	}

	gv, ok := store.GetGauge("trusted_gauge")
	if !ok || gv != gaugeVal {
		t.Errorf("trusted_gauge = %v, want %v", gv, gaugeVal)
	}
}

func TestClient_SendBatch_WithUntrustedSubnet(t *testing.T) {
	t.Parallel()

	_, client := setupTestServer(t, "10.0.0.0/8")

	// IP клиента "192.168.1.50" НЕ входит в подсеть 10.0.0.0/8
	gaugeVal := 99.9
	batch := []model.Metrics{
		{ID: "untrusted_gauge", MType: model.Gauge, Value: &gaugeVal},
	}

	err := client.SendBatch(context.Background(), batch)
	if err == nil {
		t.Fatal("SendBatch with untrusted IP did not return error")
	}
}

func TestClient_SendBatch_LargeBatch(t *testing.T) {
	t.Parallel()

	store, client := setupTestServer(t, "")

	batch := make([]model.Metrics, 100)
	for i := 0; i < 100; i++ {
		val := float64(i)
		batch[i] = model.Metrics{
			ID:    "large_gauge",
			MType: model.Gauge,
			Value: &val,
		}
	}

	err := client.SendBatch(context.Background(), batch)
	if err != nil {
		t.Fatalf("SendBatch with large batch returned error: %v", err)
	}

	// Последнее значение должно быть 99.0
	gv, ok := store.GetGauge("large_gauge")
	if !ok || gv != 99.0 {
		t.Errorf("large_gauge = %v, want 99.0", gv)
	}
}

func TestClient_IPMetadata(t *testing.T) {
	t.Parallel()

	// Проверяем, что IP-адрес передаётся в метаданных
	store := storage.NewMemStorage()
	svc := grpcserver.NewMetricsServiceForTest(store, "")

	lis := bufconn.Listen(bufSize)

	// Интерцептор для проверки метаданных
	checkIPInterceptor := func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			t.Fatal("Missing metadata in interceptor")
		}

		ips := md.Get("x-real-ip")
		if len(ips) == 0 {
			t.Fatal("x-real-ip header not found in metadata")
		}
		if ips[0] != "192.168.1.50" {
			t.Errorf("x-real-ip = %q, want %q", ips[0], "192.168.1.50")
		}

		return handler(ctx, req)
	}

	s := grpc.NewServer(
		grpc.UnaryInterceptor(checkIPInterceptor),
	)
	svc.Register(s)

	go func() {
		if err := s.Serve(lis); err != nil {
			t.Errorf("server failed: %v", err)
		}
	}()

	t.Cleanup(func() {
		s.Stop()
	})

	dialer := func(context.Context, string) (net.Conn, error) {
		return lis.Dial()
	}

	conn, err := grpc.NewClient("passthrough:///bufnet", grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithContextDialer(dialer))
	if err != nil {
		t.Fatalf("Failed to dial bufnet: %v", err)
	}
	t.Cleanup(func() { conn.Close() })

	client := &Client{
		conn:    conn,
		client:  proto.NewMetricsClient(conn),
		agentIP: "192.168.1.50",
	}

	gaugeVal := 1.0
	batch := []model.Metrics{
		{ID: "ip_check_gauge", MType: model.Gauge, Value: &gaugeVal},
	}

	err = client.SendBatch(context.Background(), batch)
	if err != nil {
		t.Fatalf("SendBatch returned error: %v", err)
	}
}

func TestGetLocalIP(t *testing.T) {
	t.Parallel()

	ip := GetLocalIP()
	if ip == "" {
		t.Log("GetLocalIP returned empty string (may be expected in some environments)")
		return
	}

	// Проверяем, что это валидный IPv4-адрес
	parsed := net.ParseIP(ip)
	if parsed == nil {
		t.Errorf("GetLocalIP returned invalid IP: %q", ip)
	}
	if parsed.To4() == nil {
		t.Errorf("GetLocalIP returned non-IPv4 address: %q", ip)
	}
}
