/**
 * @license
 * Copyright 2019 Google LLC
 * SPDX-License-Identifier: BSD-3-Clause
 */
const L = globalThis, W = L.ShadowRoot && (L.ShadyCSS === void 0 || L.ShadyCSS.nativeShadow) && "adoptedStyleSheets" in Document.prototype && "replace" in CSSStyleSheet.prototype, Z = Symbol(), Y = /* @__PURE__ */ new WeakMap();
let pt = class {
  constructor(t, e, i) {
    if (this._$cssResult$ = !0, i !== Z)
      throw Error("CSSResult is not constructable. Use `unsafeCSS` or `css` instead.");
    this.cssText = t, this.t = e;
  }
  get styleSheet() {
    let t = this.o;
    const e = this.t;
    if (W && t === void 0) {
      const i = e !== void 0 && e.length === 1;
      i && (t = Y.get(e)), t === void 0 && ((this.o = t = new CSSStyleSheet()).replaceSync(this.cssText), i && Y.set(e, t));
    }
    return t;
  }
  toString() {
    return this.cssText;
  }
};
const Ut = (r) => new pt(typeof r == "string" ? r : r + "", void 0, Z), Pt = (r, ...t) => {
  const e = r.length === 1 ? r[0] : t.reduce((i, s, o) => i + ((n) => {
    if (n._$cssResult$ === !0)
      return n.cssText;
    if (typeof n == "number")
      return n;
    throw Error("Value passed to 'css' function must be a 'css' function result: " + n + ". Use 'unsafeCSS' to pass non-literal values, but take care to ensure page security.");
  })(s) + r[o + 1], r[0]);
  return new pt(e, r, Z);
}, Rt = (r, t) => {
  if (W)
    r.adoptedStyleSheets = t.map((e) => e instanceof CSSStyleSheet ? e : e.styleSheet);
  else
    for (const e of t) {
      const i = document.createElement("style"), s = L.litNonce;
      s !== void 0 && i.setAttribute("nonce", s), i.textContent = e.cssText, r.appendChild(i);
    }
}, X = W ? (r) => r : (r) => r instanceof CSSStyleSheet ? ((t) => {
  let e = "";
  for (const i of t.cssRules)
    e += i.cssText;
  return Ut(e);
})(r) : r;
/**
 * @license
 * Copyright 2017 Google LLC
 * SPDX-License-Identifier: BSD-3-Clause
 */
