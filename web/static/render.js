
import { t, lang } from './i18n.js';

export const DEFAULT_TEXT = {
    page: {
        title: t('title'),
        name: '{name}',
        device: '{device}',
        app: '{app}',
        updated: t('updated'),
        live: t('live'),
        connecting: t('connecting'),
        reconnecting: t('reconnecting'),
        footer: null,
        battery_tag: 'on',
        music_tag: 'on'
    },
    time: {
        just_now: t('just_now'),
        minutes: t('minutes'),
        hours: t('hours'),
        yesterday: t('yesterday'),
        days: t('days')
    }
};

export const DOT_COLORS = {
    green: t('c_green'),
    grey: t('c_grey'),
    amber: t('c_amber'),
    red: t('c_red'),
    blue: t('c_blue')
};

const LEGACY_COLOR = { awake: 'green', sleeping: 'grey', error: 'red' };

export function statusDefaults(status) {
    return {
        color: LEGACY_COLOR[status && status.color] || 'green',
        emoji: '',
        title: ['{status}'],
        desc: ['{desc}'],
        idle: [t('idle')]
    };
}

const isObj = (v) => v && typeof v === 'object' && !Array.isArray(v);

function clean(v) {
    if (Array.isArray(v)) {
        const lst = v.map((x) => String(x)).filter((x) => x.trim() !== '');
        return lst.length ? lst : undefined;
    }
    if (typeof v === 'string') return v.trim() === '' ? undefined : v;
    return v;
}

function mergeFlat(base, over) {
    const out = { ...base };
    if (isObj(over)) {
        for (const [k, v] of Object.entries(over)) {
            const c = clean(v);
            if (c !== undefined) out[k] = c;
        }
    }
    return out;
}

export function resolveText(overrides, status) {
    const o = isObj(overrides) ? overrides : {};
    const sid = status ? String(status.id) : '-1';
    return {
        page: mergeFlat(DEFAULT_TEXT.page, o.page),
        time: mergeFlat(DEFAULT_TEXT.time, o.time),
        status: mergeFlat(statusDefaults(status), isObj(o.status) ? o.status[sid] : null)
    };
}
export function fill(template, vars) {
    return String(template ?? '').replace(/\{(\w+)\}/g, (m, k) =>
        Object.prototype.hasOwnProperty.call(vars, k) && vars[k] !== undefined && vars[k] !== null ? String(vars[k]) : m
    );
}
const picks = new Map();
export function pick(list, key) {
    const lst = Array.isArray(list) ? list : [list];
    if (lst.length <= 1) return lst[0] ?? '';
    const cacheKey = `${key}|${lst.join('\u0000')}`;
    if (!picks.has(cacheKey)) picks.set(cacheKey, Math.floor(Math.random() * lst.length));
    return lst[picks.get(cacheKey)];
}
export function reshuffle() {
    picks.clear();
}
export function ago(ts, now, timeText) {
    if (!ts) return '';
    const sec = Math.max(0, now - ts * 1000) / 1000;
    if (sec < 60) return timeText.just_now;
    const min = Math.floor(sec / 60);
    if (min < 60) return fill(timeText.minutes, { n: min });
    const hr = Math.floor(min / 60);
    if (hr < 24) return fill(timeText.hours, { n: hr });
    const day = Math.floor(hr / 24);
    if (day === 1) return timeText.yesterday;
    return fill(timeText.days, { n: day });
}
// win: [♪x] [🔋82%⚡] title   ios sc: [77%]App
export function parseApp(raw) {
    let app = String(raw ?? '');
    let battery = '';
    let music = '';
    const fmtBattery = (num, rest) => {
        const n = Math.round(parseFloat(num));
        const charging = /⚡|充电|charging/i.test(rest || '') ? '⚡' : '';
        return Number.isFinite(n) ? `${n}%${charging}` : '';
    };

    app = app.replace(/[\[【(（]\s*(🔋|🪫|♪|🎵|🎶)\s*([^\]】)）]*)[\]】)）]/gu, (m, icon, body) => {
        if (icon === '♪' || icon === '🎵' || icon === '🎶') {
            music = body.trim() || t('playing');
        } else {
            const num = body.match(/(\d+(?:\.\d+)?)\s*%/);
            battery = num ? fmtBattery(num[1], body) : body.trim();
        }
        return ' ';
    });
    if (!battery) {
        app = app.replace(/[\[【(（]\s*(\d{1,3}(?:\.\d+)?)\s*%\s*(⚡|充电中?|charging)?\s*[\]】)）]/iu, (m, num, plug) => {
            battery = fmtBattery(num, plug);
            return ' ';
        });
    }
    if (!battery) {
        app = app.replace(/(🔋|🪫)\s*(\d+(?:\.\d+)?)\s*%\s*(⚡)?/u, (m, icon, num, plug) => {
            battery = fmtBattery(num, plug);
            return ' ';
        });
    }
    app = app.replace(/\s+/g, ' ').replace(/^[\s\-|·•,，]+|[\s\-|·•,，]+$/g, '').trim();
    return { app, battery, music };
}
function clockOf(now) {
    return new Date(now).toLocaleTimeString(lang, { hour: '2-digit', minute: '2-digit', hourCycle: 'h23' });
}

