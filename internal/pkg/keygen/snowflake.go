package keygen

import (
	"errors"
	"regexp"
	"sync"
	"time"

	"github.com/subhammahanty235/url-shortener/internal/pkg/base62"
)

const (
	EPoch         = int64(1704067200000)
	TimestampBits = 41
	MachineIDBits = 10
	SequenceBits  = 12

	MaxMachineID = (1 << MachineIDBits) - 1
	MaxSequence  = (1 << SequenceBits) - 1

	TimestampShift = MachineIDBits + SequenceBits
	MachineIDShift = SequenceBits
)

type SnowFlakeGenerator struct {
	mu            sync.Mutex
	machineID     int64
	sequence      int64
	lastTimestamp int64
	minLength     int
	maxLength     int
	customPattern *regexp.Regexp
}

type Config struct {
	MachineID int64
	MinLength int
	MaxLength int
}

func NewSnowflakeGenerator(cfg Config) (*SnowFlakeGenerator, error) {
	if cfg.MachineID < 0 || cfg.MachineID > MaxMachineID {
		return nil, errors.New("machine ID must be between 0 and 1023")
	}

	if cfg.MinLength == 0 {
		cfg.MinLength = 6
	}
	if cfg.MaxLength == 0 {
		cfg.MaxLength = 10
	}
	pattern := regexp.MustCompile(`^[a-zA-Z0-9]{` + string(rune('0'+cfg.MinLength)) + `,` + string(rune('0'+cfg.MaxLength)) + `}$`)
	return &SnowFlakeGenerator{
		machineID:     cfg.MachineID,
		sequence:      0,
		lastTimestamp: -1,
		minLength:     cfg.MinLength,
		maxLength:     cfg.MaxLength,
		customPattern: pattern,
	}, nil
}

func (g *SnowFlakeGenerator) Generate() (string, error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	timestamp := g.currentTimestamp()
	if timestamp < g.lastTimestamp {
		g.sequence = (g.sequence + 1) & MaxSequence
		if g.sequence == 0 {
			timestamp = g.waitNextMillis(g.lastTimestamp)
		} else {
			g.sequence = 0
		}
	}

	g.lastTimestamp = timestamp
	id := ((timestamp - EPoch) << TimestampShift) |
		(g.machineID << MachineIDShift) |
		g.sequence

	shortCode := base62.EncodePadded(uint64(id), g.minLength)
	return shortCode, nil

}

func (g *SnowFlakeGenerator) currentTimestamp() int64 {
	return time.Now().UnixNano() / int64(time.Millisecond)
}

func (g *SnowFlakeGenerator) waitNextMillis(lastTimestamp int64) int64 {
	timestamp := g.currentTimestamp()
	for timestamp <= lastTimestamp {
		time.Sleep(100 * time.Microsecond)
		timestamp = g.currentTimestamp()
	}
	return timestamp
}
