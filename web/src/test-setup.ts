// jsdom 26 has no native modal/fullscreen top layer. nwsapi 2.2.27
// recursively delegates these exact selectors back to Element.matches.
// Keep all other selectors real; actual popup/top-layer behavior is covered
// by Playwright rather than simulated by this unit-test environment.
const nativeMatches = Element.prototype.matches;
Element.prototype.matches = function(selector: string) {
  if (selector === ":modal" || selector === ":fullscreen") return false;
  return nativeMatches.call(this, selector);
};

// jsdom does not evaluate media queries. Browser tests own responsive layout;
// unit tests can replace this EventTarget-based fixture for media transitions.
if (!window.matchMedia) {
  Object.defineProperty(window, "matchMedia", {
    configurable: true,
    writable: true,
    value: (query: string): MediaQueryList => Object.assign(new EventTarget(), {
      media: query,
      matches: false,
      onchange: null,
      addListener() {},
      removeListener() {},
    }) as MediaQueryList,
  });
}

// jsdom 26 does not provide PointerEvent, which Base UI uses for keyboard activation.
if (!window.PointerEvent) {
  class PointerEvent extends MouseEvent {
    readonly pointerId: number;
    readonly pointerType: string;
    readonly isPrimary: boolean;
    constructor(type: string, options: PointerEventInit = {}) {
      super(type, options);
      this.pointerId = options.pointerId ?? 0;
      this.pointerType = options.pointerType ?? "mouse";
      this.isPrimary = options.isPrimary ?? true;
    }
  }
  Object.defineProperty(window, "PointerEvent", { value: PointerEvent, configurable: true });
}
