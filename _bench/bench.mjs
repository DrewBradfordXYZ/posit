// Benchmark comparison: dagre vs MSAGL vs ELK vs Posit
// Builds the same 5 graph profiles as benchmark_test.go and measures each library.

import dagre from '@dagrejs/dagre';
import ELK from 'elkjs';
import { readFileSync, writeFileSync } from 'fs';
import { dirname, join } from 'path';
import { fileURLToPath } from 'url';

const __dirname = dirname(fileURLToPath(import.meta.url));

// --- Seeded PRNG (mulberry32) ---
function makeRng(seed) {
  let s = seed | 0;
  return {
    // Returns integer in [0, n)
    intn(n) {
      s |= 0; s = s + 0x6D2B79F5 | 0;
      let t = Math.imul(s ^ s >>> 15, 1 | s);
      t = t + Math.imul(t ^ t >>> 7, 61 | t) ^ t;
      const val = ((t ^ t >>> 14) >>> 0) / 4294967296;
      return Math.floor(val * n);
    },
    // Returns float in [0, 1)
    float() {
      s |= 0; s = s + 0x6D2B79F5 | 0;
      let t = Math.imul(s ^ s >>> 15, 1 | s);
      t = t + Math.imul(t ^ t >>> 7, 61 | t) ^ t;
      return ((t ^ t >>> 14) >>> 0) / 4294967296;
    },
    // Returns a permutation of [0..n-1], take first k
    perm(n, k) {
      const arr = Array.from({length: n}, (_, i) => i);
      for (let i = arr.length - 1; i > 0; i--) {
        const j = this.intn(i + 1);
        [arr[i], arr[j]] = [arr[j], arr[i]];
      }
      return arr.slice(0, k);
    }
  };
}

// --- Graph Builders (same profiles as benchmark_test.go) ---

function buildLargeGraph() {
  const nodeCount = 500, edgeCount = 1000;
  const nodes = [];
  const edges = [];
  for (let i = 0; i < nodeCount; i++) {
    nodes.push({ id: `n${i}`, width: 50, height: 30 });
  }
  const rng = makeRng(42);
  for (let i = 0; i < edgeCount; i++) {
    const from = `n${rng.intn(nodeCount)}`;
    const to = `n${rng.intn(nodeCount)}`;
    if (from !== to) edges.push({ from, to });
  }
  return { name: 'Large (500n/1000e)', nodes, edges };
}

function buildDenseGraph() {
  const nodeCount = 100;
  const nodes = [];
  const edges = [];
  for (let i = 0; i < nodeCount; i++) {
    nodes.push({ id: `n${i}`, width: 50, height: 30 });
  }
  const rng = makeRng(42);
  for (let i = 0; i < nodeCount; i++) {
    for (let j = 0; j < nodeCount; j++) {
      if (i !== j && rng.float() < 0.20) {
        edges.push({ from: `n${i}`, to: `n${j}` });
      }
    }
  }
  return { name: 'Dense (100n/~2000e)', nodes, edges };
}

function buildWideGraph() {
  const width = 100, depth = 5;
  const nodes = [];
  const edges = [];
  for (let layer = 0; layer < depth; layer++) {
    for (let i = 0; i < width; i++) {
      nodes.push({ id: `n${layer}_${i}`, width: 50, height: 30 });
    }
  }
  const rng = makeRng(42);
  for (let layer = 0; layer < depth - 1; layer++) {
    for (let i = 0; i < width; i++) {
      const targets = rng.perm(width, 3);
      for (const j of targets) {
        edges.push({ from: `n${layer}_${i}`, to: `n${layer + 1}_${j}` });
      }
    }
  }
  return { name: 'Wide (100x5)', nodes, edges };
}

function buildDeepGraph() {
  const depth = 200;
  const nodes = [];
  const edges = [];
  for (let i = 0; i < depth; i++) {
    nodes.push({ id: `n${i}`, width: 50, height: 30 });
  }
  for (let i = 0; i < depth - 1; i++) {
    edges.push({ from: `n${i}`, to: `n${i + 1}` });
  }
  return { name: 'Deep (200-chain)', nodes, edges };
}

