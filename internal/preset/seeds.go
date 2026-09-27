package preset

// SeedPreset represents a curated world generation seed with rich metadata.
type SeedPreset struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Seed        string   `json:"seed"`
	Category    string   `json:"category"`
	Description string   `json:"description"`
	Biomes      []string `json:"biomes"`
	Features    []string `json:"features"`
	Difficulty  string   `json:"difficulty"`
	Icon        string   `json:"icon"`
}

// GetPopularSeeds returns a curated catalog of popular, high-value Minecraft Bedrock Edition seeds.
func GetPopularSeeds() []SeedPreset {
	return []SeedPreset{
		{
			ID:          "cherry-caldera",
			Name:        "Cherry Blossom Mountain Caldera",
			Seed:        "-8219986470354173872",
			Category:    "Flora & Scenic",
			Description: "Spectacular ring of jagged snow peaks and flowering cherry groves encircling a sunken flower valley with an isolated plains village safely tucked inside.",
			Biomes:      []string{"Cherry Grove", "Meadow", "Jagged Peaks", "Plains"},
			Features:    []string{"Mountain Ring", "Sunken Caldera", "Village in Crater", "Scenic Overlook"},
			Difficulty:  "Easy / Peaceful",
			Icon:        "cherry",
		},
		{
			ID:          "ancient-city-peak",
			Name:        "Ancient City Beneath Glacial Peaks",
			Seed:        "-3420545464665791887",
			Category:    "Deep Dark & Dungeons",
			Description: "Spawn directly at the foot of towering glacial summits with an expansive Ancient City structure generated straight below spawn at Y=-52.",
			Biomes:      []string{"Frozen Peaks", "Deep Dark", "Snowy Slopes", "Lush Caves"},
			Features:    []string{"Ancient City at Spawn", "Massive Mountain", "Deep Dark Warden Den", "Abundant Sculk"},
			Difficulty:  "Challenging",
			Icon:        "skull",
		},
		{
			ID:          "trial-chambers-lush",
			Name:        "1.21 Tricky Trials & Exposed Lush Chasm",
			Seed:        "16659",
			Category:    "1.21 Trials & Caves",
			Description: "Explore the new Minecraft 1.21 Trial Chambers seamlessly connected to an enormous open-air lush cave ravine right next to spawn coordinates.",
			Biomes:      []string{"Lush Caves", "Dripstone Caves", "Forest", "Plains"},
			Features:    []string{"1.21 Trial Chambers", "Exposed Lush Ravine", "Spore Blossoms", "Breeze Spawners"},
			Difficulty:  "Normal",
			Icon:        "zap",
		},
		{
			ID:          "mushroom-paradise",
			Name:        "Mooshroom Island Sanctuary",
			Seed:        "7755880011",
			Category:    "Builders Paradise",
			Description: "A massive mushroom island surrounded by tropical coral reefs. Hostile monsters never naturally spawn here, providing an idyllic, worry-free haven for mega-builders.",
			Biomes:      []string{"Mushroom Fields", "Warm Ocean", "Deep Ocean", "Coral Reef"},
			Features:    []string{"Zero Hostile Mob Spawns", "Giant Island", "Warm Water Coral", "Endless Safe Building"},
			Difficulty:  "Peaceful / Safe",
			Icon:        "sparkles",
		},
		{
			ID:          "triple-village-mansion",
			Name:        "Triple Village Coast & Woodland Mansion",
			Seed:        "570578392",
			Category:    "Villages & Exploration",
			Description: "A thriving coastal peninsula featuring three interconnected seaside villages with active iron golems, flanked by a Dark Oak forest housing an ominous Woodland Mansion.",
			Biomes:      []string{"Dark Forest", "Plains", "Coastal Ocean", "Birch Forest"},
			Features:    []string{"3x Coastal Villages", "Woodland Mansion", "Blacksmith Chests", "Natural Harbor"},
			Difficulty:  "Normal",
			Icon:        "castle",
		},
		{
			ID:          "badlands-desert-oasis",
			Name:        "Badlands Desert Oasis & Sunken Pyramid",
			Seed:        "-8471249764583274291",
			Category:    "Desert & Terracotta",
			Description: "Dramatic striated mesa canyons bordering a golden desert. Includes an intact desert temple pyramid, exposed surface gold mineshafts, and a river oasis village.",
			Biomes:      []string{"Badlands", "Wooded Badlands", "Desert", "Warm Ocean"},
			Features:    []string{"Exposed Gold Mineshafts", "Buried Desert Pyramid", "Multi-Color Terracotta", "Desert Village"},
			Difficulty:  "Normal",
			Icon:        "sun",
		},
		{
			ID:          "castaway-island",
			Name:        "Castaway Survival Island",
			Seed:        "-6465439446219808985",
			Category:    "Hardcore Survival",
			Description: "The quintessential castaway challenge! Spawn on a compact grassy island with a single lone tree, surrounded by open ocean, sunken shipwrecks, and distant monuments.",
			Biomes:      []string{"Deep Ocean", "Ocean Plains", "Warm Ocean", "Beach"},
			Features:    []string{"Isolated Castaway Island", "Single Oak Tree", "Sunken Shipwrecks", "Ocean Monument"},
			Difficulty:  "Hardcore / Expert",
			Icon:        "compass",
		},
		{
			ID:          "snowy-taiga-spikes",
			Name:        "Snowy Taiga Outpost & Ice Spikes",
			Seed:        "12000",
			Category:    "Winter Wonderland",
			Description: "A pristine winter landscape featuring towering azure ice spikes overlooking a cozy snowy spruce village, with an igloo containing a secret basement laboratory.",
			Biomes:      []string{"Ice Spikes", "Snowy Taiga", "Frozen River", "Snowy Plains"},
			Features:    []string{"Towering Ice Spikes", "Cozy Spruce Village", "Pillager Outpost", "Igloo with Secret Lab"},
			Difficulty:  "Normal",
			Icon:        "snowflake",
		},
		{
			ID:          "jungle-twin-temples",
			Name:        "Jungle River Valley & Twin Temples",
			Seed:        "78028",
			Category:    "Jungle Wilderness",
			Description: "Vibrant tropical wilderness bisected by a wide navigable river. Teeming with bamboo, wild pandas, and parrots alongside two ancient mossy jungle pyramids with trap chests.",
			Biomes:      []string{"Sparse Jungle", "Bamboo Jungle", "River", "Plains"},
			Features:    []string{"Wild Pandas & Parrots", "2x Jungle Pyramids", "Dense Bamboo", "High Treehouse Canopies"},
			Difficulty:  "Normal",
			Icon:        "trees",
		},
		{
			ID:          "mangrove-mire-hut",
			Name:        "Mangrove Mire & Witch Hut Cauldron",
			Seed:        "109312501",
			Category:    "Swamp & Nature",
			Description: "A tangled mangrove swamp with towering root networks, abundant mud flats, and friendly frogs, located right by a surface witch hut ideal for brewing and potion ingredients.",
			Biomes:      []string{"Mangrove Swamp", "Swamp", "Warm Ocean", "Dark Forest"},
			Features:    []string{"Deep Rooted Mangroves", "Surface Witch Hut", "Mud Flats & Frogs", "Warm Coastal Estuary"},
			Difficulty:  "Normal",
			Icon:        "droplets",
		},
	}
}
