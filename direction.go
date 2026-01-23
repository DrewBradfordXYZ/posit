package posit

// adjustForDirection transforms the graph before layout if needed.
// For LR/RL directions, swaps width/height so layout treats X as Y.
func (s *layoutState) adjustForDirection() {
	switch s.opts.Direction {
	case LeftToRight, RightToLeft:
		// Swap width and height for all nodes
		for _, node := range s.nodes {
			node.width, node.height = node.height, node.width
		}
	}
}

// undoDirectionAdjustment transforms coordinates after layout.
func (s *layoutState) undoDirectionAdjustment() {
	switch s.opts.Direction {
	case BottomToTop:
		s.reverseY()
		s.flipSidesVertical()
	case LeftToRight:
		s.swapXY()
		s.swapWidthHeight()
		s.rotateSidesLR()
	case RightToLeft:
		s.reverseY()
		s.swapXY()
		s.swapWidthHeight()
		s.rotateSidesRL()
	}
}

// flipSidesVertical swaps Top/Bottom sides (for BottomToTop).
func (s *layoutState) flipSidesVertical() {
	for _, edge := range s.edges {
		edge.sourceSide = flipVertical(edge.sourceSide)
		edge.targetSide = flipVertical(edge.targetSide)
	}
}

func flipVertical(side Side) Side {
	switch side {
	case Top:
		return Bottom
	case Bottom:
		return Top
	default:
		return side
	}
}

// rotateSidesLR maps internal TopToBottom sides to LeftToRight.
// Internal Top→Left, Bottom→Right, Left→Top, Right→Bottom
func (s *layoutState) rotateSidesLR() {
	for _, edge := range s.edges {
		edge.sourceSide = rotateLR(edge.sourceSide)
		edge.targetSide = rotateLR(edge.targetSide)
	}
}

func rotateLR(side Side) Side {
	switch side {
	case Top:
		return Left
	case Bottom:
		return Right
	case Left:
		return Top
	case Right:
		return Bottom
	default:
		return side
	}
}

// rotateSidesRL maps internal TopToBottom sides to RightToLeft.
// Internal Top→Right, Bottom→Left, Left→Bottom, Right→Top
func (s *layoutState) rotateSidesRL() {
	for _, edge := range s.edges {
		edge.sourceSide = rotateRL(edge.sourceSide)
		edge.targetSide = rotateRL(edge.targetSide)
	}
}

func rotateRL(side Side) Side {
	switch side {
	case Top:
		return Right
	case Bottom:
		return Left
	case Left:
		return Bottom
	case Right:
		return Top
	default:
		return side
	}
}

// reverseY flips Y coordinates (for BT and RL).
func (s *layoutState) reverseY() {
	// Find max Y (bottom of lowest node)
	maxY := 0.0
	for _, node := range s.nodes {
		bottom := node.y + node.height
		if bottom > maxY {
			maxY = bottom
		}
	}

	// Flip all Y coordinates
	for _, node := range s.nodes {
		node.y = maxY - node.y - node.height
	}

	// Flip edge points
	for _, edge := range s.edges {
		for i := range edge.points {
			edge.points[i].Y = maxY - edge.points[i].Y
		}
	}
}

// swapXY exchanges X and Y coordinates (for LR and RL).
func (s *layoutState) swapXY() {
	for _, node := range s.nodes {
		node.x, node.y = node.y, node.x
	}

	for _, edge := range s.edges {
		for i := range edge.points {
			edge.points[i].X, edge.points[i].Y = edge.points[i].Y, edge.points[i].X
		}
	}
}

// swapWidthHeight exchanges width and height (for LR and RL).
func (s *layoutState) swapWidthHeight() {
	for _, node := range s.nodes {
		node.width, node.height = node.height, node.width
	}
}
