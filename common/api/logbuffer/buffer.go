package logbuffer

import (
	"context"
	"sync"
	"time"

	"github.com/monobilisim/monokit/common/api/models"
	"github.com/rs/zerolog/log"
	"gorm.io/gorm"
)

// Buffer is a thread-safe, in-memory buffer for host logs.
// It automatically flushes logs to the database in batches.
type Buffer struct {
	mu       sync.Mutex
	db       *gorm.DB
	cfg      Config
	entries  []models.HostLog
	ticker   *time.Ticker
	quitChan chan struct{}
}

// NewBuffer creates a new log buffer.
func NewBuffer(db *gorm.DB, cfg Config) *Buffer {
	return &Buffer{
		db:       db,
		cfg:      cfg,
		entries:  make([]models.HostLog, 0, cfg.BatchSize),
		quitChan: make(chan struct{}),
	}
}

// Add adds a log entry to the buffer. If the buffer reaches its batch size,
// it triggers an immediate flush.
func (b *Buffer) Add(entry models.HostLog) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.entries = append(b.entries, entry)

	if len(b.entries) >= b.cfg.BatchSize {
		// Immediately flush a full batch
		b.flushLocked()
	}
}

// Start begins the buffer's background processing goroutine, which periodically
// flushes the buffer.
func (b *Buffer) Start() {
	b.ticker = time.NewTicker(b.cfg.FlushInterval)
	go b.loop()
}

// Close gracefully shuts down the buffer, flushing any remaining logs.
func (b *Buffer) Close() {
	close(b.quitChan)
}

// loop is the main background routine that triggers flushes.
func (b *Buffer) loop() {
	defer b.ticker.Stop()
	for {
		select {
		case <-b.ticker.C:
			b.Flush()
		case <-b.quitChan:
			log.Info().Msg("Log buffer shutting down, flushing remaining entries...")
			b.Flush()
			log.Info().Msg("Log buffer shutdown complete.")
			return
		}
	}
}

// Flush writes all pending log entries from the buffer to the database.
// This function is safe to call concurrently.
func (b *Buffer) Flush() {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.flushLocked()
}

// flushLocked performs the actual flush operation. It must be called with the
// mutex held.
func (b *Buffer) flushLocked() {
	if len(b.entries) == 0 {
		return
	}

	// Create a copy of the entries to be flushed
	toFlush := make([]models.HostLog, len(b.entries))
	copy(toFlush, b.entries)

	// Clear the buffer's entries slice
	b.entries = b.entries[:0]

	// Release the lock while flushing to the database
	b.mu.Unlock()
	defer b.mu.Lock()

	log.Debug().Int("count", len(toFlush)).Msg("Flushing log entries to database")

	// Use a background context for the database operation
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := b.db.WithContext(ctx).CreateInBatches(toFlush, b.cfg.BatchSize).Error; err != nil {
		log.Error().Err(err).Msg("Failed to flush log buffer to database")

		// If the flush fails, re-queue the logs.
		// A more robust implementation might use a persistent queue or a dead-letter queue.
		b.mu.Lock()
		b.entries = append(toFlush, b.entries...)
		// To prevent a runaway loop, we could add a backoff mechanism here.
		// For now, we'll just re-queue and let the next tick handle it.
		b.mu.Unlock()
	}
}
