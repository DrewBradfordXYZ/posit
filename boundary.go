package posit

// Rect represents a node's bounding rectangle.
type Rect struct {
	Left, Right, Top, Bottom float64
}

// IntersectResult contains the intersection point and which side it's on.
type IntersectResult struct {
	Point  EdgePoint // Intersection coordinates
	Side   Side      // Which side of the rectangle
	Offset float64   // Distance along that side from its start
}

// nodeRect returns the bounding rectangle for a layout node.
func nodeRect(n *layoutNode) Rect {
	return Rect{
		Left:   n.x,
		Right:  n.x + n.width,
		Top:    n.y,
		Bottom: n.y + n.height,
	}
}

// IntersectLineRect finds where a ray from `from` toward `to` exits rectangle `r`.
// Returns the intersection point, which side, and the offset along that side.
// The offset is measured from the side's start (top-left corner for that side):
//   - Left/Right sides: offset from top edge (Y distance)
//   - Top/Bottom sides: offset from left edge (X distance)
//
// If `from` is at the center of the rectangle and `to` is outside,
// this finds the exit point. If `from` is outside, behavior is undefined.
func IntersectLineRect(from, to EdgePoint, r Rect) IntersectResult {
	// Direction vector
	dx := to.X - from.X
	dy := to.Y - from.Y

	// Handle zero movement
	if dx == 0 && dy == 0 {
		// No direction - pick a default exit (right side, center)
		return IntersectResult{
			Point:  EdgePoint{X: r.Right, Y: (r.Top + r.Bottom) / 2},
			Side:   Right,
			Offset: (r.Bottom - r.Top) / 2,
		}
	}

	// Parametric line: P = from + t * (to - from)
	// Find t where line crosses each edge. We want the smallest positive t
	// that results in a point on the rectangle boundary.
	var bestT float64 = 2.0 // Start with value > 1 (invalid)
	var exitSide Side
	var exitPoint EdgePoint

	// Check each edge
	type edgeInfo struct {
		side   Side
		coord  float64 // edge position
		isVert bool    // true = vertical edge (left/right), false = horizontal (top/bottom)
	}
	edges := []edgeInfo{
		{Left, r.Left, true},
		{Right, r.Right, true},
		{Top, r.Top, false},
		{Bottom, r.Bottom, false},
	}

	for _, edge := range edges {
		var t float64
		if edge.isVert { // vertical edge (left/right)
			if dx == 0 {
				continue // parallel to edge
			}
			t = (edge.coord - from.X) / dx
		} else { // horizontal edge (top/bottom)
			if dy == 0 {
				continue
			}
			t = (edge.coord - from.Y) / dy
		}

		// Must be positive (moving toward target) and less than current best
		if t <= 0 || t >= bestT {
			continue
		}

		// Check if intersection is within edge bounds
		p := EdgePoint{X: from.X + t*dx, Y: from.Y + t*dy}
		if edge.isVert {
			if p.Y >= r.Top && p.Y <= r.Bottom {
				bestT = t
				exitSide = edge.side
				exitPoint = p
			}
		} else {
			if p.X >= r.Left && p.X <= r.Right {
				bestT = t
				exitSide = edge.side
				exitPoint = p
			}
		}
	}

	// If no valid intersection found, use center-based fallback
	if bestT >= 2.0 {
		// This shouldn't happen for valid inputs, but provide a sensible default
		return IntersectResult{
			Point:  EdgePoint{X: r.Right, Y: (r.Top + r.Bottom) / 2},
			Side:   Right,
			Offset: (r.Bottom - r.Top) / 2,
		}
	}

	// Compute offset along the side
	var offset float64
	switch exitSide {
	case Left, Right:
		offset = exitPoint.Y - r.Top
	case Top, Bottom:
		offset = exitPoint.X - r.Left
	}

	return IntersectResult{Point: exitPoint, Side: exitSide, Offset: offset}
}

// inferSideFromBoundary determines source and target sides by computing
// where a line between node centers intersects each node's boundary.
// Returns sides in internal coordinate space.
func inferSideFromBoundary(fromNode, toNode *layoutNode) (sourceSide, targetSide Side) {
	fromCenter := EdgePoint{
		X: fromNode.x + fromNode.width/2,
		Y: fromNode.y + fromNode.height/2,
	}
	toCenter := EdgePoint{
		X: toNode.x + toNode.width/2,
		Y: toNode.y + toNode.height/2,
	}

	// Find where line from source center exits source boundary
	srcResult := IntersectLineRect(fromCenter, toCenter, nodeRect(fromNode))

	// Find where line from target center exits target boundary (toward source)
	tgtResult := IntersectLineRect(toCenter, fromCenter, nodeRect(toNode))

	return srcResult.Side, tgtResult.Side
}

