// Os filtros valem na hora: trocar qualquer seleção recarrega o mapa com a escolha.
(function () {
  var form = document.getElementById("mapa-filtros");
  if (form) form.addEventListener("change", function () { form.requestSubmit(); });
})();

// Mapa do UYSI ARBO (MapLibre GL). Lê as escolhas dos atributos data-* de #mapa e busca os dados
// do cliente nas rotas /mapa/*.geojson, sempre só da área que está na tela.
(function () {
  var el = document.getElementById("mapa");
  if (!el || typeof maplibregl === "undefined") return;

  var cfg = {
    tiles: el.dataset.tiles,
    credito: el.dataset.credito || "",
    bairros: el.hasAttribute("data-bairros"),
    locais: el.hasAttribute("data-locais"),
    bairro: el.dataset.bairro || "0",
    grupo: el.dataset.grupo || "0",
    local: el.dataset.local || "0",
    tipo: el.dataset.tipo || "",
  };
  var status = document.getElementById("mapa-status");
  var ZOOM_DETALHE = 14; // quarteirões e locais só a partir daqui (senão seria dado demais)
  var cores = {
    residencial: "#2563eb", comercio: "#d97706", terreno_baldio: "#65a30d",
    ponto_estrategico: "#dc2626", outro: "#7c3aed",
  };

  var map = new maplibregl.Map({
    container: el,
    center: [-51.9, -14.2],
    zoom: 4,
    style: {
      version: 8,
      sources: { base: { type: "raster", tiles: [cfg.tiles], tileSize: 256, attribution: cfg.credito, maxzoom: 19 } },
      layers: [{ id: "base", type: "raster", source: "base" }],
    },
  });
  window.uysiMapa = map; // para o app.js liberar o mapa ao trocar de página sem recarregar
  map.addControl(new maplibregl.NavigationControl({ showCompass: false }), "top-right");
  map.addControl(new maplibregl.ScaleControl({ unit: "metric" }), "bottom-left");

  function vazio() { return { type: "FeatureCollection", features: [] }; }
  function url(caminho, extra) {
    var b = map.getBounds();
    var p = new URLSearchParams(extra || {});
    p.set("bbox", [b.getWest(), b.getSouth(), b.getEast(), b.getNorth()].map(function (n) { return n.toFixed(6); }).join(","));
    return caminho + "?" + p.toString();
  }
  function buscar(u) {
    return fetch(u, { credentials: "same-origin", headers: { Accept: "application/geo+json" } }).then(function (r) {
      if (!r.ok) throw new Error("HTTP " + r.status);
      return r.json();
    });
  }
  function mostrar(texto) { if (status) status.textContent = texto; }
  function camadaLigada(nome) {
    var c = document.querySelector('[data-camada="' + nome + '"]');
    return !c || c.checked;
  }

  function popup(lngLat, titulo, linhas, href, rotuloLink) {
    var caixa = document.createElement("div");
    var t = document.createElement("strong");
    t.textContent = titulo;
    caixa.appendChild(t);
    linhas.forEach(function (l) {
      var p = document.createElement("div");
      p.textContent = l;
      caixa.appendChild(p);
    });
    var a = document.createElement("a");
    a.href = href;
    a.textContent = rotuloLink;
    caixa.appendChild(a);
    new maplibregl.Popup({ closeButton: true }).setLngLat(lngLat).setDOMContent(caixa).addTo(map);
  }

  var enquadrou = false;
  function enquadrar(colecao) {
    if (enquadrou || !colecao.features.length) return;
    var b = new maplibregl.LngLatBounds();
    function estende(c) { if (typeof c[0] === "number") b.extend(c); else c.forEach(estende); }
    colecao.features.forEach(function (f) { estende(f.geometry.coordinates); });
    if (!b.isEmpty()) { map.fitBounds(b, { padding: 40, maxZoom: 17, duration: 0 }); enquadrou = true; }
  }

  map.on("load", function () {
    map.addSource("bairros", { type: "geojson", data: vazio() });
    map.addSource("quarteiroes", { type: "geojson", data: vazio() });
    map.addSource("locais", { type: "geojson", data: vazio() });

    map.addLayer({ id: "bairros-fill", type: "fill", source: "bairros", paint: { "fill-color": "#155a9e", "fill-opacity": 0.12 } });
    map.addLayer({ id: "bairros-line", type: "line", source: "bairros", paint: { "line-color": "#155a9e", "line-width": 2 } });
    map.addLayer({ id: "bairros-nome", type: "symbol", source: "bairros", maxzoom: ZOOM_DETALHE,
      layout: { "text-field": ["get", "nome"], "text-size": 12 }, paint: { "text-color": "#0f3d6e", "text-halo-color": "#fff", "text-halo-width": 1.5 } });
    map.addLayer({ id: "quarteiroes-line", type: "line", source: "quarteiroes", minzoom: ZOOM_DETALHE, paint: { "line-color": "#475569", "line-width": 1, "line-dasharray": [2, 1] } });
    map.addLayer({ id: "quarteiroes-fill", type: "fill", source: "quarteiroes", minzoom: ZOOM_DETALHE, paint: { "fill-color": "#94a3b8", "fill-opacity": 0.06 } });
    map.addLayer({ id: "quarteiroes-nome", type: "symbol", source: "quarteiroes", minzoom: 16,
      layout: { "text-field": ["get", "nome"], "text-size": 11 }, paint: { "text-color": "#334155", "text-halo-color": "#fff", "text-halo-width": 1.5 } });
    map.addLayer({
      id: "locais-ponto", type: "circle", source: "locais", minzoom: ZOOM_DETALHE,
      paint: {
        "circle-radius": ["interpolate", ["linear"], ["zoom"], 14, 3, 18, 8],
        "circle-color": ["case", ["==", ["get", "ativo"], false], "#94a3b8",
          ["match", ["get", "tipo"], "residencial", cores.residencial, "comercio", cores.comercio, "terreno_baldio", cores.terreno_baldio,
            "ponto_estrategico", cores.ponto_estrategico, cores.outro]],
        "circle-stroke-color": "#fff", "circle-stroke-width": 1.5,
      },
    });

    map.on("click", "locais-ponto", function (e) {
      var f = e.features[0];
      popup(e.lngLat, f.properties.endereco, ["Bairro " + f.properties.bairro], "/locais/" + f.properties.id, "Abrir o local");
    });
    map.on("click", "quarteiroes-fill", function (e) {
      if (map.queryRenderedFeatures(e.point, { layers: ["locais-ponto"] }).length) return;
      var f = e.features[0];
      popup(e.lngLat, "Quarteirão " + f.properties.nome, ["Bairro " + f.properties.bairro], "/bairros/" + f.properties.bairro_id + "#quarteiroes", "Abrir o bairro");
    });
    map.on("click", "bairros-fill", function (e) {
      if (map.getZoom() >= ZOOM_DETALHE) return;
      var f = e.features[0];
      popup(e.lngLat, f.properties.nome, [], "/bairros/" + f.properties.id, "Abrir o bairro");
    });
    ["locais-ponto", "quarteiroes-fill", "bairros-fill"].forEach(function (id) {
      map.on("mouseenter", id, function () { map.getCanvas().style.cursor = "pointer"; });
      map.on("mouseleave", id, function () { map.getCanvas().style.cursor = ""; });
    });

    document.querySelectorAll("[data-camada]").forEach(function (c) {
      c.addEventListener("change", function () {
        var nome = c.dataset.camada;
        var visivel = c.checked ? "visible" : "none";
        map.getStyle().layers.forEach(function (l) { if (l.source === nome) map.setLayoutProperty(l.id, "visibility", visivel); });
        atualizar();
      });
    });

    var filtro = {};
    if (cfg.bairro !== "0") filtro.bairro = cfg.bairro;

    var primeira = true;
    var espera = null;
    function atualizar() {
      var zoom = map.getZoom();
      var eraPrimeira = primeira;
      var tarefas = [];
      if (cfg.bairros && eraPrimeira) {
        tarefas.push(buscar("/mapa/bairros.geojson" + (filtro.bairro ? "?bairro=" + filtro.bairro : "")).then(function (c) {
          map.getSource("bairros").setData(c);
          if (cfg.local === "0") enquadrar(c);
        }));
      }
      if (cfg.bairros && camadaLigada("quarteiroes") && zoom >= ZOOM_DETALHE) {
        tarefas.push(buscar(url("/mapa/quarteiroes.geojson", filtro)).then(function (c) { map.getSource("quarteiroes").setData(c); }));
      }
      var msg = "";
      var locaisDoBairro = null;
      if (cfg.locais && camadaLigada("locais")) {
        var f = Object.assign({}, filtro);
        if (cfg.grupo !== "0") f.grupo = cfg.grupo;
        if (cfg.tipo) f.tipo = cfg.tipo;
        if (cfg.local !== "0") f.local = cfg.local;
        if (zoom >= ZOOM_DETALHE || cfg.local !== "0" || cfg.grupo !== "0" || cfg.bairro !== "0") {
          tarefas.push(buscar(url("/mapa/locais.geojson", f)).then(function (c) {
            map.getSource("locais").setData(c);
            if (eraPrimeira && (cfg.local !== "0" || cfg.grupo !== "0")) enquadrar(c);
            else if (eraPrimeira && cfg.bairro !== "0") locaisDoBairro = c;
            msg = c.features.length + " local(is) na tela";
            if (c.truncado) msg += " (mostrando os primeiros; aproxime o mapa)";
            if (c.sem_ponto) msg += " · " + c.sem_ponto + " sem coordenadas";
          }));
        } else {
          msg = "Aproxime o mapa para ver os locais.";
        }
      }
      primeira = false;
      Promise.all(tarefas).then(function () {
        // Bairro sem polígono desenhado: o mapa se enquadra pelos locais dele.
        if (locaisDoBairro) enquadrar(locaisDoBairro);
        mostrar(msg);
      }).catch(function () { mostrar("Não foi possível carregar o mapa. Tente de novo."); });
    }
    map.on("moveend", function () { clearTimeout(espera); espera = setTimeout(atualizar, 250); });
    atualizar();
  });
})();
