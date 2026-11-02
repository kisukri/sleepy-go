import { parseApp, ago, DEFAULT_TEXT, statusDefaults } from './render.js';
import { t, bindTheme } from './i18n.js';

const $ = (id) => document.getElementById(id);
const statusBox = $('ad-status');
const devList = $('ad-devices');
const devEmpty = $('ad-dev-empty');
const clearBtn = $('ad-clear');
const privateBox = $('ad-private');
const toastBox = $('ad-toast');

let statusList = [];
let textOverrides = {};
let panel = null;

let toastTimer = null;
export function toast(msg, isError = false) {
    toastBox.textContent = msg;
    toastBox.classList.toggle('error', isError);
    toastBox.hidden = false;
    clearTimeout(toastTimer);
    toastTimer = setTimeout(() => (toastBox.hidden = true), isError ? 4000 : 2000);
}
window.slToast = toast;

async function api(url, write = false) {
    const resp = await fetch(url, { method: write ? 'POST' : 'GET', credentials: 'same-origin', cache: 'no-store' });
    if (resp.status === 401) {
        location.href = '/panel/login';
        throw new Error(t('expired'));
    }
    const data = await resp.json();
    if (!data.success) throw new Error(data.message || data.details || `HTTP ${resp.status}`);
    return data;
}

function armed(btn, label, confirmLabel, action) {
    let timer = null;
    btn.addEventListener('click', async () => {
        if (!btn.classList.contains('armed')) {
            btn.classList.add('armed');
            btn.textContent = confirmLabel;
            timer = setTimeout(() => {
                btn.classList.remove('armed');
                btn.textContent = label;
            }, 4000);
            return;
        }
        clearTimeout(timer);
        btn.classList.remove('armed');
        btn.textContent = label;
        await action();
    });
}

function dotColor(status) {
    const o = ((textOverrides.status || {})[String(status.id)] || {}).color;
    return o || statusDefaults(status).color;
}

function drawStatus() {
    if (!panel) return;
    const cur = panel.status ? panel.status.id : -1;
    statusBox.replaceChildren(...statusList.map((s) => {
        const b = document.createElement('button');
        b.type = 'button';
        b.setAttribute('aria-pressed', String(s.id === cur));
        const dot = document.createElement('span');
        dot.className = 'sl-dot';
        dot.dataset.color = dotColor(s);
        b.append(dot, document.createTextNode(s.name));
        b.addEventListener('click', async () => {
            if (s.id === cur) return;
            statusBox.querySelectorAll('button').forEach((x) => (x.disabled = true));
            try {
                await api(`/api/status/set?status=${s.id}`, true);
                panel.status = s;
                toast(t('switched', { name: s.name }));
                window.dispatchEvent(new CustomEvent('sleepy:status-changed', { detail: s.id }));
            } catch (e) {
                toast(t('switch_failed', { err: e.message }), true);
            }
            drawStatus();
        });
        return b;
    }));
}

const openMenus = new Set();