const { is: Tt, defineProperty: Nt, getOwnPropertyDescriptor: Mt, getOwnPropertyNames: Ht, getOwnPropertySymbols: Ot, getPrototypeOf: Lt } = Object, m = globalThis, tt = m.trustedTypes, Bt = tt ? tt.emptyScript : "", I = m.reactiveElementPolyfillSupport, P = (r, t) => r, J = { toAttribute(r, t) {
  switch (t) {
    case Boolean:
      r = r ? Bt : null;
      break;
    case Object:
    case Array:
      r = r == null ? r : JSON.stringify(r);
  }
  return r;
}, fromAttribute(r, t) {
  let e = r;
  switch (t) {
    case Boolean:
      e = r !== null;
      break;
    case Number:
      e = r === null ? null : Number(r);
      break;
    case Object:
    case Array:
      try {
        e = JSON.parse(r);
      } catch {
        e = null;
      }
  }
  return e;
} }, ft = (r, t) => !Tt(r, t), et = { attribute: !0, type: String, converter: J, reflect: !1, hasChanged: ft };
Symbol.metadata ?? (Symbol.metadata = Symbol("metadata")), m.litPropertyMetadata ?? (m.litPropertyMetadata = /* @__PURE__ */ new WeakMap());
class C extends HTMLElement {
  static addInitializer(t) {
    this._$Ei(), (this.l ?? (this.l = [])).push(t);
  }
  static get observedAttributes() {
    return this.finalize(), this._$Eh && [...this._$Eh.keys()];
  }
  static createProperty(t, e = et) {
    if (e.state && (e.attribute = !1), this._$Ei(), this.elementProperties.set(t, e), !e.noAccessor) {
      const i = Symbol(), s = this.getPropertyDescriptor(t, i, e);
      s !== void 0 && Nt(this.prototype, t, s);
    }
  }
  static getPropertyDescriptor(t, e, i) {
    const { get: s, set: o } = Mt(this.prototype, t) ?? { get() {
      return this[e];
    }, set(n) {
      this[e] = n;
    } };
    return { get() {
      return s == null ? void 0 : s.call(this);
    }, set(n) {
      const h = s == null ? void 0 : s.call(this);
      o.call(this, n), this.requestUpdate(t, h, i);
    }, configurable: !0, enumerable: !0 };
  }
  static getPropertyOptions(t) {
    return this.elementProperties.get(t) ?? et;
  }
  static _$Ei() {
    if (this.hasOwnProperty(P("elementProperties")))
      return;
    const t = Lt(this);
    t.finalize(), t.l !== void 0 && (this.l = [...t.l]), this.elementProperties = new Map(t.elementProperties);
  }
  static finalize() {
    if (this.hasOwnProperty(P("finalized")))
      return;
    if (this.finalized = !0, this._$Ei(), this.hasOwnProperty(P("properties"))) {
      const e = this.properties, i = [...Ht(e), ...Ot(e)];
      for (const s of i)
        this.createProperty(s, e[s]);
    }
    const t = this[Symbol.metadata];
    if (t !== null) {
      const e = litPropertyMetadata.get(t);
      if (e !== void 0)
        for (const [i, s] of e)
          this.elementProperties.set(i, s);
    }
    this._$Eh = /* @__PURE__ */ new Map();
    for (const [e, i] of this.elementProperties) {
      const s = this._$Eu(e, i);
      s !== void 0 && this._$Eh.set(s, e);
    }
    this.elementStyles = this.finalizeStyles(this.styles);
  }
  static finalizeStyles(t) {
    const e = [];
    if (Array.isArray(t)) {
      const i = new Set(t.flat(1 / 0).reverse());
      for (const s of i)
        e.unshift(X(s));
    } else
      t !== void 0 && e.push(X(t));
    return e;
  }
  static _$Eu(t, e) {
    const i = e.attribute;
    return i === !1 ? void 0 : typeof i == "string" ? i : typeof t == "string" ? t.toLowerCase() : void 0;
  }
  constructor() {
    super(), this._$Ep = void 0, this.isUpdatePending = !1, this.hasUpdated = !1, this._$Em = null, this._$Ev();
  }
  _$Ev() {
    var t;
    this._$ES = new Promise((e) => this.enableUpdating = e), this._$AL = /* @__PURE__ */ new Map(), this._$E_(), this.requestUpdate(), (t = this.constructor.l) == null || t.forEach((e) => e(this));
  }
  addController(t) {
    var e;
    (this._$EO ?? (this._$EO = /* @__PURE__ */ new Set())).add(t), this.renderRoot !== void 0 && this.isConnected && ((e = t.hostConnected) == null || e.call(t));
  }
  removeController(t) {
    var e;
    (e = this._$EO) == null || e.delete(t);
  }
  _$E_() {
    const t = /* @__PURE__ */ new Map(), e = this.constructor.elementProperties;
    for (const i of e.keys())
      this.hasOwnProperty(i) && (t.set(i, this[i]), delete this[i]);
    t.size > 0 && (this._$Ep = t);
  }
  createRenderRoot() {
    const t = this.shadowRoot ?? this.attachShadow(this.constructor.shadowRootOptions);
    return Rt(t, this.constructor.elementStyles), t;
  }
  connectedCallback() {
    var t;
    this.renderRoot ?? (this.renderRoot = this.createRenderRoot()), this.enableUpdating(!0), (t = this._$EO) == null || t.forEach((e) => {
      var i;
      return (i = e.hostConnected) == null ? void 0 : i.call(e);
    });
  }
  enableUpdating(t) {
  }
  disconnectedCallback() {
    var t;
    (t = this._$EO) == null || t.forEach((e) => {
      var i;
      return (i = e.hostDisconnected) == null ? void 0 : i.call(e);
    });
  }
  attributeChangedCallback(t, e, i) {
    this._$AK(t, i);
  }
  _$EC(t, e) {
    var o;
    const i = this.constructor.elementProperties.get(t), s = this.constructor._$Eu(t, i);
    if (s !== void 0 && i.reflect === !0) {
      const n = (((o = i.converter) == null ? void 0 : o.toAttribute) !== void 0 ? i.converter : J).toAttribute(e, i.type);
      this._$Em = t, n == null ? this.removeAttribute(s) : this.setAttribute(s, n), this._$Em = null;
    }
  }
  _$AK(t, e) {
    var o;
    const i = this.constructor, s = i._$Eh.get(t);
    if (s !== void 0 && this._$Em !== s) {
      const n = i.getPropertyOptions(s), h = typeof n.converter == "function" ? { fromAttribute: n.converter } : ((o = n.converter) == null ? void 0 : o.fromAttribute) !== void 0 ? n.converter : J;
      this._$Em = s, this[s] = h.fromAttribute(e, n.type), this._$Em = null;
    }
  }
  requestUpdate(t, e, i) {
    if (t !== void 0) {
      if (i ?? (i = this.constructor.getPropertyOptions(t)), !(i.hasChanged ?? ft)(this[t], e))
        return;
      this.P(t, e, i);
    }
    this.isUpdatePending === !1 && (this._$ES = this._$ET());
  }
  P(t, e, i) {
    this._$AL.has(t) || this._$AL.set(t, e), i.reflect === !0 && this._$Em !== t && (this._$Ej ?? (this._$Ej = /* @__PURE__ */ new Set())).add(t);
  }
  async _$ET() {
    this.isUpdatePending = !0;
    try {
      await this._$ES;
    } catch (e) {
      Promise.reject(e);
    }
    const t = this.scheduleUpdate();
    return t != null && await t, !this.isUpdatePending;
  }
  scheduleUpdate() {
    return this.performUpdate();
  }
  performUpdate() {
    var i;
    if (!this.isUpdatePending)
      return;
    if (!this.hasUpdated) {
      if (this.renderRoot ?? (this.renderRoot = this.createRenderRoot()), this._$Ep) {
        for (const [o, n] of this._$Ep)
          this[o] = n;
        this._$Ep = void 0;
      }
      const s = this.constructor.elementProperties;
      if (s.size > 0)
        for (const [o, n] of s)
          n.wrapped !== !0 || this._$AL.has(o) || this[o] === void 0 || this.P(o, this[o], n);
    }
    let t = !1;
    const e = this._$AL;
    try {
      t = this.shouldUpdate(e), t ? (this.willUpdate(e), (i = this._$EO) == null || i.forEach((s) => {
        var o;
        return (o = s.hostUpdate) == null ? void 0 : o.call(s);
      }), this.update(e)) : this._$EU();
    } catch (s) {
      throw t = !1, this._$EU(), s;
    }
    t && this._$AE(e);
  }
  willUpdate(t) {
  }
  _$AE(t) {
    var e;
    (e = this._$EO) == null || e.forEach((i) => {
      var s;
      return (s = i.hostUpdated) == null ? void 0 : s.call(i);
    }), this.hasUpdated || (this.hasUpdated = !0, this.firstUpdated(t)), this.updated(t);
  }
  _$EU() {
    this._$AL = /* @__PURE__ */ new Map(), this.isUpdatePending = !1;
  }
  get updateComplete() {
    return this.getUpdateComplete();
  }
  getUpdateComplete() {
    return this._$ES;
  }
  shouldUpdate(t) {
    return !0;
  }
  update(t) {
    this._$Ej && (this._$Ej = this._$Ej.forEach((e) => this._$EC(e, this[e]))), this._$EU();
  }
  updated(t) {
  }
  firstUpdated(t) {
  }
}
C.elementStyles = [], C.shadowRootOptions = { mode: "open" }, C[P("elementProperties")] = /* @__PURE__ */ new Map(), C[P("finalized")] = /* @__PURE__ */ new Map(), I == null || I({ ReactiveElement: C }), (m.reactiveElementVersions ?? (m.reactiveElementVersions = [])).push("2.0.4");
/**
 * @license
 * Copyright 2017 Google LLC
 * SPDX-License-Identifier: BSD-3-Clause
 */
