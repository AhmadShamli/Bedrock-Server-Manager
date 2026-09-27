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

	serverDirWithSep := serverDir + string(filepath.Separator)
	if targetPath != serverDir && !strings.HasPrefix(targetPath, serverDirWithSep) {
		return "", fmt.Errorf("path traversal attempt detected")
	}

	return targetPath, nil
}

// ReadProperties parses a server.properties file into a map preserving order.
func ReadProperties(filePath string) (map[string]string, []string, error) {
	props := make(map[string]string)
	keys := make([]string, 0)

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

// MergeProperties merges updates into existing properties, preserving the original keys, comments, and order, while appending any new keys.
func MergeProperties(existingProps map[string]string, existingKeys []string, updates map[string]string) (map[string]string, []string) {
	props := make(map[string]string)
	for k, v := range existingProps {
		props[k] = v
	}

	keys := make([]string, 0, len(existingKeys)+len(updates))
	keySet := make(map[string]bool)
	for _, k := range existingKeys {
		if !keySet[k] {
			keys = append(keys, k)
			keySet[k] = true
		}
	}

	for k, v := range updates {
		props[k] = v
		if !keySet[k] {
			keys = append(keys, k)
			keySet[k] = true
		}
	}

	return props, keys
}

// UpdateExistingPropertyFile updates specific key-value pairs in an existing server.properties file on disk.
// If the file does not exist, it does NOT create it, allowing itzg/BDS to unzip/generate the authentic file first.
func UpdateExistingPropertyFile(filePath string, updates map[string]string) (bool, error) {
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return false, nil // File does not exist, leave it for itzg/BDS to unzip
	}

	props, keys, err := ReadProperties(filePath)
	if err != nil {
		return false, err
	}

	mergedProps, mergedKeys := MergeProperties(props, keys, updates)
	if err := WriteProperties(filePath, mergedProps, mergedKeys); err != nil {
		return false, err
	}

	return true, nil
}

// PendingPropertiesFilename is the filename used to store pre-boot configuration overrides for an uninitialized instance.
const PendingPropertiesFilename = ".pending_properties.json"

// PendingPropertiesPayload defines the format of pending pre-boot properties.
type PendingPropertiesPayload struct {
	Properties map[string]string `json:"properties"`
	Keys       []string          `json:"keys"`
}

// ReadPendingProperties reads pre-boot property overrides if present in the server directory.
func ReadPendingProperties(serverDir string) (map[string]string, []string, bool, error) {
	path := filepath.Join(serverDir, PendingPropertiesFilename)
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil, false, nil
	}
	if err != nil {
		return nil, nil, false, err
	}

	var payload PendingPropertiesPayload
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, nil, false, err
	}

	if payload.Properties == nil {
		payload.Properties = make(map[string]string)
	}
	if payload.Keys == nil {
		payload.Keys = []string{}
	}

	return payload.Properties, payload.Keys, true, nil
}

// WritePendingProperties writes pre-boot property overrides for an uninitialized instance.
func WritePendingProperties(serverDir string, props map[string]string, keys []string) error {
	if err := os.MkdirAll(serverDir, 0755); err != nil {
		return err
	}

	payload := PendingPropertiesPayload{
		Properties: props,
		Keys:       keys,
	}

	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}

	path := filepath.Join(serverDir, PendingPropertiesFilename)
	return os.WriteFile(path, data, 0644)
}

// RemovePendingProperties deletes the pre-boot pending property overrides file once applied.
func RemovePendingProperties(serverDir string) error {
	path := filepath.Join(serverDir, PendingPropertiesFilename)
	err := os.Remove(path)
	if os.IsNotExist(err) {
		return nil
	}
	return err
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
	if os.IsNotExist(err) || len(data) == 0 {
		return []AllowlistEntry{}, nil
	}
	if err != nil {
		return nil, err
	}

	list := make([]AllowlistEntry, 0)
	if err := json.Unmarshal(data, &list); err != nil {
		return nil, err
	}
	if list == nil {
		list = []AllowlistEntry{}
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
	if os.IsNotExist(err) || len(data) == 0 {
		return []PermissionEntry{}, nil
	}
	if err != nil {
		return nil, err
	}

	list := make([]PermissionEntry, 0)
	if err := json.Unmarshal(data, &list); err != nil {
		return nil, err
	}
	if list == nil {
		list = []PermissionEntry{}
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
