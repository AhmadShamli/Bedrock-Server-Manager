import React, { useState, useEffect } from 'react';
import { SeedPreset } from '../types';
import { api } from '../api/client';
import { Search, Sparkles, Copy, Check, Compass, Mountain, Trees, ShieldAlert, Castle, Sun, Snowflake, Droplets, Zap, CheckCircle2 } from 'lucide-react';

interface PopularSeedPickerProps {
  selectedSeed: string;
  onSelectSeed: (seed: string, preset?: SeedPreset) => void;
  onClose?: () => void;
  inline?: boolean;
}

const FALLBACK_SEEDS: SeedPreset[] = [
  // 1-10: Featured Classics & Flagships
  {
    id: "cherry-caldera",
    name: "Cherry Blossom Mountain Caldera",
    seed: "-8219986470354173872",
    category: "Flora & Scenic",
    description: "Spectacular ring of jagged snow peaks and flowering cherry groves encircling a sunken flower valley with an isolated plains village safely tucked inside.",
    biomes: ["Cherry Grove", "Meadow", "Jagged Peaks", "Plains"],
    features: ["Mountain Ring", "Sunken Caldera", "Village in Crater", "Scenic Overlook"],
    difficulty: "Easy / Peaceful",
    icon: "cherry",
  },
  {
    id: "ancient-city-peak",
    name: "Ancient City Beneath Glacial Peaks",
    seed: "-3420545464665791887",
    category: "Deep Dark & Dungeons",
    description: "Spawn directly at the foot of towering glacial summits with an expansive Ancient City structure generated straight below spawn at Y=-52.",
    biomes: ["Frozen Peaks", "Deep Dark", "Snowy Slopes", "Lush Caves"],
    features: ["Ancient City at Spawn", "Massive Mountain", "Deep Dark Warden Den", "Abundant Sculk"],
    difficulty: "Challenging",
    icon: "skull",
  },
  {
    id: "trial-chambers-lush",
    name: "1.21 Tricky Trials & Exposed Lush Chasm",
    seed: "16659",
    category: "1.21 Trials & Caves",
    description: "Explore the new Minecraft 1.21 Trial Chambers seamlessly connected to an enormous open-air lush cave ravine right next to spawn coordinates.",
    biomes: ["Lush Caves", "Dripstone Caves", "Forest", "Plains"],
    features: ["1.21 Trial Chambers", "Exposed Lush Ravine", "Spore Blossoms", "Breeze Spawners"],
    difficulty: "Normal",
    icon: "zap",
  },
  {
    id: "mushroom-paradise",
    name: "Mooshroom Island Sanctuary",
    seed: "7755880011",
    category: "Builders Paradise",
    description: "A massive mushroom island surrounded by tropical coral reefs. Hostile monsters never naturally spawn here, providing an idyllic, worry-free haven for mega-builders.",
    biomes: ["Mushroom Fields", "Warm Ocean", "Deep Ocean", "Coral Reef"],
    features: ["Zero Hostile Mob Spawns", "Giant Island", "Warm Water Coral", "Endless Safe Building"],
    difficulty: "Peaceful / Safe",
    icon: "sparkles",
  },
  {
    id: "triple-village-mansion",
    name: "Triple Village Coast & Woodland Mansion",
    seed: "570578392",
    category: "Villages & Exploration",
    description: "A thriving coastal peninsula featuring three interconnected seaside villages with active iron golems, flanked by a Dark Oak forest housing an ominous Woodland Mansion.",
    biomes: ["Dark Forest", "Plains", "Coastal Ocean", "Birch Forest"],
    features: ["3x Coastal Villages", "Woodland Mansion", "Blacksmith Chests", "Natural Harbor"],
    difficulty: "Normal",
    icon: "castle",
  },
  {
    id: "badlands-desert-oasis",
    name: "Badlands Desert Oasis & Sunken Pyramid",
    seed: "-8471249764583274291",
    category: "Desert & Terracotta",
    description: "Dramatic striated mesa canyons bordering a golden desert. Includes an intact desert temple pyramid, exposed surface gold mineshafts, and a river oasis village.",
    biomes: ["Badlands", "Wooded Badlands", "Desert", "Warm Ocean"],
    features: ["Exposed Gold Mineshafts", "Buried Desert Pyramid", "Multi-Color Terracotta", "Desert Village"],
    difficulty: "Normal",
    icon: "sun",
  },
  {
    id: "castaway-island",
    name: "Castaway Survival Island",
    seed: "-6465439446219808985",
    category: "Hardcore Survival",
    description: "The quintessential castaway challenge! Spawn on a compact grassy island with a single lone tree, surrounded by open ocean, sunken shipwrecks, and distant monuments.",
    biomes: ["Deep Ocean", "Ocean Plains", "Warm Ocean", "Beach"],
    features: ["Isolated Castaway Island", "Single Oak Tree", "Sunken Shipwrecks", "Ocean Monument"],
    difficulty: "Hardcore / Expert",
    icon: "compass",
  },
  {
    id: "snowy-taiga-spikes",
    name: "Snowy Taiga Outpost & Ice Spikes",
    seed: "12000",
    category: "Winter Wonderland",
    description: "A pristine winter landscape featuring towering azure ice spikes overlooking a cozy snowy spruce village, with an igloo containing a secret basement laboratory.",
    biomes: ["Ice Spikes", "Snowy Taiga", "Frozen River", "Snowy Plains"],
    features: ["Towering Ice Spikes", "Cozy Spruce Village", "Pillager Outpost", "Igloo with Secret Lab"],
    difficulty: "Normal",
    icon: "snowflake",
  },
  {
    id: "jungle-twin-temples",
    name: "Jungle River Valley & Twin Temples",
    seed: "78028",
    category: "Jungle Wilderness",
    description: "Vibrant tropical wilderness bisected by a wide navigable river. Teeming with bamboo, wild pandas, and parrots alongside two ancient mossy jungle pyramids with trap chests.",
    biomes: ["Sparse Jungle", "Bamboo Jungle", "River", "Plains"],
    features: ["Wild Pandas & Parrots", "2x Jungle Pyramids", "Dense Bamboo", "High Treehouse Canopies"],
    difficulty: "Normal",
    icon: "trees",
  },
  {
    id: "mangrove-mire-hut",
    name: "Mangrove Mire & Witch Hut Cauldron",
    seed: "109312501",
    category: "Swamp & Nature",
    description: "A tangled mangrove swamp with towering root networks, abundant mud flats, and friendly frogs, located right by a surface witch hut ideal for brewing and potion ingredients.",
    biomes: ["Mangrove Swamp", "Swamp", "Warm Ocean", "Dark Forest"],
    features: ["Deep Rooted Mangroves", "Surface Witch Hut", "Mud Flats & Frogs", "Warm Coastal Estuary"],
    difficulty: "Normal",
    icon: "droplets",
  },

  // 11-20: Mountains, Villages & Dungeons
  {
    id: "hollow-mountain-crater",
    name: "Colossal Hollow Mountain Caved-In Caldera",
    seed: "8486670481758381904",
    category: "Mountains & Cliffs",
    description: "An immense mountain whose center completely collapsed into a hollow subterranean cave basin, naturally lit by cascading skylights and waterfalls.",
    biomes: ["Jagged Peaks", "Meadow", "Lush Caves", "Stony Peaks"],
    features: ["Hollow Interior Mountain", "Waterfalls", "Skylights", "Natural Base Arena"],
    difficulty: "Normal",
    icon: "mountain",
  },
  {
    id: "quad-village-spawn",
    name: "Quadruple Village Crossroads",
    seed: "-437946913",
    category: "Villages & Exploration",
    description: "Spawn at the confluence of four distinct villages (Plains, Desert, Savanna, and Taiga) within walking distance of each other with thriving trade routes.",
    biomes: ["Plains", "Desert", "Savanna", "Taiga"],
    features: ["4x Unique Villages", "Active Trading Hub", "Iron Golems", "Diverse Architecture"],
    difficulty: "Easy / Beginner",
    icon: "castle",
  },
  {
    id: "cherry-trial-combo",
    name: "Cherry Blossom Cliff & 1.21 Trial Chamber",
    seed: "906416410424214",
    category: "1.21 Trials & Caves",
    description: "A cliffside pink cherry grove with an exposed Trial Chamber entrance embedded right in the stone face below, offering instant access to breeze spawners.",
    biomes: ["Cherry Grove", "Meadow", "Stony Shore", "Ocean"],
    features: ["Exposed Trial Chamber", "Pink Forest Cliffs", "Breeze Vaults", "Oceanfront View"],
    difficulty: "Normal",
    icon: "zap",
  },
  {
    id: "mansion-at-spawn",
    name: "Woodland Mansion at Direct Spawn",
    seed: "88888888888888888",
    category: "Villages & Exploration",
    description: "Spawn right on the front door steps of a towering multi-story Woodland Mansion in a dark oak forest. High risk, high reward early game challenge!",
    biomes: ["Dark Forest", "Plains", "Forest"],
    features: ["Mansion at 0,0", "Totem of Undying", "Vindicator Loot", "Dark Forest Canopy"],
    difficulty: "Hardcore / Expert",
    icon: "castle",
  },
  {
    id: "survival-island-monument",
    name: "Survival Island with Ocean Monument & Shipwreck",
    seed: "-1867169426",
    category: "Hardcore Survival",
    description: "A tiny sandy survival island next to an exposed surface shipwreck and an imposing Ocean Monument guarding underwater gold and sponge rooms.",
    biomes: ["Deep Ocean", "Ocean", "Beach"],
    features: ["Elder Guardians", "Exposed Shipwreck", "Prismarine Monument", "Open Sea"],
    difficulty: "Hard",
    icon: "compass",
  },
  {
    id: "deep-dark-crater",
    name: "Sunken Ancient City in Open Sinkhole",
    seed: "2041270273",
    category: "Deep Dark & Dungeons",
    description: "A massive surface sinkhole dropping straight down hundreds of blocks into an Ancient City, allowing daylight to beam onto sculk shriekers and warden grounds.",
    biomes: ["Plains", "Deep Dark", "Dripstone Caves"],
    features: ["Direct Vertical Sinkhole", "Ancient City", "Skylit Warden Lair", "Sculk Catalysts"],
    difficulty: "Challenging",
    icon: "skull",
  },
  {
    id: "flower-forest-valley",
    name: "Enchanted Flower Forest & Meadow Plateau",
    seed: "4466332211",
    category: "Flora & Scenic",
    description: "A vibrant sea of allium, tulips, and orchids covering rolling hills surrounded by high snowy mountain ridges and bee nests everywhere.",
    biomes: ["Flower Forest", "Meadow", "Snowy Slopes", "Forest"],
    features: ["Abundant Bee Hives", "All Flower Species", "High Mountain Backdrop", "Peaceful Plateau"],
    difficulty: "Peaceful / Easy",
    icon: "cherry",
  },
  {
    id: "desert-pyramid-village",
    name: "Desert Temple Merged Inside Sand Village",
    seed: "-2028232924",
    category: "Desert & Terracotta",
    description: "A desert village whose centerpiece is a buried desert pyramid with its treasure vaults sitting directly under the central village well.",
    biomes: ["Desert", "Badlands", "River"],
    features: ["Pyramid in Town Square", "4x Treasure Chests", "Camel Pens", "Desert Blacksmith"],
    difficulty: "Easy",
    icon: "sun",
  },
  {
    id: "mega-badlands-canyon",
    name: "Bryce Canyon Spired Badlands & Surface Mines",
    seed: "-5492817491028471928",
    category: "Desert & Terracotta",
    description: "Spectacular eroded badlands spikes towering like Bryce Canyon, filled with exposed surface gold ore and abandoned mineshaft tracks.",
    biomes: ["Eroded Badlands", "Badlands", "Wooded Badlands"],
    features: ["Eroded Terracotta Spikes", "Exposed Surface Gold", "Mineshaft Network", "Mesa Plateau"],
    difficulty: "Normal",
    icon: "sun",
  },
  {
    id: "snowy-island-fortress",
    name: "Glacial Iceberg Island with Igloo Lab",
    seed: "-29840134958172",
    category: "Winter Wonderland",
    description: "A remote frozen archipelago surrounded by icebergs, frozen shipwrecks, and an igloo containing a secret brewing stand and zombie cure lab.",
    biomes: ["Frozen Ocean", "Snowy Plains", "Ice Spikes"],
    features: ["Igloo with Basement Lab", "Pack Ice Icebergs", "Sunken Frozen Galleon", "Polar Bears"],
    difficulty: "Normal",
    icon: "snowflake",
  },

  // 21-30: Caves, Coves & Rare Biomes
  {
    id: "lush-caves-sinkhole",
    name: "Massive Lush Cave Sinkhole in Plains",
    seed: "6473829104",
    category: "1.21 Trials & Caves",
    description: "A circular surface collapse surrounded by plains flowers opening up to show glowing spore blossoms, lush moss carpets, and axolotl pools.",
    biomes: ["Plains", "Lush Caves", "Forest"],
    features: ["Surface Lush Cave", "Glow Berry Ceilings", "Axolotl Spawns", "Underground Lakes"],
    difficulty: "Normal",
    icon: "zap",
  },
  {
    id: "bamboo-jungle-cove",
    name: "Secluded Bamboo Jungle Bay & Pirate Ship",
    seed: "2840184719",
    category: "Jungle Wilderness",
    description: "A secret ocean cove flanked by vertical jungle cliffs, lush bamboo, and a fully intact beached pirate ship loaded with treasure maps.",
    biomes: ["Bamboo Jungle", "Jungle", "Warm Ocean", "Beach"],
    features: ["Beached Pirate Galleon", "Wild Pandas", "Secret Cove Lagoon", "Bamboo Canopies"],
    difficulty: "Normal",
    icon: "trees",
  },
  {
    id: "pillager-outpost-village",
    name: "Pillager Outpost Siege on Hilltop Village",
    seed: "5142789123",
    category: "Villages & Exploration",
    description: "An intense tactical setup: an ominous Pillager Outpost stands directly opposite a fortified hilltop plains village with active iron golems.",
    biomes: ["Plains", "Meadow", "Forest"],
    features: ["Outpost Overlooking Town", "Allay Cages", "Iron Golems", "Raid Starting Point"],
    difficulty: "Challenging",
    icon: "castle",
  },
  {
    id: "coral-atoll-kingdom",
    name: "Tropical Coral Atoll & Warm Lagoon",
    seed: "99999999999",
    category: "Builders Paradise",
    description: "A circular ring of white sand dunes enclosing a shallow turquoise lagoon teeming with tropical fish, sea pickles, and vibrant coral reefs.",
    biomes: ["Warm Ocean", "Coral Reef", "Beach"],
    features: ["Circular Coral Atoll", "Tropical Fish", "Sea Turtles", "Lagoon Water"],
    difficulty: "Peaceful / Safe",
    icon: "sparkles",
  },
  {
    id: "twin-ancient-cities",
    name: "Twin Ancient Cities Beneath Dual Mountain Peaks",
    seed: "-721998471932014",
    category: "Deep Dark & Dungeons",
    description: "Two interconnected Ancient City complexes spanning over 300 blocks under twin snowy mountain crests, offering double the loot and challenge.",
    biomes: ["Frozen Peaks", "Deep Dark", "Snowy Slopes"],
    features: ["2x Linked Ancient Cities", "Double Warden Portals", "Silence Armor Trims", "Echo Shards"],
    difficulty: "Hardcore / Expert",
    icon: "skull",
  },
  {
    id: "spruce-taiga-river",
    name: "Old Growth Pine Taiga & Grand Canyon River",
    seed: "304918273645",
    category: "Flora & Scenic",
    description: "Vast dense old growth spruce wilderness with podzol ground and giant mossy boulders lining an immense river canyon.",
    biomes: ["Old Growth Pine Taiga", "Taiga", "River", "Plains"],
    features: ["Giant Redwood-Style Pines", "Deep River Canyon", "Mossy Cobblestone Boulders", "Campfire Haven"],
    difficulty: "Easy / Peaceful",
    icon: "trees",
  },
  {
    id: "savanna-plateau-waterfalls",
    name: "Windswept Savanna Overhangs & Waterfalls",
    seed: "-143928104",
    category: "Mountains & Cliffs",
    description: "Gravity-defying floating savanna islands, massive stone archways, and cascading lava and waterfalls plunging straight into the ocean.",
    biomes: ["Windswept Savanna", "Savanna", "Ocean"],
    features: ["Floating Island Archways", "High Lava & Water Cascades", "Extreme Cliffs", "Acacia Overhangs"],
    difficulty: "Normal",
    icon: "mountain",
  },
  {
    id: "lone-tree-ocean-island",
    name: "One-Tree Hardcore Castaway Rock",
    seed: "1010101010101",
    category: "Hardcore Survival",
    description: "The purest survival test: a single patch of grass and dirt, one lone oak tree, and endless deep ocean for thousands of blocks.",
    biomes: ["Deep Ocean", "Ocean Plains"],
    features: ["Single Oak Tree", "Tiny Grassy Outcrop", "No Other Land in Sight", "True Solo Test"],
    difficulty: "Hardcore / Expert",
    icon: "compass",
  },
  {
    id: "witch-swamp-village",
    name: "Swamp Village Overlapping Witch Hut",
    seed: "678129034",
    category: "Swamp & Nature",
    description: "A rare village generation expanding into swamp waters with stilted huts constructed right alongside a witch hut with an active black cat.",
    biomes: ["Swamp", "Plains", "River"],
    features: ["Village Merged with Witch Hut", "Stilted Boardwalks", "Lilypads & Slimes", "Cauldron Brewing"],
    difficulty: "Normal",
    icon: "droplets",
  },
  {
    id: "cherry-snow-peaks",
    name: "Snow Peaks Ring with Multi-Tier Cherry Groves",
    seed: "184910283746",
    category: "Flora & Scenic",
    description: "Terraced cherry groves cascading down snow-capped mountains into a pristine crystal lake surrounded by flower meadows.",
    biomes: ["Cherry Grove", "Jagged Peaks", "Meadow", "Snowy Slopes"],
    features: ["Terraced Blossom Slopes", "Crystal Glacial Lake", "Alpine Meadow", "Bee Colonies"],
    difficulty: "Easy / Peaceful",
    icon: "cherry",
  },

  // 31-40: Exploration, Temples & Archaeology
  {
    id: "triple-desert-temple",
    name: "Triple Desert Temple & Camel Village",
    seed: "4819203817",
    category: "Desert & Terracotta",
    description: "Spawn in a vast desert biome featuring three separate desert temples within 300 blocks and a desert village with friendly camels.",
    biomes: ["Desert", "Badlands", "Warm Ocean"],
    features: ["3x Desert Pyramids", "Camel Caravans", "Archaeology Sand Sites", "Cactus Forests"],
    difficulty: "Normal",
    icon: "sun",
  },
  {
    id: "jungle-mansion-hybrid",
    name: "Woodland Mansion Embedded in Jungle Cliff",
    seed: "-68291047192",
    category: "Villages & Exploration",
    description: "A rare anomaly where a Woodland Mansion spawns right on the boundary of dark forest and deep jungle, overgrown with tropical vines.",
    biomes: ["Dark Forest", "Jungle", "Bamboo Jungle"],
    features: ["Vine-Covered Mansion", "Parrots on Roof", "Jungle Canopy Access", "Secret Treasure Rooms"],
    difficulty: "Hard",
    icon: "castle",
  },
  {
    id: "frozen-peaks-village",
    name: "Alpine Mountain Village at World Height",
    seed: "-1029384756",
    category: "Winter Wonderland",
    description: "A mountain village perched high at Y=140 overlooking glaciers and jagged peaks with cozy fireplaces inside every log cabin.",
    biomes: ["Frozen Peaks", "Snowy Slopes", "Meadow"],
    features: ["High Altitude Village", "Cliffside Log Cabins", "Panoramic Vistas", "Goats on Slopes"],
    difficulty: "Normal",
    icon: "snowflake",
  },
  {
    id: "trial-chamber-stronghold",
    name: "1.21 Trial Chamber Connected to Stronghold",
    seed: "402918471",
    category: "1.21 Trials & Caves",
    description: "A rare underground intersection where new 1.21 Trial Chamber corridors break directly into an End Portal Stronghold library.",
    biomes: ["Deepslate Caves", "Trial Chambers", "Plains"],
    features: ["End Portal Intersection", "Breeze Spawners in Stronghold", "Silverfish vs Breeze", "Fast End Access"],
    difficulty: "Challenging",
    icon: "zap",
  },
  {
    id: "badlands-mansion",
    name: "Badlands Plateau Woodland Mansion",
    seed: "839102847193",
    category: "Desert & Terracotta",
    description: "A dark oak forest generated high on top of a red terracotta mesa plateau containing a grand Woodland Mansion overlooking the canyon.",
    biomes: ["Badlands", "Wooded Badlands", "Dark Forest"],
    features: ["Mesa Top Mansion", "Terracotta Canyon View", "Gold Mineshafts Below", "Dramatic Backdrop"],
    difficulty: "Hard",
    icon: "sun",
  },
  {
    id: "underground-village",
    name: "Ravine Village Partially Underground",
    seed: "-192837465",
    category: "Villages & Exploration",
    description: "Villager houses and wheat farms generated down on the ledges and floor of a massive limestone ravine with bridges across the gap.",
    biomes: ["Plains", "Deep Ravine", "Forest"],
    features: ["Subterranean Village", "Suspended Rope Bridges", "Waterfalls in Streets", "Iron Golem Defender"],
    difficulty: "Normal",
    icon: "castle",
  },
  {
    id: "mooshroom-continent",
    name: "Colossal Mooshroom Continent & Ocean Monument",
    seed: "-8880300584",
    category: "Builders Paradise",
    description: "Over 1,500 blocks of continuous hostile-mob-free mushroom terrain bordering warm ocean reefs and an ancient sunken temple.",
    biomes: ["Mushroom Fields", "Warm Ocean", "Deep Warm Ocean"],
    features: ["Endless Safe Haven", "Zero Mob Spawns", "Mooshroom Herds", "Ocean Monument on Shore"],
    difficulty: "Peaceful / Safe",
    icon: "sparkles",
  },
  {
    id: "ancient-city-mine",
    name: "Ancient City Intersected by Abandoned Mineshaft",
    seed: "7491028374",
    category: "Deep Dark & Dungeons",
    description: "Mineshaft wooden bridges and spider webs span across the dark sculk towers and reinforced deepslate chests of an Ancient City.",
    biomes: ["Deep Dark", "Mineshaft", "Dripstone Caves"],
    features: ["Mineshaft in Ancient City", "Chest Minecarts", "Warden Danger", "Webbed Bridges"],
    difficulty: "Challenging",
    icon: "skull",
  },
  {
    id: "cherry-village-valley",
    name: "Cherry Blossom Basin with Meadow Village",
    seed: "7129521798835824969",
    category: "Flora & Scenic",
    description: "A protected mountain basin filled with cherry blossoms, wild bees, grazing sheep, and a full meadow village with cobblestone paths.",
    biomes: ["Cherry Grove", "Meadow", "Plains", "Forest"],
    features: ["Enclosed Blossom Basin", "Active Meadow Village", "Bee Sanctuaries", "Cozy Farm Plots"],
    difficulty: "Easy / Peaceful",
    icon: "cherry",
  },
  {
    id: "hardcore-nether-spawn",
    name: "Ruined Portal Direct to Nether Fortress",
    seed: "39102847561",
    category: "Hardcore Survival",
    description: "Spawn right next to an almost-complete Ruined Portal with obsidian in the chest. Lighting it drops you directly onto a Nether Fortress bridge!",
    biomes: ["Plains", "Nether Wastes", "Fortress"],
    features: ["Instant Nether Access", "Fortress at Portal Spawn", "Blaze Spawners", "Speedrun Setup"],
    difficulty: "Hardcore / Expert",
    icon: "compass",
  },

  // 41-52: Wonders, Ocean Fortresses & Mega Landscapes
  {
    id: "ocean-monument-triad",
    name: "Triangle of Three Ocean Monuments",
    seed: "19283740192",
    category: "Hardcore Survival",
    description: "Three Elder Guardian Ocean Monuments generated in close proximity in a deep warm ocean, perfect for mega prismarine farming.",
    biomes: ["Deep Warm Ocean", "Deep Ocean", "Warm Ocean"],
    features: ["3x Ocean Monuments", "Triple Elder Guardians", "Massive Sponge Reserves", "Prismarine Mega-Farm"],
    difficulty: "Hard",
    icon: "compass",
  },
  {
    id: "windswept-gravel-peaks",
    name: "Windswept Gravelly Hills & Floating Monoliths",
    seed: "-5029183746",
    category: "Mountains & Cliffs",
    description: "Towering shattered peaks reaching up to world height with extreme overhangs, natural arches, and precarious gravel slides.",
    biomes: ["Windswept Gravelly Hills", "Windswept Hills", "Plains"],
    features: ["Floating Monoliths", "Extreme Natural Arches", "Clouds Inside Mountains", "Acrobatic Terrain"],
    difficulty: "Normal",
    icon: "mountain",
  },
  {
    id: "sunflower-plains-sanctuary",
    name: "Infinite Sunflower Plains & River Bend",
    seed: "91827364501",
    category: "Flora & Scenic",
    description: "Golden fields of thousands of sunflowers along a tranquil winding river with friendly horses and cows, ideal for peaceful farming realms.",
    biomes: ["Sunflower Plains", "Plains", "River", "Forest"],
    features: ["Endless Sunflower Fields", "Lazy River Bend", "Horse Herds", "Flat Building Canvas"],
    difficulty: "Easy / Peaceful",
    icon: "cherry",
  },
  {
    id: "dripstone-cavern-abyss",
    name: "Colossal Dripstone Cave Chasm",
    seed: "-30192847561",
    category: "1.21 Trials & Caves",
    description: "A staggering subterranean cavern with floor-to-ceiling pointed dripstone stalactites, lava falls, and rich copper and iron veins.",
    biomes: ["Dripstone Caves", "Deep Dark", "Plains"],
    features: ["Massive Stalactite Pillars", "Lava Aquifers", "Endless Dripstone", "Underground Canyon"],
    difficulty: "Normal",
    icon: "zap",
  },
  {
    id: "desert-well-ruins",
    name: "Sniffer Dig Site & Desert Village",
    seed: "555123444",
    category: "Desert & Terracotta",
    description: "Suspicious sand archaeological pits, ancient desert well, and ocean ruins for brush archaeology to unearth Sniffer eggs and pottery sherds.",
    biomes: ["Desert", "Beach", "Warm Ocean"],
    features: ["Suspicious Sand Dig Sites", "Sniffer Eggs Possible", "Desert Village", "Ocean Ruins on Shore"],
    difficulty: "Easy",
    icon: "sun",
  },
  {
    id: "spruce-coastal-fjord",
    name: "Nordic Spruce Fjord & Frozen Ocean",
    seed: "-81920394857",
    category: "Winter Wonderland",
    description: "Dramatic sheer stone fjords lined with dense dark spruce trees meeting a frozen ocean packed with shipwrecks and blue ice.",
    biomes: ["Old Growth Taiga", "Frozen Ocean", "Stony Peaks"],
    features: ["Nordic Fjord Cliffs", "Blue Ice Deposits", "Sunken Viking Galleons", "Spruce Forest"],
    difficulty: "Normal",
    icon: "snowflake",
  },
  {
    id: "mangrove-coral-junction",
    name: "Mangrove Swamp Meets Tropical Coral Lagoon",
    seed: "49102938475",
    category: "Swamp & Nature",
    description: "Where muddy mangrove tree roots meet crystal clear shallow turquoise waters filled with sea turtles, tropical fish, and coral fans.",
    biomes: ["Mangrove Swamp", "Warm Ocean", "Coral Reef"],
    features: ["Mangrove Root Boardwalks", "Tropical Sea Turtles", "Mud Bricks Abundant", "Warm Ocean Shallows"],
    difficulty: "Normal",
    icon: "droplets",
  },
  {
    id: "trial-chamber-ancient-city",
    name: "Trial Chamber Overlapping Ancient City",
    seed: "-109283746501",
    category: "1.21 Trials & Caves",
    description: "The ultimate challenge dungeon: 1.21 Breeze spawners and copper trial vaults generated right above a sculk Warden sanctuary at deepslate level.",
    biomes: ["Trial Chambers", "Deep Dark", "Deepslate"],
    features: ["Breeze & Warden Collide", "Heavy Core Vaults", "Silence Armor Trim", "Epic Dual Dungeon"],
    difficulty: "Hardcore / Expert",
    icon: "zap",
  },
  {
    id: "jungle-crater-lake",
    name: "Jungle Caldera with Hidden Lagoon",
    seed: "7770192834",
    category: "Jungle Wilderness",
    description: "A volcanic-style ring of lush jungle cliffs concealing a hidden central freshwater lagoon with wild pandas, parrots, and cocoa trees.",
    biomes: ["Jungle", "Bamboo Jungle", "River"],
    features: ["Ring of Jungle Cliffs", "Hidden Lagoon", "Wild Pandas", "Treehouse Canopy Setup"],
    difficulty: "Normal",
    icon: "trees",
  },
  {
    id: "triple-blacksmith-village",
    name: "Triple Blacksmith Coastal Village",
    seed: "1920394857102",
    category: "Villages & Exploration",
    description: "A wealthy seaside settlement with three active blacksmith shops filled with iron armor, diamond pickaxes, obsidian, and iron ingots.",
    biomes: ["Plains", "Coastal Ocean", "Beach"],
    features: ["3x Blacksmith Houses", "Free Diamond & Iron Gear", "Safe Harbor", "Natural Docks"],
    difficulty: "Easy / Beginner",
    icon: "castle",
  },
  {
    id: "ice-spike-ring-valley",
    name: "Ice Spikes Ring Circling Frozen Meadow",
    seed: "-920192837465",
    category: "Winter Wonderland",
    description: "A fortress-like ring of towering glacial ice spikes encasing a snowy meadow valley with polar bears, stray archers, and an igloo.",
    biomes: ["Ice Spikes", "Snowy Plains", "Frozen River"],
    features: ["Circular Ice Spike Ring", "Protected Glacial Basin", "Polar Bear Cubs", "Packed Ice Towers"],
    difficulty: "Normal",
    icon: "snowflake",
  },
  {
    id: "mega-caves-trial-network",
    name: "1.21 Trial Chambers with Triple Spider Spawners",
    seed: "8837192047",
    category: "1.21 Trials & Caves",
    description: "An underground dungeon runner's dream: 1.21 Trial Chambers connected directly into multiple abandoned dungeon mob spawners for endless mob farming.",
    biomes: ["Trial Chambers", "Dungeons", "Lush Caves"],
    features: ["3x Connected Dungeon Spawners", "1.21 Breeze Trial Vaults", "Infinite XP Farm Potential", "Lush Cave Oasis"],
    difficulty: "Challenging",
    icon: "zap",
  },
];

