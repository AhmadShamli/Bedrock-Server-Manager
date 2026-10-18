package bds

import (
	"context"
	"errors"
	"time"
)

var (
	// ErrEULANotAccepted is returned when BDS binary download is attempted without accepting the EULA.
	ErrEULANotAccepted = errors.New("Minecraft Bedrock Server EULA and Microsoft Privacy Policy must be accepted before downloading official BDS binaries")
)

const (
	SettingEULAAccepted   = "bds_eula_accepted"
	SettingEULAAcceptedAt = "bds_eula_accepted_at"
	SettingEULAAcceptedBy = "bds_eula_accepted_by"

	MinecraftEULAURL       = "https://www.minecraft.net/en-us/eula"
	MicrosoftPrivacyURL    = "https://privacy.microsoft.com/en-us/privacystatement"
)

// SettingGetter abstracts retrieval of configuration settings.
type SettingGetter interface {
	GetSetting(ctx context.Context, key string) (string, error)
}

// SettingSetter abstracts storage of configuration settings.
type SettingSetter interface {
	SetSetting(ctx context.Context, key, value string) error
}

// IsEULAAccepted checks whether the Minecraft EULA has been accepted in settings.
func IsEULAAccepted(ctx context.Context, db SettingGetter) (bool, error) {
	if db == nil {
		return false, nil
	}
	val, err := db.GetSetting(ctx, SettingEULAAccepted)
	if err != nil {
		return false, nil
	}
	return val == "true" || val == "1", nil
}

// AcceptEULA records acceptance of the Minecraft EULA and Privacy Policy.
func AcceptEULA(ctx context.Context, db SettingSetter, username string) error {
	if db == nil {
		return errors.New("database setter cannot be nil")
	}
	if err := db.SetSetting(ctx, SettingEULAAccepted, "true"); err != nil {
		return err
	}
	if err := db.SetSetting(ctx, SettingEULAAcceptedAt, time.Now().UTC().Format(time.RFC3339)); err != nil {
		return err
	}
	if username != "" {
		if err := db.SetSetting(ctx, SettingEULAAcceptedBy, username); err != nil {
			return err
		}
	}
	return nil
}
