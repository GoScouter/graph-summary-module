package main

import (
	"encoding/json"
	"strings"
)

type Summary struct {
	Target     string `json:"target"`
	Host       string `json:"host"`
	Subdomains int    `json:"subdomains"`
	Reachable  int    `json:"reachable"`
}

type node struct {
	ID        string     `json:"id"`
	Root      bool       `json:"root"`
	Reachable bool       `json:"reachable"`
	Report    HostResult `json:"report"`
}

type link struct {
	Source string `json:"source"`
	Target string `json:"target"`
}

type payload struct {
	Target string `json:"target"`
	Host   string `json:"host"`
	Found  int    `json:"found"`
	Live   int    `json:"live"`
	Nodes  []node `json:"nodes"`
	Links  []link `json:"links"`
}

func Page(r Result) (string, Summary) {
	root := r.Host
	if root == "" {
		root = r.Target
	}

	// scan probes the target itself alongside its subdomains, so the target is
	// one of the hosts. That entry is the root node — otherwise it would show up
	// twice, once as an empty placeholder and once linked to itself.
	rootIdx := -1
	for i := range r.Hosts {
		if r.Hosts[i].Host == root {
			rootIdx = i
			break
		}
	}

	// Found and Live count the target too; the counts here are about subdomains.
	found, live := r.Found, r.Live
	if r.Host != "" {
		found--
	}
	if rootIdx >= 0 {
		live--
	}
	found, live = max(found, 0), max(live, 0)

	rootNode := node{ID: root, Root: true, Report: HostResult{Host: root}}
	if rootIdx >= 0 {
		rootNode.Report = r.Hosts[rootIdx]
		rootNode.Reachable = r.Hosts[rootIdx].reachable()
	}

	sum := Summary{Target: r.Target, Host: root, Subdomains: found}
	if rootNode.Reachable {
		sum.Reachable++
	}

	p := payload{
		Target: r.Target,
		Host:   root,
		Found:  found,
		Live:   live,
		Nodes:  []node{rootNode},
	}

	for i, h := range r.Hosts {
		if i == rootIdx {
			continue
		}

		up := h.reachable()
		if up {
			sum.Reachable++
		}

		p.Nodes = append(p.Nodes, node{ID: h.Host, Reachable: up, Report: h})
		p.Links = append(p.Links, link{Source: root, Target: h.Host})
	}

	data, err := json.Marshal(p)
	if err != nil {
		data = []byte("null")
	}

	return strings.Replace(pageTemplate, "/*__GRAPH_DATA__*/", string(data), 1), sum
}

