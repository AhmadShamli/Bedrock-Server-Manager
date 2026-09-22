package configfile

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// SafePath ensures the requested relative file is strictly within the server directory.
func SafePath(dataDir, serverID, filename string) (string, error) {
	cleanFilename := filepath.Clean(filename)
	if strings.Contains(cleanFilename, "..") || filepath.IsAbs(cleanFilename) {
		return "", fmt.Errorf("invalid file path: directory traversal attempt")
	}

	serverDir, err := filepath.Abs(filepath.Join(dataDir, "servers", serverID))
	if err != nil {
		return "", err
	}

	targetPath, err := filepath.Abs(filepath.Join(serverDir, cleanFilename))
	if err != nil {
		return "", err
	}

	if !strings.HasPrefix(targetPath, serverDir) {
		return "", fmt.Errorf("path traversal attempt detected")
	}

	return targetPath, nil
}

// ReadProperties parses a server.properties file into a map preserving order.
func ReadProperties(filePath string) (map[string]string, []string, error) {
	props := make(map[string]string)
	var keys []string

	f, err := os.Open(filePath)
	if os.IsNotExist(err) {
		return props, keys, nil
	}
	if err != nil {
		return nil, nil, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			k := strings.TrimSpace(parts[0])
			v := strings.TrimSpace(parts[1])
			if _, exists := props[k]; !exists {
				keys = append(keys, k)
			}
			props[k] = v
		}
	}

	return props, keys, scanner.Err()
}

// WriteProperties writes a key-value map to server.properties with proper header comments.
func WriteProperties(filePath string, props map[string]string, keys []string) error {
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	var sb strings.Builder
	sb.WriteString("# Minecraft Bedrock Dedicated Server Properties\n")
	sb.WriteString("# Managed by Bedrock Server Manager (BSM)\n\n")

	written := make(map[string]bool)

	// Write existing keys in order
	for _, k := range keys {
		if v, ok := props[k]; ok {
			sb.WriteString(fmt.Sprintf("%s=%s\n", k, v))
			written[k] = true
		}
	}

	// Write any newly added keys
	for k, v := range props {
		if !written[k] {
			sb.WriteString(fmt.Sprintf("%s=%s\n", k, v))
		}
	}

	return os.WriteFile(filePath, []byte(sb.String()), 0644)
}

// AllowlistEntry represents a player allowlist entry.
type AllowlistEntry struct {
	Name               string `json:"name"`
	XUID               string `json:"xuid,omitempty"`
	IgnoresPlayerLimit bool   `json:"ignoresPlayerLimit"`
}

// ReadAllowlist loads allowlist.json.
func ReadAllowlist(filePath string) ([]AllowlistEntry, error) {
	data, err := os.ReadFile(filePath)
	if os.IsNotExist(err) {
		return []AllowlistEntry{}, nil
	}
	if err != nil {
		return nil, err
	}

	var list []AllowlistEntry
	if len(data) == 0 {
		return list, nil
	}

	if err := json.Unmarshal(data, &list); err != nil {
		return nil, err
	}
	return list, nil
}

// WriteAllowlist saves allowlist.json.
func WriteAllowlist(filePath string, list []AllowlistEntry) error {
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(list, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filePath, data, 0644)
}

// PermissionEntry represents a player permissions entry.
type PermissionEntry struct {
	Permission string `json:"permission"` // visitor, member, operator
	XUID       string `json:"xuid"`
}

// ReadPermissions loads permissions.json.
func ReadPermissions(filePath string) ([]PermissionEntry, error) {
	data, err := os.ReadFile(filePath)
	if os.IsNotExist(err) {
		return []PermissionEntry{}, nil
	}
	if err != nil {
		return nil, err
	}

	var list []PermissionEntry
	if len(data) == 0 {
		return list, nil
	}

	if err := json.Unmarshal(data, &list); err != nil {
		return nil, err
	}
	return list, nil
}

// WritePermissions saves permissions.json.
func WritePermissions(filePath string, list []PermissionEntry) error {
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(list, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filePath, data, 0644)
}
