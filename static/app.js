// ===== Language Toggle =====
// The server renders the page language into <html lang> and remembers it in
// a cookie, so switching only needs a reload with ?lang=
document.getElementById('lang-toggle').addEventListener('click', () => {
    const newLang = document.documentElement.lang === 'th' ? 'en' : 'th';

    // On a language-specific post, switch to the equivalent post
    const postMatch = window.location.pathname.match(/^\/posts\/(th|en)-(.+)$/);
    const url = new URL(window.location.href);
    if (postMatch) {
        url.pathname = '/posts/' + newLang + '-' + postMatch[2];
    }
    url.searchParams.set('lang', newLang);
    // Use replace to avoid adding to browser history
    window.location.replace(url.toString());
});

// ===== Theme Toggle =====
document.getElementById('theme-toggle').addEventListener('click', () => {
    const isDark = document.documentElement.getAttribute('data-theme') !== 'dark';
    const theme = isDark ? 'dark' : 'light';
    document.documentElement.setAttribute('data-theme', theme);
    try {
        localStorage.setItem('theme', theme);
    } catch (e) {
        // Theme still applies for this page view
    }
});

// ===== Disclaimer Popup Logic =====
const disclaimerModal = document.getElementById('disclaimer-modal');
const acceptBtn = document.getElementById('accept-disclaimer');

// Show popup on each new browser session
if (!sessionStorage.getItem('disclaimerAccepted')) {
    disclaimerModal.classList.add('open');
    document.body.style.overflow = 'hidden';
}

acceptBtn.addEventListener('click', () => {
    sessionStorage.setItem('disclaimerAccepted', 'true');
    disclaimerModal.classList.remove('open');
    document.body.style.overflow = '';
});
