// shared.js — auth, API helpers, navigation, utilities
// Loaded on all pages except the login page.

var token = localStorage.getItem('nvr_token');

// Redirect to login if no token (except on login page itself)
if (!token && !/^\/(index\.html)?$/.test(window.location.pathname)) {
  window.location.href = '/';
}

async function api(path, opts) {
  opts = opts || {};
  var headers = Object.assign({}, opts.headers || {});
  if (token) headers['Authorization'] = 'Bearer ' + token;
  if (opts.body && typeof opts.body === 'string' && !headers['Content-Type']) {
    headers['Content-Type'] = 'application/json';
  }
  var resp = await fetch(path, Object.assign({}, opts, { headers: headers }));
  if (resp.status === 401) { logout(); throw new Error('unauthorized'); }
  return resp;
}

function logout() {
  localStorage.removeItem('nvr_token');
  window.location.href = '/';
}

function renderNav(active) {
  var nav = document.getElementById('topnav');
  if (!nav) return;
  var links = [
    { href: '/monitor.html', label: 'Monitor' },
    { href: '/logs.html', label: 'Logs' },
    { href: '/cameras.html', label: 'Cameras' }
  ];
  nav.innerHTML = links.map(function(l) {
    var cls = l.label.toLowerCase() === active ? 'active' : '';
    return '<a href="' + l.href + '" class="' + cls + '">' + l.label + '</a>';
  }).join('');
}

function fmtTime(ms) {
  if (!ms) return '\u2014';
  return new Date(ms).toLocaleString();
}

function fmtDuration(ms) {
  if (!ms && ms !== 0) return '\u2014';
  if (ms < 1000) return ms + 'ms';
  var s = Math.round(ms / 1000);
  if (s < 60) return s + 's';
  var m = Math.floor(s / 60);
  return m + 'm ' + (s % 60) + 's';
}

function fmtRelative(ms) {
  if (!ms) return '';
  var diff = Date.now() - ms;
  if (diff < 60000) return Math.round(diff / 1000) + 's ago';
  if (diff < 3600000) return Math.round(diff / 60000) + 'm ago';
  if (diff < 86400000) return Math.round(diff / 3600000) + 'h ago';
  return Math.round(diff / 86400000) + 'd ago';
}

function escHtml(s) {
  var d = document.createElement('div');
  d.textContent = s;
  return d.innerHTML;
}

// SSE via fetch + ReadableStream (allows Authorization header)
async function connectSSE(url, onEvent, onError, onClose) {
  try {
    var resp = await fetch(url, { headers: { 'Authorization': 'Bearer ' + token } });
    if (resp.status === 401) { logout(); return; }
    if (!resp.ok) throw new Error('SSE ' + resp.status);
    var reader = resp.body.getReader();
    var decoder = new TextDecoder();
    var buf = '';
    while (true) {
      var result = await reader.read();
      if (result.done) break;
      buf += decoder.decode(result.value, { stream: true });
      var parts = buf.split('\n\n');
      buf = parts.pop();
      for (var i = 0; i < parts.length; i++) {
        var part = parts[i].trim();
        if (!part) continue;
        var evtType = '', data = '';
        var lines = part.split('\n');
        for (var j = 0; j < lines.length; j++) {
          if (lines[j].indexOf('event: ') === 0) evtType = lines[j].slice(7);
          else if (lines[j].indexOf('data: ') === 0) data += lines[j].slice(6);
          // ignore comments (:)
        }
        if (data) {
          try { onEvent(evtType, JSON.parse(data)); } catch(e) { /* skip bad JSON */ }
        }
      }
    }
  } catch (err) {
    if (onError) onError(err);
  }
  if (onClose) onClose();
}
