import {
    DEFAULT_TEXT, DOT_COLORS, statusDefaults, buildView, render, reshuffle, fill
} from '../static/render.js';
import { t } from '../static/i18n.js';

const root = document.getElementById('ct-root');

const VARS = Object.fromEntries(
    ['name', 'status', 'desc', 'device', 'app', 'ago', 'count', 'battery', 'music', 'last_ago', 'last_device', 'updated_ago', 'time', 'n']
        .map((k) => [k, t('v_' + k)])
);
const LABEL_TO_KEY = Object.fromEntries(Object.entries(VARS).map(([k, v]) => [v, k]));
const toDisplay = (s) => String(s ?? '').replace(/\{(\w+)\}/g, (m, k) => (VARS[k] ? `【${VARS[k]}】` : m));
const toStore = (s) => String(s ?? '').replace(/【([^】]+)】/g, (m, l) => (LABEL_TO_KEY[l] ? `{${LABEL_TO_KEY[l]}}` : m));

const STATUS_VARS = ['name', 'device', 'app', 'ago', 'last_ago', 'last_device'];
const MORE_VARS = ['status', 'desc', 'count', 'battery', 'music', 'updated_ago', 'time'];
const DEVICE_VARS = ['device', 'app', 'ago', 'battery', 'music'];

const state = {
    saved: {},
    draft: {},
    statusList: [],
    data: null,
    meta: null,
    moreText: '',
    sid: 0,
    scene: 'using'
};

const clone = (o) => JSON.parse(JSON.stringify(o || {}));

function h(tag, attrs = {}, ...kids) {
    const e = document.createElement(tag);
    for (const [k, v] of Object.entries(attrs)) {
        if (k === 'class') e.className = v;
        else if (k.startsWith('on')) e.addEventListener(k.slice(2), v);
        else if (v !== false && v !== undefined && v !== null) e.setAttribute(k, v === true ? '' : v);
    }
    for (const kid of kids.flat()) if (kid !== null && kid !== undefined && kid !== false) e.append(kid);
    return e;
}

function toast(msg, isError) {
    if (window.slToast) return window.slToast(msg, isError);
    let box = document.getElementById('ct-toast');
    if (!box) document.body.append((box = h('div', { id: 'ct-toast', class: 'ct-toast' })));
    box.textContent = msg;
    box.classList.toggle('error', !!isError);
    box.hidden = false;
    setTimeout(() => (box.hidden = true), 2000);
}

const currentStatus = () =>
    state.statusList.find((s) => s.id === state.sid) || state.statusList[0] || { id: 0, name: '', desc: '', color: 'awake' };

function getVal(t, g, k) {
    if (g === 'status') return ((t.status || {})[String(state.sid)] || {})[k];
    return (t[g] || {})[k];
}

function setVal(t, g, k, v) {
    const empty = v === undefined || v === '' || (Array.isArray(v) && !v.length);
    let box;
    if (g === 'status') {
        t.status = t.status || {};
        box = t.status[String(state.sid)] = t.status[String(state.sid)] || {};
    } else {
        box = t[g] = t[g] || {};
    }
    if (empty) delete box[k];
    else box[k] = v;
    if (g === 'status' && !Object.keys(box).length) delete t.status[String(state.sid)];
    if (t.status && !Object.keys(t.status).length) delete t.status;
    if (g !== 'status' && !Object.keys(box).length) delete t[g];
}

function defaultTemplate(g, k) {
    if (g === 'page') return k === 'footer' ? state.moreText : DEFAULT_TEXT.page[k];
    if (g === 'time') return DEFAULT_TEXT.time[k];
    return statusDefaults(currentStatus())[k];
}

let previewRoot = null;
let lastView = null;

function previewData() {
    const data = clone(state.data);
    data.status = currentStatus();
    const now = Date.now() / 1000;
    const devices = data.device || {};
    if (state.scene === 'idle') {
        for (const d of Object.values(devices)) d.using = false;
    } else if (!Object.values(devices).some((d) => d.using === true)) {
        devices.__sample = { id: '__sample', show_name: t('e_sample_dev'), status: `[🔋80%] ${t('e_sample_app')}`, using: true, last_updated: now - 120 };
    }
    data.device = devices;
    return data;
}

