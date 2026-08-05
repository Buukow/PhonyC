(function () {
  'use strict';

  document.documentElement.classList.add('docs-js');

  function headingText(heading) {
    var copy = heading.cloneNode(true);
    Array.prototype.forEach.call(copy.querySelectorAll('.anchor-heading'), function (anchor) {
      anchor.remove();
    });
    return (copy.textContent || '').trim();
  }

  function buildPageToc() {
    var wrap = document.querySelector('.main-content-wrap');
    var content = document.querySelector('#main-content');
    var main = content && content.querySelector('main');
    if (!wrap || !content || !main || wrap.querySelector('.page-toc')) return;

    var headings = Array.prototype.slice.call(main.querySelectorAll('h2[id], h3[id]'))
      .filter(function (heading) { return headingText(heading); });
    if (!headings.length) return;

    var aside = document.createElement('aside');
    aside.className = 'page-toc';
    aside.setAttribute('aria-label', '本页目录');

    var inner = document.createElement('div');
    inner.className = 'page-toc-inner';

    var title = document.createElement('div');
    title.className = 'page-toc-title';
    title.textContent = '本页目录';

    var nav = document.createElement('nav');
    nav.className = 'page-toc-nav';

    headings.forEach(function (heading) {
      var link = document.createElement('a');
      link.className = 'page-toc-link page-toc-level-' + heading.tagName.slice(1);
      link.href = '#' + encodeURIComponent(heading.id);
      link.textContent = headingText(heading);
      link.dataset.target = heading.id;
      nav.appendChild(link);
    });

    inner.appendChild(title);
    inner.appendChild(nav);
    aside.appendChild(inner);
    wrap.appendChild(aside);
    document.documentElement.classList.add('has-page-toc');

    var links = Array.prototype.slice.call(nav.querySelectorAll('.page-toc-link'));

    function setActive(id) {
      links.forEach(function (link) {
        var active = link.dataset.target === id;
        link.classList.toggle('active', active);
        if (active) link.setAttribute('aria-current', 'location');
        else link.removeAttribute('aria-current');
      });
    }

    var ticking = false;

    function updateActiveHeading() {
      var marker = Math.max(96, window.innerHeight * 0.22);
      var current = headings[0];
      headings.forEach(function (heading) {
        if (heading.getBoundingClientRect().top <= marker) current = heading;
      });
      if (window.scrollY + window.innerHeight >= document.documentElement.scrollHeight - 2) {
        current = headings[headings.length - 1];
      }
      setActive(current.id);
      ticking = false;
    }

    function requestActiveHeadingUpdate() {
      if (ticking) return;
      ticking = true;
      window.requestAnimationFrame(updateActiveHeading);
    }

    window.addEventListener('scroll', requestActiveHeadingUpdate, { passive: true });
    window.addEventListener('resize', requestActiveHeadingUpdate, false);

    nav.addEventListener('click', function (event) {
      var link = event.target.closest && event.target.closest('.page-toc-link');
      if (link) setActive(link.dataset.target);
    });

    var hash = decodeURIComponent(window.location.hash.slice(1));
    var initial = links.some(function (link) { return link.dataset.target === hash; }) ? hash : headings[0].id;
    setActive(initial);
    requestActiveHeadingUpdate();
  }

  function init() {
    buildPageToc();
    window.requestAnimationFrame(function () {
      document.documentElement.classList.add('docs-ready');
    });
  }

  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', init, false);
  } else {
    init();
  }
}());
