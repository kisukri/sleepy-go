let W = {};
try {
    W = JSON.parse((document.getElementById('sl-words') || {}).textContent || '{}');
} catch (e) {
    W = {};
}
export const lang = document.documentElement.lang || 'ja';

export function t(key, vars) {
    const s = W[key] ?? key;
    if (!vars) return s;
    return s.replace(/\{(\w+)\}/g, (m, k) => (k in vars ? String(vars[k]) : m));
}

// theme: html[data-theme] + localStorage, null = follow system
const dark = window.matchMedia ? window.matchMedia('(prefers-color-scheme: dark)') : null;
const isDark = () => {
    const v = document.documentElement.dataset.theme;
    return v ? v === 'dark' : !!(dark && dark.matches);
};
function paintMeta() {
    const explicit = !!document.documentElement.dataset.theme;
    const bg = getComputedStyle(document.documentElement).getPropertyValue('--sl-bg').trim();
    document.querySelectorAll('meta[name="theme-color"]').forEach((m) => {
        if (!m.dataset.orig) m.dataset.orig = m.getAttribute('content');
        m.setAttribute('content', explicit && bg ? bg : m.dataset.orig);
    });
}
export function bindTheme(btn) {
    if (!btn) return;
    const sync = () => {
        btn.setAttribute('aria-label', t(isDark() ? 'to_light' : 'to_dark'));
        btn.title = btn.getAttribute('aria-label');
        paintMeta();
    };
    btn.addEventListener('click', () => {
        const next = isDark() ? 'light' : 'dark';
        // back to "system" when the pick matches it
        const sys = dark && dark.matches ? 'dark' : 'light';
        if (next === sys) delete document.documentElement.dataset.theme;
        else document.documentElement.dataset.theme = next;
        try {
            if (next === sys) localStorage.removeItem('sl-theme');
            else localStorage.setItem('sl-theme', next);
        } catch (e) {}
        sync();
    });
    if (dark && dark.addEventListener) dark.addEventListener('change', sync);
    sync();
}