const R = globalThis, B = R.trustedTypes, st = B ? B.createPolicy("lit-html", { createHTML: (r) => r }) : void 0, G = "$lit$", g = `lit$${(Math.random() + "").slice(9)}$`, K = "?" + g, jt = `<${K}>`, S = document, N = () => S.createComment(""), M = (r) => r === null || typeof r != "object" && typeof r != "function", $t = Array.isArray, _t = (r) => $t(r) || typeof (r == null ? void 0 : r[Symbol.iterator]) == "function", D = `[ 	
\f\r]`, k = /<(?:(!--|\/[^a-zA-Z])|(\/?[a-zA-Z][^>\s]*)|(\/?$))/g, it = /-->/g, rt = />/g, A = RegExp(`>|${D}(?:([^\\s"'>=/]+)(${D}*=${D}*(?:[^ 	
\f\r"'\`<>=]|("|')|))|$)`, "g"), nt = /'/g, ot = /"/g, gt = /^(?:script|style|textarea|title)$/i, It = (r) => (t, ...e) => ({ _$litType$: r, strings: t, values: e }), y = It(1), v = Symbol.for("lit-noChange"), f = Symbol.for("lit-nothing"), lt = /* @__PURE__ */ new WeakMap(), E = S.createTreeWalker(S, 129);
function mt(r, t) {
  if (!Array.isArray(r) || !r.hasOwnProperty("raw"))
    throw Error("invalid template strings array");
  return st !== void 0 ? st.createHTML(t) : t;
}
const vt = (r, t) => {
  const e = r.length - 1, i = [];
  let s, o = t === 2 ? "<svg>" : "", n = k;
  for (let h = 0; h < e; h++) {
    const l = r[h];
    let c, p, a = -1, u = 0;
    for (; u < l.length && (n.lastIndex = u, p = n.exec(l), p !== null); )
      u = n.lastIndex, n === k ? p[1] === "!--" ? n = it : p[1] !== void 0 ? n = rt : p[2] !== void 0 ? (gt.test(p[2]) && (s = RegExp("</" + p[2], "g")), n = A) : p[3] !== void 0 && (n = A) : n === A ? p[0] === ">" ? (n = s ?? k, a = -1) : p[1] === void 0 ? a = -2 : (a = n.lastIndex - p[2].length, c = p[1], n = p[3] === void 0 ? A : p[3] === '"' ? ot : nt) : n === ot || n === nt ? n = A : n === it || n === rt ? n = k : (n = A, s = void 0);
    const d = n === A && r[h + 1].startsWith("/>") ? " " : "";
    o += n === k ? l + jt : a >= 0 ? (i.push(c), l.slice(0, a) + G + l.slice(a) + g + d) : l + g + (a === -2 ? h : d);
  }
  return [mt(r, o + (r[e] || "<?>") + (t === 2 ? "</svg>" : "")), i];
};
class H {
  constructor({ strings: t, _$litType$: e }, i) {
    let s;
    this.parts = [];
    let o = 0, n = 0;
    const h = t.length - 1, l = this.parts, [c, p] = vt(t, e);
    if (this.el = H.createElement(c, i), E.currentNode = this.el.content, e === 2) {
      const a = this.el.content.firstChild;
      a.replaceWith(...a.childNodes);
    }
    for (; (s = E.nextNode()) !== null && l.length < h; ) {
      if (s.nodeType === 1) {
        if (s.hasAttributes())
          for (const a of s.getAttributeNames())
            if (a.endsWith(G)) {
              const u = p[n++], d = s.getAttribute(a).split(g), $ = /([.?@])?(.*)/.exec(u);
              l.push({ type: 1, index: o, name: $[2], strings: d, ctor: $[1] === "." ? yt : $[1] === "?" ? bt : $[1] === "@" ? Et : O }), s.removeAttribute(a);
            } else
              a.startsWith(g) && (l.push({ type: 6, index: o }), s.removeAttribute(a));
        if (gt.test(s.tagName)) {
          const a = s.textContent.split(g), u = a.length - 1;
          if (u > 0) {
            s.textContent = B ? B.emptyScript : "";
            for (let d = 0; d < u; d++)
              s.append(a[d], N()), E.nextNode(), l.push({ type: 2, index: ++o });
            s.append(a[u], N());
          }
        }
      } else if (s.nodeType === 8)
        if (s.data === K)
          l.push({ type: 2, index: o });
        else {
          let a = -1;
          for (; (a = s.data.indexOf(g, a + 1)) !== -1; )
            l.push({ type: 7, index: o }), a += g.length - 1;
        }
      o++;
    }
  }
  static createElement(t, e) {
    const i = S.createElement("template");
    return i.innerHTML = t, i;
  }
}
function w(r, t, e = r, i) {
  var n, h;
  if (t === v)
    return t;
  let s = i !== void 0 ? (n = e._$Co) == null ? void 0 : n[i] : e._$Cl;
  const o = M(t) ? void 0 : t._$litDirective$;
  return (s == null ? void 0 : s.constructor) !== o && ((h = s == null ? void 0 : s._$AO) == null || h.call(s, !1), o === void 0 ? s = void 0 : (s = new o(r), s._$AT(r, e, i)), i !== void 0 ? (e._$Co ?? (e._$Co = []))[i] = s : e._$Cl = s), s !== void 0 && (t = w(r, s._$AS(r, t.values), s, i)), t;
}
class At {
  constructor(t, e) {
    this._$AV = [], this._$AN = void 0, this._$AD = t, this._$AM = e;
  }
  get parentNode() {
    return this._$AM.parentNode;
  }
  get _$AU() {
    return this._$AM._$AU;
  }
  u(t) {
    const { el: { content: e }, parts: i } = this._$AD, s = ((t == null ? void 0 : t.creationScope) ?? S).importNode(e, !0);
    E.currentNode = s;
    let o = E.nextNode(), n = 0, h = 0, l = i[0];
    for (; l !== void 0; ) {
      if (n === l.index) {
        let c;
        l.type === 2 ? c = new x(o, o.nextSibling, this, t) : l.type === 1 ? c = new l.ctor(o, l.name, l.strings, this, t) : l.type === 6 && (c = new St(o, this, t)), this._$AV.push(c), l = i[++h];
      }
      n !== (l == null ? void 0 : l.index) && (o = E.nextNode(), n++);
    }
    return E.currentNode = S, s;
  }
  p(t) {
    let e = 0;
    for (const i of this._$AV)
      i !== void 0 && (i.strings !== void 0 ? (i._$AI(t, i, e), e += i.strings.length - 2) : i._$AI(t[e])), e++;
  }
}
class x {
  get _$AU() {
    var t;
    return ((t = this._$AM) == null ? void 0 : t._$AU) ?? this._$Cv;
  }
  constructor(t, e, i, s) {
    this.type = 2, this._$AH = f, this._$AN = void 0, this._$AA = t, this._$AB = e, this._$AM = i, this.options = s, this._$Cv = (s == null ? void 0 : s.isConnected) ?? !0;
  }
  get parentNode() {
    let t = this._$AA.parentNode;
    const e = this._$AM;
    return e !== void 0 && (t == null ? void 0 : t.nodeType) === 11 && (t = e.parentNode), t;
  }
  get startNode() {
    return this._$AA;
  }
  get endNode() {
    return this._$AB;
  }
  _$AI(t, e = this) {
    t = w(this, t, e), M(t) ? t === f || t == null || t === "" ? (this._$AH !== f && this._$AR(), this._$AH = f) : t !== this._$AH && t !== v && this._(t) : t._$litType$ !== void 0 ? this.$(t) : t.nodeType !== void 0 ? this.T(t) : _t(t) ? this.k(t) : this._(t);
  }
  S(t) {
    return this._$AA.parentNode.insertBefore(t, this._$AB);
  }
  T(t) {
    this._$AH !== t && (this._$AR(), this._$AH = this.S(t));
  }
  _(t) {
    this._$AH !== f && M(this._$AH) ? this._$AA.nextSibling.data = t : this.T(S.createTextNode(t)), this._$AH = t;
  }
  $(t) {
    var o;
    const { values: e, _$litType$: i } = t, s = typeof i == "number" ? this._$AC(t) : (i.el === void 0 && (i.el = H.createElement(mt(i.h, i.h[0]), this.options)), i);
    if (((o = this._$AH) == null ? void 0 : o._$AD) === s)
      this._$AH.p(e);
    else {
      const n = new At(s, this), h = n.u(this.options);
      n.p(e), this.T(h), this._$AH = n;
    }
  }
  _$AC(t) {
    let e = lt.get(t.strings);
    return e === void 0 && lt.set(t.strings, e = new H(t)), e;
  }
  k(t) {
    $t(this._$AH) || (this._$AH = [], this._$AR());
    const e = this._$AH;
    let i, s = 0;
    for (const o of t)
      s === e.length ? e.push(i = new x(this.S(N()), this.S(N()), this, this.options)) : i = e[s], i._$AI(o), s++;
    s < e.length && (this._$AR(i && i._$AB.nextSibling, s), e.length = s);
  }
  _$AR(t = this._$AA.nextSibling, e) {
    var i;
    for ((i = this._$AP) == null ? void 0 : i.call(this, !1, !0, e); t && t !== this._$AB; ) {
      const s = t.nextSibling;
      t.remove(), t = s;
    }
  }
  setConnected(t) {
    var e;
    this._$AM === void 0 && (this._$Cv = t, (e = this._$AP) == null || e.call(this, t));
  }
}
class O {
  get tagName() {
    return this.element.tagName;
  }
  get _$AU() {
    return this._$AM._$AU;
  }
  constructor(t, e, i, s, o) {
    this.type = 1, this._$AH = f, this._$AN = void 0, this.element = t, this.name = e, this._$AM = s, this.options = o, i.length > 2 || i[0] !== "" || i[1] !== "" ? (this._$AH = Array(i.length - 1).fill(new String()), this.strings = i) : this._$AH = f;
  }
  _$AI(t, e = this, i, s) {
    const o = this.strings;
    let n = !1;
    if (o === void 0)
      t = w(this, t, e, 0), n = !M(t) || t !== this._$AH && t !== v, n && (this._$AH = t);
    else {
      const h = t;
      let l, c;
      for (t = o[0], l = 0; l < o.length - 1; l++)
        c = w(this, h[i + l], e, l), c === v && (c = this._$AH[l]), n || (n = !M(c) || c !== this._$AH[l]), c === f ? t = f : t !== f && (t += (c ?? "") + o[l + 1]), this._$AH[l] = c;
    }
    n && !s && this.j(t);
  }
  j(t) {
    t === f ? this.element.removeAttribute(this.name) : this.element.setAttribute(this.name, t ?? "");
  }
}
class yt extends O {
  constructor() {
    super(...arguments), this.type = 3;
  }
  j(t) {
    this.element[this.name] = t === f ? void 0 : t;
  }
}
class bt extends O {
  constructor() {
    super(...arguments), this.type = 4;
  }
  j(t) {
    this.element.toggleAttribute(this.name, !!t && t !== f);
  }
}
class Et extends O {
  constructor(t, e, i, s, o) {
    super(t, e, i, s, o), this.type = 5;
  }
  _$AI(t, e = this) {
    if ((t = w(this, t, e, 0) ?? f) === v)
      return;
    const i = this._$AH, s = t === f && i !== f || t.capture !== i.capture || t.once !== i.once || t.passive !== i.passive, o = t !== f && (i === f || s);
    s && this.element.removeEventListener(this.name, this, i), o && this.element.addEventListener(this.name, this, t), this._$AH = t;
  }
  handleEvent(t) {
    var e;
    typeof this._$AH == "function" ? this._$AH.call(((e = this.options) == null ? void 0 : e.host) ?? this.element, t) : this._$AH.handleEvent(t);
  }
}
class St {
  constructor(t, e, i) {
    this.element = t, this.type = 6, this._$AN = void 0, this._$AM = e, this.options = i;
  }
  get _$AU() {
    return this._$AM._$AU;
  }
  _$AI(t) {
    w(this, t);
  }
}
const Dt = { P: G, A: g, C: K, M: 1, L: vt, R: At, D: _t, V: w, I: x, H: O, N: bt, U: Et, B: yt, F: St }, z = R.litHtmlPolyfillSupport;
z == null || z(H, x), (R.litHtmlVersions ?? (R.litHtmlVersions = [])).push("3.1.2");
const zt = (r, t, e) => {
  const i = (e == null ? void 0 : e.renderBefore) ?? t;
  let s = i._$litPart$;
  if (s === void 0) {
    const o = (e == null ? void 0 : e.renderBefore) ?? null;
    i._$litPart$ = s = new x(t.insertBefore(N(), o), o, void 0, e ?? {});
  }
  return s._$AI(r), s;
};
/**
 * @license
 * Copyright 2017 Google LLC
 * SPDX-License-Identifier: BSD-3-Clause
 */
