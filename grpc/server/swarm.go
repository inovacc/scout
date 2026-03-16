package server

import (
	"context"
	"encoding/json"
	"time"

	pb "github.com/inovacc/scout/grpc/scoutpb"
	"github.com/inovacc/scout/internal/engine/swarm"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ════════════════════════ Swarm (Distributed Crawling) ════════════════════════

func (s *ScoutServer) requireSwarm() (*swarm.Coordinator, error) {
	if s.swarmCoord == nil {
		return nil, status.Error(codes.FailedPrecondition, "swarm coordinator not configured")
	}
	return s.swarmCoord, nil
}

// JoinSwarm registers a worker with the swarm coordinator.
func (s *ScoutServer) JoinSwarm(_ context.Context, req *pb.JoinSwarmRequest) (*pb.JoinSwarmResponse, error) {
	coord, err := s.requireSwarm()
	if err != nil {
		return nil, err
	}

	if err := coord.RegisterWorker(req.GetWorkerId(), req.GetProxy()); err != nil {
		return &pb.JoinSwarmResponse{
			Accepted: false,
			Message:  err.Error(),
		}, nil
	}

	return &pb.JoinSwarmResponse{
		Accepted:            true,
		Message:             "worker registered",
		BatchSize:           int32(coord.Config().BatchSize),
		HeartbeatIntervalMs: int32(coord.Config().HeartbeatInterval / time.Millisecond),
	}, nil
}

// LeaveSwarm unregisters a worker from the swarm coordinator.
func (s *ScoutServer) LeaveSwarm(_ context.Context, req *pb.LeaveSwarmRequest) (*pb.LeaveSwarmResponse, error) {
	coord, err := s.requireSwarm()
	if err != nil {
		return nil, err
	}

	requeueCount := coord.InFlightCount(req.GetWorkerId())

	if err := coord.UnregisterWorker(req.GetWorkerId()); err != nil {
		return nil, status.Errorf(codes.NotFound, "leave swarm: %v", err)
	}

	return &pb.LeaveSwarmResponse{
		Acknowledged: true,
		UrlsRequeued: int32(requeueCount),
	}, nil
}

// FetchBatch returns a batch of URLs for the worker to crawl.
func (s *ScoutServer) FetchBatch(_ context.Context, req *pb.FetchBatchRequest) (*pb.FetchBatchResponse, error) {
	coord, err := s.requireSwarm()
	if err != nil {
		return nil, err
	}

	batchSize := int(req.GetMaxUrls())
	reqs, err := coord.Dequeue(req.GetWorkerId(), batchSize)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "fetch batch: %v", err)
	}

	urls := make([]*pb.CrawlURL, 0, len(reqs))
	for _, r := range reqs {
		urls = append(urls, &pb.CrawlURL{
			Url:    r.URL,
			Depth:  int32(r.Depth),
			Domain: r.Domain,
		})
	}

	return &pb.FetchBatchResponse{
		Urls:  urls,
		Drain: len(urls) == 0,
	}, nil
}

// SubmitResults accepts crawl results from a worker and enqueues discovered URLs.
func (s *ScoutServer) SubmitResults(_ context.Context, req *pb.SubmitResultsRequest) (*pb.SubmitResultsResponse, error) {
	coord, err := s.requireSwarm()
	if err != nil {
		return nil, err
	}

	results := make([]swarm.CrawlResult, 0, len(req.GetResults()))
	for _, r := range req.GetResults() {
		cr := swarm.CrawlResult{
			URL:            r.GetUrl(),
			StatusCode:     int(r.GetStatusCode()),
			Error:          r.GetError(),
			DiscoveredURLs: r.GetDiscoveredUrls(),
			Duration:       time.Duration(r.GetDurationMs() * float64(time.Millisecond)),
		}

		if dataJSON := r.GetDataJson(); dataJSON != "" {
			var data map[string]any
			if err := json.Unmarshal([]byte(dataJSON), &data); err == nil {
				cr.Data = data
			}
		}

		results = append(results, cr)
	}

	if err := coord.SubmitResults(req.GetWorkerId(), results); err != nil {
		return nil, status.Errorf(codes.Internal, "submit results: %v", err)
	}

	return &pb.SubmitResultsResponse{
		Accepted:      int32(len(results)),
		NewUrlsQueued: int32(coord.QueueLen()),
	}, nil
}

// SwarmStatus returns the current state of the swarm: workers, queue depth, and progress.
func (s *ScoutServer) SwarmStatus(_ context.Context, _ *pb.SwarmStatusRequest) (*pb.SwarmStatusResponse, error) {
	coord, err := s.requireSwarm()
	if err != nil {
		return nil, err
	}

	workers := coord.Workers()
	var active int32
	var totalProcessed int64
	pbWorkers := make([]*pb.SwarmWorkerInfo, 0, len(workers))

	for _, w := range workers {
		if w.Status == swarm.WorkerBusy {
			active++
		}
		totalProcessed += w.Processed

		pbWorkers = append(pbWorkers, &pb.SwarmWorkerInfo{
			WorkerId:     w.ID,
			Status:       w.Status.String(),
			Proxy:        w.Proxy,
			Processed:    w.Processed,
			LastSeenUnix: w.LastSeen.Unix(),
		})
	}

	return &pb.SwarmStatusResponse{
		TotalWorkers:  int32(len(workers)),
		ActiveWorkers: active,
		UrlsQueued:    int64(coord.QueueLen()),
		UrlsProcessed: totalProcessed,
		Workers:       pbWorkers,
	}, nil
}
