package configfile

import (
	"strconv"

	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/models"
)

// DefaultServerPropertyKeys lists canonical Bedrock Dedicated Server property keys in recommended order.
var DefaultServerPropertyKeys = []string{
	"server-name",
	"gamemode",
	"force-gamemode",
	"difficulty",
	"allow-cheats",
	"max-players",
	"online-mode",
	"allow-list",
	"white-list",
	"server-port",
	"server-portv6",
	"transport",
	"enable-lan-visibility",
	"view-distance",
	"tick-distance",
	"player-idle-timeout",
	"max-threads",
	"level-name",
	"level-seed",
	"default-player-permission-level",
	"texturepack-required",
	"content-log-file-enabled",
	"content-log-console-output-enabled",
	"content-log-level",
	"compression-threshold",
	"compression-algorithm",
	"server-authoritative-movement-strict",
	"server-authoritative-dismount-strict",
	"server-authoritative-entity-interactions-strict",
	"player-position-acceptance-threshold",
	"player-movement-action-direction-threshold",
	"server-authoritative-block-breaking-pick-range-scalar",
	"chat-restriction",
	"disable-player-interaction",
	"client-side-chunk-generation-enabled",
	"block-network-ids-are-hashes",
	"disable-persona",
	"disable-custom-skins",
	"server-build-radius-ratio",
	"allow-outbound-script-debugging",
	"allow-inbound-script-debugging",
	"script-debugger-auto-attach",
}

// DefaultServerProperties provides baseline default values for Minecraft Bedrock Dedicated Servers.
var DefaultServerProperties = map[string]string{
	"server-name":                                          "Dedicated Server",
	"gamemode":                                             "survival",
	"force-gamemode":                                       "false",
	"difficulty":                                           "easy",
	"allow-cheats":                                         "false",
	"max-players":                                          "10",
	"online-mode":                                          "true",
	"allow-list":                                           "false",
	"white-list":                                           "false",
	"server-port":                                          "19132",
	"server-portv6":                                        "19133",
	"transport":                                            "raknet",
	"enable-lan-visibility":                                "true",
	"view-distance":                                        "32",
	"tick-distance":                                        "4",
	"player-idle-timeout":                                  "30",
	"max-threads":                                          "8",
	"level-name":                                           "Bedrock level",
	"level-seed":                                           "",
	"default-player-permission-level":                      "member",
	"texturepack-required":                                 "false",
	"content-log-file-enabled":                             "false",
	"content-log-console-output-enabled":                   "false",
	"content-log-level":                                    "info",
	"compression-threshold":                                "1",
	"compression-algorithm":                                "zlib",
	"server-authoritative-movement-strict":                 "false",
	"server-authoritative-dismount-strict":                 "false",
	"server-authoritative-entity-interactions-strict":     "false",
	"player-position-acceptance-threshold":                 "0.5",
	"player-movement-action-direction-threshold":           "0.85",
	"server-authoritative-block-breaking-pick-range-scalar": "1.5",
	"chat-restriction":                                     "None",
	"disable-player-interaction":                           "false",
	"client-side-chunk-generation-enabled":                 "true",
	"block-network-ids-are-hashes":                         "true",
	"disable-persona":                                      "false",
	"disable-custom-skins":                                 "false",
	"server-build-radius-ratio":                            "Disabled",
	"allow-outbound-script-debugging":                      "false",
	"allow-inbound-script-debugging":                       "false",
	"script-debugger-auto-attach":                          "disabled",
}

// GenerateDefaultProperties returns a complete map and key ordering populated from defaults and the server model.
func GenerateDefaultProperties(server *models.Server) (map[string]string, []string) {
	return MergeDefaultProperties(nil, nil, server)
}

// MergeDefaultProperties merges missing standard properties into existingProps and existingKeys, preserving existing values and order.
func MergeDefaultProperties(existingProps map[string]string, existingKeys []string, server *models.Server) (map[string]string, []string) {
	props := make(map[string]string)
	for k, v := range existingProps {
		props[k] = v
	}

	keys := make([]string, 0, len(existingKeys)+len(DefaultServerPropertyKeys))
	keySet := make(map[string]bool)
	for _, k := range existingKeys {
		if !keySet[k] {
			keys = append(keys, k)
			keySet[k] = true
		}
	}

	for _, k := range DefaultServerPropertyKeys {
		if _, exists := props[k]; !exists {
			val := DefaultServerProperties[k]

			if server != nil {
				switch k {
				case "server-name":
					if server.Name != "" {
						val = server.Name
					}
				case "gamemode":
					if server.Mode != "" {
						val = server.Mode
					}
				case "difficulty":
					if server.Difficulty != "" {
						val = server.Difficulty
					}
				case "server-port":
					if server.Port > 0 {
						val = strconv.Itoa(server.Port)
					}
				case "server-portv6":
					if server.PortV6 > 0 {
						val = strconv.Itoa(server.PortV6)
					}
				case "level-name":
					if server.Name != "" {
						val = server.Name
					}
				case "level-seed":
					if server.Seed != "" {
						val = server.Seed
					}
				}
			}

			props[k] = val
			if !keySet[k] {
				keys = append(keys, k)
				keySet[k] = true
			}
		} else if k == "level-seed" && props["level-seed"] == "" && server != nil && server.Seed != "" {
			props["level-seed"] = server.Seed
		}
	}

	return props, keys
}
