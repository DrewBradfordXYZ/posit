// Benchmark comparison: dagre vs MSAGL vs ELK vs Posit
// Uses graph profiles exported from Go (profiles.json) to ensure identical graphs.
// Generate profiles: go test -run TestBenchmarkReport -bench-export

import dagre from '@dagrejs/dagre';
import ELK from 'elkjs';
import { readFileSync, writeFileSync, existsSync } from 'fs';
import { dirname, join } from 'path';
import { fileURLToPath } from 'url';

const __dirname = dirname(fileURLToPath(import.meta.url));

// --- Load Graph Profiles ---

const profilesPath = join(__dirname, 'profiles.json');
if (!existsSync(profilesPath)) {
  console.error('Error: profiles.json not found.');
  console.error('Generate it with: go test -run TestBenchmarkReport -bench-export');
  process.exit(1);
}

const profiles = JSON.parse(readFileSync(profilesPath, 'utf8'));

// --- Crossing Detection ---

function countCrossings(edgePoints) {
  // edgePoints: array of arrays of {x,y} points (one array per edge)
  const segments = [];
  for (let ei = 0; ei < edgePoints.length; ei++) {
    const pts = edgePoints[ei];
    if (pts.length < 2) continue;
    for (let i = 0; i < pts.length - 1; i++) {
      segments.push({
        edge: ei,
        x1: pts[i].x, y1: pts[i].y,
        x2: pts[i + 1].x, y2: pts[i + 1].y,
        minX: Math.min(pts[i].x, pts[i + 1].x),
        minY: Math.min(pts[i].y, pts[i + 1].y),
        maxX: Math.max(pts[i].x, pts[i + 1].x),
        maxY: Math.max(pts[i].y, pts[i + 1].y),
      });
    }
  }

  let crossings = 0;
  for (let i = 0; i < segments.length; i++) {
    for (let j = i + 1; j < segments.length; j++) {
      if (segments[i].edge === segments[j].edge) continue;
      const a = segments[i], b = segments[j];
      // Bounding box pre-filter
      if (a.maxX < b.minX || b.maxX < a.minX ||
          a.maxY < b.minY || b.maxY < a.minY) continue;
      if (segmentsIntersect(a, b)) crossings++;
    }
  }
  return crossings;
}

function segmentsIntersect(a, b) {
  const eps = 1e-9;
  // Skip shared endpoints
  if ((Math.abs(a.x1 - b.x1) < eps && Math.abs(a.y1 - b.y1) < eps) ||
      (Math.abs(a.x1 - b.x2) < eps && Math.abs(a.y1 - b.y2) < eps) ||
      (Math.abs(a.x2 - b.x1) < eps && Math.abs(a.y2 - b.y1) < eps) ||
      (Math.abs(a.x2 - b.x2) < eps && Math.abs(a.y2 - b.y2) < eps)) {
    return false;
  }
  const d1 = cross(b.x1, b.y1, b.x2, b.y2, a.x1, a.y1);
  const d2 = cross(b.x1, b.y1, b.x2, b.y2, a.x2, a.y2);
  const d3 = cross(a.x1, a.y1, a.x2, a.y2, b.x1, b.y1);
  const d4 = cross(a.x1, a.y1, a.x2, a.y2, b.x2, b.y2);
  return ((d1 > 0 && d2 < 0) || (d1 < 0 && d2 > 0)) &&
         ((d3 > 0 && d4 < 0) || (d3 < 0 && d4 > 0));
}

function cross(ax, ay, bx, by, cx, cy) {
  return (bx - ax) * (cy - ay) - (by - ay) * (cx - ax);
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

  // Compute metrics
  let minX = Infinity, minY = Infinity, maxX = -Infinity, maxY = -Infinity;
  const layerYs = new Set();
  for (const id of g.nodes()) {
    const n = g.node(id);
    if (!n) continue;
    if (n.x - n.width / 2 < minX) minX = n.x - n.width / 2;
    if (n.x + n.width / 2 > maxX) maxX = n.x + n.width / 2;
    if (n.y - n.height / 2 < minY) minY = n.y - n.height / 2;
    if (n.y + n.height / 2 > maxY) maxY = n.y + n.height / 2;
    layerYs.add(Math.round(n.y));
  }

  // Collect edge points for crossing detection
  const edgePoints = [];
  for (const e of g.edges()) {
    const edge = g.edge(e);
    if (edge && edge.points) {
      edgePoints.push(edge.points);
    }
  }

  return {
    time_ms: elapsed,
    width: maxX - minX,
    height: maxY - minY,
    layers: layerYs.size,
    crossings: countCrossings(edgePoints),
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

  try {
    const start = performance.now();
    const result = await elk.layout(graph);
    const elapsed = performance.now() - start;

    // Compute metrics
    let minX = Infinity, minY = Infinity, maxX = -Infinity, maxY = -Infinity;
    const layerYs = new Set();
    for (const n of result.children || []) {
      if (n.x < minX) minX = n.x;
      if (n.x + n.width > maxX) maxX = n.x + n.width;
      if (n.y < minY) minY = n.y;
      if (n.y + n.height > maxY) maxY = n.y + n.height;
      layerYs.add(Math.round(n.y));
    }

    // Collect edge points for crossing detection
    const edgePoints = [];
    for (const e of result.edges || []) {
      if (e.sections) {
        for (const section of e.sections) {
          const pts = [];
          if (section.startPoint) pts.push(section.startPoint);
          if (section.bendPoints) pts.push(...section.bendPoints);
          if (section.endPoint) pts.push(section.endPoint);
          if (pts.length >= 2) edgePoints.push(pts);
        }
      }
    }

    return {
      time_ms: elapsed,
      width: maxX - minX,
      height: maxY - minY,
      layers: layerYs.size,
      crossings: countCrossings(edgePoints),
    };
  } catch (e) {
    console.log(`  ELK error on "${profile.name}": ${e.message}`);
    return null;
  }
}

// --- MSAGL Runner ---

let msaglAvailable = false;
let msaglModule = null;

try {
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
        new GeomEdge(edge);
      }
    }

    const geomGraph = new GeomGraph(graph);

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
    const layerYs = new Set();
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
      time_ms: elapsed,
      width: maxX - minX,
      height: maxY - minY,
      layers: layerYs.size,
      crossings: -1, // MSAGL edge points are complex curves, skip crossing count
    };
  } catch (e) {
    console.log(`  MSAGL error on "${profile.name}": ${e.message}`);
    return null;
  }
}

