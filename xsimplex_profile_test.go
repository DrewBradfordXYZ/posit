package posit

import (
	"fmt"
	"math/rand"
	"testing"
	"time"
)

// TestXSimplexProfile profiles X simplex for CHDI-like graphs to find bottlenecks.
func TestXSimplexProfile(t *testing.T) {
	g := buildCHDIGraphForProfile()
	t.Logf("Graph: %d nodes, %d edges", len(g.nodes), len(g.edges))

	// Run layout to get to position phase
	s := newLayoutState(g, Options{
		Algorithm:       NetworkSimplex,
		XCoordAlgorithm: XNetworkSimplex,
	})

	// Run phases up to coordinate assignment
	start := time.Now()
	s.makeAcyclic()
	s.assignLayers()
	s.addDummyNodes()
	s.minimizeCrossings()
	t.Logf("Pre-X phases: %v", time.Since(start))
	t.Logf("After ranking: %d nodes, %d edges, %d layers",
		len(s.nodes), len(s.edges), len(s.layers))

	// Now profile X simplex phases
	t.Log("\n=== X Simplex Profile ===")

	// Phase 1: Simple placement
	start = time.Now()
	s.assignXCoordinatesSimple()
	t.Logf("1. Simple placement:     %v", time.Since(start))

	// Create simplex state
	xs := &xSimplexState{
		s:        s,
		auxNodes: make(map[string]*xAuxNode),
		auxEdges: make(map[xEdgeKey]*xAuxEdge),
		auxSucc:  make(map[string][]string),
		auxPred:  make(map[string][]string),
	}

	// Phase 2: Build auxiliary graph
	start = time.Now()
	xs.buildAuxiliaryGraph()
	t.Logf("2. Build aux graph:      %v (nodes=%d, edges=%d)",
		time.Since(start), len(xs.auxNodes), len(xs.auxEdges))

	// Phase 3: Remove leaf subtrees
	start = time.Now()
	xs.removeAuxSubtreeLeaves()
	t.Logf("3. Remove leaves:        %v (remaining=%d nodes)",
		time.Since(start), len(xs.auxNodes))

	// Phase 4: Feasible tree
	start = time.Now()
	xs.tree = xs.xFeasibleTree()
	t.Logf("4. Feasible tree:        %v (tree edges=%d)",
		time.Since(start), len(xs.tree.treeEdges)/2)

	// Phase 5: Init low/lim values
	start = time.Now()
	xs.tree.initLowLim()
	t.Logf("5. Init low/lim:         %v", time.Since(start))

	// Phase 6: Init cut values
	start = time.Now()
	xs.initCutValues()
	t.Logf("6. Init cut values:      %v", time.Since(start))

	// Phase 7: Sort edges (one-time)
	start = time.Now()
	xs.sortedEdgeKeys = make([]xEdgeKey, 0, len(xs.auxEdges))
	for key := range xs.auxEdges {
		xs.sortedEdgeKeys = append(xs.sortedEdgeKeys, key)
	}
	t.Logf("7. Cache edge keys:      %v", time.Since(start))

	// Phase 8: Simplex loop (the main loop)
	maxIterations := max(len(xs.auxNodes)*len(xs.auxEdges), 1000)
	t.Logf("\n=== Simplex Loop (max %d iterations) ===", maxIterations)

	// Track timing per 100 iterations
	iterCount := 0
	loopStart := time.Now()
	batchStart := time.Now()
	leaveTime := time.Duration(0)
	enterTime := time.Duration(0)
	exchangeTime := time.Duration(0)

	for i := 0; i < maxIterations; i++ {
		// Time leaveEdge
		t1 := time.Now()
		leave, found := xs.leaveEdge()
		leaveTime += time.Since(t1)

		if !found {
			t.Logf("Converged after %d iterations", i)
			break
		}

		// Time enterEdge
		t2 := time.Now()
		enter := xs.enterEdge(leave)
		enterTime += time.Since(t2)

		if enter.from == "" {
			t.Logf("No entering edge at iteration %d", i)
			break
		}

		// Time exchangeEdges
		t3 := time.Now()
		xs.exchangeEdges(leave, enter)
		exchangeTime += time.Since(t3)

		iterCount++

		// Report every 100 iterations
		if (i+1)%100 == 0 {
			t.Logf("  Iterations %d-%d: %v (leave=%v, enter=%v, exchange=%v)",
				i-99, i, time.Since(batchStart),
				leaveTime, enterTime, exchangeTime)
			batchStart = time.Now()
			leaveTime = 0
			enterTime = 0
			exchangeTime = 0
		}
	}

	t.Logf("8. Simplex loop total:   %v (%d iterations)", time.Since(loopStart), iterCount)

	// Phase 9: Reattach leaves
	start = time.Now()
	xs.reattachAuxSubtreeLeaves()
	t.Logf("9. Reattach leaves:      %v", time.Since(start))

	// Phase 10: Extract coordinates
	start = time.Now()
	xs.extractCoordinates()
	t.Logf("10. Extract coords:      %v", time.Since(start))
}

// buildCHDIGraphForProfile creates a CHDI-like graph for profiling
func buildCHDIGraphForProfile() *Graph {
	g := NewGraph()

	hubs := []struct {
		name        string
		childEdges  int
		parentEdges int
	}{
		{"AgreementWorkflows", 92, 111},
		{"PaymentWorkflows", 120, 6},
		{"ClinicalStudies", 64, 17},
		{"ProjectWorkflowDocs", 30, 56},
		{"ProjectCodes", 4, 58},
		{"Organizations", 4, 45},
		{"OrgContacts", 4, 34},
		{"Materials", 15, 3},
		{"MaterialWebSubs", 32, 2},
		{"AnimalLineWebSubs", 22, 2},
		{"AnimalLines", 25, 2},
	}

	for _, h := range hubs {
		g.AddNode(h.name, NodeOptions{Width: 120, Height: 40})
	}

	regularTables := 98
	for i := 0; i < regularTables; i++ {
		g.AddNode(fmt.Sprintf("Table%d", i), NodeOptions{Width: 80, Height: 30})
	}

	rng := rand.New(rand.NewSource(42))

	for i := 0; i < len(hubs); i++ {
		for j := i + 1; j < len(hubs); j++ {
			if rng.Float32() < 0.6 {
				g.AddEdge(hubs[i].name, hubs[j].name)
			}
		}
	}

	regularTableNames := make([]string, regularTables)
	for i := 0; i < regularTables; i++ {
		regularTableNames[i] = fmt.Sprintf("Table%d", i)
	}

	for _, h := range hubs {
		targetCount := h.childEdges / 2
		if targetCount > regularTables {
			targetCount = regularTables / 2
		}
		targets := rng.Perm(regularTables)[:targetCount]
		for _, t := range targets {
			g.AddEdge(h.name, regularTableNames[t])
		}

		sourceCount := h.parentEdges / 2
		if sourceCount > regularTables {
			sourceCount = regularTables / 2
		}
		sources := rng.Perm(regularTables)[:sourceCount]
		for _, s := range sources {
			g.AddEdge(regularTableNames[s], h.name)
		}
	}

	for i := 0; i < regularTables; i++ {
		edgeCount := rng.Intn(3)
		for j := 0; j < edgeCount; j++ {
			target := rng.Intn(regularTables)
			if target != i {
				g.AddEdge(regularTableNames[i], regularTableNames[target])
			}
		}
	}

	return g
}
