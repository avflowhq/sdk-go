package avflow

import "testing"

func TestAudioLevelConfigDefaults(t *testing.T) {
	c := &AudioLevelConfig{Enabled: true}
	c.SetDefaults()
	if c.Levels != DefaultAudioLevels {
		t.Fatalf("levels = %d, want %d", c.Levels, DefaultAudioLevels)
	}
	if c.IntervalMs != DefaultLevelIntervalMs {
		t.Fatalf("intervalMs = %d, want %d", c.IntervalMs, DefaultLevelIntervalMs)
	}
	if c.FloorDb != DefaultLevelFloorDb {
		t.Fatalf("floorDb = %d, want %d", c.FloorDb, DefaultLevelFloorDb)
	}
}

func TestAudioLevelConfigClamp(t *testing.T) {
	c := &AudioLevelConfig{Levels: 1, IntervalMs: 10, FloorDb: 50}
	c.SetDefaults()
	if c.Levels != MinAudioLevels {
		t.Fatalf("levels not clamped up: %d", c.Levels)
	}
	if c.IntervalMs != MinLevelIntervalMs {
		t.Fatalf("intervalMs not clamped up: %d", c.IntervalMs)
	}
	if c.FloorDb != -50 {
		t.Fatalf("positive floorDb not negated: %d", c.FloorDb)
	}

	c2 := &AudioLevelConfig{Levels: 9999, IntervalMs: 99999}
	c2.SetDefaults()
	if c2.Levels != MaxAudioLevels {
		t.Fatalf("levels not clamped down: %d", c2.Levels)
	}
	if c2.IntervalMs != MaxLevelIntervalMs {
		t.Fatalf("intervalMs not clamped down: %d", c2.IntervalMs)
	}
}

func TestAudioMixerConfigPropagatesAudioLevels(t *testing.T) {
	c := &AudioMixerConfig{AudioLevels: &AudioLevelConfig{Enabled: true}}
	c.SetDefaults()
	if c.AudioLevels.Levels != DefaultAudioLevels {
		t.Fatalf("nested SetDefaults not applied: %d", c.AudioLevels.Levels)
	}
}