let T = class extends C {
  constructor() {
    super(...arguments), this.renderOptions = { host: this }, this._$Do = void 0;
  }
  createRenderRoot() {
    var e;
    const t = super.createRenderRoot();
    return (e = this.renderOptions).renderBefore ?? (e.renderBefore = t.firstChild), t;
  }
  update(t) {
    const e = this.render();
    this.hasUpdated || (this.renderOptions.isConnected = this.isConnected), super.update(t), this._$Do = zt(e, this.renderRoot, this.renderOptions);
  }
  connectedCallback() {
    var t;
    super.connectedCallback(), (t = this._$Do) == null || t.setConnected(!0);
  }
  disconnectedCallback() {
    var t;
    super.disconnectedCallback(), (t = this._$Do) == null || t.setConnected(!1);
  }
  render() {
    return v;
  }
};
var ut;
T._$litElement$ = !0, T.finalized = !0, (ut = globalThis.litElementHydrateSupport) == null || ut.call(globalThis, { LitElement: T });
const F = globalThis.litElementPolyfillSupport;
F == null || F({ LitElement: T });
(globalThis.litElementVersions ?? (globalThis.litElementVersions = [])).push("4.0.4");
/**
 * @license
 * Copyright 2017 Google LLC
 * SPDX-License-Identifier: BSD-3-Clause
 */
