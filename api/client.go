package api

import (
	"net"
	"context"
	"time"

	"github.com/Trojan-Qt5/go-shadowsocks2/stat"
	"google.golang.org/grpc"
)

type ClientAPIService struct {
	SSServiceServer
	meter         stat.TrafficMeter
	uploadSpeed   uint64
	downloadSpeed uint64
	lastSent      uint64
	lastRecv      uint64
	ctx           context.Context
}

func (s *ClientAPIService) QueryStats(ctx context.Context, req *StatsRequest) (*StatsReply, error) {
	sent, recv := s.meter.Query()
	reply := &StatsReply{
		UploadTraffic:   sent,
		DownloadTraffic: recv,
		UploadSpeed:     s.uploadSpeed,
		DownloadSpeed:   s.downloadSpeed,
	}
	return reply, nil
}

func (s *ClientAPIService) calcSpeed() {
	for {
		select {
		case <-time.After(time.Second):
			sent, recv := s.meter.Query()
			s.uploadSpeed = sent - s.lastSent
			s.downloadSpeed = recv - s.lastRecv
			s.lastSent = sent
			s.lastRecv = recv
		case <-s.ctx.Done():
			return
		}
	}
}

func RunClientAPIService(ctx context.Context, APIAddress string, meter stat.TrafficMeter) error {
	server := grpc.NewServer()
	service := &ClientAPIService{
		meter: meter,
		ctx:   ctx,
	}
	go service.calcSpeed()
	RegisterSSServiceServer(server, service)
	listener, err := net.Listen("tcp", APIAddress)
	if err != nil {
		return err
	}
	errChan := make(chan error, 1)
	go func() {
		errChan <- server.Serve(listener)
	}()
	select {
	case err := <-errChan:
		return err
	case <-ctx.Done():
		server.Stop()
		return nil
	}
}