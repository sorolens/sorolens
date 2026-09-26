// Classic (non-module) content-script entry point.
//
// Content scripts cannot use static `import`, but they can dynamically import
// a web-accessible module. This loader is the only code that has to be a plain
// script; everything else lives in ES modules shared with the popup and the
// service worker.

(() => {
  if (globalThis.__sorolensContentLoaded) return;
  globalThis.__sorolensContentLoaded = true;

  const api = globalThis.browser || globalThis.chrome;
  if (!api || !api.runtime || !api.runtime.getURL) {
    console.warn("[Sorolens] extension API unavailable; skipping injection");
    return;
  }

  import(api.runtime.getURL("src/content/content.js"))
    .then((module) => module.start())
    .catch((error) => {
      console.error("[Sorolens] failed to start content script", error);
    });
})();