const wt = { ATTRIBUTE: 1, CHILD: 2, PROPERTY: 3, BOOLEAN_ATTRIBUTE: 4, EVENT: 5, ELEMENT: 6 }, Ct = (r) => (...t) => ({ _$litDirective$: r, values: t });
class xt {
  constructor(t) {
  }
  get _$AU() {
    return this._$AM._$AU;
  }
  _$AT(t, e, i) {
    this._$Ct = t, this._$AM = e, this._$Ci = i;
  }
  _$AS(t, e) {
    return this.update(t, e);
  }
  update(t, e) {
    return this.render(...e);
  }
}
/**
 * @license
 * Copyright 2020 Google LLC
 * SPDX-License-Identifier: BSD-3-Clause
 */
const { I: Ft } = Dt, ht = () => document.createComment(""), U = (r, t, e) => {
  var o;
  const i = r._$AA.parentNode, s = t === void 0 ? r._$AB : t._$AA;
  if (e === void 0) {
    const n = i.insertBefore(ht(), s), h = i.insertBefore(ht(), s);
    e = new Ft(n, h, r, r.options);
  } else {
    const n = e._$AB.nextSibling, h = e._$AM, l = h !== r;
    if (l) {
      let c;
      (o = e._$AQ) == null || o.call(e, r), e._$AM = r, e._$AP !== void 0 && (c = r._$AU) !== h._$AU && e._$AP(c);
    }
    if (n !== s || l) {
      let c = e._$AA;
      for (; c !== n; ) {
        const p = c.nextSibling;
        i.insertBefore(c, s), c = p;
      }
    }
  }
  return e;
}, b = (r, t, e = r) => (r._$AI(t, e), r), qt = {}, Jt = (r, t = qt) => r._$AH = t, Vt = (r) => r._$AH, q = (r) => {
  var i;
  (i = r._$AP) == null || i.call(r, !1, !0);
  let t = r._$AA;
  const e = r._$AB.nextSibling;
  for (; t !== e; ) {
    const s = t.nextSibling;
    t.remove(), t = s;
  }
};
/**
 * @license
 * Copyright 2017 Google LLC
 * SPDX-License-Identifier: BSD-3-Clause
 */
