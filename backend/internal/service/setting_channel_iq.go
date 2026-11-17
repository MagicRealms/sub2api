package service

import (
	"context"
	"fmt"
	"strconv"
)

// GetChannelIQVisibility defaults to visible for existing installations. Storage
// errors propagate so a hidden page never becomes public on a failed lookup.
func (s *SettingService) GetChannelIQVisibility(ctx context.Context) (bool, error) {
	values, err := s.settingRepo.GetMultiple(ctx, []string{SettingKeyChannelIQEnabled})
	if err != nil {
		return false, fmt.Errorf("read channel IQ visibility: %w", err)
	}
	return !isFalseSettingValue(values[SettingKeyChannelIQEnabled]), nil
}

func (s *SettingService) SetChannelIQVisibility(ctx context.Context, visible bool) error {
	if err := s.settingRepo.Set(ctx, SettingKeyChannelIQEnabled, strconv.FormatBool(visible)); err != nil {
		return fmt.Errorf("save channel IQ visibility: %w", err)
	}
	if s.onUpdate != nil {
		s.onUpdate()
	}
	return nil
}