const EDITABLE = [
    ['.sl-name', 'name', t('e_name')],
    ['.sl-head', 'title', t('e_title')],
    ['.sl-desc', 'desc', t('e_desc')],
    ['.sl-list', 'device', t('e_device')],
    ['.sl-idle', 'idle', t('e_idle')],
    ['.sl-foot-r', 'foot', t('e_foot')],
    ['.sl-extra', 'footer', t('e_footer')]
];

function drawPreview(overrides = state.draft) {
    if (!previewRoot || !state.data) return;
    lastView = buildView({ data: previewData(), meta: state.meta, overrides, moreText: state.moreText });
    render(previewRoot, lastView, { state: 'live' });
    for (const [sel, id, label] of EDITABLE) {
        const node = previewRoot.querySelector(sel);
        if (!node || node.dataset.edit) continue;
        node.dataset.edit = id;
        node.tabIndex = 0;
        node.setAttribute('role', 'button');
        node.setAttribute('aria-label', t('e_edit', { label }));
    }
}

function sheetDef(id) {
    const s = currentStatus();
    const L = (k) => ({ g: 'status', k, type: 'lines', vars: STATUS_VARS });
    switch (id) {
        case 'name':
            return { title: t('e_name'), fields: [{ g: 'page', k: 'name', type: 'line', vars: ['name', 'status', 'time'] }] };
        case 'title':
            return {
                title: t('s_title', { status: s.name }),
                note: t('s_random'),
                fields: [
                    L('title'),
                    { g: 'status', k: 'color', type: 'color', label: t('s_dot') },
                    { g: 'status', k: 'emoji', type: 'line', label: t('s_emoji') }
                ]
            };
        case 'desc':
            return { title: t('s_desc', { status: s.name }), note: t('s_random'), fields: [L('desc')] };
        case 'idle':
            return { title: t('s_idle', { status: s.name }), note: t('s_random'), fields: [L('idle')] };
        case 'device':
            return {
                title: t('e_device'),
                note: t('s_device_note'),
                fields: [
                    { g: 'page', k: 'device', type: 'line', label: t('f_device'), vars: DEVICE_VARS },
                    { g: 'page', k: 'app', type: 'line', label: t('f_app'), vars: DEVICE_VARS },
                    { g: 'page', k: 'battery_tag', type: 'toggle', label: t('f_battery'), hint: t('f_under_app') },
                    { g: 'page', k: 'music_tag', type: 'toggle', label: t('f_music'), hint: t('f_under_app') }
                ]
            };
        case 'foot':
            return {
                title: t('e_foot'),
                fields: [
                    { g: 'page', k: 'updated', type: 'line', label: t('f_updated'), vars: ['updated_ago', 'time'] },
                    { g: 'page', k: 'live', type: 'line', label: t('f_live') },
                    { g: 'page', k: 'connecting', type: 'line', label: t('f_connecting') },
                    { g: 'page', k: 'reconnecting', type: 'line', label: t('f_reconnect'), vars: ['n'] }
                ]
            };
        case 'footer':
            return {
                title: t('e_footer'),
                note: t('s_footer_note'),
                fields: [{ g: 'page', k: 'footer', type: 'textarea' }]
            };
        case 'pagetitle':
            return { title: t('s_pagetitle'), fields: [{ g: 'page', k: 'title', type: 'line', vars: ['name', 'status', 'device', 'app'] }] };
        case 'time':
            return {
                title: t('s_time'),
                note: t('s_time_note'),
                fields: [
                    { g: 'time', k: 'just_now', type: 'line', label: t('t_just_now') },
                    { g: 'time', k: 'minutes', type: 'line', label: t('t_minutes'), vars: ['n'] },
                    { g: 'time', k: 'hours', type: 'line', label: t('t_hours'), vars: ['n'] },
                    { g: 'time', k: 'yesterday', type: 'line', label: t('t_yesterday') },
                    { g: 'time', k: 'days', type: 'line', label: t('t_days'), vars: ['n'] }
                ]
            };
    }
    return null;
}

