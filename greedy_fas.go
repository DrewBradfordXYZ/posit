package posit

import (
	"slices"
	"sort"
)

// negativeInfinity is used as an initial value when finding the maximum
// difference between out-degree and in-degree in the greedy FAS algorithm.
const negativeInfinity = -1e30

// makeAcyclicGreedy implements the Eades/Lin/Smyth greedy heuristic for
// feedback arc set (FAS) minimization. This algorithm produces better results
// for weighted graphs by ordering nodes based on in/out degree differences.
//
// Algorithm:
// 1. Partition nodes into buckets based on in/out degree
// 2. Repeatedly extract sources (no incoming) and sinks (no outgoing)
// 3. When no sources/sinks remain, pick node with max (out-degree - in-degree)
// 4. Edges pointing "backward" in this ordering are reversed
func (s *layoutState) makeAcyclicGreedy() {
	// Handle self-loops first (same as DFS approach)
	s.removeSelfLoops()

	if len(s.nodes) == 0 {
		return
	}

	// Build weighted degree info
	// For each node, track weighted in-degree and out-degree
	type degreeInfo struct {
		id         string
		inDegree   float64
		outDegree  float64
		inEdges    []edgeKey
		outEdges   []edgeKey
	}

	degrees := make(map[string]*degreeInfo)
	for id := range s.nodes {
		degrees[id] = &degreeInfo{id: id}
	}

	// Calculate initial degrees from edges
	for key, edge := range s.edges {
		if info := degrees[key.from]; info != nil {
			info.outDegree += edge.weight
			info.outEdges = append(info.outEdges, key)
		}
		if info := degrees[key.to]; info != nil {
			info.inDegree += edge.weight
			info.inEdges = append(info.inEdges, key)
		}
	}

	// Build the node ordering using greedy selection
	// sL: nodes added from the left (sources and max-diff nodes)
	// sR: nodes added from the right (sinks)
	sL := make([]string, 0, len(s.nodes))
	sR := make([]string, 0, len(s.nodes))
	remaining := make(map[string]bool)
	for id := range s.nodes {
		remaining[id] = true
	}

	for len(remaining) > 0 {
		// Find sources (no incoming edges from remaining nodes)
		sources := make([]string, 0)
		for id := range remaining {
			info := degrees[id]
			hasIncoming := false
			for _, key := range info.inEdges {
				if remaining[key.from] {
					hasIncoming = true
					break
				}
			}
			if !hasIncoming {
				sources = append(sources, id)
			}
		}

		// Process all sources
		sort.Strings(sources) // Deterministic ordering
		for _, id := range sources {
			sL = append(sL, id)
			delete(remaining, id)
		}

		if len(remaining) == 0 {
			break
		}

		// Find sinks (no outgoing edges to remaining nodes)
		sinks := make([]string, 0)
		for id := range remaining {
			info := degrees[id]
			hasOutgoing := false
			for _, key := range info.outEdges {
				if remaining[key.to] {
					hasOutgoing = true
					break
				}
			}
			if !hasOutgoing {
				sinks = append(sinks, id)
			}
		}

		// Process all sinks
		sort.Strings(sinks) // Deterministic ordering
		for _, id := range sinks {
			sR = append(sR, id)
			delete(remaining, id)
		}

		if len(remaining) == 0 {
			break
		}

		// If no sources or sinks, pick node with max (outDegree - inDegree)
		// considering only edges to/from remaining nodes
		var maxNode string
		maxDiff := negativeInfinity

		for id := range remaining {
			info := degrees[id]
			// Calculate effective degrees (only counting remaining neighbors)
			effectiveOut := 0.0
			for _, key := range info.outEdges {
				if remaining[key.to] {
					effectiveOut += s.edges[key].weight
				}
			}
			effectiveIn := 0.0
			for _, key := range info.inEdges {
				if remaining[key.from] {
					effectiveIn += s.edges[key].weight
				}
			}

			diff := effectiveOut - effectiveIn
			if diff > maxDiff || (diff == maxDiff && id < maxNode) {
				maxDiff = diff
				maxNode = id
			}
		}

		if maxNode != "" {
			sL = append(sL, maxNode)
			delete(remaining, maxNode)
		}
	}

	// Final ordering: sL followed by reversed sR
	slices.Reverse(sR)
	ordering := append(sL, sR...)

	// Build position map for the ordering
	position := make(map[string]int)
	for i, id := range ordering {
		position[id] = i
	}

	// Reverse edges that point backward in the ordering
	var reversed []edgeKey
	for key := range s.edges {
		if position[key.from] > position[key.to] {
			// Edge is backward - reverse it
			s.reverseEdge(key)
			reversed = append(reversed, key)
		}
	}

	s.reversedEdges = reversed
}
