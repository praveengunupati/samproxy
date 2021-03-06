package sample

import (
	"crypto/sha1"
	"fmt"
	"math"

	"github.com/honeycombio/samproxy/config"
	"github.com/honeycombio/samproxy/logger"
	"github.com/honeycombio/samproxy/types"
)

// shardingSalt is a random bit to make sure we don't shard the same as any
// other sharding that uses the trace ID (eg deterministic sharding)
const shardingSalt = "5VQ8l2jE5aJLPVqk"

type DeterministicSampler struct {
	Config config.Config
	Logger logger.Logger

	sampleRate int
	upperBound uint32
	configName string
}

type DetSamplerConfig struct {
	SampleRate int
}

func (d *DeterministicSampler) Start() error {
	d.Logger.Debugf("Starting DeterministicSampler")
	defer func() { d.Logger.Debugf("Finished starting DeterministicSampler") }()
	if err := d.loadConfigs(); err != nil {
		return err
	}

	// listen for config reloads with an errorless version of the reload
	d.Config.RegisterReloadCallback(func() {
		d.Logger.Debugf("reloading deterministic sampler config")
		if err := d.loadConfigs(); err != nil {
			d.Logger.WithField("error", err).Errorf("failed to reload deterministic sampler configs")
		}
	})

	return nil
}

func (d *DeterministicSampler) loadConfigs() error {
	dsConfig := DetSamplerConfig{}
	configKey := fmt.Sprintf("SamplerConfig.%s", d.configName)
	err := d.Config.GetOtherConfig(configKey, &dsConfig)
	if err != nil {
		return err
	}
	if dsConfig.SampleRate < 1 {
		d.Logger.WithField("sample_rate", dsConfig.SampleRate).Debugf("configured sample rate for deterministic sampler was less than 1; forcing to 1")
		dsConfig.SampleRate = 1
	}
	d.sampleRate = dsConfig.SampleRate

	// Get the actual upper bound - the largest possible value divided by
	// the sample rate. In the case where the sample rate is 1, this should
	// sample every value.
	d.upperBound = math.MaxUint32 / uint32(d.sampleRate)
	return nil
}

func (d *DeterministicSampler) GetSampleRate(trace *types.Trace) (rate uint, keep bool) {
	if d.sampleRate <= 1 {
		return 1, true
	}
	sum := sha1.Sum([]byte(trace.TraceID + shardingSalt))
	v := bytesToUint32be(sum[:4])
	return uint(d.sampleRate), v <= d.upperBound
}

// bytesToUint32 takes a slice of 4 bytes representing a big endian 32 bit
// unsigned value and returns the equivalent uint32.
func bytesToUint32be(b []byte) uint32 {
	return uint32(b[3]) | (uint32(b[2]) << 8) | (uint32(b[1]) << 16) | (uint32(b[0]) << 24)
}
