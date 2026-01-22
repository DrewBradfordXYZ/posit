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
	case LeftToRight:
		s.swapXY()
		s.swapWidthHeight()
	case RightToLeft:
		s.reverseY()
		s.swapXY()
		s.swapWidthHeight()
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
