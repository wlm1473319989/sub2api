package domain

const (
	DefaultBackupFailoverFailureWindowSeconds    = 60
	DefaultBackupFailoverFailureThreshold        = 3
	DefaultBackupFailoverOpenCooldownSeconds     = 180
	DefaultBackupFailoverHalfOpenSuccessThreshold = 3
	DefaultBackupFailoverMaxOpenCooldownSeconds  = 1800
	DefaultBackupFailoverCooldownBackoffMultiplier = 2.0
)

type GroupBackupFailoverConfig struct {
	FailureWindowSeconds      int     `json:"failure_window_seconds,omitempty"`
	FailureThreshold          int     `json:"failure_threshold,omitempty"`
	OpenCooldownSeconds       int     `json:"open_cooldown_seconds,omitempty"`
	HalfOpenSuccessThreshold  int     `json:"half_open_success_threshold,omitempty"`
	MaxOpenCooldownSeconds    int     `json:"max_open_cooldown_seconds,omitempty"`
	CooldownBackoffMultiplier float64 `json:"cooldown_backoff_multiplier,omitempty"`
}

func DefaultGroupBackupFailoverConfig() GroupBackupFailoverConfig {
	return GroupBackupFailoverConfig{
		FailureWindowSeconds:      DefaultBackupFailoverFailureWindowSeconds,
		FailureThreshold:          DefaultBackupFailoverFailureThreshold,
		OpenCooldownSeconds:       DefaultBackupFailoverOpenCooldownSeconds,
		HalfOpenSuccessThreshold:  DefaultBackupFailoverHalfOpenSuccessThreshold,
		MaxOpenCooldownSeconds:    DefaultBackupFailoverMaxOpenCooldownSeconds,
		CooldownBackoffMultiplier: DefaultBackupFailoverCooldownBackoffMultiplier,
	}
}

func NormalizeGroupBackupFailoverConfig(cfg GroupBackupFailoverConfig) GroupBackupFailoverConfig {
	defaults := DefaultGroupBackupFailoverConfig()
	if cfg.FailureWindowSeconds <= 0 {
		cfg.FailureWindowSeconds = defaults.FailureWindowSeconds
	}
	if cfg.FailureThreshold <= 0 {
		cfg.FailureThreshold = defaults.FailureThreshold
	}
	if cfg.OpenCooldownSeconds <= 0 {
		cfg.OpenCooldownSeconds = defaults.OpenCooldownSeconds
	}
	if cfg.HalfOpenSuccessThreshold <= 0 {
		cfg.HalfOpenSuccessThreshold = defaults.HalfOpenSuccessThreshold
	}
	if cfg.MaxOpenCooldownSeconds <= 0 {
		cfg.MaxOpenCooldownSeconds = defaults.MaxOpenCooldownSeconds
	}
	if cfg.MaxOpenCooldownSeconds < cfg.OpenCooldownSeconds {
		cfg.MaxOpenCooldownSeconds = cfg.OpenCooldownSeconds
	}
	if cfg.CooldownBackoffMultiplier <= 0 {
		cfg.CooldownBackoffMultiplier = defaults.CooldownBackoffMultiplier
	}
	return cfg
}
