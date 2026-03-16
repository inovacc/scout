package server

import (
	"context"

	pb "github.com/inovacc/scout/grpc/scoutpb"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ════════════════════════ Swarm (Distributed Crawling) ════════════════════════

// JoinSwarm registers a worker with the swarm coordinator.
func (s *ScoutServer) JoinSwarm(_ context.Context, _ *pb.JoinSwarmRequest) (*pb.JoinSwarmResponse, error) {
	return nil, status.Error(codes.Unimplemented, "JoinSwarm not implemented")
}

// LeaveSwarm unregisters a worker from the swarm coordinator.
func (s *ScoutServer) LeaveSwarm(_ context.Context, _ *pb.LeaveSwarmRequest) (*pb.LeaveSwarmResponse, error) {
	return nil, status.Error(codes.Unimplemented, "LeaveSwarm not implemented")
}

// FetchBatch returns a batch of URLs for the worker to crawl.
func (s *ScoutServer) FetchBatch(_ context.Context, _ *pb.FetchBatchRequest) (*pb.FetchBatchResponse, error) {
	return nil, status.Error(codes.Unimplemented, "FetchBatch not implemented")
}

// SubmitResults accepts crawl results from a worker and enqueues discovered URLs.
func (s *ScoutServer) SubmitResults(_ context.Context, _ *pb.SubmitResultsRequest) (*pb.SubmitResultsResponse, error) {
	return nil, status.Error(codes.Unimplemented, "SubmitResults not implemented")
}

// SwarmStatus returns the current state of the swarm: workers, queue depth, and progress.
func (s *ScoutServer) SwarmStatus(_ context.Context, _ *pb.SwarmStatusRequest) (*pb.SwarmStatusResponse, error) {
	return nil, status.Error(codes.Unimplemented, "SwarmStatus not implemented")
}
