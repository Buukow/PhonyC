(function () {
  'use strict';

  var storageKey = 'phonyg-docs:expanded-navigation';

  function getItems() {
    return Array.prototype.slice.call(document.querySelectorAll('#site-nav .nav-list-expander'))
      .map(function (button) {
        var link = button.parentNode && button.parentNode.querySelector(':scope > .nav-list-link');
        // just-the-docs removes href from the current page link. A missing href
        // on a parent node therefore means that the current pathname is its key.
        var key = link && (link.getAttribute('href') || window.location.pathname);
        return { button: button, item: button.parentNode, key: key };
      })
      .filter(function (entry) { return entry.key; });
  }

  function readState() {
    try {
      var value = window.localStorage.getItem(storageKey);
      var parsed = value ? JSON.parse(value) : {};
      return parsed && typeof parsed === 'object' ? parsed : {};
    } catch (_) {
      return {};
    }
  }

  function writeState() {
    var state = {};
    getItems().forEach(function (entry) {
      state[entry.key] = entry.item.classList.contains('active');
    });
    try {
      window.localStorage.setItem(storageKey, JSON.stringify(state));
    } catch (_) {
      // Storage may be disabled; navigation still works without persistence.
    }
  }

  function restoreState() {
    var state = readState();
    getItems().forEach(function (entry) {
      if (state[entry.key] === true) {
        entry.item.classList.add('active');
        entry.button.setAttribute('aria-expanded', 'true');
      }
    });
  }

  function init() {
    // The theme registers its ready callback before this custom include. Restore
    // after activateNav() has run so the current page remains active as well.
    window.setTimeout(restoreState, 0);

    document.addEventListener('click', function (event) {
      var button = event.target.closest && event.target.closest('#site-nav .nav-list-expander');
      if (!button) return;

      // just-the-docs toggles the item in its earlier document listener.
      // The theme listener runs first, so this synchronous write captures the
      // final state even when the next action immediately navigates away.
      writeState();
      window.setTimeout(writeState, 0);
    }, false);
  }

  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', init, false);
  } else {
    init();
  }
}());
