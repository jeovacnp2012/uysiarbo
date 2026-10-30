// Rastro GPS: pontos das visitas com coordenadas e a linha do dia de cada agente (MapLibre GL).
(function () {
  var form = document.getElementById("rastro-filtros");
  if (form) form.addEventListener("change", function () { form.requestSubmit(); });

  var el = document.getElementById("mapa");
  if (!el || typeof maplibregl === "undefined") return;
  var status = document.getElementById("mapa-status");

  var map = new maplibregl.Map({
    container: el,
    center: [-51.9, -14.2],
    zoom: 4,
    style: {
      version: 8,
      sources: { base: { type: "raster", tiles: [el.dataset.tiles], tileSize: 256, attribution: el.dataset.credito || "", maxzoom: 19 } },
      layers: [{ id: "base", type: "raster", source: "base" }],
    },
  });
  window.uysiMapa = map; // para o app.js liberar o mapa ao trocar de página sem recarregar
  map.addControl(new maplibregl.NavigationControl({ showCompass: false }), "top-right");
  map.addControl(new maplibregl.ScaleControl({ unit: "metric" }), "bottom-left");

  // O popup abre ao passar o mouse; sai pouco depois de o mouse deixar o ponto, a não ser que ele esteja sobre o popup.
  // O clique (ou o toque, no celular) fixa o popup até fechar no X.
  var aberto = null, fixo = false, fechar = null;
  function cancelarFechar() { if (fechar) { clearTimeout(fechar); fechar = null; } }
  function agendarFechar() {
    cancelarFechar();
    if (fixo) return;
    fechar = setTimeout(function () { if (aberto && !fixo) { aberto.remove(); aberto = null; } }, 300);
  }
  function popup(lngLat, p, fixar) {
    cancelarFechar();
    if (aberto) { aberto.remove(); aberto = null; }
    fixo = !!fixar;
    var caixa = document.createElement("div");
    caixa.style.color = "#0f172a"; // fundo do popup é claro, inclusive no tema escuro
    caixa.addEventListener("mouseenter", cancelarFechar);
    caixa.addEventListener("mouseleave", agendarFechar);
    var t = document.createElement("strong");
    t.textContent = p.endereco;
    caixa.appendChild(t);
    [p.do_local ? "Posição do local (visita sem GPS)" : "", p.agente, p.data + (p.hora ? " " + p.hora : ""), p.precisao ? "Precisão " + Math.round(p.precisao) + " m" : ""].forEach(function (l) {
      if (!l) return;
      var d = document.createElement("div");
      d.textContent = l;
      caixa.appendChild(d);
    });
    var a = document.createElement("a");
    a.href = "/visitas/" + p.id;
    a.textContent = "Abrir a visita";
    a.style.color = "#155a9e";
    caixa.appendChild(a);
    aberto = new maplibregl.Popup({ closeButton: true, offset: 8, closeOnClick: false }).setLngLat(lngLat).setDOMContent(caixa).addTo(map);
    aberto.on("close", function () { fixo = false; });
  }

  map.on("load", function () {
    map.addSource("rastro", { type: "geojson", data: { type: "FeatureCollection", features: [] } });
    map.addLayer({ id: "rastro-linha", type: "line", source: "rastro", filter: ["==", ["get", "linha"], true],
      paint: { "line-color": "#155a9e", "line-width": 2, "line-opacity": 0.7 } });
    map.addLayer({ id: "rastro-ponto", type: "circle", source: "rastro", filter: ["!=", ["get", "linha"], true],
      paint: {
        "circle-radius": 5,
        "circle-color": ["case", ["!=", ["get", "pendencia"], "nenhuma"], "#94a3b8", ["get", "deposito"], "#d97706", "#2563eb"],
        "circle-stroke-color": ["case", ["==", ["get", "do_local"], true], "#0f172a", "#fff"], "circle-stroke-width": 1.5,
      } });
    map.on("click", "rastro-ponto", function (e) { popup(e.features[0].geometry.coordinates.slice(), e.features[0].properties, true); });
    map.on("mouseenter", "rastro-ponto", function (e) {
      map.getCanvas().style.cursor = "pointer";
      if (!fixo) popup(e.features[0].geometry.coordinates.slice(), e.features[0].properties, false);
    });
    map.on("mouseleave", "rastro-ponto", function () { map.getCanvas().style.cursor = ""; agendarFechar(); });

    var qs = window.location.search;
    fetch("/mapa/rastro.geojson" + qs, { credentials: "same-origin" }).then(function (r) {
      if (!r.ok) throw new Error("HTTP " + r.status);
      return r.json();
    }).then(function (col) {
      map.getSource("rastro").setData(col);
      var b = new maplibregl.LngLatBounds();
      col.features.forEach(function (f) { if (f.geometry.type === "Point") b.extend(f.geometry.coordinates); });
      if (!b.isEmpty()) map.fitBounds(b, { padding: 50, maxZoom: 17, duration: 0 });
      if (status) {
        status.textContent = col.pontos === 0 ? "Nenhuma visita com GPS neste filtro." :
          col.pontos + " visitas com GPS" + (col.truncado ? " (mostrando as primeiras " + col.pontos + "; estreite o filtro)" : "");
      }
    }).catch(function () { if (status) status.textContent = "Não foi possível carregar o rastro."; });
  });
})();