// edgePortSideBoundary computes the attachment side for a PortFixedOffset port
// using boundary intersection. The side is determined by where a ray from the
// port's position toward the connected node's center exits the node boundary.
//
// Returns a user-space side that should be converted to internal space by the caller.
func (s *layoutState) edgePortSideBoundary(port *PortOptions, thisNode, connNode *layoutNode) Side {
	// Port position: use offset along the node (we don't know the side yet,
	// so start from center and find the exit direction)
	var portPoint EdgePoint
	switch port.Axis {
	case PortAxisHorizontal:
		// Port could be on left or right; use the declared offset as Y position
		portPoint = EdgePoint{
			X: thisNode.x + thisNode.width/2,
			Y: thisNode.y + port.Offset,
		}
	case PortAxisVertical:
		// Port could be on top or bottom; use the declared offset as X position
		portPoint = EdgePoint{
			X: thisNode.x + port.Offset,
			Y: thisNode.y + thisNode.height/2,
		}
	default:
		// PortAxisAny: use node center
		portPoint = EdgePoint{
			X: thisNode.x + thisNode.width/2,
			Y: thisNode.y + thisNode.height/2,
		}
	}

	// Target point: connected node center
	tgtPoint := EdgePoint{
		X: connNode.x + connNode.width/2,
		Y: connNode.y + connNode.height/2,
	}

	// Find exit intersection
	result := IntersectLineRect(portPoint, tgtPoint, nodeRect(thisNode))

	// Result.Side is in internal space - transform to user space
	side := s.internalToUserSide(result.Side)

	// Apply axis constraint (in user space): if intersection is on wrong axis, force to correct side
	switch port.Axis {
	case PortAxisHorizontal:
		if side == Top || side == Bottom {
			// Transform direction to user space to determine side
			dx, dy := tgtPoint.X-portPoint.X, tgtPoint.Y-portPoint.Y
			dx, dy = s.internalToUserDirection(dx, dy)
			if dx >= 0 {
				return Right
			}
			return Left
		}
	case PortAxisVertical:
		if side == Left || side == Right {
			// Transform direction to user space to determine side
			dx, dy := tgtPoint.X-portPoint.X, tgtPoint.Y-portPoint.Y
			dx, dy = s.internalToUserDirection(dx, dy)
			if dy >= 0 {
				return Bottom
			}
			return Top
		}
	}

	return side
}

// internalToUserSide transforms a side from internal layout space to user-visible space.
// This is the inverse of portSideToInternal. Since rotateLR and rotateRL are involutions
// (self-inverse functions), we use the same function for both directions.
func (s *layoutState) internalToUserSide(side Side) Side {
	switch s.opts.Direction {
	case LeftToRight:
		// rotateLR is its own inverse
		return rotateLR(side)
	case RightToLeft:
		// rotateRL is its own inverse
		return rotateRL(side)
	case BottomToTop:
		// flipVertical is its own inverse
		return flipVertical(side)
	default:
		return side
	}
}

// assignPortSideFromBoundary determines the best side for a PortFree/PortFixedOffset
// port using boundary intersection voting across all connected nodes.
//
// Returns a user-space side.
func (s *layoutState) assignPortSideFromBoundary(node *layoutNode, port *PortOptions) Side {
	// Find all nodes connected to this port
	connectedNodes := s.getNodesConnectedToPort(node.id, port.ID)
	if len(connectedNodes) == 0 {
		return s.defaultSide(port.Axis)
	}

	// Vote: for each connected node, compute boundary intersection side
	votes := make(map[Side]int)

	// Determine starting point based on axis (in internal coordinates)
	var portPoint EdgePoint
	switch port.Axis {
	case PortAxisHorizontal:
		portPoint = EdgePoint{
			X: node.x + node.width/2,
			Y: node.y + port.Offset,
		}
	case PortAxisVertical:
		portPoint = EdgePoint{
			X: node.x + port.Offset,
			Y: node.y + node.height/2,
		}
	default:
		portPoint = EdgePoint{
			X: node.x + node.width/2,
			Y: node.y + node.height/2,
		}
	}

	for _, conn := range connectedNodes {
		connCenter := EdgePoint{
			X: conn.x + conn.width/2,
			Y: conn.y + conn.height/2,
		}
		result := IntersectLineRect(portPoint, connCenter, nodeRect(node))

		// Result.Side is in internal space - transform to user space
		side := s.internalToUserSide(result.Side)

		// Apply axis constraint (in user space)
		if port.Axis == PortAxisHorizontal && (side == Top || side == Bottom) {
			// Transform connected node direction to user space to determine side
			dx, dy := connCenter.X-portPoint.X, connCenter.Y-portPoint.Y
			dx, dy = s.internalToUserDirection(dx, dy)
			if dx >= 0 {
				side = Right
			} else {
				side = Left
			}
		} else if port.Axis == PortAxisVertical && (side == Left || side == Right) {
			dx, dy := connCenter.X-portPoint.X, connCenter.Y-portPoint.Y
			dx, dy = s.internalToUserDirection(dx, dy)
			if dy >= 0 {
				side = Bottom
			} else {
				side = Top
			}
		}
		votes[side]++
	}

	// Return side with most votes
	var bestSide Side
	var maxVotes int
	for side, count := range votes {
		if count > maxVotes {
			maxVotes = count
			bestSide = side
		}
	}
	return bestSide
}

// getNodesConnectedToPort returns all layout nodes connected to a specific port.
func (s *layoutState) getNodesConnectedToPort(nodeID, portID string) []*layoutNode {
	var result []*layoutNode
	for _, edge := range s.edges {
		var connNode *layoutNode
		if edge.key.from == nodeID && edge.sourcePort == portID {
			connNode = s.nodes[edge.key.to]
		} else if edge.key.to == nodeID && edge.targetPort == portID {
			connNode = s.nodes[edge.key.from]
		}
		if connNode != nil {
			result = append(result, connNode)
		}
	}
	return result
}

// defaultSide returns a sensible default side when no connections exist.
func (s *layoutState) defaultSide(axis PortAxis) Side {
	switch axis {
	case PortAxisHorizontal:
		return Right
	case PortAxisVertical:
		return Bottom
	default:
		return Right
	}
}
