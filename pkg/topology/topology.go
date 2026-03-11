// Package topology provides the Topology type for managing platform topology structure.
// The topology defines the skeleton of the platform - paths where nodes and components attach.
package topology

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Topology represents the platform topology configuration.
// It preserves YAML order for consistent output.
type Topology struct {
	node *yaml.Node
	data map[string]map[string][]interface{}
}

// YAMLNode returns the underlying YAML document node.
func (t *Topology) YAMLNode() *yaml.Node {
	return t.node
}

// SetYAMLNode replaces the underlying YAML document node.
func (t *Topology) SetYAMLNode(n *yaml.Node) {
	t.node = n
}

// RawData returns the parsed topology data structure.
func (t *Topology) RawData() map[string]map[string][]interface{} {
	return t.data
}

// SetRawData replaces the parsed topology data structure.
func (t *Topology) SetRawData(d map[string]map[string][]interface{}) {
	t.data = d
}

// Load reads and parses topology.yaml from the given directory.
func Load(dir string) (*Topology, error) {
	path := filepath.Join(dir, "topology.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read topology.yaml: %w", err)
	}

	var node yaml.Node
	if err := yaml.Unmarshal(data, &node); err != nil {
		return nil, fmt.Errorf("failed to parse topology.yaml: %w", err)
	}

	var parsed map[string]map[string][]interface{}
	if err := yaml.Unmarshal(data, &parsed); err != nil {
		return nil, fmt.Errorf("failed to parse topology.yaml: %w", err)
	}

	return &Topology{
		node: &node,
		data: parsed,
	}, nil
}

// Flatten returns all zone paths in tree traversal order.
// Example output: ["platform", "platform.foundation", "platform.foundation.cluster", ...]
func (t *Topology) Flatten() []string {
	if t.node == nil || len(t.node.Content) == 0 {
		return nil
	}

	var paths []string
	rootNode := t.node.Content[0]
	if rootNode.Kind != yaml.MappingNode {
		return nil
	}

	// Iterate root keys (e.g., "platform")
	for i := 0; i < len(rootNode.Content); i += 2 {
		rootKey := rootNode.Content[i].Value
		rootValue := rootNode.Content[i+1]
		paths = append(paths, rootKey)

		if rootValue.Kind != yaml.MappingNode {
			continue
		}

		// Iterate layers (e.g., "foundation", "interaction")
		for j := 0; j < len(rootValue.Content); j += 2 {
			layerKey := rootValue.Content[j].Value
			layerValue := rootValue.Content[j+1]
			layerPrefix := rootKey + "." + layerKey
			paths = append(paths, layerPrefix)

			if layerValue.Kind == yaml.SequenceNode {
				paths = append(paths, flattenSequence(layerPrefix, layerValue)...)
			}
		}
	}

	return paths
}

// flattenSequence recursively flattens a YAML sequence preserving order
func flattenSequence(prefix string, node *yaml.Node) []string {
	var paths []string

	for _, item := range node.Content {
		switch item.Kind {
		case yaml.ScalarNode:
			paths = append(paths, prefix+"."+item.Value)
		case yaml.MappingNode:
			for k := 0; k < len(item.Content); k += 2 {
				key := item.Content[k].Value
				value := item.Content[k+1]
				newPrefix := prefix + "." + key
				paths = append(paths, newPrefix)
				if value.Kind == yaml.SequenceNode {
					paths = append(paths, flattenSequence(newPrefix, value)...)
				}
			}
		}
	}

	return paths
}

// Exists checks if a zone path exists.
func (t *Topology) Exists(zonePath string) bool {
	for _, path := range t.Flatten() {
		if path == zonePath {
			return true
		}
	}
	return false
}

// Root returns the root zone name (e.g., "platform").
func (t *Topology) Root() string {
	paths := t.Flatten()
	if len(paths) > 0 {
		return paths[0]
	}
	return ""
}

// Children returns the direct children of a zone path.
func (t *Topology) Children(zonePath string) []string {
	var children []string
	prefix := zonePath + "."

	for _, path := range t.Flatten() {
		if strings.HasPrefix(path, prefix) {
			// Check it's a direct child (no more dots after prefix)
			remainder := path[len(prefix):]
			if !strings.Contains(remainder, ".") {
				children = append(children, path)
			}
		}
	}

	return children
}

// ChildrenMap returns a map of zone path to its direct children.
func (t *Topology) ChildrenMap() map[string][]string {
	result := make(map[string][]string)

	for _, zonePath := range t.Flatten() {
		parent := Parent(zonePath)
		if parent != "" {
			result[parent] = append(result[parent], zonePath)
		}
	}

	return result
}

// Ancestors returns all ancestors of a given zone path.
// Example: "platform.foundation.cluster.control" returns
// ["platform.foundation.cluster", "platform.foundation", "platform"]
func (t *Topology) Ancestors(zonePath string) []string {
	var ancestors []string
	current := zonePath

	for {
		parent := Parent(current)
		if parent == "" {
			break
		}
		ancestors = append(ancestors, parent)
		current = parent
	}

	return ancestors
}

// AncestorsMap returns a map of zone path to its ancestors.
func (t *Topology) AncestorsMap() map[string][]string {
	result := make(map[string][]string)

	for _, zonePath := range t.Flatten() {
		result[zonePath] = t.Ancestors(zonePath)
	}

	return result
}

// Parent returns the parent of a given zone path.
// Example: "platform.foundation.cluster" returns "platform.foundation"
// Returns empty string for root paths.
func Parent(zonePath string) string {
	idx := strings.LastIndex(zonePath, ".")
	if idx == -1 {
		return ""
	}
	return zonePath[:idx]
}

// IsDescendantOf checks if zonePath is a descendant of ancestor.
func IsDescendantOf(zonePath, ancestor string) bool {
	return strings.HasPrefix(zonePath, ancestor+".")
}

// FlattenWithPrefix returns zone paths that start with the given prefix.
func (t *Topology) FlattenWithPrefix(prefix string) []string {
	all := t.Flatten()
	if prefix == "" {
		return all
	}

	var filtered []string
	for _, path := range all {
		if path == prefix || strings.HasPrefix(path, prefix+".") {
			filtered = append(filtered, path)
		}
	}
	return filtered
}

// ValidatePath checks that a zone path is well-formed.
// Segments must be non-empty and contain only lowercase letters, digits, hyphens, or underscores.
func ValidatePath(zonePath string) error {
	if zonePath == "" {
		return fmt.Errorf("zone path cannot be empty")
	}
	parts := strings.Split(zonePath, ".")
	for i, part := range parts {
		if part == "" {
			return fmt.Errorf("zone path has empty segment at position %d", i+1)
		}
		for _, r := range part {
			if !((r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '_') {
				return fmt.Errorf("zone path segment %q contains invalid character %q", part, string(r))
			}
		}
	}
	return nil
}
