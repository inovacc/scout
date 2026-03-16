package server

import (
	"context"
	"net"
	"testing"
	"time"

	pb "github.com/inovacc/scout/grpc/scoutpb"
	"github.com/inovacc/scout/internal/engine/swarm"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

// swarmTestEnv holds a gRPC client and the underlying ScoutServer + Coordinator
// so tests can enqueue URLs and inspect coordinator state directly.
type swarmTestEnv struct {
	client pb.ScoutServiceClient
	srv    *ScoutServer
	coord  *swarm.Coordinator
}

// setupSwarmTestServer starts an in-process gRPC server with a swarm coordinator attached.
func setupSwarmTestServer(t *testing.T) *swarmTestEnv {
	t.Helper()

	cfg := swarm.SwarmConfig{
		BatchSize:         5,
		MaxWorkers:        10,
		HeartbeatInterval: 30 * time.Second,
		HeartbeatTimeout:  60 * time.Second,
		DefaultRateLimit:  10 * time.Millisecond,
	}

	coord := swarm.NewCoordinator(cfg, nil)

	ctx, cancel := context.WithCancel(context.Background())
	coord.Start(ctx)

	srv := New()
	srv.SetSwarm(coord)

	grpcServer := grpc.NewServer()
	pb.RegisterScoutServiceServer(grpcServer, srv)

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		cancel()
		t.Fatalf("listen: %v", err)
	}

	go func() { _ = grpcServer.Serve(lis) }()

	conn, err := grpc.NewClient(
		lis.Addr().String(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		cancel()
		t.Fatalf("dial: %v", err)
	}

	t.Cleanup(func() {
		_ = conn.Close()
		grpcServer.Stop()
		coord.Stop()
		cancel()
	})

	return &swarmTestEnv{
		client: pb.NewScoutServiceClient(conn),
		srv:    srv,
		coord:  coord,
	}
}

func TestSwarmJoinLeave(t *testing.T) {
	env := setupSwarmTestServer(t)
	ctx := context.Background()

	// Join
	joinResp, err := env.client.JoinSwarm(ctx, &pb.JoinSwarmRequest{
		WorkerId: "worker-1",
		Proxy:    "socks5://proxy:1080",
	})
	if err != nil {
		t.Fatalf("JoinSwarm: %v", err)
	}

	if !joinResp.GetAccepted() {
		t.Fatalf("expected Accepted=true, got false: %s", joinResp.GetMessage())
	}

	if joinResp.GetBatchSize() != 5 {
		t.Errorf("expected BatchSize=5, got %d", joinResp.GetBatchSize())
	}

	if joinResp.GetHeartbeatIntervalMs() <= 0 {
		t.Errorf("expected positive HeartbeatIntervalMs, got %d", joinResp.GetHeartbeatIntervalMs())
	}

	// Leave
	leaveResp, err := env.client.LeaveSwarm(ctx, &pb.LeaveSwarmRequest{
		WorkerId: "worker-1",
	})
	if err != nil {
		t.Fatalf("LeaveSwarm: %v", err)
	}

	if !leaveResp.GetAcknowledged() {
		t.Error("expected Acknowledged=true")
	}

	// Leaving again should fail (NotFound).
	_, err = env.client.LeaveSwarm(ctx, &pb.LeaveSwarmRequest{
		WorkerId: "worker-1",
	})
	if err == nil {
		t.Fatal("expected error on second LeaveSwarm, got nil")
	}

	if s, ok := status.FromError(err); !ok || s.Code() != codes.NotFound {
		t.Errorf("expected NotFound, got %v", err)
	}
}

func TestSwarmFetchBatchEmpty(t *testing.T) {
	env := setupSwarmTestServer(t)
	ctx := context.Background()

	// Join worker
	_, err := env.client.JoinSwarm(ctx, &pb.JoinSwarmRequest{WorkerId: "worker-empty"})
	if err != nil {
		t.Fatalf("JoinSwarm: %v", err)
	}

	// FetchBatch with nothing in queue
	resp, err := env.client.FetchBatch(ctx, &pb.FetchBatchRequest{
		WorkerId: "worker-empty",
		MaxUrls:  5,
	})
	if err != nil {
		t.Fatalf("FetchBatch: %v", err)
	}

	if len(resp.GetUrls()) != 0 {
		t.Errorf("expected 0 URLs, got %d", len(resp.GetUrls()))
	}

	if !resp.GetDrain() {
		t.Error("expected Drain=true when queue is empty")
	}
}

func TestSwarmFullFlow(t *testing.T) {
	env := setupSwarmTestServer(t)
	ctx := context.Background()

	// 1. Join worker
	_, err := env.client.JoinSwarm(ctx, &pb.JoinSwarmRequest{WorkerId: "worker-full"})
	if err != nil {
		t.Fatalf("JoinSwarm: %v", err)
	}

	// 2. Enqueue seed URLs directly via coordinator.
	// Use different domains to avoid per-domain rate limiting in DomainQueue.
	seeds := []swarm.CrawlRequest{
		{URL: "https://alpha.example.com/page1", Depth: 0, Domain: "alpha.example.com"},
		{URL: "https://beta.example.com/page2", Depth: 0, Domain: "beta.example.com"},
		{URL: "https://gamma.example.com/page3", Depth: 0, Domain: "gamma.example.com"},
	}

	n, err := env.coord.Enqueue(seeds)
	if err != nil {
		t.Fatalf("Enqueue seeds: %v", err)
	}

	if n != 3 {
		t.Fatalf("expected 3 enqueued, got %d", n)
	}

	// 3. FetchBatch — should get the 3 seed URLs
	fetchResp, err := env.client.FetchBatch(ctx, &pb.FetchBatchRequest{
		WorkerId: "worker-full",
		MaxUrls:  10,
	})
	if err != nil {
		t.Fatalf("FetchBatch: %v", err)
	}

	if len(fetchResp.GetUrls()) != 3 {
		t.Fatalf("expected 3 URLs, got %d", len(fetchResp.GetUrls()))
	}

	if fetchResp.GetDrain() {
		t.Error("expected Drain=false when URLs were returned")
	}

	// Verify URL contents
	gotURLs := make(map[string]bool)
	for _, u := range fetchResp.GetUrls() {
		gotURLs[u.GetUrl()] = true
	}

	for _, s := range seeds {
		if !gotURLs[s.URL] {
			t.Errorf("missing expected URL %q in batch", s.URL)
		}
	}

	// 4. SubmitResults with discovered URLs (use distinct domains to avoid rate limiting).
	submitResp, err := env.client.SubmitResults(ctx, &pb.SubmitResultsRequest{
		WorkerId: "worker-full",
		Results: []*pb.CrawlResultEntry{
			{
				Url:            "https://alpha.example.com/page1",
				StatusCode:     200,
				DurationMs:     150,
				DiscoveredUrls: []string{"https://delta.example.com/discovered1", "https://epsilon.example.com/discovered2"},
			},
			{
				Url:        "https://beta.example.com/page2",
				StatusCode: 200,
				DurationMs: 100,
			},
			{
				Url:        "https://gamma.example.com/page3",
				StatusCode: 404,
				Error:      "not found",
				DurationMs: 50,
			},
		},
	})
	if err != nil {
		t.Fatalf("SubmitResults: %v", err)
	}

	if submitResp.GetAccepted() != 3 {
		t.Errorf("expected 3 accepted, got %d", submitResp.GetAccepted())
	}

	if submitResp.GetNewUrlsQueued() < 2 {
		t.Errorf("expected at least 2 new URLs queued, got %d", submitResp.GetNewUrlsQueued())
	}

	// 5. FetchBatch again — should get the discovered URLs
	fetchResp2, err := env.client.FetchBatch(ctx, &pb.FetchBatchRequest{
		WorkerId: "worker-full",
		MaxUrls:  10,
	})
	if err != nil {
		t.Fatalf("FetchBatch (round 2): %v", err)
	}

	if len(fetchResp2.GetUrls()) != 2 {
		t.Fatalf("expected 2 discovered URLs, got %d", len(fetchResp2.GetUrls()))
	}

	gotDiscovered := make(map[string]bool)
	for _, u := range fetchResp2.GetUrls() {
		gotDiscovered[u.GetUrl()] = true
	}

	if !gotDiscovered["https://delta.example.com/discovered1"] || !gotDiscovered["https://epsilon.example.com/discovered2"] {
		t.Errorf("discovered URLs mismatch: %v", gotDiscovered)
	}

	// 6. SwarmStatus
	statusResp, err := env.client.SwarmStatus(ctx, &pb.SwarmStatusRequest{})
	if err != nil {
		t.Fatalf("SwarmStatus: %v", err)
	}

	if statusResp.GetTotalWorkers() != 1 {
		t.Errorf("expected 1 total worker, got %d", statusResp.GetTotalWorkers())
	}

	if statusResp.GetUrlsProcessed() < 3 {
		t.Errorf("expected at least 3 processed URLs, got %d", statusResp.GetUrlsProcessed())
	}

	if len(statusResp.GetWorkers()) != 1 {
		t.Errorf("expected 1 worker info, got %d", len(statusResp.GetWorkers()))
	} else {
		w := statusResp.GetWorkers()[0]
		if w.GetWorkerId() != "worker-full" {
			t.Errorf("expected worker-full, got %s", w.GetWorkerId())
		}

		if w.GetProcessed() < 3 {
			t.Errorf("expected worker processed >= 3, got %d", w.GetProcessed())
		}
	}

	// 7. Leave
	_, err = env.client.LeaveSwarm(ctx, &pb.LeaveSwarmRequest{WorkerId: "worker-full"})
	if err != nil {
		t.Fatalf("LeaveSwarm: %v", err)
	}
}

func TestSwarmStatusNoCoordinator(t *testing.T) {
	// Create a server WITHOUT SetSwarm.
	srv := New()

	grpcServer := grpc.NewServer()
	pb.RegisterScoutServiceServer(grpcServer, srv)

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}

	go func() { _ = grpcServer.Serve(lis) }()

	t.Cleanup(grpcServer.Stop)

	conn, err := grpc.NewClient(
		lis.Addr().String(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}

	t.Cleanup(func() { _ = conn.Close() })

	client := pb.NewScoutServiceClient(conn)
	ctx := context.Background()

	_, err = client.SwarmStatus(ctx, &pb.SwarmStatusRequest{})
	if err == nil {
		t.Fatal("expected error when swarm coordinator is not configured")
	}

	s, ok := status.FromError(err)
	if !ok || s.Code() != codes.FailedPrecondition {
		t.Errorf("expected FailedPrecondition, got %v", err)
	}

	// Also verify JoinSwarm fails without coordinator.
	_, err = client.JoinSwarm(ctx, &pb.JoinSwarmRequest{WorkerId: "w1"})
	if err == nil {
		t.Fatal("expected error on JoinSwarm without coordinator")
	}

	s, ok = status.FromError(err)
	if !ok || s.Code() != codes.FailedPrecondition {
		t.Errorf("expected FailedPrecondition for JoinSwarm, got %v", err)
	}
}

func TestSwarmFetchBatchNotJoined(t *testing.T) {
	env := setupSwarmTestServer(t)
	ctx := context.Background()

	// FetchBatch without joining — should get Internal error (unknown worker).
	_, err := env.client.FetchBatch(ctx, &pb.FetchBatchRequest{
		WorkerId: "ghost-worker",
		MaxUrls:  5,
	})
	if err == nil {
		t.Fatal("expected error on FetchBatch without joining")
	}

	s, ok := status.FromError(err)
	if !ok || s.Code() != codes.Internal {
		t.Errorf("expected Internal error, got %v", err)
	}
}
