package main

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"sync/atomic"

	"github.com/0xKhennati/bundle-broadcaster/strategies"
	"github.com/gorilla/websocket"
	"github.com/rs/zerolog"
)

const (
	workerCount   = 256
	queueCapacity = 4096
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type WSServer struct {
	manager *RelayManager
	logger  zerolog.Logger
	queue   chan *strategies.IncomingBundle
	wg      sync.WaitGroup
	closed  atomic.Bool
	ctx     context.Context
	cancel  context.CancelFunc
}

func NewWSServer(manager *RelayManager, logger zerolog.Logger) *WSServer {
	ctx, cancel := context.WithCancel(context.Background())
	return &WSServer{
		manager: manager,
		logger:  logger,
		queue:   make(chan *strategies.IncomingBundle, queueCapacity),
		ctx:     ctx,
		cancel:  cancel,
	}
}

func (s *WSServer) Start() {
	for i := 0; i < workerCount; i++ {
		s.wg.Add(1)
		go s.worker(i)
	}
	s.logger.Info().Int("workers", workerCount).Msg("WebSocket server started")
}

func (s *WSServer) worker(id int) {
	defer s.wg.Done()
	for {
		select {
		case <-s.ctx.Done():
			s.logger.Info().Int("worker", id).Msg("WebSocket server worker shutting down")
			return
		case bundle, ok := <-s.queue:
			if !ok {
				return
			}
			s.manager.Broadcast(s.ctx, bundle)
		}
	}
}

func (s *WSServer) HandleWS(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		s.logger.Error().Err(err).Msg("WebSocket upgrade failed")
		return
	}
	defer conn.Close()

	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
				s.logger.Debug().Msg("WebSocket connection closed normally")
			} else {
				s.logger.Error().Err(err).Msg("WebSocket read error")
			}
			return
		}

		var bundle strategies.IncomingBundle
		if err := json.Unmarshal(message, &bundle); err != nil {
			s.logger.Error().Err(err).Str("raw", string(message)).Msg("invalid bundle JSON")
			continue
		}

		s.logger.Debug().
			Str("bundle_id", bundle.BundleID).
			Str("strategy_type", bundle.StrategyType).
			Uint64("target_block", bundle.TargetBlock).
			Str("target_tx_hash", bundle.TargetTxHash).
			Strs("raw_txs", bundle.RawTxs).
			Uint64("min_timestamp", bundle.MinTimestamp).
			Uint64("max_timestamp", bundle.MaxTimestamp).
			Strs("reverting_tx_hashes", bundle.RevertingTxHashes).
			Strs("target_pools", bundle.TargetPools).
			Msg("bundle received")

		BundleReceivedTotal.Inc()

		if s.closed.Load() {
			s.logger.Warn().Str("bundle_id", bundle.BundleID).Msg("dropping bundle during shutdown")
			continue
		}

		select {
		case s.queue <- &bundle:
		default:
			s.logger.Warn().Str("bundle_id", bundle.BundleID).Msg("queue full, dropping bundle")
		}
	}
}

func (s *WSServer) Shutdown(ctx context.Context) error {
	s.closed.Store(true)
	s.cancel()
	close(s.queue)
	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-done:
		return nil
	}
}