function drawDevices() {
    if (!panel) return;
    if (devList.querySelector('.armed')) {
        setTimeout(drawDevices, 4200);
        return;
    }
    const now = Date.now();
    const devices = Object.entries(panel.devices || {}).map(([id, d]) => ({ id, ...d }));
    devices.sort((a, b) => (b.using === true) - (a.using === true) || (b.last_updated || 0) - (a.last_updated || 0));

    devList.replaceChildren(...devices.map((d) => {
        const li = document.createElement('li');
        li.className = 'ad-dev';
        const row = document.createElement('div');
        row.className = 'ad-dev-row';

        const dot = document.createElement('span');
        dot.className = 'sl-dot';
        dot.dataset.color = d.using ? 'green' : 'grey';

        const main = document.createElement('div');
        main.className = 'ad-dev-main';
        const name = document.createElement('div');
        name.className = 'ad-dev-name';
        name.textContent = d.show_name || d.id;
        const sub = document.createElement('div');
        sub.className = 'ad-dev-sub';
        const parsed = parseApp(d.status);
        const what = d.using ? (parsed.app || t('in_use')) : t('not_in_use');
        sub.textContent = `${what} · ${ago(d.last_updated, now, DEFAULT_TEXT.time)}`;
        sub.title = d.status || '';
        main.append(name, sub);

        const more = document.createElement('button');
        more.type = 'button';
        more.className = 'ad-more';
        more.textContent = '⋯';
        more.setAttribute('aria-label', t('more_actions', { name: d.show_name || d.id }));
        more.setAttribute('aria-expanded', String(openMenus.has(d.id)));

        row.append(dot, main, more);
        li.append(row);

        if (openMenus.has(d.id)) {
            const act = document.createElement('div');
            act.className = 'ad-dev-actions';
            const idInfo = document.createElement('span');
            idInfo.append(t('device_id'));
            const code = document.createElement('code');
            code.textContent = d.id;
            idInfo.append(code);
            const del = document.createElement('button');
            del.type = 'button';
            del.className = 'ad-danger-link';
            del.textContent = t('dev_delete');
            armed(del, t('dev_delete'), t('confirm_delete'), async () => {
                try {
                    await api(`/api/device/remove?id=${encodeURIComponent(d.id)}`, true);
                    delete panel.devices[d.id];
                    openMenus.delete(d.id);
                    toast(t('deleted'));
                } catch (e) {
                    toast(t('delete_failed', { err: e.message }), true);
                }
                drawDevices();
            });
            act.append(idInfo, del);
            li.append(act);
        }

        more.addEventListener('click', () => {
            openMenus.has(d.id) ? openMenus.delete(d.id) : openMenus.add(d.id);
            drawDevices();
        });
        return li;
    }));

    devEmpty.hidden = devices.length > 0;
    clearBtn.hidden = devices.length === 0;
}

armed(clearBtn, t('dev_clear'), t('confirm_clear'), async () => {
    try {
        await api('/api/device/clear', true);
        panel.devices = {};
        toast(t('cleared'));
    } catch (e) {
        toast(t('clear_failed', { err: e.message }), true);
    }
    drawDevices();
});

privateBox.addEventListener('change', async () => {
    const want = privateBox.checked;
    try {
        await api(`/api/device/private?private=${want}`, true);
        panel.private_mode = want;
        toast(t(want ? 'private_on' : 'private_off'));
    } catch (e) {
        privateBox.checked = !want;
        toast(t('switch_failed', { err: e.message }), true);
    }
});

async function reload() {
    panel = await api('/panel/data');
    privateBox.checked = !!panel.private_mode;
    drawStatus();
    drawDevices();
}

let reloadTimer = null;
function reloadSoon() {
    clearTimeout(reloadTimer);
    reloadTimer = setTimeout(() => reload().catch((e) => console.warn(e)), 300);
}

window.addEventListener('sleepy:text-saved', (e) => {
    textOverrides = e.detail || {};
    drawStatus();
});

bindTheme(document.getElementById('sl-theme'));

async function init() {
    const [list, text] = await Promise.all([
        api('/api/status/list'),
        fetch('/plugin/custom_text/text').then((r) => (r.ok ? r.json() : { text: {} })).catch(() => ({ text: {} }))
    ]);
    statusList = list.status_list || [];
    textOverrides = text.text || {};
    await reload();

    if (typeof EventSource !== 'undefined') {
        const es = new EventSource('/api/status/events');
        es.addEventListener('update', reloadSoon);
    }
    document.addEventListener('visibilitychange', () => {
        if (document.visibilityState === 'visible') reloadSoon();
    });
    setInterval(drawDevices, 30000);
}

init().catch((e) => {
    statusBox.replaceChildren(Object.assign(document.createElement('p'), { className: 'ad-mute', textContent: t('load_failed', { err: e.message }) }));
});