const at = (r, t, e) => {
  const i = /* @__PURE__ */ new Map();
  for (let s = t; s <= e; s++)
    i.set(r[s], s);
  return i;
}, Wt = Ct(class extends xt {
  constructor(r) {
    if (super(r), r.type !== wt.CHILD)
      throw Error("repeat() can only be used in text expressions");
  }
  dt(r, t, e) {
    let i;
    e === void 0 ? e = t : t !== void 0 && (i = t);
    const s = [], o = [];
    let n = 0;
    for (const h of r)
      s[n] = i ? i(h, n) : n, o[n] = e(h, n), n++;
    return { values: o, keys: s };
  }
  render(r, t, e) {
    return this.dt(r, t, e).values;
  }
  update(r, [t, e, i]) {
    const s = Vt(r), { values: o, keys: n } = this.dt(t, e, i);
    if (!Array.isArray(s))
      return this.ut = n, o;
    const h = this.ut ?? (this.ut = []), l = [];
    let c, p, a = 0, u = s.length - 1, d = 0, $ = o.length - 1;
    for (; a <= u && d <= $; )
      if (s[a] === null)
        a++;
      else if (s[u] === null)
        u--;
      else if (h[a] === n[d])
        l[d] = b(s[a], o[d]), a++, d++;
      else if (h[u] === n[$])
        l[$] = b(s[u], o[$]), u--, $--;
      else if (h[a] === n[$])
        l[$] = b(s[a], o[$]), U(r, l[$ + 1], s[a]), a++, $--;
      else if (h[u] === n[d])
        l[d] = b(s[u], o[d]), U(r, s[a], s[u]), u--, d++;
      else if (c === void 0 && (c = at(n, d, $), p = at(h, a, u)), c.has(h[a]))
        if (c.has(h[u])) {
          const _ = p.get(n[d]), j = _ !== void 0 ? s[_] : null;
          if (j === null) {
            const Q = U(r, s[a]);
            b(Q, o[d]), l[d] = Q;
          } else
            l[d] = b(j, o[d]), U(r, s[a], j), s[_] = null;
          d++;
        } else
          q(s[u]), u--;
      else
        q(s[a]), a++;
    for (; d <= $; ) {
      const _ = U(r, l[$ + 1]);
      b(_, o[d]), l[d++] = _;
    }
    for (; a <= u; ) {
      const _ = s[a++];
      _ !== null && q(_);
    }
    return this.ut = n, Jt(r, l), v;
  }
});
/**
 * @license
 * Copyright 2017 Google LLC
 * SPDX-License-Identifier: BSD-3-Clause
 */