let mask = null;
let lastInput = null;

function closeSheet() {
    if (!mask) return;
    mask.remove();
    mask = null;
    document.body.classList.remove('ct-lock');
    state.draft = clone(state.saved);
    drawPreview();
}

function placeholderOf(g, k, idx = 0) {
    let d = defaultTemplate(g, k);
    if (Array.isArray(d)) d = d[idx] ?? d[0];
    if (!d) return k === 'emoji' ? t('x_emoji_ph') : t('x_empty');
    if (g === 'page' && k === 'footer') return d;
    const vars = lastView ? { ...lastView.vars, n: 3 } : {};
    return fill(d, vars);
}

function onChange() {
    drawPreview(state.draft);
}

function chips(keys, field) {
    if (!keys || !keys.length) return null;
    const box = h('div', { class: 'ct-chips' }, h('span', {}, t('x_insert')));
    const addChip = (k) => box.append(h('button', {
            type: 'button',
            onmousedown: (e) => e.preventDefault(),
            onclick: () => {
                const inp = lastInput && field.contains(lastInput) ? lastInput : field.querySelector('input, textarea');
                if (!inp) return;
                const token = `【${VARS[k]}】`;
                const a = inp.selectionStart ?? inp.value.length;
                const b = inp.selectionEnd ?? inp.value.length;
                inp.value = inp.value.slice(0, a) + token + inp.value.slice(b);
                inp.focus();
                inp.setSelectionRange(a + token.length, a + token.length);
                inp.dispatchEvent(new Event('input', { bubbles: true }));
            }
        }, `+ ${VARS[k]}`));
    keys.forEach(addChip);
    if (keys === STATUS_VARS) {
        const more = h('button', {
            type: 'button', class: 'ct-chip-more',
            onclick: () => { more.remove(); MORE_VARS.forEach(addChip); }
        }, t('x_more_vars'));
        box.append(more);
    }
    return box;
}

function lineInput(value, placeholder, onInput, tag = 'input') {
    const inp = tag === 'textarea' ? h('textarea', { rows: 3 }) : h('input', { type: 'text' });
    inp.value = value;
    inp.placeholder = placeholder;
    inp.addEventListener('input', onInput);
    inp.addEventListener('focus', () => (lastInput = inp));
    return inp;
}

