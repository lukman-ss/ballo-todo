'use strict';

// Delete confirmation — intercept all .delete-form submits
document.addEventListener('submit', function (e) {
  if (e.target.classList.contains('delete-form')) {
    if (!confirm('Hapus todo ini?')) {
      e.preventDefault();
    }
  }
});

// Prevent double submit — disable [data-disable-on-submit] buttons on submit
document.addEventListener('submit', function (e) {
  var btn = e.target.querySelector('[data-disable-on-submit]');
  if (btn) {
    // Delay so the form value is still submitted before disabling
    setTimeout(function () { btn.disabled = true; }, 0);
  }
});

// Auto-submit sort select when changed (already has onchange inline as fallback;
// this is the JS-only path that works without the inline handler)
document.addEventListener('change', function (e) {
  if (e.target.dataset.autoSubmit !== undefined) {
    var form = e.target.form || document.getElementById(e.target.getAttribute('form'));
    if (form) form.submit();
  }
});