const pageTemplate = `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>GoScouter scan</title>
<style>
  :root { color-scheme: dark; }
  * { box-sizing: border-box; }
  html, body { margin: 0; height: 100%; background: #0d1117; color: #c9d1d9;
    font: 14px/1.5 ui-monospace, SFMono-Regular, Menlo, Consolas, monospace; }
  #wrap { display: flex; height: 100%; }
  #graph { flex: 1; position: relative; }
  canvas { display: block; width: 100%; height: 100%; cursor: grab; }
  canvas:active { cursor: grabbing; }
  #panel { width: 380px; max-width: 45vw; border-left: 1px solid #21262d;
    background: #010409; padding: 18px; overflow-y: auto; }
  #panel h1 { font-size: 15px; margin: 0 0 4px; color: #58a6ff; word-break: break-all; }
  #panel .hint { color: #6e7681; }
  #panel h2 { font-size: 12px; text-transform: uppercase; letter-spacing: .08em;
    color: #8b949e; margin: 18px 0 6px; border-bottom: 1px solid #21262d; padding-bottom: 4px; }
  #panel .kv { display: grid; grid-template-columns: 88px 1fr; gap: 2px 10px; margin-top: 8px; }
  #panel .kv .k { color: #6e7681; }
  #panel .kv .v { color: #c9d1d9; word-break: break-all; }
  #panel .err { color: #f85149; word-break: break-all; }
  #panel pre.out { margin: 6px 0 0; padding: 10px; background: #0d1117;
    border: 1px solid #21262d; border-radius: 6px; overflow-x: auto;
    white-space: pre-wrap; word-break: break-word; color: #c9d1d9; font-size: 12px; }
  #panel .badge { display: inline-block; padding: 1px 8px; border-radius: 999px;
    font-size: 11px; margin-top: 6px; }
  #panel .up { background: #12351d; color: #3fb950; }
  #panel .down { background: #3d1417; color: #f85149; }
  #legend { position: absolute; left: 14px; bottom: 12px; color: #6e7681; font-size: 12px; }
  #legend span { display: inline-flex; align-items: center; margin-right: 14px; }
  #legend i { width: 10px; height: 10px; border-radius: 50%; margin-right: 6px; }
  header { position: absolute; top: 12px; left: 16px; }
  header b { color: #58a6ff; } header small { color: #6e7681; margin-left: 8px; }
  #find { position: absolute; top: 12px; right: 16px; width: 200px; padding: 4px 8px;
    background: #010409; border: 1px solid #21262d; border-radius: 6px; color: #c9d1d9;
    font: inherit; font-size: 12px; }
  #find:focus { outline: none; border-color: #58a6ff; }
</style>
</head>
<body>
<div id="wrap">
  <div id="graph">
    <canvas id="cv"></canvas>
    <header><b>GoScouter</b> scan <small id="tgt"></small></header>
    <input id="find" placeholder="filter hosts" autocomplete="off">
    <div id="legend">
      <span><i style="background:#3fb950"></i>reachable</span>
      <span><i style="background:#6e7681"></i>no response</span>
    </div>
  </div>
  <aside id="panel">
    <h1 id="pTitle">Select a node</h1>
    <div class="hint">Click any host in the graph to inspect every module's output for it. Drag to reposition.</div>
    <div id="pBody"></div>
  </aside>
</div>
<script>
const DATA = /*__GRAPH_DATA__*/;
document.getElementById('tgt').textContent =
  (DATA.target || '') + ' — ' + DATA.found + ' subdomains, ' + DATA.live + ' live';

const cv = document.getElementById('cv'), ctx = cv.getContext('2d');
let W = 0, H = 0, DPR = window.devicePixelRatio || 1;
function resize() {
  W = cv.clientWidth; H = cv.clientHeight;
  cv.width = W * DPR; cv.height = H * DPR;
  ctx.setTransform(DPR, 0, 0, DPR, 0, 0);
}
window.addEventListener('resize', resize); resize();

// Lay nodes on a ring around the pinned root, then relax with a small force sim.
const N = DATA.nodes.map((n, i) => {
  const a = (i / Math.max(1, DATA.nodes.length)) * Math.PI * 2;
  const r = n.root ? 0 : 180 + (i % 5) * 26;
  return { ...n, x: W/2 + Math.cos(a)*r, y: H/2 + Math.sin(a)*r, vx: 0, vy: 0, dim: false };
});
const byId = Object.fromEntries(N.map(n => [n.id, n]));
const L = DATA.links.map(l => ({ s: byId[l.source], t: byId[l.target] })).filter(l => l.s && l.t);

const root = N.find(n => n.root);
let sel = null, drag = null, hover = null;

function step() {
  const cx = W/2, cy = H/2;
  for (const n of N) {
    if (n === drag) continue;
    // centering gravity
    n.vx += (cx - n.x) * 0.0015;
    n.vy += (cy - n.y) * 0.0015;
  }
  // pairwise repulsion
  for (let i = 0; i < N.length; i++) {
    for (let j = i+1; j < N.length; j++) {
      const a = N[i], b = N[j];
      let dx = a.x - b.x, dy = a.y - b.y;
      let d2 = dx*dx + dy*dy || 0.01;
      const f = 6000 / d2;
      const d = Math.sqrt(d2);
      const fx = (dx/d)*f, fy = (dy/d)*f;
      a.vx += fx; a.vy += fy; b.vx -= fx; b.vy -= fy;
    }
  }
  // spring along links
  for (const l of L) {
    let dx = l.t.x - l.s.x, dy = l.t.y - l.s.y;
    const d = Math.sqrt(dx*dx + dy*dy) || 0.01;
    const f = (d - 170) * 0.02;
    const fx = (dx/d)*f, fy = (dy/d)*f;
    l.s.vx += fx; l.s.vy += fy; l.t.vx -= fx; l.t.vy -= fy;
  }
  for (const n of N) {
    if (n === drag) continue;
    n.vx *= 0.86; n.vy *= 0.86;
    n.x += n.vx; n.y += n.vy;
  }
  if (root && root !== drag) { root.x = cx; root.y = cy; root.vx = root.vy = 0; }
}

function radius(n) { return n.root ? 11 : 7; }

function draw() {
  ctx.clearRect(0, 0, W, H);
  ctx.lineWidth = 1;
  for (const l of L) {
    ctx.strokeStyle = (hover && (hover === l.s || hover === l.t)) ? '#3fb95066' : '#21262d';
    ctx.beginPath(); ctx.moveTo(l.s.x, l.s.y); ctx.lineTo(l.t.x, l.t.y); ctx.stroke();
  }
  for (const n of N) {
    ctx.globalAlpha = n.dim ? 0.15 : 1;
    const col = n.reachable ? '#3fb950' : '#6e7681';
    ctx.beginPath(); ctx.arc(n.x, n.y, radius(n), 0, Math.PI*2);
    ctx.fillStyle = col; ctx.fill();
    if (n === sel) { ctx.strokeStyle = '#58a6ff'; ctx.lineWidth = 2.5; ctx.stroke(); ctx.lineWidth = 1; }
    if (n.root || n === hover || n === sel) {
      ctx.fillStyle = '#c9d1d9'; ctx.font = '12px ui-monospace, monospace';
      ctx.fillText(n.id, n.x + radius(n) + 4, n.y + 4);
    }
    ctx.globalAlpha = 1;
  }
}

function frame() { step(); draw(); requestAnimationFrame(frame); }
frame();

function at(mx, my) {
  let best = null, bd = 16*16;
  for (const n of N) {
    if (n.dim) continue;
    const dx = n.x - mx, dy = n.y - my, d2 = dx*dx + dy*dy;
    if (d2 < bd) { bd = d2; best = n; }
  }
  return best;
}
function pos(e) { const r = cv.getBoundingClientRect(); return [e.clientX - r.left, e.clientY - r.top]; }

cv.addEventListener('mousedown', e => { const [x,y] = pos(e); const n = at(x,y); if (n) { drag = n; select(n); } });
window.addEventListener('mousemove', e => {
  const [x,y] = pos(e);
  if (drag) { drag.x = x; drag.y = y; drag.vx = drag.vy = 0; }
  else { hover = at(x,y); cv.style.cursor = hover ? 'pointer' : 'grab'; }
});
window.addEventListener('mouseup', () => { drag = null; });

document.getElementById('find').addEventListener('input', e => {
  const q = e.target.value.trim().toLowerCase();
  for (const n of N) n.dim = !n.root && q !== '' && !n.id.toLowerCase().includes(q);
});

function esc(s) { return String(s).replace(/[&<>]/g, c => ({'&':'&amp;','<':'&lt;','>':'&gt;'}[c])); }

function row(k, v) { return '<div class="k">' + esc(k) + '</div><div class="v">' + esc(v) + '</div>'; }

function moduleBlock(m) {
  let out = '<h2>' + esc(m.module || 'module') + '</h2>';
  if (m.error) return out + '<div class="err">' + esc(m.error) + '</div>';
  if (m.data === undefined || m.data === null) return out + '<div class="hint">nothing to report</div>';
  let text;
  try { text = JSON.stringify(m.data, null, 2); } catch (e) { text = String(m.data); }
  if (!text || text === 'null' || text === '{}' || text === '[]') return out + '<div class="hint">nothing to report</div>';
  return out + '<pre class="out">' + esc(text) + '</pre>';
}

function select(n) {
  sel = n;
  const r = n.report || {};
  document.getElementById('pTitle').textContent = r.host || n.id;

  let html = n.reachable ? '<span class="badge up">reachable</span>'
                         : '<span class="badge down">no response</span>';

  let kv = '';
  if (n.root) kv += row('target', DATA.target) + row('subdomains', DATA.found) + row('live', DATA.live);
  if (r.addresses && r.addresses.length) kv += row('addresses', r.addresses.join(', '));
  if (r.last_seen) kv += row('last seen', String(r.last_seen).replace('T', ' ').replace(/(\.\d+)?Z$/, ' UTC'));
  if (kv) html += '<div class="kv">' + kv + '</div>';

  // The target is scanned like any other host, so the root carries module output too.
  const mods = r.modules || [];
  if (!mods.length) {
    html += n.root
      ? '<div class="hint" style="margin-top:14px">Pick a host in the graph to see its module output.</div>'
      : '<div class="hint">no module output</div>';
  }
  for (const m of mods) html += moduleBlock(m);
  document.getElementById('pBody').innerHTML = html;
}
if (root) select(root);
</script>
</body>
</html>
`
