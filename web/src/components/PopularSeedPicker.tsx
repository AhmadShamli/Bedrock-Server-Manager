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
      <div className="flex flex-col sm:flex-row items-stretch sm:items-center justify-between gap-3 mb-4">
        <div className="relative flex-1">
          <Search className="w-4 h-4 text-slate-400 absolute left-3 top-1/2 -translate-y-1/2" />
          <input
            type="text"
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            placeholder="Search popular seeds (e.g. cherry, ancient city, trial chambers, 1.21)..."
            className="w-full pl-9 pr-4 py-2 rounded-lg bg-obsidian-950 border border-obsidian-700 focus:border-emerald-500 text-slate-100 font-mono text-xs placeholder:text-slate-500"
          />
        </div>

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

      {/* Category Pills */}
      <div className="flex items-center gap-1.5 overflow-x-auto pb-2 mb-4 scrollbar-thin">
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
      <div className="grid grid-cols-1 md:grid-cols-2 gap-3 max-h-[360px] overflow-y-auto pr-1 scrollbar-thin">
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
                  {seed.features.slice(0, 2).map((feat) => (
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