// --- Main ---

const warmupRuns = 2;
const timedRuns = 5;

function median(arr) {
  const sorted = [...arr].sort((a, b) => a - b);
  return sorted[Math.floor(sorted.length / 2)];
}

// Load Posit baseline for comparison
let positBaseline = null;
try {
  const baselinePath = join(__dirname, '..', 'benchmark_baseline.json');
  positBaseline = JSON.parse(readFileSync(baselinePath, 'utf8'));
} catch (e) {
  console.log('Note: No Posit baseline found. Run `go test -run TestBenchmarkReport -bench-save` first.\n');
}

console.log('Benchmark Comparison');
console.log('====================');
console.log(`Timing: median of ${timedRuns} runs (${warmupRuns} warm-up)\n`);

// Header
const libs = ['Posit', 'Dagre', 'ELK'];
if (msaglAvailable) libs.splice(2, 0, 'MSAGL');

console.log(
  'Profile'.padEnd(12) + '| ' +
  libs.map(l => (l + ' ms').padStart(10)).join(' | ') +
  ' | ' + libs.map(l => (l + ' cx').padStart(10)).join(' | ')
);
console.log(
  '-'.repeat(12) + '+' +
  libs.map(() => '-'.repeat(12)).join('+') +
  '-+-' +
  libs.map(() => '-'.repeat(12)).join('+')
);

const results = {};

for (let pi = 0; pi < profiles.length; pi++) {
  const profile = profiles[pi];
  const row = { name: profile.name };

  // Posit (from baseline)
  if (positBaseline && positBaseline.profiles[pi]) {
    row.posit_ms = positBaseline.profiles[pi].time_ms;
    row.posit_cx = positBaseline.profiles[pi].crossings;
  }

  // Dagre — warm-up + median timing
  for (let i = 0; i < warmupRuns; i++) runDagre(profile);
  const dagreTimes = [];
  let dagreResult;
  for (let i = 0; i < timedRuns; i++) {
    dagreResult = runDagre(profile);
    dagreTimes.push(dagreResult.time_ms);
  }
  row.dagre_ms = median(dagreTimes);
  row.dagre_cx = dagreResult.crossings;

  // MSAGL — warm-up + median timing
  if (msaglAvailable) {
    for (let i = 0; i < warmupRuns; i++) await runMsagl(profile);
    const msaglTimes = [];
    let msaglResult;
    for (let i = 0; i < timedRuns; i++) {
      msaglResult = await runMsagl(profile);
      if (msaglResult) msaglTimes.push(msaglResult.time_ms);
    }
    if (msaglTimes.length > 0) {
      row.msagl_ms = median(msaglTimes);
      row.msagl_cx = msaglResult.crossings;
    }
  }

  // ELK — warm-up + median timing
  for (let i = 0; i < warmupRuns; i++) await runElk(profile);
  const elkTimes = [];
  let elkResult;
  for (let i = 0; i < timedRuns; i++) {
    elkResult = await runElk(profile);
    if (elkResult) elkTimes.push(elkResult.time_ms);
  }
  if (elkTimes.length > 0) {
    row.elk_ms = median(elkTimes);
    row.elk_cx = elkResult.crossings;
  }

  results[profile.name] = row;

  // Format row — timing
  const timeCells = [];
  timeCells.push(row.posit_ms != null ? `${row.posit_ms.toFixed(1)}` : 'n/a');
  timeCells.push(row.dagre_ms != null ? `${row.dagre_ms.toFixed(1)}` : 'error');
  if (msaglAvailable) timeCells.push(row.msagl_ms != null ? `${row.msagl_ms.toFixed(1)}` : 'error');
  timeCells.push(row.elk_ms != null ? `${row.elk_ms.toFixed(1)}` : 'error');

  // Format row — crossings
  const cxCells = [];
  cxCells.push(row.posit_cx != null ? `${row.posit_cx}` : 'n/a');
  cxCells.push(row.dagre_cx != null ? `${row.dagre_cx}` : 'n/a');
  if (msaglAvailable) cxCells.push(row.msagl_cx != null && row.msagl_cx >= 0 ? `${row.msagl_cx}` : 'n/a');
  cxCells.push(row.elk_cx != null ? `${row.elk_cx}` : 'n/a');

  console.log(
    profile.name.padEnd(12) + '| ' +
    timeCells.map(c => c.padStart(10)).join(' | ') +
    ' | ' +
    cxCells.map(c => c.padStart(10)).join(' | ')
  );
}

// Save results
const outputPath = join(__dirname, 'results.json');
writeFileSync(outputPath, JSON.stringify(results, null, 2));
console.log(`\nResults saved to ${outputPath}`);