function buildField(f) {
    const wrap = h('div', { class: 'ct-field' });
    if (f.label) wrap.append(h('div', { class: 'ct-flabel' }, f.label));

    if (f.type === 'toggle') {
        const box = h('input', { type: 'checkbox', role: 'switch' });
        box.checked = getVal(state.draft, f.g, f.k) !== 'off';
        box.addEventListener('change', () => {
            setVal(state.draft, f.g, f.k, box.checked ? '' : 'off');
            onChange();
        });
        wrap.className = 'ct-field ct-toggle';
        wrap.replaceChildren(h('label', {},
            h('span', {}, h('b', {}, f.label), f.hint ? h('small', {}, f.hint) : null),
            box, h('i', { 'aria-hidden': 'true' })));
        return wrap;
    }

    if (f.type === 'color') {
        const cur = getVal(state.draft, f.g, f.k) || statusDefaults(currentStatus()).color;
        const row = h('div', { class: 'ct-colors' });
        for (const [c, name] of Object.entries(DOT_COLORS)) {
            row.append(h('button', {
                type: 'button', class: 'ct-color' + (c === cur ? ' on' : ''), 'aria-pressed': String(c === cur),
                onclick: () => {
                    setVal(state.draft, f.g, f.k, c === statusDefaults(currentStatus()).color ? '' : c);
                    row.querySelectorAll('.ct-color').forEach((b) => {
                        const on = b === row.querySelector(`[data-c="${c}"]`);
                        b.classList.toggle('on', on);
                        b.setAttribute('aria-pressed', String(on));
                    });
                    onChange();
                },
                'data-c': c
            }, h('i', { class: 'sl-dot', 'data-color': c }), name));
        }
        wrap.append(row);
        return wrap;
    }

    if (f.type === 'lines') {
        const list = h('div', { class: 'ct-lines' });
        const collect = () => {
            const vals = [...list.querySelectorAll('input')].map((i) => toStore(i.value)).filter((v) => v.trim());
            setVal(state.draft, f.g, f.k, vals);
            onChange();
        };
        const addRow = (value = '', idx = 0) => {
            const inp = lineInput(toDisplay(value), placeholderOf(f.g, f.k, idx), collect);
            const del = h('button', {
                type: 'button', class: 'ct-del', 'aria-label': t('x_del_line'),
                onclick: () => {
                    if (list.children.length > 1) row.remove();
                    else inp.value = '';
                    collect();
                }
            }, '✕');
            const row = h('div', { class: 'ct-line' }, inp, del);
            list.append(row);
            return inp;
        };
        const vals = getVal(state.draft, f.g, f.k) || [];
        if (vals.length) vals.forEach((v, i) => addRow(v, i));
        else addRow('', 0);
        const add = h('button', {
            type: 'button', class: 'ct-add',
            onclick: () => addRow('', list.children.length).focus()
        }, t('x_add_line'));
        wrap.append(list, add);
        const c = chips(f.vars, wrap);
        if (c) wrap.append(c);
        return wrap;
    }

    const raw = getVal(state.draft, f.g, f.k) || '';
    const inp = lineInput(
        f.type === 'textarea' ? raw : toDisplay(raw),
        placeholderOf(f.g, f.k),
        () => {
            setVal(state.draft, f.g, f.k, f.type === 'textarea' ? inp.value : toStore(inp.value));
            onChange();
        },
        f.type === 'textarea' ? 'textarea' : 'input'
    );
    wrap.append(inp);
    const c = chips(f.vars, wrap);
    if (c) wrap.append(c);
    return wrap;
}

function openSheet(id) {
    const def = sheetDef(id);
    if (!def) return;
    if (id === 'idle' && state.scene !== 'idle') setScene('idle');
    if (id === 'device' && state.scene !== 'using') setScene('using');
    closeSheet();
    state.draft = clone(state.saved);
    lastInput = null;

    const body = h('div', { class: 'ct-sheet-body' }, def.fields.map(buildField));
    const errLine = h('p', { class: 'ct-err', hidden: true });

    const resetBtn = h('button', {
        type: 'button', class: 'ct-btn ct-btn-ghost',
        onclick: () => {
            for (const f of def.fields) setVal(state.draft, f.g, f.k, '');
            body.replaceChildren(...def.fields.map(buildField));
            onChange();
        }
    }, t('x_reset'));

    const doneBtn = h('button', {
        type: 'button', class: 'ct-btn ct-btn-dark',
        onclick: async () => {
            doneBtn.disabled = true;
            errLine.hidden = true;
            try {
                const resp = await fetch('/plugin/custom_text/text', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    credentials: 'same-origin',
                    body: JSON.stringify({ text: state.draft })
                });
                const data = await resp.json();
                if (!data.success) throw new Error(data.message || resp.status);
                state.saved = clone(data.text);
                closeSheet();
                toast(t('x_saved'));
                window.dispatchEvent(new CustomEvent('sleepy:text-saved', { detail: clone(data.text) }));
            } catch (e) {
                errLine.textContent = t('x_save_failed', { err: e.message });
                errLine.hidden = false;
                doneBtn.disabled = false;
            }
        }
    }, t('x_done'));

    const sheet = h('div', { class: 'ct-sheet', role: 'dialog', 'aria-modal': 'true', 'aria-label': def.title },
        h('div', { class: 'ct-sheet-head' },
            h('b', {}, def.title),
            h('button', { type: 'button', class: 'ct-x', 'aria-label': t('x_close'), onclick: closeSheet }, '✕')),
        def.note ? h('p', { class: 'ct-note' }, def.note) : null,
        body,
        errLine,
        h('div', { class: 'ct-sheet-foot' }, resetBtn, h('span', { class: 'ct-grow' }),
            h('button', { type: 'button', class: 'ct-btn ct-btn-ghost', onclick: closeSheet }, t('x_cancel')), doneBtn)
    );
    mask = h('div', { class: 'ct-mask', onclick: (e) => { if (e.target === mask) closeSheet(); } }, sheet);
    document.body.append(mask);
    document.body.classList.add('ct-lock');
    const first = sheet.querySelector('input, textarea');
    if (first && window.matchMedia('(hover: hover)').matches) first.focus();
}

