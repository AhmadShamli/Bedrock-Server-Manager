package preset

// Preset represents a pre-configured template for server creation.
type Preset struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Mode        string            `json:"mode"`
	Difficulty  string            `json:"difficulty"`
	Properties  map[string]string `json:"properties"`
}

// GetPresets returns the standard creation presets.
func GetPresets() []Preset {
	return []Preset{
		{
			ID:          "survival",
			Name:        "Vanilla Survival",
			Description: "Classic Minecraft survival with normal difficulty and achievements enabled.",
			Mode:        "survival",
			Difficulty:  "normal",
			Properties: map[string]string{
				"gamemode":              "survival",
				"difficulty":            "normal",
				"allow-cheats":          "false",
				"max-players":           "10",
				"view-distance":         "32",
				"tick-distance":         "4",
				"player-idle-timeout":   "30",
				"default-player-permission-level": "member",
			},
		},
		{
			ID:          "creative",
			Name:        "Creative Sandbox",
			Description: "Peaceful building canvas with unlimited resources and cheats enabled.",
			Mode:        "creative",
			Difficulty:  "peaceful",
			Properties: map[string]string{
				"gamemode":              "creative",
				"difficulty":            "peaceful",
				"allow-cheats":          "true",
				"max-players":           "20",
				"view-distance":         "32",
				"default-player-permission-level": "operator",
			},
		},
		{
			ID:          "hardcore",
			Name:        "Hardcore Challenge",
			Description: "Tough survival experience with hard difficulty and locked settings.",
			Mode:        "survival",
			Difficulty:  "hard",
			Properties: map[string]string{
				"gamemode":              "survival",
				"difficulty":            "hard",
				"allow-cheats":          "false",
				"max-players":           "10",
				"view-distance":         "24",
				"default-player-permission-level": "member",
			},
		},
		{
			ID:          "pvp_arena",
			Name:        "Adventure & PvP Arena",
			Description: "Adventure mode designed for minigames, combat arenas, and custom maps.",
			Mode:        "adventure",
			Difficulty:  "normal",
			Properties: map[string]string{
				"gamemode":              "adventure",
				"difficulty":            "normal",
				"allow-cheats":          "false",
				"pvp":                   "true",
				"max-players":           "30",
			},
		},
	}
}

// GetPreset returns a preset by its ID.
func GetPreset(id string) *Preset {
	for _, p := range GetPresets() {
		if p.ID == id {
			return &p
		}
	}
	return nil
}
