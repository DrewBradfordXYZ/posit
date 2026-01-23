package posit

import (
	"math"
	"sort"
)

// computePortOffsets assigns computed offsets to PortFixedSide, PortFixedOrder,
// and PortFree ports. Called after coordinate assignment, before edge routing.
// Writes offsets (and for PortFree, sides) back to node.ports so that
// getPortPosition() works unchanged.
func (s *layoutState) computePortOffsets() {
	for _, node := range s.nodes {
		if node.isDummy || len(node.ports) == 0 {
			continue
		}

		// Check if any ports need computation
		hasComputed := false
		for _, port := range node.ports {
			if port.Constraint == PortFixedSide || port.Constraint == PortFixedOrder || port.Constraint == PortFree {
				hasComputed = true
				break
			}
		}
		if !hasComputed {
			continue
		}

		// Phase 1: For PortFree ports, determine the best side
		s.assignFreeSides(node)

		// Phase 2: Group port indices by (internal) side and compute offsets
		sideGroups := map[Side][]int{}
		for i := range node.ports {
			port := &node.ports[i]
			if port.Constraint == PortFixedSide || port.Constraint == PortFixedOrder || port.Constraint == PortFree {
				side := s.portSideToInternal(port.Side)
				sideGroups[side] = append(sideGroups[side], i)
			}
		}

		for side, indices := range sideGroups {
			s.computeSideOffsets(node, side, indices)
		}
	}
}

// assignFreeSides determines the best side for each PortFree port on a node.
// Uses the position of connected nodes relative to this node's center.
func (s *layoutState) assignFreeSides(node *layoutNode) {
	nodeCX := node.x + node.width/2
	nodeCY := node.y + node.height/2

	for i := range node.ports {
		port := &node.ports[i]
		if port.Constraint != PortFree {
			continue
		}

		// Find average direction to connected nodes
		dx, dy := s.portConnectedDirection(node.id, port.ID, nodeCX, nodeCY)

		// Pick the best side based on direction and axis constraint
		port.Side = s.bestSide(dx, dy, port.Axis)
	}
}

// portConnectedDirection returns the average direction vector from a node's center
// to all nodes connected to a specific port.
func (s *layoutState) portConnectedDirection(nodeID, portID string, nodeCX, nodeCY float64) (dx, dy float64) {
	count := 0
	for _, edge := range s.edges {
		var connNode *layoutNode
		if edge.key.from == nodeID && edge.sourcePort == portID {
			connNode = s.nodes[edge.key.to]
		} else if edge.key.to == nodeID && edge.targetPort == portID {
			connNode = s.nodes[edge.key.from]
		}
		if connNode == nil {
			continue
		}
		dx += (connNode.x + connNode.width/2) - nodeCX
		dy += (connNode.y + connNode.height/2) - nodeCY
		count++
	}
	if count > 0 {
		dx /= float64(count)
		dy /= float64(count)
	}
	return dx, dy
}

// bestSide picks the side that best faces the given direction, constrained by axis.
func (s *layoutState) bestSide(dx, dy float64, axis PortAxis) Side {
	switch axis {
	case PortAxisHorizontal:
		if dx >= 0 {
			return Right
		}
		return Left
	case PortAxisVertical:
		if dy >= 0 {
			return Bottom
		}
		return Top
	default: // PortAxisAny
		if math.Abs(dx) >= math.Abs(dy) {
			if dx >= 0 {
				return Right
			}
			return Left
		}
		if dy >= 0 {
			return Bottom
		}
		return Top
	}
}

// computeSideOffsets computes evenly-distributed offsets for ports on one side of a node.
func (s *layoutState) computeSideOffsets(node *layoutNode, side Side, indices []int) {
	if len(indices) == 0 {
		return
	}

	// Determine the side length
	var sideLength float64
	switch side {
	case Left, Right:
		sideLength = node.height
	case Top, Bottom:
		sideLength = node.width
	}

	// Sort indices by the appropriate criteria
	sort.SliceStable(indices, func(a, b int) bool {
		portA := &node.ports[indices[a]]
		portB := &node.ports[indices[b]]

		if portA.Constraint == PortFixedOrder && portB.Constraint == PortFixedOrder {
			// Both PortFixedOrder: sort by declared Order
			return portA.Order < portB.Order
		}

		// PortFixedSide / PortFree: sort by connected node position along this side
		posA := s.portSideBarycenter(node, portA.ID, side)
		posB := s.portSideBarycenter(node, portB.ID, side)
		return posA < posB
	})

	// Compute evenly-distributed offsets
	count := len(indices)
	for rank, idx := range indices {
		node.ports[idx].Offset = sideLength * float64(rank+1) / float64(count+1)
	}
}

// portSideBarycenter returns the average position of connected nodes along the
// axis relevant to the given side. For Left/Right sides, returns average Y;
// for Top/Bottom sides, returns average X.
func (s *layoutState) portSideBarycenter(node *layoutNode, portID string, side Side) float64 {
	sum := 0.0
	count := 0

	for _, edge := range s.edges {
		var connNode *layoutNode
		if edge.key.from == node.id && edge.sourcePort == portID {
			connNode = s.nodes[edge.key.to]
		} else if edge.key.to == node.id && edge.targetPort == portID {
			connNode = s.nodes[edge.key.from]
		}
		if connNode == nil {
			continue
		}

		switch side {
		case Left, Right:
			sum += connNode.y + connNode.height/2
		case Top, Bottom:
			sum += connNode.x + connNode.width/2
		}
		count++
	}

	if count == 0 {
		return 0
	}
	return sum / float64(count)
}