class V extends xt {
  constructor(t) {
    if (super(t), this.it = f, t.type !== wt.CHILD)
      throw Error(this.constructor.directiveName + "() can only be used in child bindings");
  }
  render(t) {
    if (t === f || t == null)
      return this._t = void 0, this.it = t;
    if (t === v)
      return t;
    if (typeof t != "string")
      throw Error(this.constructor.directiveName + "() called with a non-string value");
    if (t === this.it)
      return this._t;
    this.it = t;
    const e = [t];
    return e.raw = e, this._t = { _$litType$: this.constructor.resultType, strings: e, values: [] };
  }
}
V.directiveName = "unsafeHTML", V.resultType = 1;
const ct = Ct(V), dt = ["time", "level", "msg"], kt = (r, t = {}, e = "") => {
  if (typeof r != "object")
    return e ? t : r;
  const i = e ? `${e}.` : "";
  for (let s in r) {
    const o = r[s];
    o && (typeof o == "object" ? kt(o, t, i + s) : t[i + s] = o);
  }
  return t;
};
class Zt extends T {
  static get properties() {
    return {
      connected: { type: Boolean },
      error: { type: Error },
      url: { type: String },
      filter: { type: String }
    };
  }
  constructor() {
    super(), this.connected = !1, this.reconnecting = !1, this.filter = "", this.linesCount = 0, this.lines = [], this._handleMessage = this._handleMessage.bind(this), this._filterAlike = this._filterAlike.bind(this);
  }
  connectedCallback() {
    super.connectedCallback();
    const t = this.source = new EventSource(this.url);
    t.onopen = () => {
      this.connected = !0, this.reconnecting = !1, this.error = null, this.requestUpdate();
    }, t.onerror = () => {
      this.connected ? (this.reconnecting = !0, this._append(
        JSON.stringify({
          level: "ERROR",
          msg: "connection lost"
        })
      )) : this.error = new Error("failed to connect to event source"), this.requestUpdate();
    }, t.addEventListener("welcome", this._handleMessage), t.addEventListener("disconnect", this._handleMessage), t.addEventListener("confirm_subscription", this._handleMessage), t.addEventListener("reject_subscription", this._handleMessage), t.addEventListener("ping", this._handleMessage), t.onmessage = this._handleMessage, this.renderRoot.addEventListener("click", this._filterAlike);
  }
  disconnectedCallback() {
    super.disconnectedCallback(), this.renderRoot.removeEventListener("click", this._filterAlike), this.source && (this.source.close(), this.lines.length = 0);
  }
  _handleMessage(t) {
    if (t.type === "ping") {
      this._animateStatus();
      return;
    }
    if (t.type === "welcome") {
      this._append(
        JSON.stringify({
          level: "DEBUG",
          msg: "connected"
        })
      ), this.requestUpdate();
      return;
    }
    if (t.type === "confirm_subscription") {
      this._append(
        JSON.stringify({
          level: "DEBUG",
          msg: "subscribed"
        })
      ), this.requestUpdate();
      return;
    }
    if (t.type === "disconnect") {
      let { reason: s } = JSON.parse(t.data);
      this._append(
        JSON.stringify({
          level: "ERROR",
          msg: "connection closed by server",
          reason: s
        })
      ), this.requestUpdate();
      return;
    }
    const e = JSON.parse(t.data);
    for (let s of e)
      this._append(JSON.stringify(s));
    const i = this.renderRoot.querySelector(".console");
    this.shouldScroll = i.scrollTop + i.offsetHeight + 10 > i.scrollHeight, this.requestUpdate();
  }
  get _filteredLines() {
    return this.lines.filter((t) => this._matchFilter(t));
  }
  _append(t) {
    if (t) {
      try {
        t = JSON.parse(t);
      } catch (e) {
        console.error(e);
        return;
      }
      t = kt(t), this.linesCount++, this.lines.push({ data: t, raw: this._compileLog(t), id: this.linesCount });
    }
  }
  _matchFilter(t) {
    return this.filter ? this.filterRx.test(t.raw) : !0;
  }
  _onFilterChange(t) {
    this._filter(t.target.value);
  }
  _filterAlike(t) {
    if (t.target.classList.contains("log-filter")) {
      t.preventDefault();
      const e = t.target.textContent, i = this.renderRoot.getElementById("filter");
      this._filter(e), i.value = e;
    }
  }
  _resetFilter() {
    const t = this.renderRoot.getElementById("filter");
    this._filter(""), t.value = "";
  }
  _filter(t) {
    this.filter = t, this.filter && (this.filterRx = new RegExp(`((?:^|>)[^<>]*?)(${this.filter})`, "gim")), this.shouldScroll = !0;
  }
  // Generate a string representation of a log for filtering purposes
  _compileLog(t) {
    let e = t.time, i = t.level, s = t.msg, o = [];
    for (let n in t) {
      if (dt.includes(n))
        continue;
      let h = t[n];
      h && (typeof h == "object" && (h = JSON.stringify(h)), o.push(`${n}=${h}`));
    }
    return `${e} ${i} ${s} ${o.join(" ")}`;
  }
  _formatLog(t) {
    let e = y`<span class="log-ts">${this._highlight(t.time)}</span>`, i = y`[<span class="log-filter log-level-${t.level.toLowerCase()}">${this._highlight(t.level)}</span>]`, s = y`<span class="log-message">${this._highlight(
      t.msg
    )}</span>`, o = [];
    for (let n in t) {
      if (dt.includes(n))
        continue;
      let h = t[n];
      if (!h)
        continue;
      typeof h == "object" && (h = JSON.stringify(h));
      const l = `${n}=${h}`;
      o.push(`<span class="log-filter">${l}</span>`);
    }
    return y`<li>${e} ${i} ${s} ${this._highlight(
      o.join(" ")
    )}</li>`;
  }
  _highlight(t) {
    return this.filter ? ct(t.replace(this.filterRx, "$1<mark>$2</mark>")) : ct(t);
  }
  _animateStatus() {
    const t = this.renderRoot.querySelector(".status");
    t && t.classList.add("status-animated");
  }
  _clearStatusAnimation() {
    const t = this.renderRoot.querySelector(".status");
    t && t.classList.remove("status-animated");
  }
  updated() {
    if (super.updated(), this.shouldScroll) {
      this.shouldScroll = !1;
      const t = this.renderRoot.querySelector(".console");
      t.scrollTop = t.scrollHeight - t.offsetHeight;
    }
  }
  render() {
    return this.error ? y`<span class="status status-error"></span><div class="console"><div class="log-level-error">Error: ${this.error.message}</div></div>` : this.connected ? y`
      <span class="status ${this.reconnecting ? "status-loading" : ""}" @animationend=${this._clearStatusAnimation}></span>
      <nav>
        <i id="filter-icon">
          <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor">
          <path stroke-linecap="round" stroke-linejoin="round" d="M12 3c2.755 0 5.455.232 8.083.678.533.09.917.556.917 1.096v1.044a2.25 2.25 0 0 1-.659 1.591l-5.432 5.432a2.25 2.25 0 0 0-.659 1.591v2.927a2.25 2.25 0 0 1-1.244 2.013L9.75 21v-6.568a2.25 2.25 0 0 0-.659-1.591L3.659 7.409A2.25 2.25 0 0 1 3 5.818V4.774c0-.54.384-1.006.917-1.096A48.32 48.32 0 0 1 12 3Z" />
          </svg>
        </i>
        <input type="text" id="filter" @input=${this._onFilterChange}/>
        <i id="reset-filter-icon" @click=${this._resetFilter} title="reset filter" style="${!this.filter && "display: none;"}">
          <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor" class="w-6 h-6">
            <path stroke-linecap="round" stroke-linejoin="round" d="M12 9.75 14.25 12m0 0 2.25 2.25M14.25 12l2.25-2.25M14.25 12 12 14.25m-2.58 4.92-6.374-6.375a1.125 1.125 0 0 1 0-1.59L9.42 4.83c.21-.211.497-.33.795-.33H19.5a2.25 2.25 0 0 1 2.25 2.25v10.5a2.25 2.25 0 0 1-2.25 2.25h-9.284c-.298 0-.585-.119-.795-.33Z" />
          </svg>
        </i>
      </nav>
      <ul class="console">
        ${Wt(
      this._filteredLines,
      (t) => t.id,
      (t) => this._formatLog(t.data)
    )}
      </ul>
    ` : y`<span class="status status-loading"></span><div class="console">Loading...</div>`;
  }
  static get styles() {
    return Pt`
      :host {
        max-width: 1280px;
        height: 100%;
        margin: 0 auto;
        display: block;
        color: var(--console-color, rgb(134 239 172));
        background-color: var(--console-bg, rgb(27, 14, 65));
        border-radius: 8px;
        position: relative;
        font-family: var(--console-font-family, monospace);
      }

      .console {
        min-width: 100%;
        height: 100%;
        box-sizing: border-box;
        padding: 2rem;
        position: relative;
        list-style-type: none;
        word-wrap: break-word;
        overflow-y: scroll;
      }

      .console li {
        word-wrap: break-word;
      }

      .console li:hover {
        color: white;
      }

      .console li:not(:first-child) {
        margin-top: 0.5rem;
      }

      .log-level-info {
        color: cyan;
      }

      .log-level-error {
        color: red;
      }

      .log-level-warn {
        color: #FFBF00;
      }

      .log-filter {
        cursor: pointer;
      }

      .log-filter:hover {
        text-decoration: underline;
      }
      
      @keyframes status-blink {
        0% {opacity: 1;}
        50% {opacity: 0.5;}
        100% {opacity: 1;}
      }
      
      .status-animated {
        animation: status-blink 1s linear;
      }

      .status-loading {
        animation: status-blink 2s linear infinite;
        background-color: #FFBF00 !important;
      }

      .status-error {
        background-color: red !important;
      }

      .status {
        position: absolute;
        background-color: #4FFFB0;
        top: 10px;
        left: 10px;
        display: block;
        width: 10px;
        height: 10px;
        border-radius: 10px;
      }

      nav {
        min-width: 50%;
        position: absolute;
        top: 0.25rem;
        right: 2rem;
        z-index: 10;
        background-color: var(--console-bg, rgb(27, 14, 65));
        background-opacity: 0.75;
      }

      nav i {
        position: absolute;
        width: 1rem;
        height: 1rem;
        color: #fff;
      }

      #filter-icon {
        left: -1.25rem;
        top: 0.125rem;
      }

      #reset-filter-icon {
        right: -0.5rem;
        top: 0.125rem;
        cursor: pointer;
        transition: color 0.5s ease;
      }

      #reset-filter-icon:hover {
        color: var(--console-color, rgb(134 239 172));
      }

      nav input {
        margin-right: 0.75rem;
        width: 100%;
        appearance: none;
        outline: none;
        border-style: none;
        background-color: transparent;
        padding-top: 0.25rem;
        padding-bottom: 0.25rem;
        padding-right: 0.5rem;
        color: var(--controls-color, #fff);
        border-bottom: 1px solid var(--console-color, rgb(134 239 172));
        font-family: var(--console-font-family, monospace);
        font-size: 100%;
      }

      @media (prefers-color-scheme: light) {
      }
    `;
  }
}
window.customElements.define("anycable-logs", Zt);