function buildMediumGraph() {
  const nodeCount = 100, edgeCount = 200;
  const nodes = [];
  const edges = [];
  for (let i = 0; i < nodeCount; i++) {
    nodes.push({ id: `n${i}`, width: 50, height: 30 });
  }
  const rng = makeRng(42);
  for (let i = 0; i < edgeCount; i++) {
    const from = `n${rng.intn(nodeCount)}`;
    const to = `n${rng.intn(nodeCount)}`;
    if (from !== to) edges.push({ from, to });
  }
  return { name: 'Medium (100n/200e)', nodes, edges };
}

// --- Dagre Runner ---

function runDagre(profile) {
  const g = new dagre.graphlib.Graph();
  g.setGraph({ rankdir: 'TB', nodesep: 50, ranksep: 75 });
  g.setDefaultEdgeLabel(() => ({}));

  for (const node of profile.nodes) {
    g.setNode(node.id, { width: node.width, height: node.height });
  }
  for (const edge of profile.edges) {
    g.setEdge(edge.from, edge.to);
  }

  const start = performance.now();
  dagre.layout(g);
  const elapsed = performance.now() - start;

  // Compute metrics from result
  let minX = Infinity, minY = Infinity, maxX = -Infinity, maxY = -Infinity;
  let layerYs = new Set();
  for (const id of g.nodes()) {
    const n = g.node(id);
    if (!n) continue;
    const left = n.x - n.width / 2;
    const right = n.x + n.width / 2;
    const top = n.y - n.height / 2;
    const bottom = n.y + n.height / 2;
    if (left < minX) minX = left;
    if (right > maxX) maxX = right;
    if (top < minY) minY = top;
    if (bottom > maxY) maxY = bottom;
    layerYs.add(Math.round(n.y));
  }

  return {
    time_ms: Math.round(elapsed),
    width: Math.round(maxX - minX),
    height: Math.round(maxY - minY),
    layers: layerYs.size,
  };
}

// --- ELK Runner ---

async function runElk(profile) {
  const elk = new ELK();

  const graph = {
    id: 'root',
    layoutOptions: {
      'elk.algorithm': 'layered',
      'elk.direction': 'DOWN',
      'elk.spacing.nodeNode': '50',
      'elk.layered.spacing.nodeNodeBetweenLayers': '75',
    },
    children: profile.nodes.map(n => ({
      id: n.id,
      width: n.width,
      height: n.height,
    })),
    edges: profile.edges.map((e, i) => ({
      id: `e${i}`,
      sources: [e.from],
      targets: [e.to],
    })),
  };

  const start = performance.now();
  const result = await elk.layout(graph);
  const elapsed = performance.now() - start;

  // Compute metrics
  let minX = Infinity, minY = Infinity, maxX = -Infinity, maxY = -Infinity;
  let layerYs = new Set();
  for (const n of result.children || []) {
    const left = n.x;
    const right = n.x + n.width;
    const top = n.y;
    const bottom = n.y + n.height;
    if (left < minX) minX = left;
    if (right > maxX) maxX = right;
    if (top < minY) minY = top;
    if (bottom > maxY) maxY = bottom;
    layerYs.add(Math.round(n.y));
  }

  return {
    time_ms: Math.round(elapsed),
    width: Math.round(maxX - minX),
    height: Math.round(maxY - minY),
    layers: layerYs.size,
  };
}

// --- MSAGL Runner ---

let msaglAvailable = false;
let msaglModule = null;

try {
  // Use pre-bundled MSAGL (the npm package has broken ESM imports for Node.js)
  msaglModule = await import('./msagl-bundle.mjs');
  msaglAvailable = true;
} catch (e) {
  console.log(`Note: MSAGL not available (${e.message}).`);
  console.log('  Run: npx esbuild node_modules/@msagl/core/dist/index.js --bundle --format=esm --outfile=msagl-bundle.mjs --platform=node');
}

