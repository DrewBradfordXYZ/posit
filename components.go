package posit

import "sort"

// packComponents rearranges disconnected components according to the packing options.
// This runs after coordinate assignment but before edge routing.
func (s *layoutState) packComponents() {
	components := s.findComponents()
	if len(components) <= 1 {
		return
	}

	gap := s.opts.ComponentGap
	if gap <= 0 {
		gap = s.opts.NodeSep * 2
	}

	// Sort components deterministically by first node ID
	sort.Slice(components, func(i, j int) bool {
		if len(components[i]) == 0 {
			return true
		}
		if len(components[j]) == 0 {
			return false
		}
		return components[i][0] < components[j][0]
	})

	// Compute bounding box for each component
	type bbox struct {
		minX, minY, maxX, maxY float64
	}
	boxes := make([]bbox, len(components))

	for ci, comp := range components {
		first := true
		for _, id := range comp {
			node := s.nodes[id]
			if node == nil {
				continue
			}
			left := node.x
			top := node.y
			right := node.x + node.width
			bottom := node.y + node.height

			if first {
				boxes[ci] = bbox{left, top, right, bottom}
				first = false
			} else {
				if left < boxes[ci].minX {
					boxes[ci].minX = left
				}
				if top < boxes[ci].minY {
					boxes[ci].minY = top
				}
				if right > boxes[ci].maxX {
					boxes[ci].maxX = right
				}
				if bottom > boxes[ci].maxY {
					boxes[ci].maxY = bottom
				}
			}
		}
	}

	// Position components according to packing style
	offset := 0.0
	for ci, comp := range components {
		b := boxes[ci]
		var dx, dy float64

		if s.opts.ComponentPacking == PackVertical {
			// Stack vertically
			dy = offset - b.minY
			dx = -b.minX // Align to left edge
			offset += (b.maxY - b.minY) + gap
		} else {
			// Side by side (horizontal)
			dx = offset - b.minX
			dy = -b.minY // Align to top edge
			offset += (b.maxX - b.minX) + gap
		}

		// Apply offset to all nodes in this component
		for _, id := range comp {
			node := s.nodes[id]
			if node != nil {
				node.x += dx
				node.y += dy
			}
		}
	}
}

// findComponents identifies disconnected components using BFS.
func (s *layoutState) findComponents() [][]string {
	visited := make(map[string]bool, len(s.nodes))
	var components [][]string

	// Get sorted node IDs for deterministic ordering
	nodeIDs := make([]string, 0, len(s.nodes))
	for id := range s.nodes {
		if !s.nodes[id].isDummy {
			nodeIDs = append(nodeIDs, id)
		}
	}
	sort.Strings(nodeIDs)

	for _, startID := range nodeIDs {
		if visited[startID] {
			continue
		}

		// BFS from this node
		var component []string
		queue := []string{startID}
		visited[startID] = true

		for len(queue) > 0 {
			id := queue[0]
			queue = queue[1:]
			component = append(component, id)

			// Visit successors
			for _, succ := range s.successors[id] {
				if !visited[succ] && !s.nodes[succ].isDummy {
					visited[succ] = true
					queue = append(queue, succ)
				}
			}

			// Visit predecessors
			for _, pred := range s.predecessors[id] {
				if !visited[pred] && !s.nodes[pred].isDummy {
					visited[pred] = true
					queue = append(queue, pred)
				}
			}
		}

		sort.Strings(component)
		components = append(components, component)
	}

	return components
}