// pick an outline icon from the device name / id
export function deviceKind(d) {
    const s = `${d.id || ''} ${d.show_name || ''}`.toLowerCase();
    if (/ipad|tablet|\btab|pad\b|平板|タブレット/.test(s)) return 'tab';
    if (/iphone|phone|android|pixel|galaxy|xiaomi|redmi|huawei|honor|oppo|vivo|oneplus|手机|スマホ|携帯/.test(s)) return 'phone';
    return 'pc';
}

function slice(text, max) {
    if (!max || text.length <= max) return text;
    return text.slice(0, Math.max(1, max - 1)) + '…';
}

export function buildView({ data, meta, overrides, moreText = '', now = Date.now() }) {
    const status = data.status || { id: -1, name: '', desc: '', color: 'awake' };
    const text = resolveText(overrides, status);
    const name = (meta && meta.page && meta.page.name) || '';
    const slicelen = (meta && meta.status && meta.status.device_slice) || 0;
    const t = text.time;
    const all = Object.values(data.device || {});
    const using = all
        .filter((d) => d.using === true)
        .sort((a, b) => (b.last_updated || 0) - (a.last_updated || 0));
    const latest = all.reduce((acc, d) => (!acc || (d.last_updated || 0) > (acc.last_updated || 0) ? d : acc), null);
    const lastTs = latest ? latest.last_updated : data.last_updated;
    const first = using[0] ? parseApp(using[0].status) : { app: '', battery: '', music: '' };
    const base = {
        name,
        status: status.name || '',
        desc: String(status.desc || '').replace(/<[^>]*>/g, ''),
        count: using.length,
        battery: first.battery,
        music: first.music,
        last_ago: ago(lastTs, now, t),
        last_device: latest ? latest.show_name : '',
        updated_ago: ago(data.last_updated, now, t),
        time: clockOf(now)
    };
    base.device = using[0] ? using[0].show_name : '';
    base.app = first.app;
    base.ago = using[0] ? ago(using[0].last_updated, now, t) : '';
    const sid = String(status.id);
    const s = text.status;
    // the admin's order wins; unranked devices follow, newest first
    const rank = new Map((data.order || []).map((id, i) => [id, i]));
    const listed = [...using].sort((a, b) => (rank.get(a.id) ?? Infinity) - (rank.get(b.id) ?? Infinity) || 0);
    const devices = listed.map((d) => {
        const parsed = parseApp(d.status);
        const vars = {
            ...base,
            device: d.show_name || d.id,
            app: slice(parsed.app, slicelen),
            ago: ago(d.last_updated, now, t),
            battery: parsed.battery,
            music: parsed.music
        };
        return {
            id: d.id,
            kind: deviceKind(d),
            device: fill(text.page.device, vars),
            app: fill(text.page.app, vars),
            ago: vars.ago,
            battery: text.page.battery_tag === 'off' ? '' : parsed.battery,
            music: text.page.music_tag === 'off' ? '' : parsed.music
        };
    });
    return {
        vars: base,
        title: fill(text.page.title, base),
        name: fill(text.page.name, base),
        color: DOT_COLORS[s.color] ? s.color : 'green',
        emoji: s.emoji || '',
        statusTitle: fill(pick(s.title, `${sid}.title`), base),
        statusDesc: fill(pick(s.desc, `${sid}.desc`), base),
        devices,
        idle: devices.length ? '' : fill(pick(s.idle, `${sid}.idle`), base),
        updated: fill(text.page.updated, base),
        clock: clockOf(now),
        footer: text.page.footer ?? moreText ?? '',
        conn: {
            live: text.page.live,
            connecting: text.page.connecting,
            reconnecting: text.page.reconnecting
        }
    };
}
function el(tag, cls, txt) {
    const e = document.createElement(tag);
    if (cls) e.className = cls;
    if (txt !== undefined) e.textContent = txt;
    return e;
}
function setText(node, value) {
    if (node.textContent !== value) {
        node.textContent = value;
        node.classList.remove('sl-fade');
        void node.offsetWidth;
        node.classList.add('sl-fade');
    }
}
const SVG = {
    pc: '<rect x="3" y="4.5" width="18" height="12" rx="1.5"/><path d="M9 20h6M12 16.5V20"/>',
    tab: '<rect x="4.5" y="2.5" width="15" height="19" rx="2.2"/><path d="M11 18.5h2"/>',
    phone: '<rect x="7" y="2.5" width="10" height="19" rx="2.2"/><path d="M11 18.5h2"/>'
};
function icon(kind) {
    const span = el('span', 'sl-ico');
    span.setAttribute('aria-hidden', 'true');
    span.innerHTML = `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.4" stroke-linecap="round" stroke-linejoin="round">${SVG[kind] || SVG.pc}</svg>`;
    span.dataset.kind = kind;
    return span;
}
export function render(root, view, conn = { state: 'live' }) {
    if (!root._sl) {
        root.innerHTML = '';
        root.classList.add('sl-root');
        const r = {};
        r.name = el('h1', 'sl-name');
        r.head = el('div', 'sl-head');
        r.dot = el('span', 'sl-dot');
        r.dot.setAttribute('aria-hidden', 'true');
        r.emoji = el('span', 'sl-emoji');
        r.title = el('p', 'sl-status');
        r.head.append(r.dot, r.emoji, r.title);
        r.desc = el('p', 'sl-desc');
        r.list = el('ul', 'sl-list');
        r.idle = el('p', 'sl-idle');
        const intro = el('div', 'sl-intro');
        intro.append(r.name, r.head, r.desc);
        const aside = el('div', 'sl-aside');
        aside.append(r.list, r.idle);
        const body = el('div', 'sl-body');
        body.append(intro, aside);
        r.foot = el('footer', 'sl-foot');
        r.clock = el('span', 'sl-clock');
        r.time = el('span', 'sl-time');
        r.clock.append(r.time);
        r.updated = el('span', 'sl-updated');
        r.conn = el('span', 'sl-conn');
        const right = el('span', 'sl-foot-r');
        right.append(r.updated, r.conn);
        r.extra = el('div', 'sl-extra');
        const row = el('div', 'sl-foot-row');
        row.append(r.clock, right);
        r.foot.append(row, r.extra);
        root.append(body, r.foot);
        root._sl = r;
    }
    const r = root._sl;
    setText(r.name, view.name);
    r.dot.dataset.color = view.color;
    r.emoji.textContent = view.emoji;
    r.emoji.hidden = !view.emoji;
    setText(r.title, view.statusTitle);
    setText(r.desc, view.statusDesc);
    r.desc.hidden = !view.statusDesc;
    const old = new Map([...r.list.children].map((li) => [li.dataset.id, li]));
    const frag = [];
    for (const d of view.devices) {
        let li = old.get(d.id);
        if (!li || li.firstChild.dataset.kind !== d.kind) {
            li = el('li', 'sl-item sl-fade');
            li.dataset.id = d.id;
            li.append(icon(d.kind), el('span', 'sl-app'), el('span', 'sl-ago'), el('span', 'sl-meta'));
        }
        old.delete(d.id);
        const [, app, ag, meta] = li.children;
        setText(app, d.app || d.device);
        app.title = d.app;
        ag.textContent = d.ago;
        const bits = [d.app ? d.device : ''];
        if (d.battery) bits.push(d.battery);
        if (d.music) bits.push(`♪ ${d.music}`);
        setText(meta, bits.filter(Boolean).join(' · '));
        meta.hidden = !meta.textContent;
        frag.push(li);
    }
    r.list.replaceChildren(...frag);
    r.list.hidden = !view.devices.length;
    setText(r.idle, view.idle);
    r.idle.hidden = !view.idle;
    r.time.textContent = view.clock || '';
    r.updated.textContent = view.updated;
    const state = conn.state || 'live';
    r.conn.dataset.state = state;
    r.conn.textContent = fill(view.conn[state] || '', { n: conn.n ?? '' });
    if (r.extra._html !== view.footer) {
        r.extra.innerHTML = view.footer || '';
        r.extra._html = view.footer;
    }
    r.extra.hidden = !view.footer;
}