async function runMsagl(profile) {
  if (!msaglAvailable) return null;

  const { Graph, Node, Edge, GeomGraph, GeomNode, GeomEdge,
          Rectangle, SugiyamaLayoutSettings, layoutGraphWithSugiayma } = msaglModule;

  try {
    const graph = new Graph();
    const nodeMap = new Map();

    for (const n of profile.nodes) {
      const node = new Node(n.id);
      graph.addNode(node);
      nodeMap.set(n.id, node);
    }

    for (const e of profile.edges) {
      const src = nodeMap.get(e.from);
      const tgt = nodeMap.get(e.to);
      if (src && tgt) {
        const edge = new Edge(src, tgt);
        new GeomEdge(edge); // MSAGL requires GeomEdge for each edge
      }
    }

    const geomGraph = new GeomGraph(graph);

    // Set node sizes via bounding boxes
    for (const n of profile.nodes) {
      const node = nodeMap.get(n.id);
      const gn = new GeomNode(node);
      gn.boundaryCurve = Rectangle.mkPP(
        { x: 0, y: 0 },
        { x: n.width, y: n.height }
      ).perimeter();
    }

    geomGraph.layoutSettings = new SugiyamaLayoutSettings();

    const start = performance.now();
    layoutGraphWithSugiayma(geomGraph, null, false);
    const elapsed = performance.now() - start;

    // Compute metrics
    let minX = Infinity, minY = Infinity, maxX = -Infinity, maxY = -Infinity;
    let layerYs = new Set();
    for (const n of profile.nodes) {
      const node = nodeMap.get(n.id);
      const gn = GeomNode.getGeom(node);
      if (!gn || !gn.boundaryCurve) continue;
      const bb = gn.boundingBox;
      if (bb.left < minX) minX = bb.left;
      if (bb.right > maxX) maxX = bb.right;
      if (bb.bottom < minY) minY = bb.bottom;
      if (bb.top > maxY) maxY = bb.top;
      layerYs.add(Math.round(bb.bottom));
    }

    return {
      time_ms: Math.round(elapsed),
      width: Math.round(maxX - minX),
      height: Math.round(maxY - minY),
      layers: layerYs.size,
    };
  } catch (e) {
    console.log(`  MSAGL error: ${e.message}`);
    return null;
  }
}

// --- Main ---

const profiles = [
  buildLargeGraph(),
  buildDenseGraph(),
  buildWideGraph(),
  buildDeepGraph(),
  buildMediumGraph(),
];

// Load Posit baseline for comparison
let positBaseline = null;
try {
  const baselinePath = join(__dirname, '..', 'benchmark_baseline.json');
  positBaseline = JSON.parse(readFileSync(baselinePath, 'utf8'));
} catch (e) {
  console.log('Note: No Posit baseline found. Run `go test -run TestBenchmarkReport -bench-save` first.\n');
}

console.log('Benchmark Comparison');
console.log('====================\n');

// Header
const libs = ['Posit', 'Dagre', 'ELK'];
if (msaglAvailable) libs.splice(2, 0, 'MSAGL');

console.log(
  'Profile'.padEnd(22) + '| ' +
  libs.map(l => l.padStart(10)).join(' | ')
);
console.log('-'.repeat(22) + '+' + libs.map(() => '-'.repeat(12)).join('+'));

const results = {};

for (let i = 0; i < profiles.length; i++) {
  const profile = profiles[i];
  const row = { name: profile.name };

  // Posit (from baseline)
  if (positBaseline && positBaseline.profiles[i]) {
    row.posit = positBaseline.profiles[i].time_ms;
  }

  // Dagre
  row.dagre = runDagre(profile).time_ms;

  // MSAGL
  if (msaglAvailable) {
    const msaglResult = await runMsagl(profile);
    row.msagl = msaglResult ? msaglResult.time_ms : null;
  }

  // ELK
  row.elk = (await runElk(profile)).time_ms;

  results[profile.name] = row;

  // Format row
  const cells = [];
  cells.push(row.posit != null ? `${row.posit}ms` : 'n/a');
  cells.push(`${row.dagre}ms`);
  if (msaglAvailable) cells.push(row.msagl != null ? `${row.msagl}ms` : 'error');
  cells.push(`${row.elk}ms`);

  console.log(
    profile.name.padEnd(22) + '| ' +
    cells.map(c => c.padStart(10)).join(' | ')
  );
}

// Save results
const outputPath = join(__dirname, 'results.json');
writeFileSync(outputPath, JSON.stringify(results, null, 2));
console.log(`\nResults saved to ${outputPath}`);