document.addEventListener('keydown', (e) => {
    if (e.key === 'Escape' && mask) closeSheet();
});

let statusSeg = null;
let sceneSeg = null;

function seg(items, current, onPick) {
    const box = h('div', { class: 'ct-seg', role: 'group' });
    for (const [val, label] of items) {
        box.append(h('button', {
            type: 'button', 'data-v': String(val), 'aria-pressed': String(String(val) === String(current)),
            onclick: () => onPick(val)
        }, label));
    }
    return box;
}
function markSeg(box, current) {
    box.querySelectorAll('button').forEach((b) => b.setAttribute('aria-pressed', String(b.dataset.v === String(current))));
}
function setScene(v) {
    state.scene = v;
    markSeg(sceneSeg, v);
    drawPreview();
}

async function loadData() {
    let r = await fetch('/admin/data', { credentials: 'same-origin', cache: 'no-store' });
    if (r.ok) {
        const p = await r.json();
        return { success: true, status: p.status, device: p.devices, last_updated: p.last_updated };
    }
    r = await fetch('/api/status/query');
    return r.json();
}

async function init() {
    const [txt, list, data, meta] = await Promise.all([
        fetch('/plugin/custom_text/text').then((r) => r.json()),
        fetch('/api/status/list').then((r) => r.json()),
        loadData(),
        fetch('/api/meta').then((r) => r.json())
    ]);
    state.saved = clone(txt.text);
    state.draft = clone(txt.text);
    state.moreText = txt.more_text || '';
    state.statusList = list.status_list || [];
    state.data = data;
    state.meta = meta;
    state.sid = data.status ? data.status.id : 0;
    if (!state.statusList.some((s) => s.id === state.sid)) state.sid = state.statusList.length ? state.statusList[0].id : 0;

    statusSeg = seg(state.statusList.map((s) => [s.id, s.name]), state.sid, (v) => {
        state.sid = Number(v);
        markSeg(statusSeg, v);
        drawPreview();
    });
    sceneSeg = seg([['using', t('x_using')], ['idle', t('x_nothing')]], state.scene, setScene);

    window.addEventListener('sleepy:status-changed', (e) => {
        if (mask || !state.statusList.some((s) => s.id === e.detail)) return;
        state.sid = e.detail;
        markSeg(statusSeg, e.detail);
        drawPreview();
    });

    previewRoot = h('div');
    const preview = h('div', { class: 'ct-preview' }, h('div', { class: 'sl-page' }, previewRoot));
    preview.addEventListener('click', (e) => {
        const hit = e.target.closest('[data-edit]');
        if (hit && !e.target.closest('a')) openSheet(hit.dataset.edit);
    });
    preview.addEventListener('keydown', (e) => {
        const hit = e.target.closest('[data-edit]');
        if (hit && (e.key === 'Enter' || e.key === ' ')) {
            e.preventDefault();
            openSheet(hit.dataset.edit);
        }
    });

    const more = h('div', { class: 'ct-more' },
        h('span', {}, t('x_more')),
        [['pagetitle', t('s_pagetitle')], ['time', t('s_time')], ['footer', t('e_footer')], ['foot', t('x_live')]].map(([id, label]) =>
            h('button', { type: 'button', onclick: () => openSheet(id) }, label)),
        h('button', { type: 'button', onclick: () => { reshuffle(); drawPreview(); } }, t('x_shuffle'))
    );

    root.replaceChildren(
        h('div', { class: 'ct-bar' }, statusSeg, sceneSeg),
        preview,
        h('p', { class: 'ct-tip' }, t('x_tip')),
        more
    );
    drawPreview();
}

init().catch((e) => {
    root.textContent = t('x_load_failed', { err: e.message });
    console.error(e);
});