export const PopularSeedPicker: React.FC<PopularSeedPickerProps> = ({
  selectedSeed,
  onSelectSeed,
  onClose,
  inline = false,
}) => {
  const [seeds, setSeeds] = useState<SeedPreset[]>(FALLBACK_SEEDS);
  const [search, setSearch] = useState('');
  const [selectedCategory, setSelectedCategory] = useState<string>('All');
  const [copiedSeed, setCopiedSeed] = useState<string | null>(null);

  useEffect(() => {
    api.listSeeds()
      .then((data) => {
        if (data && data.length > 0) {
          setSeeds(data);
        }
      })
      .catch(() => {
        // Fallback already active
      });
  }, []);

  const categories = ['All', ...Array.from(new Set(seeds.map((s) => s.category)))];

  const filteredSeeds = seeds.filter((s) => {
    const matchesCategory = selectedCategory === 'All' || s.category === selectedCategory;
    const query = search.toLowerCase().trim();
    if (!query) return matchesCategory;

    const matchesSearch =
      s.name.toLowerCase().includes(query) ||
      s.description.toLowerCase().includes(query) ||
      s.seed.includes(query) ||
      s.biomes.some((b) => b.toLowerCase().includes(query)) ||
      s.features.some((f) => f.toLowerCase().includes(query));

    return matchesCategory && matchesSearch;
  });

  const handleCopy = (seedText: string, e: React.MouseEvent) => {
    e.stopPropagation();
    navigator.clipboard.writeText(seedText);
    setCopiedSeed(seedText);
    setTimeout(() => setCopiedSeed(null), 2000);
  };

  const getCategoryIcon = (category: string) => {
    switch (category.toLowerCase()) {
      case 'flora & scenic':
        return <Trees className="w-3.5 h-3.5 text-pink-400" />;
      case 'deep dark & dungeons':
        return <ShieldAlert className="w-3.5 h-3.5 text-cyan-400" />;
      case '1.21 trials & caves':
        return <Zap className="w-3.5 h-3.5 text-amber-400" />;
      case 'builders paradise':
        return <Sparkles className="w-3.5 h-3.5 text-purple-400" />;
      case 'villages & exploration':
        return <Castle className="w-3.5 h-3.5 text-emerald-400" />;
      case 'desert & terracotta':
        return <Sun className="w-3.5 h-3.5 text-orange-400" />;
      case 'hardcore survival':
        return <Compass className="w-3.5 h-3.5 text-rose-400" />;
      case 'winter wonderland':
        return <Snowflake className="w-3.5 h-3.5 text-sky-400" />;
      case 'mountains & cliffs':
        return <Mountain className="w-3.5 h-3.5 text-indigo-400" />;
      case 'jungle wilderness':
        return <Trees className="w-3.5 h-3.5 text-lime-400" />;
      case 'swamp & nature':
        return <Droplets className="w-3.5 h-3.5 text-teal-400" />;
      default:
        return <Mountain className="w-3.5 h-3.5 text-slate-400" />;
    }
  };

  return (
    <div className={`flex flex-col ${inline ? 'w-full' : 'p-4 sm:p-6'}`}>
      {/* Header controls */}
      <div className="flex flex-col sm:flex-row items-stretch sm:items-center justify-between gap-3 mb-3">
        <div className="relative flex-1">
          <Search className="w-4 h-4 text-slate-400 absolute left-3 top-1/2 -translate-y-1/2" />
          <input
            type="text"
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            placeholder="Search 50+ popular seeds (e.g. cherry, ancient city, trial chambers, mansion, island, sniffer)..."
            className="w-full pl-9 pr-4 py-2 rounded-lg bg-obsidian-950 border border-obsidian-700 focus:border-emerald-500 text-slate-100 font-mono text-xs placeholder:text-slate-500"
          />
        </div>

        <div className="flex items-center space-x-2">
          <span className="text-[11px] font-mono text-slate-400 px-2 py-1 rounded bg-obsidian-950 border border-obsidian-800">
            {filteredSeeds.length} of {seeds.length} seeds
          </span>

          <button
            type="button"
            onClick={() => {
              onSelectSeed('');
              if (onClose) onClose();
            }}
            className={`px-3 py-2 rounded-lg border font-mono text-xs transition-colors flex items-center justify-center space-x-1.5 ${
              !selectedSeed
                ? 'bg-emerald-500/20 text-emerald-300 border-emerald-500/50'
                : 'bg-obsidian-900 text-slate-400 border-obsidian-700 hover:text-slate-200'
            }`}
          >
            <Sparkles className="w-3.5 h-3.5 text-amber-400" />
            <span>Random World (RNG)</span>
          </button>
        </div>
      </div>

      {/* Category Pills */}
      <div className="flex items-center gap-1.5 overflow-x-auto pb-2 mb-3 scrollbar-thin">
        {categories.map((cat) => (
          <button
            key={cat}
            type="button"
            onClick={() => setSelectedCategory(cat)}
            className={`px-2.5 py-1 rounded-full text-[11px] font-mono whitespace-nowrap transition-colors flex items-center space-x-1.5 ${
              selectedCategory === cat
                ? 'bg-emerald-500/20 text-emerald-400 border border-emerald-500/40 font-semibold'
                : 'bg-obsidian-950/80 text-slate-400 border border-obsidian-800 hover:text-slate-200'
            }`}
          >
            {cat !== 'All' && getCategoryIcon(cat)}
            <span>{cat}</span>
          </button>
        ))}
      </div>

      {/* Seeds Grid */}
      <div className="grid grid-cols-1 md:grid-cols-2 gap-3 max-h-[380px] overflow-y-auto pr-1 scrollbar-thin">
        {filteredSeeds.map((seed) => {
          const isSelected = selectedSeed.trim() === seed.seed.trim();

          return (
            <div
              key={seed.id}
              onClick={() => {
                onSelectSeed(seed.seed, seed);
                if (onClose) onClose();
              }}
              className={`p-3.5 rounded-xl border text-left cursor-pointer transition-all flex flex-col justify-between ${
                isSelected
                  ? 'bg-emerald-950/40 border-emerald-500/60 shadow-[0_0_15px_rgba(16,185,129,0.15)] ring-1 ring-emerald-500/40'
                  : 'bg-obsidian-950/60 border-obsidian-800/80 hover:border-obsidian-700 hover:bg-obsidian-900/60'
              }`}
            >
              <div>
                {/* Title & Category */}
                <div className="flex items-start justify-between gap-2 mb-1.5">
                  <div className="flex items-center space-x-2">
                    <span className="p-1 rounded bg-obsidian-900 border border-obsidian-700/60">
                      {getCategoryIcon(seed.category)}
                    </span>
                    <h4 className="font-mono text-xs font-bold text-slate-100 leading-snug">
                      {seed.name}
                    </h4>
                  </div>
                  {isSelected ? (
                    <span className="flex items-center space-x-1 text-[10px] font-mono text-emerald-400 bg-emerald-500/10 px-2 py-0.5 rounded border border-emerald-500/30">
                      <CheckCircle2 className="w-3 h-3 text-emerald-400" />
                      <span>Active</span>
                    </span>
                  ) : (
                    <span className="text-[10px] font-mono text-slate-500 bg-obsidian-900 px-2 py-0.5 rounded border border-obsidian-800">
                      {seed.difficulty}
                    </span>
                  )}
                </div>

                {/* Description */}
                <p className="text-[11px] text-slate-400 leading-relaxed mb-2.5">
                  {seed.description}
                </p>

                {/* Biomes */}
                <div className="flex flex-wrap gap-1 mb-2">
                  {seed.biomes.slice(0, 3).map((biome) => (
                    <span
                      key={biome}
                      className="px-1.5 py-0.5 rounded bg-obsidian-900 border border-obsidian-800 text-[10px] font-mono text-emerald-400/90"
                    >
                      {biome}
                    </span>
                  ))}
                  {seed.biomes.length > 3 && (
                    <span className="px-1.5 py-0.5 rounded bg-obsidian-900 text-[10px] font-mono text-slate-500">
                      +{seed.biomes.length - 3}
                    </span>
                  )}
                </div>

                {/* Features */}
                <div className="flex flex-wrap gap-1">
                  {seed.features.slice(0, 3).map((feat) => (
                    <span
                      key={feat}
                      className="px-1.5 py-0.5 rounded bg-obsidian-900/60 border border-obsidian-800/60 text-[10px] font-mono text-slate-400"
                    >
                      • {feat}
                    </span>
                  ))}
                </div>
              </div>

              {/* Bottom Seed & Action */}
              <div className="mt-3 pt-2.5 border-t border-obsidian-800/80 flex items-center justify-between">
                <div className="flex items-center space-x-1.5 font-mono text-[11px] text-slate-300">
                  <span className="text-slate-500">Seed:</span>
                  <code className="bg-obsidian-900 px-1.5 py-0.5 rounded border border-obsidian-800 text-emerald-400 text-[10px]">
                    {seed.seed}
                  </code>
                  <button
                    type="button"
                    onClick={(e) => handleCopy(seed.seed, e)}
                    title="Copy seed string"
                    className="p-1 rounded hover:bg-obsidian-800 text-slate-400 hover:text-slate-200 transition-colors"
                  >
                    {copiedSeed === seed.seed ? (
                      <Check className="w-3 h-3 text-emerald-400" />
                    ) : (
                      <Copy className="w-3 h-3" />
                    )}
                  </button>
                </div>

                <button
                  type="button"
                  onClick={(e) => {
                    e.stopPropagation();
                    onSelectSeed(seed.seed, seed);
                    if (onClose) onClose();
                  }}
                  className={`px-2.5 py-1 rounded text-[11px] font-mono font-medium transition-colors ${
                    isSelected
                      ? 'bg-emerald-500 text-slate-950 font-bold'
                      : 'bg-obsidian-800 hover:bg-emerald-600 hover:text-slate-950 text-slate-200'
                  }`}
                >
                  {isSelected ? 'Selected' : 'Use Seed'}
                </button>
              </div>
            </div>
          );
        })}

        {filteredSeeds.length === 0 && (
          <div className="col-span-full py-8 text-center text-slate-500 font-mono text-xs">
            No popular seeds match "{search}". Try searching for another biome or category.
          </div>
        )}
      </div>
    </div>
  );
};
