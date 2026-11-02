const $ = (id) => document.getElementById(id);
let W = {};
try {
    W = JSON.parse($('sl-words').textContent);
} catch (e) {}
const t = (k, n) => String(W[k] || k).replace('{n}', n);
const form = document.querySelector('.login-form');
const input = $('secret');
const error = $('error-message');
const btn = document.querySelector('.login-btn');

function showError(msg) {
    error.textContent = msg;
    error.style.display = 'block';
}

async function login() {
    const secret = input.value;
    if (!secret) return showError(t('login_empty'));
    btn.disabled = true;
    error.style.display = 'none';
    try {
        const resp = await fetch('/admin/auth', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ secret })
        });
        if (resp.ok) {
            location.href = '/admin';
            return;
        }
        showError(resp.status === 401 ? t('login_wrong') : t('login_failed', resp.status));
    } catch (e) {
        showError(t('login_offline'));
    }
    btn.disabled = false;
}

async function init() {
    try {
        const resp = await fetch('/admin/verify', { credentials: 'include', cache: 'no-cache' });
        if (resp.ok) {
            location.href = '/admin';
            return;
        }
    } catch (e) {
    }
    $('loading-message').style.display = 'none';
    form.style.display = 'block';
    input.focus();
    input.addEventListener('keydown', (e) => {
        if (e.key === 'Enter') login();
    });
    btn.addEventListener('click', login);
}

init();
