import { buildView, render } from './render.js';
import { bindTheme } from './i18n.js';

const root = document.getElementById('sl-root');
const overrides = readJson('sleepy-text') || {};
const moreTpl = document.getElementById('sl-more-text');
const moreText = moreTpl ? moreTpl.innerHTML.trim() : '';

let meta = null;
let lastData = null;
let conn = { state: 'connecting' };

function readJson(id) {
    const node = document.getElementById(id);
    if (!node) return null;
    try {
        return JSON.parse(node.textContent);
    } catch (e) {
        console.warn('[paper] bad text json', e);
        return null;
    }
}

function draw() {
    if (!lastData || !meta) {
        const c = root.querySelector('.sl-conn');
        if (c) c.dataset.state = conn.state;
        return;
    }
    const view = buildView({ data: lastData, meta, overrides, moreText });
    render(root, view, conn);
    document.title = view.title;
}

function setConn(state, n) {
    conn = { state, n };
    draw();
}

function onData(data) {
    if (!data || !data.success) {
        console.warn('[paper] server error', data);
        return;
    }
    lastData = data;
    draw();
}

let es = null;
let attempts = 0;
let retryTimer = null;
let countdownTimer = null;
let watchdog = null;
let lastMsg = Date.now();
const MAX_DELAY = 30000;

function clearTimers() {
    clearTimeout(retryTimer);
    clearInterval(countdownTimer);
}

function connect() {
    clearTimers();
    if (es) es.close();
    setConn('connecting');
    es = new EventSource('/api/status/events');

    es.onopen = () => {
        attempts = 0;
        lastMsg = Date.now();
    };
    es.addEventListener('update', (e) => {
        lastMsg = Date.now();
        try {
            onData(JSON.parse(e.data));
        } catch (err) {
            console.warn('[paper] bad update', err);
        }
        if (conn.state !== 'live') setConn('live');
    });
    es.addEventListener('heartbeat', () => {
        lastMsg = Date.now();
        if (conn.state !== 'live') setConn('live');
    });
    es.onerror = () => scheduleReconnect();
}

function scheduleReconnect() {
    if (es) {
        es.close();
        es = null;
    }
    clearTimers();
    const delay = Math.min(1000 * 2 ** attempts, MAX_DELAY);
    attempts++;
    let left = Math.ceil(delay / 1000);
    setConn('reconnecting', left);
    countdownTimer = setInterval(() => {
        left--;
        if (left > 0) setConn('reconnecting', left);
    }, 1000);
    retryTimer = setTimeout(connect, delay);
}

function startWatchdog() {
    clearInterval(watchdog);
    watchdog = setInterval(() => {
        if (es && Date.now() - lastMsg > 120000) {
            console.warn('[paper] stream idle, reconnecting');
            scheduleReconnect();
        }
    }, 10000);
}

async function poll() {
    const interval = (meta && meta.status && meta.status.refresh_interval) || 5000;
    for (;;) {
        if (document.visibilityState === 'visible') {
            try {
                const resp = await fetch('/api/status/query');
                onData(await resp.json());
                if (conn.state !== 'live') setConn('live');
            } catch (e) {
                setConn('reconnecting', Math.round(interval / 1000));
            }
        }
        await new Promise((r) => setTimeout(r, interval));
    }
}

async function start() {
    try {
        meta = await (await fetch('/api/meta')).json();
    } catch (e) {
        console.warn('[paper] meta failed', e);
        meta = { page: {}, status: {} };
    }
    if (typeof EventSource === 'undefined') {
        poll();
        return;
    }
    connect();
    startWatchdog();

    document.addEventListener('visibilitychange', () => {
        if (document.visibilityState === 'visible') {
            attempts = 0;
            connect();
        }
    });
    window.addEventListener('pagehide', () => es && es.close());
    window.addEventListener('pageshow', (e) => {
        if (e.persisted) connect();
    });
}

setTimeout(() => root.querySelectorAll('.sl-pending').forEach((n) => n.classList.remove('sl-pending')), 2000);

setInterval(draw, 15000);
bindTheme(document.getElementById('sl-theme'));

start();
